package httpclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetWithRetryHonorsRetryAfter(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if hits.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	start := time.Now()
	body, response, err := New(Config{DefaultRPS: 1000}).GetWithRetry(context.Background(), server.URL)
	if err != nil || response.StatusCode != http.StatusOK || string(body) != "ok" {
		t.Fatalf("body=%q response=%v err=%v", body, response, err)
	}
	if elapsed := time.Since(start); elapsed < time.Second {
		t.Fatalf("retried after %s, want at least Retry-After 1s", elapsed)
	}
	if hits.Load() != 2 {
		t.Fatalf("requests=%d, want 2", hits.Load())
	}
}

func TestGetWithRetryStopsWhenRetryAfterExceedsPolicy(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	_, response, err := New(Config{DefaultRPS: 1000}).GetWithRetry(context.Background(), server.URL)
	var statusErr *HTTPStatusError
	if !errors.As(err, &statusErr) || statusErr.Status != http.StatusTooManyRequests || response == nil {
		t.Fatalf("response=%v err=%v, want 429 without retry", response, err)
	}
	if hits.Load() != 1 {
		t.Fatalf("requests=%d, want 1", hits.Load())
	}
}

func TestGetWithRetryCancellationInterruptsRetryAfter(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	timer := time.AfterFunc(50*time.Millisecond, cancel)
	defer timer.Stop()
	start := time.Now()
	_, _, err := New(Config{DefaultRPS: 1000}).GetWithRetry(ctx, server.URL)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v, want context cancellation", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("canceled retry waited %s", elapsed)
	}
	if hits.Load() != 1 {
		t.Fatalf("requests=%d, want no retry after cancellation", hits.Load())
	}
}

func TestGetRejectsTruncatedBodyAndSanitizesErrorURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("too long"))
	}))
	defer server.Close()
	url := server.URL + "/page?token=secret#private"
	body, response, err := New(Config{MaxBodyBytes: 4}).Get(context.Background(), url)
	var sizeErr *BodyTooLargeError
	if !errors.As(err, &sizeErr) || response == nil || string(body) != "too " {
		t.Fatalf("body=%q response=%v err=%v", body, response, err)
	}
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "private") {
		t.Fatalf("error exposed URL secrets: %v", err)
	}
	statusErr := StatusError(http.StatusForbidden, url)
	if strings.Contains(statusErr.Error(), "secret") || strings.Contains(statusErr.Error(), "private") {
		t.Fatalf("status error exposed URL secrets: %v", statusErr)
	}
}

func TestRetryAfterParser(t *testing.T) {
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		value   string
		want    time.Duration
		ok      bool
		tooLong bool
	}{
		{value: "2", want: 2 * time.Second, ok: true},
		{value: now.Add(2 * time.Second).Format(http.TimeFormat), want: 2 * time.Second, ok: true},
		{value: now.Add(-time.Second).Format(http.TimeFormat), ok: true},
		{value: "3600", ok: true, tooLong: true},
		{value: "invalid"},
	} {
		got, ok, tooLong := retryAfterDelay(tc.value, now)
		if got != tc.want || ok != tc.ok || tooLong != tc.tooLong {
			t.Errorf("Retry-After %q = (%s,%t,%t), want (%s,%t,%t)", tc.value, got, ok, tooLong, tc.want, tc.ok, tc.tooLong)
		}
	}
}
