package woocommerce

import (
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

	entity "github.com/ibednov/go-lepsios/shopparser"
	crawlerr "github.com/ibednov/go-lepsios/shopparser/errors"
	"github.com/ibednov/go-lepsios/shopparser/httpclient"
)

func TestOneConfiguredStrategyServesMultipleSourceCodes(t *testing.T) {
	productsPage := readFixture(t, "simple_products_page.json")
	product := readFixture(t, "product_simple.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/wp-json/wc/store/v1/products":
			w.Header().Set("X-WP-Total", "4")
			_, _ = w.Write(withTestOrigin(productsPage, "http://"+r.Host))
		case "/wp-json/wc/store/v1/products/71":
			_, _ = w.Write(withTestOrigin(product, "http://"+r.Host))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	teaCfg := json.RawMessage(`{"version":1,"include_category_keywords":["чай","tea"],"exclude_category_keywords":["посуд","картин","интерьер","аксессуар"]}`)
	for _, code := range []string{"shop-a", "shop-b", "shop-c", "shop-d"} {
		adapter, err := New(httpclient.New(httpclient.Config{DefaultRPS: 100}), code, server.URL, teaCfg)
		if err != nil {
			t.Fatal(err)
		}
		refs, err := adapter.Discover(context.Background())
		if err != nil || len(refs) != 1 || refs[0].SourceCode != code || refs[0].SourceID != "71" {
			t.Fatalf("%s refs=%+v err=%v", code, refs, err)
		}
		offers, err := adapter.Fetch(context.Background(), refs[0])
		if err != nil || len(offers) != 1 {
			t.Fatalf("%s offers=%+v err=%v", code, offers, err)
		}
		if offers[0].Source != code || offers[0].Price.String() != "12.5" || offers[0].Currency != "BYN" || offers[0].WeightG == nil || *offers[0].WeightG != 100 {
			t.Fatalf("%s offer=%+v", code, offers[0])
		}
		_, err = adapter.Fetch(context.Background(), entity.ProductRef{URL: refs[0].URL, SourceID: "99"})
		if err == nil {
			t.Fatal("unknown product ID should fail")
		}
		diagnostic, ok := adapter.LastDiagnosticResponse()
		if !ok || diagnostic.StatusCode != http.StatusNotFound || diagnostic.BodyHash == "" || diagnostic.BytesRead == 0 {
			t.Fatalf("%s diagnostic=%+v available=%v", code, diagnostic, ok)
		}
	}
}

func TestConfiguredCategoryFiltersApplyThroughSharedWrapper(t *testing.T) {
	productsPage := readFixture(t, "simple_products_page.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wp-json/wc/store/v1/products" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-WP-Total", "4")
		_, _ = w.Write(withTestOrigin(productsPage, "http://"+r.Host))
	}))
	defer server.Close()

	for _, code := range []string{"shop-a", "shop-b", "shop-c", "shop-d"} {
		for _, tc := range []struct {
			name string
			cfg  string
			want []string
		}{
			{name: "include tea keywords", cfg: `{"version":1,"include_category_keywords":["чай","tea"],"exclude_category_keywords":["посуд","картин","интерьер","аксессуар"]}`, want: []string{"71"}},
			{name: "keyword include", cfg: `{"version":1,"include_category_keywords":["herb"]}`, want: []string{"73"}},
			{name: "category ID include", cfg: `{"version":1,"include_category_ids":[12]}`, want: []string{"73"}},
			{name: "empty filter keeps all", cfg: `{"version":1}`, want: []string{"71", "72", "73", "74"}},
		} {
			t.Run(code+"/"+tc.name, func(t *testing.T) {
				adapter, err := New(httpclient.New(httpclient.Config{DefaultRPS: 100}), code, server.URL, json.RawMessage(tc.cfg))
				if err != nil {
					t.Fatal(err)
				}
				refs, err := adapter.Discover(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				got := make([]string, 0, len(refs))
				for _, ref := range refs {
					if ref.SourceCode != code {
						t.Errorf("ref source=%q want %q", ref.SourceCode, code)
					}
					got = append(got, ref.SourceID)
				}
				if strings.Join(got, ",") != strings.Join(tc.want, ",") {
					t.Fatalf("refs IDs=%v want %v", got, tc.want)
				}
			})
		}
	}
}

