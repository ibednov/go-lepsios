package emailpassword

import "testing"

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := HashPassword("ValidPass123")
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyPassword(hash, "ValidPass123"); err != nil {
		t.Fatal(err)
	}
	if err := VerifyPassword(hash, "DifferentPass123"); err == nil {
		t.Fatal("expected wrong password to fail")
	}
}

func TestPasswordHashUsesSharedStrengthPolicy(t *testing.T) {
	if _, err := HashPassword("weak"); err == nil {
		t.Fatal("expected weak password to fail")
	}
}
