package productmatch

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

var (
	spaceRE    = regexp.MustCompile(`\s+`)
	nonAlnumRE = regexp.MustCompile(`[^a-z0-9а-яё]+`)
)

var trackingQueryParams = map[string]struct{}{
	"utm_source":   {},
	"utm_medium":   {},
	"utm_campaign": {},
	"utm_term":     {},
	"utm_content":  {},
	"fbclid":       {},
	"gclid":        {},
	"yclid":        {},
	"ref":          {},
	"from":         {},
}

// NormalizeProduct builds a searchable profile from raw input.
func NormalizeProduct(input RawProductInput) NormalizedProduct {
	title := strings.TrimSpace(input.Title)
	normalizedTitle := NormalizeTitle(title)
	tokens := tokenize(normalizedTitle)

	return NormalizedProduct{
		OriginalTitle:   title,
		NormalizedTitle: normalizedTitle,
		Brand:           NormalizeToken(input.Brand),
		Model:           NormalizeToken(input.Model),
		Category:        NormalizeToken(input.Category),
		Tokens:          tokens,
		Fingerprint:     BuildFingerprint(normalizedTitle, input.Brand, input.Model),
		NormalizedURL:     NormalizeURL(input.URL),
	}
}

var noiseWordSet = map[string]struct{}{
	"новый": {}, "new": {}, "оригинал": {}, "original": {}, "купить": {}, "buy": {},
	"цена": {}, "price": {}, "скидка": {}, "sale": {}, "акция": {}, "promo": {},
}

// NormalizeTitle lowercases and removes noise from a product title.
func NormalizeTitle(title string) string {
	title = strings.ToLower(strings.TrimSpace(title))
	title = nonAlnumRE.ReplaceAllString(title, " ")
	title = spaceRE.ReplaceAllString(title, " ")

	tokens := strings.Fields(title)
	filtered := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if _, drop := noiseWordSet[token]; drop {
			continue
		}
		filtered = append(filtered, token)
	}
	return strings.TrimSpace(strings.Join(filtered, " "))
}

// NormalizeToken normalizes a single token-like field.
func NormalizeToken(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

// NormalizeURL canonicalizes a product URL for exact duplicate checks.
func NormalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return strings.ToLower(raw)
	}

	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""

	if parsed.Scheme == "" {
		parsed.Scheme = "https"
	}

	query := parsed.Query()
	for key := range query {
		lowerKey := strings.ToLower(key)
		if _, drop := trackingQueryParams[lowerKey]; drop || strings.HasPrefix(lowerKey, "utm_") {
			query.Del(key)
		}
	}
	parsed.RawQuery = encodeSortedQuery(query)
	parsed.Path = strings.TrimRight(parsed.Path, "/")

	return parsed.String()
}

func encodeSortedQuery(values url.Values) string {
	if len(values) == 0 {
		return ""
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		vals := values[key]
		sort.Strings(vals)
		for _, val := range vals {
			parts = append(parts, url.QueryEscape(key)+"="+url.QueryEscape(val))
		}
	}
	return strings.Join(parts, "&")
}

// BuildFingerprint creates a stable key from title/brand/model.
func BuildFingerprint(normalizedTitle, brand, model string) string {
	parts := []string{
		NormalizeToken(brand),
		NormalizeToken(model),
		normalizedTitle,
	}
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	return strings.Join(filtered, "|")
}

func tokenize(normalizedTitle string) []string {
	if normalizedTitle == "" {
		return nil
	}

	raw := strings.Fields(normalizedTitle)
	seen := make(map[string]struct{}, len(raw))
	tokens := make([]string, 0, len(raw))
	for _, token := range raw {
		if len(token) < 2 {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		tokens = append(tokens, token)
	}
	sort.Strings(tokens)
	return tokens
}

func isLetterOrDigit(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
