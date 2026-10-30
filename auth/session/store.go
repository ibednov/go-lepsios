package session

import (
	"context"
	"time"
)

// Store persists refresh tokens (implemented in service, e.g. Redis).
// FindUserByRefresh must return an empty user ID and a nil error when the
// token does not exist or has expired. Errors are reserved for storage and
// other operational failures.
type Store interface {
	SaveRefresh(ctx context.Context, tokenHash, userID string, expiresAt time.Time) error
	FindUserByRefresh(ctx context.Context, tokenHash string) (userID string, err error)
	RevokeRefresh(ctx context.Context, tokenHash string) error
	RevokeAllForUser(ctx context.Context, userID string) error
}

// GenerationStore is an optional extension for applications that enforce
// credential generations. Existing Store implementations remain valid when
// RefreshService.WithGeneration is not configured.
type GenerationStore interface {
	SaveRefreshGeneration(ctx context.Context, tokenHash, userID string, expiresAt time.Time, generation uint64) error
	FindRefreshGeneration(ctx context.Context, tokenHash string) (userID string, generation uint64, err error)
}

// ClientStore is an optional Store extension for client-aware refresh
// sessions. Existing Store implementations remain compatible with legacy
// sessions whose client ID is empty. Implementations must return an empty
// client ID for refresh tokens created before client metadata was stored.
type ClientStore interface {
	SaveRefreshWithClient(ctx context.Context, tokenHash, userID, clientID string, expiresAt time.Time) error
	FindRefreshClientID(ctx context.Context, tokenHash string) (clientID string, err error)
	RevokeAllForClient(ctx context.Context, userID, clientID string) error
	RevokeAllUnmarkedForUser(ctx context.Context, userID string) error
}

// ClientGenerationStore is required when a client-aware session store is also
// used with credential-generation enforcement. Implementations save both
// fields atomically so a partial session record cannot bypass either policy.
type ClientGenerationStore interface {
	ClientStore
	GenerationStore
	SaveRefreshClientGeneration(ctx context.Context, tokenHash, userID, clientID string, expiresAt time.Time, generation uint64) error
}

// StableSession is the durable identity behind a rotating refresh credential.
// For a legacy credential, ID can be its original refresh hash: that keeps
// already-issued access tokens valid after the first rotation.
type StableSession struct {
	ID             string
	UserID         string
	ClientID       string
	AuthGeneration *uint64
	ExpiresAt      time.Time
}

// StableSessionStore enables stable access-token session IDs and atomic refresh
// rotation. It is optional so existing Store implementations keep their legacy
// behavior. FindStableSessionByRefresh must return a zero session for an absent
// or expired credential. RotateStableSession must atomically verify that the
// old credential still owns the session, install the new credential, update
// metadata, and remove the old credential. A lost race or revoked session must
// return ErrInvalidRefreshToken without creating a successor. Operational
// errors must leave the old credential usable.
type StableSessionStore interface {
	SaveStableSession(ctx context.Context, refreshHash string, session StableSession) error
	FindStableSessionByRefresh(ctx context.Context, refreshHash string) (StableSession, error)
	RotateStableSession(ctx context.Context, oldHash, newHash string, session StableSession) error
	RevokeStableSession(ctx context.Context, sessionID string) error
}

// TokenPair is returned after login/refresh.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}
