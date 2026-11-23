package dns

import (
	"context"
	"slices"
	"strings"

	sidecarparser "github.com/ibednov/go-lepsios/parser"
)

var supportedHosts = []string{
	"dns-shop.by",
	"www.dns-shop.by",
	// dns-shop.ru — Qrator 401 без браузера, не поддерживаем; RU-каталог: dns-shop.by/ru/...
	"dns-shop.kz",
	"www.dns-shop.kz",
	"dns-shop.kg",
	"www.dns-shop.kg",
}

type Strategy struct{}

func NewStrategy() *Strategy {
	return &Strategy{}
}

func (s *Strategy) CanHandle(host string) bool {
	h := strings.ToLower(host)
	return slices.Contains(supportedHosts, h)
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
		return sidecarparser.RawParseResult{}, sidecarErrInvalidInput("parser.dns.parse_card", err)
	}

	product, err := s.fetchProduct(ctx, normalizedURL)
	if err != nil {
		return sidecarparser.RawParseResult{}, sidecarErrParseFailed("parser.dns.parse_card", err)
	}

	price := product.Price
	return sidecarparser.RawParseResult{
		Title:    product.Title,
		Price:    &price,
		Currency: productCurrencyCode(product.Currency, normalizedURL),
		ImageURL: nil,
	}, nil
}

func (s *Strategy) ParseProductImageURL(
	ctx context.Context,
	normalizedURL string,
) (*string, error) {
	if err := validateProductURL(normalizedURL); err != nil {
		return nil, sidecarErrInvalidInput("parser.dns.parse_product_image", err)
	}

	product, err := s.fetchProduct(ctx, normalizedURL)
	if err != nil {
		return nil, sidecarErrParseFailed("parser.dns.parse_product_image", err)
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
