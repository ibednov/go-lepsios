package onliner

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/ibednov/go-lepsios/currency"
	sidecarparser "github.com/ibednov/go-lepsios/parser"
)

var supportedHosts = []string{
	"catalog.onliner.by",
	"www.catalog.onliner.by",
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
	productKey, err := resolveProductKey(normalizedURL)
	if err != nil {
		return sidecarparser.RawParseResult{}, sidecarErrInvalidInput("parser.onliner.parse_card", err)
	}

	product, err := s.fetchProduct(ctx, normalizedURL, productKey)
	if err != nil {
		return sidecarparser.RawParseResult{}, sidecarErrParseFailed("parser.onliner.parse_card", err)
	}

	if product.Price <= 0 {
		// Catalog row exists (title/image) but shops have no offers — not a generic parse failure.
		return sidecarparser.RawParseResult{}, sidecarErrNoPrice(
			"parser.onliner.parse_card",
			fmt.Errorf("%w: %s", errNoShopOffers, product.Title),
		)
	}

	price := product.Price
	out := sidecarparser.RawParseResult{
		Title:    product.Title,
		Price:    &price,
		Currency: currency.BYN,
		ImageURL: nil,
	}
	if c := strings.TrimSpace(product.Currency); c != "" {
		if code := currency.Code(strings.ToUpper(c)); code.IsValid() {
			out.Currency = code
		}
	}
	return out, nil
}

func (s *Strategy) ParseProductImageURL(
	ctx context.Context,
	normalizedURL string,
) (*string, error) {
	productKey, err := resolveProductKey(normalizedURL)
	if err != nil {
		return nil, sidecarErrInvalidInput("parser.onliner.parse_product_image", err)
	}

	product, err := s.fetchProduct(ctx, normalizedURL, productKey)
	if err != nil {
		return nil, sidecarErrParseFailed("parser.onliner.parse_product_image", err)
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

var errNoShopOffers = errors.New("no shop offers with price")

func sidecarErrNoPrice(source string, err error) *sidecarparser.SidecarError {
	return &sidecarparser.SidecarError{
		Code:   "no_price",
		Source: source,
		Err:    err,
	}
}
