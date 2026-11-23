package parser

import (
	"testing"

	"github.com/ibednov/go-lepsios/currency"
)

func TestParsedLogFromKeepsNullPriceAndError(t *testing.T) {
	out := parsedLogFrom(RawParseResult{
		Title:      "Пиалы для чайной церемонии 60 мл",
		Currency:   currency.BYN,
		PriceError: "http: wb detail API returned HTTP 403; basket: empty sizes",
	})
	if out.Title == "" || out.Currency != "BYN" {
		t.Fatalf("log=%+v", out)
	}
	if out.Price != nil {
		t.Fatalf("price=%v", *out.Price)
	}
	if out.PriceError == "" {
		t.Fatal("expected price_error")
	}
}

func TestParsedLogFromKeepsPricePointer(t *testing.T) {
	price := 12.99
	out := parsedLogFrom(RawParseResult{Title: "X", Price: &price, Currency: currency.BYN})
	if out.Price == nil || *out.Price != 12.99 || out.PriceError != "" {
		t.Fatalf("log=%+v", out)
	}
}
