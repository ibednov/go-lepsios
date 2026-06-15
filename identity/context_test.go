package identity_test

import (
	"context"
	"testing"

	"github.com/ibednov/go-lepsios/identity"
	"github.com/stretchr/testify/require"
)

func TestUserContext(t *testing.T) {
	ctx := context.Background()
	u := identity.User{ID: "u1", Kind: identity.ActorCustomer, Email: "a@b.c"}
	ctx = identity.SetUser(ctx, u)

	got, ok := identity.UserFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, "u1", got.ID)
	require.Equal(t, identity.ActorCustomer, got.Kind)
}

func TestSystemUser(t *testing.T) {
	u := identity.SystemUser("eco-back")
	require.Equal(t, "system:eco-back", u.ID)
	require.Equal(t, identity.ActorSystem, u.Kind)

	ctx := identity.WithSystemUser(context.Background(), "eco-back")
	got, ok := identity.UserFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, u.ID, got.ID)
}

func TestAuthorContext(t *testing.T) {
	ctx := identity.SetAuthor(context.Background(), identity.Author{
		UserID: "u1",
		Kind:   identity.ActorAdmin,
		Name:   "Admin",
	})
	a, ok := identity.AuthorFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, "Admin", a.Name)
}
