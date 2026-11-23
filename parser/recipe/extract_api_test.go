package recipe

import "testing"

func TestExtractAPISteamShape(t *testing.T) {
	body := `{"570":{"success":true,"data":{"name":"Dota 2","is_free":true,"header_image":"https://cdn.steam/h.jpg","price_overview":null}}}`
	cfg := Config{
		Code: "steam", Hosts: []string{"store.steampowered.com"},
		APIRequireTrue:  "{id}.success",
		APIFreePath:     "{id}.data.is_free",
		APITitlePaths:   []string{"{id}.data.name"},
		APIPricePaths:   []string{"{id}.data.price_overview.final"},
		APICurrencyPaths: []string{"{id}.data.price_overview.currency"},
		APIImagePaths:   []string{"{id}.data.header_image"},
		APIPriceDivideBy: 100,
		DefaultCurrency: "USD",
	}
	s := ExtractAPI(body, cfg, "570")
	if s.Title != "Dota 2" || !s.HasPrice || s.Price != 0 {
		t.Fatalf("%+v", s)
	}
	if s.ImageURL != "https://cdn.steam/h.jpg" {
		t.Fatalf("img %q", s.ImageURL)
	}
}

func TestExtractAPISteamPaid(t *testing.T) {
	body := `{"1":{"success":true,"data":{"name":"Paid","is_free":false,"header_image":"https://x","price_overview":{"currency":"USD","final":1999}}}}`
	cfg := Config{
		APIRequireTrue: "{id}.success", APIFreePath: "{id}.data.is_free",
		APITitlePaths: []string{"{id}.data.name"},
		APIPricePaths: []string{"{id}.data.price_overview.final"},
		APICurrencyPaths: []string{"{id}.data.price_overview.currency"},
		APIImagePaths: []string{"{id}.data.header_image"},
		APIPriceDivideBy: 100, DefaultCurrency: "USD",
		Code: "s", Hosts: []string{"x"},
	}
	s := ExtractAPI(body, cfg, "1")
	if s.Title != "Paid" || s.Price != 19.99 || s.Currency != "USD" {
		t.Fatalf("%+v", s)
	}
}

func TestExtractAPIOnliner(t *testing.T) {
	body := `{"extended_name":"Phone X","full_name":"","name":"x","images":{"header":"https://img/h.jpg"},"prices":{"price_min":{"amount":"123.45","currency":"BYN"}}}`
	cfg := Config{
		Code: "onliner", Hosts: []string{"catalog.onliner.by"},
		APITitlePaths: []string{"extended_name", "full_name", "name"},
		APIPricePaths: []string{"prices.price_min.amount"},
		APICurrencyPaths: []string{"prices.price_min.currency"},
		APIImagePaths: []string{"images.header"},
		DefaultCurrency: "BYN",
	}
	s := ExtractAPI(body, cfg, "key")
	if s.Title != "Phone X" || s.Price != 123.45 || s.ImageURL == "" {
		t.Fatalf("%+v", s)
	}
}

func TestExtractAPIDetmir(t *testing.T) {
	body := `{"item":{"title":"Toy","price":{"price":10.5},"prices":{"sale":0},"pictures":[{"web":"https://p/w.jpg","original":"https://p/o.jpg"}],"brands":[{"title":"Brand"}]}}`
	cfg := Config{
		Code: "detmir", Hosts: []string{"detmir.by"},
		APITitlePaths: []string{"item.title"},
		APIPricePaths: []string{"item.price.price", "item.prices.sale"},
		APIImagePaths: []string{"item.pictures.0.web", "item.pictures.0.original"},
		DefaultCurrency: "BYN",
	}
	s := ExtractAPI(body, cfg, "1")
	if s.Title != "Toy" || s.Price != 10.5 || s.ImageURL != "https://p/w.jpg" {
		t.Fatalf("%+v", s)
	}
}
