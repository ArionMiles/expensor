package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

func TestAuthRepositoryBootstrapAndSessionLifecycle(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()

	ctx := context.Background()
	required, err := ts.BootstrapRequired(ctx)
	if err != nil {
		t.Fatalf("BootstrapRequired() error = %v", err)
	}
	if !required {
		t.Fatal("BootstrapRequired() = false before users exist")
	}

	admin, err := ts.CreateBootstrapAdmin(ctx, store.CreateBootstrapAdminInput{
		Email:        "admin@example.com",
		DisplayName:  "Admin",
		PasswordHash: "$2a$10$abcdefghijklmnopqrstuu6Z6RMcYbqVvB6KZlSmLfHLj6y8s3zme",
		AvatarKey:    "default",
	})
	if err != nil {
		t.Fatalf("CreateBootstrapAdmin() error = %v", err)
	}
	if admin.TenantID != admin.ID || admin.Role != store.UserRoleAdmin {
		t.Fatalf("admin = %#v", admin)
	}

	required, err = ts.BootstrapRequired(ctx)
	if err != nil {
		t.Fatalf("BootstrapRequired() after admin error = %v", err)
	}
	if required {
		t.Fatal("BootstrapRequired() = true after admin exists")
	}

	expiresAt := time.Now().Add(time.Hour)
	session, err := ts.CreateSession(ctx, store.CreateSessionInput{
		UserID:    admin.ID,
		TokenHash: "sha256:session-token",
		ExpiresAt: expiresAt,
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if session.UserID != admin.ID || session.TokenHash != "sha256:session-token" {
		t.Fatalf("session = %#v", session)
	}

	foundSession, err := ts.FindSessionByHash(ctx, "sha256:session-token")
	if err != nil {
		t.Fatalf("FindSessionByHash() error = %v", err)
	}
	if foundSession == nil || foundSession.UserID != admin.ID {
		t.Fatalf("foundSession = %#v", foundSession)
	}

	if err := ts.RevokeSession(ctx, session.ID); err != nil {
		t.Fatalf("RevokeSession() error = %v", err)
	}
	revokedSession, err := ts.FindSessionByHash(ctx, "sha256:session-token")
	if err != nil {
		t.Fatalf("FindSessionByHash() after revoke error = %v", err)
	}
	if revokedSession == nil || revokedSession.RevokedAt == nil {
		t.Fatalf("revokedSession = %#v", revokedSession)
	}

	_, err = ts.CreateBootstrapAdmin(ctx, store.CreateBootstrapAdminInput{
		Email:        "other@example.com",
		DisplayName:  "Other",
		PasswordHash: "$2a$10$abcdefghijklmnopqrstuu6Z6RMcYbqVvB6KZlSmLfHLj6y8s3zme",
		AvatarKey:    "default",
	})
	if !errors.Is(err, store.ErrBootstrapUnavailable) {
		t.Fatalf("second bootstrap error = %v, want ErrBootstrapUnavailable", err)
	}
}

func TestAuthRepositoryStoresOnlyTokenHashes(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()

	ctx := context.Background()
	admin, err := ts.CreateBootstrapAdmin(ctx, store.CreateBootstrapAdminInput{
		Email:        "admin@example.com",
		DisplayName:  "Admin",
		PasswordHash: "$2a$10$abcdefghijklmnopqrstuu6Z6RMcYbqVvB6KZlSmLfHLj6y8s3zme",
		AvatarKey:    "default",
	})
	if err != nil {
		t.Fatalf("CreateBootstrapAdmin() error = %v", err)
	}

	token, err := ts.CreateAccessToken(ctx, store.CreateAccessTokenInput{
		UserID:    admin.ID,
		Name:      "cli",
		TokenHash: "sha256:abc123",
		ExpiresAt: nil,
	})
	if err != nil {
		t.Fatalf("CreateAccessToken() error = %v", err)
	}
	if token.ID == "" || token.Name != "cli" {
		t.Fatalf("token = %#v", token)
	}

	found, err := ts.FindAccessTokenByHash(ctx, "sha256:abc123")
	if err != nil {
		t.Fatalf("FindAccessTokenByHash() error = %v", err)
	}
	if found == nil || found.UserID != admin.ID || found.Name != "cli" {
		t.Fatalf("found = %#v", found)
	}

	if err := ts.RevokeAccessToken(ctx, token.ID, admin.ID); err != nil {
		t.Fatalf("RevokeAccessToken() error = %v", err)
	}
	revoked, err := ts.FindAccessTokenByHash(ctx, "sha256:abc123")
	if err != nil {
		t.Fatalf("FindAccessTokenByHash() after revoke error = %v", err)
	}
	if revoked == nil || revoked.RevokedAt == nil {
		t.Fatalf("revoked = %#v", revoked)
	}
}

func TestAuthRepositoryListsActiveAccessTokenMetadata(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()

	ctx := context.Background()
	admin, err := ts.CreateBootstrapAdmin(ctx, store.CreateBootstrapAdminInput{
		Email:        "admin@example.com",
		DisplayName:  "Admin",
		PasswordHash: "$2a$10$abcdefghijklmnopqrstuu6Z6RMcYbqVvB6KZlSmLfHLj6y8s3zme",
		AvatarKey:    "default",
	})
	if err != nil {
		t.Fatalf("CreateBootstrapAdmin() error = %v", err)
	}
	other, err := ts.CreateUser(ctx, store.CreateUserInput{
		Email:       "other@example.com",
		DisplayName: "Other",
		Role:        store.UserRoleUser,
		AvatarKey:   "ledger",
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	adminToken, err := ts.CreateAccessToken(ctx, store.CreateAccessTokenInput{
		UserID:    admin.ID,
		Name:      "cli",
		TokenHash: "sha256:admin-token",
	})
	if err != nil {
		t.Fatalf("CreateAccessToken(admin) error = %v", err)
	}
	revokedToken, err := ts.CreateAccessToken(ctx, store.CreateAccessTokenInput{
		UserID:    admin.ID,
		Name:      "old",
		TokenHash: "sha256:old-token",
	})
	if err != nil {
		t.Fatalf("CreateAccessToken(revoked) error = %v", err)
	}
	if _, err := ts.CreateAccessToken(ctx, store.CreateAccessTokenInput{
		UserID:    other.ID,
		Name:      "other",
		TokenHash: "sha256:other-token",
	}); err != nil {
		t.Fatalf("CreateAccessToken(other) error = %v", err)
	}
	if err := ts.RevokeAccessToken(ctx, revokedToken.ID, admin.ID); err != nil {
		t.Fatalf("RevokeAccessToken() error = %v", err)
	}

	tokens, err := ts.ListAccessTokens(ctx, admin.ID)
	if err != nil {
		t.Fatalf("ListAccessTokens() error = %v", err)
	}
	if len(tokens) != 1 || tokens[0].ID != adminToken.ID || tokens[0].Name != "cli" {
		t.Fatalf("tokens = %#v", tokens)
	}
	if tokens[0].TokenHash != "sha256:admin-token" {
		t.Fatalf("token hash changed unexpectedly: %#v", tokens[0])
	}
}

func TestAuthRepositoryListsAndUpdatesUsers(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()

	ctx := context.Background()
	admin, err := ts.CreateBootstrapAdmin(ctx, store.CreateBootstrapAdminInput{
		Email:        "admin@example.com",
		DisplayName:  "Admin",
		PasswordHash: "$2a$10$abcdefghijklmnopqrstuu6Z6RMcYbqVvB6KZlSmLfHLj6y8s3zme",
		AvatarKey:    "default",
	})
	if err != nil {
		t.Fatalf("CreateBootstrapAdmin() error = %v", err)
	}
	user, err := ts.CreateUser(ctx, store.CreateUserInput{
		Email:       "user@example.com",
		DisplayName: "User",
		Role:        store.UserRoleUser,
		AvatarKey:   "ledger",
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	displayName := "Updated User"
	role := store.UserRoleAdmin
	avatarKey := "wallet"
	disabled := true
	updated, err := ts.UpdateUser(ctx, user.ID, store.UpdateUserInput{
		DisplayName: &displayName,
		Role:        &role,
		AvatarKey:   &avatarKey,
		Disabled:    &disabled,
	})
	if err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}
	if updated.DisplayName != displayName || updated.Role != role || updated.AvatarKey != avatarKey || updated.DisabledAt == nil {
		t.Fatalf("updated user = %#v", updated)
	}

	disabled = false
	updated, err = ts.UpdateUser(ctx, user.ID, store.UpdateUserInput{Disabled: &disabled})
	if err != nil {
		t.Fatalf("UpdateUser(enable) error = %v", err)
	}
	if updated.DisabledAt != nil {
		t.Fatalf("updated user remains disabled: %#v", updated)
	}

	users, err := ts.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(users) != 2 || users[0].ID != admin.ID || users[1].ID != user.ID {
		t.Fatalf("users = %#v", users)
	}
}

func TestAuthRepositoryCompletesAccountSetupOnce(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()

	ctx := context.Background()
	user, err := ts.CreateUser(ctx, store.CreateUserInput{
		Email:       "setup@example.com",
		DisplayName: "Setup User",
		Role:        store.UserRoleUser,
		AvatarKey:   "default",
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	token, err := ts.CreateAccountSetupToken(ctx, store.CreateAccountSetupTokenInput{
		UserID:    user.ID,
		TokenHash: "sha256:setup-token",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateAccountSetupToken() error = %v", err)
	}

	updated, err := ts.CompleteAccountSetup(ctx, "sha256:setup-token", "$2a$10$newhashabcdefghijklmnop")
	if err != nil {
		t.Fatalf("CompleteAccountSetup() error = %v", err)
	}
	if updated.ID != user.ID || updated.PasswordHash != "$2a$10$newhashabcdefghijklmnop" {
		t.Fatalf("updated user = %#v", updated)
	}
	used, err := ts.FindAccountSetupTokenByHash(ctx, token.TokenHash)
	if err != nil {
		t.Fatalf("FindAccountSetupTokenByHash() error = %v", err)
	}
	if used.UsedAt == nil {
		t.Fatalf("setup token was not marked used: %#v", used)
	}
	if _, err := ts.CompleteAccountSetup(ctx, "sha256:setup-token", "$2a$10$otherhashabcdefghijklmn"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("second CompleteAccountSetup() error = %v, want ErrNotFound", err)
	}
}
