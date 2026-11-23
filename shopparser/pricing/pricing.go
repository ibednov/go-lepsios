package pricing

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/shopspring/decimal"
)

// Labeled grams: "25-gr", "50 гр", "Блин 357 гр", "100g". Longest units first so
// "gr" is not truncated to "g".
var reLabeledGrams = regexp.MustCompile(`(?i)(\d+[.,]?\d*)\s*[-_]?\s*(килограмм(?:ов|а)?|кг|kg|грамм(?:ов|а)?|гр\.?|gr|г\.?|g)(?:\s|$|[^\p{L}\p{N}])`)
var reBareGrams = regexp.MustCompile(`(?i)^\s*(\d+[.,]?\d*)\s*$`)

// DefaultRUBToBYN is BYN per 1 RUB when NBRB rates are unavailable.
// NBRB publishes ~3.7 BYN per 100 RUB; 0.037 is the matching unit rate.
var DefaultRUBToBYN = decimal.RequireFromString("0.037")

// PriceFromMinorUnits converts Store API minor-unit strings to major units.
// BYN minor_unit=2: "8600" → 86.00. BYR minor_unit=1 (garbata): "380" → 38.0 as BYN.
func PriceFromMinorUnits(raw string, minorUnit int) (decimal.Decimal, error) {
	if raw == "" {
		return decimal.Zero, fmt.Errorf("empty price")
	}
	n, err := decimal.NewFromString(raw)
	if err != nil {
		return decimal.Zero, err
	}
	if minorUnit < 0 {
		return decimal.Zero, fmt.Errorf("negative minor unit")
	}
	div := decimal.NewFromInt(1)
	for i := 0; i < minorUnit; i++ {
		div = div.Mul(decimal.NewFromInt(10))
	}
	return n.Div(div), nil
}

// NormalizeCurrency maps shop currency codes to a supported catalog source currency.
// garbata sends BYR with minor_unit=1 — treat as BYN after PriceFromMinorUnits.
// RUB/USD/EUR/CNY stay as-is so crawls can store the original amount and convert to BYN.
func NormalizeCurrency(code string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "BYN", "BYR":
		return "BYN", nil
	case "RUB", "USD", "EUR", "CNY":
		return strings.ToUpper(strings.TrimSpace(code)), nil
	default:
		return "", fmt.Errorf("unexpected currency %q", code)
	}
}

// FX converts a source-currency amount into BYN for catalog comparison.
type FX interface {
	ToBYN(amount decimal.Decimal, from string) (decimal.Decimal, error)
}

// StaticFX uses a configured RUB→BYN unit rate. Other non-BYN codes fail so
// callers fall back to a live NBRB snapshot when available.
type StaticFX struct {
	RUBToBYN decimal.Decimal
}

func (s StaticFX) ToBYN(amount decimal.Decimal, from string) (decimal.Decimal, error) {
	code, err := NormalizeCurrency(from)
	if err != nil {
		return decimal.Zero, err
	}
	if code == "BYN" {
		return amount, nil
	}
	if code != "RUB" {
		return decimal.Zero, fmt.Errorf("no fallback rate for %s", code)
	}
	rate := s.RUBToBYN
	if !rate.IsPositive() {
		rate = DefaultRUBToBYN
	}
	return amount.Mul(rate).Round(4), nil
}

// BYNPerUnit is NBRB's official formula: BasePerUnit BYN per Scale foreign units.
func BYNPerUnit(scale int, basePerUnit float64) decimal.Decimal {
	if scale <= 0 || basePerUnit <= 0 {
		return decimal.Zero
	}
	return decimal.NewFromFloat(basePerUnit).Div(decimal.NewFromInt(int64(scale)))
}

// ConvertOfficialToBYN applies an NBRB row (scale + BYN per scale units).
func ConvertOfficialToBYN(amount decimal.Decimal, scale int, basePerUnit float64) (decimal.Decimal, error) {
	rate := BYNPerUnit(scale, basePerUnit)
	if !rate.IsPositive() {
		return decimal.Zero, fmt.Errorf("invalid official rate scale=%d base_per_unit=%v", scale, basePerUnit)
	}
	return amount.Mul(rate).Round(4), nil
}

// FallbackFX uses Primary when it can convert, otherwise Fallback (typically a static RUB rate).
type FallbackFX struct {
	Primary  FX
	Fallback FX
}