func TestVariableProductVariantsPreserveConfiguredSource(t *testing.T) {
	productsPage := readFixture(t, "variable_products_page.json")
	product := readFixture(t, "product_variable.json")
	productPage := readFixture(t, "product_variable.html")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/wp-json/wc/store/v1/products":
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-WP-Total", "1")
			_, _ = w.Write(withTestOrigin(productsPage, "http://"+r.Host))
		case "/wp-json/wc/store/v1/products/1003":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(withTestOrigin(product, "http://"+r.Host))
		case "/product/da-hong-pao-badurov/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(productPage)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	want := map[string]struct {
		price   string
		weight  *int
		inStock bool
	}{
		"badu-25":     {price: "22", weight: intPointer(25), inStock: true},
		"badu-50":     {price: "44", weight: intPointer(50), inStock: true},
		"badu-100":    {price: "88", weight: intPointer(100), inStock: true},
		"badu-absent": {price: "0", inStock: false},
	}
	for _, code := range []string{"shop-a", "shop-b", "shop-c", "shop-d"} {
		adapter, err := New(httpclient.New(httpclient.Config{DefaultRPS: 100}), code, server.URL, json.RawMessage(`{"version":1}`))
		if err != nil {
			t.Fatal(err)
		}
		refs, err := adapter.Discover(context.Background())
		if err != nil || len(refs) != 1 || refs[0].SourceID != "1003" || refs[0].SourceCode != code {
			t.Fatalf("%s refs=%+v err=%v", code, refs, err)
		}
		offers, err := adapter.Fetch(context.Background(), refs[0])
		if err != nil || len(offers) != len(want) {
			t.Fatalf("%s offers=%+v err=%v", code, offers, err)
		}
		seen := make(map[string]bool, len(offers))
		for _, offer := range offers {
			expected, ok := want[offer.SourceSKU]
			if !ok {
				t.Errorf("%s unexpected variant %q", code, offer.SourceSKU)
				continue
			}
			if seen[offer.SourceSKU] {
				t.Errorf("%s duplicate variant %q", code, offer.SourceSKU)
			}
			seen[offer.SourceSKU] = true
			if offer.Source != code || offer.Attrs["adapter"] != Kind || offer.URL != server.URL+"/product/da-hong-pao-badurov/" || offer.Price.String() != expected.price || offer.Currency != "BYN" || offer.InStock != expected.inStock {
				t.Errorf("%s variant=%+v want price=%s stock=%v", code, offer, expected.price, expected.inStock)
			}
			if (offer.WeightG == nil) != (expected.weight == nil) || (offer.WeightG != nil && *offer.WeightG != *expected.weight) {
				t.Errorf("%s variant=%s weight=%v want %v", code, offer.SourceSKU, offer.WeightG, expected.weight)
			}
		}
		for sku := range want {
			if !seen[sku] {
				t.Errorf("%s missing variant %q", code, sku)
			}
		}
	}
}

