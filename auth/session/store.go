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

// TokenPair is returned after login/refresh.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}
