package mailer

import "testing"

func TestNewSMTPRequiresAddressAndFrom(t *testing.T) {
	if _, err := NewSMTP(Config{}); err == nil {
		t.Fatal("expected required configuration error")
	}
}
func TestNewSMTPRejectsInvalidAddress(t *testing.T) {
	if _, err := NewSMTP(Config{Address: "bad address", From: "noreply@example.test"}); err == nil {
		t.Fatal("expected address validation error")
	}
}
