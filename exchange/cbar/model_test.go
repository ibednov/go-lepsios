package cbar

import (
	"testing"
	"time"

	"github.com/ibednov/go-lepsios/currency"
	"github.com/stretchr/testify/require"
)

func TestRatesFromXML(t *testing.T) {
	t.Parallel()

	const sample = `<?xml version="1.0" encoding="UTF-8"?>
<ValCurs Date="02.10.2026" Name="AZN">
  <ValType Type="Bank metalları">
    <Valute Code="XAU"><Nominal>1 t.u.</Nominal><Name>Qızıl</Name><Value>7114.3555</Value></Valute>
  </ValType>
  <ValType Type="Xarici valyutalar">
    <Valute Code="USD"><Nominal>1</Nominal><Name>1 ABŞ dolları</Name><Value>1.7</Value></Valute>
    <Valute Code="RUB"><Nominal>100</Nominal><Name>100 Rusiya rublu</Name><Value>2.0186</Value></Valute>
    <Valute Code="BYN"><Nominal>1</Nominal><Name>1 Belarus rublu</Name><Value>0.5638</Value></Valute>
    <Valute Code="XXX"><Nominal>1</Nominal><Name>skip</Name><Value>1</Value></Valute>
  </ValType>
</ValCurs>`

	now := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	items, err := parseXMLValutes([]byte(sample))
	require.NoError(t, err)

	rates := ratesFromXML(items, now)
	byCode := map[currency.Code]currency.OfficialRate{}
	for _, r := range rates {
		byCode[r.Code] = r
	}
	require.Contains(t, byCode, currency.USD)
	require.Equal(t, 1, byCode[currency.USD].Scale)
	require.Equal(t, 1.7, byCode[currency.USD].BasePerUnit)
	require.Contains(t, byCode, currency.RUB)
	require.Equal(t, 100, byCode[currency.RUB].Scale)
	require.Equal(t, 2.0186, byCode[currency.RUB].BasePerUnit)
	require.Contains(t, byCode, currency.BYN)
	require.Contains(t, byCode, currency.AZN)
	require.Equal(t, 1.0, byCode[currency.AZN].BasePerUnit)
	require.NotContains(t, byCode, currency.Code("XAU"))
	require.NotContains(t, byCode, currency.Code("XXX"))
}
