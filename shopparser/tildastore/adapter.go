package tildastore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	entity "github.com/ibednov/go-lepsios/shopparser"
	crawlerr "github.com/ibednov/go-lepsios/shopparser/errors"
	"github.com/ibednov/go-lepsios/shopparser/httpclient"
	"github.com/ibednov/go-lepsios/shopparser/pricing"
	"github.com/shopspring/decimal"
)

const Kind = "tilda_store"

const (
	defaultAPIBase = "https://store.tildacdn.com"
	fallbackAPI    = "https://store.tildaapi.com"
	pageSize       = 100
	maxSlices      = 50
)

type Config struct {
	Version             int      `json:"version"`
	StorepartUIDs       []string `json:"storepart_uids"`
	DefaultCurrency     string   `json:"currency"`
	APIBase             string   `json:"api_base"`
	IncludeNameKeywords []string `json:"include_name_keywords"`
	ExcludeNameKeywords []string `json:"exclude_name_keywords"`
}

type Adapter struct {
	cli          *httpclient.Client
	code         string
	base         *url.URL
	config       Config
	diagnosticMu sync.Mutex
	diagnostic   *httpclient.DiagnosticResponse
	cacheMu      sync.RWMutex
	byUID        map[string]storeProduct
	byURL        map[string]storeProduct
}

func New(cli *httpclient.Client, code, baseURL string, raw json.RawMessage) (*Adapter, error) {
	cfg, err := decodeConfig(baseURL, raw)
	if err != nil {
		return nil, err
	}
	base, _ := url.Parse(strings.TrimRight(baseURL, "/"))
	return &Adapter{
		cli: cli, code: code, base: base, config: cfg,
		byUID: make(map[string]storeProduct), byURL: make(map[string]storeProduct),
	}, nil
}

func ValidateConfig(baseURL string, raw json.RawMessage) error {
	_, err := decodeConfig(baseURL, raw)
	return err
}

func decodeConfig(baseURL string, raw json.RawMessage) (Config, error) {
	base, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || base.Hostname() == "" || (base.Scheme != "https" && base.Scheme != "http") || base.User != nil {
		return Config{}, fmt.Errorf("%s requires an http(s) base_url without credentials", Kind)
	}
	var cfg Config
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return Config{}, fmt.Errorf("decode %s config: %w", Kind, err)
		}
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.Version != 1 {
		return Config{}, fmt.Errorf("unsupported %s config version %d", Kind, cfg.Version)
	}
	if cfg.DefaultCurrency == "" {
		cfg.DefaultCurrency = "RUB"
	}
	if _, err := pricing.NormalizeCurrency(cfg.DefaultCurrency); err != nil {
		return Config{}, fmt.Errorf("%s currency %q is not supported", Kind, cfg.DefaultCurrency)
	}
	if len(cfg.StorepartUIDs) == 0 {
		return Config{}, fmt.Errorf("%s requires storepart_uids from the Tilda store block", Kind)
	}
	if len(cfg.StorepartUIDs) > 20 {
		return Config{}, fmt.Errorf("%s supports at most 20 storepart_uids", Kind)
	}
	seen := map[string]struct{}{}
	for i, uid := range cfg.StorepartUIDs {
		uid = strings.TrimSpace(uid)
		if !storepartUID.MatchString(uid) {
			return Config{}, fmt.Errorf("invalid storepart_uids[%d]", i)
		}
		if _, dup := seen[uid]; dup {
			return Config{}, fmt.Errorf("duplicate storepart uid %s", uid)
		}
		seen[uid] = struct{}{}
		cfg.StorepartUIDs[i] = uid
	}
	if cfg.APIBase == "" {
		cfg.APIBase = defaultAPIBase
	}
	api, err := url.Parse(strings.TrimRight(cfg.APIBase, "/"))
	if err != nil || api.Hostname() == "" || api.Scheme != "https" || api.User != nil {
		return Config{}, fmt.Errorf("%s api_base must be a public https URL", Kind)
	}
	host := strings.ToLower(api.Hostname())
	if host != "store.tildacdn.com" && host != "store.tildaapi.com" {
		return Config{}, fmt.Errorf("%s api_base must be store.tildacdn.com or store.tildaapi.com", Kind)
	}
	cfg.APIBase = strings.TrimRight(api.String(), "/")
	if len(cfg.IncludeNameKeywords)+len(cfg.ExcludeNameKeywords) > 50 {
		return Config{}, fmt.Errorf("%s name filters exceed supported limits", Kind)
	}
	for _, word := range append(append([]string{}, cfg.IncludeNameKeywords...), cfg.ExcludeNameKeywords...) {
		if strings.TrimSpace(word) == "" || len(word) > 100 {
			return Config{}, fmt.Errorf("%s name keywords must contain 1–100 characters", Kind)
		}
	}
	return cfg, nil
}

func (a *Adapter) Code() string { return a.code }

