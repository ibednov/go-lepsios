package nbrb

import (
	"testing"
	"time"

	"github.com/ibednov/go-lepsios/currency"
	"github.com/stretchr/testify/require"
)

func TestRatesFromDTOKeepsCNY(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)
	rates := ratesFromDTO([]rateDTO{
		{CurAbbreviation: "CNY", CurScale: 10, CurOfficialRate: 4.61, Date: now.Format(time.RFC3339)},
		{CurAbbreviation: "USD", CurScale: 1, CurOfficialRate: 3.27, Date: now.Format(time.RFC3339)},
		{CurAbbreviation: "XXX", CurScale: 1, CurOfficialRate: 1, Date: now.Format(time.RFC3339)},
	}, now)

	codes := make([]currency.Code, 0, len(rates))
	for _, rate := range rates {
		codes = append(codes, rate.Code)
	}
	require.Contains(t, codes, currency.CNY)
	require.Contains(t, codes, currency.USD)
	require.Contains(t, codes, currency.BYN)
	require.NotContains(t, codes, currency.Code("XXX"))
}
