// Package meilisearch is a backwards-compatible facade for the search/meilisearch
// implementation. New consumers should depend on search.Engine and search.Index.
package meilisearch

import backend "github.com/ibednov/go-lepsios/search/meilisearch"

type Config = backend.Config
type Client = backend.Client
type SearchRequest = backend.SearchRequest
type SearchResult = backend.SearchResult
type SuggestRequest = backend.SuggestRequest
type MultiSearchRequest = backend.MultiSearchRequest
type MultiSearchQuery = backend.MultiSearchQuery
type MultiSearchResult = backend.MultiSearchResult
type IndexSettings = backend.IndexSettings
type FacetingSettings = backend.FacetingSettings
type PaginationSettings = backend.PaginationSettings
type TypoTolerance = backend.TypoTolerance
type Task = backend.Task
type HTTPError = backend.HTTPError
type DeleteDocumentsRequest = backend.DeleteDocumentsRequest
type DeleteDocumentIDsRequest = backend.DeleteDocumentIDsRequest

func New(config Config) (*Client, error) { return backend.New(config) }
