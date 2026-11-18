package search

import "encoding/json"

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

type FacetingSettings struct {
	MaxValuesPerFacet int `json:"maxValuesPerFacet,omitempty"`
}

type PaginationSettings struct {
	MaxTotalHits int `json:"maxTotalHits,omitempty"`
}

type TypoTolerance struct {
	Enabled             bool           `json:"enabled"`
	MinWordSizeForTypos map[string]int `json:"minWordSizeForTypos,omitempty"`
	DisableOnWords      []string       `json:"disableOnWords,omitempty"`
	DisableOnAttributes []string       `json:"disableOnAttributes,omitempty"`
}

// MarshalJSON keeps omitted settings distinct from explicit empty values.
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
	var words *[]string
	if s.StopWords != nil {
		words = &s.StopWords
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
		StopWords:            words,
		Synonyms:             synonyms,
		TypoTolerance:        s.TypoTolerance,
		DistinctAttribute:    s.DistinctAttribute,
		Faceting:             s.Faceting,
		Pagination:           s.Pagination,
	})
}
