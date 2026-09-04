package middleware

import (
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
	"github.com/ibednov/go-lepsios/httpx/response"
	"github.com/ibednov/go-lepsios/identity"
)

const (
	// MessageKeyForbiddenRole is returned when the actor lacks a required role.
	MessageKeyForbiddenRole = "AUTH.FORBIDDEN.ROLE"
	// MessageKeyForbiddenFeature is returned when the actor lacks a required feature.
	MessageKeyForbiddenFeature = "AUTH.FORBIDDEN.FEATURE"
)

// RequireRole blocks the request unless the authenticated actor has the role.
// Must run after Required (or Optional) and identity mapping in the chain.
func RequireRole(role string, opts ...Option) gin.HandlerFunc {
	o := applyOptions(opts)
	return func(c *gin.Context) {
		if shouldSkip(c.Request.URL.Path, o.skipPaths) {
			c.Next()
			return
		}
		user, ok := identity.UserFromContext(c.Request.Context())
		if !ok || !slices.Contains(user.Roles, role) {
			response.Error(c, http.StatusForbidden, MessageKeyForbiddenRole, "forbidden")
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireFeature blocks the request unless the authenticated actor has the
// feature flag. Must run after Required (or Optional) and identity mapping.
func RequireFeature(feature string, opts ...Option) gin.HandlerFunc {
	o := applyOptions(opts)
	return func(c *gin.Context) {
		if shouldSkip(c.Request.URL.Path, o.skipPaths) {
			c.Next()
			return
		}
		user, ok := identity.UserFromContext(c.Request.Context())
		if !ok || !slices.Contains(user.Features, feature) {
			response.Error(c, http.StatusForbidden, MessageKeyForbiddenFeature, "forbidden")
			c.Abort()
			return
		}
		c.Next()
	}
}