package steam

type productSnapshot struct {
	Title    string
	Price    float64
	Currency string
	ImageURL string
}

type appDetailsResponse map[string]appDetailsEntry

type appDetailsEntry struct {
	Success bool            `json:"success"`
	Data    *appDetailsData `json:"data"`
}

type appDetailsData struct {
	Name          string         `json:"name"`
	IsFree        bool           `json:"is_free"`
	HeaderImage   string         `json:"header_image"`
	PriceOverview *priceOverview `json:"price_overview"`
}

type priceOverview struct {
	Currency string `json:"currency"`
	Final    int    `json:"final"`
	Initial  int    `json:"initial"`
}
