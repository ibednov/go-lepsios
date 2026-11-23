package wildberries

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"

	sidecarparser "github.com/ibednov/go-lepsios/productparse"
	"github.com/ibednov/go-lepsios/browser"
)

func errNoProductID(path string) error {
	return fmt.Errorf("cannot extract product id from path: %s", path)
}

var links = []string{
	"wildberries.ru",
	"www.wildberries.ru",
	"wildberries.by",
	"www.wildberries.by",
}

var idPattern = regexp.MustCompile(`/catalog/(\d+)/detail\.aspx`)

type Strategy struct {
	capturer browser.JSONCapturer
	// pages is set when capturer also implements PageFetcher (RemoteClient / Client).
	pages browser.PageFetcher
}

func NewStrategy(capturer browser.JSONCapturer) *Strategy {
	s := &Strategy{capturer: capturer}
	if pages, ok := capturer.(browser.PageFetcher); ok {
		s.pages = pages
	}
	return s
}

func (s *Strategy) CanHandle(host string) bool {
	h := strings.ToLower(host)
	return slices.Contains(links, h)
}

func getID(u *url.URL) (string, error) {
	matches := idPattern.FindStringSubmatch(u.Path)
	if len(matches) < 2 {
		return "", errNoProductID(u.Path)
	}
	return matches[1], nil
}

func (s *Strategy) resolveWBID(normalizedURL string) (string, error) {
	u, err := url.Parse(normalizedURL)
	if err != nil {
		return "", err
	}
	return getID(u)
}

// Parse — полный парсинг (карточка + URL превью), без загрузки в файловый сервис.
func (s *Strategy) Parse(
	ctx context.Context,
	normalizedURL string,
) (sidecarparser.RawParseResult, error) {
	wbID, err := s.resolveWBID(normalizedURL)
	if err != nil {
		return sidecarparser.RawParseResult{}, sidecarErrInvalidInput("parser.wildberries.parse", err)
	}

	ctx, trace := withWBTrace(ctx)
	sources := s.collectDetailSources(ctx, normalizedURL, wbID)
	result, err := cardResult(normalizedURL, sources, trace, "parser.wildberries.parse")
	if err != nil {
		return sidecarparser.RawParseResult{}, err
	}

	if imageURL, imgErr := s.resolveBasketImageURL(ctx, wbID, maxPics(sources)); imgErr == nil {
		result.ImageURL = imageURL
	}
	result.WB = trace.snapshot()
	return result, nil
}

// ParseCard — title и price, если WB вернул оба поля.
func (s *Strategy) ParseCard(
	ctx context.Context,
	normalizedURL string,
) (sidecarparser.RawParseResult, error) {
	wbID, err := s.resolveWBID(normalizedURL)
	if err != nil {
		return sidecarparser.RawParseResult{}, sidecarErrInvalidInput("parser.wildberries.parse_card", err)
	}

	return s.parseCardResult(ctx, normalizedURL, wbID, "parser.wildberries.parse_card")
}

func (s *Strategy) parseCardResult(
	ctx context.Context,
	normalizedURL string,
	wbID string,
	source string,
) (sidecarparser.RawParseResult, error) {
	ctx, trace := withWBTrace(ctx)
	return cardResult(normalizedURL, s.collectDetailSources(ctx, normalizedURL, wbID), trace, source)
}

// maxPics: napi has a title but no pics, so the first metadata source is not enough.
func maxPics(sources []detailSource) int {
	best := 0
	for _, src := range sources {
		if src.err == nil && len(src.data.Products) > 0 && src.data.Products[0].Pics > best {
			best = src.data.Products[0].Pics
		}
	}
	return best
}

func cardResult(
	normalizedURL string,
	sources []detailSource,
	trace *wbTrace,
	source string,
) (sidecarparser.RawParseResult, error) {
	title, price, priceErr, metadataErr := cardFromSources(normalizedURL, sources)
	if metadataErr != nil {
		return sidecarparser.RawParseResult{WB: trace.snapshot()}, sidecarErrParseFailed(source, metadataErr)
	}

	result := sidecarparser.RawParseResult{
		Title:    title,
		Price:    price,
		Currency: currencyFromURL(normalizedURL),
		WB:       trace.snapshot(),
	}
	if priceErr != nil {
		result.PriceError = priceErr.Error()
	}
	return result, nil
}

// ParseProductImageURL — внешний URL превью (basket CDN), после чтения detail для pics.
func (s *Strategy) ParseProductImageURL(
	ctx context.Context,
	normalizedURL string,
) (*string, error) {
	wbID, err := s.resolveWBID(normalizedURL)
	if err != nil {
		return nil, sidecarErrInvalidInput("parser.wildberries.parse_product_image", err)
	}

	_, pics, err := s.fetchWBProductMetadata(ctx, normalizedURL, wbID)
	if err != nil {
		return nil, sidecarErrParseFailed("parser.wildberries.parse_product_image", err)
	}

	return s.resolveBasketImageURL(ctx, wbID, pics)
}

func sidecarErrInvalidInput(source string, err error) *sidecarparser.SidecarError {
	return &sidecarparser.SidecarError{
		Code:   "invalid_input",
		Source: source,
		Err:    err,
	}
}

func sidecarErrParseFailed(source string, err error) *sidecarparser.SidecarError {
	return &sidecarparser.SidecarError{
		Code:   "parse_failed",
		Source: source,
		Err:    err,
	}
}
