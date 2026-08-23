package exchange_test

import (
	"context"
	"testing"
	"time"

	"github.com/ibednov/go-lepsios/exchange"
	"github.com/stretchr/testify/require"
)

type stubProvider struct {
	id        string
	countries []string
}

func (p stubProvider) ID() string { return p.id }

func (p stubProvider) Countries() []string { return p.countries }

func (p stubProvider) FetchRates(_ context.Context, _ time.Time) (exchange.Snapshot, error) {
	return exchange.Snapshot{ProviderID: p.id}, nil
}

func TestRegistryGet(t *testing.T) {
	t.Parallel()

	reg := exchange.NewRegistry(stubProvider{id: "nbrb", countries: []string{"by"}})

	got, err := reg.Get("BY")
	require.NoError(t, err)
	require.Equal(t, "nbrb", got.ID())

	_, err = reg.Get("CN")
	require.ErrorIs(t, err, exchange.ErrProviderNotFound)
}
