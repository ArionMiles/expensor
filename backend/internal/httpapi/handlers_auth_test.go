package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/auth"
	"github.com/ArionMiles/expensor/backend/internal/store"
)

func TestAuthMiddlewareRejectsAnonymousPrivateRoute(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	mux := http.NewServeMux()
	registerRoutes(mux, h)
	handler := authMiddleware(h, mux)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/transactions", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body = %s", rec.Code, rec.Body.String())
	}
}

func TestAuthMiddlewareSessionSetsRequestTenant(t *testing.T) {
	raw, hash, err := auth.NewOpaqueToken(sessionTokenPrefix)
	if err != nil {
		t.Fatalf("NewOpaqueToken() error = %v", err)
	}
	user := &store.User{
		ID:          "user-a",
		TenantID:    "tenant-a",
		Email:       "a@example.com",
		DisplayName: "Tenant A",
		Role:        store.UserRoleUser,
		AvatarKey:   "default",
	}
	ms := &mockStore{
		appConfig: map[string]string{
			"base_currency": "INR",
		},
		sessionsByHash: map[string]*store.Session{
			hash: {ID: "session-a", UserID: user.ID, TokenHash: hash, ExpiresAt: time.Now().Add(time.Hour)},
		},
		usersByID: map[string]*store.User{user.ID: user},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})
	mux := http.NewServeMux()
	registerRoutes(mux, h)
	handler := authMiddleware(h, mux)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/config/preferences", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: raw})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	if ms.lastAppConfigTenant.ID != "tenant-a" {
		t.Fatalf("request tenant = %q, want tenant-a", ms.lastAppConfigTenant.ID)
	}
}

func TestAuthMiddlewareBearerSetsRequestTenant(t *testing.T) {
	raw, hash, err := auth.NewOpaqueToken(accessTokenPrefix)
	if err != nil {
		t.Fatalf("NewOpaqueToken() error = %v", err)
	}
	user := &store.User{
		ID:          "user-a",
		TenantID:    "tenant-a",
		Email:       "a@example.com",
		DisplayName: "Tenant A",
		Role:        store.UserRoleUser,
		AvatarKey:   "default",
	}
	ms := &mockStore{
		appConfig: map[string]string{"base_currency": "INR"},
		accessTokensByHash: map[string]*store.AccessToken{
			hash: {ID: "token-a", UserID: user.ID, TokenHash: hash},
		},
		usersByID: map[string]*store.User{user.ID: user},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})
	mux := http.NewServeMux()
	registerRoutes(mux, h)
	handler := authMiddleware(h, mux)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/config/preferences", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	if ms.lastAppConfigTenant.ID != "tenant-a" {
		t.Fatalf("request tenant = %q, want tenant-a", ms.lastAppConfigTenant.ID)
	}
}

func TestBootstrapCreatesAdminAndSessionCookie(t *testing.T) {
	ms := &mockStore{bootstrapRequired: true}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/api/bootstrap",
		strings.NewReader(`{"email":"admin@example.com","password":"correct horse battery staple","display_name":"Admin","avatar_key":"default"}`),
	)
	rec := httptest.NewRecorder()

	h.Bootstrap(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}
	if ms.createdBootstrapAdmin.Email != "admin@example.com" {
		t.Fatalf("created admin = %#v", ms.createdBootstrapAdmin)
	}
	if ms.createdBootstrapAdmin.PasswordHash == "" || strings.Contains(ms.createdBootstrapAdmin.PasswordHash, "correct horse") {
		t.Fatalf("password hash was not persisted safely: %#v", ms.createdBootstrapAdmin)
	}
	if cookie := findCookie(rec.Result().Cookies(), sessionCookieName); cookie == nil || cookie.Value == "" || !cookie.HttpOnly {
		t.Fatalf("session cookie = %#v, want HttpOnly cookie with value", cookie)
	}
}

func TestValidAvatarKeyAllowsCatalogKeys(t *testing.T) {
	for _, key := range []string{"default", "ledger", "wallet"} {
		if !ValidAvatarKey(key) {
			t.Fatalf("ValidAvatarKey(%q) = false, want true", key)
		}
	}
	if ValidAvatarKey("unknown") {
		t.Fatal(`ValidAvatarKey("unknown") = true, want false`)
	}
}

