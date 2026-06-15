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
