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
}

// LogoutScope controls which refresh sessions a logout request revokes.
type LogoutScope uint8

const (
	// LogoutAllSessions preserves the historical behavior of revoking every
	// refresh session owned by the authenticated user.
	LogoutAllSessions LogoutScope = iota
	// LogoutCurrentSession revokes only the refresh session identified by the
	// authenticated access token's SessionID claim.
	LogoutCurrentSession
)

type refreshLogoutOptions struct {
	transport   TokenTransport
	logoutScope LogoutScope
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
