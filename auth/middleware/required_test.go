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

func TestRequiredSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := token.NewManager(token.Config{Secret: "s", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	raw, err := mgr.SignAccess(context.Background(), claims.AccessClaims{UserID: "u1"})
	require.NoError(t, err)

	r := gin.New()
	r.Use(middleware.Required(mgr.Validator()))
	r.GET("/", func(c *gin.Context) {
		p, ok := claims.PrincipalFromContext(c.Request.Context())
		require.True(t, ok)
		c.String(http.StatusOK, p.UserID)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestRequiredMissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := token.NewManager(token.Config{Secret: "s", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	r := gin.New()
	r.Use(middleware.Required(mgr.Validator()))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequiredSkipPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fail := validator.TokenValidatorFunc(func(context.Context, string) (claims.Principal, error) {
		return claims.Principal{}, context.Canceled
	})
	r := gin.New()
	r.Use(middleware.Required(fail, middleware.WithSkipPaths("/public")))
	r.GET("/public", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/public", nil))
	require.Equal(t, http.StatusOK, w.Code)
}
