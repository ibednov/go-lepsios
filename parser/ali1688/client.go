package ali1688

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ibednov/go-lepsios/log"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

const (
	httpUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36"
	httpTimeout   = 20 * time.Second
)

func newHTTPClient(proxyRaw string) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxyRaw = strings.TrimSpace(proxyRaw); proxyRaw != "" {
		proxyURL, err := url.Parse(proxyRaw)
		if err != nil || proxyURL.Scheme == "" || proxyURL.Host == "" {
			log.Error("parser.ali1688.invalid_proxy",
				"component", "parser",
				"strategy", "ali1688",
				"error", fmt.Sprintf("invalid PARSER_1688_HTTP_PROXY: %s", proxyRaw),
			)
		} else {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}
	return &http.Client{Timeout: httpTimeout, Transport: transport}
}

func (s *Strategy) client() *http.Client {
	if s != nil && s.httpClient != nil {
		return s.httpClient
	}
	return newHTTPClient("")
}

func (s *Strategy) fetchProduct(ctx context.Context, pageURL string) (productSnapshot, error) {
	offerID, err := s.resolveOfferID(ctx, pageURL)
	if err != nil {
		return productSnapshot{}, err
	}

	htmlBody, err := s.fetchHTML(ctx, detailURL(offerID))
	if err != nil {
		return productSnapshot{}, err
	}
	if isPunishPage(htmlBody) {
		log.Warn("parser.ali1688.waf_blocked",
			"component", "parser",
			"strategy", "ali1688",
			"url", pageURL,
			"offer_id", offerID,
			"html_bytes", len(htmlBody),
		)
		return productSnapshot{}, errAccessDenied
	}

	product, ok := parseProductFromHTML(htmlBody, offerID)
	if !ok {
		log.Warn("parser.ali1688.parse_failed",
			"component", "parser",
			"strategy", "ali1688",
			"url", pageURL,
			"offer_id", offerID,
			"html_bytes", len(htmlBody),
		)
		return productSnapshot{}, errProductNotFound
	}
	return product, nil
}

func (s *Strategy) resolveOfferID(ctx context.Context, pageURL string) (string, error) {
	id, err := resolveOfferID(pageURL, "")
	if err == nil {
		return id, nil
	}

	u, parseErr := url.Parse(pageURL)
	if parseErr != nil {
		return "", err
	}
	host := strings.ToLower(strings.TrimPrefix(u.Hostname(), "www."))
	if host != "qr.1688.com" {
		return "", err
	}

	body, fetchErr := s.fetchHTML(ctx, pageURL)
	if fetchErr != nil {
		return "", fetchErr
	}
	return resolveOfferID(pageURL, body)
}

func (s *Strategy) fetchHTML(ctx context.Context, pageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", httpUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := s.client().Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d for %s", resp.StatusCode, pageURL)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", err
	}
	return decodeBody(body, resp.Header.Get("Content-Type")), nil
}

func decodeBody(body []byte, contentType string) string {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "gbk") || strings.Contains(ct, "gb2312") || strings.Contains(ct, "gb18030") {
		reader := transform.NewReader(bytes.NewReader(body), simplifiedchinese.GB18030.NewDecoder())
		decoded, err := io.ReadAll(reader)
		if err == nil {
			return string(decoded)
		}
	}
	return string(body)
}
