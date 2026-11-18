// Package meilisearch provides a small product-neutral HTTP client for the
// Meilisearch document and index settings APIs.
package meilisearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	searchapi "github.com/ibednov/go-lepsios/search"
)

type Config struct {
	URL     string
	APIKey  string
	Timeout time.Duration
	Client  *http.Client
}

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

var _ searchapi.Engine = (*Client)(nil)

func (c *Client) SearchMulti(ctx context.Context, request searchapi.MultiSearchRequest) (searchapi.MultiSearchResult, error) {
	var req MultiSearchRequest
	if err := convertJSON(request, &req); err != nil {
		return searchapi.MultiSearchResult{}, err
	}
	queries := make([]MultiSearchQuery, len(req.Queries))
	for i, query := range req.Queries {
		queries[i] = MultiSearchQuery{IndexUID: query.IndexUID, SearchRequest: query.SearchRequest}
	}
	result, err := c.MultiSearch(ctx, MultiSearchRequest{Queries: queries})
	if err != nil {
		return searchapi.MultiSearchResult{}, err
	}
	var out searchapi.MultiSearchResult
	if err := convertJSON(result, &out); err != nil {
		return searchapi.MultiSearchResult{}, err
	}
	return out, nil
}

func New(cfg Config) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.URL), "/")
	if base == "" {
		return nil, &searchapi.ConfigError{Field: "URL"}
	}
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, &searchapi.ConfigError{Field: "URL", Cause: err}
	}
	httpClient := cfg.Client
	if httpClient == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 350 * time.Millisecond
		}
		httpClient = &http.Client{Timeout: timeout}
	}
	return &Client{baseURL: base, apiKey: cfg.APIKey, http: httpClient}, nil
}

type SearchRequest struct {
	Q                    string   `json:"q"`
	Limit                int      `json:"limit,omitempty"`
	Offset               int      `json:"offset,omitempty"`
	Sort                 []string `json:"sort,omitempty"`
	Filter               []string `json:"filter,omitempty"`
	Facets               []string `json:"facets,omitempty"`
	Distinct             string   `json:"distinct,omitempty"`
	Attributes           []string `json:"attributesToRetrieve,omitempty"`
	AttributesToSearchOn []string `json:"attributesToSearchOn,omitempty"`
	Highlight            []string `json:"attributesToHighlight,omitempty"`
	ShowMatches          bool     `json:"showMatchesPosition,omitempty"`
}

type SuggestRequest = searchapi.SuggestRequest

type SearchResult struct {
	Hits               []map[string]any              `json:"hits"`
	Query              string                        `json:"query"`
	ProcessingTimeMS   int                           `json:"processingTimeMs"`
	EstimatedTotalHits int                           `json:"estimatedTotalHits"`
	FacetDistribution  map[string]map[string]int     `json:"facetDistribution,omitempty"`
	FacetStats         map[string]map[string]float64 `json:"facetStats,omitempty"`
}

// MultiSearchRequest runs several index searches in one request. IndexUID is
// required by Meilisearch for each query.
type MultiSearchRequest struct {
	Queries []MultiSearchQuery `json:"queries"`
}

type MultiSearchQuery struct {
	IndexUID string `json:"indexUid"`
	SearchRequest
}

type MultiSearchResult struct {
	Results []SearchResult `json:"results"`
}

type IndexSettings struct {
	SearchableAttributes []string            `json:"searchableAttributes,omitempty"`
	DisplayedAttributes  []string            `json:"displayedAttributes,omitempty"`
	FilterableAttributes []string            `json:"filterableAttributes,omitempty"`
	SortableAttributes   []string            `json:"sortableAttributes,omitempty"`
	RankingRules         []string            `json:"rankingRules,omitempty"`
	StopWords            []string            `json:"stopWords,omitempty"`
	Synonyms             map[string][]string `json:"synonyms,omitempty"`
	TypoTolerance        *TypoTolerance      `json:"typoTolerance,omitempty"`
	DistinctAttribute    string              `json:"distinctAttribute,omitempty"`
	Faceting             *FacetingSettings   `json:"faceting,omitempty"`
	Pagination           *PaginationSettings `json:"pagination,omitempty"`
}

// MarshalJSON preserves the distinction between an omitted setting (nil) and
// an explicit empty setting (non-nil but length zero). Meilisearch uses an
// empty array/map to clear a setting, while other callers may leave it untouched.
func (s IndexSettings) MarshalJSON() ([]byte, error) {
	type settingsJSON struct {
		SearchableAttributes []string             `json:"searchableAttributes,omitempty"`
		DisplayedAttributes  []string             `json:"displayedAttributes,omitempty"`
		FilterableAttributes []string             `json:"filterableAttributes,omitempty"`
		SortableAttributes   []string             `json:"sortableAttributes,omitempty"`
		RankingRules         []string             `json:"rankingRules,omitempty"`
		StopWords            *[]string            `json:"stopWords,omitempty"`
		Synonyms             *map[string][]string `json:"synonyms,omitempty"`
		TypoTolerance        *TypoTolerance       `json:"typoTolerance,omitempty"`
		DistinctAttribute    string               `json:"distinctAttribute,omitempty"`
		Faceting             *FacetingSettings    `json:"faceting,omitempty"`
		Pagination           *PaginationSettings  `json:"pagination,omitempty"`
	}
	var stopWords *[]string
	if s.StopWords != nil {
		stopWords = &s.StopWords
	}
	var synonyms *map[string][]string
	if s.Synonyms != nil {
		synonyms = &s.Synonyms
	}
	return json.Marshal(settingsJSON{
		SearchableAttributes: s.SearchableAttributes,
		DisplayedAttributes:  s.DisplayedAttributes,
		FilterableAttributes: s.FilterableAttributes,
		SortableAttributes:   s.SortableAttributes,
		RankingRules:         s.RankingRules,
		StopWords:            stopWords,
		Synonyms:             synonyms,
		TypoTolerance:        s.TypoTolerance,
		DistinctAttribute:    s.DistinctAttribute,
		Faceting:             s.Faceting,
		Pagination:           s.Pagination,
	})
}

