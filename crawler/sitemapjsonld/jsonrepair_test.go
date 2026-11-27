package sitemapjsonld

import (
	"encoding/json"
	"testing"
)

func TestRepairUnescapedQuotes(t *testing.T) {
	raw := []byte("{\"@type\": \"Product\",\n \"name\": \"Apple iPad 11\" 2025 256GB (желтый)\",\n \"description\": \"11.0\\\" IPS\nline\", \"offers\": {\"price\": 1}}")
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err == nil {
		t.Fatal("fixture must be invalid JSON")
	}
	if err := json.Unmarshal(repairUnescapedQuotes(raw), &v); err != nil {
		t.Fatalf("repair failed: %v", err)
	}
	if v["name"] != "Apple iPad 11\" 2025 256GB (желтый)" {
		t.Fatalf("name = %q", v["name"])
	}
	valid := []byte(`{"a":"b \"c\"","d":["e"]}`)
	if string(repairUnescapedQuotes(valid)) != string(valid) {
		t.Fatal("valid JSON must be unchanged")
	}
}

func TestDecodeProductsRepairsInchMarks(t *testing.T) {
	body := []byte(`<html><script type="application/ld+json">{"@context":"https://schema.org","@type":"Product","name":"Apple MacBook Air 13.6" M5 2026","brand":"Apple","offers":{"@type":"Offer","priceCurrency":"BYN","price":3940.0,"availability":"https://schema.org/InStock"}}</script></html>`)
	products, err := decodeProducts(body)
	if err != nil || len(products) != 1 {
		t.Fatalf("decode: %v %d", err, len(products))
	}
	if products[0].Name != `Apple MacBook Air 13.6" M5 2026` {
		t.Fatalf("name = %q", products[0].Name)
	}
}

func TestPageSpecRows(t *testing.T) {
	body := []byte(`<div id="details"><table><tr><td colspan="2" class="product-section">Основные</td></tr>
<tr class="product-item"><td>Диагональ экрана</td><td>11.0"</td></tr>
<tr class="product-item"><td>Объем встроенной памяти</td><td> 256
 ГБ</td></tr></table></div>`)
	specs := pageSpecRows(body, "tr.product-item")
	if len(specs) != 2 || specs["spec:Объем встроенной памяти"] != "256 ГБ" || specs["spec:Диагональ экрана"] != `11.0"` {
		t.Fatalf("specs = %#v", specs)
	}
}
