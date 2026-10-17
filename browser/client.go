package browser

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mxschmitt/playwright-go"
)

const stealthInitScript = `
Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
window.chrome = window.chrome || { runtime: {} };
Object.defineProperty(navigator, 'languages', { get: () => ['ru-RU', 'ru', 'en-US', 'en'] });
Object.defineProperty(navigator, 'plugins', { get: () => [1, 2, 3, 4, 5] });
`

// Client is one Chromium per process. Launch is lazy: playwright.Run on first use.
// Pages go through a semaphore, not a lock on Goto.
type Client struct {
	opts Options

	mu      sync.Mutex
	closed  bool
	pw      *playwright.Playwright
	browser playwright.Browser
	sem     chan struct{}
}

// New does not start the browser process. Start happens on first capture/fetch.
func New(opts Options) *Client {
	opts = opts.withDefaults()
	return &Client{
		opts: opts,
		sem:  make(chan struct{}, opts.MaxConcurrent),
	}
}

// Close stops the browser and driver. A second call is a no-op.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true

	if c.browser != nil {
		_ = c.browser.Close()
		c.browser = nil
	}
	if c.pw != nil {
		err := c.pw.Stop()
		c.pw = nil
		return err
	}
	return nil
}

func (c *Client) startLocked() error {
	if c.closed {
		return ErrClosed
	}
	if c.browser != nil {
		return nil
	}

	pw, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("playwright run: %w (hint: %v)", err, ErrNotInstalled)
	}

	headless := c.opts.Headless
	launch := playwright.BrowserTypeLaunchOptions{
		Headless: &headless,
		Args: []string{
			"--disable-blink-features=AutomationControlled",
			"--disable-dev-shm-usage",
			"--no-sandbox",
			"--disable-gpu",
		},
	}
	if c.opts.ExecutablePath != "" {
		exec := c.opts.ExecutablePath
		launch.ExecutablePath = &exec
	}

	br, err := pw.Chromium.Launch(launch)
	if err != nil {
		_ = pw.Stop()
		return fmt.Errorf("chromium launch: %w (hint: %v)", err, ErrNotInstalled)
	}

	c.pw = pw
	c.browser = br
	return nil
}

// CaptureJSON registers ExpectResponse before Goto and reads the matched body.
func (c *Client) CaptureJSON(ctx context.Context, pageURL, urlGlob string) ([]byte, error) {
	if c == nil {
		return nil, ErrClosed
	}
	if pageURL == "" {
		return nil, ErrEmptyPageURL
	}
	if urlGlob == "" {
		return nil, ErrEmptyGlob
	}

	if err := c.acquire(ctx); err != nil {
		return nil, err
	}
	defer c.release()

	c.mu.Lock()
	if err := c.startLocked(); err != nil {
		c.mu.Unlock()
		return nil, err
	}
	browser := c.browser
	c.mu.Unlock()

	captureCtx, cancel := context.WithTimeout(ctx, c.opts.Timeout)
	defer cancel()

	bctx, page, err := newPage(browser, c.opts)
	if err != nil {
		return nil, err
	}
	defer func() { _ = bctx.Close() }()

	go func() {
		<-captureCtx.Done()
		_ = bctx.Close()
	}()

	timeoutMS := float64(c.opts.Timeout.Milliseconds())
	resp, err := page.ExpectResponse(urlGlob, func() error {
		_, gotoErr := page.Goto(pageURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
			Timeout:   &timeoutMS,
		})
		return gotoErr
	}, playwright.PageExpectResponseOptions{Timeout: &timeoutMS})
	if err != nil {
		if captureCtx.Err() != nil {
			return nil, fmt.Errorf("capture %s: %w", urlGlob, captureCtx.Err())
		}
		return nil, fmt.Errorf("capture %s: %w", urlGlob, err)
	}
	if resp.Status() != 200 {
		return nil, fmt.Errorf("captured %s HTTP %d", resp.URL(), resp.Status())
	}

	body, err := resp.Body()
	if err != nil {
		return nil, fmt.Errorf("captured body: %w", err)
	}
	return body, nil
}

