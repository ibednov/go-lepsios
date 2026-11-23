package wildberries

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CDN и basket часто отвечают дольше 1-2s; короткий таймаут даёт ложные промахи.
var wbHTTPClient = &http.Client{
	Timeout: 8 * time.Second,
}

const wbHTTPUserAgent = "Mozilla/5.0 (compatible; ProductParser/1.0)"

// Шаблоны путей превью на basket (часть артикулов не кладёт файлы только в images/big).
var basketImagePathPatterns = []string{
	"https://%s/vol%d/part%d/%d/images/big/%d.%s",
	"https://%s/vol%d/part%d/%d/images/hq/%d.%s",
	"https://%s/vol%d/part%d/%d/images/c246x328/%d.%s",
	"https://%s/vol%d/part%d/%d/images/c516x688/%d.%s",
}

var basketHostPatterns = []string{
	"basket-%02d.wbbasket.ru",
	"mns-basket-cdn-%02d.geobasket.net",
}

// WB периодически добавляет новые basket-кластеры (например, basket-24),
// поэтому верхняя граница должна быть с запасом.
const basketHostMaxIndex = 65 // research 2026-10-26: live map ~65; nm>~960M need higher shards

// Delivery region ids for card.wb.ru (prices/stock depend on dest).
const (
	// Documented public Moscow default used by WB .ru catalog/search examples.
	wbDestMoscow = "-1257786"
	// Historical .by dest (hardcoded with curr=byn since commit 9bac928).
	wbDestBYPrimary = "-59199"
	// Provisional .by failover when primary returns non-OK / unusable price.
	// Observed as destinations[0] from user-geo-data with currency=BYN (2026-10-26);
	// not Minsk-confirmed — replace after capturing dest= from wildberries.by catalog XHR.
	wbDestBYAlt = "-364110"
)

type wbMarketParams struct {
	curr string
	dest string
}

func wbMarketFromURL(normalizedURL string) wbMarketParams {
	u, err := url.Parse(normalizedURL)
	if err != nil {
		return wbMarketParams{curr: "byn", dest: wbDestBYPrimary}
	}
	host := strings.ToLower(u.Hostname())
	if strings.HasSuffix(host, ".ru") {
		return wbMarketParams{curr: "rub", dest: wbDestMoscow}
	}
	return wbMarketParams{curr: "byn", dest: wbDestBYPrimary}
}

// wbDestCandidates: .by tries primary then alt; .ru uses Moscow only.
func wbDestCandidates(market wbMarketParams) []string {
	if market.curr == "byn" {
		return []string{wbDestBYPrimary, wbDestBYAlt}
	}
	return []string{market.dest}
}

func (s *Strategy) fetchWBProductMetadata(
	ctx context.Context,
	normalizedURL string,
	wbID string,
) (title string, pics int, err error) {
	return firstMetadataDetail(collectMetadataSources(ctx, normalizedURL, wbID))
}

// collectMetadataSources skips the browser: title and pics do not need the BY price.
func collectMetadataSources(ctx context.Context, normalizedURL, wbID string) []detailSource {
	httpData, httpErr := fetchDetail(ctx, normalizedURL, wbID)
	sources := []detailSource{{name: "http", data: httpData, err: httpErr}}
	if _, _, err := firstMetadataDetail(sources); err == nil {
		return sources
	}
	basketData, _, basketErr := fetchBasketBest(ctx, wbID)
	return append(sources, detailSource{name: "basket", data: basketData, err: basketErr})
}

