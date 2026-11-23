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
			Version:          1,
			Code:             "shopby",
			Hosts:            []string{"shop.by", "www.shop.by"},
			PathRegex:        `^/([^/]+)/[^/]+/?$`,
			RejectPathGroup1: []string{"news", "cart", "order", "search", "account", "login"},
			DefaultCurrency:  "BYN",
			Title:            []string{"jsonld_product_name", "og_title"},
			Price:            []string{"jsonld_aggregate_low_price", "jsonld_offer_price"},
			Image:            []string{"jsonld_product_image", "og_image"},
			Currency:         []string{"jsonld_price_currency", "default_currency"},
			NoPrice:          "error",
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
		{
			Version: 1, Code: "dns",
			Hosts: []string{
				"dns-shop.by", "www.dns-shop.by",
				"dns-shop.kz", "www.dns-shop.kz",
				"dns-shop.kg", "www.dns-shop.kg",
			},
			PathRegex:          `^/(?:(?:ru|be|kg)/)?product/[0-9a-f]{16}/[^/]+/?$`,
			RejectPathContains: []string{"/opinions", "/service-rating", "/catalog/", "/category/"},
			AcceptLanguage:     "ru",
			UserAgent:          "Mozilla/5.0 (compatible; WishimiParser/1.0)",
			DefaultCurrency:    "BYN",
			CurrencyPathPrefix: map[string]string{"/ru/": "RUB", "/kz/": "KZT"},
			ImageFitUpscale:    "500/500",
			PriceRegexes: []string{
				`product-card-purchase__current-price[^>]*>\s*([\d\s]+(?:[.,]\d+)?)`,
			},
			ImageRegexes: []string{
				`<link rel="preload" href="(https://c\.dns-shop\.ru/[^"]+)" as="image">`,
				`js--product-card-slider__image-main-link="(https://c\.dns-shop\.ru/[^"]+)"`,
				`<meta property="og:image" content="(https://c\.dns-shop\.ru/[^"]+)"`,
			},
			Title:    []string{"jsonld_product_name", "h1_text"},
			Price:    []string{"jsonld_offer_price", "price_regex"},
			Image:    []string{"image_regex", "jsonld_product_image", "og_image"},
			Currency: []string{"jsonld_price_currency", "default_currency"},
			NoPrice:  "error",
		},
		{
			Version: 1, Code: "vek21",
			Hosts:           []string{"21vek.by", "www.21vek.by", "m.21vek.by"},
			PathRegex:       `^/([^/]+)/([^/]+)\.html$`,
			DefaultCurrency: "BYN",
			UserAgent:       "Mozilla/5.0 (compatible; WishimiParser/1.0)",
			// nested: __NEXT_DATA__.props.pageProps.initialState is a JSON string
			NextDataStateStringPath: "props.pageProps.initialState",
			NextDataNamePath:        "productCard.fullProductData.name",
			NextDataPricePaths: []string{
				"productCard.fullProductData.prices.salePrice",
				"productCard.fullProductData.prices.price",
			},
			NextDataImagePath: "productCard.fullProductData.gallery.0.preview",
			Title:             []string{"jsonld_product_name", "next_data_name", "og_title"},
			Price:             []string{"jsonld_offer_price", "next_data_price"},
			Image:             []string{"jsonld_product_image", "next_data_image", "og_image"},
			Currency:          []string{"jsonld_price_currency", "default_currency"},
			NoPrice:           "error",
		},
	}
}
