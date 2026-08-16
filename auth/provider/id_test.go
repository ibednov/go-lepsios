package provider_test

import (
	"testing"

	"github.com/ibednov/go-lepsios/auth/provider"
	"github.com/stretchr/testify/require"
)

func TestParseAndValidate(t *testing.T) {
	id, err := provider.Parse(string(provider.LocalEmailPassword))
	require.NoError(t, err)
	require.Equal(t, provider.LocalEmailPassword, id)

	_, err = provider.Parse("unknown")
	require.Error(t, err)
}

func TestAll(t *testing.T) {
	require.Contains(t, provider.All(), provider.LocalEmailPassword)
	require.Contains(t, provider.All(), provider.Telegram)
}