func (a *Adapter) Discover(ctx context.Context) ([]entity.ProductRef, error) {
	a.resetDiagnostic()
	products, err := a.loadAll(ctx)
	if err != nil {
		return nil, err
	}
	refs := make([]entity.ProductRef, 0, len(products))
	seen := map[string]struct{}{}
	for _, product := range products {
		if !a.nameAllowed(product.Title) {
			continue
		}
		productURL := a.productURL(product)
		if productURL == "" {
			continue
		}
		uid := string(product.UID)
		if _, exists := seen[uid]; exists {
			continue
		}
		seen[uid] = struct{}{}
		refs = append(refs, entity.ProductRef{SourceCode: a.code, URL: productURL, SourceID: uid})
	}
	if len(refs) == 0 {
		return nil, crawlerr.ParseFailed(a.code, fmt.Errorf("empty Tilda store discovery result"))
	}
	return refs, nil
}

func (a *Adapter) Fetch(ctx context.Context, ref entity.ProductRef) ([]entity.RawOffer, error) {
	a.resetDiagnostic()
	if ref.URL == "" && ref.SourceID == "" {
		return nil, crawlerr.ParseFailed(a.code, fmt.Errorf("empty product ref"))
	}
	product, ok := a.cached(ref)
	if !ok {
		if _, err := a.loadAll(ctx); err != nil {
			return nil, err
		}
		product, ok = a.cached(ref)
	}
	if !ok {
		return nil, crawlerr.SkippedProduct(a.code, fmt.Errorf("tilda product %s is not in configured store parts", firstNonEmpty(ref.SourceID, ref.URL)))
	}
	if !a.nameAllowed(product.Title) {
		return nil, crawlerr.SkippedProduct(a.code, fmt.Errorf("product filtered by name"))
	}
	offers := productToOffers(a.code, a.productURL(product), a.config.DefaultCurrency, product)
	if len(offers) == 0 {
		return nil, crawlerr.SkippedProduct(a.code, fmt.Errorf("product has no priced weight editions"))
	}
	return offers, nil
}

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

func (a *Adapter) loadAll(ctx context.Context) ([]storeProduct, error) {
	var out []storeProduct
	for _, uid := range a.config.StorepartUIDs {
		products, err := a.loadStorepart(ctx, uid)
		if err != nil {
			return nil, err
		}
		out = append(out, products...)
	}
	a.cacheMu.Lock()
	defer a.cacheMu.Unlock()
	for _, product := range out {
		a.byUID[string(product.UID)] = product
		if productURL := a.productURL(product); productURL != "" {
			a.byURL[canonicalProductURL(productURL)] = product
		}
	}
	return out, nil
}

func (a *Adapter) loadStorepart(ctx context.Context, uid string) ([]storeProduct, error) {
	var out []storeProduct
	for slice := 1; slice <= maxSlices; slice++ {
		body, status, err := a.getList(ctx, uid, slice)
		if err != nil {
			return nil, classifyHTTPError(err)
		}
		if status != http.StatusOK {
			return nil, classifyHTTPError(httpclient.StatusError(status, a.listURL(a.config.APIBase, uid, slice)))
		}
		page, err := parseProductList(body)
		if err != nil {
			return nil, crawlerr.ParseFailed(a.code, err)
		}
		out = append(out, page.Products...)
		if page.NextSlice == 0 || len(page.Products) == 0 {
			break
		}
		if page.Total > 0 && len(out) >= page.Total {
			break
		}
	}
	return out, nil
}

func (a *Adapter) getList(ctx context.Context, uid string, slice int) ([]byte, int, error) {
	bases := []string{a.config.APIBase}
	if a.config.APIBase == defaultAPIBase {
		bases = append(bases, fallbackAPI)
	}
	var lastBody []byte
	var lastStatus int
	var lastErr error
	for _, base := range bases {
		body, resp, err := a.get(ctx, a.listURL(base, uid, slice))
		if resp != nil {
			lastStatus = resp.StatusCode
		}
		lastBody, lastErr = body, err
		if err == nil && resp != nil && resp.StatusCode == http.StatusOK {
			return body, resp.StatusCode, nil
		}
	}
	return lastBody, lastStatus, lastErr
}

func (a *Adapter) listURL(apiBase, uid string, slice int) string {
	return fmt.Sprintf("%s/api/getproductslist/?storepartuid=%s&slice=%d&size=%d&getoptions=true&getparts=true",
		strings.TrimRight(apiBase, "/"), url.QueryEscape(uid), slice, pageSize)
}

func (a *Adapter) get(ctx context.Context, rawURL string) ([]byte, *http.Response, error) {
	body, response, err := a.cli.GetWithRetry(ctx, rawURL)
	if response == nil {
		return body, response, err
	}
	bodyHash := httpclient.CapturedBodyHash(body)
	bytesRead := int64(len(body))
	var tooLarge *httpclient.BodyTooLargeError
	if errors.As(err, &tooLarge) {
		bodyHash = tooLarge.BodyHash
		bytesRead = tooLarge.BytesRead
	}
	captured := httpclient.DiagnosticResponse{
		URL: sanitizeDiagnosticURL(rawURL), StatusCode: response.StatusCode,
		ContentType: response.Header.Get("Content-Type"), ReceivedAt: time.Now().UTC(),
		BodyHash: bodyHash, BytesRead: bytesRead, BodyTruncated: httpclient.IsBodyTruncated(err),
	}
	if len(body) > httpclient.MaxDiagnosticBodyBytes {
		captured.Body = append([]byte(nil), body[:httpclient.MaxDiagnosticBodyBytes]...)
		captured.StorageTruncated = true
	} else {
		captured.Body = append([]byte(nil), body...)
	}
	a.diagnosticMu.Lock()
	a.diagnostic = &captured
	a.diagnosticMu.Unlock()
	return body, response, err
}

