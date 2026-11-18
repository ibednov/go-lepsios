package meilisearch

import (
	"context"
	"encoding/json"
	"fmt"

	searchapi "github.com/ibednov/go-lepsios/search"
)

var _ searchapi.Engine = (*Client)(nil)

type index struct {
	client *Client
	uid    string
}

func (c *Client) Index(uid string) searchapi.Index {
	return &index{client: c, uid: uid}
}

func (i *index) UID() string { return i.uid }

func (i *index) Search(ctx context.Context, request searchapi.SearchRequest) (searchapi.SearchResult, error) {
	var req SearchRequest
	if err := convertJSON(request, &req); err != nil {
		return searchapi.SearchResult{}, err
	}
	result, err := i.client.Search(ctx, i.uid, req)
	if err != nil {
		return searchapi.SearchResult{}, err
	}
	var out searchapi.SearchResult
	if err := convertJSON(result, &out); err != nil {
		return searchapi.SearchResult{}, err
	}
	return out, nil
}

func (i *index) Suggest(ctx context.Context, request searchapi.SuggestRequest) (searchapi.SearchResult, error) {
	req := SearchRequest{Q: request.Q, Limit: request.Limit, Filter: request.Filter,
		AttributesToSearchOn: request.SearchOn, Attributes: request.Attributes, Highlight: request.Highlight}
	result, err := i.client.Search(ctx, i.uid, req)
	if err != nil {
		return searchapi.SearchResult{}, err
	}
	var out searchapi.SearchResult
	if err := convertJSON(result, &out); err != nil {
		return searchapi.SearchResult{}, err
	}
	return out, nil
}

func (i *index) AddDocuments(ctx context.Context, primaryKey string, documents any) (searchapi.Task, error) {
	return i.client.AddDocuments(ctx, i.uid, primaryKey, documents)
}

func (i *index) UpdateSettings(ctx context.Context, settings searchapi.IndexSettings) (searchapi.Task, error) {
	var meiliSettings IndexSettings
	if err := convertJSON(settings, &meiliSettings); err != nil {
		return searchapi.Task{}, err
	}
	return i.client.UpdateSettings(ctx, i.uid, meiliSettings)
}

func (i *index) DeleteAllDocuments(ctx context.Context) (searchapi.Task, error) {
	return i.client.DeleteAllDocuments(ctx, i.uid)
}

func (i *index) DeleteDocumentsByFilter(ctx context.Context, filters []string) (searchapi.Task, error) {
	return i.client.DeleteDocumentsByFilter(ctx, i.uid, filters)
}

func (i *index) DeleteDocuments(ctx context.Context, ids []int64) (searchapi.Task, error) {
	return i.client.DeleteDocuments(ctx, i.uid, ids)
}

func convertJSON[T any](input any, output *T) error {
	data, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("meilisearch convert request: %w", err)
	}
	if err := json.Unmarshal(data, output); err != nil {
		return fmt.Errorf("meilisearch convert response: %w", err)
	}
	return nil
}