func TestBootstrapRejectsUnknownAvatarKey(t *testing.T) {
	ms := &mockStore{bootstrapRequired: true}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/api/bootstrap",
		strings.NewReader(`{"email":"admin@example.com","password":"correct horse battery staple","display_name":"Admin","avatar_key":"unknown"}`),
	)
	rec := httptest.NewRecorder()

	h.Bootstrap(rec, req)

	assertValidationError(t, rec, "avatar_key", "body", "must be one of: default, ledger, wallet")
	if ms.createdBootstrapAdmin.Email != "" {
		t.Fatalf("created admin = %#v, want no store write", ms.createdBootstrapAdmin)
	}
}

func TestCreateAccessTokenReturnsRawTokenOnce(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/tokens", strings.NewReader(`{"name":"cli"}`))
	rec := httptest.NewRecorder()

	h.CreateAccessToken(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}
	var resp accessTokenResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !strings.HasPrefix(resp.Token, accessTokenPrefix+"_") {
		t.Fatalf("token = %q, want %s_ prefix", resp.Token, accessTokenPrefix)
	}
	if ms.createdAccessToken.UserID != "user-a" || ms.createdAccessToken.Name != "cli" {
		t.Fatalf("created token = %#v", ms.createdAccessToken)
	}
	if strings.Contains(ms.createdAccessToken.TokenHash, resp.Token) {
		t.Fatalf("stored token hash contains raw token")
	}
}

func TestCreateAccessTokenNameConflictReturnsConflict(t *testing.T) {
	ms := &mockStore{createAccessTokenErr: store.ErrAccessTokenNameConflict}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/tokens", strings.NewReader(`{"name":"test"}`))
	rec := httptest.NewRecorder()

	h.CreateAccessToken(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", rec.Code, rec.Body.String())
	}
	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error != "Token test already exists." {
		t.Fatalf("error = %q, want duplicate token message", resp.Error)
	}
}

