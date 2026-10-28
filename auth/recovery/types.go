// Package recovery provides product-neutral orchestration for one-time account
// recovery links. Product adapters own account lookup and transactional writes.
package recovery

import (
	"context"
	"time"

	"github.com/ibednov/go-lepsios/mailer"
)

type Policy struct {
	Enabled                         bool
	RequestAttemptsPerClientPerHour int
	ConfirmAttemptsPerClientPerHour int
	TokenTTL                        time.Duration
	Limits                          Limits
}

type Client struct {
	// Fingerprint is a product-keyed pseudonym, never a raw IP or device value.
	Fingerprint []byte
	// RateLimitFingerprint may preserve an existing limiter key independently
	// from the fingerprint persisted in the product audit store.
	RateLimitFingerprint []byte
}

type Limits struct {
	AccountPerHour int
	ClientPerHour  int
	Daily          int
	Monthly        int
}

type ReservationCommand struct {
	AccountHint string
	Fingerprint []byte
	Enabled     bool
	TokenID     string
	EventID     string
	TokenHash   []byte
	TTL         time.Duration
	Limits      Limits
}

// Reservation contains delivery data read with the product's atomic quota
// reservation. Empty delivery fields mean the request was suppressed safely.
type Reservation struct {
	Reserved    bool
	TokenID     string
	EventID     string
	Recipient   string
	Strategy    string
	Sender      string
	LinkBaseURL string
	ExpiresAt   time.Time
}

type Delivery struct {
	Recipient   string
	Sender      string
	RawToken    string
	TokenID     string
	LinkBaseURL string
	ExpiresAt   time.Time
}

type Subject struct {
	ID string
}

type RequestResult struct {
	Accepted bool
}

type ConfirmationCommand struct {
	TokenHash   []byte
	Fingerprint []byte
	ClientLimit int
}

type DeliveryFailure struct {
	Reason     string
	Provider   string
	Diagnostic []byte
}

type PolicyStore interface {
	Policy(context.Context) (Policy, error)
}

type Limiter interface {
	Allow(context.Context, string, []byte, int, time.Duration) error
}

type Store[Tx any] interface {
	PolicyStore
	Reserve(context.Context, ReservationCommand) (Reservation, error)
	MarkAccepted(context.Context, string, mailer.Receipt) error
	MarkFailed(context.Context, string, string, DeliveryFailure) error
	FindActive(context.Context, []byte) (Subject, error)
	RecordRejected(context.Context, string, string, string, []byte) error
	Confirm(context.Context, ConfirmationCommand, func(context.Context, Tx, Subject) error) (Subject, error)
	RecordSessionInvalidationFailure(context.Context, string, []byte) error
}

type ConfirmationAction[Tx any] func(context.Context, Subject) (func(context.Context, Tx, Subject) error, error)

type Composer interface {
	Compose(context.Context, Delivery) (mailer.Message, error)
}

type Sender interface {
	Send(context.Context, string, string, mailer.Message) (mailer.Receipt, error)
}

type SessionInvalidator interface {
	Invalidate(context.Context, string) error
}
