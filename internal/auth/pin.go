package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/crypto/argon2"
)

var usernamePattern = regexp.MustCompile(`^[a-z][a-z0-9._-]{2,31}$`)
var pinPattern = regexp.MustCompile(`^[0-9]{6}$`)

func NormalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func ValidateUsername(username string) error {
	if !usernamePattern.MatchString(NormalizeUsername(username)) {
		return fmt.Errorf("username must be 3-32 characters and contain only letters, numbers, dot, underscore, or hyphen")
	}
	return nil
}

func ValidatePIN(pin string) error {
	if !pinPattern.MatchString(strings.TrimSpace(pin)) {
		return fmt.Errorf("PIN must contain exactly 6 digits")
	}
	return nil
}

func HashPIN(pin string) (string, error) {
	pin = strings.TrimSpace(pin)
	if err := ValidatePIN(pin); err != nil {
		return "", err
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(pin), salt, 2, 64*1024, 2, 32)
	return fmt.Sprintf("argon2id$v=19$m=65536,t=2,p=2$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}

func VerifyPIN(encoded, pin string) bool {
	if ValidatePIN(pin) != nil {
		return false
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 5 || parts[0] != "argon2id" {
		return false
	}
	salt, err1 := base64.RawStdEncoding.DecodeString(parts[3])
	expected, err2 := base64.RawStdEncoding.DecodeString(parts[4])
	if err1 != nil || err2 != nil {
		return false
	}
	actual := argon2.IDKey([]byte(strings.TrimSpace(pin)), salt, 2, 64*1024, 2, 32)
	if len(actual) != len(expected) {
		return false
	}
	var mismatch byte
	for i := range actual {
		mismatch |= actual[i] ^ expected[i]
	}
	return mismatch == 0
}
