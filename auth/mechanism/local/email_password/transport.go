package emailpassword

// TokenDelivery defines where tokens are read/written.
type TokenDelivery string

const (
	DeliverJSON   TokenDelivery = "json"
	DeliverCookie TokenDelivery = "cookie"
)

// TokenTransport configures access/refresh delivery for auth responses.
type TokenTransport struct {
	Access  TokenDelivery
	Refresh TokenDelivery
	// RefreshCookieName separates sessions for apps sharing an API host.
	// Empty retains the legacy "refresh_token" cookie name.
	RefreshCookieName string
}
