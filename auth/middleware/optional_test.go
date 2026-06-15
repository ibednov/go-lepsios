package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ibednov/go-lepsios/auth/claims"
	"github.com/ibednov/go-lepsios/auth/middleware"
	"github.com/ibednov/go-lepsios/auth/token"
	"github.com/ibednov/go-lepsios/auth/validator"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOptionalNoHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := token.NewManager(token.Config{Secret: "s", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	r := gin.New()
	r.Use(middleware.Optional(mgr.Validator()))
	r.GET("/", func(c *gin.Context) {
		_, ok := claims.PrincipalFromContext(c.Request.Context())
		require.False(t, ok)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, w.Code)
}

func TestOptionalInvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fail := validator.TokenValidatorFunc(func(context.Context, string) (claims.Principal, error) {
		return claims.Principal{}, context.Canceled
	})
	r := gin.New()
	r.Use(middleware.Optional(fail))
	r.GET("/", func(c *gin.Context) {
		_, ok := claims.PrincipalFromContext(c.Request.Context())
		require.False(t, ok)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer bad")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestOptionalValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := token.NewManager(token.Config{Secret: "s", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	raw, err := mgr.SignAccess(context.Background(), claims.AccessClaims{UserID: "u1"})
	require.NoError(t, err)

	r := gin.New()
	r.Use(middleware.Optional(mgr.Validator()))
	r.GET("/", func(c *gin.Context) {
		p, ok := claims.PrincipalFromContext(c.Request.Context())
		require.True(t, ok)
		require.Equal(t, "u1", p.UserID)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}
