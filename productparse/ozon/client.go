package ozon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ibednov/go-lepsios/log"
	"github.com/ibednov/go-lepsios/productparse/fetchretry"
)

const (
	ozonHTTPUserAgent   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	ozonFetchAttempts   = 3
	ozonSessionTTL      = 30 * time.Minute
	ozonMaxRedirectHops = 12
)

var (
	errInvalidProductURL   = errors.New("invalid ozon product url")
	errProductNotFound     = errors.New("product data not found")
	errAccessDenied        = errors.New("access denied by ozon antibot")
	errResponseUnparseable = errors.New("ozon response could not be parsed")
	errBrowserUnavailable  = errors.New("ozon browser sidecar is not configured")
)

var (
	productPathPattern   = regexp.MustCompile(`^/product/[^/]+/?$`)
	shortLinkPathPattern = regexp.MustCompile(`^/t/[A-Za-z0-9]+/?$`)
	canonicalLinkPattern = regexp.MustCompile(`<link[^>]+rel=["']canonical["'][^>]+href=["']([^"']+)["']`)
	htmlLDJSONPattern    = regexp.MustCompile(`<script type="application/ld\+json"[^>]*>([\s\S]*?)</script>`)
)

type hostSession struct {
	client    *http.Client
	baseURL   string
	createdAt time.Time
}

var (
	sessionMu sync.Mutex
	sessions  = map[string]hostSession{}
)

func validateProductURL(rawURL string) error {
	if _, _, err := ozonPaths(rawURL); err != nil {
		return err
	}
	return nil
}

func ozonPaths(rawURL string) (baseURL, path string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", err
	}

	host := strings.ToLower(u.Hostname())
	if !strings.HasSuffix(host, "ozon.by") && !strings.HasSuffix(host, "ozon.ru") {
		return "", "", fmt.Errorf("%w: unsupported host %s", errInvalidProductURL, host)
	}

	path = u.Path
	if !productPathPattern.MatchString(path) && !shortLinkPathPattern.MatchString(path) {
		return "", "", fmt.Errorf(
			"%w: path must match /product/{slug} or /t/{code}, got %s",
			errInvalidProductURL,
			path,
		)
	}

	scheme := u.Scheme
	if scheme == "" {
		scheme = "https"
	}
	baseURL = fmt.Sprintf("%s://%s/", scheme, u.Hostname())
	return baseURL, normalizeComposerPath(path), nil
}

func normalizeComposerPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return path
}

func composerAPIURL(baseURL, composerPath string) string {
	return strings.TrimSuffix(baseURL, "/") + "/api/composer-api.bx/page/json/v2?url=" + url.QueryEscape(composerPath)
}

