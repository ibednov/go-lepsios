package ozby

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ibednov/go-lepsios/log"
	"github.com/ibednov/go-lepsios/productparse/fetchretry"
)

var ozbyHTTPClient = &http.Client{
	Timeout: 12 * time.Second,
}

const (
	ozbyHTTPUserAgent = "Mozilla/5.0 (compatible; WishimiParser/1.0)"
	ozbyFetchAttempts = 3
)

var productPathPattern = regexp.MustCompile(`^/([a-z0-9_]+)/more(\d+)\.html$`)

var blockedCategories = []string{
	"sseries",
	"producer",
	"help",
	"checkout",
	"about",
	"store",
	"bonus",
	"partners",
	"app",
	"personal",
}

var (
	errInvalidProductPath = errors.New("product path must match /{category}/more{id}.html")
	errProductNotFound    = errors.New("product data not found in page")
)

func validateProductURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}

	match := productPathPattern.FindStringSubmatch(u.Path)
	if len(match) < 2 {
		return fmt.Errorf("%w: %s", errInvalidProductPath, u.Path)
	}

	category := strings.ToLower(match[1])
	if slices.Contains(blockedCategories, category) {
		return fmt.Errorf("%w: blocked category %s", errInvalidProductPath, category)
	}

	return nil
}

func productIDFromURL(pageURL string) (string, error) {
	u, err := url.Parse(pageURL)
	if err != nil {
		return "", err
	}

	match := productPathPattern.FindStringSubmatch(u.Path)
	if len(match) < 3 {
		return "", fmt.Errorf("%w: %s", errInvalidProductPath, u.Path)
	}

	return match[2], nil
}

func (s *Strategy) fetchProduct(ctx context.Context, pageURL string) (productSnapshot, error) {
	productID, err := productIDFromURL(pageURL)
	if err != nil {
		return productSnapshot{}, err
	}

	retryNumber := 0
	for attempt := 0; attempt < ozbyFetchAttempts; attempt++ {
		htmlBody, fetchErr := fetchProductHTML(ctx, pageURL)
		if fetchErr != nil {
			if fetchretry.Retryable(fetchErr) && attempt+1 < ozbyFetchAttempts {
				retryNumber++
				var statusErr *fetchretry.HTTPStatusError
				var retryAfter time.Duration
				if errors.As(fetchErr, &statusErr) {
					retryAfter = statusErr.RetryAfter
				}
				if err := fetchretry.Wait(ctx, retryNumber, retryAfter); err != nil {
					return productSnapshot{}, err
				}
				continue
			}
			return productSnapshot{}, fetchErr
		}

		if product, ok := parseProductFromHTML(htmlBody, productID); ok {
			return enrichProductImage(product, htmlBody), nil
		}
		logParseFailure(pageURL, htmlBody)
		return productSnapshot{}, errProductNotFound
	}
	return productSnapshot{}, errProductNotFound
}

func parseProductFromHTML(htmlBody, productID string) (productSnapshot, bool) {
	if product, ok := extractFromMicrodata(htmlBody); ok {
		return product, true
	}
	if product, ok := extractFromProductsJSON(htmlBody, productID); ok {
		return product, true
	}
	if product, ok := extractFromAdProduct(htmlBody); ok {
		return product, true
	}
	if product, ok := extractFromOGFallback(htmlBody, productID); ok {
		return product, true
	}

	return productSnapshot{}, false
}

func enrichProductImage(product productSnapshot, htmlBody string) productSnapshot {
	if product.ImageURL == "" {
		product.ImageURL = extractOGImage(htmlBody)
	}
	if product.ImageURL == "" {
		product.ImageURL = extractProductImageURL(htmlBody)
	}
	return product
}

func logParseFailure(pageURL, htmlBody string) {
	title := extractPageTitle(htmlBody)
	log.Warn("parser.ozby.parse_failed",
		"component", "parser",
		"strategy", "ozby",
		"url", pageURL,
		"html_bytes", len(htmlBody),
		"page_title", title,
		"has_product_marker", productPagePattern.MatchString(htmlBody),
		"has_products_json", strings.Contains(htmlBody, `"id":"`) && strings.Contains(htmlBody, `"price":`),
	)
}

