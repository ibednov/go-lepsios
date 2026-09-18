package billing_test

import (
	"context"
	"testing"
	"time"

	"github.com/ibednov/go-lepsios/billing"
	"github.com/ibednov/go-lepsios/currency"
	"github.com/stretchr/testify/require"
)

func TestAdminConfirm(t *testing.T) {
	t.Parallel()
	intent := billing.NewPaymentIntent("u1", billing.ProductKindSubscription, "premium", 9.9, currency.BYN)
	require.Equal(t, billing.IntentCreated, intent.Status)

	err := billing.AdminConfirm{}.Confirm(context.Background(), &intent)
	require.NoError(t, err)
	require.Equal(t, billing.IntentConfirmed, intent.Status)
	require.Equal(t, billing.StrategyAdminConfirm, intent.Strategy)

	// idempotent
	require.NoError(t, billing.AdminConfirm{}.Confirm(context.Background(), &intent))
}

func TestAdminConfirmRejectsTerminal(t *testing.T) {
	t.Parallel()
	intent := billing.NewPaymentIntent("u1", billing.ProductKindOneTime, "pack", 1, currency.BYN)
	intent.Status = billing.IntentFailed
	require.ErrorIs(t, billing.AdminConfirm{}.Confirm(context.Background(), &intent), billing.ErrInvalidIntentState)
}

type stubAssigner struct {
	called bool
	slug   string
}

func (s *stubAssigner) AssignPlan(_ context.Context, _, planSlug string, _ *string, _ *time.Time) error {
	s.called = true
	s.slug = planSlug
	return nil
}

func TestAssignSubscription(t *testing.T) {
	t.Parallel()
	a := &stubAssigner{}
	by := "admin1"
	intent, err := billing.AssignSubscription(context.Background(), a, billing.AssignSubscriptionInput{
		SubjectID:  "u1",
		PlanSlug:   "standard",
		Amount:     4.9,
		AssignedBy: &by,
	})
	require.NoError(t, err)
	require.True(t, a.called)
	require.Equal(t, "standard", a.slug)
	require.Equal(t, billing.IntentConfirmed, intent.Status)
	require.Equal(t, billing.ProductKindSubscription, intent.ProductKind)
}
