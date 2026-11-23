package onliner

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

const catalogAPIBase = "https://catalog.onliner.by/sdapi/catalog.api/products/"

var onlinerHTTPClient = &http.Client{
	Timeout: 12 * time.Second,
}

const onlinerHTTPUserAgent = "Mozilla/5.0 (compatible; WishimiParser/1.0)"

var (
	reservedPathRoots = map[string]struct{}{
		"cat":     {},
		"compare": {},
		"promo":   {},
		"search":  {},
	}

	productPageSuffixes = map[string]struct{}{
		"prices":     {},
		"reviews":    {},
		"discussion": {},
	}

	lowPriceJSONLDPattern = regexp.MustCompile(`"lowPrice"\s*:\s*"([\d.]+)"`)
	ogImagePattern        = regexp.MustCompile(`property="og:image"\s+content="([^"]+)"`)
	ogTitlePattern        = regexp.MustCompile(`property="og:title"\s+content="([^"]+)"`)
)

var (
	errInvalidProductURL = errors.New("invalid onliner catalog product url")
	errProductNotFound   = errors.New("product data not found")
)

func resolveProductKey(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	productKey := extractProductKey(u.Path)
	if productKey == "" {
		return "", fmt.Errorf("%w: %s", errInvalidProductURL, u.Path)
	}

	return productKey, nil
}

func extractProductKey(path string) string {
	path = strings.Trim(path, "/")
	if path == "" {
		return ""
	}

	parts := strings.Split(path, "/")
	if len(parts) < 3 {
		return ""
	}

	if _, reserved := reservedPathRoots[parts[0]]; reserved {
		return ""
	}

	last := parts[len(parts)-1]
	if _, suffix := productPageSuffixes[last]; suffix {
		if len(parts) < 3 {
			return ""
		}
		last = parts[len(parts)-2]
	}

	return last
}

func (s *Strategy) fetchProduct(ctx context.Context, pageURL, productKey string) (productSnapshot, error) {
	if product, err := fetchProductFromAPI(ctx, productKey); err == nil {
		return product, nil
	} else if !errors.Is(err, errProductNotFound) {
		if product, ok, htmlErr := fetchProductFromHTML(ctx, pageURL); htmlErr == nil && ok {
			return product, nil
		}
		return productSnapshot{}, err
	}

	if product, ok, err := fetchProductFromHTML(ctx, pageURL); err != nil {
		return productSnapshot{}, err
	} else if ok {
		return product, nil
	}

	return productSnapshot{}, errProductNotFound
}

func fetchProductFromAPI(ctx context.Context, productKey string) (productSnapshot, error) {
	reqURL := catalogAPIBase + url.PathEscape(productKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return productSnapshot{}, err
	}
	req.Header.Set("User-Agent", onlinerHTTPUserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := onlinerHTTPClient.Do(req)
	if err != nil {
		return productSnapshot{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return productSnapshot{}, fmt.Errorf("api status %d for %s", resp.StatusCode, reqURL)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return productSnapshot{}, err
	}

	var payload apiProductResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return productSnapshot{}, err
	}

	return snapshotFromAPI(payload)
}

func snapshotFromAPI(payload apiProductResponse) (productSnapshot, error) {
	title := strings.TrimSpace(payload.ExtendedName)
	if title == "" {
		title = strings.TrimSpace(payload.FullName)
	}
	if title == "" {
		title = strings.TrimSpace(payload.Name)
	}
	if title == "" {
		return productSnapshot{}, errProductNotFound
	}

	// Catalog products may ship without offers (prices: null) — title/image still usable.
	price, _ := parseAmount(payload.Prices.PriceMin.Amount)
	if price < 0 {
		price = 0
	}

	return productSnapshot{
		Title:    title,
		Price:    price,
		Currency: strings.TrimSpace(payload.Prices.PriceMin.Currency),
		ImageURL: strings.TrimSpace(payload.Images.Header),
	}, nil
}

func fetchProductFromHTML(ctx context.Context, pageURL string) (productSnapshot, bool, error) {
	html, err := fetchProductHTML(ctx, pageURL)
	if err != nil {
		return productSnapshot{}, false, err
	}

	product, ok := extractFromHTML(html)
	return product, ok, nil
}

func extractFromHTML(html string) (productSnapshot, bool) {
	for _, block := range extractJSONLDBlocks(html) {
		var payload jsonLDProduct
		if err := json.Unmarshal([]byte(block), &payload); err != nil {
			continue
		}
		if !strings.EqualFold(payload.Type, "Product") {
			continue
		}

		priceStr := payload.Offers.LowPrice
		if priceStr == "" {
			priceStr = payload.Offers.Price
		}
		price, _ := parseAmount(priceStr)
		if payload.Name == "" {
			continue
		}
		if price < 0 {
			price = 0
		}

		return productSnapshot{
			Title:    strings.TrimSpace(payload.Name),
			Price:    price,
			Currency: "",
			ImageURL: firstImageURL(payload.Image),
		}, true
	}

	title := ""
	if match := ogTitlePattern.FindStringSubmatch(html); len(match) > 1 {
		title = strings.TrimSpace(match[1])
	}

	price := 0.0
	if match := lowPriceJSONLDPattern.FindStringSubmatch(html); len(match) > 1 {
		if parsed, err := parseAmount(match[1]); err == nil && parsed > 0 {
			price = parsed
		}
	}

	imageURL := ""
	if match := ogImagePattern.FindStringSubmatch(html); len(match) > 1 {
		imageURL = strings.TrimSpace(match[1])
	}

	if title == "" {
		return productSnapshot{}, false
	}

	return productSnapshot{
		Title:    title,
		Price:    price,
		Currency: "",
		ImageURL: imageURL,
	}, true
}

func fetchProductHTML(ctx context.Context, pageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", onlinerHTTPUserAgent)
	req.Header.Set("Accept-Language", "ru")

	resp, err := onlinerHTTPClient.Do(req)
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

func parseAmount(raw string) (float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, errors.New("empty amount")
	}
	return strconv.ParseFloat(strings.ReplaceAll(raw, ",", "."), 64)
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
	`<script[^>]*type="application/ld\+json"[^>]*>(.*?)</script>`,
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

var jsonUnmarshal = json.Unmarshal
