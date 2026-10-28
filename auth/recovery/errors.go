package recovery

import (
	"encoding/json"
	"errors"
)

var (
	ErrInvalidToken = errors.New("recovery token is invalid, expired, or already used")
	ErrDisabled     = errors.New("account recovery is disabled")
	ErrRateLimited  = errors.New("recovery request rate limit exceeded")
)

type DeliveryError struct {
	Failure DeliveryFailure
	Cause   error
}

func (e *DeliveryError) Error() string {
	if e == nil {
		return "recovery delivery failed"
	}
	return "recovery delivery failed: " + e.Failure.Reason
}

func (e *DeliveryError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func NewDeliveryError(failure DeliveryFailure, cause error) error {
	if cause == nil {
		return nil
	}
	if failure.Reason == "" {
		failure.Reason = "provider_failure"
	}
	if len(failure.Diagnostic) > 0 && !json.Valid(failure.Diagnostic) {
		failure.Diagnostic = nil
	}
	return &DeliveryError{Failure: failure, Cause: cause}
}
