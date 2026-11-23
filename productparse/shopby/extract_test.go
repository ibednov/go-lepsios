package shopby

import "testing"

const sampleProductHTML = `<!DOCTYPE html>
<html lang="ru">
<head>
<meta property="og:title" content="Смарт-приставка Apple TV 4K 64GB (3-е поколение) купить недорого в Минске, цены – Shop.by">
<meta property="og:image" content="https://shop.by/images/apple_tv_4k_64gb_(3_e_pokolenie)_1.webp">
<script type="application/ld+json">{"@context":"https://schema.org","@graph":[{"@type":"Product","name":"Смарт-приставка Apple TV 4K 64GB (3-е поколение)","image":["https://shop.by/images/apple_tv_4k_64gb_(3_e_pokolenie)_1.webp"],"offers":{"@type":"AggregateOffer","lowPrice":590,"highPrice":1370,"priceCurrency":"BYN","offerCount":18}}]}</script>
</head>
<body data-page="model" data-title="Смарт-приставка Apple TV 4K 64GB (3-е поколение)" data-model-id="691619">
</body>
</html>`

const sampleLegacyMicrodataHTML = `<!DOCTYPE html>
<html lang="ru">
<head>
<meta property="og:title" content="Смарт-приставка Apple TV 4K 64GB (3-е поколение) купить недорого в Минске, цены – Shop.by">
<meta property="og:image" content="https://shop.by/images/apple_tv_4k_64gb_(3_e_pokolenie)_1.webp">
</head>
<body data-page="model" data-title="Смарт-приставка Apple TV 4K 64GB (3-е поколение)" data-model-id="691619">
<h1 class="Page__TitleActivePage" itemprop="name">Смарт-приставка Apple TV 4K 64GB (3-е поколение)</h1>
<div itemprop="offers" itemtype="http://schema.org/AggregateOffer">
<meta itemprop="priceCurrency" content="BYN"/>
<meta itemprop="lowPrice" content="590.00"/>
<meta itemprop="highPrice" content="1205.00"/>
</div>
</body>
</html>`

func TestValidateProductURL(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{
			"https://shop.by/multimedia_proigryvateli/apple_tv_4k_64gb_3_e_pokolenie/",
			false,
		},
		{
			"https://shop.by/multimedia_proigryvateli/",
			true,
		},
		{
			"https://shop.by/news/sovetyi/example/",
			true,
		},
		{
			"https://shop.by/cart/",
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

func TestParseProductFromHTML(t *testing.T) {
	assertParsedProduct(t, sampleProductHTML)
}

func TestParseProductFromLegacyMicrodataHTML(t *testing.T) {
	assertParsedProduct(t, sampleLegacyMicrodataHTML)
}

func assertParsedProduct(t *testing.T, htmlBody string) {
	t.Helper()
	product, ok := parseProductFromHTML(htmlBody)
	if !ok {
		t.Fatal("expected product to parse")
	}
	if product.Title != "Смарт-приставка Apple TV 4K 64GB (3-е поколение)" {
		t.Fatalf("unexpected title: %q", product.Title)
	}
	if product.Price != 590 {
		t.Fatalf("unexpected price: %v", product.Price)
	}
	if product.Currency != "BYN" {
		t.Fatalf("unexpected currency: %q", product.Currency)
	}
	if product.ImageURL != "https://shop.by/images/apple_tv_4k_64gb_(3_e_pokolenie)_1.webp" {
		t.Fatalf("unexpected image: %q", product.ImageURL)
	}
}

func TestNormalizeOGTitle(t *testing.T) {
	got := normalizeOGTitle("Apple TV 4K купить недорого в Минске, цены – Shop.by")
	want := "Apple TV 4K"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
