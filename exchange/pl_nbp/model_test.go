package pl_nbp

import (
	"testing"
	"time"

	"github.com/ibednov/go-lepsios/currency"
	"github.com/stretchr/testify/require"
)

func TestRatesFromDTO(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	rates := ratesFromDTO([]rateDTO{
		{Code: "USD", Mid: 3.9132},
		{Code: "EUR", Mid: 4.3789},
		{Code: "CNY", Mid: 0.5839},
		{Code: "RUB", Mid: 0.0458},
		{Code: "XXX", Mid: 1},
		{Code: "THB", Mid: 0.1163},
	}, now)

	byCode := map[currency.Code]currency.OfficialRate{}
	for _, r := range rates {
		byCode[r.Code] = r
	}
	require.Contains(t, byCode, currency.USD)
	require.Equal(t, 1, byCode[currency.USD].Scale)
	require.Equal(t, 3.9132, byCode[currency.USD].BasePerUnit)
	require.Contains(t, byCode, currency.EUR)
	require.Contains(t, byCode, currency.CNY)
	require.Contains(t, byCode, currency.RUB)
	require.Contains(t, byCode, currency.PLN)
	require.Equal(t, 1.0, byCode[currency.PLN].BasePerUnit)
	require.NotContains(t, byCode, currency.Code("XXX"))
	require.NotContains(t, byCode, currency.Code("THB"))
}
