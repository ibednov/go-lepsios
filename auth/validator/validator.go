package validator

import (
	"context"
	"errors"

	"github.com/ibednov/go-lepsios/auth/claims"
)

var ErrStaleAuthGeneration = errors.New("validator: stale auth generation")

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

// WithSessionGeneration rejects signed access tokens whose credential
// generation no longer matches the authoritative store. A token without a
// generation claim is accepted only while the current generation is zero, for
// compatibility with sessions issued before generation claims were added.
// Lookup errors fail closed. Apply this outside any session lookup cache.
func WithSessionGeneration(base TokenValidator, current func(context.Context, string) (uint64, error)) TokenValidator {
	return TokenValidatorFunc(func(ctx context.Context, rawToken string) (claims.Principal, error) {
		if base == nil || current == nil {
			return claims.Principal{}, errors.New("validator: session generation is not configured")
		}
		principal, err := base.ValidateToken(ctx, rawToken)
		if err != nil {
			return claims.Principal{}, err
		}
		generation, err := current(ctx, principal.UserID)
		if err != nil {
			return claims.Principal{}, err
		}
		if principal.AuthGeneration == nil {
			if generation == 0 {
				return principal, nil
			}
			return claims.Principal{}, ErrStaleAuthGeneration
		}
		if *principal.AuthGeneration != generation {
			return claims.Principal{}, ErrStaleAuthGeneration
		}
		return principal, nil
	})
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
