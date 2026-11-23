package wildberries

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	http "github.com/bogdanfinn/fhttp"
)

func TestChromeTLSClientConstructs(t *testing.T) {
	client, err := chromeTLS()
	if err != nil {
		t.Fatalf("chrome TLS client: %v", err)
	}
	if client == nil {
		t.Fatal("chrome TLS client is nil")
	}
}

func TestChromeTLSHeadersMatchChromeFetch(t *testing.T) {
	h := chromeTLSHeaders("https://www.wildberries.ru")
	if got := firstHeader(h, "user-agent"); !strings.Contains(got, "Chrome/152") {
		t.Fatalf("ua=%q", got)
	}
	if firstHeader(h, "origin") != "https://www.wildberries.ru" {
		t.Fatalf("origin=%q", firstHeader(h, "origin"))
	}
	if firstHeader(h, "sec-fetch-site") != "cross-site" {
		t.Fatalf("sec-fetch-site=%q", firstHeader(h, "sec-fetch-site"))
	}
	order := h[http.HeaderOrderKey]
	if len(order) == 0 || order[0] != "sec-ch-ua" || order[len(order)-1] != "accept-language" {
		t.Fatalf("header order=%v", order)
	}
}

func firstHeader(h http.Header, key string) string {
	vals := h[key]
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

func TestWBCatalogOrigin(t *testing.T) {
	if got := wbCatalogOrigin("https://www.wildberries.ru/catalog/1/detail.aspx"); got != "https://www.wildberries.ru" {
		t.Fatalf("ru origin=%q", got)
	}
	if got := wbCatalogOrigin("https://www.wildberries.by/catalog/1/detail.aspx"); got != "https://www.wildberries.by" {
		t.Fatalf("by origin=%q", got)
	}
}

func TestFetchDetailUsesChromeTLSBody(t *testing.T) {
	prev := doChromeTLSGet
	t.Cleanup(func() { doChromeTLSGet = prev })
	doChromeTLSGet = func(_ context.Context, reqURL, origin string) (int, []byte, error) {
		if !strings.Contains(reqURL, "nm=150707791") || origin != "https://www.wildberries.ru" {
			t.Fatalf("url=%q origin=%q", reqURL, origin)
		}
		return 200, []byte(`{"products":[{"name":"Phone","brand":"Acme","pics":3,"sizes":[{"price":{"product":19900,"logistics":100}}]}]}`), nil
	}

	data, err := fetchDetail(context.Background(), "https://www.wildberries.ru/catalog/150707791/detail.aspx", "150707791")
	if err != nil {
		t.Fatal(err)
	}
	title, price, pics, err := extractProductInfo(data, "https://www.wildberries.ru/catalog/150707791/detail.aspx")
	if err != nil {
		t.Fatal(err)
	}
	if title != "Phone - Бренд Acme" || price != 199 || pics != 3 {
		t.Fatalf("title=%q price=%v pics=%d", title, price, pics)
	}
}

func TestFetchDetailRecordsBothHostsOn403(t *testing.T) {
	prev := doChromeTLSGet
	t.Cleanup(func() { doChromeTLSGet = prev })
	doChromeTLSGet = func(context.Context, string, string) (int, []byte, error) {
		return 403, []byte("wbaas"), nil
	}

	ctx, trace := withWBTrace(context.Background())
	_, err := fetchDetail(ctx, "https://www.wildberries.by/catalog/409842715/detail.aspx", "409842715")
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") {
		t.Fatalf("err=%v", err)
	}
	// .by tries primary dest then alt × two detail hosts.
	if len(trace.calls) != 4 {
		t.Fatalf("calls=%d %+v", len(trace.calls), trace.calls)
	}
	if trace.calls[0].Name != "card.wb.ru" || trace.calls[0].Status != 403 {
		t.Fatalf("first=%+v", trace.calls[0])
	}
	if trace.calls[1].Name != "u-card.wb.ru" || trace.calls[1].Status != 403 {
		t.Fatalf("second=%+v", trace.calls[1])
	}
	if !strings.Contains(trace.calls[0].URL, "curr=byn") || !strings.Contains(trace.calls[0].URL, "nm=409842715") {
		t.Fatalf("url=%q", trace.calls[0].URL)
	}
	if !strings.Contains(trace.calls[0].URL, "dest="+wbDestBYPrimary) {
		t.Fatalf("primary dest missing: %q", trace.calls[0].URL)
	}
	if !strings.Contains(trace.calls[2].URL, "dest="+wbDestBYAlt) {
		t.Fatalf("alt dest missing: %q", trace.calls[2].URL)
	}
}

func TestFetchDetailChromeTLSHTTPStatus(t *testing.T) {
	prev := doChromeTLSGet
	t.Cleanup(func() { doChromeTLSGet = prev })
	doChromeTLSGet = func(context.Context, string, string) (int, []byte, error) {
		return 403, []byte("wbaas"), nil
	}

	_, err := fetchDetail(context.Background(), "https://www.wildberries.ru/catalog/1/detail.aspx", "1")
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") {
		t.Fatalf("err=%v", err)
	}
}

func TestFetchDetailFallsBackWhenChromeTLSUnavailable(t *testing.T) {
	prev := doChromeTLSGet
	t.Cleanup(func() { doChromeTLSGet = prev })
	doChromeTLSGet = func(context.Context, string, string) (int, []byte, error) {
		return 0, nil, errChromeTLSUnavailable
	}

	_, err := fetchDetail(context.Background(), "https://www.wildberries.ru/catalog/1/detail.aspx", "1")
	if err == nil {
		t.Fatal("expected stdlib fallback error")
	}
	if errors.Is(err, errChromeTLSUnavailable) {
		t.Fatalf("should not leak chrome init error: %v", err)
	}
}

func TestChromeTLSFingerprintLive(t *testing.T) {
	if testing.Short() {
		t.Skip("live fingerprint")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	status, body, err := chromeTLSGetJSON(ctx, "https://tls.peet.ws/api/all", "https://www.wildberries.ru")
	if err != nil {
		t.Skip(err)
	}
	if status != 200 {
		t.Skipf("peet.ws HTTP %d", status)
	}
	if !strings.Contains(string(body), `"tls"`) {
		t.Fatalf("unexpected peet body: %s", body)
	}
}
