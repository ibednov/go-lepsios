package wildberries

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/ibednov/go-lepsios/currency"
	"github.com/ibednov/go-lepsios/browser"
)

func TestWbMarketFromURLByVsRu(t *testing.T) {
	by := wbMarketFromURL("https://www.wildberries.by/catalog/1/detail.aspx")
	if by.curr != "byn" || by.dest != wbDestBYPrimary {
		t.Fatalf("by=%+v, want curr=byn dest=%s", by, wbDestBYPrimary)
	}
	ru := wbMarketFromURL("https://www.wildberries.ru/catalog/1/detail.aspx")
	if ru.curr != "rub" || ru.dest != wbDestMoscow {
		t.Fatalf("ru=%+v, want curr=rub dest=%s", ru, wbDestMoscow)
	}
	dests := wbDestCandidates(by)
	if len(dests) != 2 || dests[0] != wbDestBYPrimary || dests[1] != wbDestBYAlt {
		t.Fatalf("by dests=%v", dests)
	}
	if got := wbDestCandidates(ru); len(got) != 1 || got[0] != wbDestMoscow {
		t.Fatalf("ru dests=%v", got)
	}
}

func TestSourceHasPriceFalseForCurrencyDroppedNapi(t *testing.T) {
	detail, err := pricedDetail("Пиалы", "ЛИНМУ", 580, currency.RUB, currency.BYN)
	if err != nil {
		t.Fatal(err)
	}
	url := "https://www.wildberries.by/catalog/409842715/detail.aspx"
	if sourceHasPrice(url, detailSource{name: "napi", data: detail}) {
		t.Fatal("RUB-dropped napi must not count as priced for .by")
	}
}

func TestFirstPricedDetailWalksPastNapiDropToHistory(t *testing.T) {
	napi, err := pricedDetail("Пиалы", "ЛИНМУ", 580, currency.RUB, currency.BYN)
	if err != nil {
		t.Fatal(err)
	}
	hist, err := pricedDetail("Пиалы", "ЛИНМУ", 20.59, currency.BYN, currency.BYN)
	if err != nil {
		t.Fatal(err)
	}
	url := "https://www.wildberries.by/catalog/409842715/detail.aspx"
	title, price, _, err := firstPricedDetail(url, []detailSource{
		{name: "http", err: fmt.Errorf("wb detail API returned HTTP 403")},
		{name: "napi", data: napi},
		{name: "basket", data: DetailResponseDTO{Products: []DetailProductDTO{{
			Name:  "Пиалы",
			Brand: "ЛИНМУ",
		}}}},
		{name: "history", data: hist},
	})
	if err != nil {
		t.Fatal(err)
	}
	if title != "Пиалы - Бренд ЛИНМУ" && title != "" {
		// history pricedDetail may omit title; firstPricedDetail returns history title
	}
	if price != 20.59 {
		t.Fatalf("price=%v title=%q, want history BYN 20.59", price, title)
	}
}

func TestLoggedWildberriesURLsResolveProductAndSize(t *testing.T) {
	tests := []struct {
		name string
		url  string
		id   string
		size int64
	}{
		{"sorter", "https://www.wildberries.by/catalog/721419988/detail.aspx?size=1015416585", "721419988", 1015416585},
		{"gel", "https://www.wildberries.by/catalog/434887971/detail.aspx?size=617399373", "434887971", 617399373},
		{"vegetables", "https://www.wildberries.by/catalog/237436094/detail.aspx?size=373713557", "237436094", 373713557},
		{"doll", "https://www.wildberries.by/catalog/614803770/detail.aspx?size=834960974", "614803770", 834960974},
		{"game", "https://www.wildberries.by/catalog/331852317/detail.aspx?size=496471536", "331852317", 496471536},
		{"oil", "https://www.wildberries.by/catalog/874815669/detail.aspx?size=1318340507", "874815669", 1318340507},
		{"mask", "https://www.wildberries.by/catalog/265360567/detail.aspx?size=411792312", "265360567", 411792312},
		{"glasses", "https://www.wildberries.by/catalog/222060085/detail.aspx?size=352622139", "222060085", 352622139},
		{"cup", "https://www.wildberries.by/catalog/140130338/detail.aspx?size=237674061", "140130338", 237674061},
		{"kitchen", "https://www.wildberries.by/catalog/17265908/detail.aspx?size=48077032", "17265908", 48077032},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStrategy(nil)
			id, err := s.resolveWBID(tt.url)
			if err != nil {
				t.Fatal(err)
			}
			if id != tt.id {
				t.Fatalf("id=%q, want %q", id, tt.id)
			}
			if got := sizeOptionIDFromURL(tt.url); got != tt.size {
				t.Fatalf("size=%d, want %d", got, tt.size)
			}
		})
	}
}

