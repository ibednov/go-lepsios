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

func TestBuiltinValid(t *testing.T) {
	for _, c := range Builtin() {
		if err := c.Validate(); err != nil {
			t.Fatal(err)
		}
	}
}
