package auth

import "testing"

func TestPINHashRoundTrip(t *testing.T) {
	hash, err := HashPIN("482617")
	if err != nil {
		t.Fatalf("hash PIN: %v", err)
	}
	if hash == "482617" {
		t.Fatal("PIN was stored in plaintext")
	}
	if !VerifyPIN(hash, "482617") {
		t.Fatal("valid PIN was rejected")
	}
	if VerifyPIN(hash, "482618") {
		t.Fatal("invalid PIN was accepted")
	}
}

func TestPINValidation(t *testing.T) {
	for _, pin := range []string{"12345", "1234567", "12a456", ""} {
		if _, err := HashPIN(pin); err == nil {
			t.Fatalf("invalid PIN %q was accepted", pin)
		}
	}
}

func TestUsernameValidation(t *testing.T) {
	if NormalizeUsername("  Inventory.User ") != "inventory.user" {
		t.Fatal("username was not normalized")
	}
	if err := ValidateUsername("ab"); err == nil {
		t.Fatal("short username was accepted")
	}
	if err := ValidateUsername("inventory.user"); err != nil {
		t.Fatalf("valid username was rejected: %v", err)
	}
}
