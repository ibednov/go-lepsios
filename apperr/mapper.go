package apperr

import (
	"errors"
	"net/http"

	"github.com/ibednov/go-lepsios/httpx/response"
	"github.com/ibednov/go-lepsios/rate_limit"
)

// CodeInternal is the fallback code returned when no specific mapping applies.
const CodeInternal Code = "INTERNAL_ERROR"

// Mapper builds an httpx response.ErrorMapper. localize translates an i18n
// message key to a user-facing string (nil keeps the key). defaultStatus is used
// for orrors that carry no explicit HTTP status.
func Mapper(localize func(messageKey string) string, defaultStatus int) response.ErrorMapper {
	if localize == nil {
		localize = func(k string) string { return k }
	}
	return func(err error) (int, string, string, bool) {
		if err == nil {
			return 0, "", "", false
		}
		if errors.Is(err, rate_limit.ErrTooManyAttempts) {
			return http.StatusTooManyRequests, "TOO_MANY_ATTEMPTS", localize("errors.too_many_attempts"), true
		}
		var e *Error
		if errors.As(err, &e) {
			status := defaultStatus
			if e.HTTPStatus != 0 {
				status = e.HTTPStatus
			}
			return status, e.GetErrorCode(), localize(e.GetMessageKey()), true
		}
		return defaultStatus, string(CodeInternal), err.Error(), true
	}
}