// Package phone2fa provides phone-based TOTP routes without email transport fields.
package phone2fa

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ibednov/go-lepsios/auth/claims"
	emailpassword "github.com/ibednov/go-lepsios/auth/mechanism/local/email_password"
	"github.com/ibednov/go-lepsios/auth/mechanism/local/phone_password"
	"github.com/ibednov/go-lepsios/auth/provider"
	"github.com/ibednov/go-lepsios/auth/session"
	"github.com/ibednov/go-lepsios/auth/token"
	"github.com/ibednov/go-lepsios/httpx/response"
	"github.com/ibednov/go-lepsios/i18n"
	"github.com/ibednov/go-lepsios/log"
)

const (
	checkFailedCode      = "error.auth.2fa.check_failed"
	generateFailedCode   = "error.auth.2fa.generate_failed"
	verifyFailedCode     = "error.auth.2fa.verify_failed"
	tokenIssueFailedCode = "error.auth.2fa.token_issue_failed"
	commitFailedCode     = "error.auth.2fa.credential_commit_failed"
	cleanupFailedCode    = "error.auth.2fa.session_cleanup_failed"
)

// ErrCredentialCommitConflict marks a one-time credential mutation that lost
// a race after token issuance. The pair must not be disclosed to this caller.
var ErrCredentialCommitConflict = errors.New("phone 2FA credential commit conflict")

// TwoFAStore delegates phone and TOTP business logic to the consuming service.
type TwoFAStore interface {
	Check(ctx context.Context, c *gin.Context) (any, error)
	Generate(ctx context.Context, c *gin.Context) (any, error)
	Verify(ctx context.Context, c *gin.Context) (VerifiedUser, error)
}

// VerifiedUser may defer a one-time credential mutation until a session exists.
// CommitSession runs before the token pair is returned to the client.
type VerifiedUser struct {
	phonepassword.VerifiedUser
	CommitSession func(context.Context) error
	AfterCommit   func(context.Context)
}

type options struct {
	transport phonepassword.TokenTransport
	clientID  string
}

// Option configures token delivery for verified sessions.
type Option func(*options)

// WithTokenTransport sets access and refresh token delivery.
func WithTokenTransport(transport phonepassword.TokenTransport) Option {
	return func(o *options) { o.transport = transport }
}

// WithClientID assigns a server-configured client ID to sessions issued by
// this module. It overrides VerifiedUser.ClientID and is never request input.
func WithClientID(clientID string) Option {
	return func(o *options) { o.clientID = clientID }
}

// Module registers phone-based TOTP routes.
type Module struct {
	providerID provider.ID
	tokens     *token.Manager
	refresh    *session.RefreshService
	store      TwoFAStore
	transport  phonepassword.TokenTransport
	clientID   string
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
		clientID:   o.clientID,
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

func (m *Module) respondWithTokens(c *gin.Context, verified VerifiedUser) {
	providerID := verified.Provider
	if providerID == "" {
		providerID = m.providerID
	}
	pair, err := m.refresh.Issue(c.Request.Context(), claims.AccessClaims{
		UserID:         verified.UserID,
		ClientID:       emailpassword.ResolveClientID(m.clientID, verified.ClientID),
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
	if verified.CommitSession != nil {
		if err := verified.CommitSession(c.Request.Context()); err != nil {
			// The token pair has not been disclosed. Revoke even when the request
			// context was cancelled while committing the credential mutation.
			revokeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if revokeErr := m.refresh.Logout(revokeCtx, pair.RefreshToken); revokeErr != nil {
				logger := log.FromContext(revokeCtx)
				logger.Error().Err(revokeErr).Msg("auth.2fa.session_cleanup_failed")
				response.Error(c, http.StatusInternalServerError, cleanupFailedCode, "Unable to complete 2FA verification.")
				return
			}
			if errors.Is(err, ErrCredentialCommitConflict) {
				response.Unauthorized(c, verifyFailedCode, localizedErrorMessage(c, err, verifyFailedCode, "Unable to verify the 2FA code."))
				return
			}
			logger := log.FromContext(c.Request.Context())
			logger.Error().Err(err).Msg("auth.2fa.credential_commit_failed")
			response.Error(c, http.StatusInternalServerError, commitFailedCode, "Unable to complete 2FA verification.")
			return
		}
	}
	if verified.AfterCommit != nil {
		verified.AfterCommit(c.Request.Context())
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
		session.SetRefreshCookie(c, m.transport.RefreshCookieName, pair.RefreshToken, m.tokens.RefreshTTL())
	case phonepassword.DeliverJSON:
		data["refresh_token"] = pair.RefreshToken
	}
	c.JSON(http.StatusOK, response.SuccessBody{Data: data})
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
