package sitemapjsonld

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	htmlnode "golang.org/x/net/html"

	entity "github.com/ibednov/go-lepsios/crawler"
	crawlerr "github.com/ibednov/go-lepsios/crawler/errors"
	"github.com/ibednov/go-lepsios/crawler/httpclient"
	"github.com/ibednov/go-lepsios/crawler/pricing"
	"github.com/shopspring/decimal"
)

const Kind = "sitemap_jsonld"

var (
	jsonScript                 = regexp.MustCompile(`(?is)<script\b[^>]*type=["']([^"']+)["'][^>]*>(.*?)</script>`)
	pathPattern                = regexp.MustCompile(`^/[^?#]{1,240}$`)
	selectorName               = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)
	weightPattern              = regexp.MustCompile(`(?i)(?:^|[^\pL\d])(?:(\d{1,4})\s*[*×xх]\s*)?(\d{1,4})\s*(кг|kg|грамм(?:ов|а)?|гр\.?|г|g)(?:$|[^\pL])`)
	belarusianKopPricePattern  = regexp.MustCompile(`(?i)^(\d+)[pр](\d{1,2})\s*коп\.?$`)
	pricePattern               = regexp.MustCompile(`(?i)^(?:\d+(?:[.,]\d+)?|\d{1,3}(?: \d{3})+(?:[.,]\d+)?)\s*(?:BYN|Br|руб\.?|бел\.?\s*руб\.?)?$`)
	errCategoryCardDisappeared = errors.New("category card disappeared")
)

type Config struct {
	Version                          int               `json:"version"`
	SitemapURL                       string            `json:"sitemap_url"`
	ProductPathPrefixes              []string          `json:"product_path_prefixes"`
	IncludeCategoryHints             []string          `json:"include_category_hints"`
	ExcludeCategoryHints             []string          `json:"exclude_category_hints"`
	ScriptTypes                      []string          `json:"script_types"`
	DataFormats                      []string          `json:"data_formats"`
	DefaultCurrency                  string            `json:"currency"`
	IncludePaths                     []string          `json:"include_paths"`
	ExcludePaths                     []string          `json:"exclude_paths"`
	IncludeNameKeywords              []string          `json:"include_name_keywords"`
	ExcludeNameKeywords              []string          `json:"exclude_name_keywords"`
	CategoryURLs                     []string          `json:"category_urls"`
	PriceSelector                    string            `json:"price_selector"`
	UserAgent                        string            `json:"user_agent"`
	AcceptLanguage                   string            `json:"accept_language"`
	HTMLTitleSelector                string            `json:"html_title_selector"`
	HTMLProductContainerSelector     string            `json:"html_product_container_selector"`
	HTMLPriceSelector                string            `json:"html_price_selector"`
	HTMLStockSelector                string            `json:"html_stock_selector"`
	HTMLOutOfStockSelector           string            `json:"html_out_of_stock_selector"`
	HTMLWeightSelector               string            `json:"html_weight_selector"`
	HTMLOptionSelector               string            `json:"html_option_selector"`
	HTMLOptionNameAttribute          string            `json:"html_option_name_attribute"`
	HTMLOptionName                   string            `json:"html_option_name"`
	HTMLOptionPriceAttribute         string            `json:"html_option_price_attribute"`
	HTMLOptionValueAttribute         string            `json:"html_option_value_attribute"`
	HTMLOptionIDAttribute            string            `json:"html_option_id_attribute"`
	HTMLProductIDSelector            string            `json:"html_product_id_selector"`
	HTMLProductIDNameAttribute       string            `json:"html_product_id_name_attribute"`
	HTMLProductIDName                string            `json:"html_product_id_name"`
	HTMLProductIDValueAttribute      string            `json:"html_product_id_value_attribute"`
	IncludePathContains              []string          `json:"include_path_contains"`
	ExcludePathContains              []string          `json:"exclude_path_contains"`
	IncludePathRegex                 []string          `json:"include_path_regex"`
	ExcludePathRegex                 []string          `json:"exclude_path_regex"`
	CategoryCardSelector             string            `json:"category_card_selector"`
	CategoryCardTitleSelector        string            `json:"category_card_title_selector"`
	CategoryCardPriceSelector        string            `json:"category_card_price_selector"`
	CategoryCardStockSelector        string            `json:"category_card_stock_selector"`
	CategoryCardWeightSelector       string            `json:"category_card_weight_selector"`
	CategoryCardIDAttribute          string            `json:"category_card_id_attribute"`
	CategoryEmptySelector            string            `json:"category_empty_selector"`
	SkipNotFound                     bool              `json:"skip_not_found"`
	SkipMissingCard                  bool              `json:"skip_missing_card"`
	SnapshotCategoryCards            bool              `json:"snapshot_category_cards"`
	SkipOutOfStockZeroPrice          bool              `json:"skip_out_of_stock_zero_price"`
	SkipHTMLOutOfStockWithoutOptions bool              `json:"skip_html_out_of_stock_without_options"`
	MaxResponseBytes                 int64             `json:"max_response_bytes"`
	Collect                          []collectRule     `json:"collect"`
	AttrMap                          map[string]string `json:"attr_map"`
}

type collectRule struct {
	Kind              string   `json:"kind"`
	Enabled           *bool    `json:"enabled,omitempty"`
	IncludeCategories []string `json:"include_categories,omitempty"`
	IncludeKeywords   []string `json:"include_keywords,omitempty"`
	ExcludeKeywords   []string `json:"exclude_keywords,omitempty"`
}

type Adapter struct {
	cli          *httpclient.Client
	code         string
	base         *url.URL
	config       Config
	rateLimit    float64
	limiterMu    sync.Mutex
	lastRequest  time.Time
	diagnosticMu sync.Mutex
	diagnostic   *httpclient.DiagnosticResponse
	categoryMu   sync.RWMutex
	categoryBody map[string][]byte
}

func New(cli *httpclient.Client, code, baseURL string, rateLimit float64, raw json.RawMessage) (*Adapter, error) {
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("decode %s config: %w", Kind, err)
	}
	if err := validateConfig(baseURL, cfg); err != nil {
		return nil, err
	}
	base, _ := url.Parse(strings.TrimRight(upgradePublicURL(baseURL), "/"))
	cfg.SitemapURL = upgradePublicURL(cfg.SitemapURL)
	for i, raw := range cfg.CategoryURLs {
		cfg.CategoryURLs[i] = upgradePublicURL(raw)
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.DefaultCurrency == "" {
		cfg.DefaultCurrency = "BYN"
	}
	if len(cfg.ScriptTypes) == 0 {
		cfg.ScriptTypes = []string{"application/ld+json"}
	}
	if len(cfg.DataFormats) == 0 {
		cfg.DataFormats = []string{"jsonld"}
	}
	if rateLimit <= 0 {
		rateLimit = 1
	}
	return &Adapter{cli: cli, code: code, base: base, config: cfg, rateLimit: rateLimit, categoryBody: make(map[string][]byte)}, nil
}

// ValidateConfig checks a persisted adapter configuration without network I/O.
func ValidateConfig(baseURL string, raw json.RawMessage) error {
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("decode %s config: %w", Kind, err)
	}
	return validateConfig(baseURL, cfg)
}

