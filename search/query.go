package search

// SearchRequest is the product-neutral subset of common document-search
// parameters. Filters use the target engine's documented expression syntax.
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

// SuggestRequest is an autocomplete-oriented query. Engines may implement it
// natively or translate it to a ranked SearchRequest.
type SuggestRequest struct {
	Q          string   `json:"q"`
	Limit      int      `json:"limit,omitempty"`
	Filter     []string `json:"filter,omitempty"`
	SearchOn   []string `json:"attributesToSearchOn,omitempty"`
	Attributes []string `json:"attributesToRetrieve,omitempty"`
	Highlight  []string `json:"attributesToHighlight,omitempty"`
}

type SearchResult struct {
	Hits               []map[string]any              `json:"hits"`
	Query              string                        `json:"query"`
	ProcessingTimeMS   int                           `json:"processingTimeMs"`
	EstimatedTotalHits int                           `json:"estimatedTotalHits"`
	FacetDistribution  map[string]map[string]int     `json:"facetDistribution,omitempty"`
	FacetStats         map[string]map[string]float64 `json:"facetStats,omitempty"`
}

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
