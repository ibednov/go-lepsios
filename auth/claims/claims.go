package claims

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/ibednov/go-lepsios/auth/provider"
	"github.com/ibednov/go-lepsios/identity"
)

// AccessClaims are signed into JWT access tokens.
type AccessClaims struct {
	UserID string `json:"user_id"`
	// AuthGeneration binds a session to the account credential generation.
	// A nil value means this token predates generation enforcement.
	AuthGeneration *uint64 `json:"auth_generation,omitempty"`
	// SessionID binds an access token to the refresh session that issued it.
	// Services can reject the access token immediately when that session is revoked.
	SessionID string `json:"sid,omitempty"`
	// ClientID identifies the application client that owns this session. It is
	// omitted for tokens issued before client-aware sessions were introduced.
	ClientID string             `json:"client_id,omitempty"`
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
	UserID         string
	AuthGeneration *uint64
	SessionID      string
	ClientID       string
	Provider       provider.ID
	Email          string
	Roles          []string
	Plan           string
	Features       []string
	Kind           identity.ActorKind
}

// ToPrincipal maps access claims to Principal.
func (c AccessClaims) ToPrincipal() Principal {
	return Principal{
		UserID:         c.UserID,
		AuthGeneration: c.AuthGeneration,
		SessionID:      c.SessionID,
		ClientID:       c.ClientID,
		Provider:       c.Provider,
		Email:          c.Email,
		Roles:          c.Roles,
		Plan:           c.Plan,
		Features:       c.Features,
		Kind:           c.Kind,
	}
}
