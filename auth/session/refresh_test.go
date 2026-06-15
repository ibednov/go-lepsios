package session_test

import (
	"context"
	"testing"
	"time"

	"github.com/ibednov/go-lepsios/auth/claims"
	"github.com/ibednov/go-lepsios/auth/session"
	"github.com/ibednov/go-lepsios/auth/token"
	"github.com/stretchr/testify/require"
)

type memStore struct {
	entries map[string]string
}

func (m *memStore) SaveRefresh(_ context.Context, tokenHash, userID string, _ time.Time) error {
	m.entries[tokenHash] = userID
	return nil
}

func (m *memStore) FindUserByRefresh(_ context.Context, tokenHash string) (string, error) {
	return m.entries[tokenHash], nil
}

func (m *memStore) RevokeRefresh(_ context.Context, tokenHash string) error {
	delete(m.entries, tokenHash)
	return nil
}

func (m *memStore) RevokeAllForUser(_ context.Context, _ string) error {
	clear(m.entries)
	return nil
}

func TestIssue(t *testing.T) {
	mgr := token.NewManager(token.Config{
		Secret:     "secret",
		AccessTTL:  time.Minute,
		RefreshTTL: time.Hour,
	})
	store := &memStore{entries: map[string]string{}}
	svc := session.NewRefreshService(mgr, store, nil)

	pair, err := svc.Issue(context.Background(), claims.AccessClaims{UserID: "u1"})
	require.NoError(t, err)
	require.NotEmpty(t, pair.AccessToken)
	require.NotEmpty(t, pair.RefreshToken)
	require.Equal(t, int64(60), pair.ExpiresIn)

	hash := session.HashToken(pair.RefreshToken)
	require.Equal(t, "u1", store.entries[hash])
}

func TestRefresh(t *testing.T) {
	mgr := token.NewManager(token.Config{
		Secret:     "secret",
		AccessTTL:  time.Minute,
		RefreshTTL: time.Hour,
	})
	store := &memStore{entries: map[string]string{}}
	svc := session.NewRefreshService(mgr, store, nil)

	pair, err := svc.Issue(context.Background(), claims.AccessClaims{UserID: "u1"})
	require.NoError(t, err)

	newPair, err := svc.Refresh(context.Background(), pair.RefreshToken)
	require.NoError(t, err)
	require.NotEmpty(t, newPair.AccessToken)
	require.NotEmpty(t, newPair.RefreshToken)
	require.NotEqual(t, pair.RefreshToken, newPair.RefreshToken)

	oldHash := session.HashToken(pair.RefreshToken)
	_, ok := store.entries[oldHash]
	require.False(t, ok)
}

func TestLogoutUser(t *testing.T) {
	mgr := token.NewManager(token.Config{
		Secret:     "secret",
		AccessTTL:  time.Minute,
		RefreshTTL: time.Hour,
	})
	store := &memStore{entries: map[string]string{}}
	svc := session.NewRefreshService(mgr, store, nil)

	_, err := svc.Issue(context.Background(), claims.AccessClaims{UserID: "u1"})
	require.NoError(t, err)

	require.NoError(t, svc.LogoutUser(context.Background(), "u1"))
	require.Empty(t, store.entries)
}
