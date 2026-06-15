package middleware

import (
	"strings"

	"github.com/ibednov/go-lepsios/auth/claims"
	"github.com/ibednov/go-lepsios/auth/validator"
	"github.com/ibednov/go-lepsios/httpx/response"
	"github.com/gin-gonic/gin"
)

// Required authenticates requests via Bearer JWT.
func Required(v validator.TokenValidator, opts ...Option) gin.HandlerFunc {
	o := applyOptions(opts)
	return func(c *gin.Context) {
		if shouldSkip(c.Request.URL.Path, o.skipPaths) {
			c.Next()
			return
		}

		raw, ok := extractBearer(c.GetHeader("Authorization"))
		if !ok {
			response.Unauthorized(c, "UNAUTHORIZED", "Authorization header is required")
			c.Abort()
			return
		}

		principal, err := v.ValidateToken(c.Request.Context(), raw)
		if err != nil {
			response.Unauthorized(c, "INVALID_TOKEN", "Invalid or expired token")
			c.Abort()
			return
		}

		ctx := claims.SetPrincipal(c.Request.Context(), principal)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func extractBearer(header string) (string, bool) {
	if header == "" {
		return "", false
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}