type FacetingSettings struct {
	MaxValuesPerFacet int `json:"maxValuesPerFacet,omitempty"`
}

type PaginationSettings struct {
	MaxTotalHits int `json:"maxTotalHits,omitempty"`
}

type DeleteDocumentsRequest struct {
	Filter []string `json:"filter"`
}

type DeleteDocumentIDsRequest struct {
	IDs []int64 `json:"ids"`
}

type TypoTolerance struct {
	Enabled             bool           `json:"enabled"`
	MinWordSizeForTypos map[string]int `json:"minWordSizeForTypos,omitempty"`
	DisableOnWords      []string       `json:"disableOnWords,omitempty"`
	DisableOnAttributes []string       `json:"disableOnAttributes,omitempty"`
}

type Task = searchapi.Task

type HTTPError = searchapi.HTTPError

func (c *Client) CreateIndex(ctx context.Context, uid, primaryKey string) (Task, error) {
	var task Task
	err := c.do(ctx, http.MethodPost, "/indexes", map[string]string{"uid": uid, "primaryKey": primaryKey}, &task)
	return task, err
}

func (c *Client) Search(ctx context.Context, index string, req SearchRequest) (SearchResult, error) {
	var result SearchResult
	err := c.do(ctx, http.MethodPost, "/indexes/"+url.PathEscape(index)+"/search", req, &result)
	return result, err
}

func (c *Client) MultiSearch(ctx context.Context, req MultiSearchRequest) (MultiSearchResult, error) {
	var result MultiSearchResult
	err := c.do(ctx, http.MethodPost, "/multi-search", req, &result)
	return result, err
}

func (c *Client) AddDocuments(ctx context.Context, index, primaryKey string, documents any) (Task, error) {
	path := "/indexes/" + url.PathEscape(index) + "/documents"
	if primaryKey != "" {
		path += "?primaryKey=" + url.QueryEscape(primaryKey)
	}
	var task Task
	err := c.do(ctx, http.MethodPost, path, documents, &task)
	return task, err
}

func (c *Client) UpdateSettings(ctx context.Context, index string, settings IndexSettings) (Task, error) {
	var task Task
	err := c.do(ctx, http.MethodPatch, "/indexes/"+url.PathEscape(index)+"/settings", settings, &task)
	return task, err
}

func (c *Client) DeleteAllDocuments(ctx context.Context, index string) (Task, error) {
	var task Task
	err := c.do(ctx, http.MethodDelete, "/indexes/"+url.PathEscape(index)+"/documents", nil, &task)
	return task, err
}

func (c *Client) DeleteDocumentsByFilter(ctx context.Context, index string, filters []string) (Task, error) {
	var task Task
	err := c.do(ctx, http.MethodPost, "/indexes/"+url.PathEscape(index)+"/documents/delete", DeleteDocumentsRequest{Filter: filters}, &task)
	return task, err
}

func (c *Client) DeleteDocuments(ctx context.Context, index string, ids []int64) (Task, error) {
	var task Task
	err := c.do(ctx, http.MethodPost, "/indexes/"+url.PathEscape(index)+"/documents/delete", DeleteDocumentIDsRequest{IDs: ids}, &task)
	return task, err
}

func (c *Client) SwapIndexes(ctx context.Context, first, second string) (Task, error) {
	return c.SwapIndexPairs(ctx, [][2]string{{first, second}})
}

// SwapIndexPairs atomically swaps all supplied index pairs in one Meilisearch
// task. This is useful when multiple related indexes must move together.
func (c *Client) SwapIndexPairs(ctx context.Context, pairs [][2]string) (Task, error) {
	var task Task
	indexes := make([]map[string][]string, 0, len(pairs))
	for _, pair := range pairs {
		indexes = append(indexes, map[string][]string{"indexes": {pair[0], pair[1]}})
	}
	err := c.do(ctx, http.MethodPost, "/swap-indexes", indexes, &task)
	return task, err
}

func (c *Client) WaitForTask(ctx context.Context, taskUID int64) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		var task Task
		if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/tasks/%d", taskUID), nil, &task); err != nil {
			return err
		}
		switch task.Status {
		case "succeeded":
			return nil
		case "failed", "canceled":
			if task.Error != nil {
				return &searchapi.TaskError{ID: taskUID, Status: task.Status, Cause: task.Error.Message}
			}
			return &searchapi.TaskError{ID: taskUID, Status: task.Status}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (c *Client) Health(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/health", nil, nil)
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("meilisearch encode request: %w", err)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("meilisearch create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("meilisearch request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return &searchapi.HTTPError{StatusCode: resp.StatusCode}
	}
	if out != nil {
		if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(out); err != nil {
			return fmt.Errorf("meilisearch decode response: %w", err)
		}
	}
	return nil
}