func TestListAccessTokensReturnsCurrentUserTokenMetadata(t *testing.T) {
	createdAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	lastUsedAt := createdAt.Add(time.Hour)
	ms := &mockStore{
		accessTokens: []store.AccessToken{
			{
				ID:         "token-a",
				UserID:     "user-a",
				Name:       "cli",
				TokenHash:  "sha256:secret",
				CreatedAt:  createdAt,
				LastUsedAt: &lastUsedAt,
			},
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/tokens", nil)
	rec := httptest.NewRecorder()

	h.ListAccessTokens(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	if ms.listedAccessTokensUserID != "user-a" {
		t.Fatalf("listed tokens for user = %q, want user-a", ms.listedAccessTokensUserID)
	}
	var resp []accessTokenResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp) != 1 || resp[0].ID != "token-a" || resp[0].Name != "cli" || resp[0].Token != "" {
		t.Fatalf("tokens response = %#v", resp)
	}
	if !resp[0].CreatedAt.Equal(createdAt) || resp[0].LastUsedAt == nil || !resp[0].LastUsedAt.Equal(lastUsedAt) {
		t.Fatalf("token timestamps = %#v", resp[0])
	}
	if strings.Contains(rec.Body.String(), "sha256:secret") {
		t.Fatalf("token response leaked token hash: %s", rec.Body.String())
	}
}

func TestUpdateProfileUpdatesCurrentUserOnly(t *testing.T) {
	ms := &mockStore{
		updatedUserResult: &store.User{
			ID:          "user-a",
			TenantID:    "tenant-a",
			Email:       "a@example.com",
			DisplayName: "Updated",
			Role:        store.UserRoleUser,
			AvatarKey:   "wallet",
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(
		ctx,
		http.MethodPatch,
		"/api/profile",
		strings.NewReader(`{"display_name":" Updated ","avatar_key":"wallet"}`),
	)
	rec := httptest.NewRecorder()

	h.UpdateProfile(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	if ms.updatedUserID != "user-a" {
		t.Fatalf("updated user id = %q, want user-a", ms.updatedUserID)
	}
	if ms.updatedUser.DisplayName == nil || *ms.updatedUser.DisplayName != "Updated" {
		t.Fatalf("updated display name = %#v", ms.updatedUser.DisplayName)
	}
	if ms.updatedUser.AvatarKey == nil || *ms.updatedUser.AvatarKey != "wallet" {
		t.Fatalf("updated avatar = %#v", ms.updatedUser.AvatarKey)
	}
	var resp principalResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.DisplayName != "Updated" || resp.AvatarKey != "wallet" {
		t.Fatalf("profile response = %#v", resp)
	}
}

func TestUpdateProfileRejectsUnknownAvatarKey(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/api/profile", strings.NewReader(`{"avatar_key":"unknown"}`))
	rec := httptest.NewRecorder()

	h.UpdateProfile(rec, req)

	assertValidationError(t, rec, "avatar_key", "body", "must be one of: default, ledger, wallet")
	if ms.updatedUserID != "" {
		t.Fatalf("updated user id = %q, want no store write", ms.updatedUserID)
	}
}

func TestCreateUserRequiresAdminRole(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/admin/users", strings.NewReader(`{"email":"b@example.com","display_name":"B"}`))
	rec := httptest.NewRecorder()

	h.CreateUser(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
	}
}

func TestCreateUserAsAdmin(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "admin", TenantID: "admin", Role: auth.RoleAdmin})
	req := httptest.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"/api/admin/users",
		strings.NewReader(`{"email":"b@example.com","display_name":"B","role":"user","avatar_key":"default"}`),
	)
	rec := httptest.NewRecorder()

	h.CreateUser(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}
	if ms.createdUser.Email != "b@example.com" || ms.createdUser.Role != store.UserRoleUser {
		t.Fatalf("created user = %#v", ms.createdUser)
	}
}

func TestCreateUserRejectsUnknownAvatarKey(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "admin", TenantID: "admin", Role: auth.RoleAdmin})
	req := httptest.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"/api/admin/users",
		strings.NewReader(`{"email":"b@example.com","display_name":"B","role":"user","avatar_key":"unknown"}`),
	)
	rec := httptest.NewRecorder()

	h.CreateUser(rec, req)

	assertValidationError(t, rec, "avatar_key", "body", "must be one of: default, ledger, wallet")
	if ms.createdUser.Email != "" {
		t.Fatalf("created user = %#v, want no store write", ms.createdUser)
	}
}

func TestListUsersRequiresAdminRole(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/admin/users", nil)
	rec := httptest.NewRecorder()

	h.ListUsers(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
	}
}

func TestListUsersAsAdmin(t *testing.T) {
	createdAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	disabledAt := createdAt.Add(time.Hour)
	ms := &mockStore{
		users: []store.User{
			{
				ID:          "admin",
				TenantID:    "admin",
				Email:       "admin@example.com",
				DisplayName: "Admin",
				Role:        store.UserRoleAdmin,
				AvatarKey:   "default",
				CreatedAt:   createdAt,
				UpdatedAt:   createdAt,
			},
			{
				ID:          "user-b",
				TenantID:    "user-b",
				Email:       "b@example.com",
				DisplayName: "B",
				Role:        store.UserRoleUser,
				AvatarKey:   "ledger",
				DisabledAt:  &disabledAt,
				CreatedAt:   createdAt,
				UpdatedAt:   disabledAt,
			},
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "admin", TenantID: "admin", Role: auth.RoleAdmin})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/admin/users", nil)
	rec := httptest.NewRecorder()

	h.ListUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var resp []userResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp) != 2 || resp[0].Email != "admin@example.com" || resp[1].DisabledAt == nil {
		t.Fatalf("users response = %#v", resp)
	}
}

