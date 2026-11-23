package emall

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ibednov/go-lepsios/log"
	"github.com/ibednov/go-lepsios/productparse/fetchretry"
)

const (
	emallBaseURL       = "https://emall.by/"
	emallHTTPUserAgent = "Mozilla/5.0 (compatible; WishimiParser/1.0)"
	emallFetchAttempts = 3
	emallSessionTTL    = 30 * time.Minute
)

var (
	emallHTTPClient     *http.Client
	emallHTTPClientOnce sync.Once

	errInvalidProductURL = errors.New("invalid emall.by product url")
	errProductNotFound   = errors.New("product data not found in page")
	errAccessDenied      = errors.New("access denied by emall.by")
)

var (
	productPathPattern = regexp.MustCompile(`^/product/(\d+)/?$`)
	ldJSONPattern      = regexp.MustCompile(`<script type="application/ld\+json"[^>]*>([\s\S]*?)</script>`)
	ogTitlePattern     = regexp.MustCompile(`property="og:title"\s+content="([^"]+)"`)
	ogImagePattern     = regexp.MustCompile(`property="og:image"\s+content="([^"]+)"`)
)

type sessionState struct {
	client    *http.Client
	createdAt time.Time
}

var (
	sessionMu sync.Mutex
	session   sessionState
)

func emallClient() *http.Client {
	emallHTTPClientOnce.Do(func() {
		jar, err := cookiejar.New(nil)
		if err != nil {
			panic(fmt.Sprintf("emall cookie jar: %v", err))
		}
		emallHTTPClient = &http.Client{
			Timeout: 12 * time.Second,
			Jar:     jar,
		}
	})
	return emallHTTPClient
}

func validateProductURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}

	if !productPathPattern.MatchString(u.Path) {
		return fmt.Errorf("%w: path must match /product/{id}, got %s", errInvalidProductURL, u.Path)
	}

	return nil
}

func productIDFromURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	match := productPathPattern.FindStringSubmatch(u.Path)
	if len(match) < 2 {
		return "", fmt.Errorf("%w: %s", errInvalidProductURL, u.Path)
	}

	return match[1], nil
}

