package currency_test

import (
	"testing"

	"github.com/ibednov/go-lepsios/currency"
	"github.com/stretchr/testify/require"
)

func TestParseCNY(t *testing.T) {
	t.Parallel()

	code, err := currency.Parse("cny")
	require.NoError(t, err)
	require.Equal(t, currency.CNY, code)
	require.True(t, code.IsValid())
}

func TestConvertSameCurrency(t *testing.T) {
	t.Parallel()

	got, err := currency.Convert(48.88, currency.CNY, currency.CNY)
	require.NoError(t, err)
	require.Equal(t, 48.88, got)
}

func TestConvertCNYToBYN(t *testing.T) {
	t.Parallel()

	got, err := currency.Convert(10, currency.CNY, currency.BYN)
	require.NoError(t, err)
	require.Equal(t, 4.55, got)
}

func TestConvertUnknownCode(t *testing.T) {
	t.Parallel()

	_, err := currency.Convert(1, currency.Code("XXX"), currency.BYN)
	require.ErrorIs(t, err, currency.ErrUnknownCode)
}
