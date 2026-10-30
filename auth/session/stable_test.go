package session_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ibednov/go-lepsios/auth/claims"
	"github.com/ibednov/go-lepsios/auth/session"
	"github.com/ibednov/go-lepsios/auth/token"
	"github.com/stretchr/testify/require"
)

type stableMemoryStore struct {
	mu          sync.Mutex
	credentials map[string]session.StableSession
	rotateErr   error
}

func newStableMemoryStore() *stableMemoryStore {
	return &stableMemoryStore{credentials: make(map[string]session.StableSession)}
}

func (s *stableMemoryStore) SaveRefresh(_ context.Context, hash, userID string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.credentials[hash] = session.StableSession{ID: hash, UserID: userID, ExpiresAt: expiresAt}
	return nil
}

func (s *stableMemoryStore) FindUserByRefresh(_ context.Context, hash string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.credentials[hash].UserID, nil
}

func (s *stableMemoryStore) RevokeRefresh(_ context.Context, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.credentials, hash)
	return nil
}

func (s *stableMemoryStore) RevokeAllForUser(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for hash, entry := range s.credentials {
		if entry.UserID == userID {
			delete(s.credentials, hash)
		}
	}
	return nil
}

func (s *stableMemoryStore) SaveStableSession(_ context.Context, hash string, entry session.StableSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.credentials[hash] = entry
	return nil
}

func (s *stableMemoryStore) FindStableSessionByRefresh(_ context.Context, hash string) (session.StableSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.credentials[hash], nil
}

func (s *stableMemoryStore) RotateStableSession(_ context.Context, oldHash, newHash string, entry session.StableSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rotateErr != nil {
		return s.rotateErr
	}
	current, ok := s.credentials[oldHash]
	if !ok || current.ID != entry.ID || current.UserID != entry.UserID {
		return session.ErrInvalidRefreshToken
	}
	if _, exists := s.credentials[newHash]; exists {
		return errors.New("successor hash collision")
	}
	s.credentials[newHash] = entry
	delete(s.credentials, oldHash)
	return nil
}

func (s *stableMemoryStore) RevokeStableSession(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for hash, entry := range s.credentials {
		if entry.ID == sessionID {
			delete(s.credentials, hash)
		}
	}
	return nil
}

