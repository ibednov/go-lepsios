package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ibednov/go-lepsios/auth/claims"
	"github.com/ibednov/go-lepsios/auth/validator"
	httpmw "github.com/ibednov/go-lepsios/httpx/middleware"
	"github.com/ibednov/go-lepsios/httpx/response"
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
			unauthorized(c, o, "UNAUTHORIZED", "Authorization header is required")
			return
		}

		principal, err := v.ValidateToken(c.Request.Context(), raw)
		if err != nil {
			unauthorized(c, o, "INVALID_TOKEN", "Invalid or expired token")
			return
		}

		ctx := claims.SetPrincipal(c.Request.Context(), principal)
		c.Request = c.Request.WithContext(ctx)
		c.Set(httpmw.CtxKeyUserID, principal.UserID)
		c.Next()
	}
}

func unauthorized(c *gin.Context, o options, code, defaultMessage string) {
	if o.unauthorizedFn != nil {
		o.unauthorizedFn(c, code)
	} else {
		response.Unauthorized(c, code, defaultMessage)
	}
	c.Abort()
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
