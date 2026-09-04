// Package apperr provides a reusable structured application error type and an
// HTTP mapping hook that integrates with go-lepsios/httpx/response.
//
// Consumers keep their product error codes and MessageKeys (e.g.
// "AUTH.EMAIL_2FA.VERIFY.INVALID_CODE"); this package carries only the
// structure, constructors and the generic HTTP mapper.
package apperr

// Code is a stable, machine-readable error code (UPPER_SNAKE_CASE), e.g.
// "AUTH.EMAIL_2FA.VERIFY.INVALID_CODE".
type Code string

// Error is a structured application error.
type Error struct {
	Code       Code
	MessageKey string // i18n key; if empty, GetMessageKey returns Code.
	Message    string // technical message for logs.
	HTTPStatus int    // optional HTTP status; 0 means handler default.
	Err        error  // original cause, for logging.
}

// Error implements error.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return string(e.Code)
}

// Unwrap returns the wrapped original error.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// GetMessageKey returns the i18n key; falls back to the code string.
func (e *Error) GetMessageKey() string {
	if e == nil {
		return ""
	}
	if e.MessageKey != "" {
		return e.MessageKey
	}
	return string(e.Code)
}

// GetErrorCode returns the error code string.
func (e *Error) GetErrorCode() string {
	if e == nil {
		return ""
	}
	return string(e.Code)
}

// New builds a typed error with a code and technical message.
func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Wrap builds a typed error wrapping an underlying cause. The i18n message key
// defaults to the code string.
func Wrap(code Code, message string, err error) *Error {
	return &Error{Code: code, MessageKey: string(code), Message: message, Err: err}
}

// WithMessageKey returns a copy of err with an explicit i18n message key.
func WithMessageKey(err *Error, key string) *Error {
	out := &Error{Code: err.Code, Message: err.Message, HTTPStatus: err.HTTPStatus, Err: err.Err}
	if key != "" {
		out.MessageKey = key
	} else {
		out.MessageKey = err.MessageKey
	}
	return out
}

// WithHTTPStatus returns a copy of err with an explicit HTTP status.
func WithHTTPStatus(err *Error, status int) *Error {
	out := &Error{Code: err.Code, MessageKey: err.MessageKey, Message: err.Message, Err: err.Err}
	out.HTTPStatus = status
	return out
}

var _ error = (*Error)(nil)