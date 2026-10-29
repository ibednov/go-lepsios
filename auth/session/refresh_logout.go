package session

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ibednov/go-lepsios/auth/validator"
	"github.com/ibednov/go-lepsios/httpx/response"
)

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
		refreshToken, ok := readRefreshToken(c, o.transport.Refresh, o.transport.RefreshCookieName)
		if !ok {
			response.Unauthorized(c, "UNAUTHORIZED", "Refresh token is required")
			return
		}

		pair, err := svc.RefreshWithClientID(c.Request.Context(), refreshToken, o.clientID)
		if err != nil {
			if errors.Is(err, ErrInvalidRefreshToken) {
				response.Unauthorized(c, "INVALID_TOKEN", "Invalid or expired refresh token")
				return
			}
			response.Internal(c, "failed to refresh session")
			return
		}

		data := gin.H{
			"access_token": pair.AccessToken,
			"expires_in":   pair.ExpiresIn,
		}

		switch o.transport.Refresh {
		case DeliverCookie:
			SetRefreshCookie(c, o.transport.RefreshCookieName, pair.RefreshToken, svc.RefreshTTL())
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

		var revokeErr error
		switch o.logoutScope {
		case LogoutAllSessions:
			revokeErr = svc.LogoutUser(c.Request.Context(), principal.UserID)
		case LogoutCurrentSession:
			if principal.SessionID == "" {
				response.Unauthorized(c, "INVALID_TOKEN", "Invalid or expired token")
				return
			}
			revokeErr = svc.LogoutSession(c.Request.Context(), principal.SessionID)
		case LogoutClientSessions:
			if principal.ClientID == "" {
				// Legacy access tokens have no client marker. Restrict their
				// logout to the bound session instead of widening the scope.
				if principal.SessionID == "" {
					response.Unauthorized(c, "INVALID_TOKEN", "Invalid or expired token")
					return
				}
				revokeErr = svc.LogoutSession(c.Request.Context(), principal.SessionID)
			} else {
				revokeErr = svc.LogoutClient(c.Request.Context(), principal.UserID, principal.ClientID)
			}
		case LogoutUnmarkedSessions:
			revokeErr = svc.LogoutUnmarkedSessions(c.Request.Context(), principal.UserID)
		default:
			response.Internal(c, "failed to logout")
			return
		}
		if revokeErr != nil {
			response.Internal(c, "failed to logout")
			return
		}

		if o.transport.Refresh == DeliverCookie {
			ClearRefreshCookie(c, o.transport.RefreshCookieName)
		}

		response.OK(c, nil)
	}
}

func readRefreshToken(c *gin.Context, delivery TokenDelivery, cookieName string) (string, bool) {
	switch delivery {
	case DeliverCookie:
		token, err := c.Cookie(effectiveRefreshCookieName(cookieName))
		if (err != nil || token == "") && cookieName != "" && cookieName != DefaultRefreshCookieName {
			// Accept the old cookie on the first refresh after a client split.
			token, err = c.Cookie(DefaultRefreshCookieName)
		}
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

// SetRefreshCookie writes a secure HTTP-only refresh cookie. Empty cookieName
// retains the legacy name for existing consumers.
func SetRefreshCookie(c *gin.Context, cookieName, refreshToken string, ttl time.Duration) {
	secure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(effectiveRefreshCookieName(cookieName), refreshToken, int(ttl.Seconds()), "/", "", secure, true)
}

// ClearRefreshCookie expires the selected refresh cookie.
func ClearRefreshCookie(c *gin.Context, cookieName string) {
	secure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
	c.SetCookie(effectiveRefreshCookieName(cookieName), "", -1, "/", "", secure, true)
}

func effectiveRefreshCookieName(cookieName string) string {
	if cookieName == "" {
		return DefaultRefreshCookieName
	}
	return cookieName
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
