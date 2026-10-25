package rate_limit

import (
	"errors"
	"time"
)

var ErrTooManyAttempts = errors.New("rate_limit: too many attempts")

// TooManyAttemptsError carries the remaining time in the active rate-limit
// window while preserving errors.Is(err, ErrTooManyAttempts).
type TooManyAttemptsError struct {
	RetryAfter time.Duration
}

func (e *TooManyAttemptsError) Error() string { return ErrTooManyAttempts.Error() }

func (e *TooManyAttemptsError) Unwrap() error { return ErrTooManyAttempts }

// RetryAfterDuration reports the remaining time before the attempt can be
// accepted again.
func (e *TooManyAttemptsError) RetryAfterDuration() time.Duration {
	if e == nil {
		return 0
	}
	return e.RetryAfter
}
