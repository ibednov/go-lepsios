package dns

type productSnapshot struct {
	Title    string
	Price    float64
	Currency string
	ImageURL string
}

type jsonLDProduct struct {
	Type   string       `json:"@type"`
	Name   string       `json:"name"`
	Image  jsonLDImage  `json:"image"`
	Brand  jsonLDBrand  `json:"brand"`
	Offers jsonLDOffers `json:"offers"`
}

type jsonLDBrand struct {
	Name string `json:"name"`
}

type jsonLDOffers struct {
	Price         float64 `json:"price"`
	PriceCurrency string  `json:"priceCurrency"`
}

type jsonLDGraph struct {
	Graph []jsonLDProduct `json:"@graph"`
}

// jsonLDImage — в разметке бывает строка или массив URL.
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
