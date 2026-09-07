package subscription

import "time"

// Subscription binds a subject (user/org) to a plan for a period.
type Subscription struct {
	SubjectID  string
	PlanID     string
	AssignedBy *string
	StartedAt  time.Time
	ExpiresAt  *time.Time
}

// IsActive reports whether the subscription is within its validity window.
func (s Subscription) IsActive(now time.Time) bool {
	if s.SubjectID == "" || s.PlanID == "" {
		return false
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if !s.StartedAt.IsZero() && now.Before(s.StartedAt) {
		return false
	}
	if s.ExpiresAt != nil && !s.ExpiresAt.IsZero() && !now.Before(*s.ExpiresAt) {
		return false
	}
	return true
}

// IsExpired is the inverse of IsActive for an already-started subscription.
func (s Subscription) IsExpired(now time.Time) bool {
	return !s.IsActive(now)
}
