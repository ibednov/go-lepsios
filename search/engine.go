// Package search defines a vendor-neutral API for document search engines.
package search

import "context"

// Engine manages indexes and engine-wide operations.
type Engine interface {
	Index(uid string) Index
	SearchMulti(ctx context.Context, request MultiSearchRequest) (MultiSearchResult, error)
	CreateIndex(ctx context.Context, uid, primaryKey string) (Task, error)
	SwapIndexes(ctx context.Context, first, second string) (Task, error)
	SwapIndexPairs(ctx context.Context, pairs [][2]string) (Task, error)
	WaitForTask(ctx context.Context, taskUID int64) error
	Health(ctx context.Context) error
}

// Index reads and mutates documents in one named index.
type Index interface {
	UID() string
	Search(ctx context.Context, request SearchRequest) (SearchResult, error)
	Suggest(ctx context.Context, request SuggestRequest) (SearchResult, error)
	AddDocuments(ctx context.Context, primaryKey string, documents any) (Task, error)
	UpdateSettings(ctx context.Context, settings IndexSettings) (Task, error)
	DeleteAllDocuments(ctx context.Context) (Task, error)
	DeleteDocumentsByFilter(ctx context.Context, filters []string) (Task, error)
	DeleteDocuments(ctx context.Context, ids []int64) (Task, error)
}

// IndexStatsReader is an optional capability supported by engines that expose
// index operational metadata. Keeping it separate from Index preserves support
// for providers that cannot report statistics.
type IndexStatsReader interface {
	Stats(ctx context.Context) (IndexStats, error)
}

// IndexStats contains vendor-neutral index counters and state.
type IndexStats struct {
	NumberOfDocuments int64          `json:"numberOfDocuments"`
	IsIndexing        bool           `json:"isIndexing"`
	FieldDistribution map[string]int `json:"fieldDistribution,omitempty"`
}