// collectDetailSources order: .by browser → card.wb.ru → .ru browser → napi → basket → history.
// Early-stop only when sourceHasPrice is true. Currency-dropped napi (RUB offer on
// .by keeps title, empty sizes) does not stop the chain — basket/history still run.
func (s *Strategy) collectDetailSources(
	ctx context.Context,
	normalizedURL string,
	wbID string,
) []detailSource {
	sources := make([]detailSource, 0, 5)
	market := wbMarketFromURL(normalizedURL)

	// .by: DC IPs get 403 on card.wb.ru. Browser first: CaptureJSON of __internal
	// (page cookies / curr=byn), then FetchInPage of the same URL if the XHR never fired.
	if market.curr == "byn" && s != nil && s.capturer != nil {
		src := s.captureDetail(ctx, normalizedURL)
		sources = append(sources, src)
		if sourceHasPrice(normalizedURL, src) {
			return sources
		}
	}

	httpData, httpErr := fetchDetail(ctx, normalizedURL, wbID)
	sources = append(sources, detailSource{name: "http", data: httpData, err: httpErr})
	if sourceHasPrice(normalizedURL, sources[len(sources)-1]) {
		return sources
	}

	if market.curr != "byn" && s != nil && s.capturer != nil {
		src := s.captureDetail(ctx, normalizedURL)
		sources = append(sources, src)
		if sourceHasPrice(normalizedURL, src) {
			return sources
		}
	}

	napiData, napiErr := fetchNapiDetail(ctx, wbID, normalizedURL)
	sources = append(sources, detailSource{name: "napi", data: napiData, err: napiErr})
	if sourceHasPrice(normalizedURL, sources[len(sources)-1]) {
		return sources
	}

	basketData, host, basketErr := fetchBasketBest(ctx, wbID)
	sources = append(sources, detailSource{name: "basket", data: basketData, err: basketErr})
	if sourceHasPrice(normalizedURL, sources[len(sources)-1]) {
		return sources
	}

	nmID, convErr := strconv.Atoi(wbID)
	if convErr != nil || host == "" {
		return sources
	}
	histData, histErr := fetchHistoryDetail(ctx, host, nmID/100000, nmID/1000, nmID, normalizedURL)
	return append(sources, detailSource{name: "history", data: histData, err: histErr})
}

func sourceHasPrice(normalizedURL string, src detailSource) bool {
	if src.err != nil {
		return false
	}
	_, _, _, err := extractProductInfo(src.data, normalizedURL)
	return err == nil
}

func cardFromSources(
	normalizedURL string,
	sources []detailSource,
) (title string, price *float64, priceErr, metadataErr error) {
	title, _, metadataErr = firstMetadataDetail(sources)
	if metadataErr != nil {
		return "", nil, nil, metadataErr
	}
	_, value, _, err := firstPricedDetail(normalizedURL, sources)
	if err != nil {
		return title, nil, err, nil
	}
	return title, &value, nil, nil
}

const (
	// Public card.wb.ru/u-card XHR. First match from a DC IP is often HTTP 403.
	wbDetailResponseGlob = "**/cards/v4/detail**"
	// .by catalog fires this with curr=byn after x_wbaas_token is on the page.
	wbInternalDetailGlob = "**/__internal/u-card/cards/v4/detail**"
)

func wbCaptureGlob(normalizedURL string) string {
	if wbMarketFromURL(normalizedURL).curr == "byn" {
		return wbInternalDetailGlob
	}
	return wbDetailResponseGlob
}

func wbInternalDetailURL(normalizedURL, nm, dest string) string {
	market := wbMarketFromURL(normalizedURL)
	return fmt.Sprintf(
		"%s/__internal/u-card/cards/v4/detail?appType=1&curr=%s&dest=%s&lang=ru&nm=%s",
		wbCatalogOrigin(normalizedURL),
		url.QueryEscape(market.curr),
		url.QueryEscape(dest),
		url.QueryEscape(nm),
	)
}

// A working capture takes ~7s; the WB antibot page never fires cards/v4/detail.
const (
	// First BY page may mint x_wbaas_token (research: 4–70s). Keep under PARSER_BROWSER_TIMEOUT.
	wbBrowserTimeout = 85 * time.Second
	// CaptureJSON only after __internal misses, and only with the time left.
	wbCaptureJSONBudget = 25 * time.Second
)

