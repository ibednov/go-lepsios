package session_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ibednov/go-lepsios/auth/claims"
	"github.com/ibednov/go-lepsios/auth/session"
	"github.com/ibednov/go-lepsios/auth/token"
	"github.com/ibednov/go-lepsios/auth/validator"
	"github.com/stretchr/testify/require"
)

type generationEntry struct {
	userID     string
	generation uint64
}
type generationStore struct{ entries map[string]generationEntry }

func (s *generationStore) SaveRefresh(_ context.Context, hash, userID string, _ time.Time) error {
	s.entries[hash] = generationEntry{userID: userID}
	return nil
}
func (s *generationStore) FindUserByRefresh(_ context.Context, hash string) (string, error) {
	return s.entries[hash].userID, nil
}
func (s *generationStore) SaveRefreshGeneration(_ context.Context, hash, userID string, _ time.Time, generation uint64) error {
	s.entries[hash] = generationEntry{userID: userID, generation: generation}
	return nil
}
func (s *generationStore) FindRefreshGeneration(_ context.Context, hash string) (string, uint64, error) {
	entry := s.entries[hash]
	return entry.userID, entry.generation, nil
}
func (s *generationStore) RevokeRefresh(_ context.Context, hash string) error {
	delete(s.entries, hash)
	return nil
}
func (s *generationStore) RevokeAllForUser(_ context.Context, userID string) error {
	for hash, entry := range s.entries {
		if entry.userID == userID {
			delete(s.entries, hash)
		}
	}
	return nil
}

func TestGenerationRejectsStaleIssueRefreshAndAccess(t *testing.T) {
	ctx := context.Background()
	mgr := token.NewManager(token.Config{Secret: "secret", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	store := &generationStore{entries: make(map[string]generationEntry)}
	current := uint64(0)
	resolve := func(context.Context, string) (uint64, error) { return current, nil }
	svc := session.NewRefreshService(mgr, store, nil).WithGeneration(resolve)
	zero := uint64(0)
	pair, err := svc.Issue(ctx, claims.AccessClaims{UserID: "u1", AuthGeneration: &zero})
	require.NoError(t, err)
	require.Equal(t, zero, store.entries[session.HashToken(pair.RefreshToken)].generation)
	guarded := validator.WithSessionGeneration(mgr.Validator(), resolve)
	_, err = guarded.ValidateToken(ctx, pair.AccessToken)
	require.NoError(t, err)

	current = 1 // The password reset commits a new generation.
	_, err = guarded.ValidateToken(ctx, pair.AccessToken)
	require.ErrorIs(t, err, validator.ErrStaleAuthGeneration)
	_, err = svc.Refresh(ctx, pair.RefreshToken)
	require.ErrorIs(t, err, session.ErrInvalidRefreshToken)
	_, err = svc.Issue(ctx, claims.AccessClaims{UserID: "u1", AuthGeneration: &zero})
	require.ErrorIs(t, err, session.ErrStaleAuthGeneration)

	newPair, err := svc.Issue(ctx, claims.AccessClaims{UserID: "u1"})
	require.NoError(t, err)
	_, err = guarded.ValidateToken(ctx, newPair.AccessToken)
	require.NoError(t, err)
}

func TestGenerationLookupFailureFailsClosed(t *testing.T) {
	ctx := context.Background()
	mgr := token.NewManager(token.Config{Secret: "secret", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	store := &generationStore{entries: make(map[string]generationEntry)}
	lookupErr := errors.New("database unavailable")
	current := uint64(0)
	lookup := func(context.Context, string) (uint64, error) { return current, nil }
	svc := session.NewRefreshService(mgr, store, nil).WithGeneration(lookup)
	pair, err := svc.Issue(ctx, claims.AccessClaims{UserID: "u1"})
	require.NoError(t, err)
	svc.WithGeneration(func(context.Context, string) (uint64, error) { return 0, lookupErr })
	_, err = svc.Refresh(ctx, pair.RefreshToken)
	require.ErrorIs(t, err, lookupErr)
	_, err = svc.Issue(ctx, claims.AccessClaims{UserID: "u1"})
	require.ErrorIs(t, err, lookupErr)
	guarded := validator.WithSessionGeneration(mgr.Validator(), func(context.Context, string) (uint64, error) { return 0, lookupErr })
	_, err = guarded.ValidateToken(ctx, pair.AccessToken)
	require.ErrorIs(t, err, lookupErr)
}

func TestGenerationModeRejectsLegacyStoreAndAllowsLegacyClaimAtZero(t *testing.T) {
	ctx := context.Background()
	mgr := token.NewManager(token.Config{Secret: "secret", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	resolve := func(context.Context, string) (uint64, error) { return 0, nil }
	legacy := &memStore{entries: make(map[string]string)}
	svc := session.NewRefreshService(mgr, legacy, nil).WithGeneration(resolve)
	_, err := svc.Issue(ctx, claims.AccessClaims{UserID: "u1"})
	require.ErrorContains(t, err, "generation store")

	legacyToken, err := mgr.SignAccess(ctx, claims.AccessClaims{UserID: "u1"})
	require.NoError(t, err)
	guarded := validator.WithSessionGeneration(mgr.Validator(), resolve)
	_, err = guarded.ValidateToken(ctx, legacyToken)
	require.NoError(t, err)
	guarded = validator.WithSessionGeneration(mgr.Validator(), func(context.Context, string) (uint64, error) { return 1, nil })
	_, err = guarded.ValidateToken(ctx, legacyToken)
	require.ErrorIs(t, err, validator.ErrStaleAuthGeneration)
}
