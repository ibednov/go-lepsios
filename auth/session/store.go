package session

import (
	"context"
	"time"
)

// Store persists refresh tokens (implemented in service, e.g. Redis).
type Store interface {
	SaveRefresh(ctx context.Context, tokenHash, userID string, expiresAt time.Time) error
	FindUserByRefresh(ctx context.Context, tokenHash string) (userID string, err error)
	RevokeRefresh(ctx context.Context, tokenHash string) error
	RevokeAllForUser(ctx context.Context, userID string) error
}

// TokenPair is returned after login/refresh.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}
