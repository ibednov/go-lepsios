package phone_test

import (
	"testing"

	"github.com/ibednov/go-lepsios/auth/phone"
	"github.com/stretchr/testify/require"
)

func TestNormalizeE164(t *testing.T) {
	t.Parallel()

	const region = "BY"

	t.Run("international BY", func(t *testing.T) {
		t.Parallel()
		got, err := phone.NormalizeE164("+375291234567", region)
		require.NoError(t, err)
		require.Equal(t, "+375291234567", got)
	})

	t.Run("national BY", func(t *testing.T) {
		t.Parallel()
		got, err := phone.NormalizeE164("291234567", region)
		require.NoError(t, err)
		require.Equal(t, "+375291234567", got)
	})

	t.Run("rejects junk", func(t *testing.T) {
		t.Parallel()
		for _, raw := range []string{"12345678", "987654321", "123123123123", "+67875342"} {
			_, err := phone.NormalizeE164(raw, region)
			require.Error(t, err)
		}
	})
}

func TestIsValidE164(t *testing.T) {
	t.Parallel()
	require.True(t, phone.IsValidE164("+375291234567", "BY"))
	require.False(t, phone.IsValidE164("12345678", "BY"))
}
