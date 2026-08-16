package telegram

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	emailpassword "github.com/ibednov/go-lepsios/auth/mechanism/local/email_password"
	"github.com/ibednov/go-lepsios/auth/claims"
	"github.com/ibednov/go-lepsios/auth/provider"
	"github.com/ibednov/go-lepsios/auth/session"
	"github.com/ibednov/go-lepsios/auth/token"
	"github.com/ibednov/go-lepsios/auth/validator"
	"github.com/ibednov/go-lepsios/httpx/response"
)

// SessionResult is returned by Store.Session.
type SessionResult struct {
	NeedsLink bool
	Verified  *emailpassword.VerifiedUser
}

// Store delegates Telegram auth business logic to the service.
type Store interface {
	Session(ctx context.Context, c *gin.Context) (SessionResult, error)
	Link(ctx context.Context, c *gin.Context, userID string) (emailpassword.VerifiedUser, error)
	CreateChallenge(ctx context.Context, c *gin.Context) (any, error)
	GetChallenge(ctx context.Context, c *gin.Context) (any, error)
	ApproveChallenge(ctx context.Context, c *gin.Context) (SessionResult, error)
}

type options struct {
	transport emailpassword.TokenTransport
}

// Option configures the telegram auth module.
type Option func(*options)

// WithTokenTransport sets token delivery mode.
func WithTokenTransport(t emailpassword.TokenTransport) Option {
	return func(o *options) {
		o.transport = t
	}
}

// Module registers Telegram auth routes.
type Module struct {
	providerID provider.ID
	tokens     *token.Manager
	refresh    *session.RefreshService
	store      Store
	validator  validator.TokenValidator
	transport  emailpassword.TokenTransport
}

// New creates a Telegram auth module.
func New(
	providerID provider.ID,
	tokens *token.Manager,
	refresh *session.RefreshService,
	store Store,
	tokenValidator validator.TokenValidator,
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
		validator:  tokenValidator,
		transport:  o.transport,
	}
}

// RegisterRoutes mounts Telegram auth endpoints.
//
//	POST /session              — bot assertion → tokens or needs_link
//	POST /link                 — Bearer user + bot assertion → link + tokens
//	POST /challenges           — create login/link challenge (for future web poll)
//	GET  /challenges/:id       — challenge status (poll)
//	POST /challenges/:id/approve — bot assertion approves challenge
func (m *Module) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/session", m.session)
	rg.POST("/link", m.link)
	rg.POST("/challenges", m.createChallenge)
	rg.GET("/challenges/:id", m.getChallenge)
	rg.POST("/challenges/:id/approve", m.approveChallenge)
}

func (m *Module) session(c *gin.Context) {
	result, err := m.store.Session(c.Request.Context(), c)
	if err != nil {
		response.Unauthorized(c, "TELEGRAM_SESSION_FAILED", err.Error())
		return
	}
	if result.NeedsLink || result.Verified == nil {
		response.OK(c, gin.H{"status": "needs_link"})
		return
	}
	m.respondWithTokens(c, http.StatusOK, *result.Verified)
}

func (m *Module) link(c *gin.Context) {
	userID, err := m.userIDFromBearer(c)
	if err != nil {
		response.Unauthorized(c, "UNAUTHORIZED", "Bearer access token required")
		return
	}
	verified, err := m.store.Link(c.Request.Context(), c, userID)
	if err != nil {
		response.BadRequest(c, "TELEGRAM_LINK_FAILED", err.Error())
		return
	}
	m.respondWithTokens(c, http.StatusOK, verified)
}

func (m *Module) createChallenge(c *gin.Context) {
	result, err := m.store.CreateChallenge(c.Request.Context(), c)
	if err != nil {
		response.BadRequest(c, "TELEGRAM_CHALLENGE_CREATE_FAILED", err.Error())
		return
	}
	response.Created(c, result)
}

func (m *Module) getChallenge(c *gin.Context) {
	result, err := m.store.GetChallenge(c.Request.Context(), c)
	if err != nil {
		response.BadRequest(c, "TELEGRAM_CHALLENGE_GET_FAILED", err.Error())
		return
	}
	response.OK(c, result)
}

func (m *Module) approveChallenge(c *gin.Context) {
	result, err := m.store.ApproveChallenge(c.Request.Context(), c)
	if err != nil {
		response.Unauthorized(c, "TELEGRAM_CHALLENGE_APPROVE_FAILED", err.Error())
		return
	}
	if result.NeedsLink || result.Verified == nil {
		response.OK(c, gin.H{"status": "needs_link"})
		return
	}
	m.respondWithTokens(c, http.StatusOK, *result.Verified)
}

func (m *Module) userIDFromBearer(c *gin.Context) (string, error) {
	raw := c.GetHeader("Authorization")
	const prefix = "Bearer "
	if len(raw) <= len(prefix) || raw[:len(prefix)] != prefix {
		return "", http.ErrNoCookie
	}
	principal, err := m.validator.ValidateToken(c.Request.Context(), raw[len(prefix):])
	if err != nil {
		return "", err
	}
	return principal.UserID, nil
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
		Plan:     verified.Plan,
		Features: verified.Features,
	}

	pair, err := m.refresh.Issue(c.Request.Context(), accessClaims)
	if err != nil {
		response.Internal(c, "failed to issue tokens")
		return
	}

	data := gin.H{
		"access_token": pair.AccessToken,
		"expires_in":   pair.ExpiresIn,
		"user":         gin.H{"id": verified.UserID, "email": verified.Email},
	}
	if verified.User != nil {
		data["user"] = verified.User
	}

	switch m.transport.Refresh {
	case emailpassword.DeliverCookie:
		setRefreshCookie(c, pair.RefreshToken, m.tokens.RefreshTTL())
	case emailpassword.DeliverJSON:
		data["refresh_token"] = pair.RefreshToken
	}

	c.JSON(status, response.SuccessBody{Data: data})
}

func setRefreshCookie(c *gin.Context, refreshToken string, ttl time.Duration) {
	secure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("refresh_token", refreshToken, int(ttl.Seconds()), "/", "", secure, true)
}
