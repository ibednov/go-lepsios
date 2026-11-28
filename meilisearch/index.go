package meilisearch

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Index is the subset of GET /indexes/:uid this package uses.
type Index struct {
	UID        string `json:"uid"`
	PrimaryKey string `json:"primaryKey"`
}

// IndexStats is the subset of GET /indexes/:uid/stats this package uses.
type IndexStats struct {
	NumberOfDocuments int  `json:"numberOfDocuments"`
	IsIndexing        bool `json:"isIndexing"`
}

// IsNotFound reports whether err is a Meilisearch HTTP 404.
func IsNotFound(err error) bool {
	var he *HTTPError
	return errors.As(err, &he) && he.StatusCode == http.StatusNotFound
}

func indexPath(uid string) string { return "/indexes/" + url.PathEscape(uid) }

// GetIndex reads one index (IsNotFound(err) when it does not exist).
func (c *Client) GetIndex(ctx context.Context, uid string) (Index, error) {
	var idx Index
	err := c.do(ctx, http.MethodGet, indexPath(uid), nil, &idx)
	return idx, err
}

// EnsureIndex creates the index when it is missing and waits for the creation
// task; an existing index is left untouched. Pair it with UpdateSettings +
// WaitForTask so the index is fully configured before the first write.
func (c *Client) EnsureIndex(ctx context.Context, uid, primaryKey string) error {
	if _, err := c.GetIndex(ctx, uid); err == nil {
		return nil
	} else if !IsNotFound(err) {
		return err
	}
	task, err := c.CreateIndex(ctx, uid, primaryKey)
	if err != nil {
		return err
	}
	if err := c.WaitForTask(ctx, task.TaskUID); err != nil {
		if _, getErr := c.GetIndex(ctx, uid); getErr == nil {
			return nil // lost a creation race; the index exists
		}
		return err
	}
	return nil
}

// DeleteIndex deletes an index and waits; a missing index is not an error.
func (c *Client) DeleteIndex(ctx context.Context, uid string) error {
	var task Task
	if err := c.do(ctx, http.MethodDelete, indexPath(uid), nil, &task); err != nil {
		if IsNotFound(err) {
			return nil
		}
		return err
	}
	return c.WaitForTask(ctx, task.TaskUID)
}

// Stats reads index stats (document count, indexing flag).
func (c *Client) Stats(ctx context.Context, uid string) (IndexStats, error) {
	var st IndexStats
	err := c.do(ctx, http.MethodGet, indexPath(uid)+"/stats", nil, &st)
	return st, err
}

// UpdateDocuments adds-or-MERGES documents (PUT): fields a document omits keep
// their stored values. AddDocuments (POST) replaces whole documents.
func (c *Client) UpdateDocuments(ctx context.Context, index, primaryKey string, documents any) (Task, error) {
	path := indexPath(index) + "/documents"
	if primaryKey != "" {
		path += "?primaryKey=" + url.QueryEscape(primaryKey)
	}
	var task Task
	err := c.do(ctx, http.MethodPut, path, documents, &task)
	return task, err
}

// WriteMode selects replace (POST) or merge (PUT) document writes.
type WriteMode int

const (
	// Replace writes whole documents (AddDocuments).
	Replace WriteMode = iota
	// Merge partially updates documents (UpdateDocuments).
	Merge
)

// WriteBatched writes docs in batches of batchSize, waiting for each batch's
// task before sending the next. This keeps both the client payload and the
// Meilisearch indexing memory bounded (no queued backlog) — the safe default
// on RAM-tight hosts. Returns the number of documents acknowledged.
func WriteBatched[T any](ctx context.Context, c *Client, index, primaryKey string, docs []T, batchSize int, mode WriteMode) (int, error) {
	if batchSize <= 0 {
		batchSize = 500
	}
	done := 0
	for start := 0; start < len(docs); start += batchSize {
		end := min(start+batchSize, len(docs))
		var (
			task Task
			err  error
		)
		if mode == Merge {
			task, err = c.UpdateDocuments(ctx, index, primaryKey, docs[start:end])
		} else {
			task, err = c.AddDocuments(ctx, index, primaryKey, docs[start:end])
		}
		if err != nil {
			return done, fmt.Errorf("meilisearch write batch %d-%d: %w", start, end, err)
		}
		if err := c.WaitForTask(ctx, task.TaskUID); err != nil {
			return done, fmt.Errorf("meilisearch index batch %d-%d: %w", start, end, err)
		}
		done = end
	}
	return done, nil
}

// QuoteFilter returns s as a double-quoted filter string literal. Use it for
// every caller-supplied value that enters a filter expression.
func QuoteFilter(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// InFilter builds `attr IN ["a", "b"]` with every value quoted.
func InFilter(attr string, values []string) string {
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = QuoteFilter(v)
	}
	return attr + " IN [" + strings.Join(quoted, ", ") + "]"
}
