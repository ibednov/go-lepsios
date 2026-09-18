package featureflags_test

import (
	"testing"

	"github.com/ibednov/go-lepsios/featureflags"
	"github.com/stretchr/testify/require"
)

func TestNormalizeMergeWithout(t *testing.T) {
	t.Parallel()
	require.Equal(t, []string{"a", "b"}, featureflags.Normalize([]string{"a", "", "b", "a"}))
	require.True(t, featureflags.Contains([]string{"x", "y"}, "y"))
	require.Equal(t, []string{"a", "b", "c"}, featureflags.Merge([]string{"a", "b"}, []string{"b", "c"}))
	require.Equal(t, []string{"a", "c"}, featureflags.Without([]string{"a", "b", "c"}, []string{"b"}))
}
