package validator_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ibednov/go-lepsios/auth/claims"
	"github.com/ibednov/go-lepsios/auth/validator"
	"github.com/stretchr/testify/require"
)

func TestChainFirstSuccess(t *testing.T) {
	fail := validator.TokenValidatorFunc(func(context.Context, string) (claims.Principal, error) {
		return claims.Principal{}, errors.New("fail")
	})
	ok := validator.TokenValidatorFunc(func(context.Context, string) (claims.Principal, error) {
		return claims.Principal{UserID: "u1"}, nil
	})
	p, err := validator.Chain(fail, ok).ValidateToken(context.Background(), "tok")
	require.NoError(t, err)
	require.Equal(t, "u1", p.UserID)
}

func TestChainAllFail(t *testing.T) {
	fail := validator.TokenValidatorFunc(func(context.Context, string) (claims.Principal, error) {
		return claims.Principal{}, errors.New("fail")
	})
	_, err := validator.Chain(fail).ValidateToken(context.Background(), "tok")
	require.Error(t, err)
}

func TestWithSessionGenerationAllowsLegacyOnlyAtZero(t *testing.T) {
	ctx := context.Background()
	legacy := validator.TokenValidatorFunc(func(context.Context, string) (claims.Principal, error) {
		return claims.Principal{UserID: "user-1"}, nil
	})

	guarded := validator.WithSessionGeneration(legacy, func(context.Context, string) (uint64, error) {
		return 0, nil
	})
	principal, err := guarded.ValidateToken(ctx, "signed-legacy-token")
	require.NoError(t, err)
	require.Equal(t, "user-1", principal.UserID)
	require.Nil(t, principal.AuthGeneration)

	guarded = validator.WithSessionGeneration(legacy, func(context.Context, string) (uint64, error) {
		return 1, nil
	})
	_, err = guarded.ValidateToken(ctx, "signed-legacy-token")
	require.ErrorIs(t, err, validator.ErrStaleAuthGeneration)
}

func TestWithSessionGenerationRequiresClaimToMatch(t *testing.T) {
	ctx := context.Background()
	generation := uint64(2)
	base := validator.TokenValidatorFunc(func(context.Context, string) (claims.Principal, error) {
		return claims.Principal{UserID: "user-1", AuthGeneration: &generation}, nil
	})

	guarded := validator.WithSessionGeneration(base, func(context.Context, string) (uint64, error) {
		return 2, nil
	})
	_, err := guarded.ValidateToken(ctx, "current-token")
	require.NoError(t, err)

	guarded = validator.WithSessionGeneration(base, func(context.Context, string) (uint64, error) {
		return 3, nil
	})
	_, err = guarded.ValidateToken(ctx, "stale-token")
	require.ErrorIs(t, err, validator.ErrStaleAuthGeneration)
}

func TestWithSessionGenerationFailsClosedOnLookupError(t *testing.T) {
	lookupErr := errors.New("database unavailable")
	base := validator.TokenValidatorFunc(func(context.Context, string) (claims.Principal, error) {
		return claims.Principal{UserID: "user-1"}, nil
	})
	guarded := validator.WithSessionGeneration(base, func(context.Context, string) (uint64, error) {
		return 0, lookupErr
	})

	_, err := guarded.ValidateToken(context.Background(), "signed-legacy-token")
	require.ErrorIs(t, err, lookupErr)
}
