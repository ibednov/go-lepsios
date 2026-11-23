package detmir

type productSnapshot struct {
	Title    string
	Price    float64
	Currency string
	ImageURL string
}

type apiProductResponse struct {
	Item apiProductItem `json:"item"`
}

type apiProductItem struct {
	Title    string       `json:"title"`
	Price    apiMoney     `json:"price"`
	Prices   apiPrices    `json:"prices"`
	Pictures []apiPicture `json:"pictures"`
	Brands   []apiBrand   `json:"brands"`
}

type apiMoney struct {
	Price float64 `json:"price"`
}

type apiPrices struct {
	Sale float64 `json:"sale"`
}

type apiPicture struct {
	Web      string `json:"web"`
	Original string `json:"original"`
}

type apiBrand struct {
	Title string `json:"title"`
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
	Price float64 `json:"price"`
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
