package recipe

import (
	"encoding/json"
	"html"
	"regexp"
	"strconv"
	"strings"
)

var (
	ldJSONPattern  = regexp.MustCompile(`(?is)<script[^>]*type=["']application/ld\+json["'][^>]*>(.*?)</script>`)
	ogTitlePattern = regexp.MustCompile(`(?is)<meta[^>]+property=["']og:title["'][^>]+content=["']([^"']+)["']`)
	ogTitleAlt     = regexp.MustCompile(`(?is)<meta[^>]+content=["']([^"']+)["'][^>]+property=["']og:title["']`)
	ogImagePattern = regexp.MustCompile(`(?is)<meta[^>]+property=["']og:image["'][^>]+content=["']([^"']+)["']`)
	ogImageAlt     = regexp.MustCompile(`(?is)<meta[^>]+content=["']([^"']+)["'][^>]+property=["']og:image["']`)

	h1NamePattern = regexp.MustCompile(
		`(?is)<h1[^>]*itemprop=["']name["'][^>]*>([\s\S]*?)</h1>`,
	)
	priceContentPattern = regexp.MustCompile(
		`(?is)itemprop=["']price["']\s+content=["']([\d.]+)["']`,
	)
	priceContentAltPattern = regexp.MustCompile(
		`(?is)content=["']([\d.]+)["'][^>]*itemprop=["']price["']`,
	)
	priceCurrencyPattern = regexp.MustCompile(
		`(?is)itemprop=["']priceCurrency["']\s+content=["']([A-Z]{3})["']`,
	)
	microdataImagePattern = regexp.MustCompile(
		`(?is)itemprop=["']image["'][\s\S]{0,500}?itemprop=["']contentUrl["']\s+(?:href|content)=["'](https?://[^"']+)["']`,
	)
	adProductPattern = regexp.MustCompile(
		`window\.ad_product=\{id:"([^"]*)",vendor:"[^"]*",price:"([^"]*)"[^}]*name:"((?:\\.|[^"\\])*)"`,
	)
	tagStrip = regexp.MustCompile(`<[^>]+>`)
)

// Snapshot is a neutral product card (before host currency types).
type Snapshot struct {
	Title    string
	Price    float64
	Currency string
	ImageURL string
	HasPrice bool
}

// Extract applies cfg field pipelines to HTML (no path product id).
func Extract(htmlBody string, cfg Config) Snapshot {
	return ExtractPage(htmlBody, cfg, "")
}

// ExtractPage is Extract plus optional pathID for embedded_*_by_id extractors.
func ExtractPage(htmlBody string, cfg Config, pathID string) Snapshot {
	cfg = cfg.withDefaults()
	products := collectJSONLDProducts(htmlBody)
	var out Snapshot

	for _, id := range cfg.Title {
		if out.Title != "" {
			break
		}
		switch id {
		case "jsonld_product_name":
			for _, p := range products {
				if t := strings.TrimSpace(p.Title); t != "" {
					out.Title = t
					break
				}
			}
		case "og_title":
			out.Title = stripTitleSuffixes(firstMeta(htmlBody, ogTitlePattern, ogTitleAlt), cfg.TitleStripSuffixes)
		case "microdata_name":
			out.Title = extractH1Name(htmlBody)
		case "h1_text":
			out.Title = extractPlainH1(htmlBody)
		case "title_regex":
			out.Title = firstRegexCapture(htmlBody, cfg.TitleRegexes)
		case "embedded_name_by_id":
			if pathID != "" {
				if p, ok := extractEmbeddedByID(htmlBody, pathID); ok {
					out.Title = p.Title
				}
			}
		case "ad_product_name":
			if p, ok := extractAdProduct(htmlBody); ok {
				out.Title = p.Title
			}
		}
	}

	for _, id := range cfg.Price {
		if out.HasPrice {
			break
		}
		switch id {
		case "jsonld_offer_price", "jsonld_aggregate_low_price":
			for _, p := range products {
				if p.HasPrice && p.Price > 0 {
					out.Price = p.Price
					out.HasPrice = true
					if out.Currency == "" && p.Currency != "" {
						out.Currency = p.Currency
					}
					break
				}
			}
		case "microdata_price":
			if p, ok := parseFloatMatch(htmlBody, priceContentPattern, priceContentAltPattern); ok {
				out.Price = p
				out.HasPrice = true
			}
		case "price_regex":
			if p, ok := firstRegexPrice(htmlBody, cfg.PriceRegexes); ok {
				out.Price = p
				out.HasPrice = true
			}
		case "embedded_price_by_id":
			if pathID != "" {
				if p, ok := extractEmbeddedByID(htmlBody, pathID); ok && p.HasPrice {
					out.Price = p.Price
					out.HasPrice = true
					if out.Currency == "" {
						out.Currency = p.Currency
					}
					if out.Title == "" {
						out.Title = p.Title
					}
				}
			}
		case "ad_product_price":
			if p, ok := extractAdProduct(htmlBody); ok && p.HasPrice {
				out.Price = p.Price
				out.HasPrice = true
				if out.Title == "" {
					out.Title = p.Title
				}
			}
		}
	}

	for _, id := range cfg.Image {
		if out.ImageURL != "" {
			break
		}
		switch id {
		case "jsonld_product_image":
			for _, p := range products {
				if u := strings.TrimSpace(p.ImageURL); u != "" {
					out.ImageURL = u
					break
				}
			}
		case "og_image":
			out.ImageURL = firstMeta(htmlBody, ogImagePattern, ogImageAlt)
		case "image_regex":
			out.ImageURL = firstRegexCapture(htmlBody, cfg.ImageRegexes)
		case "microdata_image":
			if m := microdataImagePattern.FindStringSubmatch(htmlBody); len(m) >= 2 {
				out.ImageURL = strings.TrimSpace(m[1])
			}
		}
	}

	for _, id := range cfg.Currency {
		if out.Currency != "" {
			break
		}
		switch id {
		case "jsonld_price_currency":
			for _, p := range products {
				if c := strings.TrimSpace(p.Currency); c != "" {
					out.Currency = c
					break
				}
			}
		case "microdata_currency":
			if m := priceCurrencyPattern.FindStringSubmatch(htmlBody); len(m) >= 2 {
				out.Currency = strings.TrimSpace(m[1])
			}
		case "default_currency":
			out.Currency = cfg.DefaultCurrency
		}
	}
	if out.Currency == "" {
		out.Currency = cfg.DefaultCurrency
	}
	if out.ImageURL != "" && cfg.ImageFitUpscale != "" {
		out.ImageURL = upscaleFitURL(out.ImageURL, cfg.ImageFitUpscale)
	}
	return out
}

