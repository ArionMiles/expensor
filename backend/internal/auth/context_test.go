package auth_test

import (
	"context"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/auth"
)

func TestPrincipalContextRoundTrip(t *testing.T) {
	principal := auth.Principal{
		UserID:     "user-1",
		TenantID:   "tenant-1",
		Role:       auth.RoleAdmin,
		AuthMethod: "session",
	}

	ctx := auth.WithPrincipal(context.Background(), principal)
	got, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		t.Fatal("PrincipalFromContext() ok = false")
	}
	if got != principal {
		t.Fatalf("PrincipalFromContext() = %#v, want %#v", got, principal)
	}
}

func TestPrincipalFromContextMissing(t *testing.T) {
	if got, ok := auth.PrincipalFromContext(context.Background()); ok {
		t.Fatalf("PrincipalFromContext() = %#v, true; want false", got)
	}
}
