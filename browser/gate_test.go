package browser

import (
	"context"
	"errors"
	"testing"
	"time"
)

type flakyCapturer struct {
	calls int
	err   error
	body  []byte
}

func (f *flakyCapturer) CaptureJSON(context.Context, string, string) ([]byte, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.body, nil
}

func (f *flakyCapturer) FetchInPage(context.Context, string, string) (PageSnapshot, error) {
	f.calls++
	if f.err != nil {
		return PageSnapshot{}, f.err
	}
	return PageSnapshot{Body: f.body}, nil
}

func TestGateSkipsAfterHardFailure(t *testing.T) {
	inner := &flakyCapturer{err: ErrRemoteUnavailable}
	g := NewGate(inner, 50*time.Millisecond)

	if _, err := g.CaptureJSON(context.Background(), "https://www.wildberries.by/x", "**/detail**"); !errors.Is(err, ErrRemoteUnavailable) {
		t.Fatalf("first: %v", err)
	}
	if inner.calls != 1 {
		t.Fatalf("calls=%d", inner.calls)
	}
	if _, err := g.CaptureJSON(context.Background(), "https://www.wildberries.by/x", "**/detail**"); !errors.Is(err, ErrRemoteUnavailable) {
		t.Fatalf("gated: %v", err)
	}
	if inner.calls != 1 {
		t.Fatalf("cooldown must not call inner again, calls=%d", inner.calls)
	}

	time.Sleep(60 * time.Millisecond)
	inner.err = nil
	inner.body = []byte(`{"ok":true}`)
	body, err := g.CaptureJSON(context.Background(), "https://www.wildberries.by/x", "**/detail**")
	if err != nil || string(body) != `{"ok":true}` || inner.calls != 2 {
		t.Fatalf("after cooldown body=%s err=%v calls=%d", body, err, inner.calls)
	}
}

func TestGateFetchInPageUsesSameCooldown(t *testing.T) {
	inner := &flakyCapturer{err: errors.New("dial tcp 127.0.0.1:8090: connection refused")}
	raw := NewGate(inner, time.Minute)
	g, ok := raw.(*Gate)
	if !ok {
		t.Fatal("expected *Gate")
	}
	if _, err := g.FetchInPage(context.Background(), "https://www.wildberries.by/x", "https://www.wildberries.by/y"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := g.FetchInPage(context.Background(), "https://www.wildberries.by/x", "https://www.wildberries.by/y"); !errors.Is(err, ErrRemoteUnavailable) {
		t.Fatalf("second should be cool-down: %v", err)
	}
	if inner.calls != 1 {
		t.Fatalf("calls=%d", inner.calls)
	}
}

func TestNewGateNil(t *testing.T) {
	if NewGate(nil, 0) != nil {
		t.Fatal("nil inner")
	}
}
