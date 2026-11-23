package wildberries

import (
	"bytes"
	"context"
	"sync"

	sidecarparser "github.com/ibednov/go-lepsios/parser"
)

const wbTraceBodyLimit = 240

type wbTraceKey struct{}

// wbTrace is appended to from parallel basket probes.
type wbTrace struct {
	mu    sync.Mutex
	calls []sidecarparser.WBCall
}

func (t *wbTrace) snapshot() []sidecarparser.WBCall {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]sidecarparser.WBCall(nil), t.calls...)
}

func withWBTrace(ctx context.Context) (context.Context, *wbTrace) {
	trace := &wbTrace{}
	return context.WithValue(ctx, wbTraceKey{}, trace), trace
}

func recordWB(ctx context.Context, call sidecarparser.WBCall) {
	trace, ok := ctx.Value(wbTraceKey{}).(*wbTrace)
	if !ok || trace == nil {
		return
	}
	call.Body = truncateWBBody(call.Body)
	trace.mu.Lock()
	trace.calls = append(trace.calls, call)
	trace.mu.Unlock()
}

func recordWBHTTP(ctx context.Context, name, method, reqURL string, status int, body []byte, callErr error) {
	call := sidecarparser.WBCall{
		Name:   name,
		Method: method,
		URL:    reqURL,
		Status: status,
	}
	if callErr != nil {
		call.Error = callErr.Error()
	}
	if len(body) > 0 {
		call.Body = string(body)
	}
	recordWB(ctx, call)
}

func truncateWBBody(body string) string {
	if body == "" {
		return ""
	}
	raw := []byte(body)
	if i := bytes.Index(raw, []byte("application/ld+json")); i >= 0 {
		end := i + wbTraceBodyLimit
		if end > len(raw) {
			end = len(raw)
		}
		return string(raw[i:end])
	}
	if len(body) > wbTraceBodyLimit {
		return body[:wbTraceBodyLimit]
	}
	return body
}
