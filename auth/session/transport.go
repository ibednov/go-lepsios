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

type refreshLogoutOptions struct {
	transport TokenTransport
}

// RefreshLogoutOption configures RegisterRefreshLogout.
type RefreshLogoutOption func(*refreshLogoutOptions)

// WithTokenTransport sets token delivery for refresh/logout handlers.
func WithTokenTransport(t TokenTransport) RefreshLogoutOption {
	return func(o *refreshLogoutOptions) {
		o.transport = t
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
