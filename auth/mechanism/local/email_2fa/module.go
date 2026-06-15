package email2fa

import (
	"context"
	"net/http"
	"time"

	emailpassword "github.com/ibednov/go-lepsios/auth/mechanism/local/email_password"
	"github.com/ibednov/go-lepsios/auth/claims"
	"github.com/ibednov/go-lepsios/auth/provider"
	"github.com/ibednov/go-lepsios/auth/session"
	"github.com/ibednov/go-lepsios/auth/token"
	"github.com/ibednov/go-lepsios/httpx/response"
	"github.com/gin-gonic/gin"
)

// TwoFAStore delegates 2FA business logic to the service.
type TwoFAStore interface {
	Check(ctx context.Context, c *gin.Context) (any, error)
	Generate(ctx context.Context, c *gin.Context) (any, error)
	Verify(ctx context.Context, c *gin.Context) (emailpassword.VerifiedUser, error)
}

type options struct {
	transport emailpassword.TokenTransport
}

// Option configures the email/2FA module.
type Option func(*options)

// WithTokenTransport sets token delivery mode.
func WithTokenTransport(t emailpassword.TokenTransport) Option {
	return func(o *options) {
		o.transport = t
	}
}

// Module registers email/2FA auth routes.
type Module struct {
	providerID provider.ID
	tokens     *token.Manager
	refresh    *session.RefreshService
	store      TwoFAStore
	transport  emailpassword.TokenTransport
}

// New creates an email/2FA auth module.
func New(
	providerID provider.ID,
	tokens *token.Manager,
	refresh *session.RefreshService,
	store TwoFAStore,
	opts ...Option,
) *Module {
	o := options{
		transport: emailpassword.TokenTransport{
			Access:  emailpassword.DeliverJSON,
			Refresh: emailpassword.DeliverJSON,
		},
	}
	for _, opt := range opts {
		opt(&o)
	}
	return &Module{
		providerID: providerID,
		tokens:     tokens,
		refresh:    refresh,
		store:      store,
		transport:  o.transport,
	}
}

// RegisterRoutes mounts GET /check, POST /generate, POST /verify.
func (m *Module) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/check", m.check)
	rg.POST("/generate", m.generate)
	rg.POST("/verify", m.verify)
}

func (m *Module) check(c *gin.Context) {
	result, err := m.store.Check(c.Request.Context(), c)
	if err != nil {
		response.BadRequest(c, "CHECK_FAILED", err.Error())
		return
	}
	response.OK(c, result)
}

func (m *Module) generate(c *gin.Context) {
	result, err := m.store.Generate(c.Request.Context(), c)
	if err != nil {
		response.BadRequest(c, "GENERATE_FAILED", err.Error())
		return
	}
	response.OK(c, result)
}

func (m *Module) verify(c *gin.Context) {
	verified, err := m.store.Verify(c.Request.Context(), c)
	if err != nil {
		response.Unauthorized(c, "VERIFY_FAILED", err.Error())
		return
	}
	m.respondWithTokens(c, http.StatusOK, verified)
}

func (m *Module) respondWithTokens(c *gin.Context, status int, verified emailpassword.VerifiedUser) {
	providerID := verified.Provider
	if providerID == "" {
		providerID = m.providerID
	}

	accessClaims := claims.AccessClaims{
		UserID:   verified.UserID,
		Provider: providerID,
		Kind:     verified.Kind,
		Email:    verified.Email,
		Roles:    verified.Roles,
	}

	pair, err := m.refresh.Issue(c.Request.Context(), accessClaims)
	if err != nil {
		response.Internal(c, "failed to issue tokens")
		return
	}

	data := gin.H{
		"access_token": pair.AccessToken,
		"expires_in":   pair.ExpiresIn,
	}
	switch {
	case verified.User == nil:
		data["user"] = gin.H{"id": verified.UserID, "email": verified.Email}
	case flattenUserData(data, verified.User):
	default:
		data["user"] = verified.User
	}

	switch m.transport.Refresh {
	case emailpassword.DeliverCookie:
		setRefreshCookie(c, pair.RefreshToken, m.tokens.RefreshTTL())
	case emailpassword.DeliverJSON:
		data["refresh_token"] = pair.RefreshToken
	}

	if status == http.StatusCreated {
		response.Created(c, data)
		return
	}
	c.JSON(status, response.SuccessBody{Data: data})
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
	if bc, ok := m["backup_codes"]; ok {
		data["backup_codes"] = bc
	}
	return true
}