func stripTitleSuffixes(title string, suffixes []string) string {
	title = strings.TrimSpace(html.UnescapeString(title))
	for _, suf := range suffixes {
		if suf == "" {
			continue
		}
		if idx := strings.Index(title, suf); idx > 0 {
			return strings.TrimSpace(title[:idx])
		}
	}
	return title
}

func extractH1Name(htmlBody string) string {
	m := h1NamePattern.FindStringSubmatch(htmlBody)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(tagStrip.ReplaceAllString(m[1], "")))
}

func parseFloatMatch(body string, patterns ...*regexp.Regexp) (float64, bool) {
	for _, p := range patterns {
		if m := p.FindStringSubmatch(body); len(m) >= 2 {
			v, err := strconv.ParseFloat(strings.TrimSpace(m[1]), 64)
			if err == nil && v > 0 {
				return v, true
			}
		}
	}
	return 0, false
}

func extractEmbeddedByID(htmlBody, productID string) (Snapshot, bool) {
	if productID == "" {
		return Snapshot{}, false
	}
	// "name":"...","id":"<id>","price":N
	pat := regexp.MustCompile(
		`"name":"((?:\\.|[^"\\])*)","id":"` + regexp.QuoteMeta(productID) + `","price":([\d.]+)`,
	)
	if m := pat.FindStringSubmatch(htmlBody); len(m) >= 3 {
		price, err := strconv.ParseFloat(strings.TrimSpace(m[2]), 64)
		name := decodeJSString(m[1])
		if err == nil && price > 0 && name != "" {
			return Snapshot{Title: name, Price: price, Currency: "BYN", HasPrice: true}, true
		}
	}
	// "id":"<id>" ... "price":N (within 240 chars)
	pat2 := regexp.MustCompile(
		`"id":"` + regexp.QuoteMeta(productID) + `"(?:(?!}).){0,240}?"price":([\d.]+)`,
	)
	if m := pat2.FindStringSubmatch(htmlBody); len(m) >= 2 {
		price, err := strconv.ParseFloat(strings.TrimSpace(m[1]), 64)
		if err == nil && price > 0 {
			return Snapshot{Price: price, Currency: "BYN", HasPrice: true}, true
		}
	}
	return Snapshot{}, false
}

func extractAdProduct(htmlBody string) (Snapshot, bool) {
	m := adProductPattern.FindStringSubmatch(htmlBody)
	if len(m) < 4 {
		return Snapshot{}, false
	}
	if m[1] == "" || m[1] == "undefined" || m[2] == "" || m[2] == "undefined" {
		return Snapshot{}, false
	}
	price, err := strconv.ParseFloat(m[2], 64)
	name := decodeJSString(m[3])
	if err != nil || price <= 0 || name == "" || name == "undefined" {
		return Snapshot{}, false
	}
	return Snapshot{Title: name, Price: price, Currency: "BYN", HasPrice: true}, true
}

func decodeJSString(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var decoded string
	if err := json.Unmarshal([]byte(`"`+raw+`"`), &decoded); err == nil {
		return decoded
	}
	replaced := strings.ReplaceAll(raw, `\x22`, `"`)
	replaced = strings.ReplaceAll(replaced, `\"`, `"`)
	return html.UnescapeString(replaced)
}


