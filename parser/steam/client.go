package steam

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

const (
	steamStoreAPIBase = "https://store.steampowered.com/api/appdetails"
	steamCountryCode  = "us"
	steamLanguage     = "english"
)

var steamHTTPClient = &http.Client{
	Timeout: 12 * time.Second,
}

const steamHTTPUserAgent = "Mozilla/5.0 (compatible; WishimiParser/1.0)"

var appPathPattern = regexp.MustCompile(`^/app/(\d+)(?:/|$)`)

var (
	errInvalidProductPath = errors.New("product path must match /app/{appId}/")
	errProductNotFound    = errors.New("product data not found")
)

func validateProductURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if extractAppID(u.Path) == "" {
		return fmt.Errorf("%w: %s", errInvalidProductPath, u.Path)
	}
	return nil
}

func extractAppID(path string) string {
	matches := appPathPattern.FindStringSubmatch(path)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

func (s *Strategy) fetchProduct(ctx context.Context, pageURL string) (productSnapshot, error) {
	u, err := url.Parse(pageURL)
	if err != nil {
		return productSnapshot{}, err
	}

	appID := extractAppID(u.Path)
	if appID == "" {
		return productSnapshot{}, errInvalidProductPath
	}

	return fetchProductFromAPI(ctx, appID)
}

func fetchProductFromAPI(ctx context.Context, appID string) (productSnapshot, error) {
	reqURL := fmt.Sprintf(
		"%s?appids=%s&cc=%s&l=%s",
		steamStoreAPIBase,
		url.QueryEscape(appID),
		steamCountryCode,
		steamLanguage,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return productSnapshot{}, err
	}
	req.Header.Set("User-Agent", steamHTTPUserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := steamHTTPClient.Do(req)
	if err != nil {
		return productSnapshot{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return productSnapshot{}, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, reqURL)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return productSnapshot{}, err
	}

	var payload appDetailsResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return productSnapshot{}, err
	}

	entry, ok := payload[appID]
	if !ok || !entry.Success || entry.Data == nil {
		return productSnapshot{}, errProductNotFound
	}

	return snapshotFromAppDetails(*entry.Data)
}

func snapshotFromAppDetails(data appDetailsData) (productSnapshot, error) {
	title := strings.TrimSpace(data.Name)
	if title == "" {
		return productSnapshot{}, errProductNotFound
	}

	var price float64
	if data.IsFree {
		price = 0
	} else if data.PriceOverview != nil && data.PriceOverview.Final > 0 {
		price = float64(data.PriceOverview.Final) / 100
	} else {
		return productSnapshot{}, errProductNotFound
	}

	cur := currency.USD
	if data.PriceOverview != nil && strings.TrimSpace(data.PriceOverview.Currency) != "" {
		cur = currency.Normalize(data.PriceOverview.Currency, currency.USD)
	}

	return productSnapshot{
		Title:    title,
		Price:    price,
		Currency: cur.String(),
		ImageURL: strings.TrimSpace(data.HeaderImage),
	}, nil
}
