package sitemapjsonld

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	entity "github.com/ibednov/go-lepsios/crawler"
	crawlerr "github.com/ibednov/go-lepsios/crawler/errors"
	"github.com/ibednov/go-lepsios/crawler/httpclient"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "testdata", "sitemap_jsonld", name))
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestDiscoverAndFetchConfiguredJSONLD(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host
		_, _ = w.Write([]byte(`<urlset><url><loc>` + base + `/product/green-100</loc></url><url><loc>` + base + `/about</loc></url></urlset>`))
	})
	mux.HandleFunc("/product/green-100", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<script type="application/ld+json">{"@context":"https://schema.org","@type":"Product","name":"Green tea 100 g","sku":"green","category":"Green tea","image":"/img/green.jpg","offers":{"@type":"Offer","price":"12.50","priceCurrency":"BYN","availability":"https://schema.org/InStock"}}</script>`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	base := server.URL
	config, _ := json.Marshal(Config{Version: 1, SitemapURL: base + "/sitemap.xml", ProductPathPrefixes: []string{"/product/"}, DefaultCurrency: "BYN"})
	adapter, err := New(httpclient.New(httpclient.Config{DefaultRPS: 1000}), "demo", base, 1000, config)
	if err != nil {
		t.Fatal(err)
	}
	refs, err := adapter.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].URL != base+"/product/green-100" {
		t.Fatalf("refs=%+v", refs)
	}
	offers, err := adapter.Fetch(context.Background(), refs[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(offers) != 1 || offers[0].Price.String() != "12.5" || offers[0].Currency != "BYN" || offers[0].WeightG == nil || *offers[0].WeightG != 100 || !offers[0].InStock || offers[0].ImageURL != base+"/img/green.jpg" {
		t.Fatalf("offers=%+v", offers)
	}
}

func TestConfiguredCategoryURLIsNotAProduct(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sitemap.xml" {
			_, _ = fmt.Fprintf(w, `<urlset><url><loc>http://%s/shop</loc></url><url><loc>http://%s/shop/tea</loc></url></urlset>`, r.Host, r.Host)
			return
		}
		_, _ = w.Write([]byte(`<script type="application/ld+json">{"@type":"Product","name":"Tea","offers":{"@type":"Offer","price":"10","priceCurrency":"BYN"}}</script>`))
	}))
	defer server.Close()
	config, _ := json.Marshal(Config{Version: 1, SitemapURL: server.URL + "/sitemap.xml", CategoryURLs: []string{server.URL + "/shop"}, ProductPathPrefixes: []string{"/shop"}})
	adapter, err := New(httpclient.New(httpclient.Config{DefaultRPS: 1000}), "demo", server.URL, 1000, config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Fetch(context.Background(), entity.ProductRef{URL: server.URL + "/shop"}); err == nil {
		t.Fatal("category URL was accepted as product")
	}
	if _, err := adapter.Replay(entity.ProductRef{URL: server.URL + "/shop"}, httpclient.DiagnosticResponse{Body: []byte(`x`)}); err == nil {
		t.Fatal("category URL was accepted for replay")
	}
}

