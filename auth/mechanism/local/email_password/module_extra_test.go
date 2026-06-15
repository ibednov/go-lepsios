package emailpassword_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	emailpassword "github.com/ibednov/go-lepsios/auth/mechanism/local/email_password"
	"github.com/ibednov/go-lepsios/auth/provider"
	"github.com/ibednov/go-lepsios/auth/session"
	"github.com/ibednov/go-lepsios/auth/token"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := token.NewManager(token.Config{Secret: "s", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	store := &memStore{data: map[string]string{}}
	refresh := session.NewRefreshService(mgr, store, nil)

	mod := emailpassword.New(
		provider.LocalEmailPassword,
		mgr,
		refresh,
		nil,
		func(_ context.Context, c *gin.Context) (emailpassword.VerifiedUser, error) {
			return emailpassword.VerifiedUser{UserID: "u2", Email: "new@b.c"}, nil
		},
	)

	r := gin.New()
	g := r.Group("/auth/email_password")
	mod.RegisterRoutes(g)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/email_password/register", bytes.NewReader([]byte(`{}`)))
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)
}

func TestLoginCookieRefresh(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := token.NewManager(token.Config{Secret: "s", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	store := &memStore{data: map[string]string{}}
	refresh := session.NewRefreshService(mgr, store, nil)

	mod := emailpassword.New(
		provider.LocalEmailPassword,
		mgr,
		refresh,
		func(_ context.Context, _, _ string) (emailpassword.VerifiedUser, error) {
			return emailpassword.VerifiedUser{UserID: "u1"}, nil
		},
		nil,
		emailpassword.WithTokenTransport(emailpassword.TokenTransport{
			Access:  emailpassword.DeliverJSON,
			Refresh: emailpassword.DeliverCookie,
		}),
	)

	r := gin.New()
	mod.RegisterRoutes(r.Group("/auth"))

	body, _ := json.Marshal(map[string]string{"email": "a@b.c", "password": "password1"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	cookies := w.Result().Cookies()
	var found bool
	for _, ck := range cookies {
		if ck.Name == "refresh_token" {
			found = true
			require.True(t, ck.HttpOnly)
		}
	}
	require.True(t, found)
}
