// Package phone2fa provides phone-based TOTP routes without email transport fields.
package phone2fa

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ibednov/go-lepsios/auth/claims"
	"github.com/ibednov/go-lepsios/auth/mechanism/local/phone_password"
	"github.com/ibednov/go-lepsios/auth/provider"
	"github.com/ibednov/go-lepsios/auth/session"
	"github.com/ibednov/go-lepsios/auth/token"
	"github.com/ibednov/go-lepsios/httpx/response"
	"github.com/ibednov/go-lepsios/i18n"
)

const (
	checkFailedCode      = "error.auth.2fa.check_failed"
	generateFailedCode   = "error.auth.2fa.generate_failed"
	verifyFailedCode     = "error.auth.2fa.verify_failed"
	tokenIssueFailedCode = "error.auth.2fa.token_issue_failed"
)

// TwoFAStore delegates phone and TOTP business logic to the consuming service.
type TwoFAStore interface {
	Check(ctx context.Context, c *gin.Context) (any, error)
	Generate(ctx context.Context, c *gin.Context) (any, error)
	Verify(ctx context.Context, c *gin.Context) (phonepassword.VerifiedUser, error)
}

type options struct {
	transport phonepassword.TokenTransport
}

// Option configures token delivery for verified sessions.
type Option func(*options)

// WithTokenTransport sets access and refresh token delivery.
func WithTokenTransport(transport phonepassword.TokenTransport) Option {
	return func(o *options) { o.transport = transport }
}

// Module registers phone-based TOTP routes.
type Module struct {
	providerID provider.ID
	tokens     *token.Manager
	refresh    *session.RefreshService
	store      TwoFAStore
	transport  phonepassword.TokenTransport
}

// New creates a phone-based TOTP authentication module.
func New(
	providerID provider.ID,
	tokens *token.Manager,
	refresh *session.RefreshService,
	store TwoFAStore,
	opts ...Option,
) *Module {
	o := options{transport: phonepassword.TokenTransport{
		Access:  phonepassword.DeliverJSON,
		Refresh: phonepassword.DeliverJSON,
	}}
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	return &Module{
		providerID: providerID,
		tokens:     tokens,
		refresh:    refresh,
		store:      store,
		transport:  o.transport,
	}
}

// RegisterRoutes mounts GET /check, POST /generate, and POST /verify.
func (m *Module) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/check", m.check)
	rg.POST("/generate", m.generate)
	rg.POST("/verify", m.verify)
}

func (m *Module) check(c *gin.Context) {
	result, err := m.store.Check(c.Request.Context(), c)
	if err != nil {
		response.BadRequest(c, checkFailedCode, localizedErrorMessage(c, err, checkFailedCode, "Unable to check 2FA."))
		return
	}
	response.OK(c, result)
}

func (m *Module) generate(c *gin.Context) {
	result, err := m.store.Generate(c.Request.Context(), c)
	if err != nil {
		response.BadRequest(c, generateFailedCode, localizedErrorMessage(c, err, generateFailedCode, "Unable to generate a 2FA code."))
		return
	}
	response.OK(c, result)
}

func (m *Module) verify(c *gin.Context) {
	verified, err := m.store.Verify(c.Request.Context(), c)
	if err != nil {
		response.Unauthorized(c, verifyFailedCode, localizedErrorMessage(c, err, verifyFailedCode, "Unable to verify the 2FA code."))
		return
	}
	m.respondWithTokens(c, verified)
}

func localizedErrorMessage(c *gin.Context, err error, fallbackKey, fallbackMessage string) string {
	var localizer *i18n.Localizer
	if c != nil && c.Request != nil {
		localizer = i18n.LocalizerFromContext(c.Request.Context())
	}
	var keyedError interface{ GetMessageKey() string }
	if err != nil && errors.As(err, &keyedError) && keyedError.GetMessageKey() != "" && localizer != nil {
		key := keyedError.GetMessageKey()
		localized := localizer.T(key)
		if localized != "" && localized != key {
			return localized
		}
	}
	if localizer != nil {
		localized := localizer.T(fallbackKey)
		if localized != "" && localized != fallbackKey {
			return localized
		}
	}
	return fallbackMessage
}

func (m *Module) respondWithTokens(c *gin.Context, verified phonepassword.VerifiedUser) {
	providerID := verified.Provider
	if providerID == "" {
		providerID = m.providerID
	}
	pair, err := m.refresh.Issue(c.Request.Context(), claims.AccessClaims{
		UserID:         verified.UserID,
		AuthGeneration: verified.AuthGeneration,
		Provider:       providerID,
		Kind:           verified.Kind,
		Roles:          verified.Roles,
		Plan:           verified.Plan,
		Features:       verified.Features,
	})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, tokenIssueFailedCode, localizedErrorMessage(c, nil, tokenIssueFailedCode, "Unable to start a session after 2FA verification."))
		return
	}

	data := gin.H{
		"access_token": pair.AccessToken,
		"expires_in":   pair.ExpiresIn,
	}
	switch {
	case verified.User == nil:
		data["user"] = gin.H{"id": verified.UserID}
	case flattenUserData(data, verified.User):
	default:
		data["user"] = verified.User
	}
	switch m.transport.Refresh {
	case phonepassword.DeliverCookie:
		setRefreshCookie(c, pair.RefreshToken, m.tokens.RefreshTTL())
	case phonepassword.DeliverJSON:
		data["refresh_token"] = pair.RefreshToken
	}
	c.JSON(http.StatusOK, response.SuccessBody{Data: data})
}

func setRefreshCookie(c *gin.Context, refreshToken string, ttl time.Duration) {
	secure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("refresh_token", refreshToken, int(ttl.Seconds()), "/", "", secure, true)
}

func flattenUserData(data gin.H, user any) bool {
	m, ok := user.(map[string]any)
	if !ok {
		return false
	}
	if u, ok := m["user"]; ok {
		data["user"] = u
	}
	if codes, ok := m["backup_codes"]; ok {
		data["backup_codes"] = codes
	}
	return true
}