func TestDecodeHTMLProductReadsVisiblePriceText(t *testing.T) {
	cfg := Config{
		HTMLTitleSelector: "h1",
		HTMLPriceSelector: ".prod_price",
	}
	products, err := decodeHTMLProduct([]byte(`<h1>Чай HYLEYS</h1><div class="prod_price">Цена: <span>5p80 коп.</span></div>`), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(products) != 1 || len(products[0].Offers) != 1 || normalizePrice(products[0].Offers[0].Price) != "5.80" {
		t.Fatalf("products=%+v, want visible price normalizable to 5.80", products)
	}
}

func TestConfiguredZeroPriceOutOfStockProductCanBeSkipped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<script type="application/ld+json">{"@type":"Product","name":"Tea","offers":{"@type":"Offer","price":"0.00","priceCurrency":"BYN","availability":"https://schema.org/OutOfStock"}}</script>`))
	}))
	defer server.Close()
	ref := entity.ProductRef{URL: server.URL + "/product/tea"}
	for _, tc := range []struct {
		name string
		cfg  Config
		want error
	}{
		{name: "skipped by default", cfg: Config{Version: 1, SitemapURL: server.URL + "/sitemap.xml"}, want: crawlerr.ErrSkippedProduct},
		{name: "configured skip remains skip", cfg: Config{Version: 1, SitemapURL: server.URL + "/sitemap.xml", SkipOutOfStockZeroPrice: true}, want: crawlerr.ErrSkippedProduct},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config, _ := json.Marshal(tc.cfg)
			a, err := New(httpclient.New(httpclient.Config{DefaultRPS: 1000}), "demo", server.URL, 1000, config)
			if err != nil {
				t.Fatal(err)
			}
			offers, err := a.Fetch(context.Background(), ref)
			if len(offers) != 0 {
				t.Fatalf("offers=%+v, want none", offers)
			}
			if tc.want != nil {
				if !errors.Is(err, tc.want) {
					t.Fatalf("error=%v, want %v", err, tc.want)
				}
				return
			}
			var crawlError *crawlerr.CrawlError
			if !errors.As(err, &crawlError) || crawlError.Code != "parse" {
				t.Fatalf("error=%v, want strict parse failure", err)
			}
		})
	}
}

func TestSharedStrategyFixtureRegression(t *testing.T) {
	sitemapFixture := string(readFixture(t, "sitemap.xml"))
	productFixtures := map[string][]byte{
		"/product/variants":        readFixture(t, "product_variants.html"),
		"/product/missing-price":   readFixture(t, "product_missing_price.html"),
		"/product/ambiguous-price": readFixture(t, "product_ambiguous_price.html"),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body []byte
		switch r.URL.Path {
		case "/sitemap.xml":
			body = []byte(strings.ReplaceAll(sitemapFixture, "BASE", "http://"+r.Host))
		default:
			var ok bool
			body, ok = productFixtures[r.URL.Path]
			if !ok {
				http.NotFound(w, r)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.URL.Path == "/sitemap.xml" {
			w.Header().Set("Content-Type", "application/xml")
		}
		_, _ = w.Write(body)
	}))
	defer server.Close()

	config, _ := json.Marshal(Config{Version: 1, SitemapURL: server.URL + "/sitemap.xml", ProductPathPrefixes: []string{"/product/"}, DefaultCurrency: "BYN"})
	adapter, err := New(httpclient.New(httpclient.Config{DefaultRPS: 1000}), "fixture-shop", server.URL, 1000, config)
	if err != nil {
		t.Fatal(err)
	}
	discovery, err := entity.DiscoverResultFor(context.Background(), adapter)
	if err != nil || discovery.Outcome != entity.OutcomeOK || len(discovery.Refs) != 1 || discovery.Refs[0].URL != server.URL+"/product/variants" {
		t.Fatalf("discovery=%+v err=%v", discovery, err)
	}
	offers, err := adapter.Fetch(context.Background(), discovery.Refs[0])
	want := []struct {
		sku, price string
		weight     int
		stock      bool
	}{{"shu-50", "12.5", 50, true}, {"shu-100", "21.75", 100, false}}
	if err != nil || len(offers) != len(want) {
		t.Fatalf("offers=%+v err=%v", offers, err)
	}
	for i, expected := range want {
		got := offers[i]
		if got.SourceSKU != expected.sku || got.Price.String() != expected.price || got.Currency != "BYN" || got.WeightG == nil || *got.WeightG != expected.weight || got.InStock != expected.stock || got.URL != discovery.Refs[0].URL {
			t.Errorf("offer[%d]=%+v", i, got)
		}
	}
	result, err := entity.FetchResultFor(context.Background(), adapter, entity.ProductRef{SourceCode: "fixture-shop", URL: server.URL + "/product/missing-price"})
	if !errors.Is(err, crawlerr.ErrSkippedProduct) || result.Outcome != entity.OutcomeSkipped || len(result.Offers) != 0 {
		t.Fatalf("missing price result=%+v err=%v, want skip", result, err)
	}
	result, err = entity.FetchResultFor(context.Background(), adapter, entity.ProductRef{SourceCode: "fixture-shop", URL: server.URL + "/product/ambiguous-price"})
	var crawlError *crawlerr.CrawlError
	if !errors.As(err, &crawlError) || crawlError.Code != "parse" || result.Outcome != entity.OutcomeParseFailed || len(result.Offers) != 0 {
		t.Fatalf("ambiguous price result=%+v err=%v, want parse failure", result, err)
	}
}

func TestEmptyProductSitemapIsParseFailure(t *testing.T) {
	body := readFixture(t, "empty_sitemap.xml")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sitemap.xml" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write(body)
	}))
	defer server.Close()

	cfg, _ := json.Marshal(Config{Version: 1, SitemapURL: server.URL + "/sitemap.xml", ProductPathPrefixes: []string{"/product/"}})
	a, err := New(httpclient.New(httpclient.Config{DefaultRPS: 1000}), "empty-fixture-shop", server.URL, 1000, cfg)
	if err != nil {
		t.Fatal(err)
	}
	result, err := entity.DiscoverResultFor(context.Background(), a)
	var crawlError *crawlerr.CrawlError
	if len(result.Refs) != 0 || result.Outcome != entity.OutcomeParseFailed || result.Evidence != "" || !errors.As(err, &crawlError) || crawlError.Code != "parse" {
		t.Fatalf("result=%+v err=%v, want parse failure without empty-catalog evidence", result, err)
	}
	if refs, err := a.Discover(context.Background()); len(refs) != 0 || !errors.As(err, &crawlError) || crawlError.Code != "parse" {
		t.Fatalf("legacy Discover refs=%+v err=%v, want parse failure", refs, err)
	}
}

func TestFetchCapturesBoundedFailureResponseWithoutURLSecrets(t *testing.T) {
	body := bytes.Repeat([]byte("x"), 150<<10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(body)
	}))
	defer server.Close()
	cfg, _ := json.Marshal(Config{SitemapURL: server.URL + "/sitemap.xml", ProductPathPrefixes: []string{"/product/"}})
	a, err := New(httpclient.New(httpclient.Config{DefaultRPS: 1000, MaxBodyBytes: 200 << 10}), "demo", server.URL, 1000, cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.Fetch(context.Background(), entity.ProductRef{URL: server.URL + "/product/broken?token=secret"})
	if err == nil {
		t.Fatal("expected malformed response to fail parsing")
	}
	diagnostic, ok := a.LastDiagnosticResponse()
	if !ok {
		t.Fatal("expected response diagnostic")
	}
	if strings.Contains(diagnostic.URL, "secret") || strings.Contains(diagnostic.URL, "?") {
		t.Fatalf("diagnostic URL retained query data: %q", diagnostic.URL)
	}
	if diagnostic.StatusCode != http.StatusOK || diagnostic.ContentType != "text/html; charset=utf-8" {
		t.Fatalf("response metadata=%+v", diagnostic)
	}
	if len(diagnostic.Body) != httpclient.MaxDiagnosticBodyBytes || !diagnostic.StorageTruncated || diagnostic.BodyTruncated {
		t.Fatalf("body bounds=%d storage_truncated=%v body_truncated=%v", len(diagnostic.Body), diagnostic.StorageTruncated, diagnostic.BodyTruncated)
	}
	if diagnostic.BytesRead != int64(len(body)) || diagnostic.BodyHash != httpclient.CapturedBodyHash(body) {
		t.Fatalf("read metadata bytes=%d hash_matches=%v", diagnostic.BytesRead, diagnostic.BodyHash == httpclient.CapturedBodyHash(body))
	}
}

func TestReplayUsesCapturedBodyWithoutNetwork(t *testing.T) {
	const source = `https://shop.example`
	cfg, _ := json.Marshal(Config{SitemapURL: source + "/sitemap.xml", ProductPathPrefixes: []string{"/product/"}})
	a, err := New(httpclient.New(httpclient.Config{}), "demo", source, 1, cfg)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`<script type="application/ld+json">{"@type":"Product","name":"Green tea 100 g","sku":"g1","offers":{"@type":"Offer","price":"12.50","priceCurrency":"BYN"}}</script>`)
	offers, err := a.Replay(entity.ProductRef{URL: source + "/product/green"}, httpclient.DiagnosticResponse{Body: body})
	if err != nil || len(offers) != 1 || offers[0].Price.String() != "12.5" {
		t.Fatalf("replay offers=%+v err=%v", offers, err)
	}
	if _, err := a.Replay(entity.ProductRef{URL: source + "/product/green"}, httpclient.DiagnosticResponse{Body: body, StorageTruncated: true}); err == nil {
		t.Fatal("truncated replay body should be rejected")
	}
}

func TestHTMLSelectorsExtractOfferAndFailMissingPrice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/product/missing" {
			_, _ = w.Write([]byte(`<h1 id="n">Чай 100 г</h1>`))
			return
		}
		_, _ = w.Write([]byte(`<h1 id="n">Шу пуэр 25*2 гр</h1><input class="product_price" value="24.58"><button class="add">Купить</button>`))
	}))
	defer server.Close()
	cfg, _ := json.Marshal(Config{SitemapURL: server.URL + "/sitemap.xml", ProductPathPrefixes: []string{"/product/"}, DataFormats: []string{"html"}, HTMLTitleSelector: "#n", HTMLPriceSelector: ".product_price", HTMLStockSelector: ".add"})
	adapter, err := New(httpclient.New(httpclient.Config{DefaultRPS: 1000}), "chay-by", server.URL, 1000, cfg)
	if err != nil {
		t.Fatal(err)
	}
	offers, err := adapter.Fetch(context.Background(), entity.ProductRef{URL: server.URL + "/product/tea"})
	if err != nil || len(offers) != 1 || offers[0].Price.String() != "24.58" || !offers[0].InStock || offers[0].WeightG == nil || *offers[0].WeightG != 50 {
		t.Fatalf("offers=%+v err=%v", offers, err)
	}
	_, err = adapter.Fetch(context.Background(), entity.ProductRef{URL: server.URL + "/product/missing"})
	if !errors.Is(err, crawlerr.ErrSkippedProduct) {
		t.Fatalf("missing price should be skipped: %v", err)
	}
}

