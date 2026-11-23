package httpclient

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	maxRetries   = 3
	baseDelay    = 200 * time.Millisecond
	maxDelay     = 5 * time.Second
	challengePause = 5 * time.Second
)

// Retryable reports transient transport/server failures safe to retry.
func Retryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var opErr *net.OpError
	return errors.As(err, &opErr)
}

// RetryableStatus is true for 429 and 5xx.
func RetryableStatus(code int) bool {
	return code == http.StatusTooManyRequests || (code >= 500 && code <= 599)
}

// IsJSChallengeBody detects garbata-style "One moment, please…" HTML stub.
func IsJSChallengeBody(body []byte) bool {
	s := string(body)
	return containsFold(s, "One moment, please") || containsFold(s, "window.location.reload")
}

func containsFold(s, sub string) bool {
	return len(s) >= len(sub) && (indexFold(s, sub) >= 0)
}

func indexFold(s, sub string) int {
	// small bodies only; case-insensitive contains
	ls, lsub := len(s), len(sub)
	if lsub == 0 || lsub > ls {
		if lsub == 0 {
			return 0
		}
		return -1
	}
	for i := 0; i+lsub <= ls; i++ {
		ok := true
		for j := 0; j < lsub; j++ {
			a, b := s[i+j], sub[j]
			if a >= 'A' && a <= 'Z' {
				a += 'a' - 'A'
			}
			if b >= 'A' && b <= 'Z' {
				b += 'a' - 'A'
			}
			if a != b {
				ok = false
				break
			}
		}
		if ok {
			return i
		}
	}
	return -1
}

// WaitBackoff sleeps with exponential backoff + jitter. For JS stub use min 5s.
func WaitBackoff(ctx context.Context, attempt int, jsStub bool) error {
	if attempt < 1 {
		attempt = 1
	}
	delay := baseDelay << (attempt - 1)
	if delay > maxDelay {
		delay = maxDelay
	}
	if jsStub && delay < challengePause {
		delay = challengePause
	}
	jitter := time.Duration(rand.Int63n(int64(delay/2) + 1))
	delay += jitter

	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// GetWithRetry fetches URL with bounded retries on transient errors / challenge HTML.
func (c *Client) GetWithRetry(ctx context.Context, rawURL string) ([]byte, *http.Response, error) {
	return c.GetWithRetryHeaders(ctx, rawURL, nil)
}

func (c *Client) GetWithRetryHeaders(ctx context.Context, rawURL string, headers http.Header) ([]byte, *http.Response, error) {
	return c.GetWithRetryHeadersLimit(ctx, rawURL, headers, 0)
}

// GetWithRetryHeadersLimit retries the request while retaining the same bounded
// response-body limit on each attempt. A zero limit uses the client's default.
func (c *Client) GetWithRetryHeadersLimit(ctx context.Context, rawURL string, headers http.Header, maxBytes int64) ([]byte, *http.Response, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		body, resp, err := c.GetWithHeadersLimit(ctx, rawURL, headers, maxBytes)
		if err != nil {
			lastErr = err
			if !Retryable(err) || attempt == maxRetries {
				return body, resp, err
			}
			if werr := WaitBackoff(ctx, attempt+1, false); werr != nil {
				return nil, nil, werr
			}
			continue
		}
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
			return body, resp, StatusError(resp.StatusCode, rawURL)
		}
		if RetryableStatus(resp.StatusCode) {
			lastErr = StatusError(resp.StatusCode, rawURL)
			if attempt == maxRetries {
				return body, resp, lastErr
			}
			if delay, ok, tooLong := retryAfterDelay(resp.Header.Get("Retry-After"), time.Now()); tooLong {
				return body, resp, lastErr
			} else if ok {
				if werr := WaitRetryAfter(ctx, delay); werr != nil {
					return body, resp, werr
				}
			} else if werr := WaitBackoff(ctx, attempt+1, false); werr != nil {
				return body, resp, werr
			}
			continue
		}
		if IsJSChallengeBody(body) {
			lastErr = StatusError(resp.StatusCode, rawURL)
			if attempt == maxRetries {
				return body, resp, &ChallengeError{Status: resp.StatusCode, URL: rawURL}
			}
			if werr := WaitBackoff(ctx, attempt+1, true); werr != nil {
				return body, resp, werr
			}
			continue
		}
		return body, resp, nil
	}
	return nil, nil, lastErr
}

// WaitRetryAfter waits at least the server-specified delay and adds positive
// jitter, capped by the HTTP client's maximum retry delay.
func WaitRetryAfter(ctx context.Context, delay time.Duration) error {
	if delay < 0 || delay > maxDelay {
		return errors.New("retry-after delay exceeds retry policy")
	}
	maxJitter := maxDelay - delay
	if maxJitter > delay/4 {
		maxJitter = delay / 4
	}
	if maxJitter > 0 {
		delay += time.Duration(rand.Int63n(int64(maxJitter) + 1))
	}
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// retryAfterDelay parses delta-seconds and HTTP-date values. Values above the
// retry policy ceiling are reported separately so callers stop retrying instead
// of violating the server's requested minimum delay.
func retryAfterDelay(value string, now time.Time) (delay time.Duration, ok, tooLong bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false, false
	}
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		if seconds < 0 {
			return 0, false, false
		}
		if seconds > int64(maxDelay/time.Second) {
			return 0, true, true
		}
		return time.Duration(seconds) * time.Second, true, false
	}
	date, err := http.ParseTime(value)
	if err != nil {
		return 0, false, false
	}
	delay = date.Sub(now)
	if delay <= 0 {
		return 0, true, false
	}
	if delay > maxDelay {
		return 0, true, true
	}
	return delay, true, false
}