type detailSource struct {
	name string
	data DetailResponseDTO
	err  error
}

func catalogPageURL(normalizedURL string) string {
	return normalizedURL
}

func (s *Strategy) captureDetail(ctx context.Context, normalizedURL string) detailSource {
	if s == nil || s.capturer == nil {
		return detailSource{name: "browser", err: fmt.Errorf("browser capturer is nil")}
	}
	pageURL := catalogPageURL(normalizedURL)
	capCtx, cancel := context.WithTimeout(ctx, wbBrowserTimeout)
	defer cancel()

	market := wbMarketFromURL(normalizedURL)
	canInternal := market.curr == "byn" && s.pages != nil
	var lastErr error

	// .by antibot page does not emit cards/v4/detail. Fetch __internal in the page
	// after x_wbaas_token; do not burn the budget on CaptureJSON first.
	if canInternal {
		wbID, idErr := s.resolveWBID(normalizedURL)
		if idErr != nil {
			lastErr = idErr
		} else if internal := s.captureInternalDetail(capCtx, ctx, normalizedURL, pageURL, wbID); sourceHasPrice(normalizedURL, internal) {
			return internal
		} else if internal.err != nil {
			lastErr = internal.err
		}
	}

	jsonCtx, jsonCancel := context.WithTimeout(capCtx, wbCaptureJSONBudget)
	defer jsonCancel()
	src, ok := s.captureJSONDetail(jsonCtx, ctx, normalizedURL, pageURL)
	if ok {
		return src
	}
	if src.err != nil {
		lastErr = src.err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("wb browser detail has no host-currency price")
	}
	recordWBHTTP(ctx, "browser", http.MethodGet, pageURL, 0, nil, lastErr)
	return detailSource{name: "browser", err: lastErr}
}

func (s *Strategy) captureJSONDetail(
	capCtx, recCtx context.Context,
	normalizedURL, pageURL string,
) (detailSource, bool) {
	body, capErr := s.capturer.CaptureJSON(capCtx, pageURL, wbCaptureGlob(normalizedURL))
	if capErr != nil {
		recordWBHTTP(recCtx, "browser", http.MethodGet, pageURL, 0, nil, capErr)
		return detailSource{name: "browser", err: capErr}, false
	}
	data, decErr := decodeDetailJSON(body)
	if decErr != nil {
		recordWBHTTP(recCtx, "browser", http.MethodGet, pageURL, http.StatusOK, body, decErr)
		return detailSource{name: "browser", err: decErr}, false
	}
	src := detailSource{name: "browser", data: data}
	if !sourceHasPrice(normalizedURL, src) {
		err := errors.New("captured detail has no host-currency price")
		recordWBHTTP(recCtx, "browser", http.MethodGet, pageURL, http.StatusOK, body, err)
		return detailSource{name: "browser", data: data, err: err}, false
	}
	recordWBHTTP(recCtx, "browser", http.MethodGet, pageURL, http.StatusOK, body, nil)
	return src, true
}

func wbBrowserWarmupURL(normalizedURL string) string {
	// Origin home solves wbaas once; product detail is heavier and often stays on challenge HTML.
	if wbMarketFromURL(normalizedURL).curr == "rub" {
		return "https://www.wildberries.ru/"
	}
	return "https://www.wildberries.by/"
}

