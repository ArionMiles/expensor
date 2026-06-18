package auth

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const passwordHashCost = bcrypt.DefaultCost

// HashPassword returns a bcrypt hash for password.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), passwordHashCost)
	if err != nil {
		return "", fmt.Errorf("hashing password: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword compares password with a stored bcrypt hash.
func VerifyPassword(hash, password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return fmt.Errorf("verifying password: %w", err)
	}
	return nil
}
