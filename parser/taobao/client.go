package taobao

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/ibednov/go-lepsios/log"
)

const (
	httpUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36"
	httpTimeout   = 15 * time.Second
)

func newHTTPClient(proxyRaw string) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxyRaw = strings.TrimSpace(proxyRaw); proxyRaw != "" {
		proxyURL, err := url.Parse(proxyRaw)
		if err != nil || proxyURL.Scheme == "" || proxyURL.Host == "" {
			log.Error("parser.taobao.invalid_proxy",
				"component", "parser",
				"strategy", "taobao",
				"error", fmt.Sprintf("invalid PARSER_TAOBAO_HTTP_PROXY: %s", proxyRaw),
			)
		} else {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		jar = nil
	}
	return &http.Client{Timeout: httpTimeout, Transport: transport, Jar: jar}
}

func (s *Strategy) client() *http.Client {
	if s != nil && s.httpClient != nil {
		return s.httpClient
	}
	return newHTTPClient("")
}

func (s *Strategy) fetchProduct(ctx context.Context, pageURL string) (productSnapshot, error) {
	itemID, fallbackPrice, err := s.resolveItemID(ctx, pageURL)
	if err != nil {
		return productSnapshot{}, err
	}

	s.warmup(ctx)

	pages := make([]string, 0, 2)
	sawBlock := false
	for _, cardURL := range cardSourceURLs(itemID) {
		htmlBody, _, fetchErr := s.fetchHTML(ctx, cardURL, pageURL)
		if fetchErr != nil {
			log.Warn("parser.taobao.card_fetch_failed",
				"component", "parser",
				"strategy", "taobao",
				"url", pageURL,
				"card_url", cardURL,
				"item_id", itemID,
				"error", fetchErr.Error(),
			)
			continue
		}
		parsed := parseCardHTML(htmlBody, itemID)
		if parsed.Title != "" || parsed.Price > 0 {
			pages = append(pages, htmlBody)
			if parsed.Title != "" && parsed.Price > 0 {
				break
			}
			continue
		}
		if isBlockedCardHTML(htmlBody) {
			sawBlock = true
			log.Warn("parser.taobao.card_blocked",
				"component", "parser",
				"strategy", "taobao",
				"url", pageURL,
				"card_url", cardURL,
				"item_id", itemID,
				"html_bytes", len(htmlBody),
			)
			continue
		}
		pages = append(pages, htmlBody)
	}

	product, err := assembleProduct(itemID, fallbackPrice, pages, sawBlock)
	if err != nil {
		log.Warn("parser.taobao.parse_failed",
			"component", "parser",
			"strategy", "taobao",
			"url", pageURL,
			"item_id", itemID,
			"fallback_price", fallbackPrice,
			"pages", len(pages),
			"blocked", sawBlock,
		)
		return productSnapshot{}, err
	}
	return product, nil
}

func (s *Strategy) resolveItemID(ctx context.Context, pageURL string) (itemID string, price float64, err error) {
	u, err := url.Parse(pageURL)
	if err != nil {
		return "", 0, fmt.Errorf("%w: %v", errInvalidProductURL, err)
	}
	if id, p := itemIDAndPriceFromURL(u); id != "" {
		return id, p, nil
	}

	host := strings.ToLower(strings.TrimPrefix(u.Hostname(), "www."))
	if !isShortHost(host) {
		return "", 0, fmt.Errorf("%w: cannot extract item id from %s", errInvalidProductURL, u.Path)
	}

	htmlBody, finalURL, err := s.fetchHTML(ctx, pageURL, "")
	if err != nil {
		return "", 0, err
	}
	if id, shortPrice, ok := itemIDFromShortHTML(htmlBody); ok {
		return id, shortPrice, nil
	}
	if final, ferr := url.Parse(finalURL); ferr == nil {
		if id, p := itemIDAndPriceFromURL(final); id != "" {
			return id, p, nil
		}
	}
	return "", 0, fmt.Errorf("%w: short link has no item id", errInvalidProductURL)
}

func itemIDAndPriceFromURL(u *url.URL) (string, float64) {
	id := itemIDFromURL(u)
	if id == "" {
		return "", 0
	}
	raw := strings.TrimSpace(u.Query().Get("price"))
	if raw == "" {
		return id, 0
	}
	parsed, err := parsePositiveFloat(raw)
	if err != nil {
		return id, 0
	}
	return id, parsed
}

func (s *Strategy) warmup(ctx context.Context) {
	_, _, _ = s.fetchHTML(ctx, "https://www.taobao.com/", "")
}

func (s *Strategy) fetchHTML(ctx context.Context, pageURL, referer string) (body string, finalURL string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", httpUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	if referer = strings.TrimSpace(referer); referer != "" {
		req.Header.Set("Referer", referer)
	}

	resp, err := s.client().Do(req)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = resp.Body.Close() }()

	finalURL = pageURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}

	if resp.StatusCode != http.StatusOK {
		return "", finalURL, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, pageURL)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", finalURL, err
	}
	return string(raw), finalURL, nil
}
