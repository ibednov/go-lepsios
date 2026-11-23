package storeapi

import (
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/ibednov/go-lepsios/crawler/pricing"
	"github.com/shopspring/decimal"
)

type storeProduct struct {
	ID         int             `json:"id"`
	Name       string          `json:"name"`
	Slug       string          `json:"slug"`
	Permalink  string          `json:"permalink"`
	Type       string          `json:"type"`
	SKU        string          `json:"sku"`
	Prices     storePrices     `json:"prices"`
	Categories []storeCategory `json:"categories"`
	Images     []storeImage    `json:"images"`
	IsInStock  bool            `json:"is_in_stock"`
	Variations json.RawMessage `json:"variations"`
	Attributes []storeAttr     `json:"attributes"`
}

type storePrices struct {
	Price             string `json:"price"`
	RegularPrice      string `json:"regular_price"`
	CurrencyCode      string `json:"currency_code"`
	CurrencyMinorUnit int    `json:"currency_minor_unit"`
}

type storeCategory struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type storeImage struct {
	Src string `json:"src"`
}

type storeAttr struct {
	Name     string `json:"name"`
	Taxonomy string `json:"taxonomy"`
	Terms    []struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"terms"`
}

type wooVariation struct {
	Attributes          map[string]string `json:"attributes"`
	DisplayPrice        float64           `json:"display_price"`
	DisplayRegularPrice float64           `json:"display_regular_price"`
	IsInStock           bool              `json:"is_in_stock"`
	SKU                 string            `json:"sku"`
	VariationID         int               `json:"variation_id"`
	Weight              string            `json:"weight"`
	Image               *struct {
		URL string `json:"url"`
	} `json:"image"`
}

var (
	reVariationsAttr = regexp.MustCompile(`(?i)data-product_variations=(?:"([^"]*)"|'([^']*)')`)
)

const absentVes = "vremenno-otsutvuet"

func parseStoreProducts(body []byte) ([]storeProduct, error) {
	var products []storeProduct
	if err := json.Unmarshal(body, &products); err != nil {
		return nil, fmt.Errorf("store products json: %w", err)
	}
	return products, nil
}

func parseStoreProduct(body []byte) (storeProduct, error) {
	var p storeProduct
	if err := json.Unmarshal(body, &p); err != nil {
		return storeProduct{}, fmt.Errorf("store product json: %w", err)
	}
	if p.ID == 0 && p.Permalink == "" {
		return storeProduct{}, fmt.Errorf("empty store product")
	}
	return p, nil
}

func isVariable(p storeProduct) bool {
	if strings.EqualFold(p.Type, "variable") {
		return true
	}
	if len(p.Variations) == 0 || string(p.Variations) == "null" {
		return false
	}
	// non-empty array → variable
	var arr []json.RawMessage
	if err := json.Unmarshal(p.Variations, &arr); err == nil {
		return len(arr) > 0
	}
	return false
}

func productAllowed(p storeProduct, f CategoryFilter) bool {
	hasInclude := len(f.NameContains) > 0 || len(f.IDs) > 0
	if len(p.Categories) > 0 {
		if categoryExcluded(p.Categories, f.ExcludeContains) {
			return false
		}
		if !hasInclude {
			return true
		}
		return categoryIncluded(p.Categories, f)
	}
	name := strings.ToLower(p.Name)
	for _, ex := range f.ExcludeContains {
		if ex != "" && strings.Contains(name, strings.ToLower(ex)) {
			return false
		}
	}
	if !hasInclude {
		return true
	}
	for _, inc := range f.NameContains {
		if inc != "" && strings.Contains(name, strings.ToLower(inc)) {
			return true
		}
	}
	return false
}

func categoryExcluded(cats []storeCategory, excludes []string) bool {
	for _, c := range cats {
		n := strings.ToLower(c.Name)
		s := strings.ToLower(c.Slug)
		for _, ex := range excludes {
			ex = strings.ToLower(ex)
			if ex == "" {
				continue
			}
			if strings.Contains(n, ex) || strings.Contains(s, ex) {
				return true
			}
		}
	}
	return false
}

func categoryIncluded(cats []storeCategory, f CategoryFilter) bool {
	idSet := map[int]struct{}{}
	for _, id := range f.IDs {
		idSet[id] = struct{}{}
	}
	for _, c := range cats {
		if _, ok := idSet[c.ID]; ok {
			return true
		}
		n := strings.ToLower(c.Name)
		s := strings.ToLower(c.Slug)
		for _, inc := range f.NameContains {
			inc = strings.ToLower(inc)
			if inc == "" {
				continue
			}
			if strings.Contains(n, inc) || strings.Contains(s, inc) {
				return true
			}
		}
	}
	return false
}

func parseProductVariationsHTML(body []byte) ([]wooVariation, error) {
	m := reVariationsAttr.FindSubmatch(body)
	if m == nil {
		return nil, fmt.Errorf("data-product_variations not found")
	}
	raw := string(m[1])
	if raw == "" {
		raw = string(m[2])
	}
	raw = html.UnescapeString(raw)
	if raw == "" || raw == "false" {
		return nil, fmt.Errorf("empty data-product_variations")
	}
	var vars []wooVariation
	if err := json.Unmarshal([]byte(raw), &vars); err != nil {
		return nil, fmt.Errorf("variations json: %w", err)
	}
	return vars, nil
}

func weightFromVariation(v wooVariation) (weight *int, absent bool) {
	for _, val := range v.Attributes {
		if strings.EqualFold(strings.TrimSpace(val), absentVes) {
			return nil, true
		}
	}
	if w := pricing.ParseGramsInt(v.Weight); w != nil {
		return w, false
	}
	// attribute_pa_ves only — never attribute_pa_cena-za-kg / «Цена за кг»
	for k, val := range v.Attributes {
		lk := strings.ToLower(k)
		if strings.Contains(lk, "cena") || strings.Contains(lk, "price") || strings.Contains(lk, "kg") {
			continue
		}
		if strings.Contains(lk, "ves") || strings.Contains(lk, "weight") || strings.HasSuffix(lk, "_ves") {
			if strings.EqualFold(strings.TrimSpace(val), absentVes) {
				return nil, true
			}
			if w := pricing.ParseGramsInt(val); w != nil {
				return w, false
			}
		}
	}
	return nil, false
}

func weightFromStoreProduct(p storeProduct) *int {
	for _, a := range p.Attributes {
		tax := strings.ToLower(a.Taxonomy + " " + a.Name)
		if strings.Contains(tax, "cena") || strings.Contains(tax, "кг") || strings.Contains(tax, "kg") {
			continue
		}
		if !(strings.Contains(tax, "ves") || strings.Contains(tax, "вес") || strings.Contains(tax, "weight")) {
			continue
		}
		for _, t := range a.Terms {
			if w := pricing.ParseGramsInt(t.Name); w != nil {
				return w
			}
			if w := pricing.ParseGramsInt(t.Slug); w != nil {
				return w
			}
		}
	}
	return nil
}

func variationAttrs(v wooVariation) map[string]string {
	out := map[string]string{}
	for k, val := range v.Attributes {
		lk := strings.ToLower(k)
		if strings.Contains(lk, "cena") || strings.Contains(lk, "kg") {
			continue // do not propagate «Цена за кг»
		}
		out[k] = val
	}
	if v.VariationID != 0 {
		out["variation_id"] = strconv.Itoa(v.VariationID)
	}
	return out
}

func decimalFromFloat(f float64) decimal.Decimal {
	return decimal.NewFromFloat(f).Round(2)
}
