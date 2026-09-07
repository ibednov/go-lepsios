package billing

import (
	"context"
	"time"
)

const StrategyAdminConfirm = "admin_confirm"

// AdminConfirm is the baseline payment strategy: an admin (or system actor)
// marks the intent as paid without a payment gateway.
type AdminConfirm struct{}

func (AdminConfirm) Name() string { return StrategyAdminConfirm }

func (AdminConfirm) Confirm(ctx context.Context, intent *PaymentIntent) error {
	_ = ctx
	if intent == nil {
		return ErrNilIntent
	}
	switch intent.Status {
	case IntentCreated, IntentPending, IntentConfirmed:
		// Confirmed is idempotent — re-confirm is a no-op success.
	default:
		return ErrInvalidIntentState
	}
	now := time.Now().UTC()
	intent.Status = IntentConfirmed
	intent.Strategy = StrategyAdminConfirm
	intent.UpdatedAt = now
	if intent.CreatedAt.IsZero() {
		intent.CreatedAt = now
	}
	return nil
}

// DefaultRegistry returns a registry with AdminConfirm registered.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(AdminConfirm{})
	return r
}
