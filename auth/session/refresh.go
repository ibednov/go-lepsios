package session

import (
	"context"
	"errors"
	"time"

	"github.com/ibednov/go-lepsios/auth/claims"
	"github.com/ibednov/go-lepsios/auth/token"
)

// ErrInvalidRefreshToken indicates that a refresh credential is missing,
// expired, revoked, or otherwise unknown to the session store.
var ErrInvalidRefreshToken = errors.New("session: invalid refresh token")
var ErrStaleAuthGeneration = errors.New("session: stale auth generation")

// RefreshService issues token pairs and manages refresh lifecycle.
type RefreshService struct {
	tokens            *token.Manager
	store             Store
	hash              func(raw string) string
	resolveClaims     ClaimsResolver
	resolveGeneration GenerationResolver
}

// ClaimsResolver rebuilds full access claims from DB on refresh.
type ClaimsResolver func(ctx context.Context, userID string) (claims.AccessClaims, error)

// GenerationResolver returns the current credential generation for a user.
// When configured, an operational error must fail closed.
type GenerationResolver func(ctx context.Context, userID string) (uint64, error)

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

// WithGeneration enables generation-bound session issuance and refresh.
// The store must also implement GenerationStore.
func (s *RefreshService) WithGeneration(resolver GenerationResolver) *RefreshService {
	if s != nil {
		s.resolveGeneration = resolver
	}
	return s
}

// Issue signs access token and stores refresh token hash.
func (s *RefreshService) Issue(ctx context.Context, access claims.AccessClaims) (TokenPair, error) {
	if s == nil || s.tokens == nil || s.store == nil {
		return TokenPair{}, errors.New("session: refresh service not configured")
	}
	var generationStore GenerationStore
	if s.resolveGeneration != nil {
		var ok bool
		generationStore, ok = s.store.(GenerationStore)
		if !ok {
			return TokenPair{}, errors.New("session: generation store is not configured")
		}
		current, err := s.resolveGeneration(ctx, access.UserID)
		if err != nil {
			return TokenPair{}, err
		}
		if access.AuthGeneration != nil && *access.AuthGeneration != current {
			return TokenPair{}, ErrStaleAuthGeneration
		}
		access.AuthGeneration = &current
	}

	refreshToken, err := GenerateRefreshToken()
	if err != nil {
		return TokenPair{}, err
	}

	tokenHash := s.hash(refreshToken)
	expiresAt := time.Now().Add(s.tokens.RefreshTTL())
	var saveErr error
	if generationStore != nil {
		saveErr = generationStore.SaveRefreshGeneration(ctx, tokenHash, access.UserID, expiresAt, *access.AuthGeneration)
	} else {
		saveErr = s.store.SaveRefresh(ctx, tokenHash, access.UserID, expiresAt)
	}
	if saveErr != nil {
		return TokenPair{}, saveErr
	}
	access.SessionID = tokenHash
	accessToken, err := s.tokens.SignAccess(ctx, access)
	if err != nil {
		_ = s.store.RevokeRefresh(ctx, tokenHash)
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
		return TokenPair{}, ErrInvalidRefreshToken
	}

	tokenHash := s.hash(refreshToken)
	var userID string
	var generation uint64
	var err error
	if s.resolveGeneration != nil {
		generationStore, ok := s.store.(GenerationStore)
		if !ok {
			return TokenPair{}, errors.New("session: generation store is not configured")
		}
		userID, generation, err = generationStore.FindRefreshGeneration(ctx, tokenHash)
	} else {
		userID, err = s.store.FindUserByRefresh(ctx, tokenHash)
	}
	if err != nil {
		return TokenPair{}, err
	}
	if userID == "" {
		return TokenPair{}, ErrInvalidRefreshToken
	}
	if s.resolveGeneration != nil {
		current, err := s.resolveGeneration(ctx, userID)
		if err != nil {
			return TokenPair{}, err
		}
		if generation != current {
			return TokenPair{}, ErrInvalidRefreshToken
		}
	}
	access, err := s.accessClaimsForUser(ctx, userID)
	if err != nil {
		return TokenPair{}, err
	}
	if s.resolveGeneration != nil {
		access.AuthGeneration = &generation
	}

	if err := s.store.RevokeRefresh(ctx, tokenHash); err != nil {
		return TokenPair{}, err
	}

	return s.Issue(ctx, access)
}

func (s *RefreshService) accessClaimsForUser(ctx context.Context, userID string) (claims.AccessClaims, error) {
	if s.resolveClaims != nil {
		access, err := s.resolveClaims(ctx, userID)
		if err == nil {
			return access, nil
		}
		if s.resolveGeneration != nil {
			return claims.AccessClaims{}, err
		}
	}
	return claims.AccessClaims{UserID: userID}, nil
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
