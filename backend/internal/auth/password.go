package auth

import (
	"golang.org/x/crypto/bcrypt"

	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

const passwordHashCost = bcrypt.DefaultCost

// HashPassword returns a bcrypt hash for password.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), passwordHashCost)
	if err != nil {
		return "", errors.B.Op("auth.password.hash_password").Text("hashing password").Err(err).Build()
	}
	return string(hash), nil
}

// VerifyPassword compares password with a stored bcrypt hash.
func VerifyPassword(hash, password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return errors.B.Op("auth.password.verify_password").Text("verifying password").Err(err).Build()
	}
	return nil
}