func (s *Strategy) captureInternalDetail(
	capCtx, recCtx context.Context,
	normalizedURL, pageURL, wbID string,
) detailSource {
	_ = pageURL
	warmup := wbBrowserWarmupURL(normalizedURL)
	var lastErr error
	for _, dest := range wbDestCandidates(wbMarketFromURL(normalizedURL)) {
		reqURL := wbInternalDetailURL(normalizedURL, wbID, dest)
		snap, err := s.pages.FetchInPage(capCtx, warmup, reqURL)
		if err != nil {
			lastErr = err
			recordWBHTTP(recCtx, "browser", http.MethodGet, reqURL, 0, nil, err)
			// Token mint failed or antibot HTML — second dest won't mint either.
			if strings.Contains(err.Error(), "x_wbaas_token") ||
				strings.Contains(err.Error(), "HTML instead of JSON") {
				break
			}
			continue
		}
		data, decErr := decodeDetailJSON(snap.Body)
		if decErr != nil {
			lastErr = decErr
			recordWBHTTP(recCtx, "browser", http.MethodGet, reqURL, 0, snap.Body, decErr)
			continue
		}
		src := detailSource{name: "browser", data: data}
		if sourceHasPrice(normalizedURL, src) {
			recordWBHTTP(recCtx, "browser", http.MethodGet, reqURL, http.StatusOK, snap.Body, nil)
			return src
		}
		lastErr = fmt.Errorf("wb internal detail dest=%s has no host-currency price", dest)
		recordWBHTTP(recCtx, "browser", http.MethodGet, reqURL, http.StatusOK, snap.Body, lastErr)
	}
	if lastErr == nil {
		lastErr = errors.New("wb internal detail: no dest")
	}
	return detailSource{name: "browser", err: lastErr}
}

func decodeDetailJSON(body []byte) (DetailResponseDTO, error) {
	var data DetailResponseDTO
	if err := json.Unmarshal(body, &data); err != nil {
		return DetailResponseDTO{}, fmt.Errorf("wb detail json: %w", err)
	}
	return data, nil
}

func firstPricedDetail(normalizedURL string, sources []detailSource) (title string, price float64, pics int, err error) {
	errs := make([]string, 0, len(sources))
	for _, src := range sources {
		if src.err != nil {
			errs = append(errs, src.name+": "+src.err.Error())
			continue
		}
		title, price, pics, extractErr := extractProductInfo(src.data, normalizedURL)
		if extractErr != nil {
			errs = append(errs, src.name+": "+extractErr.Error())
			continue
		}
		return title, price, pics, nil
	}
	if len(errs) == 0 {
		return "", 0, 0, fmt.Errorf("no wb detail sources")
	}
	return "", 0, 0, fmt.Errorf("%s", strings.Join(errs, "; "))
}

func firstMetadataDetail(sources []detailSource) (title string, pics int, err error) {
	errs := make([]string, 0, len(sources))
	for _, src := range sources {
		if src.err != nil {
			errs = append(errs, src.name+": "+src.err.Error())
			continue
		}
		title, pics, extractErr := extractProductMetadata(src.data)
		if extractErr == nil {
			return title, pics, nil
		}
		errs = append(errs, src.name+": "+extractErr.Error())
	}
	if len(errs) == 0 {
		return "", 0, fmt.Errorf("no wb detail sources")
	}
	return "", 0, fmt.Errorf("%s", strings.Join(errs, "; "))
}

var wbDetailHosts = []string{"card.wb.ru", "u-card.wb.ru"}

func fetchDetail(ctx context.Context, normalizedURL, id string) (DetailResponseDTO, error) {
	market := wbMarketFromURL(normalizedURL)
	origin := wbCatalogOrigin(normalizedURL)
	var (
		lastErr  error
		lastData DetailResponseDTO
		haveData bool
	)
	for _, dest := range wbDestCandidates(market) {
		for _, host := range wbDetailHosts {
			reqURL := fmt.Sprintf(
				"https://%s/cards/v4/detail?appType=1&curr=%s&dest=%s&lang=ru&nm=%s",
				host,
				url.QueryEscape(market.curr),
				url.QueryEscape(dest),
				url.QueryEscape(id),
			)

			status, body, err := doChromeTLSGet(ctx, reqURL, origin)
			if err != nil {
				if errors.Is(err, errChromeTLSUnavailable) {
					return fetchDetailStdlib(ctx, reqURL)
				}
				recordWBHTTP(ctx, host, http.MethodGet, reqURL, 0, nil, err)
				lastErr = err
				continue
			}
			if status != http.StatusOK {
				lastErr = fmt.Errorf("wb detail API returned HTTP %d", status)
				recordWBHTTP(ctx, host, http.MethodGet, reqURL, status, body, lastErr)
				continue
			}
			recordWBHTTP(ctx, host, http.MethodGet, reqURL, status, body, nil)
			data, decErr := decodeDetailJSON(body)
			if decErr != nil {
				lastErr = decErr
				continue
			}
			if _, _, _, priceErr := extractProductInfo(data, normalizedURL); priceErr == nil {
				return data, nil
			}
			lastData, haveData, lastErr = data, true, nil
		}
	}
	if haveData {
		return lastData, nil
	}
	if lastErr != nil {
		return DetailResponseDTO{}, lastErr
	}
	return DetailResponseDTO{}, errors.New("no wb detail hosts")
}

