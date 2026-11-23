package ali1688

import (
	"errors"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var (
	errInvalidProductURL = errors.New("invalid 1688 product url")
	errProductNotFound   = errors.New("product data not found in page")
	errAccessDenied      = errors.New("1688 waf blocked cloud ip; set PARSER_1688_HTTP_PROXY")

	offerPathPattern = regexp.MustCompile(`(?i)/offer/(\d+)(?:\.html)?/?$`)
	offerIDQueryKeys = []string{"offerId", "offerid", "id"}
	qrPathPattern    = regexp.MustCompile(`(?i)^/s/[A-Za-z0-9]+$`)
	offerIDInText    = regexp.MustCompile(`(?:offerId=|/offer/)(\d{8,})`)
	digitsIDPattern  = regexp.MustCompile(`^\d{8,}$`)
	subjectPattern   = regexp.MustCompile(`"subject"\s*:\s*"((?:\\.|[^"\\])*)"`)
	minPricePattern  = regexp.MustCompile(`"offerMinPrice"\s*:\s*"([0-9.]+)"`)
	pricePattern     = regexp.MustCompile(`"minPrice"\s*:\s*"([0-9.]+)"`)
	currentPricePat  = regexp.MustCompile(`"currentPrices"\s*:\s*\[\s*\{\s*"beginAmount"\s*:\s*1\s*,\s*"price"\s*:\s*"([0-9.]+)"`)
	mainImagePattern = regexp.MustCompile(`"mainImage"\s*:\s*\[\s*"(https://[^"]+)"`)
	fullPathPattern  = regexp.MustCompile(`"fullPathImageURI"\s*:\s*"(https://[^"]+)"`)
	titleTagPattern  = regexp.MustCompile(`(?s)<title>(.+?)</title>`)
)

func validateProductURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("%w: %v", errInvalidProductURL, err)
	}
	if offerIDFromURL(u) != "" {
		return nil
	}
	host := strings.ToLower(strings.TrimPrefix(u.Hostname(), "www."))
	if strings.HasSuffix(host, "1688.com") && qrPathPattern.MatchString(u.Path) {
		return nil
	}
	return fmt.Errorf("%w: cannot extract offer id from %s", errInvalidProductURL, u.Path)
}

func resolveOfferID(rawURL, qrBody string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("%w: %v", errInvalidProductURL, err)
	}

	if id := offerIDFromURL(u); id != "" {
		return id, nil
	}

	host := strings.ToLower(strings.TrimPrefix(u.Hostname(), "www."))
	if host == "qr.1688.com" && qrPathPattern.MatchString(u.Path) {
		if id := offerIDFromText(qrBody); id != "" {
			return id, nil
		}
		return "", fmt.Errorf("%w: qr short link has no offer id", errInvalidProductURL)
	}

	return "", fmt.Errorf("%w: cannot extract offer id from %s", errInvalidProductURL, u.Path)
}

func offerIDFromURL(u *url.URL) string {
	if match := offerPathPattern.FindStringSubmatch(u.Path); len(match) == 2 {
		return match[1]
	}
	q := u.Query()
	for _, key := range offerIDQueryKeys {
		raw := strings.TrimSpace(q.Get(key))
		raw = strings.TrimSuffix(raw, ".html")
		if digitsIDPattern.MatchString(raw) {
			return raw
		}
	}
	return offerIDFromText(u.RawQuery)
}

func offerIDFromText(raw string) string {
	match := offerIDInText.FindStringSubmatch(raw)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func parseProductFromHTML(htmlBody, expectedID string) (productSnapshot, bool) {
	title := extractSubject(htmlBody)
	if title == "" {
		title = extractTitleTag(htmlBody)
	}
	price, ok := extractPrice(htmlBody)
	if title == "" || !ok {
		return productSnapshot{}, false
	}
	return productSnapshot{
		Title:    title,
		Price:    price,
		Currency: "CNY",
		ImageURL: extractImageURL(htmlBody),
		OfferID:  expectedID,
	}, true
}

func extractSubject(htmlBody string) string {
	match := subjectPattern.FindStringSubmatch(htmlBody)
	if len(match) < 2 {
		return ""
	}
	return unescapeJSONString(match[1])
}

func extractTitleTag(htmlBody string) string {
	match := titleTagPattern.FindStringSubmatch(htmlBody)
	if len(match) < 2 {
		return ""
	}
	title := strings.TrimSpace(html.UnescapeString(match[1]))
	title = strings.TrimSuffix(title, " - 阿里巴巴")
	title = strings.TrimSuffix(title, " - 1688")
	return strings.TrimSpace(title)
}

func extractPrice(htmlBody string) (float64, bool) {
	for _, pattern := range []*regexp.Regexp{minPricePattern, currentPricePat, pricePattern} {
		match := pattern.FindStringSubmatch(htmlBody)
		if len(match) < 2 {
			continue
		}
		price, err := strconv.ParseFloat(match[1], 64)
		if err == nil && price > 0 {
			return price, true
		}
	}
	return 0, false
}

func extractImageURL(htmlBody string) string {
	for _, pattern := range []*regexp.Regexp{mainImagePattern, fullPathPattern} {
		match := pattern.FindStringSubmatch(htmlBody)
		if len(match) >= 2 {
			return strings.TrimSpace(match[1])
		}
	}
	return ""
}

func unescapeJSONString(raw string) string {
	replacer := strings.NewReplacer(
		`\"`, `"`,
		`\\`, `\`,
		`\/`, `/`,
	)
	return strings.TrimSpace(replacer.Replace(raw))
}

func detailURL(offerID string) string {
	return fmt.Sprintf("https://detail.1688.com/offer/%s.html", offerID)
}

func isPunishPage(htmlBody string) bool {
	if htmlBody == "" {
		return false
	}
	lower := strings.ToLower(htmlBody)
	return strings.Contains(lower, "cloud_ip_bl") ||
		strings.Contains(lower, "rgv587_flag") ||
		strings.Contains(lower, "action=deny") ||
		(strings.Contains(lower, "punish") && strings.Contains(lower, "bixi.alicdn.com"))
}
