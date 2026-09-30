package mailer

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestNewSMTPValidatesRootCAFile(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		_, err := NewSMTP(Config{Address: "localhost:587", From: "noreply@example.test", RootCAFile: filepath.Join(t.TempDir(), "missing.pem")})
		if err == nil {
			t.Fatal("expected missing root CA file error")
		}
	})
	t.Run("invalid PEM", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "ca.pem")
		if err := os.WriteFile(file, []byte("not a certificate"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := NewSMTP(Config{Address: "localhost:587", From: "noreply@example.test", RootCAFile: file})
		if err == nil {
			t.Fatal("expected invalid root CA PEM error")
		}
	})
}