func fetchDetailStdlib(ctx context.Context, reqURL string) (DetailResponseDTO, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return DetailResponseDTO{}, err
	}
	req.Header.Set("User-Agent", wbHTTPUserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := wbHTTPClient.Do(req)
	if err != nil {
		recordWBHTTP(ctx, "card.wb.ru", http.MethodGet, reqURL, 0, nil, err)
		return DetailResponseDTO{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512<<10))
	if err != nil {
		recordWBHTTP(ctx, "card.wb.ru", http.MethodGet, reqURL, resp.StatusCode, nil, err)
		return DetailResponseDTO{}, err
	}
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("wb detail API returned HTTP %d", resp.StatusCode)
		recordWBHTTP(ctx, "card.wb.ru", http.MethodGet, reqURL, resp.StatusCode, body, err)
		return DetailResponseDTO{}, err
	}
	recordWBHTTP(ctx, "card.wb.ru", http.MethodGet, reqURL, resp.StatusCode, body, nil)
	return decodeDetailJSON(body)
}

type basketCardDTO struct {
	Name    string          `json:"name"`
	IMTName string          `json:"imt_name"`
	Brand   string          `json:"brand"`
	Pics    int             `json:"pics"`
	Sizes   []DetailSizeDTO `json:"sizes"`
}

func fetchBasketBest(ctx context.Context, id string) (DetailResponseDTO, string, error) {
	nmID, err := strconv.Atoi(id)
	if err != nil {
		return DetailResponseDTO{}, "", fmt.Errorf("invalid wb id: %w", err)
	}
	vol, part := nmID/100000, nmID/1000
	var (
		mu    sync.Mutex
		found = map[string]DetailResponseDTO{}
	)
	host, ok := findBasketHost(ctx, vol, func(ctx context.Context, host string) bool {
		card, fetchErr := fetchBasketCard(ctx, host, vol, part, nmID)
		if fetchErr != nil {
			return false
		}
		detail := basketCardToDetail(card)
		if _, _, metadataErr := extractProductMetadata(detail); metadataErr != nil {
			return false
		}
		mu.Lock()
		found[host] = detail
		mu.Unlock()
		return true
	})
	if !ok {
		return DetailResponseDTO{}, "", fmt.Errorf("basket card with metadata not found for %s", id)
	}
	mu.Lock()
	defer mu.Unlock()
	return found[host], host, nil
}

