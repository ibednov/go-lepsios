package dns

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var dnsHTTPClient = &http.Client{
	Timeout: 12 * time.Second,
}

const dnsHTTPUserAgent = "Mozilla/5.0 (compatible; WishimiParser/1.0)"

var (
	productPathWithLocale = regexp.MustCompile(
		`^/(ru|be|kg)/product/([0-9a-f]{16})/[^/]+/?$`,
	)
	productPathRoot = regexp.MustCompile(
		`^/product/([0-9a-f]{16})/[^/]+/?$`,
	)
)

var (
	errInvalidProductPath = errors.New("product path must match /{locale}/product/{id}/{slug}/ or /product/{id}/{slug}/")
	errProductNotFound    = errors.New("product data not found in page")
)

var jsonUnmarshal = json.Unmarshal

func validateProductURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}

	path := u.Path
	if strings.Contains(path, "/opinions") ||
		strings.Contains(path, "/service-rating") ||
		strings.Contains(path, "/catalog/") ||
		strings.Contains(path, "/category/") {
		return fmt.Errorf("%w: %s", errInvalidProductPath, path)
	}

	if productPathWithLocale.MatchString(path) || productPathRoot.MatchString(path) {
		return nil
	}

	return fmt.Errorf("%w: %s", errInvalidProductPath, path)
}

func (s *Strategy) fetchProduct(ctx context.Context, pageURL string) (productSnapshot, error) {
	html, err := fetchProductHTML(ctx, pageURL)
	if err != nil {
		return productSnapshot{}, err
	}

	snapshot := productSnapshot{}
	hasCard := false

	if product, ok := extractFromJSONLD(html); ok {
		snapshot = product
		hasCard = snapshot.Title != "" && snapshot.Price > 0
	}

	if htmlProduct, ok := extractFromHTML(html); ok {
		if !hasCard {
			snapshot = htmlProduct
			hasCard = snapshot.Title != "" && snapshot.Price > 0
		} else {
			if snapshot.Title == "" {
				snapshot.Title = htmlProduct.Title
			}
			if snapshot.Price <= 0 {
				snapshot.Price = htmlProduct.Price
			}
		}
	}

	if !hasCard {
		return productSnapshot{}, errProductNotFound
	}

	if imageURL := extractBestImageURL(html); imageURL != "" {
		snapshot.ImageURL = imageURL
	} else if snapshot.ImageURL != "" {
		snapshot.ImageURL = upscaleThumbURL(snapshot.ImageURL)
	}

	return snapshot, nil
}

func fetchProductHTML(ctx context.Context, pageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", dnsHTTPUserAgent)
	req.Header.Set("Accept-Language", "ru")

	resp, err := dnsHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d for %s", resp.StatusCode, pageURL)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func extractFromJSONLD(html string) (productSnapshot, bool) {
	for _, block := range extractJSONLDBlocks(html) {
		if product, ok := parseJSONLDBlock(block); ok {
			return product, true
		}
	}

	return productSnapshot{}, false
}

func parseJSONLDBlock(block string) (productSnapshot, bool) {
	var graph jsonLDGraph
	if err := json.Unmarshal([]byte(block), &graph); err == nil {
		for _, item := range graph.Graph {
			if product, ok := productFromJSONLD(item); ok {
				return product, true
			}
		}
	}

	var product jsonLDProduct
	if err := json.Unmarshal([]byte(block), &product); err != nil {
		return productSnapshot{}, false
	}

	return productFromJSONLD(product)
}

func productFromJSONLD(payload jsonLDProduct) (productSnapshot, bool) {
	if !strings.EqualFold(payload.Type, "Product") {
		return productSnapshot{}, false
	}
	if payload.Name == "" || payload.Offers.Price <= 0 {
		return productSnapshot{}, false
	}

	return productSnapshot{
		Title:    formatTitle(payload.Name, payload.Brand.Name),
		Price:    payload.Offers.Price,
		Currency: strings.TrimSpace(payload.Offers.PriceCurrency),
		ImageURL: firstImageURL(payload.Image),
	}, true
}