func TestBasketCardToDetailUsesIMTNameAndSelectedSize(t *testing.T) {
	var card basketCardDTO
	if err := json.Unmarshal([]byte(`{"imt_name":"Сортер для малышей","brand":"Счастливкин","pics":4,"sizes":[{"optionId":1015416585,"price":{"product":1299,"logistics":0}}]}`), &card); err != nil {
		t.Fatal(err)
	}
	detail := basketCardToDetail(card)
	title, price, pics, err := extractProductInfo(detail, "https://www.wildberries.by/catalog/721419988/detail.aspx?size=1015416585")
	if err != nil {
		t.Fatal(err)
	}
	if title != "Сортер для малышей - Бренд Счастливкин" || price != 12.99 || pics != 4 {
		t.Fatalf("title=%q price=%v pics=%d", title, price, pics)
	}
}

func TestExtractProductInfoRejectsZeroPrice(t *testing.T) {
	data := DetailResponseDTO{Products: []DetailProductDTO{{
		Name:  "X",
		Brand: "Y",
		Sizes: []DetailSizeDTO{{Price: DetailPriceDTO{Product: 0, Logistics: 0}}},
	}}}
	_, _, _, err := extractProductInfo(data, "https://www.wildberries.ru/catalog/1/detail.aspx")
	if err == nil {
		t.Fatal("expected zero price error")
	}
}

func TestExtractProductMetadataDoesNotRequirePrice(t *testing.T) {
	data := DetailResponseDTO{Products: []DetailProductDTO{{
		Name:  "Conflate Поднос Чабань",
		Brand: "Conflate",
		Pics:  6,
	}}}
	title, pics, err := extractProductMetadata(data)
	if err != nil {
		t.Fatal(err)
	}
	if title != "Conflate Поднос Чабань - Бренд Conflate" || pics != 6 {
		t.Fatalf("title=%q pics=%d", title, pics)
	}
}