func (f FallbackFX) ToBYN(amount decimal.Decimal, from string) (decimal.Decimal, error) {
	if f.Primary != nil {
		got, err := f.Primary.ToBYN(amount, from)
		if err == nil && got.IsPositive() {
			return got, nil
		}
	}
	if f.Fallback != nil {
		return f.Fallback.ToBYN(amount, from)
	}
	return decimal.Zero, fmt.Errorf("no FX converter configured")
}

// PricePer100g returns price / weight * 100. Nil weight → nil.
func PricePer100g(price decimal.Decimal, weightG *int) *decimal.Decimal {
	if weightG == nil || *weightG <= 0 || !price.IsPositive() {
		return nil
	}
	w := decimal.NewFromInt(int64(*weightG))
	return PricePer100gDecimal(price, &w)
}

// PricePer100gDecimal returns the normalized price using exact grams. Nil or
// non-positive weight and price produce no comparable price.
func PricePer100gDecimal(price decimal.Decimal, weightG *decimal.Decimal) *decimal.Decimal {
	if weightG == nil || !weightG.IsPositive() || !price.IsPositive() {
		return nil
	}
	v := price.Div(*weightG).Mul(decimal.NewFromInt(100)).Round(2)
	return &v
}

const (
	FlagOutlier      = "outlier"
	FlagImplausible  = "implausible"
	implausibleRatio = "0.3"
)

// PriceFlag marks extreme BYN/100 g prices for review. Type-specific exceptions
// will be configured separately; this conservative limit protects new offers.
func PriceFlag(per100g *decimal.Decimal) string {
	if per100g != nil && per100g.GreaterThan(decimal.NewFromInt(2000)) {
		return FlagOutlier
	}
	return ""
}

// ParseGrams reads a variant mass from shop strings. WooCommerce slugs such as
// "25-gr" and labels "50 гр" / "Блин 357 гр" must not fall through to a cake
// weight taken from the product title.
func ParseGrams(s string) *decimal.Decimal {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	for _, match := range reLabeledGrams.FindAllStringSubmatch(s, -1) {
		num, err := decimal.NewFromString(strings.ReplaceAll(match[1], ",", "."))
		if err != nil || !num.IsPositive() {
			continue
		}
		unit := strings.ToLower(match[2])
		if isGramUnit(unit) && isYearNumber(match[1]) {
			continue
		}
		grams := num
		if isKilogramUnit(unit) {
			grams = num.Mul(decimal.NewFromInt(1000))
		}
		if !grams.IsPositive() || grams.GreaterThan(decimal.NewFromInt(10000)) {
			continue
		}
		copy := grams
		return &copy
	}
	if match := reBareGrams.FindStringSubmatch(s); match != nil {
		num, err := decimal.NewFromString(strings.ReplaceAll(match[1], ",", "."))
		if err != nil || !num.IsPositive() || !num.Equal(num.Truncate(0)) || num.GreaterThan(decimal.NewFromInt(10000)) {
			return nil
		}
		copy := num
		return &copy
	}
	return nil
}

// ParseGramsInt is the legacy whole-gram view of ParseGrams.
func ParseGramsInt(s string) *int {
	grams := ParseGrams(s)
	if grams == nil {
		return nil
	}
	n := int(grams.Round(0).IntPart())
	if n <= 0 {
		return nil
	}
	return &n
}

func isGramUnit(unit string) bool {
	switch unit {
	case "г", "г.", "g", "гр", "гр.", "gr", "грамм", "грамма", "граммов":
		return true
	}
	return strings.HasPrefix(unit, "грамм")
}

func isKilogramUnit(unit string) bool {
	return unit == "kg" || unit == "кг" || strings.HasPrefix(unit, "килограмм")
}

func isYearNumber(raw string) bool {
	if len(raw) != 4 {
		return false
	}
	return strings.HasPrefix(raw, "19") || strings.HasPrefix(raw, "20")
}

// ImplausibleVsMedian reports whether per-100g is wildly below the canonical
// median (under 30%). Such offers must not win "best price".
func ImplausibleVsMedian(per100g, median *decimal.Decimal) bool {
	if per100g == nil || median == nil || !median.IsPositive() || !per100g.IsPositive() {
		return false
	}
	return per100g.LessThan(median.Mul(decimal.RequireFromString(implausibleRatio)))
}
