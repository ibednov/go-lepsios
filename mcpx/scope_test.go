package mcpx

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeScope(t *testing.T) {
	t.Parallel()
	require.Equal(t, ScopeRead, NormalizeScope(""))
	require.Equal(t, ScopeRead, NormalizeScope("read"))
	require.Equal(t, ScopeRead, NormalizeScope("bogus"))
	require.Equal(t, ScopeWrite, NormalizeScope("write"))
}

func TestIdentityCanWrite(t *testing.T) {
	t.Parallel()
	require.False(t, Identity{Scope: ScopeRead}.CanWrite())
	require.False(t, Identity{Scope: ""}.CanWrite())
	require.False(t, Identity{Scope: "bogus"}.CanWrite())
	require.True(t, Identity{Scope: ScopeWrite}.CanWrite())
}
