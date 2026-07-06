package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/auth"
)

func TestGetAdminLoggingSettingsRequiresAdminRole(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user", TenantID: "user", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/admin/logging/settings", nil)
	rec := httptest.NewRecorder()

	h.GetAdminLoggingSettings(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (body=%s)", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestPatchAdminLoggingSettingsUpdatesRuntimeLevel(t *testing.T) {
	var level slog.LevelVar
	level.Set(slog.LevelInfo)
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	h.logLevel = &level

	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "admin", TenantID: "admin", Role: auth.RoleAdmin})
	req := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/api/admin/logging/settings", strings.NewReader(`{"level":"debug"}`))
	rec := httptest.NewRecorder()

	h.PatchAdminLoggingSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := level.Level(); got != slog.LevelDebug {
		t.Fatalf("log level = %s, want %s", got, slog.LevelDebug)
	}
	var body AdminLoggingSettingsResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Level != "debug" {
		t.Fatalf("response level = %q, want debug", body.Level)
	}
}

func TestPatchAdminLoggingSettingsRejectsUnsupportedLevel(t *testing.T) {
	var level slog.LevelVar
	level.Set(slog.LevelInfo)
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	h.logLevel = &level

	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "admin", TenantID: "admin", Role: auth.RoleAdmin})
	req := httptest.NewRequestWithContext(ctx, http.MethodPatch, "/api/admin/logging/settings", strings.NewReader(`{"level":"trace"}`))
	rec := httptest.NewRecorder()

	h.PatchAdminLoggingSettings(rec, req)

	assertValidationError(t, rec, "level", "body", "must be one of: debug, info, warn, error")
	if got := level.Level(); got != slog.LevelInfo {
		t.Fatalf("log level changed after invalid request: %s", got)
	}
}
