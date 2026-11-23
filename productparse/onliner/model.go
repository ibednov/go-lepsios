package onliner

type productSnapshot struct {
	Title    string
	Price    float64
	Currency string
	ImageURL string
}

type apiProductResponse struct {
	Key          string         `json:"key"`
	ExtendedName string         `json:"extended_name"`
	FullName     string         `json:"full_name"`
	Name         string         `json:"name"`
	Images       apiImages      `json:"images"`
	Prices       apiPricesBlock `json:"prices"`
}

type apiImages struct {
	Header string `json:"header"`
}

type apiPricesBlock struct {
	PriceMin apiMoney `json:"price_min"`
}

type apiMoney struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

type jsonLDProduct struct {
	Type   string       `json:"@type"`
	Name   string       `json:"name"`
	Image  jsonLDImage  `json:"image"`
	Offers jsonLDOffers `json:"offers"`
}

type jsonLDOffers struct {
	LowPrice string `json:"lowPrice"`
	Price    string `json:"price"`
}

type jsonLDImage []string

func (i *jsonLDImage) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	if data[0] == '"' {
		var single string
		if err := jsonUnmarshal(data, &single); err != nil {
			return err
		}
		*i = []string{single}
		return nil
	}
	var many []string
	if err := jsonUnmarshal(data, &many); err != nil {
		return err
	}
	*i = many
	return nil
}