func validateConfig(baseURL string, cfg Config) error {
	base, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || base.Hostname() == "" || (base.Scheme != "https" && base.Scheme != "http") {
		return fmt.Errorf("%s requires an http(s) base_url", Kind)
	}
	if cfg.DefaultCurrency != "" {
		if _, err := pricing.NormalizeCurrency(cfg.DefaultCurrency); err != nil {
			return fmt.Errorf("%s currency %q is not supported", Kind, cfg.DefaultCurrency)
		}
	}
	if cfg.MaxResponseBytes < 0 || cfg.MaxResponseBytes > httpclient.MaxConfiguredResponseBytes {
		return fmt.Errorf("max_response_bytes must be between 0 and %d", httpclient.MaxConfiguredResponseBytes)
	}
	if len(cfg.Collect) > 30 {
		return fmt.Errorf("collect supports at most 30 rules")
	}
	seenKinds := map[string]bool{}
	for i, rule := range cfg.Collect {
		if !selectorName.MatchString(rule.Kind) {
			return fmt.Errorf("collect[%d].kind is invalid", i)
		}
		if seenKinds[rule.Kind] {
			return fmt.Errorf("collect duplicates kind %q", rule.Kind)
		}
		seenKinds[rule.Kind] = true
		for _, list := range [][]string{rule.IncludeCategories, rule.IncludeKeywords, rule.ExcludeKeywords} {
			if len(list) > 200 {
				return fmt.Errorf("collect[%d] list exceeds 200 entries", i)
			}
			for _, value := range list {
				if strings.TrimSpace(value) == "" {
					return fmt.Errorf("collect[%d] contains an empty rule", i)
				}
			}
		}
	}
	if len(cfg.AttrMap) > 500 {
		return fmt.Errorf("attr_map supports at most 500 entries")
	}
	for sourceKey, code := range cfg.AttrMap {
		if strings.TrimSpace(sourceKey) == "" || !selectorName.MatchString(code) {
			return fmt.Errorf("invalid attr_map entry %q", sourceKey)
		}
	}
	for _, scriptType := range cfg.ScriptTypes {
		switch strings.ToLower(strings.TrimSpace(scriptType)) {
		case "application/ld+json", "application/json", "application/":
		default:
			return fmt.Errorf("unsupported JSON script type %q", scriptType)
		}
	}
	for _, format := range cfg.DataFormats {
		switch strings.ToLower(strings.TrimSpace(format)) {
		case "jsonld", "microdata", "html":
		default:
			return fmt.Errorf("unsupported structured data format %q", format)
		}
	}
	if cfg.SitemapURL == "" && len(cfg.CategoryURLs) == 0 {
		return fmt.Errorf("%s requires sitemap_url or category_urls", Kind)
	}
	if cfg.CategoryEmptySelector != "" && len(cfg.CategoryURLs) == 0 {
		return fmt.Errorf("category_empty_selector requires category_urls")
	}
	if cfg.SnapshotCategoryCards && cfg.CategoryCardSelector == "" {
		return fmt.Errorf("snapshot_category_cards requires category_card_selector")
	}
	if cfg.SitemapURL != "" {
		sitemap, err := url.Parse(cfg.SitemapURL)
		if err != nil || sitemap.Hostname() == "" || !sameOrigin(base, sitemap) {
			return fmt.Errorf("%s sitemap_url must use the source origin", Kind)
		}
	}
	for _, prefix := range cfg.ProductPathPrefixes {
		if !strings.HasPrefix(prefix, "/") || !pathPattern.MatchString(prefix) {
			return fmt.Errorf("invalid product path prefix %q", prefix)
		}
	}
	for _, path := range append(append([]string{}, cfg.IncludePaths...), cfg.ExcludePaths...) {
		if !strings.HasPrefix(path, "/") || !pathPattern.MatchString(path) {
			return fmt.Errorf("invalid path filter %q", path)
		}
	}
	for _, raw := range cfg.CategoryURLs {
		u, err := url.Parse(raw)
		if err != nil || !sameOrigin(base, u) || u.User != nil {
			return fmt.Errorf("category URL must use the source origin: %q", raw)
		}
	}
	for _, expression := range append(append([]string{}, cfg.IncludePathRegex...), cfg.ExcludePathRegex...) {
		if _, err := regexp.Compile(expression); err != nil {
			return fmt.Errorf("invalid path regex %q: %w", expression, err)
		}
	}
	for _, fragment := range append(append([]string{}, cfg.IncludePathContains...), cfg.ExcludePathContains...) {
		if strings.TrimSpace(fragment) == "" {
			return fmt.Errorf("path contains filter must not be empty")
		}
	}
	if slices.Contains(cfg.DataFormats, "html") && (cfg.HTMLTitleSelector == "" || (firstNonEmpty(cfg.HTMLPriceSelector, cfg.PriceSelector) == "" && cfg.HTMLOptionSelector == "")) {
		return fmt.Errorf("html format requires html_title_selector and either html_price_selector or html_option_selector")
	}
	for _, selector := range []string{cfg.PriceSelector, cfg.HTMLTitleSelector, cfg.HTMLProductContainerSelector, cfg.HTMLPriceSelector, cfg.HTMLStockSelector, cfg.HTMLOutOfStockSelector, cfg.HTMLWeightSelector, cfg.HTMLOptionSelector, cfg.HTMLProductIDSelector, cfg.CategoryCardSelector, cfg.CategoryCardTitleSelector, cfg.CategoryCardPriceSelector, cfg.CategoryCardStockSelector, cfg.CategoryCardWeightSelector, cfg.CategoryEmptySelector} {
		if selector != "" && !validSelector(selector) {
			return fmt.Errorf("unsupported HTML selector %q; use tag, #id, .class or tag.class", selector)
		}
	}
	for _, attr := range []string{cfg.HTMLOptionNameAttribute, cfg.HTMLOptionPriceAttribute, cfg.HTMLOptionValueAttribute, cfg.HTMLOptionIDAttribute, cfg.HTMLProductIDNameAttribute, cfg.HTMLProductIDValueAttribute} {
		if attr != "" && !selectorName.MatchString(attr) {
			return fmt.Errorf("invalid HTML option attribute name %q", attr)
		}
	}
	if cfg.HTMLOptionSelector != "" && (cfg.HTMLOptionPriceAttribute == "" || cfg.HTMLOptionValueAttribute == "") {
		return fmt.Errorf("html_option_selector requires price and value attributes")
	}
	if cfg.SkipHTMLOutOfStockWithoutOptions && cfg.HTMLOutOfStockSelector == "" {
		return fmt.Errorf("skip_html_out_of_stock_without_options requires html_out_of_stock_selector")
	}
	if cfg.HTMLOptionNameAttribute != "" && cfg.HTMLOptionName == "" {
		return fmt.Errorf("html_option_name_attribute requires html_option_name")
	}
	if cfg.HTMLProductIDSelector != "" && (cfg.HTMLProductIDNameAttribute == "" || cfg.HTMLProductIDName == "" || cfg.HTMLProductIDValueAttribute == "" || cfg.HTMLOptionIDAttribute == "") {
		return fmt.Errorf("html_product_id_selector requires product ID name/value and option ID attributes")
	}
	if cfg.CategoryCardSelector != "" && (len(cfg.CategoryURLs) == 0 || cfg.CategoryCardTitleSelector == "" || cfg.CategoryCardPriceSelector == "" || cfg.CategoryCardIDAttribute == "") {
		return fmt.Errorf("category cards require category_urls, title, price and id attribute selectors")
	}
	if cfg.CategoryCardIDAttribute != "" && !selectorName.MatchString(cfg.CategoryCardIDAttribute) {
		return fmt.Errorf("invalid category card ID attribute %q", cfg.CategoryCardIDAttribute)
	}
	if strings.ContainsAny(cfg.UserAgent+cfg.AcceptLanguage, "\r\n") {
		return fmt.Errorf("request headers must not contain line breaks")
	}
	return nil
}

func validSelector(selector string) bool {
	selector = strings.TrimSpace(selector)
	if strings.HasPrefix(selector, "#") || strings.HasPrefix(selector, ".") {
		return selectorName.MatchString(selector[1:])
	}
	parts := strings.Split(selector, ".")
	return len(parts) <= 2 && selectorName.MatchString(parts[0]) && (len(parts) == 1 || selectorName.MatchString(parts[1]))
}

func (a *Adapter) Code() string { return a.code }

func (a *Adapter) LastDiagnosticResponse() (httpclient.DiagnosticResponse, bool) {
	a.diagnosticMu.Lock()
	defer a.diagnosticMu.Unlock()
	if a.diagnostic == nil {
		return httpclient.DiagnosticResponse{}, false
	}
	copy := *a.diagnostic
	copy.Body = append([]byte(nil), a.diagnostic.Body...)
	return copy, true
}

func (a *Adapter) Discover(ctx context.Context) ([]entity.ProductRef, error) {
	if len(a.config.CategoryURLs) > 0 {
		refs, confirmedEmpty, err := a.discoverCategories(ctx)
		if err == nil && confirmedEmpty {
			err = crawlerr.ParseFailed(a.code, fmt.Errorf("empty category discovery requires the extended outcome contract"))
		}
		return refs, err
	}
	queue := []string{a.config.SitemapURL}
	seenSitemaps := map[string]struct{}{}
	seenProducts := map[string]struct{}{}
	refs := make([]entity.ProductRef, 0)
	for len(queue) > 0 {
		if len(seenSitemaps) >= 20 {
			return nil, crawlerr.ParseFailed(a.code, fmt.Errorf("sitemap index exceeds 20 documents"))
		}
		current := queue[0]
		queue = queue[1:]
		if _, ok := seenSitemaps[current]; ok {
			continue
		}
		seenSitemaps[current] = struct{}{}
		body, err := a.fetch(ctx, current)
		if err != nil {
			return nil, crawlerr.Transient(a.code, err)
		}
		var document struct {
			URLs []struct {
				Loc string `xml:"loc"`
			} `xml:"url"`
			Sitemaps []struct {
				Loc string `xml:"loc"`
			} `xml:"sitemap"`
		}
		if err := xml.Unmarshal(body, &document); err != nil {
			return nil, crawlerr.ParseFailed(a.code, fmt.Errorf("decode sitemap %s: %w", current, err))
		}
		for _, child := range document.Sitemaps {
			if u, ok := a.sameSiteURL(child.Loc); ok {
				queue = append(queue, u.String())
			}
		}
		for _, item := range document.URLs {
			raw := strings.TrimSpace(item.Loc)
			u, ok := a.sameSiteURL(raw)
			if !ok || !a.isProductPath(u.Path) || a.isCategoryPage(u) {
				continue
			}
			key := u.String()
			if _, ok := seenProducts[key]; ok {
				continue
			}
			seenProducts[key] = struct{}{}
			refs = append(refs, entity.ProductRef{SourceCode: a.code, URL: key})
		}
	}
	if len(refs) == 0 {
		return nil, crawlerr.ParseFailed(a.code, fmt.Errorf("no product pages matched configured path prefixes"))
	}
	return refs, nil
}

