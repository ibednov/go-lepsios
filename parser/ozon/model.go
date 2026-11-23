package ozon

type productSnapshot struct {
	Title    string
	Price    float64
	Currency string
	ImageURL string
}

type composerPage struct {
	SEO struct {
		Title  string `json:"title"`
		Script []struct {
			Type      string `json:"type"`
			InnerHTML string `json:"innerHTML"`
		} `json:"script"`
	} `json:"seo"`
	WidgetStates map[string]string `json:"widgetStates"`
}
