package mailer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestResendSendsMessageAndReturnsProviderID(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.String() != resendEndpoint {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
		}
		if req.Header.Get("Authorization") != "Bearer secret" || req.Header.Get("Idempotency-Key") != "reset-123" {
			t.Fatal("missing authentication or idempotency key")
		}
		var payload struct {
			From    string   `json:"from"`
			To      []string `json:"to"`
			Subject string   `json:"subject"`
			Text    string   `json:"text"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.From != "noreply@example.test" || len(payload.To) != 1 || payload.To[0] != "user@example.test" || payload.Subject != "Reset" || payload.Text != "Open link" {
			t.Fatalf("unexpected payload: %+v", payload)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"id":"email-123"}`)), Header: make(http.Header)}, nil
	})}
	resend, err := NewResend(ResendConfig{APIKey: "secret", From: "noreply@example.test", Client: client})
	if err != nil {
		t.Fatal(err)
	}
	router, err := NewRouter(map[string]Strategy{"resend": resend})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := router.Send(context.Background(), "resend", Message{To: "user@example.test", Subject: "Reset", Body: "Open link", IdempotencyKey: "reset-123"})
	if err != nil || receipt.Provider != "resend" || receipt.MessageID != "email-123" {
		t.Fatalf("unexpected receipt: %+v, %v", receipt, err)
	}
}

func TestResendErrorIsTypedAndDoesNotExposeBody(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 429, Body: io.NopCloser(strings.NewReader("sensitive provider body")), Header: http.Header{"Retry-After": []string{"5"}}}, nil
	})}
	resend, err := NewResend(ResendConfig{APIKey: "secret", From: "noreply@example.test", Client: client})
	if err != nil {
		t.Fatal(err)
	}
	_, err = resend.Deliver(context.Background(), Message{To: "user@example.test", Subject: "Reset", Body: "Open link"})
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) || providerErr.StatusCode != 429 || providerErr.RetryAfter != "5" || strings.Contains(err.Error(), "sensitive") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRouterRejectsUnknownStrategy(t *testing.T) {
	resend, err := NewResend(ResendConfig{APIKey: "secret", From: "noreply@example.test"})
	if err != nil {
		t.Fatal(err)
	}
	router, err := NewRouter(map[string]Strategy{"resend": resend})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := router.Send(context.Background(), "smtp", Message{}); err == nil {
		t.Fatal("expected unknown strategy error")
	}
}
