package keycloak

import (
	"encoding/json"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestParseFeaturesClaim(t *testing.T) {
	t.Parallel()

	t.Run("object", func(t *testing.T) {
		t.Parallel()
		got := parseFeaturesClaim(json.RawMessage(`{"multiple-university-profiles":2}`))
		require.Equal(t, map[string]any{"multiple-university-profiles": float64(2)}, got)
	})

	t.Run("stringified object", func(t *testing.T) {
		t.Parallel()
		got := parseFeaturesClaim(json.RawMessage(`"{\"a\":1}"`))
		require.Equal(t, map[string]any{"a": float64(1)}, got)
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, parseFeaturesClaim(nil))
		require.Nil(t, parseFeaturesClaim(json.RawMessage("null")))
	})
}

func TestAudienceMatches(t *testing.T) {
	t.Parallel()
	require.True(t, audienceMatches(nil, "api"))
	require.True(t, audienceMatches(jwt.ClaimStrings{"api", "other"}, "api"))
	require.False(t, audienceMatches(jwt.ClaimStrings{"other"}, "api"))
}
