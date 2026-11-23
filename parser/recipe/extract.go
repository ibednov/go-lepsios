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
)

// Snapshot is a neutral product card (before host currency types).
type Snapshot struct {
	Title    string
	Price    float64
	Currency string
	ImageURL string
	HasPrice bool
}

// Extract applies cfg field pipelines to HTML.
func Extract(htmlBody string, cfg Config) Snapshot {
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
			out.Title = firstMeta(htmlBody, ogTitlePattern, ogTitleAlt)
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
		case "default_currency":
			out.Currency = cfg.DefaultCurrency
		}
	}
	if out.Currency == "" {
		out.Currency = cfg.DefaultCurrency
	}
	return out
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
