package auth_test

import (
	"strings"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/auth"
)

func TestPasswordHashVerify(t *testing.T) {
	hash, err := auth.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if hash == "correct horse battery staple" || !strings.HasPrefix(hash, "$2") {
		t.Fatalf("hash %q does not look like bcrypt", hash)
	}
	if err := auth.VerifyPassword(hash, "correct horse battery staple"); err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
	if err := auth.VerifyPassword(hash, "wrong"); err == nil {
		t.Fatal("VerifyPassword() succeeded for wrong password")
	}
}
