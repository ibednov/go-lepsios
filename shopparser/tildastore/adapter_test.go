package tildastore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	entity "github.com/ibednov/go-lepsios/shopparser"
	"github.com/ibednov/go-lepsios/shopparser/httpclient"
)

func TestValidateConfigRequiresStorepartAndAllowsRUB(t *testing.T) {
	if err := ValidateConfig("https://sintonia.ru/", json.RawMessage(`{"version":1,"currency":"RUB"}`)); err == nil {
		t.Fatal("missing storepart_uids must fail")
	}
	if err := ValidateConfig("https://sintonia.ru/", json.RawMessage(`{"version":1,"storepart_uids":["484700456321"],"currency":"RUB"}`)); err != nil {
		t.Fatalf("valid config: %v", err)
	}
	if err := ValidateConfig("https://sintonia.ru/", json.RawMessage(`{"version":1,"storepart_uids":["484700456321"],"currency":"KZT"}`)); err == nil {
		t.Fatal("KZT must be rejected")
	}
}

func TestDiscoverAndFetchCollapseEditionsByWeight(t *testing.T) {
	payload := `{
		"total":1,"nextslice":0,
		"products":[{
			"uid":208379601182,
			"title":"Кения Перископ",
			"price":"1100.0000",
			"url":"BASE/keniya-periskop",
			"quantity":"",
			"gallery":"[{\"img\":\"https://static.tildacdn.com/img.jpg\"}]",
			"editions":[
				{"uid":1,"price":"1 100.00","quantity":"","pack_m":250,"Вес":"250г","Помол":"пуровер (средний)"},
				{"uid":2,"price":"1 100.00","quantity":"","pack_m":250,"Вес":"250г","Помол":"не молоть"},
				{"uid":3,"price":"3 750.00","quantity":"","pack_m":1000,"Вес":"1000г","Помол":"не молоть"},
				{"uid":4,"price":"3 750.00","quantity":"","pack_m":1000,"Вес":"1 кг","Помол":"эспрессо (мелкий)"}
			]
		}]
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/getproductslist/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(strings.ReplaceAll(payload, "BASE", "http://"+r.Host)))
	}))
	defer server.Close()

	adapter, err := newTestAdapter(server.URL, "sintonia-ru", server.URL, []string{"484700456321"})
	if err != nil {
		t.Fatal(err)
	}
	refs, err := adapter.Discover(context.Background())
	if err != nil || len(refs) != 1 || refs[0].SourceID != "208379601182" {
		t.Fatalf("refs=%+v err=%v", refs, err)
	}
	offers, err := adapter.Fetch(context.Background(), refs[0])
	if err != nil || len(offers) != 2 {
		t.Fatalf("offers=%d err=%v %+v", len(offers), err, offers)
	}
	byWeight := map[int]entity.RawOffer{}
	for _, offer := range offers {
		if offer.WeightG == nil {
			t.Fatalf("missing weight: %+v", offer)
		}
		byWeight[*offer.WeightG] = offer
	}
	if byWeight[250].Price.String() != "1100" || byWeight[250].Currency != "RUB" || byWeight[250].Attrs["tilda_edition_uid"] != "2" {
		t.Fatalf("250g offer=%+v", byWeight[250])
	}
	if byWeight[1000].Price.String() != "3750" {
		t.Fatalf("1000g offer=%+v", byWeight[1000])
	}
}

func TestSilkyDrumStyleKgEditions(t *testing.T) {
	payload := `{"total":1,"products":[{"uid":11,"title":"Кения Мутиту","price":"1154.0000","url":"BASE/catalog/kenya-mutitu-natural","editions":[
		{"uid":21,"price":"1 154.00","вес":"200 г"},
		{"uid":22,"price":"5 131.00","вес":"1 кг"}
	]}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(strings.ReplaceAll(payload, "BASE", "http://"+r.Host)))
	}))
	defer server.Close()
	adapter, err := newTestAdapter(server.URL, "silkydrum-ru", server.URL, []string{"951089447941"})
	if err != nil {
		t.Fatal(err)
	}
	refs, err := adapter.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	offers, err := adapter.Fetch(context.Background(), refs[0])
	if err != nil || len(offers) != 2 {
		t.Fatalf("offers=%d err=%v", len(offers), err)
	}
	weights := map[int]bool{}
	for _, offer := range offers {
		weights[*offer.WeightG] = true
	}
	if !weights[200] || !weights[1000] {
		t.Fatalf("weights=%v", weights)
	}
}

func newTestAdapter(apiBase, code, shopURL string, uids []string) (*Adapter, error) {
	return &Adapter{
		cli:    httpclient.New(httpclient.Config{DefaultRPS: 1000}),
		code:   code,
		base:   mustURL(shopURL),
		config: Config{Version: 1, StorepartUIDs: uids, DefaultCurrency: "RUB", APIBase: apiBase},
		byUID:  make(map[string]storeProduct),
		byURL:  make(map[string]storeProduct),
	}, nil
}

func mustURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}
