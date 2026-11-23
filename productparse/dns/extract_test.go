package dns

import (
	"strings"
	"testing"
)

const sampleJSONLDBlock = `{"@context":"https://schema.org","@graph":[{"@type":"BreadcrumbList"},{"@context":"https://schema.org","@type":"Product","name":"Клавиатура 8BitDo","sku":"5432739","offers":{"@type":"Offer","price":263.9,"priceCurrency":"BYN"},"image":["https://c.dns-shop.ru/thumb/st4/fit/45/45/abc/q85_test.jpg"],"brand":{"@type":"Brand","name":"8BitDo"}}]}`

const sampleHTMLFragment = `
<link rel="preload" href="https://c.dns-shop.ru/thumb/st4/fit/500/500/d8cf/preload.jpg" as="image">
<h1>Клавиатура  проводная 8BitDo</h1>
<div class="product-card-purchase__current-price ">
                        263.90
                    </div>
`

func TestValidateProductURL(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{
			"https://dns-shop.by/ru/product/0d7f3e4e5691ed20/klaviatura-test/",
			false,
		},
		{
			"https://dns-shop.by/ru/product/0d7f3e4e5691ed20/klaviatura-test/opinions/",
			true,
		},
		{
			"https://dns-shop.by/ru/catalog/",
			true,
		},
	}

	for _, tt := range tests {
		err := validateProductURL(tt.url)
		if (err != nil) != tt.wantErr {
			t.Fatalf("validateProductURL(%q) err=%v wantErr=%v", tt.url, err, tt.wantErr)
		}
	}
}

func TestExtractFromJSONLD(t *testing.T) {
	product, ok := extractFromJSONLD(
		`<script type="application/ld+json">` + sampleJSONLDBlock + `</script>`,
	)
	if !ok {
		t.Fatal("expected product from json-ld")
	}
	if product.Title != "Клавиатура 8BitDo" {
		t.Fatalf("title=%q", product.Title)
	}
	if product.Price != 263.9 {
		t.Fatalf("price=%v", product.Price)
	}
	if product.ImageURL == "" {
		t.Fatal("expected image url")
	}
}

func TestExtractFromHTML(t *testing.T) {
	product, ok := extractFromHTML(sampleHTMLFragment)
	if !ok {
		t.Fatal("expected product from html")
	}
	if product.Title == "" || product.Price != 263.9 {
		t.Fatalf("title=%q price=%v", product.Title, product.Price)
	}
}

func TestExtractBestImageURL(t *testing.T) {
	imageURL := extractBestImageURL(sampleHTMLFragment)
	if imageURL == "" {
		t.Fatal("expected preload image")
	}
	if !strings.Contains(imageURL, "/fit/500/500/") {
		t.Fatalf("image=%q", imageURL)
	}
}

func TestUpscaleThumbURL(t *testing.T) {
	src := "https://c.dns-shop.ru/thumb/st4/fit/45/45/hash/q85_test.jpg"
	got := upscaleThumbURL(src)
	if got == src {
		t.Fatalf("expected upscaled url, got %q", got)
	}
	if !strings.Contains(got, "/fit/500/500/") {
		t.Fatalf("got %q", got)
	}
}
