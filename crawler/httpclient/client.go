package httpclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// DefaultMaxBodyBytes is large enough for image-heavy product pages
// (some tea shops exceed the previous 4 MiB cap) without allowing unbounded reads.
const DefaultMaxBodyBytes int64 = 10 << 20

// Config for outbound crawl HTTP.
type Config struct {
	UserAgent    string
	DefaultRPS   float64
	Timeout      time.Duration
	MaxBodyBytes int64
	// SameOriginRedirectsOnly rejects redirects to a different scheme/host/port.
	SameOriginRedirectsOnly bool
}

// BodyTooLargeError reports that the response exceeded the configured limit.
// The returned body is only the safe prefix and must never be parsed as complete.
type BodyTooLargeError struct {
	Limit     int64
	URL       string
	BytesRead int64
	BodyHash  string
}

// BodyReadError reports that the response stream ended with an error after a
// prefix was received. The prefix is diagnostic only and must not be parsed.
type BodyReadError struct {
	URL       string
	BytesRead int64
	Err       error
}

func (e *BodyReadError) Error() string {
	return fmt.Sprintf("response body read failed after %d bytes for %s", e.BytesRead, sanitizeURL(e.URL))
}

func (e *BodyReadError) Unwrap() error { return e.Err }

func IsBodyTruncated(err error) bool {
	var tooLarge *BodyTooLargeError
	var readErr *BodyReadError
	return errors.As(err, &tooLarge) || errors.As(err, &readErr)
}

// DiagnosticResponse contains response metadata without request headers.
type DiagnosticResponse struct {
	URL              string
	StatusCode       int
	ContentType      string
	ReceivedAt       time.Time
	Body             []byte
	BodyHash         string
	BytesRead        int64
	BodyTruncated    bool
	StorageTruncated bool
}

const MaxDiagnosticBodyBytes = 128 << 10

// MaxConfiguredResponseBytes bounds per-request size overrides used by universal adapters.
const MaxConfiguredResponseBytes int64 = 16 << 20

func (e *BodyTooLargeError) Error() string {
	return fmt.Sprintf("response body exceeded %d bytes for %s", e.Limit, sanitizeURL(e.URL))
}

// CapturedBodyHash hashes the bytes read by the HTTP client; when the body was
// truncated, this is a prefix hash and callers must retain the truncation flag.
func CapturedBodyHash(body []byte) string {
	h := sha256.Sum256(body)
	return hex.EncodeToString(h[:])
}

// Client is TLS-verifying HTTP with per-host rate limit.
// Certificate verification stays on (default http.Transport).
type Client struct {
	http       *http.Client
	ua         string
	maxBody    int64
	rps        float64
	mu         sync.Mutex
	lastByHost map[string]time.Time
}

func New(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	rps := cfg.DefaultRPS
	if rps <= 0 {
		rps = 1
	}
	maxBody := cfg.MaxBodyBytes
	if maxBody <= 0 {
		maxBody = DefaultMaxBodyBytes
	}
	ua := cfg.UserAgent
	if ua == "" {
		ua = "CrawlerBot/0.1 (+https://github.com/ibednov/go-lepsios)"
	}
	return &Client{
		http: &http.Client{Timeout: timeout, CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if cfg.SameOriginRedirectsOnly && len(via) > 0 && !sameOriginURL(via[0].URL, req.URL) {
				return http.ErrUseLastResponse
			}
			return nil
		}},
		ua:         ua,
		maxBody:    maxBody,
		rps:        rps,
		lastByHost: make(map[string]time.Time),
	}
}

func sameOriginURL(a, b *url.URL) bool {
	return a != nil && b != nil && strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}

func (c *Client) Get(ctx context.Context, rawURL string) ([]byte, *http.Response, error) {
	return c.GetWithHeaders(ctx, rawURL, nil)
}

func (c *Client) GetWithHeaders(ctx context.Context, rawURL string, headers http.Header) ([]byte, *http.Response, error) {
	return c.GetWithHeadersLimit(ctx, rawURL, headers, 0)
}

// GetWithHeadersLimit reads at most maxBytes for this request. A zero limit uses
// the client's default; overrides are capped to protect memory use.
func (c *Client) GetWithHeadersLimit(ctx context.Context, rawURL string, headers http.Header, maxBytes int64) ([]byte, *http.Response, error) {
	if maxBytes == 0 {
		maxBytes = c.maxBody
	}
	if maxBytes < 1 || maxBytes > MaxConfiguredResponseBytes {
		return nil, nil, fmt.Errorf("response body limit must be between 1 and %d bytes", MaxConfiguredResponseBytes)
	}
	if err := c.waitHost(ctx, rawURL); err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", c.ua)
	req.Header.Set("Accept", "application/json, text/html, */*")
	req.Header.Set("Accept-Language", "ru,en;q=0.8")
	for key, values := range headers {
		for _, value := range values {
			req.Header.Set(key, value)
		}
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if resp != nil && resp.Body != nil {
			// caller may still need headers; body closed after read below
		}
	}()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	_ = resp.Body.Close()
	if err != nil {
		readErr := &BodyReadError{URL: rawURL, BytesRead: int64(len(body)), Err: err}
		c.captureResponse(ctx, rawURL, resp, body, readErr)
		return body, resp, readErr
	}
	if int64(len(body)) > maxBytes {
		readBytes := int64(len(body))
		bodyHash := CapturedBodyHash(body)
		body = body[:maxBytes]
		tooLarge := &BodyTooLargeError{Limit: maxBytes, URL: rawURL, BytesRead: readBytes, BodyHash: bodyHash}
		c.captureResponse(ctx, rawURL, resp, body, tooLarge)
		return body, resp, tooLarge
	}
	c.captureResponse(ctx, rawURL, resp, body, nil)
	return body, resp, nil
}

func (c *Client) waitHost(ctx context.Context, rawURL string) error {
	host := hostOf(rawURL)
	minGap := time.Duration(float64(time.Second) / c.rps)
	for {
		c.mu.Lock()
		last := c.lastByHost[host]
		wait := minGap - time.Since(last)
		if wait <= 0 || last.IsZero() {
			c.lastByHost[host] = time.Now()
			c.mu.Unlock()
			return nil
		}
		c.mu.Unlock()
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return strings.ToLower(u.Hostname())
}

type HTTPStatusError struct {
	Status int
	URL    string
}

// ChallengeError reports that bounded retries did not clear a JavaScript or
// anti-bot challenge response.
type ChallengeError struct {
	Status int
	URL    string
}

func (e *ChallengeError) Error() string {
	return fmt.Sprintf("source returned a challenge response with status %d for %s", e.Status, sanitizeURL(e.URL))
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("unexpected status %d for %s", e.Status, sanitizeURL(e.URL))
}

func StatusError(status int, url string) error { return &HTTPStatusError{Status: status, URL: url} }

func sanitizeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "<invalid-url>"
	}
	u.User = nil
	u.RawQuery = ""
	u.ForceQuery = false
	u.Fragment = ""
	return u.String()
}
