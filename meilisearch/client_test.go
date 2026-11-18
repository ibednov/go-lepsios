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

func TestIndexSettingsDistinguishOmittedAndEmptyValues(t *testing.T) {
	untouched, err := json.Marshal(IndexSettings{SearchableAttributes: []string{"name"}})
	if err != nil {
		t.Fatal(err)
	}
	var untouchedBody map[string]any
	if err := json.Unmarshal(untouched, &untouchedBody); err != nil {
		t.Fatal(err)
	}
	if _, exists := untouchedBody["stopWords"]; exists {
		t.Fatalf("nil stopWords should be omitted: %s", untouched)
	}
	if _, exists := untouchedBody["synonyms"]; exists {
		t.Fatalf("nil synonyms should be omitted: %s", untouched)
	}

	clearValues, err := json.Marshal(IndexSettings{StopWords: []string{}, Synonyms: map[string][]string{}})
	if err != nil {
		t.Fatal(err)
	}
	var clearBody map[string]any
	if err := json.Unmarshal(clearValues, &clearBody); err != nil {
		t.Fatal(err)
	}
	if words, ok := clearBody["stopWords"].([]any); !ok || len(words) != 0 {
		t.Fatalf("empty stopWords should be serialized to clear: %s", clearValues)
	}
	if synonyms, ok := clearBody["synonyms"].(map[string]any); !ok || len(synonyms) != 0 {
		t.Fatalf("empty synonyms should be serialized to clear: %s", clearValues)
	}
}

func TestMultiSearchReturnsFacetData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/multi-search" || r.Method != http.MethodPost {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		var body MultiSearchRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Queries) != 1 || body.Queries[0].IndexUID != "catalog" || len(body.Queries[0].Facets) != 1 || body.Queries[0].Facets[0] != "attributes.country" {
			t.Fatalf("request body = %+v", body)
		}
		_, _ = w.Write([]byte(`{"results":[{"hits":[],"facetDistribution":{"attributes.country":{"cn":3}},"facetStats":{"range_attributes.weight":{"min":1,"max":4}}}]}`))
	}))
	defer server.Close()
	client, err := New(Config{URL: server.URL, Client: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.MultiSearch(context.Background(), MultiSearchRequest{Queries: []MultiSearchQuery{{IndexUID: "catalog", SearchRequest: SearchRequest{Facets: []string{"attributes.country"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Results) != 1 || result.Results[0].FacetDistribution["attributes.country"]["cn"] != 3 || result.Results[0].FacetStats["range_attributes.weight"]["max"] != 4 {
		t.Fatalf("result = %+v", result)
	}
}

func TestSwapIndexPairsSendsOneAtomicBatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/swap-indexes" {
			t.Fatalf("request=%s %s", r.Method, r.URL.Path)
		}
		var body []struct {
			Indexes []string `json:"indexes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body) != 2 || len(body[0].Indexes) != 2 || body[0].Indexes[0] != "catalog" || body[0].Indexes[1] != "catalog_staging" || body[1].Indexes[0] != "facets" || body[1].Indexes[1] != "facets_staging" {
			t.Fatalf("swap body=%+v", body)
		}
		_, _ = w.Write([]byte(`{"uid":12,"status":"enqueued"}`))
	}))
	defer server.Close()
	client, err := New(Config{URL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	task, err := client.SwapIndexPairs(context.Background(), [][2]string{{"catalog", "catalog_staging"}, {"facets", "facets_staging"}})
	if err != nil || task.UID != 12 {
		t.Fatalf("task=%+v err=%v", task, err)
	}
}
