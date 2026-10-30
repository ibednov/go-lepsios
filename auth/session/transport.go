package session

// TokenDelivery defines where tokens are read/written.
type TokenDelivery string

const (
	DeliverJSON   TokenDelivery = "json"
	DeliverCookie TokenDelivery = "cookie"
)

// TokenTransport configures refresh delivery for session handlers.
type TokenTransport struct {
	Access  TokenDelivery
	Refresh TokenDelivery
	// RefreshCookieName isolates refresh cookies for clients sharing an API
	// host. Empty preserves the historical "refresh_token" name.
	RefreshCookieName string
	// RefreshCookieCandidates opts a legacy route into accepting several named
	// cookies. Refresh requires exactly one present candidate and writes its
	// successor to that same name. An ambiguous request fails without rotation.
	// Leave empty to retain RefreshCookieName and its legacy fallback behavior.
	RefreshCookieCandidates []string
}

// DefaultRefreshCookieName is used when a transport does not select a name.
const DefaultRefreshCookieName = "refresh_token"

// LogoutScope controls which refresh sessions a logout request revokes.
type LogoutScope uint8

const (
	// LogoutAllSessions preserves the historical behavior of revoking every
	// refresh session owned by the authenticated user.
	LogoutAllSessions LogoutScope = iota
	// LogoutCurrentSession revokes only the refresh session identified by the
	// authenticated access token's SessionID claim.
	LogoutCurrentSession
	// LogoutClientSessions revokes all sessions for the authenticated token's
	// ClientID. A missing ClientID is rejected; no broader fallback is used.
	LogoutClientSessions
	// LogoutUnmarkedSessions revokes legacy sessions that have no ClientID in
	// client-aware stores; older Store adapters preserve account-wide behavior.
	LogoutUnmarkedSessions
)

type refreshLogoutOptions struct {
	transport   TokenTransport
	logoutScope LogoutScope
	clientID    string
}

// RefreshLogoutOption configures RegisterRefreshLogout.
type RefreshLogoutOption func(*refreshLogoutOptions)

// WithTokenTransport sets token delivery for refresh/logout handlers.
func WithTokenTransport(t TokenTransport) RefreshLogoutOption {
	return func(o *refreshLogoutOptions) {
		o.transport = t
	}
}

// WithLogoutScope sets which refresh sessions the logout handler revokes.
// The default is LogoutAllSessions for backward compatibility.
func WithLogoutScope(scope LogoutScope) RefreshLogoutOption {
	return func(o *refreshLogoutOptions) {
		o.logoutScope = scope
	}
}

// WithClientID assigns a server-configured client ID to sessions created by
// refresh and provides a safe legacy fallback during refresh migration. The
// value is never read from the request.
func WithClientID(clientID string) RefreshLogoutOption {
	return func(o *refreshLogoutOptions) { o.clientID = clientID }
}

func applyRefreshLogoutOptions(opts []RefreshLogoutOption) refreshLogoutOptions {
	o := refreshLogoutOptions{
		transport: TokenTransport{
			Access:  DeliverJSON,
			Refresh: DeliverCookie,
		},
	}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}
