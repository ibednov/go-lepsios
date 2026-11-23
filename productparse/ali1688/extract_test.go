package ali1688

import (
	"net/http"
	"testing"
)

const sampleDetailHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<title>女童休闲加厚套装粉色兔子印花洋气卫衣外套+卫裤两件套新款秋装 - 阿里巴巴</title>
</head>
<body>
<script>
window.__INIT_DATA = {
  "subject":"女童休闲加厚套装粉色兔子印花洋气卫衣外套+卫裤两件套新款秋装",
  "offerId":841393674653,
  "offerMinPrice":"48.88",
  "minPrice":"48.88",
  "currentPrices":[{"beginAmount":1,"price":"48.88"}],
  "mainImage":["https://cbu01.alicdn.com/img/ibank/O1CN0132j6tN1aV1Tr4EsZL_!!2214597613334-0-cib.jpg"]
};
</script>
</body>
</html>`

const sampleQRHTML = `<iframe src="wireless1688://ma.m.1688.com/offer?id=841393674653.html?offerId=841393674653&amp;amp;__zhi_token=J04owkB64lFas"></iframe>`

func TestValidateProductURL(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{"https://detail.1688.com/offer/841393674653.html", false},
		{"https://m.1688.com/offer/841393674653.html", false},
		{"https://qr.1688.com/s/BahzNGxE", false},
		{"https://1688.com/search", true},
	}
	for _, tt := range tests {
		err := validateProductURL(tt.url)
		if (err != nil) != tt.wantErr {
			t.Fatalf("validateProductURL(%q) err=%v wantErr=%v", tt.url, err, tt.wantErr)
		}
	}
}

func TestOfferIDFromDetailAndQuery(t *testing.T) {
	id, err := resolveOfferID("https://detail.1688.com/offer/841393674653.html", "")
	if err != nil {
		t.Fatal(err)
	}
	if id != "841393674653" {
		t.Fatalf("got %q", id)
	}

	id, err = resolveOfferID("https://ma.m.1688.com/offer?id=841393674653.html", "")
	if err != nil {
		t.Fatal(err)
	}
	if id != "841393674653" {
		t.Fatalf("query id got %q", id)
	}
}

func TestOfferIDFromQRBody(t *testing.T) {
	id, err := resolveOfferID("https://qr.1688.com/s/BahzNGxE", sampleQRHTML)
	if err != nil {
		t.Fatal(err)
	}
	if id != "841393674653" {
		t.Fatalf("got %q", id)
	}
}

func TestParseProductFromHTML(t *testing.T) {
	product, ok := parseProductFromHTML(sampleDetailHTML, "841393674653")
	if !ok {
		t.Fatal("expected product to parse")
	}
	if product.Title != "女童休闲加厚套装粉色兔子印花洋气卫衣外套+卫裤两件套新款秋装" {
		t.Fatalf("title=%q", product.Title)
	}
	if product.Price != 48.88 {
		t.Fatalf("price=%v", product.Price)
	}
	if product.Currency != "CNY" {
		t.Fatalf("currency=%q", product.Currency)
	}
	if product.ImageURL == "" {
		t.Fatal("expected image url")
	}
}

func TestCanHandle(t *testing.T) {
	s := NewStrategy("")
	if !s.CanHandle("detail.1688.com") || !s.CanHandle("qr.1688.com") {
		t.Fatal("expected 1688 hosts")
	}
	if s.CanHandle("wildberries.ru") {
		t.Fatal("did not expect wb")
	}
}

func TestIsPunishPage(t *testing.T) {
	punish := `<a id="a-link" href="https://bixi.alicdn.com/punish/punish:resource:template:cbuSpace:default_38604715.html?qrcode=x|cloud_ip_bl|0&action=deny"></a><!--rgv587_flag:sm-->`
	if !isPunishPage(punish) {
		t.Fatal("expected punish page")
	}
	if isPunishPage(sampleDetailHTML) {
		t.Fatal("did not expect sample product html to look like waf")
	}
}

func TestNewHTTPClientUsesProxy(t *testing.T) {
	client := newHTTPClient("http://127.0.0.1:8888")
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport.Proxy == nil {
		t.Fatal("expected proxy on transport")
	}
	req, err := http.NewRequest(http.MethodGet, "https://detail.1688.com/", nil)
	if err != nil {
		t.Fatal(err)
	}
	u, err := transport.Proxy(req)
	if err != nil || u == nil || u.Host != "127.0.0.1:8888" {
		t.Fatalf("proxy url=%v err=%v", u, err)
	}
}
