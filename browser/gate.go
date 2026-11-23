package browser

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// defaultGateCooldown skips browser after hard failures so marketplace parse
// does not burn the full page budget on every add-link while the sidecar is down.
const defaultGateCooldown = 45 * time.Second

// Gate wraps a capturer/fetcher and short-circuits while cooling down after
// sidecar-down / not-installed failures.
type Gate struct {
	inner    JSONCapturer
	pages    PageFetcher
	cooldown time.Duration

	mu        sync.Mutex
	deadUntil time.Time
}

// NewGate returns inner unchanged when nil. cooldown <= 0 uses defaultGateCooldown.
func NewGate(inner JSONCapturer, cooldown time.Duration) JSONCapturer {
	if inner == nil {
		return nil
	}
	if cooldown <= 0 {
		cooldown = defaultGateCooldown
	}
	g := &Gate{inner: inner, cooldown: cooldown}
	if pages, ok := inner.(PageFetcher); ok {
		g.pages = pages
	}
	return g
}

func (g *Gate) CaptureJSON(ctx context.Context, pageURL, urlGlob string) ([]byte, error) {
	if err := g.blocked(); err != nil {
		return nil, err
	}
	body, err := g.inner.CaptureJSON(ctx, pageURL, urlGlob)
	g.observe(err)
	return body, err
}

func (g *Gate) FetchInPage(ctx context.Context, warmupURL, requestURL string) (PageSnapshot, error) {
	if g.pages == nil {
		return PageSnapshot{}, fmt.Errorf("%w: no page fetcher", ErrRemoteUnavailable)
	}
	if err := g.blocked(); err != nil {
		return PageSnapshot{}, err
	}
	snap, err := g.pages.FetchInPage(ctx, warmupURL, requestURL)
	g.observe(err)
	return snap, err
}

func (g *Gate) blocked() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if time.Now().Before(g.deadUntil) {
		return fmt.Errorf("%w: cooling down", ErrRemoteUnavailable)
	}
	return nil
}

func (g *Gate) observe(err error) {
	if err == nil || !isHardBrowserFailure(err) {
		return
	}
	g.mu.Lock()
	g.deadUntil = time.Now().Add(g.cooldown)
	g.mu.Unlock()
}

func isHardBrowserFailure(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrRemoteUnavailable) ||
		errors.Is(err, ErrNotInstalled) ||
		errors.Is(err, ErrClosed) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, p := range []string{
		"connection refused",
		"no such host",
		"playwright run",
		"chromium launch",
		"context deadline exceeded",
		"x_wbaas_token not set",
	} {
		if strings.Contains(msg, p) {
			return true
		}
	}
	return false
}
