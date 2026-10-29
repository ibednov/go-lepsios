package session_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ibednov/go-lepsios/auth/claims"
	"github.com/ibednov/go-lepsios/auth/session"
	"github.com/ibednov/go-lepsios/auth/token"
	"github.com/stretchr/testify/require"
)

type clientAwareStore struct {
	users       map[string]string
	clients     map[string]string
	generations map[string]uint64
}

func newClientAwareStore() *clientAwareStore {
	return &clientAwareStore{
		users:       map[string]string{},
		clients:     map[string]string{},
		generations: map[string]uint64{},
	}
}

func (s *clientAwareStore) SaveRefresh(_ context.Context, hash, userID string, _ time.Time) error {
	s.users[hash] = userID
	return nil
}

func (s *clientAwareStore) SaveRefreshWithClient(_ context.Context, hash, userID, clientID string, _ time.Time) error {
	s.users[hash] = userID
	s.clients[hash] = clientID
	return nil
}

func (s *clientAwareStore) SaveRefreshGeneration(_ context.Context, hash, userID string, _ time.Time, generation uint64) error {
	s.users[hash] = userID
	s.generations[hash] = generation
	return nil
}

func (s *clientAwareStore) SaveRefreshClientGeneration(_ context.Context, hash, userID, clientID string, _ time.Time, generation uint64) error {
	s.users[hash] = userID
	s.clients[hash] = clientID
	s.generations[hash] = generation
	return nil
}

func (s *clientAwareStore) FindUserByRefresh(_ context.Context, hash string) (string, error) {
	return s.users[hash], nil
}

func (s *clientAwareStore) FindRefreshGeneration(_ context.Context, hash string) (string, uint64, error) {
	return s.users[hash], s.generations[hash], nil
}

func (s *clientAwareStore) FindRefreshClientID(_ context.Context, hash string) (string, error) {
	return s.clients[hash], nil
}

func (s *clientAwareStore) RevokeRefresh(_ context.Context, hash string) error {
	delete(s.users, hash)
	delete(s.clients, hash)
	delete(s.generations, hash)
	return nil
}

func (s *clientAwareStore) RevokeAllForUser(_ context.Context, userID string) error {
	for hash, id := range s.users {
		if id == userID {
			_ = s.RevokeRefresh(context.Background(), hash)
		}
	}
	return nil
}

func (s *clientAwareStore) RevokeAllForClient(_ context.Context, userID, clientID string) error {
	for hash, id := range s.users {
		if id == userID && s.clients[hash] == clientID {
			_ = s.RevokeRefresh(context.Background(), hash)
		}
	}
	return nil
}

func (s *clientAwareStore) RevokeAllUnmarkedForUser(_ context.Context, userID string) error {
	for hash, id := range s.users {
		if id == userID && s.clients[hash] == "" {
			_ = s.RevokeRefresh(context.Background(), hash)
		}
	}
	return nil
}

