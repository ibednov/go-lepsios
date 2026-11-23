package taobao

import (
	"context"
	"testing"
)

const sampleShortHTML = `<html><head><title></title></head><body>
<script>
var url = 'https://item.taobao.com/item.htm?id=785934378423&sourceType=item&price=19.9&shareUniqueId=1';
var invalidUrl = "http://m.tb.cn/scanError.htm";
</script>
</body></html>`

const sampleListHTML = `<html><head>
<meta property="og:title" content="断码清仓春秋新款季女童公主鞋-淘宝Taobao | 天猫Tmall">
<meta property="og:image" content="https://img.alicdn.com/imgextra/i3/example.jpg">
</head><body>
<script>window.__INIT__={"itemExist":true,"itemPrice":{"hasRangePrice":false,"originalPrice":"19.90","promotionPrice":"19.90"}}</script>
</body></html>`

const sampleContentFirstOG = `<html><head>
<meta content="断码清仓春秋新款季女童公主鞋-淘宝Taobao | 天猫Tmall" property="og:title">
<meta content="https://img.alicdn.com/imgextra/i3/example.jpg" property="og:image">
</head><body>
<script>{"title":"断码清仓春秋新款季女童公主鞋"},"itemExist":true,"itemPrice":{"promotionPrice":19.9}}</script>
</body></html>`

const sampleBlockedHTML = `<a id="a-link"></a><script>var isMobile = navigator.userAgent.match(/login/);</script>`

func TestItemIDFromItemAndList(t *testing.T) {
	id, _, err := (&Strategy{}).resolveItemIDFromParsed("https://item.taobao.com/item.htm?id=785934378423")
	if err != nil || id != "785934378423" {
		t.Fatalf("item.htm id=%q err=%v", id, err)
	}
}

func TestItemIDFromShortHTML(t *testing.T) {
	id, price, ok := itemIDFromShortHTML(sampleShortHTML)
	if !ok || id != "785934378423" {
		t.Fatalf("id=%q ok=%v", id, ok)
	}
	if price != 19.9 {
		t.Fatalf("price=%v", price)
	}
}

func TestParseCardHTML(t *testing.T) {
	product := parseCardHTML(sampleListHTML, "785934378423")
	if product.Title != "断码清仓春秋新款季女童公主鞋" {
		t.Fatalf("title=%q", product.Title)
	}
	if product.Price != 19.9 {
		t.Fatalf("price=%v", product.Price)
	}
	if product.Currency != "CNY" || product.ImageURL == "" {
		t.Fatalf("currency=%q image=%q", product.Currency, product.ImageURL)
	}
}

func TestParseCardHTMLContentFirstAndJSON(t *testing.T) {
	product := parseCardHTML(sampleContentFirstOG, "785934378423")
	if product.Title != "断码清仓春秋新款季女童公主鞋" {
		t.Fatalf("title=%q", product.Title)
	}
	if product.Price != 19.9 {
		t.Fatalf("price=%v", product.Price)
	}
}

func TestAssembleKeepsJSONTitleOnSmallCard(t *testing.T) {
	htmlBody := `<html><body><script>{"title":"断码清仓春秋新款季女童公主鞋"},"itemExist":true</script></body></html>`
	if !isBlockedCardHTML(htmlBody) {
		t.Fatal("expected size heuristic to mark card blocked")
	}
	product, err := assembleProduct("785934378423", 19.9, []string{htmlBody}, true)
	if err != nil {
		t.Fatal(err)
	}
	if product.Title != "断码清仓春秋新款季女童公主鞋" {
		t.Fatalf("title=%q", product.Title)
	}
	if product.Price != 19.9 {
		t.Fatalf("price=%v", product.Price)
	}
}

func TestAssembleUsesShortLinkPriceWhenCardEmpty(t *testing.T) {
	product, err := assembleProduct("785934378423", 19.9, []string{sampleBlockedHTML}, true)
	if err != nil {
		t.Fatal(err)
	}
	if product.Price != 19.9 {
		t.Fatalf("price=%v", product.Price)
	}
}

func TestAssembleBlockedWithoutPrice(t *testing.T) {
	_, err := assembleProduct("785934378423", 0, nil, true)
	if err != errAccessDenied {
		t.Fatalf("err=%v", err)
	}
}

func TestAssembleMissingProduct(t *testing.T) {
	_, err := assembleProduct("785934378423", 0, []string{`<html><body>nope</body></html>`}, false)
	if err != errProductNotFound {
		t.Fatalf("err=%v", err)
	}
}

func TestCanHandle(t *testing.T) {
	s := NewStrategy("")
	if !s.CanHandle("e.tb.cn") || !s.CanHandle("item.taobao.com") || !s.CanHandle("www.taobao.com") {
		t.Fatal("expected taobao hosts")
	}
	if s.CanHandle("1688.com") {
		t.Fatal("did not expect 1688")
	}
}

func TestValidateShortLink(t *testing.T) {
	if err := validateProductURL("https://e.tb.cn/h.8QIkCHR7m0pUjke?tk=tcNHT2B1zad"); err != nil {
		t.Fatal(err)
	}
}

func (s *Strategy) resolveItemIDFromParsed(pageURL string) (string, float64, error) {
	return s.resolveItemID(context.Background(), pageURL)
}