func TestUpdateUserAsAdmin(t *testing.T) {
	ms := &mockStore{
		updatedUserResult: &store.User{
			ID:          "user-b",
			TenantID:    "user-b",
			Email:       "b@example.com",
			DisplayName: "B Updated",
			Role:        store.UserRoleAdmin,
			AvatarKey:   "wallet",
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "admin", TenantID: "admin", Role: auth.RoleAdmin})
	req := httptest.NewRequestWithContext(
		ctx,
		http.MethodPatch,
		"/api/admin/users/user-b",
		strings.NewReader(`{"display_name":" B Updated ","role":"admin","avatar_key":"wallet","disabled":true}`),
	)
	req.SetPathValue("id", "user-b")
	rec := httptest.NewRecorder()

	h.UpdateUser(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	if ms.updatedUserID != "user-b" {
		t.Fatalf("updated user id = %q, want user-b", ms.updatedUserID)
	}
	if ms.updatedUser.DisplayName == nil || *ms.updatedUser.DisplayName != "B Updated" {
		t.Fatalf("updated display name = %#v", ms.updatedUser.DisplayName)
	}
	if ms.updatedUser.Role == nil || *ms.updatedUser.Role != store.UserRoleAdmin {
		t.Fatalf("updated role = %#v", ms.updatedUser.Role)
	}
	if ms.updatedUser.AvatarKey == nil || *ms.updatedUser.AvatarKey != "wallet" {
		t.Fatalf("updated avatar = %#v", ms.updatedUser.AvatarKey)
	}
	if ms.updatedUser.Disabled == nil || !*ms.updatedUser.Disabled {
		t.Fatalf("updated disabled = %#v", ms.updatedUser.Disabled)
	}
}

func TestUpdateUserRejectsSelfRoleChange(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "admin", TenantID: "admin", Role: auth.RoleAdmin})
	req := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/api/admin/users/admin", strings.NewReader(`{"role":"user"}`))
	req.SetPathValue("id", "admin")
	rec := httptest.NewRecorder()

	h.UpdateUser(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
	}
	if ms.updatedUserID != "" {
		t.Fatalf("updated user id = %q, want no store write", ms.updatedUserID)
	}
}

func TestUpdateUserRejectsSelfDisable(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "admin", TenantID: "admin", Role: auth.RoleAdmin})
	req := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/api/admin/users/admin", strings.NewReader(`{"disabled":true}`))
	req.SetPathValue("id", "admin")
	rec := httptest.NewRecorder()

	h.UpdateUser(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
	}
	if ms.updatedUserID != "" {
		t.Fatalf("updated user id = %q, want no store write", ms.updatedUserID)
	}
}

func TestDeleteUserDeletesOtherUser(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "admin", TenantID: "admin", Role: auth.RoleAdmin})
	req := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/admin/users/user-b", nil)
	req.SetPathValue("id", "user-b")
	rec := httptest.NewRecorder()

	h.DeleteUser(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body = %s", rec.Code, rec.Body.String())
	}
	if ms.deletedUserID != "user-b" {
		t.Fatalf("deleted user id = %q, want user-b", ms.deletedUserID)
	}
}

func TestDeleteUserRejectsSelfDelete(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "admin", TenantID: "admin", Role: auth.RoleAdmin})
	req := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/admin/users/admin", nil)
	req.SetPathValue("id", "admin")
	rec := httptest.NewRecorder()

	h.DeleteUser(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
	}
	if ms.deletedUserID != "" {
		t.Fatalf("deleted user id = %q, want no store write", ms.deletedUserID)
	}
}

func TestUpdateUserRejectsInvalidRoleBeforeWriting(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "admin", TenantID: "admin", Role: auth.RoleAdmin})
	req := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/api/admin/users/user-b", strings.NewReader(`{"role":"owner"}`))
	req.SetPathValue("id", "user-b")
	rec := httptest.NewRecorder()

	h.UpdateUser(rec, req)

	assertValidationError(t, rec, "role", "body", "must be one of: admin, user")
	if ms.updatedUserID != "" {
		t.Fatalf("updated user id = %q, want no store write", ms.updatedUserID)
	}
}

