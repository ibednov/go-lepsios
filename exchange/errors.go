package exchange

import "errors"

var (
	ErrProviderNotFound = errors.New("exchange: provider not found for country")
	ErrFetchRates       = errors.New("exchange: fetch rates failed")
)
