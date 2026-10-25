package phonepassword

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ibednov/go-lepsios/auth/claims"
	"github.com/ibednov/go-lepsios/auth/provider"
	"github.com/ibednov/go-lepsios/auth/session"
	"github.com/ibednov/go-lepsios/auth/token"
	"github.com/ibednov/go-lepsios/httpx/response"
)

// LoginRequest is the phone-only password login transport. It intentionally
// has no email field so generated schemas and JSON payloads remain phone-only.
type LoginRequest struct {
	Phone    string `json:"phone" binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
}

// Normalize trims transport whitespace without assuming a default phone region.
func (req *LoginRequest) Normalize() {
	if req == nil {
		return
	}
	req.Phone = strings.TrimSpace(req.Phone)
}

// Validate checks the fields required by the generic phone/password transport.
// Phone number parsing and region policy belong to the consuming service.
func (req LoginRequest) Validate() error {
	req.Normalize()
	if req.Phone == "" {
		return errPhoneRequired
	}
	if req.Password == "" {
		return errPasswordRequired
	}
	if len(req.Password) < 8 {
		return errPasswordTooShort
	}
	return nil
}

// CredentialVerifier validates phone/password credentials in the consuming service.
type CredentialVerifier func(context.Context, LoginRequest) (VerifiedUser, error)

// Registrar binds and validates the service-specific phone registration request.
type Registrar func(context.Context, *gin.Context) (VerifiedUser, error)

type Module struct {
	providerID provider.ID
	tokens     *token.Manager
	refresh    *session.RefreshService
	verify     CredentialVerifier
	register   Registrar
	transport  TokenTransport
}

// New creates a phone-only local password authentication module.
func New(
	providerID provider.ID,
	tokens *token.Manager,
	refresh *session.RefreshService,
	verify CredentialVerifier,
	register Registrar,
	opts ...Option,
) *Module {
	o := options{transport: TokenTransport{Access: DeliverJSON, Refresh: DeliverJSON}}
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
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
	req.Normalize()
	if err := req.Validate(); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}
	if m.verify == nil {
		response.Internal(c, "phone password verifier is not configured")
		return
	}

	verified, err := m.verify(c.Request.Context(), req)
	if err != nil {
		response.Unauthorized(c, "INVALID_CREDENTIALS", "Invalid credentials")
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
	pair, err := m.refresh.Issue(c.Request.Context(), claims.AccessClaims{
		UserID:   verified.UserID,
		Provider: providerID,
		Kind:     verified.Kind,
		Roles:    verified.Roles,
		Plan:     verified.Plan,
		Features: verified.Features,
	})
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
		data["user"] = gin.H{"id": verified.UserID}
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