func TestPathRegexContainsAndHTTPSUpgrade(t *testing.T) {
	if got := upgradePublicURL("http://blackgreen.by/product/tea"); got != "https://blackgreen.by/product/tea" {
		t.Fatalf("upgrade=%s", got)
	}
	if got := upgradePublicURL("http://127.0.0.1:1234/product/tea"); got != "http://127.0.0.1:1234/product/tea" {
		t.Fatalf("loopback=%s", got)
	}
	cfg, _ := json.Marshal(Config{SitemapURL: "http://blackgreen.by/sitemap.xml", IncludePathRegex: []string{`^/catalog/.+/tea-[0-9]+$`}, IncludePathContains: []string{"/tea-"}, ExcludePathRegex: []string{`/archive/`}})
	a, err := New(httpclient.New(httpclient.Config{}), "blackgreen-by", "http://blackgreen.by", 1, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if a.base.Scheme != "https" || a.config.SitemapURL != "https://blackgreen.by/sitemap.xml" {
		t.Fatalf("base=%s sitemap=%s", a.base, a.config.SitemapURL)
	}
	if !a.isProductPath("/catalog/puer/tea-123") || a.isProductPath("/catalog/puer/coffee-123") || a.isProductPath("/archive/tea-123") {
		t.Fatal("path filters")
	}
	if u, ok := a.sameSiteURL("http://blackgreen.by/catalog/puer/tea-123"); !ok || u.Scheme != "https" {
		t.Fatalf("url=%v ok=%v", u, ok)
	}
	bad, _ := json.Marshal(Config{SitemapURL: "https://blackgreen.by/sitemap.xml", IncludePathRegex: []string{"["}})
	if err := ValidateConfig("https://blackgreen.by", bad); err == nil {
		t.Fatal("invalid regex accepted")
	}
}

func TestCategoryCardsYieldDistinctOffers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<div class="js-product" data-product-lid="101"><div class="js-product-name">Улун 50 г</div><div class="js-product-price">14.95</div></div><div class="js-product" data-product-lid="102"><div class="js-product-name">Шу пуэр 100 г</div><div class="js-product-price">24.50</div></div><a href="/about">About</a>`))
	}))
	defer server.Close()
	cfg, _ := json.Marshal(Config{CategoryURLs: []string{server.URL + "/ulun"}, CategoryCardSelector: ".js-product", CategoryCardTitleSelector: ".js-product-name", CategoryCardPriceSelector: ".js-product-price", CategoryCardIDAttribute: "data-product-lid"})
	a, err := New(httpclient.New(httpclient.Config{DefaultRPS: 1000}), "chaylavka-by", server.URL, 1000, cfg)
	if err != nil {
		t.Fatal(err)
	}
	refs, err := a.Discover(context.Background())
	if err != nil || len(refs) != 2 {
		t.Fatalf("refs=%+v err=%v", refs, err)
	}
	result, err := entity.DiscoverResultFor(context.Background(), a)
	if err != nil || result.Outcome != entity.OutcomeOK || len(result.Refs) != 2 {
		t.Fatalf("outcome discovery=%+v err=%v", result, err)
	}
	if refs[0].URL == refs[1].URL {
		t.Fatal("category cards must have distinct URLs")
	}
	for i, want := range []string{"14.95", "24.5"} {
		offers, err := a.Fetch(context.Background(), refs[i])
		if err != nil || len(offers) != 1 || offers[0].Price.String() != want || offers[0].Currency != "BYN" {
			t.Fatalf("offer %d=%+v err=%v", i, offers, err)
		}
	}
}

func TestConfiguredCategoryCardsReuseDiscoverySnapshot(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		if requests == 1 {
			_, _ = w.Write([]byte(`<div class="product" data-id="tea-1"><span class="name">Шу пуэр 100 г</span><span class="price">20.00</span></div>`))
			return
		}
		_, _ = w.Write([]byte(`<main class="category-v2"><p>Категория</p></main>`))
	}))
	defer server.Close()
	cfg, _ := json.Marshal(Config{CategoryURLs: []string{server.URL + "/tea"}, CategoryCardSelector: ".product", CategoryCardTitleSelector: ".name", CategoryCardPriceSelector: ".price", CategoryCardIDAttribute: "data-id", SnapshotCategoryCards: true})
	a, err := New(httpclient.New(httpclient.Config{DefaultRPS: 1000}), "demo", server.URL, 1000, cfg)
	if err != nil {
		t.Fatal(err)
	}
	refs, err := a.Discover(context.Background())
	if err != nil || len(refs) != 1 {
		t.Fatalf("refs=%+v err=%v", refs, err)
	}
	offers, err := a.Fetch(context.Background(), refs[0])
	if err != nil || len(offers) != 1 || offers[0].Price.String() != "20" {
		t.Fatalf("offers=%+v err=%v", offers, err)
	}
	if requests != 1 {
		t.Fatalf("category snapshot caused %d requests, want one discovery fetch", requests)
	}
}

func TestCategoryCardDisappearingAfterDiscoveryIsParseFailure(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		if requests == 1 {
			_, _ = w.Write([]byte(`<div class="product" data-id="tea-1"><span class="name">Шу пуэр 100 г</span><span class="price">20.00</span></div>`))
			return
		}
		_, _ = w.Write([]byte(`<main class="category-v2"><p>Категория</p></main>`))
	}))
	defer server.Close()
	cfg, _ := json.Marshal(Config{CategoryURLs: []string{server.URL + "/tea"}, CategoryCardSelector: ".product", CategoryCardTitleSelector: ".name", CategoryCardPriceSelector: ".price", CategoryCardIDAttribute: "data-id"})
	a, err := New(httpclient.New(httpclient.Config{DefaultRPS: 1000}), "demo", server.URL, 1000, cfg)
	if err != nil {
		t.Fatal(err)
	}
	refs, err := a.Discover(context.Background())
	if err != nil || len(refs) != 1 {
		t.Fatalf("refs=%+v err=%v", refs, err)
	}
	offers, err := a.Fetch(context.Background(), refs[0])
	var crawlError *crawlerr.CrawlError
	if len(offers) != 0 || !errors.As(err, &crawlError) || crawlError.Code != "parse" {
		t.Fatalf("offers=%+v err=%v, want parse failure without offers", offers, err)
	}
}

func TestConfiguredMissingCategoryCardIsSkipped(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		if requests == 1 {
			_, _ = w.Write([]byte(`<div class="product" data-id="tea-1"><span class="name">Шу пуэр 100 г</span><span class="price">20.00</span></div>`))
			return
		}
		_, _ = w.Write([]byte(`<main class="category-v2"><p>Категория</p></main>`))
	}))
	defer server.Close()
	cfg, _ := json.Marshal(Config{CategoryURLs: []string{server.URL + "/tea"}, CategoryCardSelector: ".product", CategoryCardTitleSelector: ".name", CategoryCardPriceSelector: ".price", CategoryCardIDAttribute: "data-id", SkipMissingCard: true})
	a, err := New(httpclient.New(httpclient.Config{DefaultRPS: 1000}), "demo", server.URL, 1000, cfg)
	if err != nil {
		t.Fatal(err)
	}
	refs, err := a.Discover(context.Background())
	if err != nil || len(refs) != 1 {
		t.Fatalf("refs=%+v err=%v", refs, err)
	}
	_, err = a.Fetch(context.Background(), refs[0])
	if !errors.Is(err, crawlerr.ErrSkippedProduct) {
		t.Fatalf("missing configured category card must be skipped: %v", err)
	}
}

func TestGenericProductHTTP404IsNotConfirmedDiscontinued(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "not found", http.StatusNotFound) }))
	defer server.Close()
	cfg, _ := json.Marshal(Config{SitemapURL: server.URL + "/sitemap.xml", ProductPathPrefixes: []string{"/product/"}})
	a, err := New(httpclient.New(httpclient.Config{DefaultRPS: 1000}), "demo", server.URL, 1000, cfg)
	if err != nil {
		t.Fatal(err)
	}
	offers, err := a.Fetch(context.Background(), entity.ProductRef{URL: server.URL + "/product/gone"})
	var crawlError *crawlerr.CrawlError
	if len(offers) != 0 || !errors.As(err, &crawlError) || crawlError.Code != "parse" || entity.ReasonForError(err) != "not_found" {
		t.Fatalf("offers=%+v err=%v, want parse failure with not_found reason", offers, err)
	}
}

func TestOutcomeContractRequiresExplicitCategoryEmptyEvidence(t *testing.T) {
	for _, tc := range []struct {
		name        string
		body        string
		wantOutcome entity.Outcome
		wantErr     bool
	}{
		{name: "explicit empty marker", body: string(readFixture(t, "category_empty.html")), wantOutcome: entity.OutcomeConfirmedEmpty},
		{name: "no products without marker", body: `<main><p>Welcome</p></main>`, wantOutcome: entity.OutcomeParseFailed, wantErr: true},
		{name: "changed selector", body: string(readFixture(t, "category_changed.html")), wantOutcome: entity.OutcomeParseFailed, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(tc.body)) }))
			defer server.Close()
			cfg, _ := json.Marshal(Config{CategoryURLs: []string{server.URL + "/tea"}, ProductPathPrefixes: []string{"/product/"}, CategoryEmptySelector: ".empty-state"})
			a, err := New(httpclient.New(httpclient.Config{DefaultRPS: 1000}), "demo", server.URL, 1000, cfg)
			if err != nil {
				t.Fatal(err)
			}
			result, err := entity.DiscoverResultFor(context.Background(), a)
			if (err != nil) != tc.wantErr || result.Outcome != tc.wantOutcome {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if tc.wantOutcome == entity.OutcomeConfirmedEmpty && result.Evidence == "" {
				t.Fatal("confirmed empty result has no evidence")
			}
			if _, err := a.Discover(context.Background()); err == nil {
				t.Fatal("legacy Discover must not turn confirmed empty into successful empty refs")
			}
		})
	}
}

func TestOutcomeContractDoesNotConfirmPartialOrNonemptyTraversalEmpty(t *testing.T) {
	for _, tc := range []struct {
		name    string
		handler func(http.ResponseWriter, *http.Request)
		wantErr bool
	}{
		{name: "pagination has unmarked page", handler: func(w http.ResponseWriter, r *http.Request) {
			if r.URL.RawQuery == "" {
				_, _ = w.Write([]byte(`<a rel="next" href="/tea?page=2">next</a><p class="empty-state">empty</p>`))
				return
			}
			_, _ = w.Write([]byte(`<main>changed page</main>`))
		}, wantErr: true},
		{name: "partial HTTP failure", handler: func(w http.ResponseWriter, r *http.Request) {
			if r.URL.RawQuery == "" {
				_, _ = w.Write([]byte(`<a rel="next" href="/tea?page=2">next</a><p class="empty-state">empty</p>`))
				return
			}
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
		}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tc.handler))
			defer server.Close()
			cfg, _ := json.Marshal(Config{CategoryURLs: []string{server.URL + "/tea"}, ProductPathPrefixes: []string{"/product/"}, CategoryEmptySelector: ".empty-state"})
			a, err := New(httpclient.New(httpclient.Config{DefaultRPS: 1000}), "demo", server.URL, 1000, cfg)
			if err != nil {
				t.Fatal(err)
			}
			result, err := entity.DiscoverResultFor(context.Background(), a)
			if err == nil || !tc.wantErr || result.Outcome == entity.OutcomeConfirmedEmpty {
				t.Fatalf("result=%+v err=%v", result, err)
			}
		})
	}
}

func TestNonBYNOfferIsSkipped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<script type="application/ld+json">{"@type":"Product","name":"Tea","offers":{"price":"20","priceCurrency":"RUB"}}</script>`))
	}))
	defer server.Close()
	cfg, _ := json.Marshal(Config{SitemapURL: server.URL + "/sitemap.xml"})
	a, err := New(httpclient.New(httpclient.Config{DefaultRPS: 1000}), "demo", server.URL, 1000, cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.Fetch(context.Background(), entity.ProductRef{URL: server.URL + "/product/tea"})
	if !errors.Is(err, crawlerr.ErrSkippedProduct) {
		t.Fatalf("foreign currency must skip: %v", err)
	}
}

