package mcpx

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// WriteError writes a JSON error body {"error": code} with the given status.
func WriteError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

// HTTPError is a precheck rejection: a status code plus a stable machine code.
// Check/IdentityCheck hooks return it (non-nil) to reject a request.
type HTTPError struct {
	Status int
	Code   string
}

// Authenticator verifies a key id + secret and resolves the Identity. ok=false
// means the credentials were rejected (unauthorized), which is distinct from a
// non-nil err (a server-side failure while authenticating).
type Authenticator interface {
	Authenticate(ctx context.Context, keyID, secret string) (Identity, bool, error)
}

// Check is a pre-auth hook (for example a global feature flag or a key-id format
// guard). It runs before authentication and receives the raw key id, so a product
// can reject malformed credentials ahead of heavier checks; returning a non-nil
// HTTPError rejects the request.
type Check func(ctx context.Context, keyID string) *HTTPError

// IdentityCheck is a post-auth hook (for example a per-subject quota). It runs
// after a successful Authenticate; returning a non-nil HTTPError rejects.
type IdentityCheck func(ctx context.Context, id Identity) *HTTPError

// Middleware authenticates MCP requests by header credentials and runs ordered
// pre/post-auth checks, placing the resolved Identity on the request context.
type Middleware struct {
	KeyIDHeader      string
	KeyHeader        string
	Authenticator    Authenticator
	UnauthorizedCode string
	AuthErrorCode    string
	PreAuth          []Check
	PostAuth         []IdentityCheck
	OnError          func(keyID string, err error)
}

func (m Middleware) unauthorizedCode() string {
	if m.UnauthorizedCode == "" {
		return "unauthorized"
	}
	return m.UnauthorizedCode
}

func (m Middleware) authErrorCode() string {
	if m.AuthErrorCode == "" {
		return "authentication_failed"
	}
	return m.AuthErrorCode
}

// Wrap authenticates each request before delegating to next. Order: read & trim
// the key id and secret headers (either empty => 401); run each PreAuth check
// (first non-nil rejects); authenticate (err => 500 via OnError+AuthErrorCode,
// !ok => 401); run each PostAuth check with the identity (first non-nil rejects);
// store the identity on the context and serve next.
func (m Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keyID := strings.TrimSpace(r.Header.Get(m.KeyIDHeader))
		secret := r.Header.Get(m.KeyHeader)
		if keyID == "" || secret == "" {
			WriteError(w, http.StatusUnauthorized, m.unauthorizedCode())
			return
		}
		ctx := r.Context()
		for _, check := range m.PreAuth {
			if herr := check(ctx, keyID); herr != nil {
				WriteError(w, herr.Status, herr.Code)
				return
			}
		}
		id, ok, err := m.Authenticator.Authenticate(ctx, keyID, secret)
		if err != nil {
			if m.OnError != nil {
				m.OnError(keyID, err)
			}
			WriteError(w, http.StatusInternalServerError, m.authErrorCode())
			return
		}
		if !ok {
			WriteError(w, http.StatusUnauthorized, m.unauthorizedCode())
			return
		}
		for _, check := range m.PostAuth {
			if herr := check(ctx, id); herr != nil {
				WriteError(w, herr.Status, herr.Code)
				return
			}
		}
		ctx = ContextWithIdentity(ctx, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// HandlerConfig configures the stateless Streamable HTTP handler. MaxBodyBytes,
// when > 0, caps the request body; NewServer builds the per-request mcp.Server
// for the authenticated Identity.
type HandlerConfig struct {
	MaxBodyBytes int64
	NewServer    func(ctx context.Context, id Identity) *mcp.Server
}

// NewHandler composes the stateless Streamable HTTP MCP handler with an optional
// request body limit and the auth Middleware. The per-request mcp.Server is built
// from the Identity the Middleware placed on the context.
//
// NewHandler intentionally does NOT include a rate limiter: a process-local RPS
// limiter is product-specific orchestration and stays OUTSIDE this handler, so a
// product composes it by further wrapping the returned handler with its own
// middleware exactly as before (body-limit + auth + MCP handler are all mcpx
// provides here).
func NewHandler(cfg HandlerConfig, mw Middleware) http.Handler {
	mcpHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		id, _ := IdentityFromContext(r.Context())
		return cfg.NewServer(r.Context(), id)
	}, &mcp.StreamableHTTPOptions{Stateless: true})

	limited := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cfg.MaxBodyBytes > 0 {
			r.Body = http.MaxBytesReader(w, r.Body, cfg.MaxBodyBytes)
		}
		mcpHandler.ServeHTTP(w, r)
	})

	return mw.Wrap(limited)
}
