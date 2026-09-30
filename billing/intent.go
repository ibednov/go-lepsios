package billing

import (
	"time"

	"github.com/ibednov/go-lepsios/currency"
)

// ProductKind distinguishes what a payment intent is buying.
type ProductKind string

const (
	ProductKindSubscription ProductKind = "subscription"
	ProductKindOneTime      ProductKind = "one_time"
	ProductKindBooking      ProductKind = "booking"
)

// IntentStatus is the lifecycle of a payment intent.
type IntentStatus string

const (
	IntentCreated   IntentStatus = "created"
	IntentPending   IntentStatus = "pending"
	IntentConfirmed IntentStatus = "confirmed"
	IntentFailed    IntentStatus = "failed"
	IntentRefunded  IntentStatus = "refunded"
	IntentCanceled  IntentStatus = "canceled"
)

func (s IntentStatus) IsValid() bool {
	switch s {
	case IntentCreated, IntentPending, IntentConfirmed, IntentFailed, IntentRefunded, IntentCanceled:
		return true
	default:
		return false
	}
}

func (s IntentStatus) String() string { return string(s) }

// PaymentIntent is a provider-agnostic payment attempt.
// Persistence and product-specific schemas stay in the consumer.
type PaymentIntent struct {
	ID          string
	SubjectID   string
	ProductKind ProductKind
	ProductRef  string // plan slug / one-time product slug / id
	// AmountMinor is the payment amount in the currency's minor units
	// (for example, kopecks/cents). It is the canonical amount used by
	// strategies and persistence adapters; never use floating point for money.
	AmountMinor int64
	Currency    currency.Code
	Status      IntentStatus
	Strategy    string // strategy name that confirmed/failed this intent
	ConfirmedBy *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Meta        map[string]any
}

// NewPaymentIntent builds a created intent with defaults.
func NewPaymentIntent(subjectID string, kind ProductKind, productRef string, amountMinor int64, cur currency.Code) PaymentIntent {
	now := time.Now().UTC()
	if cur == "" {
		cur = currency.BYN
	}
	return PaymentIntent{
		SubjectID:   subjectID,
		ProductKind: kind,
		ProductRef:  productRef,
		AmountMinor: amountMinor,
		Currency:    cur,
		Status:      IntentCreated,
		CreatedAt:   now,
		UpdatedAt:   now,
		Meta:        map[string]any{},
	}
}
