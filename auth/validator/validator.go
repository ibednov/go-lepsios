package validator

import (
	"context"
	"errors"

	"github.com/ibednov/go-lepsios/auth/claims"
)

// TokenValidator validates raw bearer tokens.
type TokenValidator interface {
	ValidateToken(ctx context.Context, rawToken string) (claims.Principal, error)
}

// TokenValidatorFunc adapts a function to TokenValidator.
type TokenValidatorFunc func(ctx context.Context, rawToken string) (claims.Principal, error)

// ValidateToken implements TokenValidator.
func (f TokenValidatorFunc) ValidateToken(ctx context.Context, rawToken string) (claims.Principal, error) {
	return f(ctx, rawToken)
}

// Chain returns the first successful validator (MixedAuth).
func Chain(validators ...TokenValidator) TokenValidator {
	return TokenValidatorFunc(func(ctx context.Context, rawToken string) (claims.Principal, error) {
		var lastErr error
		for _, v := range validators {
			if v == nil {
				continue
			}
			p, err := v.ValidateToken(ctx, rawToken)
			if err == nil {
				return p, nil
			}
			lastErr = err
		}
		if lastErr == nil {
			return claims.Principal{}, errors.New("no validators configured")
		}
		return claims.Principal{}, lastErr
	})
}
