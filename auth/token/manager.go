package token

import (
	"context"
	"errors"
	"time"

	"github.com/ibednov/go-lepsios/auth/claims"
	"github.com/golang-jwt/jwt/v5"
)

// Config configures JWT signing.
type Config struct {
	Secret     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
	Issuer     string
}

// Manager signs and parses access tokens.
type Manager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	issuer     string
}

// NewManager creates a JWT manager.
func NewManager(cfg Config) *Manager {
	issuer := cfg.Issuer
	if issuer == "" {
		issuer = "go-lepsios"
	}
	return &Manager{
		secret:     []byte(cfg.Secret),
		accessTTL:  cfg.AccessTTL,
		refreshTTL: cfg.RefreshTTL,
		issuer:     issuer,
	}
}

// SignAccess creates a signed access token.
func (m *Manager) SignAccess(_ context.Context, c claims.AccessClaims) (string, error) {
	now := time.Now()
	c.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
		IssuedAt:  jwt.NewNumericDate(now),
		Issuer:    m.issuer,
		Subject:   c.UserID,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return token.SignedString(m.secret)
}

// ParseAccess validates and parses an access token.
func (m *Manager) ParseAccess(tokenString string) (claims.AccessClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &claims.AccessClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil {
		return claims.AccessClaims{}, err
	}
	parsed, ok := token.Claims.(*claims.AccessClaims)
	if !ok || !token.Valid {
		return claims.AccessClaims{}, errors.New("invalid token")
	}
	return *parsed, nil
}

// RefreshTTL returns refresh token lifetime.
func (m *Manager) RefreshTTL() time.Duration {
	return m.refreshTTL
}

// AccessTTL returns access token lifetime in seconds.
func (m *Manager) AccessTTLSeconds() int64 {
	return int64(m.accessTTL.Seconds())
}

// Validator returns a TokenValidator backed by this manager.
func (m *Manager) Validator() *ManagerValidator {
	return &ManagerValidator{m: m}
}

// ManagerValidator adapts Manager to validator.TokenValidator.
type ManagerValidator struct {
	m *Manager
}

// ValidateToken implements validator.TokenValidator.
func (v *ManagerValidator) ValidateToken(_ context.Context, rawToken string) (claims.Principal, error) {
	c, err := v.m.ParseAccess(rawToken)
	if err != nil {
		return claims.Principal{}, err
	}
	return c.ToPrincipal(), nil
}