func extractPlainH1(htmlBody string) string {
	m := regexp.MustCompile(`(?is)<h1[^>]*>([\s\S]*?)</h1>`).FindStringSubmatch(htmlBody)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(tagStrip.ReplaceAllString(m[1], "")))
}

func firstRegexCapture(body string, patterns []string) string {
	for _, raw := range patterns {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		re, err := regexp.Compile(raw)
		if err != nil {
			continue
		}
		if m := re.FindStringSubmatch(body); len(m) >= 2 {
			return strings.TrimSpace(html.UnescapeString(tagStrip.ReplaceAllString(m[1], "")))
		}
	}
	return ""
}

func firstRegexPrice(body string, patterns []string) (float64, bool) {
	for _, raw := range patterns {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		re, err := regexp.Compile(raw)
		if err != nil {
			continue
		}
		if m := re.FindStringSubmatch(body); len(m) >= 2 {
			s := strings.ReplaceAll(strings.TrimSpace(m[1]), " ", "")
			s = strings.ReplaceAll(s, ",", ".")
			v, err := strconv.ParseFloat(s, 64)
			if err == nil && v > 0 {
				return v, true
			}
		}
	}
	return 0, false
}

func upscaleFitURL(imageURL, to string) string {
	to = strings.Trim(strings.TrimSpace(to), "/")
	if to == "" {
		return imageURL
	}
	for _, size := range []string{"45/45", "24/24", "155/155", "200/200", "300/300"} {
		from := "/fit/" + size + "/"
		if strings.Contains(imageURL, from) {
			return strings.Replace(imageURL, from, "/fit/"+to+"/", 1)
		}
	}
	return imageURL
}


func firstMeta(body string, patterns ...*regexp.Regexp) string {
	for _, p := range patterns {
		if m := p.FindStringSubmatch(body); len(m) >= 2 {
			return strings.TrimSpace(html.UnescapeString(m[1]))
		}
	}
	return ""
}

func collectJSONLDProducts(htmlBody string) []Snapshot {
	var out []Snapshot
	for _, match := range ldJSONPattern.FindAllStringSubmatch(htmlBody, -1) {
		if len(match) < 2 {
			continue
		}
		var payload any
		if err := json.Unmarshal([]byte(strings.TrimSpace(match[1])), &payload); err != nil {
			continue
		}
		walkJSONLD(payload, &out)
	}
	return out
}

func walkJSONLD(node any, out *[]Snapshot) {
	switch v := node.(type) {
	case map[string]any:
		if isProductNode(v) {
			if s, ok := snapshotFromProduct(v); ok {
				*out = append(*out, s)
			}
		}
		if g, ok := v["@graph"].([]any); ok {
			for _, item := range g {
				walkJSONLD(item, out)
			}
		}
	case []any:
		for _, item := range v {
			walkJSONLD(item, out)
		}
	}
}

func isProductNode(v map[string]any) bool {
	switch t := v["@type"].(type) {
	case string:
		return strings.EqualFold(t, "Product")
	case []any:
		for _, item := range t {
			if s, ok := item.(string); ok && strings.EqualFold(s, "Product") {
				return true
			}
		}
	}
	return false
}

func snapshotFromProduct(v map[string]any) (Snapshot, bool) {
	title, _ := v["name"].(string)
	title = strings.TrimSpace(title)
	s := Snapshot{
		Title:    title,
		ImageURL: imageFromNode(v["image"]),
	}
	if price, cur, ok := priceFromOffers(v["offers"]); ok {
		s.Price = price
		s.Currency = cur
		s.HasPrice = true
	}
	if title == "" && !s.HasPrice && s.ImageURL == "" {
		return Snapshot{}, false
	}
	return s, true
}

func imageFromNode(raw any) string {
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v)
	case []any:
		for _, item := range v {
			if u := imageFromNode(item); u != "" {
				return u
			}
		}
	case map[string]any:
		for _, key := range []string{"url", "contentUrl", "thumbnailUrl"} {
			if s, ok := v[key].(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func priceFromOffers(raw any) (float64, string, bool) {
	switch offers := raw.(type) {
	case map[string]any:
		return parseOfferMap(offers)
	case []any:
		for _, item := range offers {
			if m, ok := item.(map[string]any); ok {
				if p, c, ok := parseOfferMap(m); ok {
					return p, c, true
				}
			}
		}
	}
	return 0, "", false
}

func parseOfferMap(offer map[string]any) (float64, string, bool) {
	cur, _ := offer["priceCurrency"].(string)
	cur = strings.TrimSpace(cur)
	for _, key := range []string{"price", "lowPrice"} {
		switch value := offer[key].(type) {
		case string:
			p, err := strconv.ParseFloat(strings.TrimSpace(strings.ReplaceAll(value, " ", "")), 64)
			if err == nil && p > 0 {
				return p, cur, true
			}
		case float64:
			if value > 0 {
				return value, cur, true
			}
		}
	}
	return 0, "", false
}