// FetchInPage opens warmupURL, waits for antibot JS, then fetch(requestURL)
// in page context. HTML is always returned after warmup.
func (c *Client) FetchInPage(ctx context.Context, warmupURL, requestURL string) (PageSnapshot, error) {
	if c == nil {
		return PageSnapshot{}, ErrClosed
	}
	if warmupURL == "" {
		return PageSnapshot{}, ErrEmptyPageURL
	}
	if requestURL == "" {
		return PageSnapshot{}, ErrEmptyRequestURL
	}
	if err := CheckAllowedURL(warmupURL); err != nil {
		return PageSnapshot{}, err
	}
	if err := CheckAllowedURL(requestURL); err != nil {
		return PageSnapshot{}, err
	}

	if err := c.acquire(ctx); err != nil {
		return PageSnapshot{}, err
	}
	defer c.release()

	c.mu.Lock()
	if err := c.startLocked(); err != nil {
		c.mu.Unlock()
		return PageSnapshot{}, err
	}
	br := c.browser
	c.mu.Unlock()

	captureCtx, cancel := context.WithTimeout(ctx, c.opts.Timeout)
	defer cancel()

	bctx, page, err := newPage(br, c.opts)
	if err != nil {
		return PageSnapshot{}, err
	}
	defer func() { _ = bctx.Close() }()

	go func() {
		<-captureCtx.Done()
		_ = bctx.Close()
	}()

	timeoutMS := float64(c.opts.Timeout.Milliseconds())
	if _, err := page.Goto(warmupURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateLoad,
		Timeout:   &timeoutMS,
	}); err != nil {
		if captureCtx.Err() != nil {
			return PageSnapshot{}, fmt.Errorf("warmup %s: %w", warmupURL, captureCtx.Err())
		}
		return PageSnapshot{}, fmt.Errorf("warmup %s: %w", warmupURL, err)
	}

	if err := waitUntilPageReady(captureCtx, page, c.opts.Warmup); err != nil {
		return PageSnapshot{}, err
	}

	html, err := page.Content()
	if err != nil {
		return PageSnapshot{}, fmt.Errorf("page content: %w", err)
	}
	if LooksLikeChallenge(html) {
		if _, reloadErr := page.Reload(playwright.PageReloadOptions{
			WaitUntil: playwright.WaitUntilStateLoad,
			Timeout:   &timeoutMS,
		}); reloadErr == nil {
			_ = waitUntilPageReady(captureCtx, page, c.opts.Warmup/2)
			if next, nextErr := page.Content(); nextErr == nil {
				html = next
			}
		}
	}

	out := PageSnapshot{HTML: []byte(html)}
	if sameRequestURL(warmupURL, requestURL) {
		out.Body = out.HTML
		return out, nil
	}

	raw, err := page.Evaluate(`async (url) => {
		const r = await fetch(url, {
			credentials: 'include',
			headers: { Accept: 'application/json,text/html;q=0.9,*/*;q=0.8' },
		});
		return { status: r.status, body: await r.text() };
	}`, requestURL)
	if err != nil {
		if captureCtx.Err() != nil {
			return out, fmt.Errorf("page fetch %s: %w", requestURL, captureCtx.Err())
		}
		return out, fmt.Errorf("page fetch %s: %w", requestURL, err)
	}

	status, body, err := evaluateFetch(raw)
	if err != nil {
		return out, err
	}
	out.Status = status
	out.Body = body
	return out, nil
}

func waitUntilPageReady(ctx context.Context, page playwright.Page, maxWait time.Duration) error {
	if maxWait <= 0 {
		return nil
	}
	deadline := time.Now().Add(maxWait)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		html, err := page.Content()
		if err == nil && LooksLikeReady(html) {
			return nil
		}
		if time.Now().After(deadline) {
			return nil
		}
		page.WaitForTimeout(400)
	}
}

func LooksLikeChallenge(html string) bool {
	if html == "" {
		return true
	}
	return strings.Contains(html, "Antibot Challenge") ||
		strings.Contains(html, "challengeURL") ||
		strings.Contains(html, "ozon-antibot") ||
		(strings.Contains(html, "incidentId") && !strings.Contains(html, "widgetStates"))
}

func LooksLikeReady(html string) bool {
	if LooksLikeChallenge(html) {
		return false
	}
	return strings.Contains(html, "widgetStates") ||
		strings.Contains(html, "webProductHeading") ||
		strings.Contains(html, `"@type":"Product"`) ||
		strings.Contains(html, "og:title") ||
		len(html) > 8000
}

func evaluateFetch(raw any) (int, []byte, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return 0, nil, fmt.Errorf("page fetch: unexpected result %T", raw)
	}
	body, _ := m["body"].(string)
	status := 0
	switch v := m["status"].(type) {
	case float64:
		status = int(v)
	case int:
		status = v
	}
	return status, []byte(body), nil
}

func sameRequestURL(warmupURL, requestURL string) bool {
	return strings.TrimRight(warmupURL, "/") == strings.TrimRight(requestURL, "/")
}

func (c *Client) acquire(ctx context.Context) error {
	select {
	case c.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) release() {
	select {
	case <-c.sem:
	default:
	}
}

func newPage(browser playwright.Browser, opts Options) (playwright.BrowserContext, playwright.Page, error) {
	ua := opts.UserAgent
	bctx, err := browser.NewContext(playwright.BrowserNewContextOptions{
		UserAgent:        &ua,
		Viewport:         &playwright.Size{Width: 1920, Height: 1080},
		Locale:           strPtr("ru-RU"),
		TimezoneId:       strPtr("Europe/Minsk"),
		ExtraHttpHeaders: map[string]string{"Accept-Language": "ru-RU,ru;q=0.9"},
	})
	if err != nil {
		return nil, nil, err
	}

	if err := bctx.AddInitScript(playwright.Script{
		Content: strPtr(stealthInitScript),
	}); err != nil {
		_ = bctx.Close()
		return nil, nil, err
	}

	page, err := bctx.NewPage()
	if err != nil {
		_ = bctx.Close()
		return nil, nil, err
	}
	return bctx, page, nil
}

func strPtr(v string) *string { return &v }
