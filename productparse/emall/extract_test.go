package emall

import "testing"

const sampleProductHTML = `<!DOCTYPE html>
<html lang="ru">
<head>
<meta property="og:title" content="Протеин сывороточный Молочный мир Whey Protein КСБ 80%, без вкуса, 1 кг">
<meta property="og:image" content="https://cdn.ime.by/UserFiles/images/catalog/Goods/4386/02194386/norm/02194386.n_1.png?s=1000x1333">
<script type="application/ld+json">{
  "@context": "https://schema.org",
  "@type": "Product",
  "@id": "https://emall.by/product/2194386",
  "name": "Протеин сывороточный Молочный мир Whey Protein КСБ 80%, без вкуса, 1 кг",
  "sku": "2194386",
  "image": [{
    "@type": "ImageObject",
    "url": "https://cdn.ime.by/UserFiles/images/catalog/Goods/4386/02194386/norm/02194386.n_1.png",
    "thumbnailUrl": "https://cdn.ime.by/UserFiles/images/catalog/Goods/4386/02194386/norm/02194386.n_1.png?s=1000x1333"
  }],
  "offers": {
    "@type": "Offer",
    "priceCurrency": "BYN",
    "price": "59.99"
  }
}</script>
</head>
<body></body>
</html>`

func TestValidateProductURL(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{"https://emall.by/product/2194386", false},
		{"https://emall.by/product/2194386/", false},
		{"https://emall.by/category/5954", true},
		{"https://emall.by/product/abc", true},
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

func TestParseProductFromHTML(t *testing.T) {
	product, ok := parseProductFromHTML(sampleProductHTML, "2194386")
	if !ok {
		t.Fatal("expected product to parse")
	}
	if product.Title != "Протеин сывороточный Молочный мир Whey Protein КСБ 80%, без вкуса, 1 кг" {
		t.Fatalf("unexpected title: %q", product.Title)
	}
	if product.Price != 59.99 {
		t.Fatalf("unexpected price: %v", product.Price)
	}
	if product.Currency != "BYN" {
		t.Fatalf("unexpected currency: %q", product.Currency)
	}
	if product.ImageURL == "" {
		t.Fatal("expected image url")
	}
}

func TestProductIDFromURL(t *testing.T) {
	id, err := productIDFromURL("https://emall.by/product/2194386")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "2194386" {
		t.Fatalf("unexpected id: %q", id)
	}
}