func fetchProductHTML(ctx context.Context, pageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", ozbyHTTPUserAgent)
	req.Header.Set("Accept-Language", "ru")

	resp, err := ozbyHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fetchretry.NewHTTPStatusError(resp, pageURL)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func extractFromMicrodata(htmlBody string) (productSnapshot, bool) {
	title := extractH1Name(htmlBody)
	price, ok := extractMicrodataPrice(htmlBody)
	if title == "" || !ok {
		return productSnapshot{}, false
	}

	return productSnapshot{
		Title:        formatTitle(title, extractManufacturer(htmlBody)),
		Price:        price,
		Currency:     extractMicrodataCurrency(htmlBody),
		Manufacturer: extractManufacturer(htmlBody),
		ImageURL:     extractOGImage(htmlBody),
	}, true
}

func extractFromProductsJSON(htmlBody, productID string) (productSnapshot, bool) {
	pattern := productsJSONPattern(productID)
	match := pattern.FindStringSubmatch(htmlBody)
	if len(match) < 3 {
		return productSnapshot{}, false
	}

	price, err := strconv.ParseFloat(strings.TrimSpace(match[2]), 64)
	if err != nil || price <= 0 {
		return productSnapshot{}, false
	}

	name := decodeJSString(match[1])
	if name == "" {
		return productSnapshot{}, false
	}

	return productSnapshot{
		Title:    name,
		Price:    price,
		Currency: "BYN",
		ImageURL: extractOGImage(htmlBody),
	}, true
}

func productsJSONPattern(productID string) *regexp.Regexp {
	return regexp.MustCompile(
		`"name":"((?:\\.|[^"\\])*)","id":"` + regexp.QuoteMeta(productID) + `","price":([\d.]+)`,
	)
}

func extractFromOGFallback(htmlBody, productID string) (productSnapshot, bool) {
	title := normalizeOGTitle(extractOGTitle(htmlBody))
	if title == "" {
		return productSnapshot{}, false
	}

	price, ok := extractPriceForProductID(htmlBody, productID)
	if !ok {
		return productSnapshot{}, false
	}

	return productSnapshot{
		Title:    title,
		Price:    price,
		Currency: "BYN",
		ImageURL: extractOGImage(htmlBody),
	}, true
}

func extractPriceForProductID(htmlBody, productID string) (float64, bool) {
	if product, ok := extractFromProductsJSON(htmlBody, productID); ok {
		return product.Price, true
	}

	pattern := regexp.MustCompile(
		`"id":"` + regexp.QuoteMeta(productID) + `"(?:(?!}).){0,240}?"price":([\d.]+)`,
	)
	match := pattern.FindStringSubmatch(htmlBody)
	if len(match) < 2 {
		return 0, false
	}

	price, err := strconv.ParseFloat(strings.TrimSpace(match[1]), 64)
	if err != nil || price <= 0 {
		return 0, false
	}

	return price, true
}

func extractFromAdProduct(htmlBody string) (productSnapshot, bool) {
	match := adProductPattern.FindStringSubmatch(htmlBody)
	if len(match) < 4 {
		return productSnapshot{}, false
	}

	if match[1] == "" || match[1] == "undefined" || match[2] == "" || match[2] == "undefined" {
		return productSnapshot{}, false
	}

	price, err := strconv.ParseFloat(match[2], 64)
	if err != nil || price <= 0 {
		return productSnapshot{}, false
	}

	name := decodeJSString(match[3])
	if name == "" || name == "undefined" {
		return productSnapshot{}, false
	}

	return productSnapshot{
		Title:    name,
		Price:    price,
		Currency: "BYN",
	}, true
}

func formatTitle(name, brand string) string {
	name = strings.TrimSpace(name)
	brand = strings.TrimSpace(brand)
	if brand == "" || strings.Contains(strings.ToLower(name), strings.ToLower(brand)) {
		return name
	}
	return fmt.Sprintf("%s - Бренд %s", name, brand)
}

func normalizeOGTitle(title string) string {
	title = strings.TrimSpace(html.UnescapeString(title))
	if title == "" {
		return ""
	}

	for _, suffix := range []string{" - на OZ.by", " — на OZ.by"} {
		if idx := strings.Index(title, suffix); idx > 0 {
			return strings.TrimSpace(title[:idx])
		}
	}

	return title
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

var (
	productPagePattern = regexp.MustCompile(`itemtype="https://schema\.org/Product"`)

	h1NamePattern = regexp.MustCompile(
		`<h1[^>]*itemprop=["']name["'][^>]*>([\s\S]*?)</h1>`,
	)
	priceContentPattern = regexp.MustCompile(
		`itemprop=["']price["']\s+content=["']([\d.]+)["']`,
	)
	priceContentAltPattern = regexp.MustCompile(
		`content=["']([\d.]+)["'][^>]*itemprop=["']price["']`,
	)
	priceCurrencyPattern = regexp.MustCompile(
		`itemprop=["']priceCurrency["']\s+content=["']([A-Z]{3})["']`,
	)
	manufacturerPattern = regexp.MustCompile(
		`itemprop=["']manufacturer["'][^>]*>([^<]+)<`,
	)
	ogTitlePattern = regexp.MustCompile(
		`<meta\s+property=["']og:title["']\s+content=["']([^"']+)["']`,
	)
	ogImagePattern = regexp.MustCompile(
		`<meta\s+property=["']og:image["']\s+content=["']([^"']+)["']`,
	)
	pageTitlePattern = regexp.MustCompile(
		`<title>([^<]*)</title>`,
	)
	productImageContentPattern = regexp.MustCompile(
		`itemprop=["']image["'][\s\S]{0,400}?itemprop=["']contentUrl["']\s+href=["'](https://[^"']+-goods\.ozstatic\.by/[^"']+)["']`,
	)
	adProductPattern = regexp.MustCompile(
		`window\.ad_product=\{id:"([^"]*)",vendor:"[^"]*",price:"([^"]*)"[^}]*name:"((?:\\.|[^"\\])*)"`,
	)
)

func extractH1Name(htmlBody string) string {
	match := h1NamePattern.FindStringSubmatch(htmlBody)
	if len(match) < 2 {
		return ""
	}
	title := regexp.MustCompile(`<[^>]+>`).ReplaceAllString(match[1], "")
	return strings.TrimSpace(html.UnescapeString(title))
}

func extractMicrodataPrice(htmlBody string) (float64, bool) {
	for _, pattern := range []*regexp.Regexp{priceContentPattern, priceContentAltPattern} {
		for _, match := range pattern.FindAllStringSubmatch(htmlBody, -1) {
			if len(match) < 2 {
				continue
			}
			price, err := strconv.ParseFloat(strings.TrimSpace(match[1]), 64)
			if err == nil && price > 0 {
				return price, true
			}
		}
	}
	return 0, false
}

func extractMicrodataCurrency(htmlBody string) string {
	match := priceCurrencyPattern.FindStringSubmatch(htmlBody)
	if len(match) < 2 {
		return "BYN"
	}
	return strings.TrimSpace(match[1])
}

func extractManufacturer(htmlBody string) string {
	match := manufacturerPattern.FindStringSubmatch(htmlBody)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(match[1]))
}

func extractOGTitle(htmlBody string) string {
	match := ogTitlePattern.FindStringSubmatch(htmlBody)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func extractPageTitle(htmlBody string) string {
	match := pageTitlePattern.FindStringSubmatch(htmlBody)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(match[1]))
}

func extractOGImage(htmlBody string) string {
	match := ogImagePattern.FindStringSubmatch(htmlBody)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func extractProductImageURL(htmlBody string) string {
	match := productImageContentPattern.FindStringSubmatch(htmlBody)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}
