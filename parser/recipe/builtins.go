package recipe

// Builtin — starter html_jsonld shops (no native package).
func Builtin() []Config {
	return []Config{
		{
			Version: 1, Code: "emall",
			Hosts:           []string{"emall.by", "www.emall.by"},
			PathRegex:       `^/product/\d+/?$`,
			DefaultCurrency: "BYN",
			Title:           []string{"jsonld_product_name", "og_title"},
			Price:           []string{"jsonld_offer_price", "jsonld_aggregate_low_price"},
			Image:           []string{"jsonld_product_image", "og_image"},
			Currency:        []string{"jsonld_price_currency", "default_currency"},
			NoPrice:         "error",
		},
		{
			Version: 1, Code: "shopby",
			Hosts:           []string{"shop.by", "www.shop.by"},
			PathRegex:       `^/[^/]+/[^/]+/?$`,
			DefaultCurrency: "BYN",
			Title:           []string{"jsonld_product_name", "og_title"},
			Price:           []string{"jsonld_aggregate_low_price", "jsonld_offer_price"},
			Image:           []string{"jsonld_product_image", "og_image"},
			Currency:        []string{"jsonld_price_currency", "default_currency"},
			NoPrice:         "error",
		},
	}
}
