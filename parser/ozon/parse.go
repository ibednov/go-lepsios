package ozon

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func parseComposerPage(raw []byte) (productSnapshot, error) {
	var page composerPage
	if err := json.Unmarshal(raw, &page); err != nil {
		return productSnapshot{}, fmt.Errorf("decode composer page: %w", err)
	}

	if len(page.WidgetStates) == 0 && page.SEO.Title == "" {
		return productSnapshot{}, errProductNotFound
	}

	out := productSnapshot{}
	ld, _ := parseJSONLD(page)

	if heading, ok := parseWidget[struct {
		Title string `json:"title"`
	}](page, "webProductHeading"); ok && heading.Title != "" {
		out.Title = heading.Title
	} else if ld != nil && ld.Name != "" {
		out.Title = ld.Name
	} else {
		out.Title = normalizeSEOTitle(page.SEO.Title)
	}

	if wp, ok := parseWidget[struct {
		CardPrice string `json:"cardPrice"`
		Price     string `json:"price"`
	}](page, "webPrice"); ok {
		if price, ok := priceFromLocalizedText(wp.CardPrice); ok {
			out.Price = price
		} else if price, ok := priceFromLocalizedText(wp.Price); ok {
			out.Price = price
		}
		out.Currency = currencyFromPriceText(wp.CardPrice, wp.Price)
	}

	if ws, ok := parseWidget[struct {
		CellTrackingInfo struct {
			Product struct {
				Title      string  `json:"title"`
				FinalPrice float64 `json:"finalPrice"`
				Price      float64 `json:"price"`
			} `json:"product"`
		} `json:"cellTrackingInfo"`
	}](page, "webSale"); ok && out.Price == 0 {
		if ws.CellTrackingInfo.Product.FinalPrice > 0 {
			out.Price = ws.CellTrackingInfo.Product.FinalPrice
		} else if ws.CellTrackingInfo.Product.Price > 0 {
			out.Price = ws.CellTrackingInfo.Product.Price
		}
		if out.Title == "" {
			out.Title = ws.CellTrackingInfo.Product.Title
		}
	}

	if ld != nil {
		if out.Price == 0 && ld.Offers.Price != "" {
			if price, ok := priceFromJSONLD(ld.Offers.Price); ok {
				out.Price = price
			}
		}
		if out.Currency == "" && ld.Offers.PriceCurrency != "" {
			out.Currency = ld.Offers.PriceCurrency
		}
		if out.ImageURL == "" {
			out.ImageURL = ldImage(ld.Image)
		}
	}

	if gallery, ok := parseWidget[struct {
		CoverImage string `json:"coverImage"`
		Images     []struct {
			Src string `json:"src"`
		} `json:"images"`
	}](page, "webGallery"); ok {
		if gallery.CoverImage != "" {
			out.ImageURL = gallery.CoverImage
		} else if len(gallery.Images) > 0 && gallery.Images[0].Src != "" {
			out.ImageURL = gallery.Images[0].Src
		}
	}

	if out.Title == "" || out.Price <= 0 {
		return productSnapshot{}, errProductNotFound
	}
	if out.Currency == "" {
		out.Currency = "BYN"
	}

	return out, nil
}

type jsonLDProduct struct {
	Name   string `json:"name"`
	Image  any    `json:"image"`
	Offers struct {
		Price         string `json:"price"`
		PriceCurrency string `json:"priceCurrency"`
	} `json:"offers"`
}

func parseJSONLD(page composerPage) (*jsonLDProduct, bool) {
	for _, script := range page.SEO.Script {
		if script.Type != "application/ld+json" || script.InnerHTML == "" {
			continue
		}
		var ld jsonLDProduct
		if err := json.Unmarshal([]byte(script.InnerHTML), &ld); err == nil && ld.Name != "" {
			return &ld, true
		}
	}
	return nil, false
}

func parseWidget[T any](page composerPage, widgetName string) (T, bool) {
	var zero T
	for key, raw := range page.WidgetStates {
		if widgetKeyName(key) != widgetName {
			continue
		}
		var value T
		if err := json.Unmarshal([]byte(raw), &value); err == nil {
			return value, true
		}
	}
	return zero, false
}

func widgetKeyName(key string) string {
	if idx := strings.Index(key, "-"); idx >= 0 {
		return key[:idx]
	}
	return key
}

func priceFromLocalizedText(raw string) (float64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}

	cleaned := strings.ReplaceAll(raw, "\u2009", "")
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	for _, suffix := range []string{"₽", "BYN", "б.", "Br", "руб.", "р."} {
		cleaned = strings.TrimSuffix(cleaned, suffix)
	}
	cleaned = strings.ReplaceAll(cleaned, ",", ".")

	price, err := strconv.ParseFloat(cleaned, 64)
	if err != nil || price <= 0 {
		return 0, false
	}
	return price, true
}

func priceFromJSONLD(raw string) (float64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	if strings.Contains(raw, ".") {
		return priceFromLocalizedText(raw)
	}
	if len(raw) > 2 {
		if asFloat, err := strconv.ParseFloat(raw, 64); err == nil && asFloat > 1000 {
			return asFloat, true
		}
	}
	return priceFromLocalizedText(raw)
}

func currencyFromPriceText(values ...string) string {
	for _, value := range values {
		switch {
		case strings.Contains(value, "₽"), strings.Contains(value, "р."), strings.Contains(value, "руб"):
			return "RUB"
		case strings.Contains(value, "BYN"), strings.Contains(value, "б."), strings.Contains(value, "Br"), strings.Contains(value, "₸"):
			if strings.Contains(value, "₸") {
				return "KZT"
			}
			return "BYN"
		}
	}
	return ""
}

func ldImage(raw any) string {
	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value)
	case []any:
		if len(value) == 0 {
			return ""
		}
		if s, ok := value[0].(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func normalizeSEOTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	for _, suffix := range []string{" купить на OZON", " купить c доставкой на OZON", " купить на Ozon"} {
		if idx := strings.Index(title, suffix); idx > 0 {
			return strings.TrimSpace(title[:idx])
		}
	}
	return title
}