func extractFromHTML(html string) (productSnapshot, bool) {
	title := extractH1Title(html)
	price, ok := extractHTMLPrice(html)
	if title == "" || !ok {
		return productSnapshot{}, false
	}

	return productSnapshot{
		Title:    title,
		Price:    price,
		Currency: "",
	}, true
}

func extractBestImageURL(html string) string {
	for _, pattern := range []*regexp.Regexp{
		preloadImagePattern,
		sliderMainLinkPattern,
		ogImagePattern,
	} {
		if imageURL := firstSubmatch(pattern, html); imageURL != "" {
			if strings.Contains(imageURL, "/fit/45/45/") ||
				strings.Contains(imageURL, "/fit/24/24/") {
				return upscaleThumbURL(imageURL)
			}
			return imageURL
		}
	}

	return ""
}

func upscaleThumbURL(imageURL string) string {
	for _, size := range []string{"45/45", "24/24", "155/155", "200/200", "300/300"} {
		from := "/fit/" + size + "/"
		if strings.Contains(imageURL, from) {
			return strings.Replace(imageURL, from, "/fit/500/500/", 1)
		}
	}
	return imageURL
}

func formatTitle(name, brand string) string {
	name = strings.TrimSpace(name)
	brand = strings.TrimSpace(brand)
	if brand == "" || strings.Contains(strings.ToLower(name), strings.ToLower(brand)) {
		return name
	}
	return fmt.Sprintf("%s - Бренд %s", name, brand)
}

func firstImageURL(images jsonLDImage) string {
	for _, imageURL := range images {
		imageURL = strings.TrimSpace(imageURL)
		if imageURL != "" {
			return imageURL
		}
	}
	return ""
}

var jsonLDScriptPattern = regexp.MustCompile(
	`<script type="application/ld\+json">(.*?)</script>`,
)

func extractJSONLDBlocks(html string) []string {
	matches := jsonLDScriptPattern.FindAllStringSubmatch(html, -1)
	blocks := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		blocks = append(blocks, match[1])
	}
	return blocks
}

var (
	h1TitlePattern   = regexp.MustCompile(`<h1[^>]*>([\s\S]*?)</h1>`)
	htmlPricePattern = regexp.MustCompile(
		`product-card-purchase__current-price[^>]*>\s*([\d\s]+(?:[.,]\d+)?)`,
	)
	preloadImagePattern = regexp.MustCompile(
		`<link rel="preload" href="(https://c\.dns-shop\.ru/[^"]+)" as="image">`,
	)
	sliderMainLinkPattern = regexp.MustCompile(
		`js--product-card-slider__image-main-link="(https://c\.dns-shop\.ru/[^"]+)"`,
	)
	ogImagePattern = regexp.MustCompile(
		`<meta property="og:image" content="(https://c\.dns-shop\.ru/[^"]+)"`,
	)
)

func extractH1Title(html string) string {
	match := h1TitlePattern.FindStringSubmatch(html)
	if len(match) < 2 {
		return ""
	}
	title := regexp.MustCompile(`<[^>]+>`).ReplaceAllString(match[1], "")
	return strings.TrimSpace(title)
}

func extractHTMLPrice(html string) (float64, bool) {
	match := htmlPricePattern.FindStringSubmatch(html)
	if len(match) < 2 {
		return 0, false
	}
	raw := strings.ReplaceAll(strings.TrimSpace(match[1]), " ", "")
	raw = strings.ReplaceAll(raw, ",", ".")
	price, err := strconv.ParseFloat(raw, 64)
	if err != nil || price <= 0 {
		return 0, false
	}
	return price, true
}

func firstSubmatch(pattern *regexp.Regexp, html string) string {
	match := pattern.FindStringSubmatch(html)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}
