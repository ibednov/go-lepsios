package tildastore

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
)

var storepartUID = regexp.MustCompile(`^[0-9]{6,18}$`)
var tildaWeightPattern = regexp.MustCompile(`(?i)(?:^|[^\pL\d])(\d+(?:[.,]\d+)?)\s*(кг|kg|грамм(?:ов|а)?|гр\.?|г|g)(?:$|[^\pL])`)

type storeList struct {
	Total     int            `json:"total"`
	NextSlice int            `json:"nextslice"`
	Products  []storeProduct `json:"products"`
}

type storeProduct struct {
	UID      flexibleString  `json:"uid"`
	Title    string          `json:"title"`
	SKU      string          `json:"sku"`
	URL      string          `json:"url"`
	Price    flexibleString  `json:"price"`
	Quantity flexibleString  `json:"quantity"`
	Gallery  json.RawMessage `json:"gallery"`
	PackM    flexibleNumber  `json:"pack_m"`
	Editions []storeEdition  `json:"editions"`
}

type storeEdition struct {
	UID      flexibleString `json:"uid"`
	SKU      string         `json:"sku"`
	Price    string         `json:"price"`
	Quantity flexibleString `json:"quantity"`
	Image    string         `json:"img"`
	PackM    flexibleNumber `json:"pack_m"`
	raw      map[string]any
}

func (e *storeEdition) UnmarshalJSON(data []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	e.raw = raw
	e.UID = flexibleString(asString(raw["uid"]))
	e.SKU = asString(raw["sku"])
	e.Price = asString(raw["price"])
	e.Quantity = flexibleString(asString(raw["quantity"]))
	e.Image = asString(raw["img"])
	if n, ok := asNumber(raw["pack_m"]); ok {
		e.PackM = flexibleNumber(n)
	}
	return nil
}

func parseProductList(body []byte) (storeList, error) {
	var page storeList
	if err := json.Unmarshal(body, &page); err != nil {
		return storeList{}, fmt.Errorf("tilda store json: %w", err)
	}
	for i := range page.Products {
		page.Products[i].UID = flexibleString(strings.TrimSpace(string(page.Products[i].UID)))
	}
	return page, nil
}

func (p storeProduct) imageURL() string {
	if len(p.Gallery) == 0 {
		return ""
	}
	var items []struct {
		Img string `json:"img"`
	}
	raw := strings.TrimSpace(string(p.Gallery))
	if strings.HasPrefix(raw, "\"") {
		var encoded string
		if json.Unmarshal(p.Gallery, &encoded) == nil {
			raw = encoded
		}
	}
	if json.Unmarshal([]byte(raw), &items) == nil && len(items) > 0 {
		return items[0].Img
	}
	return ""
}

func parseTildaPrice(raw string) (decimal.Decimal, error) {
	normalized := strings.TrimSpace(raw)
	normalized = strings.ReplaceAll(normalized, "\u00a0", "")
	normalized = strings.ReplaceAll(normalized, " ", "")
	normalized = strings.ReplaceAll(normalized, ",", ".")
	if normalized == "" {
		return decimal.Zero, fmt.Errorf("empty price")
	}
	return decimal.NewFromString(normalized)
}

func editionWeightGrams(edition storeEdition) *int {
	for _, key := range []string{"Вес", "вес", "Weight", "weight"} {
		if weight := weightFromLabel(asString(edition.field(key))); weight != nil {
			return weight
		}
	}
	if edition.PackM > 0 {
		grams := int(edition.PackM)
		if grams > 0 && grams <= 10000 {
			return &grams
		}
	}
	return nil
}

func editionWeightLabel(edition storeEdition) string {
	for _, key := range []string{"Вес", "вес", "Weight", "weight"} {
		if label := strings.TrimSpace(asString(edition.field(key))); label != "" {
			return label
		}
	}
	return ""
}

func (e storeEdition) field(key string) any {
	if e.raw == nil {
		return nil
	}
	if value, ok := e.raw[key]; ok {
		return value
	}
	for existing, value := range e.raw {
		if strings.EqualFold(existing, key) {
			return value
		}
	}
	return nil
}

func editionInStock(edition storeEdition, product storeProduct) bool {
	qty := strings.TrimSpace(string(edition.Quantity))
	if qty == "" {
		qty = strings.TrimSpace(string(product.Quantity))
	}
	if qty == "" {
		return true
	}
	n, err := strconv.Atoi(qty)
	if err != nil {
		return qty != "0"
	}
	return n > 0
}

func preferEdition(candidate, current storeEdition) bool {
	return grindRank(candidate) < grindRank(current)
}

func grindRank(edition storeEdition) int {
	grind := strings.ToLower(asString(edition.field("Помол")) + asString(edition.field("помол")))
	switch {
	case grind == "" || strings.Contains(grind, "не молоть"):
		return 0
	default:
		return 1
	}
}

func weightFromLabel(raw string) *int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	match := tildaWeightPattern.FindStringSubmatch(raw)
	if match == nil {
		n, err := strconv.Atoi(strings.TrimSpace(raw))
		if err == nil && n > 0 && n <= 10000 {
			return &n
		}
		return nil
	}
	amount, err := decimal.NewFromString(strings.ReplaceAll(match[1], ",", "."))
	if err != nil || !amount.IsPositive() {
		return nil
	}
	unit := strings.ToLower(match[2])
	if strings.Contains(unit, "кг") || unit == "kg" {
		amount = amount.Mul(decimal.NewFromInt(1000))
	}
	grams := int(amount.IntPart())
	if grams <= 0 || grams > 10000 {
		return nil
	}
	return &grams
}

type flexibleString string

func (s *flexibleString) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*s = flexibleString(text)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		*s = flexibleString(n.String())
		return nil
	}
	*s = ""
	return nil
}

type flexibleNumber float64

func (n *flexibleNumber) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == `""` {
		*n = 0
		return nil
	}
	var number float64
	if err := json.Unmarshal(data, &number); err == nil {
		*n = flexibleNumber(number)
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		*n = 0
		return nil
	}
	parsed, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(text), ",", "."), 64)
	if err != nil {
		*n = 0
		return nil
	}
	*n = flexibleNumber(parsed)
	return nil
}

func asString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case json.Number:
		return v.String()
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func asNumber(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case json.Number:
		n, err := v.Float64()
		return n, err == nil
	case string:
		n, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return n, err == nil
	default:
		return 0, false
	}
}
