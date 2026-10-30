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
	stableStore, hasStableStore := s.store.(StableSessionStore)
	var generationStore GenerationStore
	if s.resolveGeneration != nil {
		var ok bool
		generationStore, ok = s.store.(GenerationStore)
		if !ok && !hasStableStore {
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
	if hasStableStore {
		rawSessionID, err := GenerateRefreshToken()
		if err != nil {
			return TokenPair{}, err
		}
		access.SessionID = HashToken(rawSessionID)
		accessToken, err := s.tokens.SignAccess(ctx, access)
		if err != nil {
			return TokenPair{}, err
		}
		if err := stableStore.SaveStableSession(ctx, tokenHash, StableSession{
			ID: access.SessionID, UserID: access.UserID, ClientID: access.ClientID,
			AuthGeneration: access.AuthGeneration, ExpiresAt: expiresAt,
		}); err != nil {
			return TokenPair{}, err
		}
		return TokenPair{
			AccessToken: accessToken, RefreshToken: refreshToken,
			ExpiresIn: s.tokens.AccessTTLSeconds(),
		}, nil
	}
	var saveErr error
	if access.ClientID != "" && generationStore != nil {
		clientGenerationStore, ok := s.store.(ClientGenerationStore)
		if !ok {
			return TokenPair{}, errors.New("session: client-generation store is not configured")
		}
		saveErr = clientGenerationStore.SaveRefreshClientGeneration(ctx, tokenHash, access.UserID, access.ClientID, expiresAt, *access.AuthGeneration)
	} else if access.ClientID != "" {
		clientStore, ok := s.store.(ClientStore)
		if !ok {
			return TokenPair{}, errors.New("session: client-aware store is not configured")
		}
		saveErr = clientStore.SaveRefreshWithClient(ctx, tokenHash, access.UserID, access.ClientID, expiresAt)
	} else if generationStore != nil {
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
	return s.RefreshWithClientID(ctx, refreshToken, "")
}

// RefreshWithClientID rotates a refresh token while assigning a trusted
// server-side client ID to legacy sessions that have no stored client ID.
// A persisted client ID always takes precedence over this fallback.
func (s *RefreshService) RefreshWithClientID(ctx context.Context, refreshToken, legacyClientID string) (TokenPair, error) {
	if s == nil || s.tokens == nil || s.store == nil {
		return TokenPair{}, errors.New("session: refresh service not configured")
	}
	if refreshToken == "" {
		return TokenPair{}, ErrInvalidRefreshToken
	}

	tokenHash := s.hash(refreshToken)
	if stableStore, ok := s.store.(StableSessionStore); ok {
		return s.refreshStable(ctx, stableStore, tokenHash, legacyClientID)
	}
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
	clientID := ""
	if clientStore, ok := s.store.(ClientStore); ok {
		clientID, err = clientStore.FindRefreshClientID(ctx, tokenHash)
		if err != nil {
			return TokenPair{}, err
		}
	}
	if clientID == "" {
		clientID = legacyClientID
	}
	access, err := s.accessClaimsForUser(ctx, userID)
	if err != nil {
		return TokenPair{}, err
	}
	if s.resolveGeneration != nil {
		access.AuthGeneration = &generation
	}
	// Persisted refresh-session metadata takes precedence over account claims.
	// Empty metadata is the legacy behavior for sessions issued before clients
	// were tagged.
	access.ClientID = clientID

	if err := s.store.RevokeRefresh(ctx, tokenHash); err != nil {
		return TokenPair{}, err
	}

	return s.Issue(ctx, access)
}

func (s *RefreshService) refreshStable(ctx context.Context, store StableSessionStore, oldHash, legacyClientID string) (TokenPair, error) {
	currentSession, err := store.FindStableSessionByRefresh(ctx, oldHash)
	if err != nil {
		return TokenPair{}, err
	}
	if currentSession.ID == "" || currentSession.UserID == "" ||
		(!currentSession.ExpiresAt.IsZero() && !time.Now().Before(currentSession.ExpiresAt)) {
		return TokenPair{}, ErrInvalidRefreshToken
	}

	if s.resolveGeneration != nil {
		currentGeneration, err := s.resolveGeneration(ctx, currentSession.UserID)
		if err != nil {
			return TokenPair{}, err
		}
		storedGeneration := uint64(0)
		if currentSession.AuthGeneration != nil {
			storedGeneration = *currentSession.AuthGeneration
		}
		if storedGeneration != currentGeneration {
			return TokenPair{}, ErrInvalidRefreshToken
		}
		currentSession.AuthGeneration = &currentGeneration
	}
	if currentSession.ClientID == "" {
		currentSession.ClientID = legacyClientID
	}
	access, err := s.accessClaimsForUser(ctx, currentSession.UserID)
	if err != nil {
		return TokenPair{}, err
	}
	access.ClientID = currentSession.ClientID
	access.AuthGeneration = currentSession.AuthGeneration
	access.SessionID = currentSession.ID

	refreshToken, err := GenerateRefreshToken()
	if err != nil {
		return TokenPair{}, err
	}
	accessToken, err := s.tokens.SignAccess(ctx, access)
	if err != nil {
		return TokenPair{}, err
	}
	currentSession.ExpiresAt = time.Now().Add(s.tokens.RefreshTTL())
	if err := store.RotateStableSession(ctx, oldHash, s.hash(refreshToken), currentSession); err != nil {
		return TokenPair{}, err
	}
	return TokenPair{
		AccessToken: accessToken, RefreshToken: refreshToken,
		ExpiresIn: s.tokens.AccessTTLSeconds(),
	}, nil
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

// sessionIDForRefreshToken resolves a cookie credential to its session without
// rotating it. The HTTP logout handler uses it to avoid clearing a different
// client's cookie when several cookie names coexist on one API host.
func (s *RefreshService) sessionIDForRefreshToken(ctx context.Context, refreshToken string) (string, error) {
	if s == nil || s.store == nil || refreshToken == "" {
		return "", ErrInvalidRefreshToken
	}
	hash := s.hash(refreshToken)
	if stableStore, ok := s.store.(StableSessionStore); ok {
		entry, err := stableStore.FindStableSessionByRefresh(ctx, hash)
		if err != nil {
			return "", err
		}
		if entry.ID == "" || entry.UserID == "" {
			return "", ErrInvalidRefreshToken
		}
		return entry.ID, nil
	}
	userID, err := s.store.FindUserByRefresh(ctx, hash)
	if err != nil {
		return "", err
	}
	if userID == "" {
		return "", ErrInvalidRefreshToken
	}
	return hash, nil
}

// LogoutSession revokes the refresh session identified by a validated access
// token's SessionID claim. With StableSessionStore this ID does not change
// during refresh; legacy Store implementations use the refresh-token hash.
func (s *RefreshService) LogoutSession(ctx context.Context, sessionID string) error {
	if s == nil || s.store == nil {
		return errors.New("session: refresh service not configured")
	}
	if sessionID == "" {
		return errors.New("session: session ID is required")
	}
	if stableStore, ok := s.store.(StableSessionStore); ok {
		return stableStore.RevokeStableSession(ctx, sessionID)
	}
	return s.store.RevokeRefresh(ctx, sessionID)
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

// LogoutClient revokes all refresh sessions belonging to one client for a
// user. It never falls back to account-wide revocation.
func (s *RefreshService) LogoutClient(ctx context.Context, userID, clientID string) error {
	if s == nil || s.store == nil {
		return errors.New("session: refresh service not configured")
	}
	if userID == "" || clientID == "" {
		return errors.New("session: user id and client id are required")
	}
	clientStore, ok := s.store.(ClientStore)
	if !ok {
		return errors.New("session: client-aware store is not configured")
	}
	return clientStore.RevokeAllForClient(ctx, userID, clientID)
}

// LogoutUnmarkedSessions revokes only legacy sessions that have no client
// marker when the store supports client-aware records. Older Store adapters
// keep their historical account-wide logout behavior.
func (s *RefreshService) LogoutUnmarkedSessions(ctx context.Context, userID string) error {
	if s == nil || s.store == nil {
		return errors.New("session: refresh service not configured")
	}
	if userID == "" {
		return errors.New("session: user id is required")
	}
	if clientStore, ok := s.store.(ClientStore); ok {
		return clientStore.RevokeAllUnmarkedForUser(ctx, userID)
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
