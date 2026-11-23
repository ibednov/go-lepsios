package vek21

import (
	"context"
	"slices"
	"strings"

	"github.com/ibednov/go-lepsios/currency"
	sidecarparser "github.com/ibednov/go-lepsios/productparse"
)

var supportedHosts = []string{
	"21vek.by",
	"www.21vek.by",
	"m.21vek.by",
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
		return sidecarparser.RawParseResult{}, sidecarErrInvalidInput("parser.vek21.parse_card", err)
	}

	product, err := s.fetchProduct(ctx, normalizedURL)
	if err != nil {
		return sidecarparser.RawParseResult{}, sidecarErrParseFailed("parser.vek21.parse_card", err)
	}

	price := product.Price
	return sidecarparser.RawParseResult{
		Title:    product.Title,
		Price:    &price,
		Currency: currency.BYN,
		ImageURL: nil,
	}, nil
}

func (s *Strategy) ParseProductImageURL(
	ctx context.Context,
	normalizedURL string,
) (*string, error) {
	if err := validateProductURL(normalizedURL); err != nil {
		return nil, sidecarErrInvalidInput("parser.vek21.parse_product_image", err)
	}

	product, err := s.fetchProduct(ctx, normalizedURL)
	if err != nil {
		return nil, sidecarErrParseFailed("parser.vek21.parse_product_image", err)
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
