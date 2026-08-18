package productmatch

import (
	"math"
	"strings"
)

// CompareProducts scores similarity between two normalized products.
func CompareProducts(a, b NormalizedProduct) SimilarityResult {
	if a.NormalizedURL != "" && b.NormalizedURL != "" && a.NormalizedURL == b.NormalizedURL {
		return SimilarityResult{
			Score:     1,
			MatchType: MatchExact,
			Reasons:   []MatchReason{ReasonSameURL},
			Breakdown: ScoreBreakdown{URLScore: 1},
		}
	}

	breakdown := ScoreBreakdown{}
	reasons := make([]MatchReason, 0, 4)

	if a.Fingerprint != "" && a.Fingerprint == b.Fingerprint {
		breakdown.FingerprintScore = 1
		reasons = append(reasons, ReasonSameFingerprint)
	}

	if a.Brand != "" && a.Model != "" && a.Brand == b.Brand && a.Model == b.Model {
		breakdown.BrandModelScore = 1
		reasons = append(reasons, ReasonSameBrandModel)
	}

	breakdown.TitleScore = titleSimilarity(a.NormalizedTitle, b.NormalizedTitle)
	if breakdown.TitleScore >= 0.85 {
		reasons = append(reasons, ReasonHighTitleSimilarity)
	}

	breakdown.TokenScore = tokenJaccard(a.Tokens, b.Tokens)
	if breakdown.TokenScore >= 0.6 {
		reasons = append(reasons, ReasonTokenOverlap)
	}

	if a.Category != "" && a.Category == b.Category {
		reasons = append(reasons, ReasonSameCategory)
	}

	score := weightedScore(breakdown)
	matchType := classify(score, DefaultThresholds())

	if matchType == MatchNone {
		reasons = nil
	}

	return SimilarityResult{
		Score:     score,
		MatchType: matchType,
		Reasons:   reasons,
		Breakdown: breakdown,
	}
}

// CompareRaw normalizes both inputs and compares them.
func CompareRaw(a, b RawProductInput) SimilarityResult {
	return CompareProducts(NormalizeProduct(a), NormalizeProduct(b))
}

func classify(score float64, thresholds Thresholds) MatchType {
	switch {
	case score >= thresholds.Exact:
		return MatchExact
	case score >= thresholds.Probable:
		return MatchProbable
	case score >= thresholds.Related:
		return MatchRelated
	default:
		return MatchNone
	}
}

func weightedScore(b ScoreBreakdown) float64 {
	if b.URLScore >= 1 {
		return 1
	}

	weights := []struct {
		value  float64
		weight float64
	}{
		{b.FingerprintScore, 0.35},
		{b.BrandModelScore, 0.25},
		{b.TitleScore, 0.25},
		{b.TokenScore, 0.15},
	}

	total := 0.0
	weightSum := 0.0
	for _, item := range weights {
		if item.value <= 0 {
			continue
		}
		total += item.value * item.weight
		weightSum += item.weight
	}

	if weightSum == 0 {
		return 0
	}
	return math.Min(1, total/weightSum)
}

func titleSimilarity(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 1
	}

	aTokens := tokenize(a)
	bTokens := tokenize(b)
	if len(aTokens) == 0 || len(bTokens) == 0 {
		return 0
	}

	intersection := 0
	setA := make(map[string]struct{}, len(aTokens))
	for _, token := range aTokens {
		setA[token] = struct{}{}
	}
	for _, token := range bTokens {
		if _, ok := setA[token]; ok {
			intersection++
		}
	}

	union := len(aTokens) + len(bTokens) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func tokenJaccard(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	if len(a) == 1 && len(b) == 1 && a[0] == b[0] {
		return 1
	}

	setA := make(map[string]struct{}, len(a))
	for _, token := range a {
		setA[token] = struct{}{}
	}

	intersection := 0
	setB := make(map[string]struct{}, len(b))
	for _, token := range b {
		setB[token] = struct{}{}
		if _, ok := setA[token]; ok {
			intersection++
		}
	}

	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// HasURLMatch reports whether two URLs refer to the same product page.
func HasURLMatch(a, b string) bool {
	left := NormalizeURL(a)
	right := NormalizeURL(b)
	return left != "" && right != "" && left == right
}

// ContainsURL checks whether normalized URL exists in a list.
func ContainsURL(urls []string, target string) bool {
	normalizedTarget := NormalizeURL(target)
	if normalizedTarget == "" {
		return false
	}
	for _, item := range urls {
		if NormalizeURL(item) == normalizedTarget {
			return true
		}
	}
	return false
}

// MergeUniqueURLs appends URLs without duplicates using normalized comparison.
func MergeUniqueURLs(existing, incoming []string) []string {
	result := append([]string(nil), existing...)
	seen := make(map[string]struct{}, len(existing))
	for _, item := range existing {
		seen[NormalizeURL(item)] = struct{}{}
	}
	for _, item := range incoming {
		normalized := NormalizeURL(item)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, item)
	}
	return result
}

// PickPrimaryReason returns the strongest reason for UI display.
func PickPrimaryReason(reasons []MatchReason) MatchReason {
	if len(reasons) == 0 {
		return ""
	}
	for _, reason := range reasons {
		if reason == ReasonSameURL {
			return reason
		}
	}
	return reasons[0]
}

// MatchTypeFromScore maps score to match type using default thresholds.
func MatchTypeFromScore(score float64) MatchType {
	return classify(score, DefaultThresholds())
}

// IsStrongMatch returns true for exact or probable matches.
func IsStrongMatch(matchType MatchType) bool {
	return matchType == MatchExact || matchType == MatchProbable
}

// StringMatchReasons converts reasons to plain strings.
func StringMatchReasons(reasons []MatchReason) []string {
	out := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		out = append(out, string(reason))
	}
	return out
}

// NormalizeManyURLs normalizes a slice of URLs, skipping empty values.
func NormalizeManyURLs(urls []string) []string {
	out := make([]string, 0, len(urls))
	for _, item := range urls {
		if normalized := NormalizeURL(item); normalized != "" {
			out = append(out, normalized)
		}
	}
	return out
}

// LongestCommonPrefixRatio is a cheap fallback for near-identical titles.
func LongestCommonPrefixRatio(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	limit := min(len(a), len(b))
	prefix := 0
	for i := 0; i < limit; i++ {
		if strings.ToLower(string(a[i])) != strings.ToLower(string(b[i])) {
			break
		}
		prefix++
	}
	return float64(prefix) / float64(max(len(a), len(b)))
}