// DiscoverResult provides explicit empty-catalog evidence only for configured
// category pages whose every successfully traversed page contains the operator
// supplied empty-state selector. Sitemap emptiness remains a parse failure.
func (a *Adapter) DiscoverResult(ctx context.Context) (entity.DiscoveryResult, error) {
	if len(a.config.CategoryURLs) > 0 {
		refs, confirmedEmpty, err := a.discoverCategories(ctx)
		if err != nil {
			return entity.DiscoveryResult{Outcome: entity.OutcomeForError(err)}, err
		}
		if len(refs) > 0 {
			return entity.DiscoveryResult{Refs: refs, Outcome: entity.OutcomeOK}, nil
		}
		if confirmedEmpty {
			return entity.DiscoveryResult{Outcome: entity.OutcomeConfirmedEmpty, Evidence: fmt.Sprintf("all successfully traversed category pages matched configured empty selector %q", a.config.CategoryEmptySelector)}, nil
		}
		return entity.DiscoveryResult{Outcome: entity.OutcomeParseFailed}, crawlerr.ParseFailed(a.code, fmt.Errorf("empty category discovery lacks configured empty-state evidence"))
	}
	refs, err := a.Discover(ctx)
	if err != nil {
		return entity.DiscoveryResult{Outcome: entity.OutcomeForError(err)}, err
	}
	return entity.DiscoveryResult{Refs: refs, Outcome: entity.OutcomeOK}, nil
}

// FetchResult preserves explicit skip semantics while exposing successful offers
// through the opt-in outcome contract.
func (a *Adapter) FetchResult(ctx context.Context, ref entity.ProductRef) (entity.FetchResult, error) {
	offers, err := a.Fetch(ctx, ref)
	if errors.Is(err, crawlerr.ErrSkippedProduct) {
		return entity.FetchResult{Outcome: entity.OutcomeSkipped, Reason: "configured parser rule skipped this product"}, nil
	}
	if err != nil {
		return entity.FetchResult{Outcome: entity.OutcomeForError(err)}, err
	}
	return entity.FetchResult{Offers: offers, Outcome: entity.OutcomeOK}, nil
}

// Category pages are read afresh on every discovery. Pagination follows rel=next
// links on the same origin, with a fixed ceiling to bound accidental loops.
func (a *Adapter) discoverCategories(ctx context.Context) ([]entity.ProductRef, bool, error) {
	queue := append([]string(nil), a.config.CategoryURLs...)
	categoryPaths := map[string]struct{}{}
	for _, raw := range a.config.CategoryURLs {
		u, _ := url.Parse(raw)
		categoryPaths[strings.TrimRight(u.Path, "/")] = struct{}{}
	}
	seenPages := map[string]struct{}{}
	seenProducts := map[string]struct{}{}
	var refs []entity.ProductRef
	allPagesMarkedEmpty := a.config.CategoryEmptySelector != ""
	for len(queue) > 0 {
		if len(seenPages) >= 100 {
			return nil, false, crawlerr.ParseFailed(a.code, fmt.Errorf("category discovery exceeds 100 pages"))
		}
		current := queue[0]
		queue = queue[1:]
		if _, ok := seenPages[current]; ok {
			continue
		}
		seenPages[current] = struct{}{}
		body, err := a.fetch(ctx, current)
		if err != nil {
			return nil, false, crawlerr.Transient(a.code, err)
		}
		if a.config.SnapshotCategoryCards {
			a.cacheCategoryBody(current, body)
		}
		doc, err := htmlnode.Parse(bytes.NewReader(body))
		if err != nil {
			return nil, false, crawlerr.ParseFailed(a.code, err)
		}
		pageHasEmptyMarker := a.config.CategoryEmptySelector != "" && hasSelector(doc, a.config.CategoryEmptySelector)
		allPagesMarkedEmpty = allPagesMarkedEmpty && pageHasEmptyMarker
		page, _ := url.Parse(current)
		if a.config.CategoryCardSelector != "" {
			for _, id := range categoryCardIDs(doc, a.config) {
				cardURL := *page
				cardURL.Fragment = "product-" + id
				key := cardURL.String()
				if _, exists := seenProducts[key]; !exists {
					seenProducts[key] = struct{}{}
					refs = append(refs, entity.ProductRef{SourceCode: a.code, URL: key})
				}
			}
		}
		var walk func(*htmlnode.Node)
		walk = func(n *htmlnode.Node) {
			if n.Type == htmlnode.ElementNode && (n.Data == "a" || n.Data == "link") {
				href := strings.TrimSpace(attrValue(n, "href"))
				if target, err := url.Parse(href); err == nil && href != "" {
					u := page.ResolveReference(target)
					if upgraded, err := url.Parse(upgradePublicURL(u.String())); err == nil {
						u = upgraded
					}
					if sameOrigin(a.base, u) && u.User == nil {
						u.Fragment = ""
						key := u.String()
						_, isCategory := categoryPaths[strings.TrimRight(u.Path, "/")]
						if a.config.CategoryCardSelector == "" && a.isProductPath(u.Path) && !isCategory {
							if _, exists := seenProducts[key]; !exists {
								seenProducts[key] = struct{}{}
								refs = append(refs, entity.ProductRef{SourceCode: a.code, URL: key})
							}
						} else if strings.EqualFold(attrValue(n, "rel"), "next") {
							queue = append(queue, key)
						}
					}
				}
			}
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				walk(child)
			}
		}
		walk(doc)
	}
	if len(refs) == 0 {
		if allPagesMarkedEmpty {
			return nil, true, nil
		}
		return nil, false, crawlerr.ParseFailed(a.code, fmt.Errorf("no product links found on category pages"))
	}
	return refs, false, nil
}

