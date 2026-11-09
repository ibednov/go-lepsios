package mcpx

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIssueAndVerifySecret(t *testing.T) {
	t.Parallel()
	secret, hash, err := IssueSecret()
	require.NoError(t, err)
	require.NotEmpty(t, secret)
	require.Len(t, hash, 32)
	require.True(t, VerifySecret(secret, hash))
	require.False(t, VerifySecret(secret+"x", hash))
	require.False(t, VerifySecret(secret, hash[:16]))
	require.Equal(t, HashSecret(secret), hash)
}

func TestHashSecretLength(t *testing.T) {
	t.Parallel()
	require.Len(t, HashSecret("anything"), 32)
	require.Equal(t, HashSecret("abc"), HashSecret("abc"))
}

func TestVerifySecretRejectsEmpty(t *testing.T) {
	t.Parallel()
	require.False(t, VerifySecret("", HashSecret("abc")))
}