func (s *Strategy) fetchProduct(ctx context.Context, pageURL string) (productSnapshot, error) {
	baseURL, composerPath, err := resolveComposerTarget(ctx, pageURL)
	if err != nil {
		return productSnapshot{}, err
	}

	apiURL := composerAPIURL(baseURL, composerPath)
	if product, err := fetchProductTLS(ctx, pageURL, apiURL); err == nil {
		return product, nil
	} else if !errors.Is(err, errChromeTLSUnavailable) {
		log.Warn("parser.ozon.tls_miss",
			"component", "parser",
			"strategy", "ozon",
			"url", pageURL,
			"error", err.Error(),
		)
		if !fetchretry.Retryable(err) &&
			!(errors.Is(err, errAccessDenied) && s != nil && s.pages != nil) {
			return productSnapshot{}, err
		}
	}

	var lastErr error

	if s.pages != nil {
		if product, err := s.fetchProductBrowser(ctx, pageURL, apiURL); err == nil {
			return product, nil
		} else {
			lastErr = err
		}
	} else {
		retryNumber := 0
		for attempt := 0; attempt < ozonFetchAttempts; attempt++ {
			body, fetchErr := fetchComposerJSON(ctx, baseURL, pageURL, apiURL)
			if fetchErr != nil {
				lastErr = fetchErr
				if fetchretry.Retryable(fetchErr) && attempt+1 < ozonFetchAttempts {
					invalidateSession(baseURL)
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
				break
			} else if !isChallengeResponse(body) {
				if product, parseErr := parseComposerPage(body); parseErr == nil {
					return product, nil
				} else {
					lastErr = parseErr
				}
			} else {
				lastErr = errAccessDenied
				invalidateSession(baseURL)
			}

			if htmlBody, htmlErr := fetchProductHTML(ctx, baseURL, pageURL); htmlErr == nil && !isChallengeResponse(htmlBody) {
				if product, ok := parseProductFromHTML(htmlBody); ok {
					return product, nil
				}
			}

			// A successful response with an unsupported/challenge payload or an
			// unparseable document is deterministic for this request. The HTML
			// fallback above gets one chance; another fetch would only repeat it.
			break
		}
	}

	if lastErr != nil {
		logParseFailure(pageURL, lastErr)
		return productSnapshot{}, lastErr
	}
	return productSnapshot{}, errProductNotFound
}

func (s *Strategy) fetchProductBrowser(ctx context.Context, pageURL, apiURL string) (productSnapshot, error) {
	if s == nil || s.pages == nil {
		return productSnapshot{}, errBrowserUnavailable
	}

	snap, err := s.pages.FetchInPage(ctx, pageURL, apiURL)
	if err != nil {
		log.Warn("parser.ozon.browser_err",
			"component", "parser",
			"strategy", "ozon",
			"url", pageURL,
			"error", err.Error(),
		)
		return productSnapshot{}, err
	}

	log.Warn("parser.ozon.browser_snap",
		"component", "parser",
		"strategy", "ozon",
		"url", pageURL,
		"body_len", len(snap.Body),
		"html_len", len(snap.HTML),
		"body_challenge", isChallengeResponse(snap.Body),
		"html_challenge", isChallengeResponse(snap.HTML),
		"body_prefix", clipForLog(snap.Body, 180),
		"html_prefix", clipForLog(snap.HTML, 180),
	)

	if len(snap.Body) > 0 && !isChallengeResponse(snap.Body) {
		if product, parseErr := parseComposerPage(snap.Body); parseErr == nil {
			return product, nil
		} else {
			err = parseErr
		}
	}
	if len(snap.HTML) > 0 && !isChallengeResponse(snap.HTML) {
		if product, ok := parseProductFromHTML(snap.HTML); ok {
			return product, nil
		}
	}
	if err != nil {
		return productSnapshot{}, err
	}
	if isChallengeResponse(snap.Body) || isChallengeResponse(snap.HTML) {
		return productSnapshot{}, errAccessDenied
	}
	return productSnapshot{}, errProductNotFound
}

func resolveComposerTarget(ctx context.Context, pageURL string) (baseURL, composerPath string, err error) {
	baseURL, composerPath, err = ozonPaths(pageURL)
	if err != nil {
		return "", "", err
	}

	if !shortLinkPathPattern.MatchString(composerPath) {
		return baseURL, composerPath, nil
	}

	if resolved, resolveErr := resolveShortLink(ctx, baseURL, pageURL); resolveErr == nil {
		if _, productPath, pathErr := ozonPaths(resolved); pathErr == nil && productPathPattern.MatchString(productPath) {
			return baseURL, productPath, nil
		}
	}

	// composer-api понимает короткие /t/{code}/ ссылки напрямую
	return baseURL, composerPath, nil
}

func resolveShortLink(ctx context.Context, baseURL, rawURL string) (string, error) {
	if err := ensureSession(ctx, baseURL, rawURL); err != nil {
		return "", err
	}

	client := activeSessionClient(baseURL)
	current := rawURL

	for hop := 0; hop < ozonMaxRedirectHops; hop++ {
		finalURL, body, status, err := fetchPageOnce(ctx, client, current)
		if err != nil {
			return "", err
		}

		if u, parseErr := url.Parse(finalURL); parseErr == nil && productPathPattern.MatchString(u.Path) {
			return finalURL, nil
		}

		if status == http.StatusOK {
			if canonical := extractCanonicalProductURL(string(body)); canonical != "" {
				return canonical, nil
			}
			break
		}

		if status == http.StatusForbidden || status == http.StatusUnauthorized {
			break
		}

		current = finalURL
	}

	return "", errProductNotFound
}

func fetchProductTLS(ctx context.Context, pageURL, apiURL string) (productSnapshot, error) {
	if _, _, err := doChromeTLSGet(ctx, pageURL, pageURL); err != nil {
		return productSnapshot{}, err
	}
	status, body, err := doChromeTLSGet(ctx, apiURL, pageURL)
	if err != nil {
		return productSnapshot{}, err
	}
	if status == http.StatusForbidden || isChallengeResponse(body) {
		return productSnapshot{}, fmt.Errorf("%w: tls composer HTTP %d", errAccessDenied, status)
	}
	if status != http.StatusOK {
		return productSnapshot{}, fetchretry.NewHTTPStatusError(&http.Response{
			StatusCode: status,
			Header:     make(http.Header),
		}, apiURL)
	}
	product, err := parseComposerPage(body)
	if err == nil {
		return product, nil
	}
	status, htmlBody, htmlErr := doChromeTLSGet(ctx, pageURL, pageURL)
	if htmlErr != nil {
		return productSnapshot{}, htmlErr
	}
	if status == http.StatusForbidden || isChallengeResponse(htmlBody) {
		return productSnapshot{}, errAccessDenied
	}
	if status != http.StatusOK {
		return productSnapshot{}, fetchretry.NewHTTPStatusError(&http.Response{
			StatusCode: status,
			Header:     make(http.Header),
		}, pageURL)
	}
	if !isChallengeResponse(htmlBody) {
		if product, ok := parseProductFromHTML(htmlBody); ok {
			return product, nil
		}
	}
	return productSnapshot{}, fmt.Errorf("%w: %v", errResponseUnparseable, err)
}

func fetchComposerJSON(ctx context.Context, baseURL, warmupURL, apiURL string) ([]byte, error) {
	if err := ensureSession(ctx, baseURL, warmupURL); err != nil {
		return nil, err
	}

	client := activeSessionClient(baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	setBrowserHeaders(req, warmupURL)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("%w: status 403", errAccessDenied)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fetchretry.NewHTTPStatusError(resp, apiURL)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 6<<20))
	if err != nil {
		return nil, err
	}
	return body, nil
}

