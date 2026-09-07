package entitlements_test

import (
	"testing"

	"github.com/ibednov/go-lepsios/billing/entitlements"
	"github.com/stretchr/testify/require"
)

func TestLimitAllows(t *testing.T) {
	t.Parallel()
	require.True(t, entitlements.NewLimit(5, false).Allows(4))
	require.False(t, entitlements.NewLimit(5, false).Allows(5))
	require.True(t, entitlements.NewLimit(0, true).Allows(1_000_000))
}

func TestGetLimit(t *testing.T) {
	t.Parallel()
	limit, unlimited := entitlements.GetLimit(map[string]any{"n": float64(1)}, "n")
	require.Equal(t, 1, limit)
	require.False(t, unlimited)

	_, unlimited = entitlements.GetLimit(map[string]any{"n": float64(-1)}, "n")
	require.True(t, unlimited)
}

func TestEnableIfLimitNonZero(t *testing.T) {
	t.Parallel()
	require.Empty(t, entitlements.EnableIfLimitNonZero(map[string]any{"n": float64(0)}, "n", "feat"))
	require.Equal(t, []string{"feat"}, entitlements.EnableIfLimitNonZero(map[string]any{"n": float64(1)}, "n", "feat"))
	require.Equal(t, []string{"feat"}, entitlements.EnableIfLimitNonZero(map[string]any{"n": float64(-1)}, "n", "feat"))
}

func TestMergeUnique(t *testing.T) {
	t.Parallel()
	require.Equal(t, []string{"a", "b", "c"}, entitlements.MergeUnique([]string{"a", "b"}, []string{"b", "c", ""}))
}
