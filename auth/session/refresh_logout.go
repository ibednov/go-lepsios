package session

import (
	"net/http"
	"time"

	"github.com/ibednov/go-lepsios/auth/validator"
	"github.com/ibednov/go-lepsios/httpx/response"
	"github.com/gin-gonic/gin"
)

const refreshCookieName = "refresh_token"

// RegisterRefreshLogout mounts POST /refresh and POST /logout.
func RegisterRefreshLogout(
	rg *gin.RouterGroup,
	svc *RefreshService,
	v validator.TokenValidator,
	opts ...RefreshLogoutOption,
) {
	o := applyRefreshLogoutOptions(opts)
	rg.POST("/refresh", refreshHandler(svc, o))
	rg.POST("/logout", logoutHandler(svc, v, o))
}

func refreshHandler(svc *RefreshService, o refreshLogoutOptions) gin.HandlerFunc {
	return func(c *gin.Context) {
		refreshToken, ok := readRefreshToken(c, o.transport.Refresh)
		if !ok {
			response.Unauthorized(c, "UNAUTHORIZED", "Refresh token is required")
			return
		}

		pair, err := svc.Refresh(c.Request.Context(), refreshToken)
		if err != nil {
			response.Unauthorized(c, "INVALID_TOKEN", "Invalid or expired refresh token")
			return
		}

		data := gin.H{
			"access_token": pair.AccessToken,
			"expires_in":   pair.ExpiresIn,
		}

		switch o.transport.Refresh {
		case DeliverCookie:
			setRefreshCookie(c, pair.RefreshToken, svc.RefreshTTL())
		case DeliverJSON:
			data["refresh_token"] = pair.RefreshToken
		}

		response.OK(c, data)
	}
}

func logoutHandler(svc *RefreshService, v validator.TokenValidator, o refreshLogoutOptions) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, ok := extractBearer(c.GetHeader("Authorization"))
		if !ok {
			response.Unauthorized(c, "UNAUTHORIZED", "Authorization header is required")
			return
		}

		principal, err := v.ValidateToken(c.Request.Context(), raw)
		if err != nil {
			response.Unauthorized(c, "INVALID_TOKEN", "Invalid or expired token")
			return
		}

		if err := svc.LogoutUser(c.Request.Context(), principal.UserID); err != nil {
			response.Internal(c, "failed to logout")
			return
		}

		if o.transport.Refresh == DeliverCookie {
			clearRefreshCookie(c)
		}

		response.OK(c, nil)
	}
}

func readRefreshToken(c *gin.Context, delivery TokenDelivery) (string, bool) {
	switch delivery {
	case DeliverCookie:
		token, err := c.Cookie(refreshCookieName)
		return token, err == nil && token != ""
	case DeliverJSON:
		var body struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			return "", false
		}
		return body.RefreshToken, body.RefreshToken != ""
	default:
		return "", false
	}
}

func setRefreshCookie(c *gin.Context, refreshToken string, ttl time.Duration) {
	secure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(refreshCookieName, refreshToken, int(ttl.Seconds()), "/", "", secure, true)
}

func clearRefreshCookie(c *gin.Context) {
	secure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
	c.SetCookie(refreshCookieName, "", -1, "/", "", secure, true)
}

func extractBearer(header string) (string, bool) {
	if header == "" {
		return "", false
	}
	const prefix = "Bearer "
	if len(header) <= len(prefix) || header[:len(prefix)] != prefix {
		return "", false
	}
	token := header[len(prefix):]
	return token, token != ""
}
