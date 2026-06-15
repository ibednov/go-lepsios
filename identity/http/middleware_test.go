package identityhttp_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ibednov/go-lepsios/auth/claims"
	"github.com/ibednov/go-lepsios/auth/provider"
	identityhttp "github.com/ibednov/go-lepsios/identity/http"
	"github.com/ibednov/go-lepsios/identity"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestFromJWT(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := claims.SetPrincipal(c.Request.Context(), claims.Principal{
			UserID:   "u1",
			Provider: provider.LocalEmailPassword,
			Email:    "a@b.c",
		})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.Use(identityhttp.FromJWT(identityhttp.WithDefaultKind(identity.ActorCustomer)))
	r.GET("/me", func(c *gin.Context) {
		u, ok := identity.UserFromContext(c.Request.Context())
		require.True(t, ok)
		c.String(http.StatusOK, u.ID)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "u1", w.Body.String())
}

func TestFromJWTNoPrincipal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(identityhttp.FromJWT())
	r.GET("/", func(c *gin.Context) {
		_, ok := identity.UserFromContext(c.Request.Context())
		if ok {
			c.Status(201)
			return
		}
		c.Status(200)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, 200, w.Code)
}
