package auth

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// MinPasswordLength is the minimum accepted length for a local (break-glass)
// password. It is enforced in production startup.
const MinPasswordLength = 12

// dummyHash is compared against when a local account is not found so the
// failure path costs the same as a real password check (no user enumeration).
var dummyHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

// HashPassword returns a bcrypt hash for the break-glass local account.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword verifies a plaintext password against a bcrypt hash.
func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// CheckDummy runs a throwaway bcrypt comparison to equalise timing.
func CheckDummy(plain string) {
	_ = bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(plain))
}

// ValidatePasswordStrength returns an error if plain is too short for a
// break-glass local account.
func ValidatePasswordStrength(plain string) error {
	if len(plain) < MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	}
	return nil
}
