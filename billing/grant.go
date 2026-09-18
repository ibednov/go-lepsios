package billing

import (
	"context"
	"time"

	"github.com/ibednov/go-lepsios/currency"
)

// SubscriptionAssigner persists plan assignment after a confirmed payment.
type SubscriptionAssigner interface {
	AssignPlan(ctx context.Context, subjectID, planSlug string, assignedBy *string, expiresAt *time.Time) error
}

// AssignSubscriptionInput is admin/system grant of a subscription plan.
type AssignSubscriptionInput struct {
	SubjectID  string
	PlanSlug   string
	Amount     float64
	Currency   currency.Code // empty → NewPaymentIntent default (BYN)
	AssignedBy *string
	ExpiresAt  *time.Time
	Strategy   Strategy // nil → AdminConfirm
}

// AssignSubscription confirms payment via strategy then assigns the plan.
// Today consumers use AdminConfirm; later swap Strategy for a real gateway.
func AssignSubscription(ctx context.Context, assigner SubscriptionAssigner, in AssignSubscriptionInput) (*PaymentIntent, error) {
	if assigner == nil {
		return nil, ErrNilIntent
	}
	strategy := in.Strategy
	if strategy == nil {
		strategy = AdminConfirm{}
	}
	intent := NewPaymentIntent(in.SubjectID, ProductKindSubscription, in.PlanSlug, in.Amount, in.Currency)
	intent.ConfirmedBy = in.AssignedBy
	if err := strategy.Confirm(ctx, &intent); err != nil {
		return &intent, err
	}
	if err := assigner.AssignPlan(ctx, in.SubjectID, in.PlanSlug, in.AssignedBy, in.ExpiresAt); err != nil {
		return &intent, err
	}
	return &intent, nil
}
