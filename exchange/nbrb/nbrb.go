// Package nbrb is a deprecated alias for exchange/by_nbrb.
//
// Deprecated: import github.com/ibednov/go-lepsios/exchange/by_nbrb instead.
package nbrb

import (
	"net/http"

	"github.com/ibednov/go-lepsios/exchange/by_nbrb"
)

const (
	ProviderID       = by_nbrb.ProviderID
	LegacyProviderID = by_nbrb.LegacyProviderID
)

type Provider = by_nbrb.Provider

func NewProvider(baseURL string, httpClient *http.Client) *Provider {
	return by_nbrb.NewProvider(baseURL, httpClient)
}