func hasSelector(doc *htmlnode.Node, selector string) bool {
	found := false
	var walk func(*htmlnode.Node)
	walk = func(n *htmlnode.Node) {
		if found {
			return
		}
		if n.Type == htmlnode.ElementNode && matchesSelector(n, selector) {
			found = true
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return found
}

func normalizePrice(raw string) string {
	raw = strings.Join(strings.Fields(html.UnescapeString(raw)), " ")
	if price := belarusianKopPricePattern.FindStringSubmatch(raw); len(price) != 3 {
		if match := regexp.MustCompile(`(?i)(\d+[pр]\d{1,2}\s*коп\.?)`).FindStringSubmatch(raw); len(match) == 2 {
			raw = match[1]
		}
	}
	if parts := belarusianKopPricePattern.FindStringSubmatch(raw); len(parts) == 3 {
		fraction := parts[2]
		if len(fraction) == 1 {
			fraction += "0"
		}
		return parts[1] + "." + fraction
	}
	if !pricePattern.MatchString(raw) {
		return ""
	}
	raw = strings.TrimSpace(raw)
	raw = regexp.MustCompile(`(?i)\s*(?:BYN|Br|руб\.?|бел\.?\s*руб\.?)$`).ReplaceAllString(raw, "")
	return strings.ReplaceAll(strings.ReplaceAll(raw, " ", ""), ",", ".")
}

func pagePrice(body []byte, selector string) string {
	return normalizePrice(pagePriceText(body, selector))
}

func pagePriceText(body []byte, selector string) string {
	doc, err := htmlnode.Parse(bytes.NewReader(body))
	if err != nil {
		return ""
	}
	var meta, selected string
	var walk func(*htmlnode.Node)
	walk = func(n *htmlnode.Node) {
		if n.Type == htmlnode.ElementNode {
			if n.Data == "meta" && meta == "" {
				property := strings.ToLower(firstNonEmpty(attrValue(n, "property"), attrValue(n, "name")))
				if property == "product:price:amount" || property == "og:price" {
					meta = attrValue(n, "content")
				}
			}
			if selector != "" && matchesSelector(n, selector) {
				selected = firstNonEmpty(selected, attrValue(n, "content"), attrValue(n, "value"), textContent(n))
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return firstNonEmpty(meta, selected)
}

func decodeHTMLProduct(body []byte, cfg Config) ([]productData, error) {
	doc, err := htmlnode.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	root := doc
	if cfg.HTMLProductContainerSelector != "" {
		var findContainer func(*htmlnode.Node) *htmlnode.Node
		findContainer = func(n *htmlnode.Node) *htmlnode.Node {
			if n.Type == htmlnode.ElementNode && matchesSelector(n, cfg.HTMLProductContainerSelector) {
				return n
			}
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				if found := findContainer(child); found != nil {
					return found
				}
			}
			return nil
		}
		root = findContainer(doc)
		if root == nil {
			return nil, fmt.Errorf("HTML product container selector found no product container")
		}
	}
	var title, price, weightText, productID string
	stock := cfg.HTMLStockSelector == ""
	outOfStock := false
	var walk func(*htmlnode.Node)
	walk = func(n *htmlnode.Node) {
		if n.Type == htmlnode.ElementNode {
			if title == "" && matchesSelector(n, cfg.HTMLTitleSelector) {
				title = microdataValue(n)
			}
			if price == "" && matchesSelector(n, firstNonEmpty(cfg.HTMLPriceSelector, cfg.PriceSelector)) {
				price = firstNonEmpty(attrValue(n, "content"), attrValue(n, "value"), textContent(n))
			}
			if weightText == "" && cfg.HTMLWeightSelector != "" && matchesSelector(n, cfg.HTMLWeightSelector) {
				weightText = microdataValue(n)
			}
			if productID == "" && cfg.HTMLProductIDSelector != "" && matchesSelector(n, cfg.HTMLProductIDSelector) && strings.EqualFold(attrValue(n, cfg.HTMLProductIDNameAttribute), cfg.HTMLProductIDName) {
				productID = attrValue(n, cfg.HTMLProductIDValueAttribute)
			}
			if cfg.HTMLStockSelector != "" && matchesSelector(n, cfg.HTMLStockSelector) {
				stock = true
			}
			if cfg.HTMLOutOfStockSelector != "" && matchesSelector(n, cfg.HTMLOutOfStockSelector) {
				outOfStock = true
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	if strings.TrimSpace(title) == "" {
		return nil, fmt.Errorf("HTML title selector found no product title")
	}
	availability := "OutOfStock"
	if stock {
		availability = "InStock"
	}
	if outOfStock && !stock {
		availability = "OutOfStock"
	}
	offers := []offerData{{Price: price, Availability: availability}}
	if cfg.HTMLOptionSelector != "" {
		offers = htmlOptionOffers(root, cfg, productID, availability, title)
	}
	if len(offers) == 0 {
		if cfg.SkipHTMLOutOfStockWithoutOptions && outOfStock {
			return nil, errHTMLProductUnavailable
		}
		return nil, fmt.Errorf("HTML option selector found no valid priced options")
	}
	return []productData{{Name: strings.TrimSpace(title), WeightG: weightFromText(weightText), Offers: offers}}, nil
}

func htmlOptionOffers(doc *htmlnode.Node, cfg Config, productID, availability, productTitle string) []offerData {
	var offers []offerData
	var walkOptions func(*htmlnode.Node, *htmlnode.Node)
	walkOptions = func(n, parent *htmlnode.Node) {
		if n.Type == htmlnode.ElementNode {
			if n.Data == "option" {
				value := attrValue(n, cfg.HTMLOptionValueAttribute)
				quantity, err := decimal.NewFromString(strings.ReplaceAll(strings.TrimSpace(value), ",", "."))
				if err == nil && quantity.IsPositive() {
					basePrice, priceErr := decimal.NewFromString(normalizePrice(attrValue(parent, cfg.HTMLOptionPriceAttribute)))
					if priceErr == nil && basePrice.IsPositive() {
						label := strings.TrimSpace(html.UnescapeString(textContent(n)))
						if label == "" {
							label = strings.TrimSpace(html.UnescapeString(microdataValue(n)))
						}
						sku := value
						if productID != "" {
							sku = productID + "-" + value
						}
						weight := weightFromOption(label, quantity)
						grams := optionWeightGrams(weight, quantity)
						attrs := map[string]string{}
						if weight != nil {
							attrs["attribute_ves-gr"] = strconv.Itoa(*weight)
						}
						offers = append(offers, offerData{
							Name: strings.TrimSpace(productTitle + " " + label), SKU: sku, Price: basePrice.Mul(quantity).Round(2).String(),
							Currency: cfg.DefaultCurrency, Availability: availability, WeightG: weight, WeightGrams: grams, Attrs: attrs,
						})
					}
				}
				return
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walkOptions(child, n)
		}
	}
	matched := false
	var find func(*htmlnode.Node)
	find = func(n *htmlnode.Node) {
		if matched && productID == "" {
			return
		}
		if n.Type == htmlnode.ElementNode && matchesSelector(n, cfg.HTMLOptionSelector) {
			if cfg.HTMLOptionNameAttribute != "" && !strings.EqualFold(attrValue(n, cfg.HTMLOptionNameAttribute), cfg.HTMLOptionName) {
				return
			}
			if productID != "" && cfg.HTMLOptionIDAttribute != "" && attrValue(n, cfg.HTMLOptionIDAttribute) != productID {
				return
			}
			walkOptions(n, nil)
			matched = true
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			find(child)
		}
	}
	find(doc)
	return offers
}

// weightFromOption prefers the option label ("10 г.") and otherwise the numeric
// value used as quantity, which shops like Tea Mail treat as grams.
func weightFromOption(label string, quantity decimal.Decimal) *int {
	if weight := weightFromText(label); weight != nil {
		return weight
	}
	grams := int(quantity.IntPart())
	if !quantity.Equal(decimal.NewFromInt(int64(grams))) || grams <= 0 || grams > 10000 {
		return nil
	}
	return &grams
}

func optionWeightGrams(weight *int, quantity decimal.Decimal) *decimal.Decimal {
	if weight != nil && *weight > 0 {
		grams := decimal.NewFromInt(int64(*weight))
		return &grams
	}
	if quantity.IsPositive() && quantity.LessThanOrEqual(decimal.NewFromInt(10000)) {
		copy := quantity
		return &copy
	}
	return nil
}

func textContent(node *htmlnode.Node) string {
	var text strings.Builder
	var walk func(*htmlnode.Node)
	walk = func(current *htmlnode.Node) {
		if current.Type == htmlnode.TextNode {
			text.WriteString(current.Data)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return text.String()
}

var errHTMLProductUnavailable = errors.New("HTML product is explicitly out of stock and has no priced options")

var cardIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,80}$`)

func categoryCardIDs(doc *htmlnode.Node, cfg Config) []string {
	var ids []string
	var walk func(*htmlnode.Node)
	walk = func(n *htmlnode.Node) {
		if n.Type == htmlnode.ElementNode && matchesSelector(n, cfg.CategoryCardSelector) {
			id := attrValue(n, cfg.CategoryCardIDAttribute)
			if cardIDPattern.MatchString(id) {
				ids = append(ids, id)
			}
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return ids
}

func decodeCategoryCard(body []byte, cfg Config, id string) ([]productData, error) {
	doc, err := htmlnode.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	var card *htmlnode.Node
	var find func(*htmlnode.Node)
	find = func(n *htmlnode.Node) {
		if card != nil {
			return
		}
		if n.Type == htmlnode.ElementNode && matchesSelector(n, cfg.CategoryCardSelector) && attrValue(n, cfg.CategoryCardIDAttribute) == id {
			card = n
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			find(child)
		}
	}
	find(doc)
	if card == nil {
		return nil, fmt.Errorf("category card %q: %w", id, errCategoryCardDisappeared)
	}
	var title, price, weightText string
	stock := cfg.CategoryCardStockSelector == ""
	var walk func(*htmlnode.Node)
	walk = func(n *htmlnode.Node) {
		if n.Type == htmlnode.ElementNode {
			if title == "" && matchesSelector(n, cfg.CategoryCardTitleSelector) {
				title = microdataValue(n)
			}
			if price == "" && matchesSelector(n, cfg.CategoryCardPriceSelector) {
				price = microdataValue(n)
			}
			if weightText == "" && cfg.CategoryCardWeightSelector != "" && matchesSelector(n, cfg.CategoryCardWeightSelector) {
				weightText = microdataValue(n)
			}
			if cfg.CategoryCardStockSelector != "" && matchesSelector(n, cfg.CategoryCardStockSelector) {
				stock = true
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(card)
	if title == "" {
		return nil, fmt.Errorf("category card %q has no title", id)
	}
	availability := "OutOfStock"
	if stock {
		availability = "InStock"
	}
	return []productData{{Name: title, SKU: id, WeightG: weightFromText(firstNonEmpty(weightText, title)), Offers: []offerData{{Price: price, Availability: availability}}}}, nil
}

// The selector supports a tag, .class, #id, or tag.class.
func matchesSelector(n *htmlnode.Node, selector string) bool {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return false
	}
	if strings.HasPrefix(selector, "#") {
		return attrValue(n, "id") == selector[1:]
	}
	parts := strings.SplitN(selector, ".", 2)
	if parts[0] != "" && n.Data != parts[0] {
		return false
	}
	if len(parts) == 1 {
		return true
	}
	for _, class := range strings.Fields(attrValue(n, "class")) {
		if class == parts[1] {
			return true
		}
	}
	return false
}

func (a *Adapter) Fetch(ctx context.Context, ref entity.ProductRef) ([]entity.RawOffer, error) {
	a.diagnosticMu.Lock()
	a.diagnostic = nil
	a.diagnosticMu.Unlock()
	u, ok := a.sameSiteURL(ref.URL)
	cardID := ""
	if ok && a.config.CategoryCardSelector != "" && strings.HasPrefix(u.Fragment, "product-") {
		cardID = strings.TrimPrefix(u.Fragment, "product-")
		if !cardIDPattern.MatchString(cardID) || !a.isCategoryPage(u) {
			cardID = ""
		}
	}
	if !ok || (cardID == "" && (!a.isProductPath(u.Path) || a.isCategoryPage(u))) {
		return nil, crawlerr.ParseFailed(a.code, fmt.Errorf("product URL is outside configured source paths"))
	}
	var body []byte
	var err error
	usedSnapshot := false
	if cardID != "" && a.config.SnapshotCategoryCards {
		body, usedSnapshot = a.cachedCategoryBody(u.String())
	}
	if !usedSnapshot {
		body, err = a.fetch(ctx, u.String())
	}
	if err != nil {
		var statusErr *httpclient.HTTPStatusError
		if errors.As(err, &statusErr) {
			if statusErr.Status == http.StatusNotFound {
				if a.config.SkipNotFound {
					return nil, crawlerr.SkippedProduct(a.code, errors.New("product page no longer exists"))
				}
				return nil, crawlerr.ParseFailed(a.code, err)
			}
			if statusErr.Status == http.StatusForbidden || statusErr.Status == http.StatusUnauthorized {
				return nil, crawlerr.Blocked(a.code, err)
			}
		}
		return nil, crawlerr.Transient(a.code, err)
	}
	return a.parseBody(body, u, cardID)
}

// Replay parses a captured response body without network access or persistence.
// It uses the same source URL validation and extraction rules as Fetch.
func (a *Adapter) Replay(ref entity.ProductRef, response httpclient.DiagnosticResponse) ([]entity.RawOffer, error) {
	u, ok := a.sameSiteURL(ref.URL)
	cardID := ""
	if ok && a.config.CategoryCardSelector != "" && strings.HasPrefix(u.Fragment, "product-") {
		cardID = strings.TrimPrefix(u.Fragment, "product-")
		if !cardIDPattern.MatchString(cardID) || !a.isCategoryPage(u) {
			cardID = ""
		}
	}
	if !ok || (cardID == "" && (!a.isProductPath(u.Path) || a.isCategoryPage(u))) {
		return nil, crawlerr.ParseFailed(a.code, fmt.Errorf("product URL is outside configured source paths"))
	}
	if response.BodyTruncated || response.StorageTruncated || len(response.Body) > httpclient.MaxDiagnosticBodyBytes {
		return nil, crawlerr.ParseFailed(a.code, fmt.Errorf("truncated diagnostic response cannot be replayed as complete"))
	}
	return a.parseBody(response.Body, u, cardID)
}

func (a *Adapter) parseBody(body []byte, u *url.URL, cardID string) ([]entity.RawOffer, error) {
	var products []productData
	var parseErrors []string
	var err error
	if cardID != "" {
		products, err = decodeCategoryCard(body, a.config, cardID)
		if err != nil {
			if a.config.SkipMissingCard && errors.Is(err, errCategoryCardDisappeared) {
				return nil, crawlerr.SkippedProduct(a.code, err)
			}
			return nil, crawlerr.ParseFailed(a.code, err)
		}
	}
	var htmlProducts, otherProducts []productData
	for _, format := range a.config.DataFormats {
		if cardID != "" {
			break
		}
		var parsed []productData
		var parseErr error
		switch strings.ToLower(strings.TrimSpace(format)) {
		case "jsonld":
			parsed, parseErr = decodeProductsWithTypes(body, a.config.ScriptTypes)
		case "microdata":
			parsed, parseErr = decodeMicrodataProducts(body)
		case "html":
			parsed, parseErr = decodeHTMLProduct(body, a.config)
		}
		if parseErr != nil {
			if errors.Is(parseErr, errHTMLProductUnavailable) {
				return nil, crawlerr.SkippedProduct(a.code, parseErr)
			}
			parseErrors = append(parseErrors, parseErr.Error())
			continue
		}
		if strings.EqualFold(strings.TrimSpace(format), "html") {
			htmlProducts = append(htmlProducts, parsed...)
			continue
		}
		otherProducts = append(otherProducts, parsed...)
	}
	if cardID == "" {
		// HTML quantity options already have per-variant weight and price.
		// JSON-LD/microdata on the same page typically carry the cake title
		// weight plus whichever variant is currently selected.
		if len(htmlProducts) > 0 && a.config.HTMLOptionSelector != "" {
			products = htmlProducts
		} else {
			products = append(otherProducts, htmlProducts...)
		}
	}
	if len(products) == 0 {
		return nil, crawlerr.ParseFailed(a.code, fmt.Errorf("no Product data found: %s", strings.Join(parseErrors, "; ")))
	}
	fallbackPrice := pagePriceText(body, firstNonEmpty(a.config.HTMLPriceSelector, a.config.PriceSelector))
	var offers []entity.RawOffer
	seenOffers := map[string]struct{}{}
	for _, product := range products {
		if (len(a.config.IncludeNameKeywords) == 0 && !a.categoryAllowed(product.Category)) || !a.nameAllowed(product.Name) {
			continue
		}
		if len(product.Offers) == 0 && fallbackPrice != "" {
			product.Offers = []offerData{{Price: fallbackPrice}}
		}
		if len(product.Offers) == 0 {
			continue
		}
		skuCounts := make(map[string]int, len(product.Offers))
		for _, offer := range product.Offers {
			baseSKU := firstNonEmpty(strings.TrimSpace(offer.SKU), strings.TrimSpace(product.SKU), u.Path)
			skuCounts[baseSKU]++
		}
		seenProductOffers := make(map[string]struct{}, len(product.Offers))
		for offerIndex, offer := range product.Offers {
			priceText := firstNonEmpty(offer.Price, fallbackPrice)
			if priceText == "" {
				continue
			}
			currency := strings.ToUpper(strings.TrimSpace(offer.Currency))
			if currency == "" {
				currency = strings.ToUpper(strings.TrimSpace(a.config.DefaultCurrency))
			}
			if currency == "" {
				return nil, crawlerr.ParseFailed(a.code, fmt.Errorf("missing currency for %s", u))
			}
			normalizedCurrency, err := pricing.NormalizeCurrency(currency)
			if err != nil {
				continue
			}
			wantCurrency := strings.ToUpper(strings.TrimSpace(a.config.DefaultCurrency))
			if wantCurrency == "" {
				wantCurrency = "BYN"
			}
			wantCurrency, _ = pricing.NormalizeCurrency(wantCurrency)
			if normalizedCurrency != wantCurrency {
				continue
			}
			currency = normalizedCurrency
			normalized := normalizePrice(priceText)
			if normalized == "" {
				return nil, crawlerr.ParseFailed(a.code, fmt.Errorf("unrecognized price for %s", u))
			}
			price, err := decimal.NewFromString(normalized)
			if err != nil {
				return nil, crawlerr.ParseFailed(a.code, fmt.Errorf("invalid price for %s: %w", u, err))
			}
			if !price.IsPositive() {
				continue
			}
			weight := offer.WeightG
			weightGrams := offer.WeightGrams
			if weight == nil {
				// Use the offer's own name first so a variant label is not
				// overwritten by a cake/pack weight that only appears in the title.
				weight = weightFromURLOrName(offer.URL, firstNonEmpty(offer.Name, product.Name))
			}
			if weight == nil && offer.Name == "" {
				weight = product.WeightG
			}
			if weightGrams == nil && weight != nil {
				grams := decimal.NewFromInt(int64(*weight))
				weightGrams = &grams
			}
			inStock := strings.Contains(strings.ToLower(offer.Availability), "instock")
			imageURL := resolveImageURL(firstNonEmpty(product.Image, offer.Image), u)
			baseSKU := firstNonEmpty(strings.TrimSpace(offer.SKU), strings.TrimSpace(product.SKU), u.Path)
			weightKey := ""
			if weight != nil {
				weightKey = strconv.Itoa(*weight)
			}
			dedupeKey := baseSKU + "|" + firstNonEmpty(offer.Name, product.Name) + "|" + price.String() + "|" + weightKey
			if _, exists := seenProductOffers[dedupeKey]; exists {
				continue
			}
			seenProductOffers[dedupeKey] = struct{}{}
			sku := baseSKU
			if skuCounts[baseSKU] > 1 {
				sku = fmt.Sprintf("%s-variant-%d", baseSKU, offerIndex+1)
			}
			key := sku + "|" + firstNonEmpty(offer.Name, product.Name) + "|" + price.String()
			if _, exists := seenOffers[key]; exists {
				continue
			}
			seenOffers[key] = struct{}{}
			attrs := map[string]string{"adapter": Kind, "schema_type": "Product"}
			for key, value := range product.Attrs {
				attrs[key] = value
			}
			for key, value := range offer.Attrs {
				attrs[key] = value
			}
			for key, attribute := range a.config.AttrMap {
				value := strings.TrimSpace(attrs[key])
				if value == "" {
					for existing, candidate := range attrs {
						if strings.EqualFold(existing, key) {
							value = strings.TrimSpace(candidate)
							break
						}
					}
				}
				if value != "" {
					attrs[attribute] = value
				}
			}
			kindHint := a.collectKind(u.Path, product.Category, firstNonEmpty(offer.Name, product.Name))
			if len(a.config.Collect) > 0 && kindHint == "" {
				attrs["_collect_excluded"] = "true"
			}
			if kindHint != "" {
				attrs["product_kind"] = kindHint
			}
			for key, value := range variantAttributesFromURL(offer.URL) {
				attrs[key] = value
			}
			offerURL := u.String()
			if resolved := resolveOfferURL(offer.URL, u); resolved != "" {
				offerURL = resolved
			}
			offers = append(offers, entity.RawOffer{
				Source: a.code, URL: offerURL, SourceSKU: sku, Title: firstNonEmpty(offer.Name, product.Name),
				Price: price, Currency: currency, WeightG: weight, WeightGrams: weightGrams, UnitHint: unitFor(weight),
				InStock: inStock, ImageURL: imageURL, Brand: product.Brand,
				ObservedAt: time.Now().UTC(),
				Attrs:      attrs,
				Kind:       kindHint,
			})
		}
	}
	if len(offers) == 0 {
		return nil, crawlerr.SkippedProduct(a.code, fmt.Errorf("product filtered or has no positive priced offer"))
	}
	return offers, nil
}

const maxCategorySnapshotBytes = 32 << 20

func categoryPageKey(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.Fragment = ""
	u.RawFragment = ""
	return u.String()
}

func (a *Adapter) cacheCategoryBody(raw string, body []byte) {
	if len(body) > maxCategorySnapshotBytes {
		return
	}
	key := categoryPageKey(raw)
	a.categoryMu.Lock()
	defer a.categoryMu.Unlock()
	if _, exists := a.categoryBody[key]; exists {
		return
	}
	var used int
	for _, cached := range a.categoryBody {
		used += len(cached)
	}
	if used+len(body) > maxCategorySnapshotBytes {
		return
	}
	a.categoryBody[key] = append([]byte(nil), body...)
}

func (a *Adapter) cachedCategoryBody(raw string) ([]byte, bool) {
	a.categoryMu.RLock()
	defer a.categoryMu.RUnlock()
	body, ok := a.categoryBody[categoryPageKey(raw)]
	return body, ok
}

func (a *Adapter) isCategoryPage(u *url.URL) bool {
	for _, raw := range a.config.CategoryURLs {
		category, err := url.Parse(raw)
		if err == nil && strings.TrimRight(category.Path, "/") == strings.TrimRight(u.Path, "/") && category.RawQuery == u.RawQuery {
			return true
		}
	}
	return false
}

func (a *Adapter) fetch(ctx context.Context, raw string) ([]byte, error) {
	u, ok := a.sameSiteURL(raw)
	if !ok {
		return nil, fmt.Errorf("refusing off-origin source URL %q", raw)
	}
	if err := a.waitRate(ctx); err != nil {
		return nil, err
	}
	headers := make(http.Header)
	if a.config.UserAgent != "" {
		headers.Set("User-Agent", a.config.UserAgent)
	}
	if a.config.AcceptLanguage != "" {
		headers.Set("Accept-Language", a.config.AcceptLanguage)
	}
	body, response, err := a.cli.GetWithRetryHeadersLimit(ctx, u.String(), headers, a.config.MaxResponseBytes)
	if response != nil {
		bodyHash := httpclient.CapturedBodyHash(body)
		bytesRead := int64(len(body))
		var tooLarge *httpclient.BodyTooLargeError
		if errors.As(err, &tooLarge) {
			bodyHash = tooLarge.BodyHash
			bytesRead = tooLarge.BytesRead
		}
		captured := httpclient.DiagnosticResponse{
			URL: sanitizeDiagnosticURL(u), StatusCode: response.StatusCode,
			ContentType: response.Header.Get("Content-Type"), ReceivedAt: time.Now().UTC(),
			BodyHash: bodyHash, BytesRead: bytesRead,
			BodyTruncated: httpclient.IsBodyTruncated(err),
		}
		if int64(len(body)) > httpclient.MaxDiagnosticBodyBytes {
			captured.Body = append([]byte(nil), body[:httpclient.MaxDiagnosticBodyBytes]...)
			captured.StorageTruncated = true
		} else {
			captured.Body = append([]byte(nil), body...)
		}
		a.diagnosticMu.Lock()
		a.diagnostic = &captured
		a.diagnosticMu.Unlock()
	}
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, httpclient.StatusError(response.StatusCode, raw)
	}
	if response.Request != nil && response.Request.URL != nil && !sameOrigin(a.base, response.Request.URL) {
		return nil, fmt.Errorf("refusing cross-origin redirect from %s", raw)
	}
	return body, nil
}

func sanitizeDiagnosticURL(u *url.URL) string {
	if u == nil {
		return ""
	}
	clean := *u
	clean.User = nil
	clean.RawQuery = ""
	clean.ForceQuery = false
	clean.Fragment = ""
	return clean.String()
}

func (a *Adapter) waitRate(ctx context.Context) error {
	gap := time.Duration(float64(time.Second) / a.rateLimit)
	for {
		a.limiterMu.Lock()
		wait := gap - time.Since(a.lastRequest)
		if wait <= 0 || a.lastRequest.IsZero() {
			a.lastRequest = time.Now()
			a.limiterMu.Unlock()
			return nil
		}
		a.limiterMu.Unlock()
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (a *Adapter) sameSiteURL(raw string) (*url.URL, bool) {
	u, err := url.Parse(upgradePublicURL(strings.TrimSpace(raw)))
	if err != nil || u.User != nil || !sameOrigin(a.base, u) {
		return nil, false
	}
	return u, true
}

func sameOrigin(a, b *url.URL) bool {
	return a != nil && b != nil && (a.Scheme == "http" || a.Scheme == "https") && (b.Scheme == "http" || b.Scheme == "https") && strings.EqualFold(a.Host, b.Host)
}

// Public shop URLs are fetched over TLS even when an old sitemap lists HTTP.
// Loopback URLs stay HTTP so local test servers remain usable.
func upgradePublicURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "http" || u.Hostname() == "localhost" || net.ParseIP(u.Hostname()) != nil {
		return raw
	}
	u.Scheme = "https"
	if u.Port() == "80" {
		u.Host = u.Hostname()
	}
	return u.String()
}

func (a *Adapter) isProductPath(path string) bool {
	for _, fragment := range a.config.ExcludePathContains {
		if strings.Contains(path, fragment) {
			return false
		}
	}
	for _, expression := range a.config.ExcludePathRegex {
		if regexp.MustCompile(expression).MatchString(path) {
			return false
		}
	}
	for _, excluded := range a.config.ExcludePaths {
		if strings.HasPrefix(path, excluded) {
			return false
		}
	}
	if len(a.config.IncludePaths) > 0 {
		included := false
		for _, prefix := range a.config.IncludePaths {
			if strings.HasPrefix(path, prefix) {
				included = true
				break
			}
		}
		if !included {
			return false
		}
	}
	if len(a.config.IncludePathContains) > 0 {
		included := false
		for _, fragment := range a.config.IncludePathContains {
			if strings.Contains(path, fragment) {
				included = true
				break
			}
		}
		if !included {
			return false
		}
	}
	if len(a.config.IncludePathRegex) > 0 {
		included := false
		for _, expression := range a.config.IncludePathRegex {
			if regexp.MustCompile(expression).MatchString(path) {
				included = true
				break
			}
		}
		if !included {
			return false
		}
	}
	if len(a.config.ProductPathPrefixes) == 0 {
		return true
	}
	for _, prefix := range a.config.ProductPathPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func (a *Adapter) nameAllowed(name string) bool {
	name = strings.ToLower(html.UnescapeString(name))
	for _, word := range a.config.ExcludeNameKeywords {
		if strings.Contains(name, strings.ToLower(word)) {
			return false
		}
	}
	if len(a.config.IncludeNameKeywords) == 0 {
		return true
	}
	for _, word := range a.config.IncludeNameKeywords {
		if strings.Contains(name, strings.ToLower(word)) {
			return true
		}
	}
	return false
}

func (a *Adapter) categoryAllowed(category string) bool {
	category = strings.ToLower(html.UnescapeString(category))
	for _, hint := range a.config.ExcludeCategoryHints {
		if strings.Contains(category, strings.ToLower(hint)) {
			return false
		}
	}
	if len(a.config.IncludeCategoryHints) == 0 {
		return true
	}
	for _, hint := range a.config.IncludeCategoryHints {
		if strings.Contains(category, strings.ToLower(hint)) {
			return true
		}
	}
	return false
}

type productData struct {
	Name     string
	SKU      string
	Image    string
	Brand    string
	Category string
	WeightG  *int
	Offers   []offerData
	Attrs    map[string]string
}

type offerData struct {
	Name         string            `json:"name"`
	SKU          string            `json:"sku"`
	Image        string            `json:"image"`
	URL          string            `json:"url"`
	Price        string            `json:"price"`
	Currency     string            `json:"priceCurrency"`
	Availability string            `json:"availability"`
	WeightG      *int              `json:"-"`
	WeightGrams  *decimal.Decimal  `json:"-"`
	Attrs        map[string]string `json:"-"`
}

func decodeProducts(body []byte) ([]productData, error) {
	return decodeProductsWithTypes(body, []string{"application/ld+json"})
}

func decodeProductsWithTypes(body []byte, scriptTypes []string) ([]productData, error) {
	allowedTypes := make(map[string]struct{}, len(scriptTypes))
	for _, scriptType := range scriptTypes {
		allowedTypes[strings.ToLower(strings.TrimSpace(scriptType))] = struct{}{}
	}
	matches := jsonScript.FindAllSubmatch(body, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("product page has no configured JSON scripts")
	}
	var products []productData
	for _, match := range matches {
		if _, ok := allowedTypes[strings.ToLower(strings.TrimSpace(string(match[1])))]; !ok {
			continue
		}
		var root any
		if err := json.Unmarshal([]byte(strings.TrimSpace(string(match[2]))), &root); err != nil {
			continue
		}
		products = append(products, productsFromNode(root)...)
	}
	if len(products) == 0 {
		return nil, fmt.Errorf("configured JSON scripts have no Product node")
	}
	return products, nil
}

func decodeMicrodataProducts(body []byte) ([]productData, error) {
	doc, err := htmlnode.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse product HTML: %w", err)
	}
	var products []productData
	var walk func(*htmlnode.Node)
	walk = func(node *htmlnode.Node) {
		if node.Type == htmlnode.ElementNode && hasSchemaItemType(node, "product") {
			product := parseMicrodataProduct(node)
			if len(product.Offers) > 0 || product.Name != "" {
				products = append(products, product)
			}
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	if len(products) == 0 {
		return nil, fmt.Errorf("HTML has no Schema.org Product microdata")
	}
	return products, nil
}

func parseMicrodataProduct(root *htmlnode.Node) productData {
	var product productData
	product.Attrs = map[string]string{}
	var walk func(*htmlnode.Node)
	walk = func(node *htmlnode.Node) {
		if node != root && node.Type == htmlnode.ElementNode && hasAttr(node, "itemscope") {
			if hasItemProp(node, "offers") && (hasSchemaItemType(node, "offer") || hasSchemaItemType(node, "aggregateoffer")) {
				product.Offers = append(product.Offers, parseMicrodataOffers(node)...)
				return
			}
			if hasItemProp(node, "brand") {
				if product.Brand == "" {
					product.Brand = microdataProperty(node, "name")
				}
				return
			}
			return
		}
		if node.Type == htmlnode.ElementNode {
			if prop := strings.TrimSpace(attrValue(node, "itemprop")); prop != "" {
				if _, reserved := map[string]bool{"name": true, "sku": true, "productID": true, "image": true, "category": true, "weight": true, "offers": true, "brand": true}[prop]; !reserved {
					if value := microdataValue(node); value != "" {
						product.Attrs[prop] = value
					}
				}
			}
			switch {
			case product.Name == "" && hasItemProp(node, "name"):
				product.Name = microdataValue(node)
			case product.SKU == "" && (hasItemProp(node, "sku") || hasItemProp(node, "productID")):
				product.SKU = microdataValue(node)
			case product.Image == "" && hasItemProp(node, "image"):
				product.Image = microdataValue(node)
			case product.Category == "" && hasItemProp(node, "category"):
				product.Category = microdataValue(node)
			case product.WeightG == nil && hasItemProp(node, "weight"):
				product.WeightG = weightFromText(microdataValue(node))
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return product
}

func parseMicrodataOffers(root *htmlnode.Node) []offerData {
	if !hasSchemaItemType(root, "aggregateoffer") {
		return []offerData{parseMicrodataOffer(root)}
	}
	var offers []offerData
	var walk func(*htmlnode.Node)
	walk = func(node *htmlnode.Node) {
		if node != root && node.Type == htmlnode.ElementNode && hasAttr(node, "itemscope") {
			if hasItemProp(node, "offers") && hasSchemaItemType(node, "offer") {
				offers = append(offers, parseMicrodataOffer(node))
			}
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	if len(offers) > 0 {
		return offers
	}
	return []offerData{{Price: firstNonEmpty(microdataProperty(root, "lowPrice"), microdataProperty(root, "highPrice")), Currency: microdataProperty(root, "priceCurrency")}}
}

func parseMicrodataOffer(root *htmlnode.Node) offerData {
	var offer offerData
	offer.Attrs = map[string]string{}
	var walk func(*htmlnode.Node)
	walk = func(node *htmlnode.Node) {
		if node != root && node.Type == htmlnode.ElementNode && hasAttr(node, "itemscope") {
			return
		}
		if node.Type == htmlnode.ElementNode {
			if prop := strings.TrimSpace(attrValue(node, "itemprop")); prop != "" {
				if _, reserved := map[string]bool{"name": true, "sku": true, "serialNumber": true, "image": true, "price": true, "priceCurrency": true, "availability": true}[prop]; !reserved {
					if value := microdataValue(node); value != "" {
						offer.Attrs[prop] = value
					}
				}
			}
			switch {
			case offer.Name == "" && hasItemProp(node, "name"):
				offer.Name = microdataValue(node)
			case offer.SKU == "" && (hasItemProp(node, "sku") || hasItemProp(node, "serialNumber")):
				offer.SKU = microdataValue(node)
			case offer.Image == "" && hasItemProp(node, "image"):
				offer.Image = microdataValue(node)
			case offer.Price == "" && hasItemProp(node, "price"):
				offer.Price = microdataValue(node)
			case offer.Currency == "" && hasItemProp(node, "priceCurrency"):
				offer.Currency = microdataValue(node)
			case offer.Availability == "" && hasItemProp(node, "availability"):
				offer.Availability = microdataValue(node)
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return offer
}

func microdataProperty(root *htmlnode.Node, property string) string {
	var value string
	var walk func(*htmlnode.Node)
	walk = func(node *htmlnode.Node) {
		if value != "" {
			return
		}
		if node.Type == htmlnode.ElementNode && hasItemProp(node, property) {
			value = microdataValue(node)
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return value
}

func hasSchemaItemType(node *htmlnode.Node, schemaType string) bool {
	typeValue := strings.ToLower(strings.TrimSpace(attrValue(node, "itemtype")))
	return strings.Contains(typeValue, "schema.org/"+strings.ToLower(schemaType))
}

func hasAttr(node *htmlnode.Node, name string) bool {
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, name) {
			return true
		}
	}
	return false
}

func hasItemProp(node *htmlnode.Node, property string) bool {
	for _, item := range strings.Fields(attrValue(node, "itemprop")) {
		if strings.EqualFold(item, property) {
			return true
		}
	}
	return false
}

func attrValue(node *htmlnode.Node, name string) string {
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, name) {
			return attr.Val
		}
	}
	return ""
}

func microdataValue(node *htmlnode.Node) string {
	for _, attr := range []string{"content", "href", "src", "value", "alt"} {
		if value := strings.TrimSpace(attrValue(node, attr)); value != "" {
			return value
		}
	}
	var text strings.Builder
	var walk func(*htmlnode.Node)
	walk = func(current *htmlnode.Node) {
		if current.Type == htmlnode.TextNode {
			text.WriteString(current.Data)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return strings.TrimSpace(text.String())
}

func productsFromNode(node any) []productData {
	switch value := node.(type) {
	case []any:
		var out []productData
		for _, child := range value {
			out = append(out, productsFromNode(child)...)
		}
		return out
	case map[string]any:
		if graph, ok := value["@graph"]; ok {
			return productsFromNode(graph)
		}
		if !hasProductType(value["@type"]) {
			return nil
		}
		product := productData{
			Name: stringValue(value["name"]), SKU: stringValue(value["sku"]),
			Image: imageValue(value["image"]), Brand: brandValue(value["brand"]),
			Category: categoryValue(value["category"]), WeightG: schemaWeight(value["weight"]), Attrs: schemaAttributes(value),
		}
		product.Offers = decodeOfferNodes(value["offers"])
		if variants, ok := value["hasVariant"]; ok {
			variantProducts := productsFromNode(variants)
			if len(variantProducts) > 0 {
				for i, variant := range variantProducts {
					if variant.Name == "" {
						variant.Name = product.Name
					}
					if variant.Image == "" {
						variant.Image = product.Image
					}
					if variant.Brand == "" {
						variant.Brand = product.Brand
					}
					if variant.Category == "" {
						variant.Category = product.Category
					}
					if variant.Attrs == nil {
						variant.Attrs = product.Attrs
					}
					if variant.WeightG == nil {
						variant.WeightG = product.WeightG
					}
					if variant.SKU == "" && product.SKU != "" {
						variant.SKU = fmt.Sprintf("%s-%d", product.SKU, i+1)
					}
					variantProducts[i] = variant
				}
				return variantProducts
			}
		}
		return []productData{product}
	}
	return nil
}

func schemaAttributes(value map[string]any) map[string]string {
	out := map[string]string{}
	// description is reserved for product body text but still useful for attr_rule extract.
	reserved := map[string]bool{"@context": true, "@type": true, "name": true, "sku": true, "image": true, "brand": true, "category": true, "weight": true, "offers": true, "hasVariant": true, "url": true, "mpn": true, "gtin": true}
	for key, raw := range value {
		if reserved[key] {
			continue
		}
		if text := schemaPrimitive(raw); text != "" {
			out[key] = text
		}
	}
	return out
}

func schemaPrimitive(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return string(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	case []any:
		parts := []string{}
		for _, item := range v {
			if text := schemaPrimitive(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, ",")
	case map[string]any:
		if name, ok := v["name"].(string); ok {
			return strings.TrimSpace(name)
		}
	}
	return ""
}

func (a *Adapter) collectKind(path, category, name string) string {
	if len(a.config.Collect) == 0 {
		return ""
	}
	path, category, name = strings.ToLower(path), strings.ToLower(category), strings.ToLower(name)
	for _, rule := range a.config.Collect {
		if rule.Enabled != nil && !*rule.Enabled {
			continue
		}
		categoryMatch := len(rule.IncludeCategories) == 0
		for _, v := range rule.IncludeCategories {
			if strings.Contains(path, strings.ToLower(v)) || strings.Contains(category, strings.ToLower(v)) {
				categoryMatch = true
				break
			}
		}
		keywordMatch := len(rule.IncludeKeywords) == 0
		for _, v := range rule.IncludeKeywords {
			if strings.Contains(name, strings.ToLower(v)) {
				keywordMatch = true
				break
			}
		}
		matched := categoryMatch && keywordMatch
		for _, v := range rule.ExcludeKeywords {
			if strings.Contains(name, strings.ToLower(v)) {
				matched = false
				break
			}
		}
		if matched {
			return rule.Kind
		}
	}
	return ""
}

func decodeOfferNodes(raw any) []offerData {
	var out []offerData
	var appendOffer func(any)
	appendOffer = func(node any) {
		switch value := node.(type) {
		case []any:
			for _, child := range value {
				appendOffer(child)
			}
		case map[string]any:
			if nested, ok := value["offers"]; ok {
				before := len(out)
				appendOffer(nested)
				if len(out) > before {
					for i := before; i < len(out); i++ {
						if out[i].Currency == "" {
							out[i].Currency = stringValue(value["priceCurrency"])
						}
					}
					return
				}
			}
			price := stringValue(value["price"])
			if price == "" {
				low, high := stringValue(value["lowPrice"]), stringValue(value["highPrice"])
				price = firstNonEmpty(low, high)
			}
			if price == "" {
				return
			}
			offerURL := stringValue(value["url"])
			var weight *int
			if specification, ok := value["priceSpecification"].(map[string]any); ok {
				if quantity, exists := specification["referenceQuantity"]; exists {
					weight = schemaWeight(quantity)
				}
			}
			if weight == nil {
				weight = weightFromAdditionalProperty(value["additionalProperty"])
			}
			if weight == nil {
				weight = weightFromURLOrName(offerURL, stringValue(value["name"]))
			}
			out = append(out, offerData{Name: stringValue(value["name"]), SKU: stringValue(value["sku"]), Image: imageValue(value["image"]), URL: offerURL, Price: price, Currency: stringValue(value["priceCurrency"]), Availability: stringValue(value["availability"]), WeightG: weight})
		}
	}
	appendOffer(raw)
	return out
}

func hasProductType(raw any) bool {
	switch value := raw.(type) {
	case string:
		return strings.EqualFold(value, "Product") || strings.EqualFold(value, "ProductGroup")
	case []any:
		for _, item := range value {
			if hasProductType(item) {
				return true
			}
		}
	}
	return false
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case map[string]any:
		if nested, ok := v["name"]; ok {
			return stringValue(nested)
		}
	}
	return ""
}

func imageValue(raw any) string {
	if list, ok := raw.([]any); ok && len(list) > 0 {
		return imageValue(list[0])
	}
	if object, ok := raw.(map[string]any); ok {
		return firstNonEmpty(stringValue(object["url"]), stringValue(object["contentUrl"]))
	}
	return stringValue(raw)
}

func brandValue(raw any) string { return stringValue(raw) }

func categoryValue(raw any) string {
	if list, ok := raw.([]any); ok {
		parts := make([]string, 0, len(list))
		for _, item := range list {
			parts = append(parts, stringValue(item))
		}
		return strings.Join(parts, " ")
	}
	return stringValue(raw)
}

func schemaWeight(raw any) *int {
	var value, unit string
	if object, ok := raw.(map[string]any); ok {
		value = stringValue(object["value"])
		unit = strings.ToLower(stringValue(object["unitCode"]) + " " + stringValue(object["unitText"]))
	} else {
		value = stringValue(raw)
	}
	amount, err := decimal.NewFromString(strings.ReplaceAll(value, ",", "."))
	if err != nil || !amount.IsPositive() {
		return nil
	}
	if strings.Contains(unit, "KGM") || strings.Contains(unit, "kg") || strings.Contains(unit, "кг") {
		amount = amount.Mul(decimal.NewFromInt(1000))
	}
	grams := int(amount.IntPart())
	if grams <= 0 || grams > 10000 {
		return nil
	}
	return &grams
}

func weightFromText(value string) *int {
	for _, match := range weightPattern.FindAllStringSubmatch(html.UnescapeString(value), -1) {
		if len(match) < 4 {
			continue
		}
		amount, err := strconv.Atoi(match[2])
		if err != nil || amount <= 0 {
			continue
		}
		unit := strings.ToLower(match[3])
		if (unit == "г" || unit == "g" || strings.HasPrefix(unit, "гр")) && amount >= 1900 && amount <= 2099 {
			continue
		}
		grams := amount
		if match[1] != "" {
			count, err := strconv.Atoi(match[1])
			if err != nil || count <= 0 {
				continue
			}
			grams *= count
		}
		if strings.Contains(unit, "кг") || unit == "kg" {
			grams *= 1000
		}
		if grams <= 0 || grams > 10000 {
			continue
		}
		return &grams
	}
	return nil
}

var weightAttributeKeyPattern = regexp.MustCompile(`(?i)(?:^|_|-)(ves-gr|ves_gr|weight|ves|вес|gramm|grams|gr)(?:$|_|-)`)
var trailingAmountPattern = regexp.MustCompile(`(?i)[-–—]\s*(\d{1,4})\s*(кг|kg|грамм(?:ов|а)?|гр\.?|г|g)?\s*$`)

func weightFromURLOrName(rawURL, name string) *int {
	if weight := weightFromAttributeURL(rawURL); weight != nil {
		return weight
	}
	if weight := weightFromText(name); weight != nil {
		return weight
	}
	if weight := weightFromPath(rawURL); weight != nil {
		return weight
	}
	if weightAttributeURL(rawURL) {
		return weightFromTrailingAmount(name)
	}
	return nil
}

func weightFromAdditionalProperty(raw any) *int {
	switch value := raw.(type) {
	case []any:
		for _, item := range value {
			if weight := weightFromAdditionalProperty(item); weight != nil {
				return weight
			}
		}
	case map[string]any:
		name := strings.ToLower(stringValue(value["name"]))
		text := firstNonEmpty(stringValue(value["value"]), stringValue(value["unitText"]))
		if strings.Contains(name, "вес") || strings.Contains(name, "weight") || strings.Contains(name, "упаков") || strings.Contains(name, "грамм") {
			if weight := weightFromText(text); weight != nil {
				return weight
			}
		}
	}
	return nil
}

var pathBarePackWeight = regexp.MustCompile(`(?i)[_-](\d{3,4})/?$`)

func weightFromPath(rawURL string) *int {
	u, err := url.Parse(rawURL)
	if err != nil || u == nil {
		return nil
	}
	if weight := weightFromText(u.Path); weight != nil {
		return weight
	}
	match := pathBarePackWeight.FindStringSubmatch(strings.TrimRight(u.Path, "/"))
	if len(match) != 2 {
		return nil
	}
	grams, err := strconv.Atoi(match[1])
	if err != nil || grams < 50 || grams > 2000 {
		return nil
	}
	return &grams
}

func weightFromAttributeURL(raw string) *int {
	u, err := url.Parse(raw)
	if err != nil || u == nil {
		return nil
	}
	for key, values := range u.Query() {
		if len(values) == 0 || !isWeightAttributeKey(key) {
			continue
		}
		if weight := parseNumericGrams(values[0]); weight != nil {
			return weight
		}
	}
	return nil
}

func weightAttributeURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u == nil {
		return false
	}
	for key := range u.Query() {
		if isWeightAttributeKey(key) {
			return true
		}
	}
	return false
}

func isWeightAttributeKey(key string) bool {
	return weightAttributeKeyPattern.MatchString(strings.ToLower(strings.TrimSpace(key)))
}

func variantAttributesFromURL(raw string) map[string]string {
	u, err := url.Parse(raw)
	if err != nil || u == nil || u.RawQuery == "" {
		return nil
	}
	out := map[string]string{}
	for key, values := range u.Query() {
		if len(values) == 0 {
			continue
		}
		kl := strings.ToLower(strings.TrimSpace(key))
		if !(strings.HasPrefix(kl, "attribute_") || strings.HasPrefix(kl, "pa_") || isWeightAttributeKey(kl)) {
			continue
		}
		if value := strings.TrimSpace(values[0]); value != "" {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func weightFromTrailingAmount(name string) *int {
	match := trailingAmountPattern.FindStringSubmatch(html.UnescapeString(name))
	if len(match) < 2 {
		return nil
	}
	return parseNumericGrams(strings.TrimSpace(match[1] + " " + strings.TrimSpace(match[2])))
}

func parseNumericGrams(raw string) *int {
	raw = strings.TrimSpace(html.UnescapeString(raw))
	if raw == "" {
		return nil
	}
	if weight := weightFromText(raw); weight != nil {
		return weight
	}
	if weight := weightFromText(raw + " g"); weight != nil {
		return weight
	}
	return nil
}

func resolveOfferURL(raw string, pageURL *url.URL) string {
	if strings.TrimSpace(raw) == "" || pageURL == nil {
		return ""
	}
	ref, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return pageURL.ResolveReference(ref).String()
}

func unitFor(weight *int) entity.UnitHint {
	if weight != nil {
		return entity.UnitG
	}
	return entity.UnitUnknown
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func resolveImageURL(raw string, pageURL *url.URL) string {
	image, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || raw == "" {
		return ""
	}
	if pageURL == nil {
		return image.String()
	}
	return pageURL.ResolveReference(image).String()
}
