package entitlements

// Limit is a numeric cap. Negative underlying value means unlimited.
type Limit int

// NewLimit builds a Limit. unlimited=true stores -1 regardless of value.
func NewLimit(value int, unlimited bool) Limit {
	if unlimited {
		return -1
	}
	return Limit(value)
}

func (l Limit) Value() int { return int(l) }

func (l Limit) Unlimited() bool { return l < 0 }

// Allows reports whether current usage is still under the cap.
func (l Limit) Allows(current int64) bool {
	return l.Unlimited() || current < int64(l)
}