func (s *Strategy) fetchProduct(ctx context.Context, pageURL string) (productSnapshot, error) {
	expectedID, err := productIDFromURL(pageURL)
	if err != nil {
		return productSnapshot{}, err
	}

	retryNumber := 0
	for attempt := 0; attempt < emallFetchAttempts; attempt++ {
		htmlBody, fetchErr := fetchProductHTML(ctx, pageURL)
		if fetchErr != nil {
			if fetchretry.Retryable(fetchErr) && attempt+1 < emallFetchAttempts {
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

		if product, ok := parseProductFromHTML(htmlBody, expectedID); ok {
			return product, nil
		}
		logParseFailure(pageURL, htmlBody)
		return productSnapshot{}, errProductNotFound
	}

	return productSnapshot{}, errProductNotFound
}

func fetchProductHTML(ctx context.Context, pageURL string) (string, error) {
	if err := ensureSession(ctx); err != nil {
		return "", err
	}

	client := activeSessionClient()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", emallHTTPUserAgent)
	req.Header.Set("Accept-Language", "ru")
	req.Header.Set("Referer", emallBaseURL)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusForbidden {
		invalidateSession()
		return "", fmt.Errorf("%w: status 403 for %s", errAccessDenied, pageURL)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fetchretry.NewHTTPStatusError(resp, pageURL)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}

	htmlBody := string(body)
	if isAccessDeniedPage(htmlBody) {
		invalidateSession()
		return "", fmt.Errorf("%w: blocked page for %s", errAccessDenied, pageURL)
	}

	return htmlBody, nil
}

func ensureSession(ctx context.Context) error {
	sessionMu.Lock()
	defer sessionMu.Unlock()

	if session.client != nil && time.Since(session.createdAt) < emallSessionTTL {
		return nil
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return err
	}

	client := &http.Client{
		Timeout: 12 * time.Second,
		Jar:     jar,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, emallBaseURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", emallHTTPUserAgent)
	req.Header.Set("Accept-Language", "ru")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fetchretry.NewHTTPStatusError(resp, emallBaseURL)
	}

	session = sessionState{
		client:    client,
		createdAt: time.Now(),
	}
	return nil
}

func activeSessionClient() *http.Client {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	if session.client != nil {
		return session.client
	}
	return emallClient()
}

func invalidateSession() {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	session = sessionState{}
}

func isAccessDeniedPage(htmlBody string) bool {
	return strings.Contains(htmlBody, "Access denied") ||
		strings.Contains(htmlBody, "Запрос заблокирован")
}

func parseProductFromHTML(htmlBody, expectedID string) (productSnapshot, bool) {
	if product, ok := extractFromJSONLD(htmlBody, expectedID); ok {
		return product, true
	}
	if product, ok := extractFromOGFallback(htmlBody); ok {
		return product, true
	}
	return productSnapshot{}, false
}

func extractFromJSONLD(htmlBody, expectedID string) (productSnapshot, bool) {
	for _, match := range ldJSONPattern.FindAllStringSubmatch(htmlBody, -1) {
		if len(match) < 2 {
			continue
		}
		if product, ok := parseJSONLDBlock(match[1], expectedID); ok {
			return product, true
		}
	}
	return productSnapshot{}, false
}

func parseJSONLDBlock(raw, expectedID string) (productSnapshot, bool) {
	var payload any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return productSnapshot{}, false
	}
	return findProductInJSONLD(payload, expectedID)
}

func findProductInJSONLD(node any, expectedID string) (productSnapshot, bool) {
	switch v := node.(type) {
	case map[string]any:
		if isProductNode(v) && productMatchesID(v, expectedID) {
			return snapshotFromProductMap(v)
		}
		if graph, ok := v["@graph"].([]any); ok {
			for _, item := range graph {
				if product, ok := findProductInJSONLD(item, expectedID); ok {
					return product, true
				}
			}
		}
	case []any:
		for _, item := range v {
			if product, ok := findProductInJSONLD(item, expectedID); ok {
				return product, true
			}
		}
	}
	return productSnapshot{}, false
}

func isProductNode(v map[string]any) bool {
	typeValue, _ := v["@type"].(string)
	return strings.EqualFold(typeValue, "Product")
}

func productMatchesID(v map[string]any, expectedID string) bool {
	if expectedID == "" {
		return true
	}

	for _, key := range []string{"sku", "productID", "mpn"} {
		if value, ok := v[key].(string); ok && value == expectedID {
			return true
		}
	}

	if idValue, ok := v["@id"].(string); ok {
		return strings.Contains(idValue, "/product/"+expectedID)
	}

	return true
}

func snapshotFromProductMap(v map[string]any) (productSnapshot, bool) {
	title, _ := v["name"].(string)
	title = strings.TrimSpace(title)
	if title == "" {
		return productSnapshot{}, false
	}

	price, currencyCode, ok := priceFromOffers(v["offers"])
	if !ok {
		return productSnapshot{}, false
	}

	return productSnapshot{
		Title:    title,
		Price:    price,
		Currency: currencyCode,
		ImageURL: imageFromNode(v["image"]),
	}, true
}

func priceFromOffers(raw any) (float64, string, bool) {
	switch offers := raw.(type) {
	case map[string]any:
		return parseOfferMap(offers)
	case []any:
		for _, item := range offers {
			if offerMap, ok := item.(map[string]any); ok {
				if price, currencyCode, ok := parseOfferMap(offerMap); ok {
					return price, currencyCode, true
				}
			}
		}
	}
	return 0, "", false
}

func parseOfferMap(offer map[string]any) (float64, string, bool) {
	currencyCode, _ := offer["priceCurrency"].(string)
	currencyCode = strings.TrimSpace(currencyCode)
	if currencyCode == "" {
		currencyCode = "BYN"
	}

	for _, key := range []string{"price", "lowPrice"} {
		switch value := offer[key].(type) {
		case string:
			price, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if err == nil && price > 0 {
				return price, currencyCode, true
			}
		case float64:
			if value > 0 {
				return value, currencyCode, true
			}
		}
	}

	return 0, "", false
}

func imageFromNode(raw any) string {
	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value)
	case map[string]any:
		if thumb, ok := value["thumbnailUrl"].(string); ok && thumb != "" {
			return strings.TrimSpace(thumb)
		}
		if imgURL, ok := value["url"].(string); ok {
			return strings.TrimSpace(imgURL)
		}
	case []any:
		for _, item := range value {
			if img := imageFromNode(item); img != "" {
				return img
			}
		}
	}
	return ""
}

func extractFromOGFallback(htmlBody string) (productSnapshot, bool) {
	title := extractOGTitle(htmlBody)
	price, ok := extractPriceFromHTML(htmlBody)
	if title == "" || !ok {
		return productSnapshot{}, false
	}

	return productSnapshot{
		Title:    title,
		Price:    price,
		Currency: "BYN",
		ImageURL: extractOGImage(htmlBody),
	}, true
}

func extractPriceFromHTML(htmlBody string) (float64, bool) {
	pattern := regexp.MustCompile(`"price"\s*:\s*"([\d.]+)"`)
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

func extractOGTitle(htmlBody string) string {
	match := ogTitlePattern.FindStringSubmatch(htmlBody)
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

func logParseFailure(pageURL, htmlBody string) {
	log.Warn("parser.emall.parse_failed",
		"component", "parser",
		"strategy", "emall",
		"url", pageURL,
		"html_bytes", len(htmlBody),
		"has_json_ld", ldJSONPattern.MatchString(htmlBody),
		"has_og_title", ogTitlePattern.MatchString(htmlBody),
	)
}