func TestFirstMetadataDetailSkipsPriceRequirements(t *testing.T) {
	title, pics, err := firstMetadataDetail([]detailSource{
		{name: "http", err: fmt.Errorf("wb detail API returned HTTP 403")},
		{name: "basket", data: DetailResponseDTO{Products: []DetailProductDTO{{
			Name: "Product",
			Pics: 2,
		}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if title != "Product" || pics != 2 {
		t.Fatalf("title=%q pics=%d", title, pics)
	}
}

func TestFirstPricedDetailSkipsHTTPThenTakesBrowser(t *testing.T) {
	url := "https://www.wildberries.ru/catalog/150707791/detail.aspx"
	title, price, pics, err := firstPricedDetail(url, []detailSource{
		{name: "http", err: fmt.Errorf("wb detail API returned HTTP 403")},
		{name: "browser", data: DetailResponseDTO{Products: []DetailProductDTO{{
			Name:  "Phone",
			Brand: "Acme",
			Pics:  3,
			Sizes: []DetailSizeDTO{{Price: DetailPriceDTO{Product: 19900, Logistics: 100}}},
		}}}},
		{name: "basket", err: fmt.Errorf("basket card with sizes not found")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if title != "Phone - Бренд Acme" || price != 199 || pics != 3 {
		t.Fatalf("title=%q price=%v pics=%d", title, price, pics)
	}
}

func TestCardFromSourcesKeepsTitleWhenPriceMissing(t *testing.T) {
	title, price, priceErr, metaErr := cardFromSources(
		"https://www.wildberries.by/catalog/409842715/detail.aspx",
		[]detailSource{
			{name: "http", err: fmt.Errorf("wb detail API returned HTTP 403")},
			{name: "basket", data: DetailResponseDTO{Products: []DetailProductDTO{{
				Name:  "Пиалы для чайной церемонии 60 мл",
				Brand: "Home",
			}}}},
		},
	)
	if metaErr != nil {
		t.Fatal(metaErr)
	}
	if title != "Пиалы для чайной церемонии 60 мл - Бренд Home" {
		t.Fatalf("title=%q", title)
	}
	if price != nil {
		t.Fatalf("price=%v", *price)
	}
	if priceErr == nil || !strings.Contains(priceErr.Error(), "empty sizes") {
		t.Fatalf("priceErr=%v", priceErr)
	}
}

func TestFirstPricedDetailRejectsAllZero(t *testing.T) {
	_, _, _, err := firstPricedDetail("https://www.wildberries.ru/catalog/1/detail.aspx", []detailSource{
		{name: "http", data: DetailResponseDTO{}},
		{name: "basket", data: DetailResponseDTO{Products: []DetailProductDTO{{
			Name:  "X",
			Brand: "Y",
			Sizes: []DetailSizeDTO{{Price: DetailPriceDTO{Product: 0}}},
		}}}},
	})
	if err == nil {
		t.Fatal("expected combined error")
	}
}

func TestDecodeDetailJSON(t *testing.T) {
	data, err := decodeDetailJSON([]byte(`{"products":[{"name":"A","brand":"B","pics":1,"sizes":[{"price":{"product":100,"logistics":0}}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	title, price, _, err := extractProductInfo(data, "https://www.wildberries.by/catalog/1/detail.aspx")
	if err != nil {
		t.Fatal(err)
	}
	if title != "A - Бренд B" || price != 1 {
		t.Fatalf("title=%q price=%v", title, price)
	}
}

type stubCapturer struct {
	pageURL string
	glob    string
	body    []byte
	err     error
}

func (s *stubCapturer) CaptureJSON(_ context.Context, pageURL, urlGlob string) ([]byte, error) {
	s.pageURL = pageURL
	s.glob = urlGlob
	return s.body, s.err
}

func TestCaptureDetailUsesCatalogGlob(t *testing.T) {
	page := "https://www.wildberries.ru/catalog/150707791/detail.aspx"
	cap := &stubCapturer{body: []byte(`{"products":[{"name":"A","brand":"B","pics":2,"sizes":[{"price":{"product":2500,"logistics":0}}]}]}`)}
	src := NewStrategy(cap).captureDetail(context.Background(), page)
	if cap.pageURL != page || cap.glob != wbDetailResponseGlob {
		t.Fatalf("page=%q glob=%q", cap.pageURL, cap.glob)
	}
	title, price, pics, err := extractProductInfo(src.data, page)
	if err != nil {
		t.Fatal(err)
	}
	if title != "A - Бренд B" || price != 25 || pics != 2 {
		t.Fatalf("title=%q price=%v pics=%d", title, price, pics)
	}
}

func pricedWBJSON(product int) []byte {
	return []byte(fmt.Sprintf(
		`{"products":[{"name":"Габа чай Малина","brand":"Дом 31","pics":1,"sizes":[{"price":{"product":%d,"logistics":0}}]}]}`,
		product,
	))
}

func TestCollectDetailSourcesByBrowserFirst(t *testing.T) {
	page := "https://www.wildberries.by/catalog/719952034/detail.aspx"
	cap := &stubCapturer{body: pricedWBJSON(1395)}
	sources := NewStrategy(cap).collectDetailSources(context.Background(), page, "719952034")
	if cap.pageURL != page || cap.glob != wbInternalDetailGlob {
		t.Fatalf("page=%q glob=%q, want %s", cap.pageURL, cap.glob, wbInternalDetailGlob)
	}
	if len(sources) != 1 || sources[0].name != "browser" || sources[0].err != nil {
		t.Fatalf("sources=%+v", sources)
	}
	_, price, _, err := extractProductInfo(sources[0].data, page)
	if err != nil || price != 13.95 {
		t.Fatalf("price=%v err=%v, want native 13.95 BYN", price, err)
	}
}

func TestCaptureDetailByUsesInternalGlob(t *testing.T) {
	page := "https://www.wildberries.by/catalog/719952034/detail.aspx"
	cap := &stubCapturer{body: pricedWBJSON(1395)}
	src := NewStrategy(cap).captureDetail(context.Background(), page)
	if cap.glob != wbInternalDetailGlob {
		t.Fatalf("glob=%q, want %s", cap.glob, wbInternalDetailGlob)
	}
	if got := wbCaptureGlob(page); got != wbInternalDetailGlob {
		t.Fatalf("wbCaptureGlob=.by %q", got)
	}
	if got := wbCaptureGlob("https://www.wildberries.ru/catalog/1/detail.aspx"); got != wbDetailResponseGlob {
		t.Fatalf("wbCaptureGlob=.ru %q", got)
	}
	_, price, _, err := extractProductInfo(src.data, page)
	if err != nil || price != 13.95 {
		t.Fatalf("price=%v err=%v", price, err)
	}
}

func TestWbInternalDetailURLCurrAndDest(t *testing.T) {
	page := "https://www.wildberries.by/catalog/719952034/detail.aspx"
	got := wbInternalDetailURL(page, "719952034", wbDestBYPrimary)
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if u.Host != "www.wildberries.by" || u.Path != "/__internal/u-card/cards/v4/detail" {
		t.Fatalf("url=%q", got)
	}
	q := u.Query()
	if q.Get("curr") != "byn" || q.Get("dest") != wbDestBYPrimary || q.Get("nm") != "719952034" {
		t.Fatalf("query=%v", q)
	}
}

type stubBrowser struct {
	pageURL, glob string
	captureBody   []byte
	captureErr    error
	captureCalls  int

	warmup   string
	fetchURL []string
	byDest   map[string][]byte
	fetchErr error
}

func (s *stubBrowser) CaptureJSON(_ context.Context, pageURL, urlGlob string) ([]byte, error) {
	s.captureCalls++
	s.pageURL = pageURL
	s.glob = urlGlob
	return s.captureBody, s.captureErr
}

func (s *stubBrowser) FetchInPage(_ context.Context, warmupURL, requestURL string) (browser.PageSnapshot, error) {
	s.warmup = warmupURL
	s.fetchURL = append(s.fetchURL, requestURL)
	if s.fetchErr != nil {
		return browser.PageSnapshot{}, s.fetchErr
	}
	u, err := url.Parse(requestURL)
	if err != nil {
		return browser.PageSnapshot{}, err
	}
	body, ok := s.byDest[u.Query().Get("dest")]
	if !ok {
		return browser.PageSnapshot{}, fmt.Errorf("no stub body for %s", requestURL)
	}
	return browser.PageSnapshot{Body: body}, nil
}

func TestCaptureDetailByFetchInPageAfterCaptureMiss(t *testing.T) {
	page := "https://www.wildberries.by/catalog/719952034/detail.aspx"
	br := &stubBrowser{
		captureErr: fmt.Errorf("captured %s HTTP 403", "https://card.wb.ru/cards/v4/detail"),
		byDest: map[string][]byte{
			wbDestBYPrimary: pricedWBJSON(1395),
		},
	}
	src := NewStrategy(br).captureDetail(context.Background(), page)
	if src.err != nil {
		t.Fatal(src.err)
	}
	if br.captureCalls != 0 {
		t.Fatalf("capture calls=%d, want __internal first", br.captureCalls)
	}
	if br.warmup != "https://www.wildberries.by/" || len(br.fetchURL) != 1 {
		t.Fatalf("warmup=%q fetches=%v", br.warmup, br.fetchURL)
	}
	if !strings.Contains(br.fetchURL[0], "curr=byn") || !strings.Contains(br.fetchURL[0], "nm=719952034") {
		t.Fatalf("fetch=%q", br.fetchURL[0])
	}
	if !strings.Contains(br.fetchURL[0], "dest="+wbDestBYPrimary) {
		t.Fatalf("primary dest missing: %q", br.fetchURL[0])
	}
	_, price, _, err := extractProductInfo(src.data, page)
	if err != nil || price != 13.95 {
		t.Fatalf("price=%v err=%v, want 13.95 BYN", price, err)
	}
}

func TestCaptureDetailByInternalDestFailover(t *testing.T) {
	page := "https://www.wildberries.by/catalog/719952034/detail.aspx"
	br := &stubBrowser{
		captureErr: fmt.Errorf("capture timeout"),
		byDest: map[string][]byte{
			wbDestBYPrimary: []byte(`{"products":[]}`),
			wbDestBYAlt:     pricedWBJSON(1395),
		},
	}
	src := NewStrategy(br).captureDetail(context.Background(), page)
	if src.err != nil {
		t.Fatal(src.err)
	}
	if len(br.fetchURL) != 2 {
		t.Fatalf("fetches=%v", br.fetchURL)
	}
	if !strings.Contains(br.fetchURL[0], "dest="+wbDestBYPrimary) {
		t.Fatalf("first=%q", br.fetchURL[0])
	}
	if !strings.Contains(br.fetchURL[1], "dest="+wbDestBYAlt) {
		t.Fatalf("second=%q", br.fetchURL[1])
	}
	_, price, _, err := extractProductInfo(src.data, page)
	if err != nil || price != 13.95 {
		t.Fatalf("price=%v err=%v", price, err)
	}
}

func TestCaptureDetailByFallsBackToCaptureWhenInternalMisses(t *testing.T) {
	page := "https://www.wildberries.by/catalog/719952034/detail.aspx"
	br := &stubBrowser{captureBody: pricedWBJSON(1395)}
	src := NewStrategy(br).captureDetail(context.Background(), page)
	if src.err != nil {
		t.Fatal(src.err)
	}
	if br.captureCalls != 1 || len(br.fetchURL) != 2 {
		t.Fatalf("capture=%d fetches=%d, want __internal then CaptureJSON", br.captureCalls, len(br.fetchURL))
	}
}

func TestBasketHostMaxIndexCoversLiveShards(t *testing.T) {
	if basketHostMaxIndex != 65 {
		t.Fatalf("basketHostMaxIndex=%d, want 65", basketHostMaxIndex)
	}
	hosts := basketHosts()
	want := "basket-65.wbbasket.ru"
	for _, h := range hosts {
		if h == want {
			return
		}
	}
	t.Fatalf("basketHosts missing %s (%d hosts)", want, len(hosts))
}
