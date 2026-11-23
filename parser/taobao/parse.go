package taobao

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
	errInvalidProductURL = errors.New("invalid taobao product url")
	errProductNotFound   = errors.New("product data not found in page")
	errAccessDenied      = errors.New("taobao blocked this ip; set PARSER_TAOBAO_HTTP_PROXY")

	itemIDQueryKeys   = []string{"id", "itemId", "item_id", "itemid"}
	digitsIDPattern   = regexp.MustCompile(`^\d{8,}$`)
	pathItemIDPattern = regexp.MustCompile(`(?i)/(?:item|i)(\d{8,})\.htm`)
	listItemIDPattern = regexp.MustCompile(`(?i)/list/item/(\d{8,})(?:\.htm)?`)
	shortURLPattern   = regexp.MustCompile(`(?i)var\s+url\s*=\s*['"]([^'"]+)['"]`)
	ogTitlePatterns   = []*regexp.Regexp{
		regexp.MustCompile(`(?i)property=["']og:title["'][^>]*content=["']([^"']+)["']`),
		regexp.MustCompile(`(?i)content=["']([^"']+)["'][^>]*property=["']og:title["']`),
	}
	ogImagePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)property=["']og:image["'][^>]*content=["']([^"']+)["']`),
		regexp.MustCompile(`(?i)content=["']([^"']+)["'][^>]*property=["']og:image["']`),
	}
	jsonItemTitlePattern = regexp.MustCompile(`"title"\s*:\s*"((?:\\.|[^"\\])*)"\s*\}\s*,\s*"itemExist"\s*:\s*true`)
	titleTagPattern      = regexp.MustCompile(`(?is)<title>(.*?)</title>`)
	promoPricePattern    = regexp.MustCompile(`"promotionPrice"\s*:\s*"?([0-9.]+)"?`)
	origPricePattern     = regexp.MustCompile(`"originalPrice"\s*:\s*"?([0-9.]+)"?`)
)

func validateProductURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("%w: %v", errInvalidProductURL, err)
	}
	host := strings.ToLower(strings.TrimPrefix(u.Hostname(), "www."))
	if isShortHost(host) {
		return nil
	}
	if itemIDFromURL(u) == "" {
		return fmt.Errorf("%w: cannot extract item id from %s", errInvalidProductURL, u.Path)
	}
	return nil
}

func isShortHost(host string) bool {
	return host == "e.tb.cn" || host == "m.tb.cn" || host == "tb.cn"
}

func itemIDFromURL(u *url.URL) string {
	if u == nil {
		return ""
	}
	q := u.Query()
	for _, key := range itemIDQueryKeys {
		raw := strings.TrimSpace(q.Get(key))
		if digitsIDPattern.MatchString(raw) {
			return raw
		}
	}
	if match := listItemIDPattern.FindStringSubmatch(u.Path); len(match) == 2 {
		return match[1]
	}
	if match := pathItemIDPattern.FindStringSubmatch(u.Path); len(match) == 2 {
		return match[1]
	}
	return ""
}

func itemIDFromShortHTML(htmlBody string) (itemID string, price float64, ok bool) {
	match := shortURLPattern.FindStringSubmatch(htmlBody)
	if len(match) < 2 {
		return "", 0, false
	}
	target, err := url.Parse(html.UnescapeString(match[1]))
	if err != nil {
		return "", 0, false
	}
	id := itemIDFromURL(target)
	if id == "" {
		return "", 0, false
	}
	if raw := strings.TrimSpace(target.Query().Get("price")); raw != "" {
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil && parsed > 0 {
			price = parsed
		}
	}
	return id, price, true
}

func parseCardHTML(htmlBody, itemID string) productSnapshot {
	return productSnapshot{
		Title:    firstNonEmpty(extractJSONItemTitle(htmlBody), extractOGTitle(htmlBody), extractTitleTag(htmlBody)),
		Price:    firstPositivePrice(htmlBody),
		Currency: "CNY",
		ImageURL: strings.TrimSpace(extractOGRaw(htmlBody, ogImagePatterns)),
		ItemID:   itemID,
	}
}

func assembleProduct(itemID string, fallbackPrice float64, pages []string, sawBlock bool) (productSnapshot, error) {
	product := productSnapshot{ItemID: itemID, Currency: "CNY"}
	for _, htmlBody := range pages {
		if strings.TrimSpace(htmlBody) == "" {
			continue
		}
		parsed := parseCardHTML(htmlBody, itemID)
		if parsed.Title != "" || parsed.Price > 0 {
			mergeSnapshot(&product, parsed)
			continue
		}
		if isBlockedCardHTML(htmlBody) {
			continue
		}
		mergeSnapshot(&product, parsed)
	}
	if product.Price <= 0 && fallbackPrice > 0 {
		product.Price = fallbackPrice
	}
	if product.Price <= 0 {
		if sawBlock {
			return productSnapshot{}, errAccessDenied
		}
		return productSnapshot{}, errProductNotFound
	}
	return product, nil
}

