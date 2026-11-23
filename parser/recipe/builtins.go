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
			PathRegex:       `^/([^/]+)/[^/]+/?$`,
			RejectPathGroup1: []string{"news", "cart", "order", "search", "account", "login"},
			DefaultCurrency: "BYN",
			Title:           []string{"jsonld_product_name", "og_title"},
			Price:           []string{"jsonld_aggregate_low_price", "jsonld_offer_price"},
			Image:           []string{"jsonld_product_image", "og_image"},
			Currency:        []string{"jsonld_price_currency", "default_currency"},
			NoPrice:         "error",
		},
		{
			Version:          1,
			Code:             "ozby",
			Hosts:            []string{"oz.by", "www.oz.by"},
			PathRegex:        `^/([a-z0-9_]+)/more(\d+)\.html$`,
			PathIDGroup:      2,
			RejectPathGroup1: []string{"sseries", "producer", "help", "checkout", "about", "store", "bonus", "partners", "app", "personal"},
			TitleStripSuffixes: []string{" - на OZ.by", " — на OZ.by"},
			DefaultCurrency:    "BYN",
			UserAgent:          "Mozilla/5.0 (compatible; WishimiParser/1.0)",
			Title: []string{
				"microdata_name", "embedded_name_by_id", "ad_product_name", "og_title",
			},
			Price: []string{
				"microdata_price", "embedded_price_by_id", "ad_product_price",
			},
			Image:    []string{"microdata_image", "og_image"},
			Currency: []string{"microdata_currency", "default_currency"},
			NoPrice:  "error",
		},
	}
}
