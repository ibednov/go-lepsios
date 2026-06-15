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

func TestFromJWTCustomMapper(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := claims.SetPrincipal(c.Request.Context(), claims.Principal{UserID: "u1"})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.Use(identityhttp.FromJWT(identityhttp.WithMapper(func(p claims.Principal) (identity.User, error) {
		return identity.User{ID: p.UserID, Kind: identity.ActorAdmin, Email: "mapped"}, nil
	})))
	r.GET("/", func(c *gin.Context) {
		u, _ := identity.UserFromContext(c.Request.Context())
		c.String(http.StatusOK, u.Email)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, "mapped", w.Body.String())
}

func TestFromJWTPrincipalKind(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := claims.SetPrincipal(c.Request.Context(), claims.Principal{
			UserID:   "u1",
			Provider: provider.Keycloak,
			Kind:     identity.ActorModerator,
		})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.Use(identityhttp.FromJWT())
	r.GET("/", func(c *gin.Context) {
		u, _ := identity.UserFromContext(c.Request.Context())
		c.String(http.StatusOK, string(u.Kind))
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, string(identity.ActorModerator), w.Body.String())
}