func TestNotFoundProductIsSkipped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil)
	}))
	defer server.Close()
	cfg, _ := json.Marshal(Config{SitemapURL: server.URL + "/sitemap.xml", ProductPathPrefixes: []string{"/product/"}, SkipNotFound: true})
	a, err := New(httpclient.New(httpclient.Config{DefaultRPS: 1000}), "demo", server.URL, 1000, cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.Fetch(context.Background(), entity.ProductRef{URL: server.URL + "/product/removed"})
	if !errors.Is(err, crawlerr.ErrSkippedProduct) {
		t.Fatalf("stale product URL must be skipped: %v", err)
	}
}

func TestConfiguredNonTeaCategoryIsSkippedFromFixture(t *testing.T) {
	body := readFixture(t, "product_non_tea.html")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer server.Close()
	cfg, _ := json.Marshal(Config{SitemapURL: server.URL + "/sitemap.xml", ExcludeCategoryHints: []string{"Посуда"}})
	a, err := New(httpclient.New(httpclient.Config{DefaultRPS: 1000}), "demo", server.URL, 1000, cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.Fetch(context.Background(), entity.ProductRef{URL: server.URL + "/product/kettle"})
	if !errors.Is(err, crawlerr.ErrSkippedProduct) {
		t.Fatalf("configured non-tea category error=%v, want intentional skip", err)
	}
}

func TestConfiguredAdapterAllowsOptionalPathRulesAndRequiresSameOrigin(t *testing.T) {
	cli := httpclient.New(httpclient.Config{})
	if _, err := New(cli, "demo", "https://tea.example.test", 1, json.RawMessage(`{"version":1,"sitemap_url":"https://tea.example.test/sitemap.xml"}`)); err != nil {
		t.Fatalf("optional path rules: %v", err)
	}
	if _, err := New(cli, "demo", "https://tea.example.test", 1, json.RawMessage(`{"version":1,"sitemap_url":"https://other.example.test/sitemap.xml","product_path_prefixes":["/product/"]}`)); err == nil {
		t.Fatal("expected off-origin sitemap to fail")
	}
}

func TestConfiguredAdapterAllowsRUBAndRejectsUnknownCurrency(t *testing.T) {
	if err := ValidateConfig("https://tea.example.test", json.RawMessage(`{"version":1,"sitemap_url":"https://tea.example.test/sitemap.xml","product_path_prefixes":["/product/"],"currency":"RUB"}`)); err != nil {
		t.Fatalf("RUB must be accepted: %v", err)
	}
	if err := ValidateConfig("https://tea.example.test", json.RawMessage(`{"version":1,"sitemap_url":"https://tea.example.test/sitemap.xml","product_path_prefixes":["/product/"],"currency":"KZT"}`)); err == nil {
		t.Fatal("expected unsupported currency to fail before enabling source")
	}
}

func TestConfiguredRUBOffersUseJSONLDWeightProperty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<script type="application/ld+json">{"@type":"Product","name":"Эфиопия Иргачефф","offers":[{"@type":"Offer","price":"529","priceCurrency":"RUB","availability":"https://schema.org/InStock","url":"https://shop.example/coffee/ethiopia","additionalProperty":{"@type":"PropertyValue","name":"Вес упаковки","value":"250 г"}},{"@type":"Offer","price":"1899","priceCurrency":"RUB","availability":"https://schema.org/InStock","url":"https://shop.example/coffee/ethiopia/1000g","additionalProperty":{"@type":"PropertyValue","name":"Вес упаковки","value":"1000 г"}}]}</script>`))
	}))
	defer server.Close()
	cfg, _ := json.Marshal(Config{SitemapURL: server.URL + "/sitemap.xml", ProductPathPrefixes: []string{"/coffee/"}, DefaultCurrency: "RUB"})
	a, err := New(httpclient.New(httpclient.Config{DefaultRPS: 1000}), "tastycoffee-ru", server.URL, 1000, cfg)
	if err != nil {
		t.Fatal(err)
	}
	offers, err := a.Fetch(context.Background(), entity.ProductRef{URL: server.URL + "/coffee/ethiopia"})
	if err != nil || len(offers) != 2 {
		t.Fatalf("offers=%d err=%v", len(offers), err)
	}
	if offers[0].Currency != "RUB" || offers[0].WeightG == nil || *offers[0].WeightG != 250 || offers[0].Price.String() != "529" {
		t.Fatalf("250g offer=%+v", offers[0])
	}
	if offers[1].WeightG == nil || *offers[1].WeightG != 1000 || offers[1].Price.String() != "1899" {
		t.Fatalf("1000g offer=%+v", offers[1])
	}
}

func TestWeightFromBitrixPathSuffix(t *testing.T) {
	if got := weightFromPath("https://verleshop.com/filter/kenya_komothai_washed_filter_200/"); got == nil || *got != 200 {
		t.Fatalf("200g path = %v", got)
	}
	if got := weightFromPath("https://verleshop.com/filter/kenya_komothai_washed_filter_1000/"); got == nil || *got != 1000 {
		t.Fatalf("1000g path = %v", got)
	}
	if got := weightFromPath("https://verleshop.com/drips/seasonal_mix_24/"); got != nil {
		t.Fatalf("24-piece SKU parsed as grams: %v", got)
	}
}

func TestEmptyEvidenceSelectorRequiresCategoryDiscovery(t *testing.T) {
	config := json.RawMessage(`{"version":1,"sitemap_url":"https://tea.example.test/sitemap.xml","category_empty_selector":".empty-state"}`)
	if err := ValidateConfig("https://tea.example.test", config); err == nil {
		t.Fatal("empty marker on sitemap discovery should be rejected")
	}
}

func TestDecodeConfiguredNonstandardJSONScriptType(t *testing.T) {
	body := []byte(`<script type="application/">{"@type":"Product","name":"Ronnefeldt Camomile tea","category":"Herbal tea","offers":{"@type":"Offer","price":"22.8","priceCurrency":"BYN"}}</script>`)
	if _, err := decodeProducts(body); err == nil {
		t.Fatal("default JSON-LD parser must ignore a nonstandard script type")
	}
	products, err := decodeProductsWithTypes(body, []string{"application/"})
	if err != nil {
		t.Fatal(err)
	}
	if len(products) != 1 || products[0].Name != "Ronnefeldt Camomile tea" || len(products[0].Offers) != 1 || products[0].Offers[0].Price != "22.8" {
		t.Fatalf("products=%+v", products)
	}
}

func TestDecodeSchemaOrgMicrodataProductAndOffer(t *testing.T) {
	body := []byte(`<div itemscope itemtype="https://schema.org/Product"><span itemprop="name">Hyleys Green tea 100 g</span><span itemprop="sku">4025</span><span itemprop="category">Green tea</span><span itemprop="brand" itemscope itemtype="https://schema.org/Brand"><span itemprop="name">Hyleys</span></span><div itemprop="offers" itemscope itemtype="https://schema.org/Offer"><meta itemprop="price" content="17.44"><meta itemprop="priceCurrency" content="BYN"><link itemprop="availability" href="https://schema.org/InStock"></div></div>`)
	products, err := decodeMicrodataProducts(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(products) != 1 || products[0].Name != "Hyleys Green tea 100 g" || products[0].SKU != "4025" || products[0].Brand != "Hyleys" || products[0].Category != "Green tea" || len(products[0].Offers) != 1 {
		t.Fatalf("products=%+v", products)
	}
	offer := products[0].Offers[0]
	if offer.Price != "17.44" || offer.Currency != "BYN" || !strings.Contains(offer.Availability, "InStock") {
		t.Fatalf("offer=%+v", offer)
	}
}

func TestWeightParsingAndVariableAggregatePrice(t *testing.T) {
	if got := weightFromText("sample 1 kg pack"); got == nil || *got != 1000 {
		t.Fatalf("kg weight=%v, want 1000g", got)
	}
	got := decodeOfferNodes(map[string]any{"@type": "AggregateOffer", "lowPrice": "10", "highPrice": "20", "priceCurrency": "BYN"})
	if len(got) != 1 || got[0].Price != "10" {
		t.Fatalf("aggregate low price must become an offer: %+v", got)
	}
	if weight := weightFromText("Чай 25*2 гр"); weight == nil || *weight != 50 {
		t.Fatalf("multipack weight=%v, want 50g", weight)
	}
	if weight := weightFromText("Шу пуэр 2008 г. 357 г"); weight == nil || *weight != 357 {
		t.Fatalf("year was treated as weight: got %v, want 357g", weight)
	}
	if weight := weightFromText("Шу пуэр 2008 г."); weight != nil {
		t.Fatalf("year was treated as weight: got %dg", *weight)
	}
	nested := decodeOfferNodes(map[string]any{"@type": "AggregateOffer", "lowPrice": "10", "offers": []any{map[string]any{"price": "12", "priceCurrency": "BYN"}}})
	if len(nested) != 1 || nested[0].Price != "12" {
		t.Fatalf("nested aggregate offers=%+v", nested)
	}
	offers := decodeOfferNodes(map[string]any{"@type": "Offer", "price": "15.50", "priceCurrency": "BYN", "priceSpecification": map[string]any{"referenceQuantity": map[string]any{"value": float64(100), "unitCode": "GRM", "unitText": "100 g"}}})
	if len(offers) != 1 || offers[0].WeightG == nil || *offers[0].WeightG != 100 {
		t.Fatalf("offer reference quantity weight=%+v", offers)
	}
}

func TestCategoryDiscoveryFiltersFallbackPriceAndHeaders(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tea", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "TeaFindTest/1" || r.Header.Get("Accept-Language") != "be,ru" {
			t.Errorf("headers=%v", r.Header)
		}
		_, _ = w.Write([]byte(`<a href="/product/green">Green</a><a href="/product/coffee">Coffee</a><link rel="next" href="/tea?page=2">`))
	})
	mux.HandleFunc("/product/green", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<meta property="product:price:amount" content="12,50"><script type="application/ld+json">{"@type":"Product","name":"Зелёный чай 25*2 гр","category":"Other"}</script>`))
	})
	mux.HandleFunc("/product/coffee", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<script type="application/ld+json">{"@type":"Product","name":"Coffee","offers":{"price":"20"}}</script>`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	config, _ := json.Marshal(Config{CategoryURLs: []string{server.URL + "/tea"}, ProductPathPrefixes: []string{"/product/"}, ExcludePaths: []string{"/product/coffee"}, IncludeNameKeywords: []string{"чай"}, UserAgent: "TeaFindTest/1", AcceptLanguage: "be,ru"})
	adapter, err := New(httpclient.New(httpclient.Config{DefaultRPS: 1000}), "demo", server.URL, 1000, config)
	if err != nil {
		t.Fatal(err)
	}
	refs, err := adapter.Discover(context.Background())
	if err != nil || len(refs) != 1 {
		t.Fatalf("refs=%+v err=%v", refs, err)
	}
	offers, err := adapter.Fetch(context.Background(), refs[0])
	if err != nil || len(offers) != 1 || offers[0].Price.String() != "12.5" || offers[0].WeightG == nil || *offers[0].WeightG != 50 {
		t.Fatalf("offers=%+v err=%v", offers, err)
	}
}

func TestCategoryDiscoverySkipsCategorySortLinks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<a href="/catalog/tea/?sort=price">Sort</a><a href="/catalog/tea/item-1/">Tea</a>`))
	}))
	defer server.Close()
	config, _ := json.Marshal(Config{CategoryURLs: []string{server.URL + "/catalog/tea/"}, ProductPathPrefixes: []string{"/catalog/tea/"}})
	adapter, err := New(httpclient.New(httpclient.Config{DefaultRPS: 1000}), "demo", server.URL, 1000, config)
	if err != nil {
		t.Fatal(err)
	}
	refs, err := adapter.Discover(context.Background())
	if err != nil || len(refs) != 1 || refs[0].URL != server.URL+"/catalog/tea/item-1/" {
		t.Fatalf("refs=%+v err=%v", refs, err)
	}
}

