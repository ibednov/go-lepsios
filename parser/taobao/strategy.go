package taobao

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"github.com/ibednov/go-lepsios/currency"
	sidecarparser "github.com/ibednov/go-lepsios/parser"
)

var supportedHosts = []string{
	"e.tb.cn",
	"m.tb.cn",
	"tb.cn",
	"item.taobao.com",
	"taobao.com",
	"h5.m.taobao.com",
	"a.m.taobao.com",
	"detail.tmall.com",
	"tmall.com",
}

type Strategy struct {
	httpClient *http.Client
}

func NewStrategy(proxyURL string) *Strategy {
	return &Strategy{httpClient: newHTTPClient(proxyURL)}
}

func (s *Strategy) CanHandle(host string) bool {
	h := strings.ToLower(strings.TrimPrefix(host, "www."))
	return slices.Contains(supportedHosts, h) ||
		strings.HasSuffix(h, ".taobao.com") ||
		strings.HasSuffix(h, ".tmall.com") ||
		strings.HasSuffix(h, ".tb.cn")
}

func (s *Strategy) Parse(
	ctx context.Context,
	normalizedURL string,
) (sidecarparser.RawParseResult, error) {
	card, err := s.ParseCard(ctx, normalizedURL)
	if err != nil {
		return sidecarparser.RawParseResult{}, err
	}

	imageURL, imgErr := s.ParseProductImageURL(ctx, normalizedURL)
	if imgErr != nil {
		imageURL = nil
	}

	return sidecarparser.RawParseResult{
		Title:    card.Title,
		Price:    card.Price,
		Currency: card.Currency,
		ImageURL: imageURL,
	}, nil
}

func (s *Strategy) ParseCard(
	ctx context.Context,
	normalizedURL string,
) (sidecarparser.RawParseResult, error) {
	if err := validateProductURL(normalizedURL); err != nil {
		return sidecarparser.RawParseResult{}, sidecarErrInvalidInput("parser.taobao.parse_card", err)
	}

	product, err := s.fetchProduct(ctx, normalizedURL)
	if err != nil {
		return sidecarparser.RawParseResult{}, sidecarErrParseFailed("parser.taobao.parse_card", err)
	}

	price := product.Price
	return sidecarparser.RawParseResult{
		Title:    product.Title,
		Price:    &price,
		Currency: currency.Normalize(product.Currency, currency.CNY),
		ImageURL: nil,
	}, nil
}

func (s *Strategy) ParseProductImageURL(
	ctx context.Context,
	normalizedURL string,
) (*string, error) {
	if err := validateProductURL(normalizedURL); err != nil {
		return nil, sidecarErrInvalidInput("parser.taobao.parse_product_image", err)
	}

	product, err := s.fetchProduct(ctx, normalizedURL)
	if err != nil {
		return nil, sidecarErrParseFailed("parser.taobao.parse_product_image", err)
	}
	if product.ImageURL == "" {
		return nil, nil
	}
	imageURL := product.ImageURL
	return &imageURL, nil
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
