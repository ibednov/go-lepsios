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

func TestParseAZN(t *testing.T) {
	t.Parallel()

	code, err := currency.Parse("azn")
	require.NoError(t, err)
	require.Equal(t, currency.AZN, code)
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

func TestConvertUSDToAZN(t *testing.T) {
	t.Parallel()

	conv := currency.NewConverter(currency.AZN, nil)
	got, err := conv.Convert(10, currency.USD, currency.AZN)
	require.NoError(t, err)
	require.Equal(t, 17.0, got)
	require.Equal(t, currency.AZN, conv.Base())
}

func TestParsePLN(t *testing.T) {
	t.Parallel()

	code, err := currency.Parse("pln")
	require.NoError(t, err)
	require.Equal(t, currency.PLN, code)
}

func TestConvertUSDToPLN(t *testing.T) {
	t.Parallel()

	conv := currency.NewConverter(currency.PLN, nil)
	got, err := conv.Convert(10, currency.USD, currency.PLN)
	require.NoError(t, err)
	require.Equal(t, 39.1, got)
	require.Equal(t, currency.PLN, conv.Base())
}

func TestConvertUnknownCode(t *testing.T) {
	t.Parallel()

	_, err := currency.Convert(1, currency.Code("XXX"), currency.BYN)
	require.ErrorIs(t, err, currency.ErrUnknownCode)
}