func TestPagePriceSelectorAndOfferDeduplication(t *testing.T) {
	if got := pagePrice([]byte(`<span class="cost">18,90 BYN</span>`), ".cost"); got != "18.90" {
		t.Fatalf("selector price=%q", got)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<script type="application/ld+json">{"@type":"Product","name":"Чай 100 г","sku":"tea","offers":{"price":"9","priceCurrency":"BYN"}}</script><script type="application/ld+json">{"@type":"Product","name":"Чай 100 г","sku":"tea","offers":{"price":"9","priceCurrency":"BYN"}}</script>`))
	}))
	defer server.Close()
	config, _ := json.Marshal(Config{SitemapURL: server.URL + "/sitemap.xml", ProductPathPrefixes: []string{"/product/"}})
	adapter, err := New(httpclient.New(httpclient.Config{DefaultRPS: 1000}), "demo", server.URL, 1000, config)
	if err != nil {
		t.Fatal(err)
	}
	offers, err := adapter.Fetch(context.Background(), entity.ProductRef{URL: server.URL + "/product/tea"})
	if err != nil || len(offers) != 1 {
		t.Fatalf("offers=%+v err=%v", offers, err)
	}
}

func TestNormalizePriceRejectsAmbiguousText(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"12,50", "12.50"},
		{"1 234,50", "1234.50"},
		{"1&nbsp;234,50", "1234.50"},
		{"18,90 BYN", "18.90"},
		{"18.30 руб.", "18.30"},
		{"18 бел. руб.", "18"},
		{"10 Br", "10"},
		{"1234.5", "1234.5"},
		{"25 30", ""},
		{"от 25 до 30", ""},
		{"25.30.40", ""},
		{"25 RUB", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := normalizePrice(tt.input); got != tt.want {
				t.Fatalf("normalizePrice(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFetchFailsOnAmbiguousPriceInsteadOfCreatingFalseOffer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/product/html-range":
			_, _ = w.Write([]byte(`<h1 id="name">Чай</h1><span class="price">от 25 до 30</span>`))
		case "/product/json-range":
			_, _ = w.Write([]byte(`<script type="application/ld+json">{"@type":"Product","name":"Чай","offers":{"price":"25 30","priceCurrency":"BYN"}}</script>`))
		case "/product/fallback-range":
			_, _ = w.Write([]byte(`<meta property="product:price:amount" content="от 25 до 30"><script type="application/ld+json">{"@type":"Product","name":"Чай"}</script>`))
		case "/product/json-number":
			_, _ = w.Write([]byte(`<script type="application/ld+json">{"@type":"Product","name":"Чай","offers":{"price":12.5,"priceCurrency":"BYN"}}</script>`))
		}
	}))
	defer server.Close()
	cli := httpclient.New(httpclient.Config{DefaultRPS: 1000})
	htmlConfig, _ := json.Marshal(Config{SitemapURL: server.URL + "/sitemap.xml", ProductPathPrefixes: []string{"/product/"}, DataFormats: []string{"html"}, HTMLTitleSelector: "#name", HTMLPriceSelector: ".price"})
	htmlAdapter, err := New(cli, "demo", server.URL, 1000, htmlConfig)
	if err != nil {
		t.Fatal(err)
	}
	jsonConfig, _ := json.Marshal(Config{SitemapURL: server.URL + "/sitemap.xml", ProductPathPrefixes: []string{"/product/"}})
	jsonAdapter, err := New(cli, "demo", server.URL, 1000, jsonConfig)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		adapter *Adapter
		path    string
	}{
		{"html range", htmlAdapter, "/product/html-range"},
		{"json range", jsonAdapter, "/product/json-range"},
		{"fallback range", jsonAdapter, "/product/fallback-range"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			offers, err := tc.adapter.Fetch(context.Background(), entity.ProductRef{URL: server.URL + tc.path})
			var crawlError *crawlerr.CrawlError
			if !errors.As(err, &crawlError) || crawlError.Code != "parse" || len(offers) != 0 {
				t.Fatalf("ambiguous price created offers=%+v, err=%v", offers, err)
			}
		})
	}
	offers, err := jsonAdapter.Fetch(context.Background(), entity.ProductRef{URL: server.URL + "/product/json-number"})
	if err != nil || len(offers) != 1 || offers[0].Price.String() != "12.5" {
		t.Fatalf("numeric JSON-LD price: offers=%+v, err=%v", offers, err)
	}
}

func TestMicrodataAggregateOfferWithNestedPrices(t *testing.T) {
	body := []byte(`<div itemscope itemtype="http://schema.org/Product"><span itemprop="name">Чай Pettiagalla OR</span><div itemprop="offers" itemscope itemtype="http://schema.org/AggregateOffer"><meta itemprop="lowPrice" content="26.00"><meta itemprop="highPrice" content="120.00"><meta itemprop="priceCurrency" content="BYN"><div itemprop="offers" itemscope itemtype="http://schema.org/Offer"><meta itemprop="price" content="26.00"><meta itemprop="priceCurrency" content="BYN"></div><div itemprop="offers" itemscope itemtype="http://schema.org/Offer"><meta itemprop="price" content="60.00"><meta itemprop="priceCurrency" content="BYN"></div><div itemprop="offers" itemscope itemtype="http://schema.org/Offer"><meta itemprop="price" content="120.00"><meta itemprop="priceCurrency" content="BYN"></div></div></div>`)
	products, err := decodeMicrodataProducts(body)
	if err != nil || len(products) != 1 || len(products[0].Offers) != 3 {
		t.Fatalf("products=%+v err=%v", products, err)
	}
	for i, want := range []string{"26.00", "60.00", "120.00"} {
		if products[0].Offers[i].Price != want || products[0].Offers[i].Currency != "BYN" {
			t.Fatalf("offer %d=%+v", i, products[0].Offers[i])
		}
	}
}

func TestValidateConfigRejectsUnsupportedHTMLSelectors(t *testing.T) {
	for _, selector := range []string{"div .price", "[itemprop=price]", "div..price"} {
		cfg, _ := json.Marshal(Config{
			CategoryURLs: []string{"https://shop.example/tea/"},
			DataFormats:  []string{"html"}, HTMLTitleSelector: "#title", HTMLPriceSelector: selector,
		})
		if err := ValidateConfig("https://shop.example", cfg); err == nil {
			t.Fatalf("unsupported selector %q accepted", selector)
		}
	}
	cfg, _ := json.Marshal(Config{
		CategoryURLs: []string{"https://shop.example/tea/"},
		DataFormats:  []string{"html"}, HTMLTitleSelector: "h1.title", HTMLPriceSelector: ".price",
	})
	if err := ValidateConfig("https://shop.example", cfg); err != nil {
		t.Fatalf("supported selectors rejected: %v", err)
	}
}

func TestJSONLDReadsWeightAndAttributesFromVariantOffers(t *testing.T) {
	body := []byte(`<script type="application/ld+json">{"@type":"Product","name":"Чай Каркаде","offers":{"@type":"AggregateOffer","lowPrice":"5.00","highPrice":"19.79","priceCurrency":"BYN","offers":[{"@type":"Offer","name":"Чай Каркаде - 500","url":"https://shop.example/product/chaj-karkade/?attribute_ves-gr=500","price":"19.79","priceCurrency":"BYN"},{"@type":"Offer","name":"Чай Каркаде - 100","url":"https://shop.example/product/chaj-karkade/?attribute_ves-gr=100","price":"5.00","priceCurrency":"BYN"}]}}</script>`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer server.Close()
	config, _ := json.Marshal(Config{Version: 1, SitemapURL: server.URL + "/sitemap.xml", ProductPathPrefixes: []string{"/product/"}})
	adapter, err := New(httpclient.New(httpclient.Config{DefaultRPS: 1000}), "demo", server.URL, 1000, config)
	if err != nil {
		t.Fatal(err)
	}
	offers, err := adapter.Fetch(context.Background(), entity.ProductRef{URL: server.URL + "/product/chaj-karkade/"})
	if err != nil {
		t.Fatal(err)
	}
	if len(offers) != 2 {
		t.Fatalf("offers=%d, want 2", len(offers))
	}
	got := map[int]entity.RawOffer{}
	for _, offer := range offers {
		if offer.WeightG == nil {
			t.Fatalf("offer %q missing weight", offer.Title)
		}
		if offer.Attrs["attribute_ves-gr"] != fmt.Sprintf("%d", *offer.WeightG) {
			t.Fatalf("offer %+v attribute does not match weight", offer)
		}
		got[*offer.WeightG] = offer
	}
	if got[100].Price.String() != "5" || got[500].Price.String() != "19.79" {
		t.Fatalf("prices=%v", got)
	}
}

func TestJSONLDCountAttributeDoesNotBecomeWeight(t *testing.T) {
	body := []byte(`<script type="application/ld+json">{"@type":"Product","name":"Чай в пирамидках","offers":{"@type":"AggregateOffer","priceCurrency":"BYN","offers":[{"@type":"Offer","name":"Чай в пирамидках - 25","url":"https://shop.example/product/tea/?attribute_kolichestvo=25","price":"9.27","priceCurrency":"BYN"},{"@type":"Offer","name":"Чай в пирамидках - 100","url":"https://shop.example/product/tea/?attribute_kolichestvo=100","price":"30.44","priceCurrency":"BYN"}]}}</script>`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer server.Close()
	config, _ := json.Marshal(Config{Version: 1, SitemapURL: server.URL + "/sitemap.xml", ProductPathPrefixes: []string{"/product/"}})
	adapter, err := New(httpclient.New(httpclient.Config{DefaultRPS: 1000}), "demo", server.URL, 1000, config)
	if err != nil {
		t.Fatal(err)
	}
	offers, err := adapter.Fetch(context.Background(), entity.ProductRef{URL: server.URL + "/product/tea/"})
	if err != nil {
		t.Fatal(err)
	}
	if len(offers) != 2 {
		t.Fatalf("offers=%d, want 2", len(offers))
	}
	seen := map[string]bool{}
	for _, offer := range offers {
		if offer.WeightG != nil {
			t.Fatalf("count attribute must not be treated as grams: %+v", offer)
		}
		key := offer.Attrs["attribute_kolichestvo"]
		if key == "" || seen[key] {
			t.Fatalf("distinct count attrs required, offer=%+v", offer)
		}
		seen[key] = true
	}
}

func TestHTMLQuantityOptionsUseVariantWeightNotTitleWeight(t *testing.T) {
	body := []byte(`
		<script type="application/ld+json">{"@type":"Product","name":"Шу пуэр \"Гу Цзюнь Да Сюэ Шань\", 357г., 2009г.","sku":"14640-36327","offers":{"@type":"Offer","price":27,"priceCurrency":"BYN","availability":"https://schema.org/InStock"}}</script>
		<h1 id="name">Шу пуэр "Гу Цзюнь Да Сюэ Шань", 357г., 2009г.</h1>
		<form id="cart-form">
			<input type="hidden" name="product_id" value="14640">
			<select name="quantity" data-id="14640" data-price="0.27">
				<option value="10">10 г.</option>
				<option value="15">15 г.</option>
				<option value="100" selected>100 г.</option>
				<option value="357">357 г.</option>
			</select>
		</form>
		<div class="products-slider-item">
			<h3>Related cake 357г.</h3>
			<select name="quantity" data-id="9999" data-price="0.50">
				<option value="10">10 г.</option>
				<option value="100" selected>100 г.</option>
			</select>
		</div>`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer server.Close()
	config, _ := json.Marshal(Config{
		Version: 1, SitemapURL: server.URL + "/sitemap.xml", ProductPathPrefixes: []string{"/product/"},
		DataFormats: []string{"jsonld", "html"}, DefaultCurrency: "BYN",
		HTMLTitleSelector: "#name", HTMLOptionSelector: "select", HTMLOptionNameAttribute: "name",
		HTMLOptionName: "quantity", HTMLOptionPriceAttribute: "data-price", HTMLOptionValueAttribute: "value",
		HTMLOptionIDAttribute: "data-id", HTMLProductIDSelector: "input", HTMLProductIDNameAttribute: "name",
		HTMLProductIDName: "product_id", HTMLProductIDValueAttribute: "value",
	})
	adapter, err := New(httpclient.New(httpclient.Config{DefaultRPS: 1000}), "teamail", server.URL, 1000, config)
	if err != nil {
		t.Fatal(err)
	}
	offers, err := adapter.Fetch(context.Background(), entity.ProductRef{URL: server.URL + "/product/gutszyun/"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[int]entity.RawOffer{}
	for _, offer := range offers {
		if offer.WeightG == nil {
			t.Fatalf("missing variant weight: %+v", offer)
		}
		if _, exists := got[*offer.WeightG]; exists {
			t.Fatalf("duplicate weight %d from related product: %+v", *offer.WeightG, offers)
		}
		got[*offer.WeightG] = offer
	}
	if len(got) != 4 {
		t.Fatalf("offers=%d, want 4 variants, got %+v", len(got), offers)
	}
	for weight, price := range map[int]string{10: "2.7", 15: "4.05", 100: "27", 357: "96.39"} {
		offer, ok := got[weight]
		if !ok || offer.Price.String() != price {
			t.Errorf("weight %dg: price=%v, want %s", weight, offer.Price, price)
		}
		if offer.WeightGrams == nil || offer.WeightGrams.String() != fmt.Sprintf("%d", weight) {
			t.Errorf("weight %dg missing WeightGrams: %v", weight, offer.WeightGrams)
		}
		if offer.Attrs["attribute_ves-gr"] != fmt.Sprintf("%d", weight) {
			t.Errorf("weight %dg missing structured attr: %v", weight, offer.Attrs)
		}
	}
}

func TestHTMLQuantityOptionsIgnoreRelatedSelectsWithoutProductID(t *testing.T) {
	body := []byte(`<h1 id="name">Чай 357г.</h1>
		<select name="quantity" data-price="0.27"><option value="10">10 г.</option><option value="100">100 г.</option></select>
		<select name="quantity" data-price="0.99"><option value="50">50 г.</option></select>`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer server.Close()
	config, _ := json.Marshal(Config{
		Version: 1, SitemapURL: server.URL + "/sitemap.xml", ProductPathPrefixes: []string{"/product/"},
		DataFormats: []string{"html"}, DefaultCurrency: "BYN", HTMLTitleSelector: "#name",
		HTMLOptionSelector: "select", HTMLOptionNameAttribute: "name", HTMLOptionName: "quantity",
		HTMLOptionPriceAttribute: "data-price", HTMLOptionValueAttribute: "value",
	})
	adapter, err := New(httpclient.New(httpclient.Config{DefaultRPS: 1000}), "teamail", server.URL, 1000, config)
	if err != nil {
		t.Fatal(err)
	}
	offers, err := adapter.Fetch(context.Background(), entity.ProductRef{URL: server.URL + "/product/tea/"})
	if err != nil || len(offers) != 2 {
		t.Fatalf("offers=%+v err=%v, want 2 from the first select only", offers, err)
	}
	got := map[int]string{}
	for _, offer := range offers {
		if offer.WeightG == nil {
			t.Fatalf("missing weight: %+v", offer)
		}
		got[*offer.WeightG] = offer.Price.String()
	}
	if got[10] != "2.7" || got[100] != "27" || got[50] != "" {
		t.Fatalf("related select leaked into offers: %v", got)
	}
}
