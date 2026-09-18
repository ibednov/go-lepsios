package purchase

import (
	"time"

	"github.com/ibednov/go-lepsios/currency"
)

// Status of a one-time purchase fulfillment.
type Status string

const (
	StatusPending   Status = "pending"
	StatusFulfilled Status = "fulfilled"
	StatusFailed    Status = "failed"
	StatusRefunded  Status = "refunded"
)

func (s Status) IsValid() bool {
	switch s {
	case StatusPending, StatusFulfilled, StatusFailed, StatusRefunded:
		return true
	default:
		return false
	}
}

// Product is a one-time purchasable SKU (not a recurring plan).
type Product struct {
	ID            string
	Slug          string
	Name          string
	PriceAmount   float64
	PriceCurrency currency.Code
	Features      map[string]any // entitlements granted on fulfill
	Active        bool
	CreatedAt     time.Time
}

// Purchase records that a subject bought a one-time product.
type Purchase struct {
	ID            string
	SubjectID     string
	ProductID     string
	PaymentIntent string
	Status        Status
	FulfilledAt   *time.Time
	CreatedAt     time.Time
}
