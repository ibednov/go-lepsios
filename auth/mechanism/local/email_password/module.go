package emailpassword

import (
	"context"
	"net/http"
	"time"

	"github.com/ibednov/go-lepsios/auth/claims"
	"github.com/ibednov/go-lepsios/auth/provider"
	"github.com/ibednov/go-lepsios/auth/session"
	"github.com/ibednov/go-lepsios/auth/token"
	"github.com/ibednov/go-lepsios/httpx/response"
	"github.com/ibednov/go-lepsios/identity"
	"github.com/gin-gonic/gin"
)

// VerifiedUser is returned by credential verification callbacks.
type VerifiedUser struct {
	UserID   string
	Provider provider.ID
	Email    string
	Kind     identity.ActorKind
	Roles    []string
	Plan     string
	Features []string
	User     any // optional service-specific user DTO in responses
}

// CredentialVerifier validates email/password credentials.
type CredentialVerifier func(ctx context.Context, email, password string) (VerifiedUser, error)

// Registrar handles register requests (service binds its own DTO).
type Registrar func(ctx context.Context, c *gin.Context) (VerifiedUser, error)

// LoginRequest is the fixed login DTO.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type options struct {
	transport TokenTransport
}

// Option configures the email/password module.
type Option func(*options)

// WithTokenTransport sets token delivery mode.
func WithTokenTransport(t TokenTransport) Option {
	return func(o *options) {
		o.transport = t
	}
}

// Module registers email/password auth routes.
type Module struct {
	providerID provider.ID
	tokens     *token.Manager
	refresh    *session.RefreshService
	verify     CredentialVerifier
	register   Registrar
	transport  TokenTransport
}

// New creates an email/password auth module.
func New(
	providerID provider.ID,
	tokens *token.Manager,
	refresh *session.RefreshService,
	verify CredentialVerifier,
	register Registrar,
	opts ...Option,
) *Module {
	o := options{
		transport: TokenTransport{Access: DeliverJSON, Refresh: DeliverJSON},
	}
	for _, opt := range opts {
		opt(&o)
	}
	return &Module{
		providerID: providerID,
		tokens:     tokens,
		refresh:    refresh,
		verify:     verify,
		register:   register,
		transport:  o.transport,
	}
}

// RegisterRoutes mounts POST /login and optional POST /register.
func (m *Module) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/login", m.login)
	if m.register != nil {
		rg.POST("/register", m.registerHandler)
	}
}

func (m *Module) login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}

	verified, err := m.verify(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		response.Unauthorized(c, "INVALID_CREDENTIALS", "Invalid email or password")
		return
	}

	m.respondWithTokens(c, http.StatusOK, verified)
}

func (m *Module) registerHandler(c *gin.Context) {
	verified, err := m.register(c.Request.Context(), c)
	if err != nil {
		response.BadRequest(c, "REGISTER_FAILED", err.Error())
		return
	}
	m.respondWithTokens(c, http.StatusCreated, verified)
}

func (m *Module) respondWithTokens(c *gin.Context, status int, verified VerifiedUser) {
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
	}
	if verified.User != nil {
		data["user"] = verified.User
	} else {
		data["user"] = gin.H{"id": verified.UserID, "email": verified.Email}
	}

	switch m.transport.Refresh {
	case DeliverCookie:
		setRefreshCookie(c, pair.RefreshToken, m.tokens.RefreshTTL())
	case DeliverJSON:
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
