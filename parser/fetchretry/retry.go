package fetchretry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	maxRetries    = 2
	baseDelay     = 200 * time.Millisecond
	maxRetryAfter = 800 * time.Millisecond
	maxRetryWait  = time.Second
)

// HTTPStatusError keeps response status and Retry-After available for retry
// policy decisions without inferring them from an error string.
type HTTPStatusError struct {
	StatusCode int
	RetryAfter time.Duration
	URL        string
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("unexpected status %d for %s", e.StatusCode, e.URL)
}

func NewHTTPStatusError(resp *http.Response, requestURL string) *HTTPStatusError {
	return &HTTPStatusError{
		StatusCode: resp.StatusCode,
		RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"), time.Now()),
		URL:        requestURL,
	}
}

// Retryable distinguishes transient transport/server failures from permanent
// HTTP responses and parser-level errors. A Retry-After beyond this operation's
// bounded retry budget is surfaced immediately instead of being shortened.
func Retryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var statusErr *HTTPStatusError
	if errors.As(err, &statusErr) {
		if statusErr.RetryAfter > maxRetryAfter {
			return false
		}
		return statusErr.StatusCode == http.StatusTooManyRequests ||
			statusErr.StatusCode >= http.StatusInternalServerError &&
				statusErr.StatusCode <= 599
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return dnsErr.Timeout() || dnsErr.Temporary()
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary())
}

// Wait applies capped exponential backoff with per-attempt jitter. Retry-After
// is treated as a minimum delay and receives additional jitter.
func Wait(ctx context.Context, retryNumber int, retryAfter time.Duration) error {
	if retryNumber < 1 || retryNumber > maxRetries {
		return fmt.Errorf("retry number %d outside bounded retry policy", retryNumber)
	}
	if retryAfter > maxRetryAfter {
		return fmt.Errorf("retry-after %s exceeds bounded retry policy", retryAfter)
	}

	delay := baseDelay << (retryNumber - 1)
	if delay > maxRetryAfter {
		delay = maxRetryAfter
	}
	jitterMax := delay / 2
	if retryAfter > 0 {
		if remaining := maxRetryWait - retryAfter; jitterMax > remaining {
			jitterMax = remaining
		}
	}
	jitter := time.Duration(rand.Int63n(int64(jitterMax) + 1))
	if retryAfter > 0 {
		delay = retryAfter + jitter
	} else {
		delay += jitter
	}
	if delay > maxRetryWait {
		return fmt.Errorf("retry delay %s exceeds bounded retry policy", delay)
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func parseRetryAfter(raw string, now time.Time) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if seconds, err := strconv.ParseInt(raw, 10, 64); err == nil && seconds >= 0 {
		if seconds > int64(maxRetryAfter/time.Second) {
			return maxRetryAfter + time.Nanosecond
		}
		return time.Duration(seconds) * time.Second
	}
	when, err := http.ParseTime(raw)
	if err != nil || !when.After(now) {
		return 0
	}
	return when.Sub(now)
}
