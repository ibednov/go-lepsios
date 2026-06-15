package token_test

import (
	"context"
	"testing"
	"time"

	"github.com/ibednov/go-lepsios/auth/claims"
	"github.com/ibednov/go-lepsios/auth/provider"
	"github.com/ibednov/go-lepsios/auth/token"
	"github.com/ibednov/go-lepsios/identity"
	"github.com/stretchr/testify/require"
)

func TestSignAndParseAccess(t *testing.T) {
	m := token.NewManager(token.Config{
		Secret:     "secret",
		AccessTTL:  time.Minute,
		RefreshTTL: time.Hour,
		Issuer:     "test",
	})

	raw, err := m.SignAccess(context.Background(), claims.AccessClaims{
		UserID:   "user-1",
		Provider: provider.LocalEmailPassword,
		Kind:     identity.ActorCustomer,
		Email:    "a@b.c",
	})
	require.NoError(t, err)

	parsed, err := m.ParseAccess(raw)
	require.NoError(t, err)
	require.Equal(t, "user-1", parsed.UserID)
	require.Equal(t, provider.LocalEmailPassword, parsed.Provider)
	require.Equal(t, identity.ActorCustomer, parsed.Kind)
}

func TestValidator(t *testing.T) {
	m := token.NewManager(token.Config{
		Secret:     "secret",
		AccessTTL:  time.Minute,
		RefreshTTL: time.Hour,
	})
	v := m.Validator()

	raw, err := m.SignAccess(context.Background(), claims.AccessClaims{UserID: "u1"})
	require.NoError(t, err)

	p, err := v.ValidateToken(context.Background(), raw)
	require.NoError(t, err)
	require.Equal(t, "u1", p.UserID)
}
