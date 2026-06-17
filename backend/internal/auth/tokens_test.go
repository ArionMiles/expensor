package auth_test

import (
	"strings"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/auth"
)

func TestTokenHashDoesNotExposeRawToken(t *testing.T) {
	raw, hash, err := auth.NewOpaqueToken("expensor_pat")
	if err != nil {
		t.Fatalf("NewOpaqueToken() error = %v", err)
	}
	if !strings.HasPrefix(raw, "expensor_pat_") {
		t.Fatalf("raw token prefix = %q", raw)
	}
	if strings.Contains(hash, raw) {
		t.Fatalf("hash contains raw token")
	}
	if got := auth.HashOpaqueToken(raw); got != hash {
		t.Fatalf("HashOpaqueToken() = %q, want %q", got, hash)
	}
}
