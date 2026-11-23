package vek21

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var vek21HTTPClient = &http.Client{
	Timeout: 12 * time.Second,
}

const vek21HTTPUserAgent = "Mozilla/5.0 (compatible; WishimiParser/1.0)"

var productPathPattern = regexp.MustCompile(`^/([^/]+)/([^/]+)\.html$`)

var (
	errInvalidProductPath = errors.New("product path must match /{category}/{alias}.html")
	errProductNotFound    = errors.New("product data not found in page")
)

func validateProductURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if !productPathPattern.MatchString(u.Path) {
		return fmt.Errorf("%w: %s", errInvalidProductPath, u.Path)
	}
	return nil
}

func (s *Strategy) fetchProduct(ctx context.Context, pageURL string) (productSnapshot, error) {
	html, err := fetchProductHTML(ctx, pageURL)
	if err != nil {
		return productSnapshot{}, err
	}

	if product, ok := extractFromJSONLD(html); ok {
		return product, nil
	}

	if product, ok := extractFromNextData(html); ok {
		return product, nil
	}

	return productSnapshot{}, errProductNotFound
}

func fetchProductHTML(ctx context.Context, pageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", vek21HTTPUserAgent)
	req.Header.Set("Accept-Language", "ru")

	resp, err := vek21HTTPClient.Do(req)
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
		var payload jsonLDProduct
		if err := json.Unmarshal([]byte(block), &payload); err != nil {
			continue
		}
		if !strings.EqualFold(payload.Type, "Product") {
			continue
		}
		if payload.Name == "" || payload.Offers.Price <= 0 {
			continue
		}

		return productSnapshot{
			Title:    formatTitle(payload.Name, payload.Brand.Name),
			Price:    payload.Offers.Price,
			Currency: strings.TrimSpace(payload.Offers.PriceCurrency),
			ImageURL: firstImageURL(payload.Image),
		}, true
	}

	return productSnapshot{}, false
}

func extractFromNextData(html string) (productSnapshot, bool) {
	raw := extractNextDataJSON(html)
	if raw == "" {
		return productSnapshot{}, false
	}

	var root nextDataRoot
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		return productSnapshot{}, false
	}
	if root.Props.PageProps.InitialState == "" {
		return productSnapshot{}, false
	}

	var state initialStateRoot
	if err := json.Unmarshal([]byte(root.Props.PageProps.InitialState), &state); err != nil {
		return productSnapshot{}, false
	}

	data := state.ProductCard.FullProductData
	if data.Name == "" {
		return productSnapshot{}, false
	}

	price := data.Prices.SalePrice
	if price <= 0 {
		price = data.Prices.Price
	}
	if price <= 0 {
		return productSnapshot{}, false
	}

	return productSnapshot{
		Title:    formatTitle(data.Name, data.Producer.Name),
		Price:    price,
		Currency: "",
		ImageURL: firstGalleryImageURL(data.Gallery),
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

func firstImageURL(images jsonLDImage) string {
	for _, imageURL := range images {
		imageURL = strings.TrimSpace(imageURL)
		if imageURL != "" {
			return imageURL
		}
	}
	return ""
}

func firstGalleryImageURL(items []nextGalleryItem) string {
	for _, item := range items {
		if item.Type != "image" {
			continue
		}
		if imageURL := strings.TrimSpace(item.Preview); imageURL != "" {
			return imageURL
		}
		if imageURL := strings.TrimSpace(item.FullSize); imageURL != "" {
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

var nextDataPattern = regexp.MustCompile(
	`<script id="__NEXT_DATA__" type="application/json">(.*?)</script>`,
)

func extractNextDataJSON(html string) string {
	match := nextDataPattern.FindStringSubmatch(html)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

// jsonUnmarshal вынесен для тестов jsonLDImage без циклического импорта encoding/json в model.
var jsonUnmarshal = json.Unmarshal
