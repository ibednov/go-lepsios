package claims_test

import (
	"context"
	"testing"

	"github.com/ibednov/go-lepsios/auth/claims"
	"github.com/ibednov/go-lepsios/auth/provider"
	"github.com/stretchr/testify/require"
)

func TestPrincipalContext(t *testing.T) {
	ctx := claims.SetPrincipal(context.Background(), claims.Principal{
		UserID:   "u1",
		Provider: provider.LocalEmailPassword,
	})
	p, ok := claims.PrincipalFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, "u1", p.UserID)
}

func TestToPrincipal(t *testing.T) {
	p := claims.AccessClaims{
		UserID:   "u1",
		Provider: provider.LocalEmail2FA,
		Plan:     "standard",
		Features: []string{"price_monitor"},
	}.ToPrincipal()
	require.Equal(t, provider.LocalEmail2FA, p.Provider)
	require.Equal(t, "standard", p.Plan)
	require.Equal(t, []string{"price_monitor"}, p.Features)
}
