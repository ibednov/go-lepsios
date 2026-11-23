package recipe

import "testing"

const emallHTML = `<!DOCTYPE html>
<head>
<meta property="og:title" content="Протеин сывороточный">
<meta property="og:image" content="https://cdn.ime.by/img.png">
<script type="application/ld+json">{
  "@type": "Product",
  "name": "Протеин сывороточный Молочный мир",
  "image": [{"@type":"ImageObject","url":"https://cdn.ime.by/product.png"}],
  "offers": {"@type":"Offer","priceCurrency":"BYN","price":"59.99"}
}</script>
</head>`

const shopbyHTML = `<head>
<meta property="og:title" content="Apple TV 4K купить">
<script type="application/ld+json">{"@graph":[{"@type":"Product","name":"Смарт-приставка Apple TV 4K","image":["https://shop.by/images/tv.webp"],"offers":{"@type":"AggregateOffer","lowPrice":590,"priceCurrency":"BYN"}}]}</script>
</head>`

const ozbyHTML = `<!DOCTYPE html>
<html><head>
<meta property="og:title" content="Кружка керамическая - на OZ.by">
<meta property="og:image" content="https://ozstatic.by/img.jpg">
</head><body>
<div itemtype="https://schema.org/Product">
<h1 itemprop="name">Кружка керамическая</h1>
<span itemprop="price" content="12.50"></span>
<meta itemprop="priceCurrency" content="BYN">
<div itemprop="image"><link itemprop="contentUrl" href="https://goods.ozstatic.by/cup.jpg"></div>
</div>
<script>var x={"name":"Кружка из JSON","id":"12345","price":11.00};</script>
</body></html>`

func TestExtractEmall(t *testing.T) {
	s := Extract(emallHTML, Builtin()[0])
	if s.Title != "Протеин сывороточный Молочный мир" {
		t.Fatalf("title=%q", s.Title)
	}
	if !s.HasPrice || s.Price != 59.99 {
		t.Fatalf("price=%v", s.Price)
	}
}

func TestExtractShopby(t *testing.T) {
	s := Extract(shopbyHTML, Builtin()[1])
	if s.Title != "Смарт-приставка Apple TV 4K" || s.Price != 590 {
		t.Fatalf("got title=%q price=%v", s.Title, s.Price)
	}
}

func TestExtractOzbyMicrodata(t *testing.T) {
	cfg := Builtin()[2]
	s := ExtractPage(ozbyHTML, cfg, "12345")
	if s.Title != "Кружка керамическая" {
		t.Fatalf("title=%q", s.Title)
	}
	if !s.HasPrice || s.Price != 12.50 {
		t.Fatalf("price=%v", s.Price)
	}
	if s.ImageURL != "https://goods.ozstatic.by/cup.jpg" {
		t.Fatalf("image=%q", s.ImageURL)
	}
	if s.Currency != "BYN" {
		t.Fatalf("currency=%q", s.Currency)
	}
}

func TestExtractOzbyEmbedded(t *testing.T) {
	html := `<script>"name":"Товар","id":"999","price":3.5</script>`
	cfg := Config{
		Code: "t", Hosts: []string{"x"}, PathIDGroup: 2,
		Title: []string{"embedded_name_by_id"},
		Price: []string{"embedded_price_by_id"},
		Currency: []string{"default_currency"}, DefaultCurrency: "BYN",
	}
	s := ExtractPage(html, cfg, "999")
	if s.Title != "Товар" || s.Price != 3.5 {
		t.Fatalf("got %+v", s)
	}
}

func TestBuiltinValid(t *testing.T) {
	for _, c := range Builtin() {
		if err := c.Validate(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBuiltinPathRegexCompiles(t *testing.T) {
	for _, c := range Builtin() {
		if err := c.Validate(); err != nil {
			t.Fatal(err)
		}
	}
}
