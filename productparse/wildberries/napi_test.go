package wildberries

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/ibednov/go-lepsios/currency"
)

func TestParseNapiOfferHTMLFromSavedPage(t *testing.T) {
	body, err := os.ReadFile("testdata/napi_409842715.html")
	if err != nil {
		t.Fatal(err)
	}
	title, brand, price, curr, err := parseNapiOfferHTML(body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(title, "Пиалы") || brand != "ЛИНМУ" || price != 580 || curr != currency.RUB {
		t.Fatalf("title=%q brand=%q price=%v curr=%s", title, brand, price, curr)
	}
}

func TestParseNapiOfferHTMLTakesMixedGraph(t *testing.T) {
	html := []byte(`<script type="application/ld+json">
[
{"name":"Пиалы для чайной церемонии 60 мл, ЛИНМУ","brand":{"name":"ЛИНМУ"},"offers":{"price":580,"priceCurrency":"RUB","availability":"https://schema.org/InStock"}},
[{"@type":"BreadcrumbList","itemListElement":[]}]
]
</script>`)
	title, brand, price, curr, err := parseNapiOfferHTML(html)
	if err != nil {
		t.Fatal(err)
	}
	if title == "" || brand != "ЛИНМУ" || price != 580 || curr != currency.RUB {
		t.Fatalf("title=%q brand=%q price=%v curr=%s", title, brand, price, curr)
	}
}

func TestParseNapiOfferHTMLTakesInStockPrice(t *testing.T) {
	html := []byte(`<script type="application/ld+json">
[{"name":"Пиалы для чайной церемонии 60 мл, ЛИНМУ","brand":{"name":"ЛИНМУ"},"offers":{"price":580,"priceCurrency":"RUB","availability":"https://schema.org/InStock"}}]
</script>`)
	title, brand, price, curr, err := parseNapiOfferHTML(html)
	if err != nil {
		t.Fatal(err)
	}
	if title == "" || brand != "ЛИНМУ" || price != 580 || curr != currency.RUB {
		t.Fatalf("title=%q brand=%q price=%v curr=%s", title, brand, price, curr)
	}
}

func TestParseNapiOfferHTMLRejectsZero(t *testing.T) {
	html := []byte(`<script type="application/ld+json">
{"name":"X","offers":{"price":0,"priceCurrency":"RUB","availability":"https://schema.org/OutOfStock"}}
</script>`)
	_, _, _, _, err := parseNapiOfferHTML(html)
	if err == nil || !strings.Contains(err.Error(), "zero price") {
		t.Fatalf("err=%v", err)
	}
}

func TestCardFromSourcesSkipsForeignCurrencyNapiPrice(t *testing.T) {
	detail, err := pricedDetail("Пиалы", "ЛИНМУ", 580, currency.RUB, currency.BYN)
	if err != nil {
		t.Fatal(err)
	}
	title, price, priceErr, metaErr := cardFromSources(
		"https://www.wildberries.by/catalog/409842715/detail.aspx",
		[]detailSource{
			{name: "http", err: fmt.Errorf("wb detail API returned HTTP 403")},
			{name: "napi", data: detail},
		},
	)
	if metaErr != nil {
		t.Fatal(metaErr)
	}
	if title != "Пиалы - Бренд ЛИНМУ" {
		t.Fatalf("title=%q", title)
	}
	if price != nil || priceErr == nil {
		t.Fatalf("price=%v err=%v, want no price for RUB on .by", price, priceErr)
	}
}

func TestPricedDetailKeepsSameCurrencyPrice(t *testing.T) {
	detail, err := pricedDetail("Пиалы", "ЛИНМУ", 580, currency.RUB, currency.RUB)
	if err != nil {
		t.Fatal(err)
	}
	_, price, _, err := extractProductInfo(detail, "https://www.wildberries.ru/catalog/409842715/detail.aspx")
	if err != nil {
		t.Fatal(err)
	}
	if price != 580 {
		t.Fatalf("price=%v, want 580", price)
	}
}

func TestHistoryPricePrefersMarketCurrency(t *testing.T) {
	raw, curr, ok := historyPrice(map[string]int{"RUB": 53680, "BYN": 2059}, currency.BYN)
	if !ok || raw != 2059 || curr != currency.BYN {
		t.Fatalf("raw=%d curr=%s ok=%v", raw, curr, ok)
	}
}
