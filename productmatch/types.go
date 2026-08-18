package productmatch

// MatchType describes how closely two products match.
type MatchType string

const (
	MatchExact    MatchType = "exact"
	MatchProbable MatchType = "probable"
	MatchRelated  MatchType = "related"
	MatchNone     MatchType = "none"
)

// MatchReason explains why two products were considered similar.
type MatchReason string

const (
	ReasonSameURL              MatchReason = "same_url"
	ReasonSameFingerprint      MatchReason = "same_fingerprint"
	ReasonSameBrandModel       MatchReason = "same_brand_model"
	ReasonHighTitleSimilarity  MatchReason = "high_title_similarity"
	ReasonSameCategory         MatchReason = "same_category"
	ReasonTokenOverlap         MatchReason = "token_overlap"
)

// RawProductInput is normalized before comparison.
type RawProductInput struct {
	Title    string
	Brand    string
	Model    string
	Category string
	URL      string
}

// NormalizedProduct is a canonical profile used for matching.
type NormalizedProduct struct {
	OriginalTitle   string
	NormalizedTitle string
	Brand           string
	Model           string
	Category        string
	Tokens          []string
	Fingerprint     string
	NormalizedURL   string
}

// SimilarityResult is the outcome of comparing two products.
type SimilarityResult struct {
	Score     float64
	MatchType MatchType
	Reasons   []MatchReason
	Breakdown ScoreBreakdown
}

// ScoreBreakdown exposes individual signal weights for debugging.
type ScoreBreakdown struct {
	URLScore         float64
	FingerprintScore float64
	TitleScore       float64
	BrandModelScore  float64
	TokenScore       float64
}

// Thresholds configures match-type cutoffs.
type Thresholds struct {
	Exact    float64
	Probable float64
	Related  float64
}

// DefaultThresholds returns product defaults.
func DefaultThresholds() Thresholds {
	return Thresholds{
		Exact:    0.90,
		Probable: 0.72,
		Related:  0.50,
	}
}
