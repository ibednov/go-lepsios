package productmatch_test

import (
	"testing"

	"github.com/ibednov/go-lepsios/productmatch"
	"github.com/stretchr/testify/require"
)

func TestNormalizeURL_stripsTrackingParams(t *testing.T) {
	left := productmatch.NormalizeURL("https://Shop.Example/item?utm_source=tg&id=1")
	right := productmatch.NormalizeURL("https://shop.example/item?id=1&utm_medium=bot")
	require.Equal(t, left, right)
}

func TestNormalizeTitle(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "lowercase trim", in: "  Xiaomi Mi Kettle  ", want: "xiaomi mi kettle"},
		{name: "remove noise", in: "Новый Xiaomi Kettle купить", want: "xiaomi kettle"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, productmatch.NormalizeTitle(tt.in))
		})
	}
}

func TestCompareProducts_sameURL(t *testing.T) {
	a := productmatch.NormalizeProduct(productmatch.RawProductInput{
		Title: "Чайник Xiaomi",
		URL:   "https://market.example/teapot?color=white",
	})
	b := productmatch.NormalizeProduct(productmatch.RawProductInput{
		Title: "Другой title",
		URL:   "https://market.example/teapot?color=white&utm_source=bot",
	})

	result := productmatch.CompareProducts(a, b)
	require.Equal(t, productmatch.MatchExact, result.MatchType)
	require.Equal(t, 1.0, result.Score)
	require.Contains(t, result.Reasons, productmatch.ReasonSameURL)
}

func TestCompareProducts_similarTitle(t *testing.T) {
	a := productmatch.NormalizeProduct(productmatch.RawProductInput{Title: "Xiaomi Mi Smart Kettle Pro"})
	b := productmatch.NormalizeProduct(productmatch.RawProductInput{Title: "Xiaomi Smart Kettle Pro"})

	result := productmatch.CompareProducts(a, b)
	require.True(t, productmatch.IsStrongMatch(result.MatchType))
	require.GreaterOrEqual(t, result.Score, 0.72)
}

func TestCompareProducts_differentProducts(t *testing.T) {
	a := productmatch.NormalizeProduct(productmatch.RawProductInput{Title: "Xiaomi Kettle"})
	b := productmatch.NormalizeProduct(productmatch.RawProductInput{Title: "Tefal Iron"})

	result := productmatch.CompareProducts(a, b)
	require.Equal(t, productmatch.MatchNone, result.MatchType)
}

func TestMergeUniqueURLs(t *testing.T) {
	merged := productmatch.MergeUniqueURLs(
		[]string{"https://shop.example/a?utm_source=tg"},
		[]string{"https://shop.example/a", "https://shop.example/b"},
	)
	require.Len(t, merged, 2)
}
