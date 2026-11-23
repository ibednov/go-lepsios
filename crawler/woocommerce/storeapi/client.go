package storeapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	crawlerr "github.com/ibednov/go-lepsios/crawler/errors"
	"github.com/ibednov/go-lepsios/crawler/httpclient"
)

func (a *Adapter) fetchProductsPage(ctx context.Context, page int) ([]byte, *http.Response, error) {
	u := fmt.Sprintf("%s/wp-json/wc/store/v1/products?per_page=100&page=%d", a.base, page)
	return a.get(ctx, u)
}

func (a *Adapter) fetchProductByID(ctx context.Context, id string) (storeProduct, error) {
	u := fmt.Sprintf("%s/wp-json/wc/store/v1/products/%s", a.base, url.PathEscape(id))
	body, resp, err := a.get(ctx, u)
	if err != nil {
		return storeProduct{}, classifyHTTPError(err)
	}
	if resp.StatusCode != http.StatusOK {
		return storeProduct{}, classifyHTTPError(httpclient.StatusError(resp.StatusCode, u))
	}
	p, err := parseStoreProduct(body)
	if err != nil {
		return storeProduct{}, crawlerr.ParseFailed(Code, err)
	}
	return p, nil
}

func (a *Adapter) fetchProductByPermalink(ctx context.Context, permalink string) (storeProduct, error) {
	// Resolve via slug search: Store API ?slug=
	slug := productSlug(permalink)
	if slug == "" {
		return storeProduct{}, crawlerr.ParseFailed(Code, fmt.Errorf("cannot derive slug from %s", permalink))
	}
	u := fmt.Sprintf("%s/wp-json/wc/store/v1/products?slug=%s", a.base, url.QueryEscape(slug))
	body, resp, err := a.get(ctx, u)
	if err != nil {
		return storeProduct{}, classifyHTTPError(err)
	}
	if resp.StatusCode != http.StatusOK {
		return storeProduct{}, classifyHTTPError(httpclient.StatusError(resp.StatusCode, u))
	}
	products, err := parseStoreProducts(body)
	if err != nil {
		return storeProduct{}, crawlerr.ParseFailed(Code, err)
	}
	if len(products) == 0 {
		return storeProduct{}, crawlerr.ParseFailed(Code, fmt.Errorf("product not found for slug %s", slug))
	}
	return products[0], nil
}

func (a *Adapter) fetchHTML(ctx context.Context, rawURL string) ([]byte, error) {
	if !sameOrigin(a.base, rawURL) {
		return nil, fmt.Errorf("refusing off-origin product page")
	}
	body, resp, err := a.get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, httpclient.StatusError(resp.StatusCode, rawURL)
	}
	return body, nil
}

func sameOrigin(base, raw string) bool {
	baseURL, baseErr := url.Parse(base)
	parsed, err := url.Parse(raw)
	return baseErr == nil && err == nil && baseURL != nil && parsed != nil &&
		strings.EqualFold(baseURL.Scheme, parsed.Scheme) && strings.EqualFold(baseURL.Host, parsed.Host)
}

func classifyHTTPError(err error) error {
	var statusErr *httpclient.HTTPStatusError
	if !errors.As(err, &statusErr) {
		return crawlerr.Transient(Code, err)
	}
	switch statusErr.Status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return crawlerr.Blocked(Code, err)
	case http.StatusTooManyRequests:
		return crawlerr.Transient(Code, err)
	default:
		if statusErr.Status >= 500 {
			return crawlerr.Transient(Code, err)
		}
		return crawlerr.ParseFailed(Code, err)
	}
}

func (a *Adapter) get(ctx context.Context, rawURL string) ([]byte, *http.Response, error) {
	body, response, err := a.cli.GetWithRetry(ctx, rawURL)
	if response == nil {
		return body, response, err
	}
	bodyHash := httpclient.CapturedBodyHash(body)
	bytesRead := int64(len(body))
	var tooLarge *httpclient.BodyTooLargeError
	if errors.As(err, &tooLarge) {
		bodyHash = tooLarge.BodyHash
		bytesRead = tooLarge.BytesRead
	}
	captured := httpclient.DiagnosticResponse{
		URL: sanitizeDiagnosticURL(rawURL), StatusCode: response.StatusCode,
		ContentType: response.Header.Get("Content-Type"), ReceivedAt: time.Now().UTC(),
		BodyHash: bodyHash, BytesRead: bytesRead,
		BodyTruncated: httpclient.IsBodyTruncated(err),
	}
	if len(body) > httpclient.MaxDiagnosticBodyBytes {
		captured.Body = append([]byte(nil), body[:httpclient.MaxDiagnosticBodyBytes]...)
		captured.StorageTruncated = true
	} else {
		captured.Body = append([]byte(nil), body...)
	}
	a.diagnosticMu.Lock()
	a.diagnostic = &captured
	a.diagnosticMu.Unlock()
	return body, response, err
}

func (a *Adapter) resetDiagnostic() {
	a.diagnosticMu.Lock()
	a.diagnostic = nil
	a.diagnosticMu.Unlock()
}

func (a *Adapter) LastDiagnosticResponse() (httpclient.DiagnosticResponse, bool) {
	a.diagnosticMu.Lock()
	defer a.diagnosticMu.Unlock()
	if a.diagnostic == nil {
		return httpclient.DiagnosticResponse{}, false
	}
	copy := *a.diagnostic
	copy.Body = append([]byte(nil), a.diagnostic.Body...)
	return copy, true
}

func sanitizeDiagnosticURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	u.User = nil
	u.RawQuery = ""
	u.ForceQuery = false
	u.Fragment = ""
	return u.String()
}

func productSlug(permalink string) string {
	u, err := url.Parse(permalink)
	if err != nil {
		return ""
	}
	parts := splitPath(u.Path)
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" && parts[i] != "product" {
			return parts[i]
		}
	}
	return ""
}

func splitPath(p string) []string {
	var out []string
	cur := ""
	for i := 0; i < len(p); i++ {
		if p[i] == '/' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(p[i])
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
