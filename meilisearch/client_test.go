package meilisearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchSendsTenantKeyAndFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/indexes/project_items/search" || r.Method != http.MethodPost {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tenant-test-key" {
			t.Fatalf("authorization = %q", got)
		}
		var body SearchRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Q != "tea" || len(body.Filter) != 1 || body.Filter[0] != "kind = coffee" {
			t.Fatalf("search body = %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":[{"id":7}],"query":"tea","processingTimeMs":2,"estimatedTotalHits":1}`))
	}))
	defer server.Close()

	client, err := New(Config{URL: server.URL, APIKey: "tenant-test-key"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Search(context.Background(), "project_items", SearchRequest{Q: "tea", Filter: []string{"kind = coffee"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hits) != 1 || result.ProcessingTimeMS != 2 {
		t.Fatalf("result = %+v", result)
	}
}

func TestNewRejectsInvalidURL(t *testing.T) {
	if _, err := New(Config{URL: "file:///tmp/index"}); err == nil {
		t.Fatal("expected invalid URL error")
	}
}
