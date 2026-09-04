package apperr

import (
	"errors"
	"net/http"
	"testing"

	"github.com/ibednov/go-lepsios/rate_limit"
)

func TestErrorGetters(t *testing.T) {
	e := New("AUTH.VERIFY.INVALID", "invalid code")
	if e.GetErrorCode() != "AUTH.VERIFY.INVALID" {
		t.Fatalf("unexpected code: %s", e.GetErrorCode())
	}
	if e.GetMessageKey() != "AUTH.VERIFY.INVALID" {
		t.Fatalf("message key should fall back to code: %s", e.GetMessageKey())
	}

	withKey := WithMessageKey(e, "auth.verify.invalid")
	if withKey.GetMessageKey() != "auth.verify.invalid" {
		t.Fatalf("message key override failed: %s", withKey.GetMessageKey())
	}
}

func TestMapperRateLimit(t *testing.T) {
	m := Mapper(nil, http.StatusBadRequest)
	status, code, _, ok := m(rate_limit.ErrTooManyAttempts)
	if !ok {
		t.Fatal("expected mapping")
	}
	if status != http.StatusTooManyRequests || code != "TOO_MANY_ATTEMPTS" {
		t.Fatalf("unexpected mapping: status=%d code=%s", status, code)
	}
}

func TestMapperDomainError(t *testing.T) {
	m := Mapper(nil, http.StatusBadRequest)
	status, code, msg, ok := m(New("DOMAIN.NOT_FOUND", "nope"))
	if !ok {
		t.Fatal("expected mapping")
	}
	if status != http.StatusBadRequest || code != "DOMAIN.NOT_FOUND" || msg != "DOMAIN.NOT_FOUND" {
		t.Fatalf("unexpected mapping: status=%d code=%s msg=%s", status, code, msg)
	}
}

func TestMapperHTTPStatusOverride(t *testing.T) {
	m := Mapper(nil, http.StatusBadRequest)
	_, code, _, _ := m(WithHTTPStatus(New("DOMAIN.CONFLICT", "conf"), http.StatusConflict))
	if code != "DOMAIN.CONFLICT" {
		t.Fatalf("unexpected code %s", code)
	}
}

func TestUnwrap(t *testing.T) {
	base := errors.New("root")
	e := Wrap("DOMAIN.WRAP", "wrapped", base)
	if !errors.Is(e, base) {
		t.Fatal("Unwrap should expose the cause")
	}
}