func fetchBasketCard(
	ctx context.Context,
	host string,
	vol, part, nmID int,
) (basketCardDTO, error) {
	reqURL := fmt.Sprintf("https://%s/vol%d/part%d/%d/info/ru/card.json", host, vol, part, nmID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return basketCardDTO{}, err
	}
	req.Header.Set("User-Agent", wbHTTPUserAgent)
	resp, err := wbHTTPClient.Do(req)
	if err != nil {
		return basketCardDTO{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return basketCardDTO{}, fmt.Errorf("wb basket API returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512<<10))
	if err != nil {
		return basketCardDTO{}, err
	}

	var card basketCardDTO
	if err := json.Unmarshal(body, &card); err != nil {
		return basketCardDTO{}, err
	}
	// One shard owns the card. Skip 404 noise from the scan.
	recordWBHTTP(ctx, "basket", http.MethodGet, reqURL, resp.StatusCode, body, nil)
	return card, nil
}

func basketCardToDetail(card basketCardDTO) DetailResponseDTO {
	name := card.Name
	if name == "" {
		name = card.IMTName
	}
	return DetailResponseDTO{Products: []DetailProductDTO{{Name: name, Brand: card.Brand, Pics: card.Pics, Sizes: card.Sizes}}}
}

func (s *Strategy) resolveBasketImageURL(ctx context.Context, wbID string, pics int) (*string, error) {
	nmID, err := strconv.Atoi(wbID)
	if err != nil {
		return nil, fmt.Errorf("invalid wb id: %w", err)
	}

	vol := nmID / 100000
	part := nmID / 1000

	maxImageIndex := pics
	if maxImageIndex < 1 {
		maxImageIndex = 1
	}
	if maxImageIndex < 30 {
		maxImageIndex = 30
	}
	if maxImageIndex > 60 {
		maxImageIndex = 60
	}

	host, hostErr := detectBasketHost(ctx, vol, part, nmID)
	if hostErr != nil {
		return nil, hostErr
	}
	if host != "" {
		if u := tryImagesOnHost(ctx, host, vol, part, nmID, maxImageIndex); u != nil {
			return u, nil
		}
	}

	return tryImagesOnAllBaskets(ctx, vol, part, nmID, maxImageIndex)
}

func tryImagesOnHost(
	ctx context.Context,
	host string,
	vol, part, nmID, maxImageIndex int,
) *string {
	for imgIndex := 1; imgIndex <= maxImageIndex; imgIndex++ {
		for _, ext := range []string{"webp", "jpg", "jpeg"} {
			for _, pattern := range basketImagePathPatterns {
				candidate := fmt.Sprintf(
					pattern,
					host,
					vol,
					part,
					nmID,
					imgIndex,
					ext,
				)
				if isImageReachable(ctx, candidate) {
					return &candidate
				}
			}
		}
	}
	return nil
}

func tryImagesOnAllBaskets(
	ctx context.Context,
	vol, part, nmID, maxImageIndex int,
) (*string, error) {
	for basket := 1; basket <= basketHostMaxIndex; basket++ {
		for _, hostPattern := range basketHostPatterns {
			host := fmt.Sprintf(hostPattern, basket)
			if u := tryImagesOnHost(ctx, host, vol, part, nmID, maxImageIndex); u != nil {
				return u, nil
			}
		}
	}
	return nil, nil
}

func detectBasketHost(
	ctx context.Context,
	vol int,
	part int,
	nmID int,
) (string, error) {
	host, _ := findBasketHost(ctx, vol, func(ctx context.Context, host string) bool {
		reqURL := fmt.Sprintf("https://%s/vol%d/part%d/%d/info/ru/card.json", host, vol, part, nmID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return false
		}
		req.Header.Set("User-Agent", wbHTTPUserAgent)
		resp, err := wbHTTPClient.Do(req)
		if err != nil {
			return false
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	})
	return host, nil
}

func isImageReachable(ctx context.Context, sourceURL string) bool {
	if probeImage(ctx, sourceURL, true) {
		return true
	}
	return probeImage(ctx, sourceURL, false)
}

func probeImage(ctx context.Context, sourceURL string, useRange bool) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", wbHTTPUserAgent)
	if useRange {
		req.Header.Set("Range", "bytes=0-0")
	}

	resp, err := wbHTTPClient.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusPartialContent:
		_, _ = io.CopyN(io.Discard, resp.Body, 64)
		return true
	case http.StatusNotFound:
		return false
	default:
		return false
	}
}