func TestGarbataVariationSlugsUseOptionGramsNotTitleCake(t *testing.T) {
	productsPage := []byte(`[{"id":760,"name":"Шу Пуэр ‘7572’ 2009, 357г, фаб. ДаИ","permalink":"https://shop.example/shop/puerh-tea/shu-puer-chyornyj/dai-7572/","type":"variable","sku":"514","prices":{"price":"50","currency_code":"BYR","currency_minor_unit":1},"categories":[{"id":57,"name":"Китайский чай","slug":"shu-puer-chyornyj"}],"is_in_stock":true}]`)
	product := []byte(`{"id":760,"name":"Шу Пуэр ‘7572’ 2009, 357г, фаб. ДаИ","permalink":"https://shop.example/shop/puerh-tea/shu-puer-chyornyj/dai-7572/","type":"variable","sku":"514","prices":{"price":"50","currency_code":"BYR","currency_minor_unit":1},"categories":[{"id":57,"name":"Китайский чай","slug":"shu-puer-chyornyj"}],"is_in_stock":true,"variations":[{"id":1675}]}`)
	productPage := []byte(`<form class="variations_form" data-product_variations="[{&quot;attributes&quot;:{&quot;attribute_pa_ves&quot;:&quot;25-gr&quot;},&quot;display_price&quot;:5,&quot;is_in_stock&quot;:true,&quot;sku&quot;:&quot;514&quot;,&quot;variation_id&quot;:1675,&quot;weight&quot;:&quot;&quot;},{&quot;attributes&quot;:{&quot;attribute_pa_ves&quot;:&quot;50-gr&quot;},&quot;display_price&quot;:10,&quot;is_in_stock&quot;:true,&quot;sku&quot;:&quot;514&quot;,&quot;variation_id&quot;:1676,&quot;weight&quot;:&quot;&quot;},{&quot;attributes&quot;:{&quot;attribute_pa_ves&quot;:&quot;100-gr&quot;},&quot;display_price&quot;:20,&quot;is_in_stock&quot;:true,&quot;sku&quot;:&quot;514&quot;,&quot;variation_id&quot;:1677,&quot;weight&quot;:&quot;&quot;},{&quot;attributes&quot;:{&quot;attribute_pa_ves&quot;:&quot;357-gr&quot;},&quot;display_price&quot;:68,&quot;is_in_stock&quot;:true,&quot;sku&quot;:&quot;514&quot;,&quot;variation_id&quot;:1678,&quot;weight&quot;:&quot;&quot;}]"></form>`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/wp-json/wc/store/v1/products":
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-WP-Total", "1")
			_, _ = w.Write(withTestOrigin(productsPage, "http://"+r.Host))
		case "/wp-json/wc/store/v1/products/760":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(withTestOrigin(product, "http://"+r.Host))
		case "/shop/puerh-tea/shu-puer-chyornyj/dai-7572/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(productPage)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	adapter, err := New(httpclient.New(httpclient.Config{DefaultRPS: 100}), "shop-a", server.URL, json.RawMessage(`{"version":1,"include_category_keywords":["чай","tea"]}`))
	if err != nil {
		t.Fatal(err)
	}
	refs, err := adapter.Discover(context.Background())
	if err != nil || len(refs) != 1 {
		t.Fatalf("refs=%+v err=%v", refs, err)
	}
	offers, err := adapter.Fetch(context.Background(), refs[0])
	if err != nil {
		t.Fatal(err)
	}
	got := map[int]string{}
	for _, offer := range offers {
		if offer.WeightG == nil {
			t.Fatalf("missing variant weight: %+v", offer)
		}
		if offer.WeightGrams == nil || offer.WeightGrams.String() != fmt.Sprintf("%d", *offer.WeightG) {
			t.Fatalf("missing WeightGrams: %+v", offer)
		}
		got[*offer.WeightG] = offer.Price.String()
	}
	want := map[int]string{25: "5", 50: "10", 100: "20", 357: "68"}
	if len(got) != len(want) {
		t.Fatalf("offers=%v want %v", got, want)
	}
	for weight, price := range want {
		if got[weight] != price {
			t.Errorf("weight %dg price=%s, want %s", weight, got[weight], price)
		}
	}
}

func TestMalformedPriceFailurePreservesConfiguredSourceAndDiagnostic(t *testing.T) {
	products := map[string][]byte{
		"81": readFixture(t, "product_missing_price.json"),
		"82": readFixture(t, "product_malformed_price.json"),
		"83": readFixture(t, "product_changed_schema.json"),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/wp-json/wc/store/v1/products/")
		product, ok := products[id]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(product)
	}))
	defer server.Close()

	for _, code := range []string{"shop-a", "shop-b", "shop-c", "shop-d"} {
		for id := range products {
			adapter, err := New(httpclient.New(httpclient.Config{DefaultRPS: 100}), code, server.URL, json.RawMessage(`{"version":1}`))
			if err != nil {
				t.Fatal(err)
			}
			offers, err := adapter.Fetch(context.Background(), entity.ProductRef{SourceCode: code, SourceID: id})
			var crawlError *crawlerr.CrawlError
			if len(offers) != 0 || !errors.As(err, &crawlError) || crawlError.Code != "parse" || crawlError.Source != code || errors.Is(err, crawlerr.ErrEmptyOffers) || errors.Is(err, crawlerr.ErrSkippedProduct) {
				t.Fatalf("%s product=%s offers=%+v err=%v, want source-attributed parse failure", code, id, offers, err)
			}
			diagnostic, ok := adapter.LastDiagnosticResponse()
			if !ok || diagnostic.StatusCode != http.StatusOK || diagnostic.BodyHash == "" || diagnostic.BytesRead == 0 {
				t.Fatalf("%s product=%s diagnostic=%+v available=%v", code, id, diagnostic, ok)
			}
		}
	}
}

func TestEmptyStoreAPIListFailureUsesConfiguredSource(t *testing.T) {
	empty := readFixture(t, "products_empty.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wp-json/wc/store/v1/products" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-WP-Total", "0")
		_, _ = w.Write(empty)
	}))
	defer server.Close()

	for _, code := range []string{"shop-a", "shop-b", "shop-c", "shop-d"} {
		adapter, err := New(httpclient.New(httpclient.Config{DefaultRPS: 100}), code, server.URL, json.RawMessage(`{"version":1}`))
		if err != nil {
			t.Fatal(err)
		}
		refs, err := adapter.Discover(context.Background())
		var crawlError *crawlerr.CrawlError
		if len(refs) != 0 || !errors.As(err, &crawlError) || crawlError.Code != "parse" || crawlError.Source != code {
			t.Fatalf("%s refs=%+v err=%v, want source-attributed parse failure", code, refs, err)
		}
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "testdata", "woocommerce", name))
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func withTestOrigin(body []byte, origin string) []byte {
	return []byte(strings.ReplaceAll(string(body), "https://shop.example", origin))
}

func intPointer(v int) *int { return &v }

func TestValidateConfigRejectsUnsupportedVersionAndIDs(t *testing.T) {
	for _, raw := range []string{`{"version":2}`, `{"include_category_ids":[0]}`, `{"include_category_keywords":[""]}`} {
		if err := ValidateConfig("https://shop.example", json.RawMessage(raw)); err == nil {
			t.Fatalf("invalid config accepted: %s", raw)
		}
	}
	for _, baseURL := range []string{"https://shop.example/?token=secret", "https://shop.example/#private"} {
		if err := ValidateConfig(baseURL, json.RawMessage(`{"version":1}`)); err == nil {
			t.Fatalf("source base URL with query or fragment accepted: %q", baseURL)
		}
	}
}