func TestClientIDSurvivesRefreshAndLegacySessionCanBeTagged(t *testing.T) {
	mgr := token.NewManager(token.Config{Secret: "secret", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	store := newClientAwareStore()
	svc := session.NewRefreshService(mgr, store, nil)

	legacyPair, err := svc.Issue(context.Background(), claims.AccessClaims{UserID: "legacy-user"})
	require.NoError(t, err)
	upgradedPair, err := svc.RefreshWithClientID(context.Background(), legacyPair.RefreshToken, "crm")
	require.NoError(t, err)
	upgradedClaims, err := mgr.ParseAccess(upgradedPair.AccessToken)
	require.NoError(t, err)
	require.Equal(t, "crm", upgradedClaims.ClientID)

	newPair, err := svc.Issue(context.Background(), claims.AccessClaims{UserID: "native-user", ClientID: "ios"})
	require.NoError(t, err)
	refreshedPair, err := svc.Refresh(context.Background(), newPair.RefreshToken)
	require.NoError(t, err)
	refreshedClaims, err := mgr.ParseAccess(refreshedPair.AccessToken)
	require.NoError(t, err)
	require.Equal(t, "ios", refreshedClaims.ClientID)
}

func TestClientLogoutLeavesOtherClientSessionsActive(t *testing.T) {
	mgr := token.NewManager(token.Config{Secret: "secret", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	store := newClientAwareStore()
	svc := session.NewRefreshService(mgr, store, nil)
	crm, err := svc.Issue(context.Background(), claims.AccessClaims{UserID: "u1", ClientID: "crm"})
	require.NoError(t, err)
	web, err := svc.Issue(context.Background(), claims.AccessClaims{UserID: "u1", ClientID: "web"})
	require.NoError(t, err)

	require.NoError(t, svc.LogoutClient(context.Background(), "u1", "crm"))
	_, err = svc.Refresh(context.Background(), crm.RefreshToken)
	require.ErrorIs(t, err, session.ErrInvalidRefreshToken)
	_, err = svc.Refresh(context.Background(), web.RefreshToken)
	require.NoError(t, err)
}

func TestLogoutUnmarkedSessionsDoesNotRevokeClientSessions(t *testing.T) {
	mgr := token.NewManager(token.Config{Secret: "secret", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	store := newClientAwareStore()
	svc := session.NewRefreshService(mgr, store, nil)
	legacy, err := svc.Issue(context.Background(), claims.AccessClaims{UserID: "u1"})
	require.NoError(t, err)
	crm, err := svc.Issue(context.Background(), claims.AccessClaims{UserID: "u1", ClientID: "crm"})
	require.NoError(t, err)
	web, err := svc.Issue(context.Background(), claims.AccessClaims{UserID: "u1", ClientID: "web"})
	require.NoError(t, err)

	require.NoError(t, svc.LogoutUnmarkedSessions(context.Background(), "u1"))
	_, err = svc.Refresh(context.Background(), legacy.RefreshToken)
	require.ErrorIs(t, err, session.ErrInvalidRefreshToken)
	for _, pair := range []session.TokenPair{crm, web} {
		_, err = svc.Refresh(context.Background(), pair.RefreshToken)
		require.NoError(t, err)
	}
}

func TestClientLogoutRouteKeepsLegacyLogoutSessionScoped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := token.NewManager(token.Config{Secret: "secret", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	store := newClientAwareStore()
	svc := session.NewRefreshService(mgr, store, nil)
	legacy, err := svc.Issue(context.Background(), claims.AccessClaims{UserID: "u1"})
	require.NoError(t, err)
	web, err := svc.Issue(context.Background(), claims.AccessClaims{UserID: "u1", ClientID: "web"})
	require.NoError(t, err)

	router := gin.New()
	session.RegisterRefreshLogout(router.Group("/auth/me"), svc, mgr.Validator(),
		session.WithLogoutScope(session.LogoutClientSessions),
	)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/me/logout", nil)
	req.Header.Set("Authorization", "Bearer "+legacy.AccessToken)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, store.users, session.HashToken(legacy.RefreshToken))
	require.Contains(t, store.users, session.HashToken(web.RefreshToken))
}

func TestClientMetadataAndCredentialGenerationRotateTogether(t *testing.T) {
	mgr := token.NewManager(token.Config{Secret: "secret", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	store := newClientAwareStore()
	svc := session.NewRefreshService(mgr, store, nil).WithGeneration(func(context.Context, string) (uint64, error) {
		return 4, nil
	})
	pair, err := svc.Issue(context.Background(), claims.AccessClaims{UserID: "u1", ClientID: "android"})
	require.NoError(t, err)
	oldHash := session.HashToken(pair.RefreshToken)
	require.Equal(t, uint64(4), store.generations[oldHash])
	require.Equal(t, "android", store.clients[oldHash])

	rotated, err := svc.Refresh(context.Background(), pair.RefreshToken)
	require.NoError(t, err)
	rotatedHash := session.HashToken(rotated.RefreshToken)
	require.Equal(t, uint64(4), store.generations[rotatedHash])
	require.Equal(t, "android", store.clients[rotatedHash])
	parsed, err := mgr.ParseAccess(rotated.AccessToken)
	require.NoError(t, err)
	require.Equal(t, "android", parsed.ClientID)
}

func TestRefreshMigratesLegacyCookieAndIssuesConfiguredClientCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := token.NewManager(token.Config{Secret: "secret", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	store := newClientAwareStore()
	svc := session.NewRefreshService(mgr, store, nil)
	legacy, err := svc.Issue(context.Background(), claims.AccessClaims{UserID: "u1"})
	require.NoError(t, err)

	router := gin.New()
	session.RegisterRefreshLogout(router.Group("/auth/me"), svc, mgr.Validator(),
		session.WithTokenTransport(session.TokenTransport{
			Refresh:           session.DeliverCookie,
			RefreshCookieName: "crm_refresh_token",
		}),
		session.WithClientID("crm"),
	)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/me/refresh", nil)
	req.AddCookie(&http.Cookie{Name: session.DefaultRefreshCookieName, Value: legacy.RefreshToken})
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var rotated *http.Cookie
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "crm_refresh_token" {
			rotated = cookie
		}
	}
	require.NotNil(t, rotated)
	var body struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	rotatedClaims, err := mgr.ParseAccess(body.Data.AccessToken)
	require.NoError(t, err)
	require.Equal(t, "crm", rotatedClaims.ClientID)
}
