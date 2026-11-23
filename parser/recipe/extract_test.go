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

func TestExtractDNS(t *testing.T) {
	html := `<script type="application/ld+json">{"@graph":[{"@type":"Product","name":"Клавиатура 8BitDo","offers":{"price":263.9,"priceCurrency":"BYN"},"image":["https://c.dns-shop.ru/thumb/st4/fit/45/45/abc.jpg"]}]}</script>
<link rel="preload" href="https://c.dns-shop.ru/thumb/st4/fit/500/500/preload.jpg" as="image">
<h1>Клавиатура проводная</h1>
<div class="product-card-purchase__current-price ">263.90</div>`
	var cfg Config
	for _, c := range Builtin() {
		if c.Code == "dns" {
			cfg = c
			break
		}
	}
	s := Extract(html, cfg)
	if s.Title != "Клавиатура 8BitDo" || s.Price != 263.9 {
		t.Fatalf("got title=%q price=%v", s.Title, s.Price)
	}
	if s.ImageURL == "" {
		t.Fatal("image empty")
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
