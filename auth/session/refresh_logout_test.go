package session_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ibednov/go-lepsios/auth/claims"
	"github.com/ibednov/go-lepsios/auth/session"
	"github.com/ibednov/go-lepsios/auth/token"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterRefreshLogoutCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := token.NewManager(token.Config{Secret: "s", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	store := &memStore{entries: map[string]string{}}
	svc := session.NewRefreshService(mgr, store, nil)

	pair, err := svc.Issue(context.Background(), claims.AccessClaims{UserID: "u1"})
	require.NoError(t, err)

	r := gin.New()
	g := r.Group("/auth/me")
	session.RegisterRefreshLogout(g, svc, mgr.Validator(),
		session.WithTokenTransport(session.TokenTransport{Refresh: session.DeliverCookie}),
	)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/me/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: pair.RefreshToken})
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "access_token")

	accessRaw, err := mgr.SignAccess(context.Background(), claims.AccessClaims{UserID: "u1"})
	require.NoError(t, err)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/auth/me/logout", nil)
	req.Header.Set("Authorization", "Bearer "+accessRaw)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Empty(t, store.entries)
}
