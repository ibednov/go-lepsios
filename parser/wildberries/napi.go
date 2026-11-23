package wildberries

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"

	"github.com/ibednov/go-lepsios/currency"
)

func fetchNapiDetail(ctx context.Context, wbID, normalizedURL string) (DetailResponseDTO, error) {
	reqURL := fmt.Sprintf("https://napi.wildberries.ru/catalog/%s/detail.aspx", wbID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return DetailResponseDTO{}, err
	}
	req.Header.Set("User-Agent", wbHTTPUserAgent)
	req.Header.Set("Accept", "text/html")

	resp, err := wbHTTPClient.Do(req)
	if err != nil {
		recordWBHTTP(ctx, "napi", http.MethodGet, reqURL, 0, nil, err)
		return DetailResponseDTO{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512<<10))
	if err != nil {
		recordWBHTTP(ctx, "napi", http.MethodGet, reqURL, resp.StatusCode, nil, err)
		return DetailResponseDTO{}, err
	}
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("wb napi returned HTTP %d", resp.StatusCode)
		recordWBHTTP(ctx, "napi", http.MethodGet, reqURL, resp.StatusCode, body, err)
		return DetailResponseDTO{}, err
	}
	title, brand, price, from, err := parseNapiOfferHTML(body)
	if err != nil {
		recordWBHTTP(ctx, "napi", http.MethodGet, reqURL, resp.StatusCode, body, err)
		return DetailResponseDTO{}, err
	}
	recordWBHTTP(ctx, "napi", http.MethodGet, reqURL, resp.StatusCode, body, nil)
	return pricedDetail(title, brand, price, from, currencyFromURL(normalizedURL))
}

func parseNapiOfferHTML(body []byte) (title, brand string, price float64, curr currency.Code, err error) {
	for _, raw := range ldJSONBlocks(body) {
		title, brand, price, curr, err = parseNapiOfferJSON(raw)
		if err == nil {
			return title, brand, price, curr, nil
		}
	}
	if err == nil {
		err = errors.New("wb napi html has no priced offer")
	}
	return "", "", 0, "", err
}

func ldJSONBlocks(body []byte) [][]byte {
	const open = `<script type="application/ld+json">`
	const close = `</script>`
	var out [][]byte
	rest := body
	for {
		start := bytes.Index(rest, []byte(open))
		if start < 0 {
			return out
		}
		rest = rest[start+len(open):]
		end := bytes.Index(rest, []byte(close))
		if end < 0 {
			return out
		}
		out = append(out, bytes.TrimSpace(rest[:end]))
		rest = rest[end+len(close):]
	}
}

type napiOfferDTO struct {
	Name   string          `json:"name"`
	Brand  json.RawMessage `json:"brand"`
	Offers json.RawMessage `json:"offers"`
}

type napiOfferPriceDTO struct {
	Price         json.Number `json:"price"`
	PriceCurrency string      `json:"priceCurrency"`
	Availability  string      `json:"availability"`
}

func parseNapiOfferJSON(raw []byte) (title, brand string, price float64, curr currency.Code, err error) {
	var last error
	for _, item := range napiOfferItems(raw) {
		title, brand, price, curr, err = offerFromDTO(item)
		if err == nil {
			return title, brand, price, curr, nil
		}
		last = err
	}
	if last == nil {
		last = errors.New("wb napi json has no offer")
	}
	return "", "", 0, "", last
}

func napiOfferItems(raw []byte) []napiOfferDTO {
	var one napiOfferDTO
	if err := json.Unmarshal(raw, &one); err == nil && (len(one.Offers) > 0 || one.Name != "") {
		return []napiOfferDTO{one}
	}

	var nodes []json.RawMessage
	if err := json.Unmarshal(raw, &nodes); err != nil {
		return nil
	}
	out := make([]napiOfferDTO, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, napiOfferItems(node)...)
	}
	return out
}

