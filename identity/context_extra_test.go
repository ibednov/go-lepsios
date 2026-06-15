package identity_test

import (
	"context"
	"testing"

	"github.com/ibednov/go-lepsios/identity"
	"github.com/stretchr/testify/require"
)

func TestMustUserPanics(t *testing.T) {
	require.Panics(t, func() {
		identity.MustUser(context.Background())
	})
}

func TestUserFromContextEmpty(t *testing.T) {
	ctx := context.Background()
	_, ok := identity.UserFromContext(ctx)
	require.False(t, ok)

	_, ok = identity.AuthorFromContext(ctx)
	require.False(t, ok)
}