func fetchProductHTML(ctx context.Context, baseURL, pageURL string) ([]byte, error) {
	if err := ensureSession(ctx, baseURL, pageURL); err != nil {
		return nil, err
	}

	client := activeSessionClient(baseURL)
	_, body, status, err := fetchPageOnce(ctx, client, pageURL)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fetchretry.NewHTTPStatusError(&http.Response{
			StatusCode: status,
			Header:     http.Header{},
		}, pageURL)
	}
	return body, nil
}

func fetchPageOnce(ctx context.Context, client *http.Client, pageURL string) (finalURL string, body []byte, status int, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", nil, 0, err
	}
	setBrowserHeaders(req, pageURL)

	resp, err := client.Do(req)
	if err != nil {
		return "", nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err = io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", nil, resp.StatusCode, err
	}

	finalURL = pageURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	return finalURL, body, resp.StatusCode, nil
}

func ensureSession(ctx context.Context, baseURL, warmupURL string) error {
	sessionMu.Lock()
	defer sessionMu.Unlock()

	if existing, ok := sessions[baseURL]; ok && time.Since(existing.createdAt) < ozonSessionTTL {
		return nil
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return err
	}

	client := &http.Client{
		Timeout: 15 * time.Second,
		Jar:     jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= ozonMaxRedirectHops {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	targets := []string{warmupURL, baseURL}
	for _, target := range targets {
		if strings.TrimSpace(target) == "" {
			continue
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			continue
		}
		setBrowserHeaders(req, baseURL)

		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}

	sessions[baseURL] = hostSession{
		client:    client,
		baseURL:   baseURL,
		createdAt: time.Now(),
	}
	return nil
}

func setBrowserHeaders(req *http.Request, referer string) {
	req.Header.Set("User-Agent", ozonHTTPUserAgent)
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en;q=0.9")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/json;q=0.9,*/*;q=0.8")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
}

func activeSessionClient(baseURL string) *http.Client {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	if existing, ok := sessions[baseURL]; ok && existing.client != nil {
		return existing.client
	}

	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Timeout: 15 * time.Second,
		Jar:     jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= ozonMaxRedirectHops {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}

func invalidateSession(baseURL string) {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	delete(sessions, baseURL)
}

func extractCanonicalProductURL(htmlBody string) string {
	match := canonicalLinkPattern.FindStringSubmatch(htmlBody)
	if len(match) < 2 {
		return ""
	}
	canonical := strings.TrimSpace(match[1])
	if !strings.Contains(canonical, "/product/") {
		return ""
	}
	return canonical
}

func parseProductFromHTML(htmlBody []byte) (productSnapshot, bool) {
	for _, match := range htmlLDJSONPattern.FindAllSubmatch(htmlBody, -1) {
		if len(match) < 2 {
			continue
		}
		var ld jsonLDProduct
		if err := json.Unmarshal(match[1], &ld); err != nil || ld.Name == "" {
			continue
		}

		out := productSnapshot{
			Title:    ld.Name,
			Currency: ld.Offers.PriceCurrency,
			ImageURL: ldImage(ld.Image),
		}
		if price, ok := priceFromJSONLD(ld.Offers.Price); ok {
			out.Price = price
		}
		if out.Price <= 0 {
			continue
		}
		if out.Currency == "" {
			out.Currency = "BYN"
		}
		return out, true
	}
	return productSnapshot{}, false
}

func isChallengeResponse(body []byte) bool {
	if len(body) == 0 {
		return true
	}
	if body[0] == '<' {
		return strings.Contains(string(body), "Antibot") ||
			strings.Contains(string(body), "challengeURL") ||
			strings.Contains(string(body), "Antibot Challenge Page")
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return false
	}
	if _, ok := payload["challengeURL"]; ok {
		return true
	}
	if _, ok := payload["incidentId"]; ok && payload["widgetStates"] == nil {
		return true
	}
	return false
}

func clipForLog(raw []byte, n int) string {
	s := strings.Join(strings.Fields(string(raw)), " ")
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func logParseFailure(pageURL string, err error) {
	log.Warn("parser.ozon.parse_failed",
		"component", "parser",
		"strategy", "ozon",
		"url", pageURL,
		"error", err.Error(),
	)
}
