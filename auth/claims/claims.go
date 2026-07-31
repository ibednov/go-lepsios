package claims

import (
	"github.com/ibednov/go-lepsios/auth/provider"
	"github.com/ibednov/go-lepsios/identity"
	"github.com/golang-jwt/jwt/v5"
)

// AccessClaims are signed into JWT access tokens.
type AccessClaims struct {
	UserID   string             `json:"user_id"`
	Provider provider.ID        `json:"provider,omitempty"`
	Kind     identity.ActorKind `json:"kind,omitempty"`
	Email    string             `json:"email,omitempty"`
	Roles    []string           `json:"roles,omitempty"`
	Plan     string             `json:"plan,omitempty"`
	Features []string           `json:"features,omitempty"`
	jwt.RegisteredClaims
}

// Principal is the validated token subject before identity.User mapping.
type Principal struct {
	UserID   string
	Provider provider.ID
	Email    string
	Roles    []string
	Plan     string
	Features []string
	Kind     identity.ActorKind
}

// ToPrincipal maps access claims to Principal.
func (c AccessClaims) ToPrincipal() Principal {
	return Principal{
		UserID:   c.UserID,
		Provider: c.Provider,
		Email:    c.Email,
		Roles:    c.Roles,
		Plan:     c.Plan,
		Features: c.Features,
		Kind:     c.Kind,
	}
}
