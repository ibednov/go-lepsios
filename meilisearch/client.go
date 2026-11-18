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

func New(cfg Config) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.URL), "/")
	if base == "" {
		return nil, fmt.Errorf("meilisearch: URL is required")
	}
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("meilisearch: invalid URL")
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
	Q           string   `json:"q"`
	Limit       int      `json:"limit,omitempty"`
	Offset      int      `json:"offset,omitempty"`
	Filter      []string `json:"filter,omitempty"`
	Attributes  []string `json:"attributesToRetrieve,omitempty"`
	Highlight   []string `json:"attributesToHighlight,omitempty"`
	ShowMatches bool     `json:"showMatchesPosition,omitempty"`
}

type SearchResult struct {
	Hits               []map[string]any `json:"hits"`
	Query              string           `json:"query"`
	ProcessingTimeMS   int              `json:"processingTimeMs"`
	EstimatedTotalHits int              `json:"estimatedTotalHits"`
}

type IndexSettings struct {
	SearchableAttributes []string            `json:"searchableAttributes,omitempty"`
	DisplayedAttributes  []string            `json:"displayedAttributes,omitempty"`
	FilterableAttributes []string            `json:"filterableAttributes,omitempty"`
	SortableAttributes   []string            `json:"sortableAttributes,omitempty"`
	StopWords            []string            `json:"stopWords,omitempty"`
	Synonyms             map[string][]string `json:"synonyms,omitempty"`
	TypoTolerance        *TypoTolerance      `json:"typoTolerance,omitempty"`
}

type TypoTolerance struct {
	Enabled             bool           `json:"enabled"`
	MinWordSizeForTypos map[string]int `json:"minWordSizeForTypos,omitempty"`
	DisableOnWords      []string       `json:"disableOnWords,omitempty"`
	DisableOnAttributes []string       `json:"disableOnAttributes,omitempty"`
}

type Task struct {
	TaskUID    int64  `json:"taskUid"`
	IndexUID   string `json:"indexUid"`
	Status     string `json:"status"`
	Type       string `json:"type"`
	EnqueuedAt string `json:" enqueuedAt"`
}

func (c *Client) Search(ctx context.Context, index string, req SearchRequest) (SearchResult, error) {
	var result SearchResult
	err := c.do(ctx, http.MethodPost, "/indexes/"+url.PathEscape(index)+"/search", req, &result)
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
		return fmt.Errorf("meilisearch request returned HTTP %d", resp.StatusCode)
	}
	if out != nil {
		if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(out); err != nil {
			return fmt.Errorf("meilisearch decode response: %w", err)
		}
	}
	return nil
}
