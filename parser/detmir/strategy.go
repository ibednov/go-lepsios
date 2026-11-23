package detmir

import (
	"context"
	"slices"
	"strings"

	"github.com/ibednov/go-lepsios/currency"
	sidecarparser "github.com/ibednov/go-lepsios/parser"
)

var supportedHosts = []string{
	"detmir.by",
	"www.detmir.by",
	"detmir.ru",
	"www.detmir.ru",
	"detmir.kz",
	"www.detmir.kz",
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
	productID, apiHost, err := resolveProductRequest(normalizedURL)
	if err != nil {
		return sidecarparser.RawParseResult{}, sidecarErrInvalidInput("parser.detmir.parse_card", err)
	}

	product, err := s.fetchProduct(ctx, normalizedURL, productID, apiHost)
	if err != nil {
		return sidecarparser.RawParseResult{}, sidecarErrParseFailed("parser.detmir.parse_card", err)
	}

	price := product.Price
	return sidecarparser.RawParseResult{
		Title:    product.Title,
		Price:    &price,
		Currency: currency.Normalize(product.Currency, currency.BYN),
		ImageURL: nil,
	}, nil
}

func (s *Strategy) ParseProductImageURL(
	ctx context.Context,
	normalizedURL string,
) (*string, error) {
	productID, apiHost, err := resolveProductRequest(normalizedURL)
	if err != nil {
		return nil, sidecarErrInvalidInput("parser.detmir.parse_product_image", err)
	}

	product, err := s.fetchProduct(ctx, normalizedURL, productID, apiHost)
	if err != nil {
		return nil, sidecarErrParseFailed("parser.detmir.parse_product_image", err)
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
