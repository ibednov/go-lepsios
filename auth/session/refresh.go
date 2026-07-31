package session

import (
	"context"
	"errors"
	"time"

	"github.com/ibednov/go-lepsios/auth/claims"
	"github.com/ibednov/go-lepsios/auth/token"
)

// RefreshService issues token pairs and manages refresh lifecycle.
type RefreshService struct {
	tokens        *token.Manager
	store         Store
	hash          func(raw string) string
	resolveClaims ClaimsResolver
}

// ClaimsResolver rebuilds full access claims from DB on refresh.
type ClaimsResolver func(ctx context.Context, userID string) (claims.AccessClaims, error)

// NewRefreshService creates a refresh service.
func NewRefreshService(m *token.Manager, store Store, hash func(raw string) string) *RefreshService {
	if hash == nil {
		hash = HashToken
	}
	return &RefreshService{tokens: m, store: store, hash: hash}
}

// WithClaimsResolver sets callback to rebuild access claims on refresh.
func (s *RefreshService) WithClaimsResolver(resolver ClaimsResolver) *RefreshService {
	if s != nil {
		s.resolveClaims = resolver
	}
	return s
}

// Issue signs access token and stores refresh token hash.
func (s *RefreshService) Issue(ctx context.Context, access claims.AccessClaims) (TokenPair, error) {
	if s == nil || s.tokens == nil || s.store == nil {
		return TokenPair{}, errors.New("session: refresh service not configured")
	}

	accessToken, err := s.tokens.SignAccess(ctx, access)
	if err != nil {
		return TokenPair{}, err
	}

	refreshToken, err := GenerateRefreshToken()
	if err != nil {
		return TokenPair{}, err
	}

	tokenHash := s.hash(refreshToken)
	expiresAt := time.Now().Add(s.tokens.RefreshTTL())
	if err := s.store.SaveRefresh(ctx, tokenHash, access.UserID, expiresAt); err != nil {
		return TokenPair{}, err
	}

	return TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    s.tokens.AccessTTLSeconds(),
	}, nil
}

// Refresh rotates refresh token and issues a new access token.
func (s *RefreshService) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	if s == nil || s.tokens == nil || s.store == nil {
		return TokenPair{}, errors.New("session: refresh service not configured")
	}
	if refreshToken == "" {
		return TokenPair{}, errors.New("session: refresh token is required")
	}

	tokenHash := s.hash(refreshToken)
	userID, err := s.store.FindUserByRefresh(ctx, tokenHash)
	if err != nil {
		return TokenPair{}, err
	}
	if userID == "" {
		return TokenPair{}, errors.New("session: invalid refresh token")
	}

	if err := s.store.RevokeRefresh(ctx, tokenHash); err != nil {
		return TokenPair{}, err
	}

	return s.Issue(ctx, s.accessClaimsForUser(ctx, userID))
}

func (s *RefreshService) accessClaimsForUser(ctx context.Context, userID string) claims.AccessClaims {
	if s.resolveClaims != nil {
		access, err := s.resolveClaims(ctx, userID)
		if err == nil {
			return access
		}
	}
	return claims.AccessClaims{UserID: userID}
}

// Logout revokes a single refresh token.
func (s *RefreshService) Logout(ctx context.Context, refreshToken string) error {
	if s == nil || s.store == nil {
		return errors.New("session: refresh service not configured")
	}
	if refreshToken == "" {
		return errors.New("session: refresh token is required")
	}
	return s.store.RevokeRefresh(ctx, s.hash(refreshToken))
}

// LogoutUser revokes all refresh tokens for a user.
func (s *RefreshService) LogoutUser(ctx context.Context, userID string) error {
	if s == nil || s.store == nil {
		return errors.New("session: refresh service not configured")
	}
	if userID == "" {
		return errors.New("session: user id is required")
	}
	return s.store.RevokeAllForUser(ctx, userID)
}

// RefreshTTL returns refresh token lifetime.
func (s *RefreshService) RefreshTTL() time.Duration {
	if s == nil || s.tokens == nil {
		return 0
	}
	return s.tokens.RefreshTTL()
}
