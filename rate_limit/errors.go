package rate_limit

import "errors"

var ErrTooManyAttempts = errors.New("rate_limit: too many attempts")
