package email2fa_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	emailpassword "github.com/ibednov/go-lepsios/auth/mechanism/local/email_password"
	email2fa "github.com/ibednov/go-lepsios/auth/mechanism/local/email_2fa"
	"github.com/ibednov/go-lepsios/auth/provider"
	"github.com/ibednov/go-lepsios/auth/session"
	"github.com/ibednov/go-lepsios/auth/token"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type memStore struct {
	data map[string]string
}

func (m *memStore) SaveRefresh(_ context.Context, h, uid string, _ time.Time) error {
	m.data[h] = uid
	return nil
}
func (m *memStore) FindUserByRefresh(context.Context, string) (string, error) { return "", nil }
func (m *memStore) RevokeRefresh(context.Context, string) error              { return nil }
func (m *memStore) RevokeAllForUser(context.Context, string) error           { return nil }

type twoFAStore struct{}

func (twoFAStore) Check(_ context.Context, _ *gin.Context) (any, error) {
	return gin.H{"enabled": true}, nil
}

func (twoFAStore) Generate(_ context.Context, _ *gin.Context) (any, error) {
	return gin.H{"secret": "ABC"}, nil
}

func (twoFAStore) Verify(_ context.Context, _ *gin.Context) (emailpassword.VerifiedUser, error) {
	return emailpassword.VerifiedUser{UserID: "u1", Email: "a@b.c"}, nil
}

func TestVerifyIssuesTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := token.NewManager(token.Config{Secret: "s", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	store := &memStore{data: map[string]string{}}
	refresh := session.NewRefreshService(mgr, store, nil)

	mod := email2fa.New(provider.LocalEmail2FA, mgr, refresh, twoFAStore{})

	r := gin.New()
	mod.RegisterRoutes(r.Group("/auth/email_2fa"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/email_2fa/verify", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "access_token")
}

func TestCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := token.NewManager(token.Config{Secret: "s", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	refresh := session.NewRefreshService(mgr, &memStore{data: map[string]string{}}, nil)
	mod := email2fa.New(provider.LocalEmail2FA, mgr, refresh, twoFAStore{})

	r := gin.New()
	mod.RegisterRoutes(r.Group("/auth/email_2fa"))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/auth/email_2fa/check", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, true, body.Data["enabled"])
}
