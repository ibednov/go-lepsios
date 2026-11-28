package meilisearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestFilterQuoting(t *testing.T) {
	if got := QuoteFilter(`a"b\c`); got != `"a\"b\\c"` {
		t.Fatalf("QuoteFilter = %s", got)
	}
	if got := InFilter("brand", []string{"Apple", `x"y`}); got != `brand IN ["Apple", "x\"y"]` {
		t.Fatalf("InFilter = %s", got)
	}
}

func TestEnsureIndexCreatesMissingAndSkipsExisting(t *testing.T) {
	var created atomic.Int32
	exists := map[string]bool{"have": true}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/indexes/have":
			_, _ = w.Write([]byte(`{"uid":"have","primaryKey":"id"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/indexes/new":
			if exists["new"] {
				_, _ = w.Write([]byte(`{"uid":"new","primaryKey":"id"}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && r.URL.Path == "/indexes":
			created.Add(1)
			exists["new"] = true
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"taskUid":1,"status":"enqueued"}`))
		case r.URL.Path == "/tasks/1":
			_, _ = w.Write([]byte(`{"uid":1,"status":"succeeded"}`))
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	c, err := New(Config{URL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.EnsureIndex(context.Background(), "have", "id"); err != nil {
		t.Fatal(err)
	}
	if err := c.EnsureIndex(context.Background(), "new", "id"); err != nil {
		t.Fatal(err)
	}
	if created.Load() != 1 {
		t.Fatalf("created = %d, want 1", created.Load())
	}
}

func TestWriteBatchedAwaitsEachBatchAndMerges(t *testing.T) {
	var batches []int
	var task int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/indexes/items/documents":
			if r.Method != http.MethodPut {
				t.Fatalf("merge must PUT, got %s", r.Method)
			}
			var docs []map[string]any
			_ = json.NewDecoder(r.Body).Decode(&docs)
			batches = append(batches, len(docs))
			task++
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]any{"taskUid": task})
		default: // /tasks/N
			_, _ = w.Write([]byte(`{"status":"succeeded"}`))
		}
	}))
	defer server.Close()
	c, _ := New(Config{URL: server.URL})
	docs := make([]map[string]int, 5)
	for i := range docs {
		docs[i] = map[string]int{"id": i}
	}
	n, err := WriteBatched(context.Background(), c, "items", "id", docs, 2, Merge)
	if err != nil || n != 5 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if len(batches) != 3 || batches[0] != 2 || batches[2] != 1 {
		t.Fatalf("batches = %v", batches)
	}
}

func TestSettingsMarshalLocalizedAttributes(t *testing.T) {
	b, err := json.Marshal(IndexSettings{LocalizedAttributes: []LocalizedAttributes{{AttributePatterns: []string{"name_ru"}, Locales: []string{"rus"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"localizedAttributes":[{"attributePatterns":["name_ru"],"locales":["rus"]}]}` {
		t.Fatalf("settings = %s", b)
	}
}
