package subscription_test

import (
	"testing"
	"time"

	"github.com/ibednov/go-lepsios/billing/subscription"
	"github.com/stretchr/testify/require"
)

func TestVisibility(t *testing.T) {
	t.Parallel()
	require.True(t, subscription.VisibilityPublic.IsValid())
	require.False(t, subscription.Visibility("nope").IsValid())
}

func TestSubscriptionActive(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	exp := now.Add(24 * time.Hour)
	past := now.Add(-time.Hour)

	s := subscription.Subscription{SubjectID: "u", PlanID: "p", StartedAt: past, ExpiresAt: &exp}
	require.True(t, s.IsActive(now))

	s.ExpiresAt = &past
	require.False(t, s.IsActive(now))
}