func offerFromDTO(item napiOfferDTO) (title, brand string, price float64, curr currency.Code, err error) {
	if len(item.Offers) == 0 {
		return "", "", 0, "", errors.New("wb napi offer missing")
	}
	var offer napiOfferPriceDTO
	if err := json.Unmarshal(item.Offers, &offer); err != nil {
		return "", "", 0, "", err
	}
	value, err := offer.Price.Float64()
	if err != nil || value <= 0 {
		return "", "", 0, "", errors.New("wb napi offer has zero price")
	}
	curr = currency.Code(strings.ToUpper(strings.TrimSpace(offer.PriceCurrency)))
	if !curr.IsValid() {
		return "", "", 0, "", fmt.Errorf("wb napi offer has unknown currency %q", offer.PriceCurrency)
	}
	return item.Name, brandName(item.Brand), value, curr, nil
}

func brandName(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var asString string
	if json.Unmarshal(raw, &asString) == nil {
		return asString
	}
	var asObj struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(raw, &asObj) == nil {
		return asObj.Name
	}
	return ""
}

// pricedDetail keeps the title but drops a price in another currency: WB prices each
// market separately (regional logistics, discounts), so RUB × bank rate is not the BY price.
func pricedDetail(title, brand string, price float64, from, want currency.Code) (DetailResponseDTO, error) {
	if from != want {
		return DetailResponseDTO{Products: []DetailProductDTO{{Name: title, Brand: brand}}}, nil
	}
	kopecks := int(math.Round(price * 100))
	if kopecks <= 0 {
		return DetailResponseDTO{}, errors.New("wb fallback price is zero")
	}
	return DetailResponseDTO{Products: []DetailProductDTO{{
		Name:  title,
		Brand: brand,
		Sizes: []DetailSizeDTO{{Price: DetailPriceDTO{Product: kopecks}}},
	}}}, nil
}

func fetchHistoryDetail(
	ctx context.Context,
	host string,
	vol, part, nmID int,
	normalizedURL string,
) (DetailResponseDTO, error) {
	if host == "" {
		return DetailResponseDTO{}, errors.New("wb basket host unknown for price history")
	}
	reqURL := fmt.Sprintf("https://%s/vol%d/part%d/%d/info/price-history.json", host, vol, part, nmID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return DetailResponseDTO{}, err
	}
	req.Header.Set("User-Agent", wbHTTPUserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := wbHTTPClient.Do(req)
	if err != nil {
		recordWBHTTP(ctx, "history", http.MethodGet, reqURL, 0, nil, err)
		return DetailResponseDTO{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	if err != nil {
		recordWBHTTP(ctx, "history", http.MethodGet, reqURL, resp.StatusCode, nil, err)
		return DetailResponseDTO{}, err
	}
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("wb price history returned HTTP %d", resp.StatusCode)
		recordWBHTTP(ctx, "history", http.MethodGet, reqURL, resp.StatusCode, body, err)
		return DetailResponseDTO{}, err
	}
	var points []struct {
		Price map[string]int `json:"price"`
	}
	if err := json.Unmarshal(body, &points); err != nil {
		recordWBHTTP(ctx, "history", http.MethodGet, reqURL, resp.StatusCode, body, err)
		return DetailResponseDTO{}, err
	}
	recordWBHTTP(ctx, "history", http.MethodGet, reqURL, resp.StatusCode, body, nil)
	if len(points) == 0 {
		return DetailResponseDTO{}, errors.New("wb price history is empty")
	}
	want := currencyFromURL(normalizedURL)
	raw, from, ok := historyPrice(points[len(points)-1].Price, want)
	if !ok {
		return DetailResponseDTO{}, errors.New("wb price history has no usable currency")
	}
	return pricedDetail("", "", float64(raw)/100, from, want)
}

func historyPrice(values map[string]int, prefer currency.Code) (int, currency.Code, bool) {
	if values == nil {
		return 0, "", false
	}
	if v := values[string(prefer)]; v > 0 {
		return v, prefer, true
	}
	if v := values[string(currency.RUB)]; v > 0 {
		return v, currency.RUB, true
	}
	for code, v := range values {
		curr := currency.Code(strings.ToUpper(code))
		if v > 0 && curr.IsValid() {
			return v, curr, true
		}
	}
	return 0, "", false
}
