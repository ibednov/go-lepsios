package browser

import (
	"context"
	"errors"
	"testing"
)

func TestCaptureJSONRejectsEmptyArgs(t *testing.T) {
	c := &Client{sem: make(chan struct{}, 1), opts: Options{}.withDefaults()}
	if _, err := c.CaptureJSON(context.Background(), "", "**/x**"); !errors.Is(err, ErrEmptyPageURL) {
		t.Fatalf("empty page: %v", err)
	}
	if _, err := c.CaptureJSON(context.Background(), "https://example.com", ""); !errors.Is(err, ErrEmptyGlob) {
		t.Fatalf("empty glob: %v", err)
	}
}

func TestCaptureJSONClosed(t *testing.T) {
	c := &Client{closed: true, sem: make(chan struct{}, 1), opts: Options{}.withDefaults()}
	_, err := c.CaptureJSON(context.Background(), "https://example.com", "**/x**")
	if !errors.Is(err, ErrClosed) {
		t.Fatalf("closed: %v", err)
	}
}

func TestNilClientCloseAndCapture(t *testing.T) {
	var c *Client
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CaptureJSON(context.Background(), "https://example.com", "**/x**"); !errors.Is(err, ErrClosed) {
		t.Fatalf("nil client: %v", err)
	}
}

func TestOptionsDefaults(t *testing.T) {
	o := Options{}.withDefaults()
	if o.Timeout != defaultTimeout || o.UserAgent == "" || o.MaxConcurrent != 1 || o.Warmup != defaultWarmup {
		t.Fatalf("%+v", o)
	}
}

func TestFetchInPageRejectsEmptyAndClosed(t *testing.T) {
	c := &Client{sem: make(chan struct{}, 1), opts: Options{}.withDefaults()}
	if _, err := c.FetchInPage(context.Background(), "", "https://ozon.by/x"); !errors.Is(err, ErrEmptyPageURL) {
		t.Fatalf("empty warmup: %v", err)
	}
	if _, err := c.FetchInPage(context.Background(), "https://ozon.by/x", ""); !errors.Is(err, ErrEmptyRequestURL) {
		t.Fatalf("empty request: %v", err)
	}
	if _, err := c.FetchInPage(context.Background(), "https://example.com/", "https://ozon.by/x"); !errors.Is(err, ErrHostNotAllowed) {
		t.Fatalf("foreign host: %v", err)
	}

	closed := &Client{closed: true, sem: make(chan struct{}, 1), opts: Options{}.withDefaults()}
	if _, err := closed.FetchInPage(context.Background(), "https://ozon.by/x", "https://ozon.by/api"); !errors.Is(err, ErrClosed) {
		t.Fatalf("closed: %v", err)
	}

	var nilClient *Client
	if _, err := nilClient.FetchInPage(context.Background(), "https://ozon.by/x", "https://ozon.by/api"); !errors.Is(err, ErrClosed) {
		t.Fatalf("nil: %v", err)
	}
}
