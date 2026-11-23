package crawler

import (
	"time"

	"github.com/ibednov/go-lepsios/crawler/httpclient"
	"github.com/shopspring/decimal"
)

const MaxDiagnosticBodyBytes = httpclient.MaxDiagnosticBodyBytes

// DiagnosticResponse contains a bounded response body and safe metadata.
// BodyHash hashes BytesRead bytes, including bytes beyond Body when storage
// truncates the body. BodyTruncated means the HTTP client's read limit hit.
type DiagnosticResponse = httpclient.DiagnosticResponse

// DiagnosticAdapter optionally exposes the latest response for failure-only
// persistence by the crawl runner.
type DiagnosticAdapter interface {
	LastDiagnosticResponse() (DiagnosticResponse, bool)
}

// ProductRef is a discoverable product pointer from a source catalog.
type ProductRef struct {
	SourceCode string
	URL        string
	SourceID   string // optional shop-side id/sku
}

// UnitHint is the physical unit of the raw offer before normalization.
type UnitHint string

const (
	UnitG       UnitHint = "g"
	UnitPiece   UnitHint = "piece"
	UnitUnknown UnitHint = "unknown"
)

// RawOffer is one weight/variant offer before product matching.
type RawOffer struct {
	Source           string
	URL              string
	SourceSKU        string
	Title            string
	Kind             string
	PriceMode        string
	KindConfidence   float64
	KindNeedsReview  bool
	KindReviewReason string
	Price            decimal.Decimal
	Currency         string
	WeightG          *int
	// WeightGrams carries exact mass while WeightG remains for legacy adapters.
	WeightGrams *decimal.Decimal
	UnitHint    UnitHint
	InStock     bool
	ImageURL    string
	Brand       string
	Attrs       map[string]string
	ObservedAt  time.Time
}
