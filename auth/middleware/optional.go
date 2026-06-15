package middleware

import (
	"github.com/ibednov/go-lepsios/auth/claims"
	"github.com/ibednov/go-lepsios/auth/validator"
	"github.com/gin-gonic/gin"
)

// Optional authenticates requests when Bearer JWT is present and valid.
// Missing or invalid token does not block the request.
func Optional(v validator.TokenValidator, opts ...Option) gin.HandlerFunc {
	o := applyOptions(opts)
	return func(c *gin.Context) {
		if shouldSkip(c.Request.URL.Path, o.skipPaths) {
			c.Next()
			return
		}

		raw, ok := extractBearer(c.GetHeader("Authorization"))
		if !ok {
			c.Next()
			return
		}

		principal, err := v.ValidateToken(c.Request.Context(), raw)
		if err != nil {
			c.Next()
			return
		}

		ctx := claims.SetPrincipal(c.Request.Context(), principal)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
