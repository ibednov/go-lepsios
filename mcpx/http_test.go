package mcpx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

const (
	testKeyIDHeader = "X-Test-Key-ID"
	testKeyHeader   = "X-Test-Key"
)

type stubAuthenticator struct {
	ok  bool
	err error
	id  Identity
}

func (s stubAuthenticator) Authenticate(_ context.Context, _, _ string) (Identity, bool, error) {
	return s.id, s.ok, s.err
}

func newServer(_ context.Context, _ Identity) *mcp.Server {
	return mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1.0.0"}, nil)
}

func baseMiddleware(a Authenticator) Middleware {
	return Middleware{
		KeyIDHeader:      testKeyIDHeader,
		KeyHeader:        testKeyHeader,
		Authenticator:    a,
		UnauthorizedCode: "unauthorized",
	}
}

func TestNewHandlerRejectsMissingHeaders(t *testing.T) {
	t.Parallel()
	h := NewHandler(HandlerConfig{MaxBodyBytes: 1 << 20, NewServer: newServer},
		baseMiddleware(stubAuthenticator{ok: false}))

	// no headers
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusUnauthorized, rr.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.Equal(t, "unauthorized", body["error"])

	// partial headers (missing secret)
	req = httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set(testKeyIDHeader, "some-id")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestNewHandlerUnauthorizedWhenAuthReturnsNotOK(t *testing.T) {
	t.Parallel()
	h := NewHandler(HandlerConfig{MaxBodyBytes: 1 << 20, NewServer: newServer},
		baseMiddleware(stubAuthenticator{ok: false}))

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set(testKeyIDHeader, "some-id")
	req.Header.Set(testKeyHeader, "secret")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusUnauthorized, rr.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.Equal(t, "unauthorized", body["error"])
}

func TestWriteError(t *testing.T) {
	t.Parallel()
	rr := httptest.NewRecorder()
	WriteError(rr, http.StatusTooManyRequests, "rate_limited")
	require.Equal(t, http.StatusTooManyRequests, rr.Code)
	require.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	var body map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.Equal(t, "rate_limited", body["error"])
}

func TestNewHandlerPreAuthCheckRejects(t *testing.T) {
	t.Parallel()
	var gotKeyID string
	mw := baseMiddleware(stubAuthenticator{ok: true})
	mw.PreAuth = []Check{
		func(_ context.Context, keyID string) *HTTPError {
			gotKeyID = keyID
			return &HTTPError{Status: http.StatusForbidden, Code: "disabled"}
		},
	}
	h := NewHandler(HandlerConfig{MaxBodyBytes: 1 << 20, NewServer: newServer}, mw)

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set(testKeyIDHeader, "some-id")
	req.Header.Set(testKeyHeader, "secret")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusForbidden, rr.Code)
	// The pre-auth hook receives the raw key id so products can guard its format.
	require.Equal(t, "some-id", gotKeyID)
	var body map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.Equal(t, "disabled", body["error"])
}

func TestNewHandlerPostAuthCheckRejects(t *testing.T) {
	t.Parallel()
	mw := baseMiddleware(stubAuthenticator{ok: true, id: Identity{Scope: ScopeRead, Subject: "u1"}})
	mw.PostAuth = []IdentityCheck{
		func(context.Context, Identity) *HTTPError {
			return &HTTPError{Status: http.StatusTooManyRequests, Code: "rate_limited"}
		},
	}
	h := NewHandler(HandlerConfig{MaxBodyBytes: 1 << 20, NewServer: newServer}, mw)

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set(testKeyIDHeader, "some-id")
	req.Header.Set(testKeyHeader, "secret")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusTooManyRequests, rr.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.Equal(t, "rate_limited", body["error"])
}

func TestNewHandlerAuthErrorYields500(t *testing.T) {
	t.Parallel()
	var captured error
	var capturedKeyID string
	mw := baseMiddleware(stubAuthenticator{err: context.DeadlineExceeded})
	mw.AuthErrorCode = "authentication_failed"
	mw.OnError = func(keyID string, err error) {
		capturedKeyID = keyID
		captured = err
	}
	h := NewHandler(HandlerConfig{MaxBodyBytes: 1 << 20, NewServer: newServer}, mw)

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set(testKeyIDHeader, "some-id")
	req.Header.Set(testKeyHeader, "secret")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusInternalServerError, rr.Code)
	require.Equal(t, context.DeadlineExceeded, captured)
	require.Equal(t, "some-id", capturedKeyID)
	var body map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.Equal(t, "authentication_failed", body["error"])
}
