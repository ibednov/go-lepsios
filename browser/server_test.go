package browser

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubPages struct {
	snap PageSnapshot
	body []byte
	err  error
}

func (s stubPages) FetchInPage(context.Context, string, string) (PageSnapshot, error) {
	return s.snap, s.err
}

func (s stubPages) CaptureJSON(context.Context, string, string) ([]byte, error) {
	return s.body, s.err
}

func TestHealth(t *testing.T) {
	srv := NewServer(stubPages{}, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestFetchInPageRequiresTokenAndAllowlist(t *testing.T) {
	srv := NewServer(stubPages{snap: PageSnapshot{Body: []byte(`{"ok":true}`)}}, "secret")
	h := srv.Handler()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/fetch-in-page", bytes.NewBufferString(
		`{"warmup_url":"https://ozon.by/product/x-1/","request_url":"https://ozon.by/api/composer-api.bx/page/json/v2?url=/product/x-1/"}`,
	))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/internal/v1/fetch-in-page", bytes.NewBufferString(
		`{"warmup_url":"https://example.com/","request_url":"https://ozon.by/product/x-1/"}`,
	))
	req.Header.Set("Authorization", "Bearer secret")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("foreign host: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/internal/v1/fetch-in-page", bytes.NewBufferString(
		`{"warmup_url":"https://ozon.by/product/x-1/","request_url":"https://ozon.by/api/composer-api.bx/page/json/v2?url=/product/x-1/"}`,
	))
	req.Header.Set("Authorization", "Bearer secret")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ok: %d %s", rec.Code, rec.Body.String())
	}
	var out fetchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if string(out.Body) != `{"ok":true}` {
		t.Fatalf("%s", out.Body)
	}
}
