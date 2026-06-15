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

func TestLoginJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := token.NewManager(token.Config{Secret: "s", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	store := &memStore{data: map[string]string{}}
	refresh := session.NewRefreshService(mgr, store, nil)

	mod := emailpassword.New(
		provider.LocalEmailPassword,
		mgr,
		refresh,
		func(_ context.Context, email, password string) (emailpassword.VerifiedUser, error) {
			if email == "a@b.c" && password == "password1" {
				return emailpassword.VerifiedUser{UserID: "u1", Email: email}, nil
			}
			return emailpassword.VerifiedUser{}, context.Canceled
		},
		nil,
	)

	r := gin.New()
	g := r.Group("/auth/email_password")
	mod.RegisterRoutes(g)

	body, _ := json.Marshal(map[string]string{"email": "a@b.c", "password": "password1"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/email_password/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "access_token")
}
