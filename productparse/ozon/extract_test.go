package ozon

import (
	"context"
	"errors"
	"testing"

	"github.com/ibednov/go-lepsios/browser"
)

const sampleComposerJSON = `{
  "seo": {
    "title": "Apple iPhone 15 купить на OZON",
    "script": [{
      "type": "application/ld+json",
      "innerHTML": "{\"@type\":\"Product\",\"name\":\"Apple Смартфон iPhone 15\",\"image\":\"https://ir.ozone.ru/s3/multimedia-1-l/7669840197.jpg\",\"offers\":{\"price\":\"53558\",\"priceCurrency\":\"RUB\"}}"
    }]
  },
  "widgetStates": {
    "webProductHeading-1-default-1": "{\"title\":\"Apple Смартфон iPhone 15 SIM+eSIM 128 ГБ, Nano-SIM, синий\"}",
    "webPrice-1-default-1": "{\"cardPrice\":\"53\u2009022\u2009₽\",\"price\":\"53\u2009558\u2009₽\"}",
    "webGallery-1-default-1": "{\"coverImage\":\"https://ir.ozone.ru/s3/multimedia-1-l/7669840197.jpg\"}"
  }
}`

func TestValidateProductURL(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{
			"https://ozon.by/product/umnyy-kontroller-rele-smart-life-s-golosovym-upravleniem-alisa-16a-3255813434/",
			false,
		},
		{
			"https://www.ozon.ru/product/apple-smartfon-iphone-15-1185261285/",
			false,
		},
		{
			"https://ozon.by/t/BSawbtL",
			false,
		},
		{
			"https://ozon.by/category/smartphones/",
			true,
		},
	}

	for _, tt := range tests {
		err := validateProductURL(tt.url)
		if tt.wantErr && err == nil {
			t.Fatalf("expected error for %s", tt.url)
		}
		if !tt.wantErr && err != nil {
			t.Fatalf("unexpected error for %s: %v", tt.url, err)
		}
	}
}

func TestParseComposerPage(t *testing.T) {
	product, err := parseComposerPage([]byte(sampleComposerJSON))
	if err != nil {
		t.Fatalf("parseComposerPage failed: %v", err)
	}
	if product.Title != "Apple Смартфон iPhone 15 SIM+eSIM 128 ГБ, Nano-SIM, синий" {
		t.Fatalf("unexpected title: %q", product.Title)
	}
	if product.Price != 53022 {
		t.Fatalf("unexpected price: %v", product.Price)
	}
	if product.Currency != "RUB" {
		t.Fatalf("unexpected currency: %q", product.Currency)
	}
	if product.ImageURL == "" {
		t.Fatal("expected image url")
	}
}

func TestNormalizeComposerPath(t *testing.T) {
	if got := normalizeComposerPath("/t/BSawbtL"); got != "/t/BSawbtL/" {
		t.Fatalf("unexpected path: %q", got)
	}
}

func TestComposerAPIURL(t *testing.T) {
	got := composerAPIURL("https://ozon.by/", "/t/BSawbtL/")
	want := "https://ozon.by/api/composer-api.bx/page/json/v2?url=%2Ft%2FBSawbtL%2F"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestPriceFromLocalizedText(t *testing.T) {
	price, ok := priceFromLocalizedText("20,13 BYN")
	if !ok || price != 20.13 {
		t.Fatalf("got %v ok=%v", price, ok)
	}
}

type stubPages struct {
	snap browser.PageSnapshot
	err  error
}

func (s stubPages) FetchInPage(context.Context, string, string) (browser.PageSnapshot, error) {
	return s.snap, s.err
}

func TestFetchProductBrowserComposer(t *testing.T) {
	s := NewStrategy(stubPages{snap: browser.PageSnapshot{Body: []byte(sampleComposerJSON)}})
	product, err := s.fetchProductBrowser(context.Background(), "https://ozon.by/product/x-1/", "https://ozon.by/api/x")
	if err != nil {
		t.Fatal(err)
	}
	if product.Title == "" || product.Price != 53022 {
		t.Fatalf("%+v", product)
	}
}

func TestFetchProductBrowserUnavailable(t *testing.T) {
	s := NewStrategy(nil)
	if _, err := s.fetchProductBrowser(context.Background(), "https://ozon.by/product/x-1/", "https://ozon.by/api/x"); !errors.Is(err, errBrowserUnavailable) {
		t.Fatalf("got %v", err)
	}
}