func (a *Adapter) cached(ref entity.ProductRef) (storeProduct, bool) {
	a.cacheMu.RLock()
	defer a.cacheMu.RUnlock()
	if ref.SourceID != "" {
		if product, ok := a.byUID[ref.SourceID]; ok {
			return product, true
		}
	}
	if ref.URL != "" {
		if product, ok := a.byURL[canonicalProductURL(ref.URL)]; ok {
			return product, true
		}
	}
	return storeProduct{}, false
}

func (a *Adapter) productURL(product storeProduct) string {
	raw := strings.TrimSpace(product.URL)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if parsed.Host == "" {
		return a.base.ResolveReference(parsed).String()
	}
	return parsed.String()
}

func (a *Adapter) nameAllowed(name string) bool {
	name = strings.ToLower(name)
	for _, word := range a.config.ExcludeNameKeywords {
		if word != "" && strings.Contains(name, strings.ToLower(word)) {
			return false
		}
	}
	if len(a.config.IncludeNameKeywords) == 0 {
		return true
	}
	for _, word := range a.config.IncludeNameKeywords {
		if word != "" && strings.Contains(name, strings.ToLower(word)) {
			return true
		}
	}
	return false
}

func (a *Adapter) resetDiagnostic() {
	a.diagnosticMu.Lock()
	a.diagnostic = nil
	a.diagnosticMu.Unlock()
}

func classifyHTTPError(err error) error {
	var statusErr *httpclient.HTTPStatusError
	if !errors.As(err, &statusErr) {
		return crawlerr.Transient(Kind, err)
	}
	switch statusErr.Status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return crawlerr.Blocked(Kind, err)
	case http.StatusTooManyRequests:
		return crawlerr.Transient(Kind, err)
	default:
		if statusErr.Status >= 500 {
			return crawlerr.Transient(Kind, err)
		}
		return crawlerr.ParseFailed(Kind, err)
	}
}

func sanitizeDiagnosticURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	u.User = nil
	u.RawQuery = ""
	u.ForceQuery = false
	u.Fragment = ""
	return u.String()
}

func canonicalProductURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u == nil {
		return strings.TrimRight(raw, "/")
	}
	u.Fragment = ""
	u.RawQuery = ""
	u.Host = strings.TrimPrefix(strings.ToLower(u.Host), "www.")
	return strings.TrimRight(u.String(), "/")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func productToOffers(code, productURL, currency string, product storeProduct) []entity.RawOffer {
	currency, _ = pricing.NormalizeCurrency(currency)
	editions := product.Editions
	if len(editions) == 0 {
		editions = []storeEdition{{UID: product.UID, Price: string(product.Price), Quantity: product.Quantity, Image: product.imageURL(), PackM: product.PackM}}
	}
	type grouped struct {
		edition storeEdition
		weight  int
		price   decimal.Decimal
	}
	best := map[string]grouped{}
	for _, edition := range editions {
		price, err := parseTildaPrice(firstNonEmpty(edition.Price, string(product.Price)))
		if err != nil || !price.IsPositive() {
			continue
		}
		weight := editionWeightGrams(edition)
		if weight == nil {
			continue
		}
		key := strconv.Itoa(*weight) + "|" + price.String()
		current, exists := best[key]
		if !exists || preferEdition(edition, current.edition) {
			best[key] = grouped{edition: edition, weight: *weight, price: price}
		}
	}
	observed := time.Now().UTC()
	out := make([]entity.RawOffer, 0, len(best))
	for _, item := range best {
		weight := item.weight
		grams := decimal.NewFromInt(int64(weight))
		sku := firstNonEmpty(item.edition.SKU, string(item.edition.UID), product.SKU, string(product.UID))
		image := firstNonEmpty(item.edition.Image, product.imageURL())
		title := product.Title
		if label := editionWeightLabel(item.edition); label != "" {
			title = product.Title + " " + label
		}
		out = append(out, entity.RawOffer{
			Source: code, URL: productURL, SourceSKU: sku, Title: title,
			Price: item.price, Currency: currency, WeightG: &weight, WeightGrams: &grams,
			UnitHint: entity.UnitG, InStock: editionInStock(item.edition, product),
			ImageURL: image, ObservedAt: observed,
			Attrs: map[string]string{"adapter": Kind, "tilda_uid": string(product.UID), "tilda_edition_uid": string(item.edition.UID)},
		})
	}
	return out
}
