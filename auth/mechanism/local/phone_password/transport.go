// Package phonepassword provides local phone-and-password authentication routes.
package phonepassword

import emailpassword "github.com/ibednov/go-lepsios/auth/mechanism/local/email_password"

// These aliases keep token delivery and verified identity compatible with the
// other local authentication modules, while this package exposes a phone-only
// request contract.
type VerifiedUser = emailpassword.VerifiedUser
type TokenDelivery = emailpassword.TokenDelivery
type TokenTransport = emailpassword.TokenTransport

type options struct {
	transport    TokenTransport
	errorHandler ErrorHandler
}

// Option configures phone/password response delivery.
type Option func(*options)

const (
	DeliverJSON   = emailpassword.DeliverJSON
	DeliverCookie = emailpassword.DeliverCookie
)

// WithTokenTransport sets access and refresh token delivery.
func WithTokenTransport(transport TokenTransport) Option {
	return func(o *options) { o.transport = transport }
}

// WithErrorHandler replaces the default registration error response. This lets
// consuming applications preserve domain error codes and response headers.
func WithErrorHandler(handler ErrorHandler) Option {
	return func(o *options) { o.errorHandler = handler }
}

// HashPassword applies the shared local password policy and bcrypt hashing.
func HashPassword(raw string) (string, error) { return emailpassword.HashPassword(raw) }

// VerifyPassword compares a password against a stored bcrypt hash.
func VerifyPassword(hash, raw string) error { return emailpassword.VerifyPassword(hash, raw) }
