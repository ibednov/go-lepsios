package browser

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRemoteFetchInPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != remotePathFetchInPage {
			t.Fatalf("path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("auth %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		var req remoteFetchRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatal(err)
		}
		if req.WarmupURL == "" || req.RequestURL == "" {
			t.Fatal("empty urls")
		}
		_ = json.NewEncoder(w).Encode(remoteFetchResponse{
			Body: []byte(`{"widgetStates":{}}`),
			HTML: []byte("<html></html>"),
		})
	}))
	defer srv.Close()

	c := NewRemote(srv.URL, "secret", time.Second)
	snap, err := c.FetchInPage(context.Background(), "https://ozon.by/product/x-1/", "https://ozon.by/api/composer-api.bx/page/json/v2?url=/product/x-1/")
	if err != nil {
		t.Fatal(err)
	}
	if string(snap.Body) != `{"widgetStates":{}}` || string(snap.HTML) != "<html></html>" {
		t.Fatalf("%+v", snap)
	}
}

func TestRemoteCaptureJSONAndErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case remotePathCaptureJSON:
			_ = json.NewEncoder(w).Encode(remoteCaptureResponse{Body: []byte(`{"ok":true}`)})
		default:
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer srv.Close()

	c := NewRemote(srv.URL, "", time.Second)
	body, err := c.CaptureJSON(context.Background(), "https://www.wildberries.ru/catalog/1/detail.aspx", "**/cards/v4/detail**")
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"ok":true}` {
		t.Fatalf("%s", body)
	}

	if _, err := c.FetchInPage(context.Background(), "https://ozon.by/product/x-1/", "https://ozon.by/api/x"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("unauthorized: %v", err)
	}
	if _, err := c.FetchInPage(context.Background(), "https://example.com/", "https://ozon.by/x"); !errors.Is(err, ErrHostNotAllowed) {
		t.Fatalf("allowlist: %v", err)
	}

	var nilRemote *RemoteClient
	if _, err := nilRemote.FetchInPage(context.Background(), "https://ozon.by/x", "https://ozon.by/y"); !errors.Is(err, ErrRemoteUnavailable) {
		t.Fatalf("nil remote: %v", err)
	}
}
