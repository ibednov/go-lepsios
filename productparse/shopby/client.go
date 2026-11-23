package shopby

import (
	"context"
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

var shopbyHTTPClient = &http.Client{
	Timeout: 12 * time.Second,
}

const (
	shopbyHTTPUserAgent = "Mozilla/5.0 (compatible; WishimiParser/1.0)"
	shopbyFetchAttempts = 3
)

var (
	errInvalidProductURL = errors.New("invalid shop.by product url")
	errProductNotFound   = errors.New("product data not found in page")
)

var reservedPathRoots = []string{
	"news",
	"cart",
	"order",
	"shop_page",
	"office",
	"search",
	"compare",
	"account",
	"login",
	"register",
	"to.php",
	"item.php",
}

var (
	modelPagePattern = regexp.MustCompile(`data-page=["']model["']`)
	dataTitlePattern = regexp.MustCompile(`data-title=["']([^"']+)["']`)
	h1NamePattern    = regexp.MustCompile(
		`<h1[^>]*itemprop=["']name["'][^>]*>([\s\S]*?)</h1>`,
	)
	// Legacy microdata (pre-2026 shop.by markup).
	lowPricePattern = regexp.MustCompile(
		`itemprop=["']lowPrice["']\s+content=["']([\d.]+)["']`,
	)
	lowPriceAltPattern = regexp.MustCompile(
		`content=["']([\d.]+)["'][^>]*itemprop=["']lowPrice["']`,
	)
	priceCurrencyPattern = regexp.MustCompile(
		`itemprop=["']priceCurrency["']\s+content=["']([A-Z]{3})["']`,
	)
	// Current markup: AggregateOffer lives in application/ld+json (@graph Product).
	// lowPrice may be a JSON number (590) or a quoted string ("590.00").
	lowPriceJSONLDPattern = regexp.MustCompile(
		`"lowPrice"\s*:\s*"?([\d.]+)"?`,
	)
	priceCurrencyJSONLDPattern = regexp.MustCompile(
		`"priceCurrency"\s*:\s*"([A-Z]{3})"`,
	)
	ogTitlePattern = regexp.MustCompile(
		`<meta\s+property=["']og:title["']\s+content=["']([^"']+)["']`,
	)
	ogImagePattern = regexp.MustCompile(
		`<meta\s+property=["']og:image["']\s+content=["']([^"']+)["']`,
	)
)

func validateProductURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}

	path := strings.Trim(u.Path, "/")
	if path == "" {
		return fmt.Errorf("%w: empty path", errInvalidProductURL)
	}

	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return fmt.Errorf("%w: expected category/product path, got %s", errInvalidProductURL, u.Path)
	}

	root := strings.ToLower(parts[0])
	if slices.Contains(reservedPathRoots, root) {
		return fmt.Errorf("%w: reserved path root %s", errInvalidProductURL, root)
	}

	return nil
}

func (s *Strategy) fetchProduct(ctx context.Context, pageURL string) (productSnapshot, error) {
	retryNumber := 0
	for attempt := 0; attempt < shopbyFetchAttempts; attempt++ {
		htmlBody, fetchErr := fetchProductHTML(ctx, pageURL)
		if fetchErr != nil {
			if fetchretry.Retryable(fetchErr) && attempt+1 < shopbyFetchAttempts {
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

		if product, ok := parseProductFromHTML(htmlBody); ok {
			return product, nil
		}
		logParseFailure(pageURL, htmlBody)
		return productSnapshot{}, errProductNotFound
	}

	return productSnapshot{}, errProductNotFound
}

func fetchProductHTML(ctx context.Context, pageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", shopbyHTTPUserAgent)
	req.Header.Set("Accept-Language", "ru")

	resp, err := shopbyHTTPClient.Do(req)
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

func parseProductFromHTML(htmlBody string) (productSnapshot, bool) {
	if !modelPagePattern.MatchString(htmlBody) {
		return productSnapshot{}, false
	}

	title := extractTitle(htmlBody)
	price, ok := extractLowPrice(htmlBody)
	if title == "" || !ok {
		return productSnapshot{}, false
	}

	return productSnapshot{
		Title:    title,
		Price:    price,
		Currency: extractPriceCurrency(htmlBody),
		ImageURL: extractOGImage(htmlBody),
	}, true
}

func extractTitle(htmlBody string) string {
	if match := h1NamePattern.FindStringSubmatch(htmlBody); len(match) >= 2 {
		title := stripHTML(match[1])
		if title != "" {
			return title
		}
	}

	if match := dataTitlePattern.FindStringSubmatch(htmlBody); len(match) >= 2 {
		title := strings.TrimSpace(html.UnescapeString(match[1]))
		if title != "" {
			return title
		}
	}

	if match := ogTitlePattern.FindStringSubmatch(htmlBody); len(match) >= 2 {
		return normalizeOGTitle(match[1])
	}

	return ""
}

func extractLowPrice(htmlBody string) (float64, bool) {
	for _, pattern := range []*regexp.Regexp{
		lowPriceJSONLDPattern,
		lowPricePattern,
		lowPriceAltPattern,
	} {
		match := pattern.FindStringSubmatch(htmlBody)
		if len(match) < 2 {
			continue
		}
		price, err := strconv.ParseFloat(strings.TrimSpace(match[1]), 64)
		if err == nil && price > 0 {
			return price, true
		}
	}
	return 0, false
}

func extractPriceCurrency(htmlBody string) string {
	for _, pattern := range []*regexp.Regexp{
		priceCurrencyJSONLDPattern,
		priceCurrencyPattern,
	} {
		match := pattern.FindStringSubmatch(htmlBody)
		if len(match) >= 2 {
			return strings.TrimSpace(match[1])
		}
	}
	return "BYN"
}

func extractOGImage(htmlBody string) string {
	match := ogImagePattern.FindStringSubmatch(htmlBody)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func normalizeOGTitle(title string) string {
	title = strings.TrimSpace(html.UnescapeString(title))
	if title == "" {
		return ""
	}

	for _, suffix := range []string{" купить недорого", " – Shop.by", " - Shop.by"} {
		if idx := strings.Index(title, suffix); idx > 0 {
			return strings.TrimSpace(title[:idx])
		}
	}

	return title
}

func stripHTML(raw string) string {
	raw = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(raw, "")
	return strings.TrimSpace(html.UnescapeString(raw))
}

func logParseFailure(pageURL, htmlBody string) {
	hasLowPrice := lowPriceJSONLDPattern.MatchString(htmlBody) ||
		lowPricePattern.MatchString(htmlBody) ||
		lowPriceAltPattern.MatchString(htmlBody)
	log.Warn("parser.shopby.parse_failed",
		"component", "parser",
		"strategy", "shopby",
		"url", pageURL,
		"html_bytes", len(htmlBody),
		"has_model_page", modelPagePattern.MatchString(htmlBody),
		"has_low_price", hasLowPrice,
	)
}