func mergeSnapshot(dst *productSnapshot, src productSnapshot) {
	if src.Title != "" && (dst.Title == "" || len(src.Title) > len(dst.Title)) {
		dst.Title = src.Title
	}
	if dst.Price <= 0 && src.Price > 0 {
		dst.Price = src.Price
	}
	if dst.ImageURL == "" {
		dst.ImageURL = src.ImageURL
	}
	if dst.Currency == "" {
		dst.Currency = src.Currency
	}
	if dst.ItemID == "" {
		dst.ItemID = src.ItemID
	}
}

func extractOGTitle(htmlBody string) string {
	title := cleanTitle(extractOGRaw(htmlBody, ogTitlePatterns))
	if isGenericTitle(title) {
		return ""
	}
	return title
}

func extractOGRaw(htmlBody string, patterns []*regexp.Regexp) string {
	for _, pattern := range patterns {
		match := pattern.FindStringSubmatch(htmlBody)
		if len(match) < 2 {
			continue
		}
		value := strings.TrimSpace(html.UnescapeString(match[1]))
		if value != "" {
			return value
		}
	}
	return ""
}

func extractJSONItemTitle(htmlBody string) string {
	match := jsonItemTitlePattern.FindStringSubmatch(htmlBody)
	if len(match) < 2 {
		return ""
	}
	title := cleanTitle(unescapeJSONString(match[1]))
	if isGenericTitle(title) {
		return ""
	}
	return title
}

func extractTitleTag(htmlBody string) string {
	match := titleTagPattern.FindStringSubmatch(htmlBody)
	if len(match) < 2 {
		return ""
	}
	title := cleanTitle(html.UnescapeString(match[1]))
	if isGenericTitle(title) {
		return ""
	}
	return title
}

func firstPositivePrice(htmlBody string) float64 {
	price, ok := extractPrice(htmlBody)
	if !ok {
		return 0
	}
	return price
}

func extractPrice(htmlBody string) (float64, bool) {
	for _, pattern := range []*regexp.Regexp{promoPricePattern, origPricePattern} {
		match := pattern.FindStringSubmatch(htmlBody)
		if len(match) < 2 {
			continue
		}
		price, err := parsePositiveFloat(match[1])
		if err == nil {
			return price, true
		}
	}
	return 0, false
}

func parsePositiveFloat(raw string) (float64, error) {
	price, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || price <= 0 {
		return 0, fmt.Errorf("invalid price %q", raw)
	}
	return price, nil
}

func cleanTitle(raw string) string {
	title := strings.TrimSpace(raw)
	title = strings.TrimSuffix(title, "-淘宝Taobao | 天猫Tmall")
	title = strings.TrimSuffix(title, "-淘寶Taobao | 天貓Tmall")
	title = strings.TrimSpace(title)
	title = strings.TrimRight(title, ".")
	title = strings.TrimSuffix(title, "...")
	return strings.TrimSpace(title)
}

func isGenericTitle(title string) bool {
	switch strings.ToLower(strings.TrimSpace(title)) {
	case "", "taobao", "tmall", "淘宝", "淘宝网", "天猫", "淘寶", "天貓":
		return true
	default:
		return false
	}
}

func isBlockedCardHTML(htmlBody string) bool {
	if strings.TrimSpace(htmlBody) == "" {
		return true
	}
	lower := strings.ToLower(htmlBody)
	if strings.Contains(lower, "cloud_ip_bl") ||
		strings.Contains(lower, "_____tmd_____") ||
		strings.Contains(lower, "x5secdata") ||
		(strings.Contains(lower, "punish") && strings.Contains(lower, "alicdn.com")) {
		return true
	}
	if len(htmlBody) < 12000 &&
		!strings.Contains(htmlBody, "promotionPrice") &&
		!strings.Contains(htmlBody, "originalPrice") &&
		extractOGTitle(htmlBody) == "" {
		return true
	}
	return false
}

func unescapeJSONString(raw string) string {
	replacer := strings.NewReplacer(
		`\"`, `"`,
		`\\`, `\`,
		`\/`, `/`,
	)
	return strings.TrimSpace(replacer.Replace(raw))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func listItemURL(itemID string) string {
	return fmt.Sprintf("https://www.taobao.com/list/item/%s.htm", itemID)
}

func worldItemURL(itemID string) string {
	return fmt.Sprintf("https://world.taobao.com/item/%s.htm", itemID)
}

func cardSourceURLs(itemID string) []string {
	return []string{listItemURL(itemID), worldItemURL(itemID)}
}