func TestUpdateUserReportsAllSemanticValidationErrors(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "admin", TenantID: "admin", Role: auth.RoleAdmin})
	req := httptest.NewRequestWithContext(
		ctx,
		http.MethodPatch,
		"/api/admin/users/user-b",
		strings.NewReader(`{"display_name":" ","role":"owner","avatar_key":"unknown"}`),
	)
	req.SetPathValue("id", "user-b")
	rec := httptest.NewRecorder()

	h.UpdateUser(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body = %s", rec.Code, rec.Body.String())
	}
	var resp ValidationErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := []ValidationErrorDetail{
		{Field: "display_name", Location: "body", Message: "is required"},
		{Field: "avatar_key", Location: "body", Message: "must be one of: default, ledger, wallet"},
		{Field: "role", Location: "body", Message: "must be one of: admin, user"},
	}
	if !reflect.DeepEqual(resp.Details, want) {
		t.Fatalf("details = %#v, want %#v", resp.Details, want)
	}
	if ms.updatedUserID != "" {
		t.Fatalf("updated user id = %q, want no store write", ms.updatedUserID)
	}
}

func TestCreateSetupTokenAsAdmin(t *testing.T) {
	user := &store.User{ID: "user-b", TenantID: "user-b", Email: "b@example.com", DisplayName: "B", Role: store.UserRoleUser, AvatarKey: "default"}
	ms := &mockStore{usersByID: map[string]*store.User{user.ID: user}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "admin", TenantID: "admin", Role: auth.RoleAdmin})
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/admin/users/user-b/setup-tokens", nil)
	req.SetPathValue("id", user.ID)
	rec := httptest.NewRecorder()

	h.CreateSetupToken(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}
	var resp setupTokenResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !strings.HasPrefix(resp.Token, setupTokenPrefix+"_") {
		t.Fatalf("setup token = %q, want %s_ prefix", resp.Token, setupTokenPrefix)
	}
	if ms.createdSetupToken.UserID != user.ID || ms.createdSetupToken.TokenHash == "" {
		t.Fatalf("created setup token = %#v", ms.createdSetupToken)
	}
}

func TestCompleteAccountSetupSetsPasswordAndSessionCookie(t *testing.T) {
	raw, hash, err := auth.NewOpaqueToken(setupTokenPrefix)
	if err != nil {
		t.Fatalf("NewOpaqueToken() error = %v", err)
	}
	user := &store.User{ID: "user-b", TenantID: "user-b", Email: "b@example.com", DisplayName: "B", Role: store.UserRoleUser, AvatarKey: "default"}
	ms := &mockStore{completedSetupUser: user}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/api/account-setup",
		strings.NewReader(`{"token":"`+raw+`","password":"correct horse battery staple"}`),
	)
	rec := httptest.NewRecorder()

	h.CompleteAccountSetup(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}
	if ms.completedSetupTokenHash != hash {
		t.Fatalf("completed setup token hash = %q, want %q", ms.completedSetupTokenHash, hash)
	}
	if ms.completedSetupPasswordHash == "" || strings.Contains(ms.completedSetupPasswordHash, "correct horse") {
		t.Fatalf("password hash was not persisted safely: %#v", ms.completedSetupPasswordHash)
	}
	if ms.createdSession.UserID != user.ID || ms.createdSession.TokenHash == "" {
		t.Fatalf("created session = %#v", ms.createdSession)
	}
	if cookie := findCookie(rec.Result().Cookies(), sessionCookieName); cookie == nil || cookie.Value == "" || !cookie.HttpOnly {
		t.Fatalf("session cookie = %#v, want HttpOnly cookie with value", cookie)
	}
}

func TestLoginCreatesSessionCookie(t *testing.T) {
	hash, err := auth.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	user := &store.User{
		ID:           "user-a",
		TenantID:     "tenant-a",
		Email:        "a@example.com",
		PasswordHash: hash,
		DisplayName:  "Tenant A",
		Role:         store.UserRoleUser,
		AvatarKey:    "default",
	}
	ms := &mockStore{usersByEmail: map[string]*store.User{user.Email: user}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/api/session",
		strings.NewReader(`{"email":"a@example.com","password":"correct horse battery staple"}`),
	)
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}
	if ms.createdSession.UserID != user.ID || ms.createdSession.TokenHash == "" {
		t.Fatalf("created session = %#v", ms.createdSession)
	}
	if cookie := findCookie(rec.Result().Cookies(), sessionCookieName); cookie == nil || cookie.Value == "" || !cookie.HttpOnly {
		t.Fatalf("session cookie = %#v, want HttpOnly cookie with value", cookie)
	}
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}