func TestStableSessionIDSurvivesRefreshAndLegacyPromotion(t *testing.T) {
	ctx := context.Background()
	mgr := token.NewManager(token.Config{Secret: "secret", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	store := newStableMemoryStore()
	svc := session.NewRefreshService(mgr, store, nil)

	pair, err := svc.Issue(ctx, claims.AccessClaims{UserID: "u1", ClientID: "web"})
	require.NoError(t, err)
	first, err := mgr.ParseAccess(pair.AccessToken)
	require.NoError(t, err)
	require.NotEqual(t, session.HashToken(pair.RefreshToken), first.SessionID)

	next, err := svc.Refresh(ctx, pair.RefreshToken)
	require.NoError(t, err)
	second, err := mgr.ParseAccess(next.AccessToken)
	require.NoError(t, err)
	require.Equal(t, first.SessionID, second.SessionID)
	require.Empty(t, store.credentials[session.HashToken(pair.RefreshToken)].UserID)
	require.Equal(t, first.SessionID, store.credentials[session.HashToken(next.RefreshToken)].ID)

	legacyToken := "legacy-refresh-token"
	legacyHash := session.HashToken(legacyToken)
	require.NoError(t, store.SaveRefresh(ctx, legacyHash, "legacy-user", time.Now().Add(time.Hour)))
	upgraded, err := svc.RefreshWithClientID(ctx, legacyToken, "web")
	require.NoError(t, err)
	upgradedClaims, err := mgr.ParseAccess(upgraded.AccessToken)
	require.NoError(t, err)
	require.Equal(t, legacyHash, upgradedClaims.SessionID)
	require.Equal(t, "web", upgradedClaims.ClientID)
	require.Equal(t, legacyHash, store.credentials[session.HashToken(upgraded.RefreshToken)].ID)
}

func TestStableRotationFailureKeepsOldCredentialAndConcurrentRefreshHasOneWinner(t *testing.T) {
	ctx := context.Background()
	mgr := token.NewManager(token.Config{Secret: "secret", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	store := newStableMemoryStore()
	svc := session.NewRefreshService(mgr, store, nil)
	pair, err := svc.Issue(ctx, claims.AccessClaims{UserID: "u1"})
	require.NoError(t, err)

	operationalErr := errors.New("redis unavailable")
	store.rotateErr = operationalErr
	_, err = svc.Refresh(ctx, pair.RefreshToken)
	require.ErrorIs(t, err, operationalErr)
	require.NotEmpty(t, store.credentials[session.HashToken(pair.RefreshToken)].UserID)
	store.rotateErr = nil

	type result struct {
		pair session.TokenPair
		err  error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, e := svc.Refresh(ctx, pair.RefreshToken)
			results <- result{p, e}
		}()
	}
	wg.Wait()
	close(results)
	successes, rejected := 0, 0
	for item := range results {
		if item.err == nil {
			successes++
			require.NotEmpty(t, store.credentials[session.HashToken(item.pair.RefreshToken)].UserID)
		} else if errors.Is(item.err, session.ErrInvalidRefreshToken) {
			rejected++
		} else {
			t.Fatalf("unexpected refresh error: %v", item.err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, rejected)
}

func TestMultiCookieRefreshRejectsAmbiguityAndWritesSelectedName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	mgr := token.NewManager(token.Config{Secret: "secret", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	store := newStableMemoryStore()
	svc := session.NewRefreshService(mgr, store, nil)
	web, err := svc.Issue(ctx, claims.AccessClaims{UserID: "u1", ClientID: "web"})
	require.NoError(t, err)
	legacy, err := svc.Issue(ctx, claims.AccessClaims{UserID: "u1"})
	require.NoError(t, err)
	router := gin.New()
	session.RegisterRefreshLogout(router.Group("/auth/me"), svc, mgr.Validator(),
		session.WithTokenTransport(session.TokenTransport{
			Refresh:                 session.DeliverCookie,
			RefreshCookieCandidates: []string{"refresh_token", "web_refresh_token"},
		}),
		session.WithLogoutScope(session.LogoutCurrentSession),
	)

	ambiguous := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/auth/me/refresh", nil)
	request.AddCookie(&http.Cookie{Name: "refresh_token", Value: legacy.RefreshToken})
	request.AddCookie(&http.Cookie{Name: "web_refresh_token", Value: web.RefreshToken})
	router.ServeHTTP(ambiguous, request)
	require.Equal(t, http.StatusConflict, ambiguous.Code)
	require.Contains(t, ambiguous.Body.String(), "error.auth.session.ambiguous_cookie")
	require.NotEmpty(t, store.credentials[session.HashToken(legacy.RefreshToken)].UserID)
	require.NotEmpty(t, store.credentials[session.HashToken(web.RefreshToken)].UserID)

	duplicate := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/auth/me/refresh", nil)
	request.AddCookie(&http.Cookie{Name: "web_refresh_token", Value: web.RefreshToken})
	request.AddCookie(&http.Cookie{Name: "web_refresh_token", Value: legacy.RefreshToken})
	router.ServeHTTP(duplicate, request)
	require.Equal(t, http.StatusConflict, duplicate.Code)
	require.Contains(t, duplicate.Body.String(), "error.auth.session.ambiguous_cookie")
	require.NotEmpty(t, store.credentials[session.HashToken(web.RefreshToken)].UserID)
	require.NotEmpty(t, store.credentials[session.HashToken(legacy.RefreshToken)].UserID)

	refresh := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/auth/me/refresh", nil)
	request.AddCookie(&http.Cookie{Name: "web_refresh_token", Value: web.RefreshToken})
	router.ServeHTTP(refresh, request)
	require.Equal(t, http.StatusOK, refresh.Code)
	cookies := refresh.Result().Cookies()
	require.Len(t, cookies, 1)
	require.Equal(t, "web_refresh_token", cookies[0].Name)
	rotatedRefresh, err := url.QueryUnescape(cookies[0].Value)
	require.NoError(t, err)
	require.NotEmpty(t, store.credentials[session.HashToken(rotatedRefresh)].UserID)

	logout := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/auth/me/logout", nil)
	request.Header.Set("Authorization", "Bearer "+web.AccessToken)
	request.AddCookie(&http.Cookie{Name: "refresh_token", Value: legacy.RefreshToken})
	request.AddCookie(&http.Cookie{Name: "web_refresh_token", Value: rotatedRefresh})
	router.ServeHTTP(logout, request)
	require.Equal(t, http.StatusOK, logout.Code)
	cleared := logout.Result().Cookies()
	require.Len(t, cleared, 1)
	require.Equal(t, "web_refresh_token", cleared[0].Name)
	require.Less(t, cleared[0].MaxAge, 0)
	require.NotEmpty(t, store.credentials[session.HashToken(legacy.RefreshToken)].UserID)

	mismatch := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/auth/me/logout", nil)
	request.Header.Set("Authorization", "Bearer "+legacy.AccessToken)
	request.AddCookie(&http.Cookie{Name: "web_refresh_token", Value: rotatedRefresh})
	router.ServeHTTP(mismatch, request)
	require.Equal(t, http.StatusOK, mismatch.Code)
	require.Empty(t, mismatch.Result().Cookies())
	require.Empty(t, store.credentials[session.HashToken(legacy.RefreshToken)].UserID)
}
