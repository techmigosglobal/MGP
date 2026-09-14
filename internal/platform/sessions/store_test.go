package sessions

import "testing"

func TestSignedCookieRejectsTampering(t *testing.T) {
	store := Store{Key: "01234567890123456789012345678901"}
	value := store.signCookie("session-id")
	if id, ok := store.verifyCookie(value); !ok || id != "session-id" {
		t.Fatal("valid signed cookie was rejected")
	}
	if _, ok := store.verifyCookie(value + "tampered"); ok {
		t.Fatal("tampered cookie was accepted")
	}
}
