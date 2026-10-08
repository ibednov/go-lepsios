package exchange_test

import (
	"context"
	"testing"
	"time"

	"github.com/ibednov/go-lepsios/currency"
	"github.com/ibednov/go-lepsios/exchange"
	"github.com/stretchr/testify/require"
)

type stubProvider struct {
	id        string
	legacy    []string
	countries []string
	base      currency.Code
}

func (p stubProvider) ID() string { return p.id }

func (p stubProvider) LegacyIDs() []string { return p.legacy }

func (p stubProvider) Countries() []string { return p.countries }

func (p stubProvider) BaseCurrency() currency.Code {
	if p.base.IsValid() {
		return p.base
	}
	return currency.BYN
}

func (p stubProvider) FetchRates(_ context.Context, _ time.Time) (exchange.Snapshot, error) {
	return exchange.Snapshot{ProviderID: p.id}, nil
}

func TestRegistryGet(t *testing.T) {
	t.Parallel()

	reg := exchange.NewRegistry(
		stubProvider{id: "by_nbrb", legacy: []string{"nbrb"}, countries: []string{"by"}},
		stubProvider{id: "az_cbar", countries: []string{"az"}, base: currency.AZN},
		stubProvider{id: "pl_nbp", countries: []string{"pl"}, base: currency.PLN},
	)

	got, err := reg.Get("BY")
	require.NoError(t, err)
	require.Equal(t, "by_nbrb", got.ID())

	got, err = reg.Get("AZ")
	require.NoError(t, err)
	require.Equal(t, "az_cbar", got.ID())

	got, err = reg.Get("PL")
	require.NoError(t, err)
	require.Equal(t, "pl_nbp", got.ID())

	_, err = reg.Get("CN")
	require.ErrorIs(t, err, exchange.ErrProviderNotFound)
}
