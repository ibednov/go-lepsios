package onliner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnapshotFromAPIAllowsMissingPrices(t *testing.T) {
	t.Parallel()

	got, err := snapshotFromAPI(apiProductResponse{
		ExtendedName: "Зимние шины Test 215/55R18",
		Images:       apiImages{Header: "https://img.example/h.jpg"},
		// Prices zero-value: catalog returns prices:null when no offers.
	})
	require.NoError(t, err)
	assert.Equal(t, "Зимние шины Test 215/55R18", got.Title)
	assert.Equal(t, 0.0, got.Price)
	assert.Equal(t, "https://img.example/h.jpg", got.ImageURL)
}

func TestSnapshotFromAPIUsesPriceMinWhenPresent(t *testing.T) {
	t.Parallel()

	got, err := snapshotFromAPI(apiProductResponse{
		Name: "Phone",
		Prices: apiPricesBlock{
			PriceMin: apiMoney{Amount: "199.90", Currency: "BYN"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "Phone", got.Title)
	assert.InDelta(t, 199.90, got.Price, 0.001)
	assert.Equal(t, "BYN", got.Currency)
}

func TestExtractFromHTMLTitleWithoutPrice(t *testing.T) {
	t.Parallel()

	html := `<html><head>
<meta property="og:title" content="Grenlander Icedefensor">
<meta property="og:image" content="https://img.example/t.jpg">
</head></html>`
	got, ok := extractFromHTML(html)
	require.True(t, ok)
	assert.Equal(t, "Grenlander Icedefensor", got.Title)
	assert.Equal(t, 0.0, got.Price)
	assert.Equal(t, "https://img.example/t.jpg", got.ImageURL)
}

func TestResolveProductKeyFromCatalogPath(t *testing.T) {
	t.Parallel()

	key, err := resolveProductKey("https://catalog.onliner.by/tires/grenlander/is221555r1895t")
	require.NoError(t, err)
	assert.Equal(t, "is221555r1895t", key)
}

func TestParseCardNoOffersIsSpecializedError(t *testing.T) {
	t.Parallel()

	// Build result path without network: productSnapshot with title, zero price.
	// Exercise strategy mapping via sidecar error helper.
	err := sidecarErrNoPrice("parser.onliner.parse_card", errNoShopOffers)
	require.Error(t, err)
	assert.Equal(t, "no_price", err.Code)
	assert.Equal(t, "PARSER_PRICE_UNAVAILABLE", err.APIErrorCode())
}

