package detmir

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

	"github.com/ibednov/go-lepsios/currency"
)

var detmirHTTPClient = &http.Client{
	Timeout: 12 * time.Second,
}

const detmirHTTPUserAgent = "Mozilla/5.0 (compatible; WishimiParser/1.0)"

var productIDPathPattern = regexp.MustCompile(`/product/index/id/(\d+)`)

var (
	errUnsupportedHost   = errors.New("unsupported detmir host")
	errInvalidProductURL = errors.New("product url must contain /product/index/id/{id}")
	errProductNotFound   = errors.New("product data not found")
)

func resolveProductRequest(rawURL string) (productID, apiHost string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", err
	}

	productID = extractProductID(u.Path)
	if productID == "" {
		return "", "", fmt.Errorf("%w: %s", errInvalidProductURL, u.Path)
	}

	apiHost, err = resolveAPIHost(u.Hostname())
	if err != nil {
		return "", "", err
	}

	return productID, apiHost, nil
}

func extractProductID(path string) string {
	matches := productIDPathPattern.FindStringSubmatch(path)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

func resolveAPIHost(hostname string) (string, error) {
	host := strings.ToLower(hostname)
	host = strings.TrimPrefix(host, "www.")

	switch host {
	case "detmir.by":
		return "api.detmir.by", nil
	case "detmir.ru":
		return "api.detmir.ru", nil
	case "detmir.kz":
		return "api.detmir.kz", nil
	default:
		return "", fmt.Errorf("%w: %s", errUnsupportedHost, hostname)
	}
}

func (s *Strategy) fetchProduct(
	ctx context.Context,
	pageURL string,
	productID string,
	apiHost string,
) (productSnapshot, error) {
	if product, err := fetchProductFromAPI(ctx, apiHost, productID); err == nil {
		return product, nil
	} else if !errors.Is(err, errProductNotFound) {
		// API недоступен — пробуем HTML.
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

func fetchProductFromAPI(ctx context.Context, apiHost, productID string) (productSnapshot, error) {
	reqURL := fmt.Sprintf("https://%s/v4/products/%s", apiHost, url.PathEscape(productID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return productSnapshot{}, err
	}
	req.Header.Set("User-Agent", detmirHTTPUserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := detmirHTTPClient.Do(req)
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

	return snapshotFromAPIItem(payload.Item, apiHost)
}

func snapshotFromAPIItem(item apiProductItem, apiHost string) (productSnapshot, error) {
	if item.Title == "" {
		return productSnapshot{}, errProductNotFound
	}

	price := item.Price.Price
	if price <= 0 {
		price = item.Prices.Sale
	}
	if price <= 0 {
		return productSnapshot{}, errProductNotFound
	}

	brand := ""
	if len(item.Brands) > 0 {
		brand = item.Brands[0].Title
	}

	return productSnapshot{
		Title:    formatTitle(item.Title, brand),
		Price:    price,
		Currency: currencyForAPIHost(apiHost).String(),
		ImageURL: firstPictureURL(item.Pictures),
	}, nil
}

func fetchProductFromHTML(ctx context.Context, pageURL string) (productSnapshot, bool, error) {
	html, err := fetchProductHTML(ctx, pageURL)
	if err != nil {
		return productSnapshot{}, false, err
	}

	product, ok := extractFromJSONLD(html, pageURL)
	return product, ok, nil
}

func fetchProductHTML(ctx context.Context, pageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", detmirHTTPUserAgent)
	req.Header.Set("Accept-Language", "ru")

	resp, err := detmirHTTPClient.Do(req)
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

func extractFromJSONLD(html string, pageURL string) (productSnapshot, bool) {
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

		cur := currency.BYN
		if _, apiHost, err := resolveProductRequest(pageURL); err == nil {
			cur = currencyForAPIHost(apiHost)
		}

		return productSnapshot{
			Title:    formatTitle(payload.Name, payload.Brand.Name),
			Price:    payload.Offers.Price,
			Currency: cur.String(),
			ImageURL: firstImageURL(payload.Image),
		}, true
	}

	return productSnapshot{}, false
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

func firstPictureURL(pictures []apiPicture) string {
	for _, picture := range pictures {
		if imageURL := strings.TrimSpace(picture.Web); imageURL != "" {
			return imageURL
		}
		if imageURL := strings.TrimSpace(picture.Original); imageURL != "" {
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

var jsonUnmarshal = json.Unmarshal
