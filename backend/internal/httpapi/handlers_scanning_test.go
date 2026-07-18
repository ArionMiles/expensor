package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/auth"
	"github.com/ArionMiles/expensor/backend/internal/daemon"
)

func TestStartDaemon_DaemonRunning_CallsStartFnWithRequestedReader(t *testing.T) {
	var started daemon.RunRequest
	ms := &mockStore{}
	dm := &mockDaemon{status: DaemonStatus{Running: true}}
	h := newTestHandlers(t, ms, dm)
	h.daemon.(*mockDaemon).startFn = func(req daemon.RunRequest) { started = req }

	body := `{"reader":"thunderbird"}`
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/daemon/start", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.StartDaemon(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if started.Reader != "thunderbird" {
		t.Fatalf("startFn reader = %q, want thunderbird", started.Reader)
	}
	if started.Tenant.ID != "tenant-a" {
		t.Fatalf("startFn tenant = %q, want tenant-a", started.Tenant.ID)
	}
}

func TestStartDaemon_RejectsMissingReader(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	h.daemon.(*mockDaemon).startFn = func(daemon.RunRequest) {}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/daemon/start", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	h.StartDaemon(rr, req)

	assertValidationError(t, rr, "reader", "body", "is required")
}

func TestStartDaemonWithoutControllerReturns501(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	h.daemon = nil
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/daemon/start", strings.NewReader(`{"reader":"gmail"}`))
	rr := httptest.NewRecorder()
	h.StartDaemon(rr, req)
	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501", rr.Code)
	}
}

func TestTriggerSyncWithoutServiceReturns503(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/config/sync", nil)
	rr := httptest.NewRecorder()
	h.TriggerSync(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}
}

func TestRescan_DaemonRunning_Returns202Rescanning(t *testing.T) {
	var rescan daemon.RunRequest
	ms := &mockStore{}
	dm := &mockDaemon{status: DaemonStatus{Running: true}}
	h := newTestHandlers(t, ms, dm)
	h.daemon.(*mockDaemon).rescanFn = func(req daemon.RunRequest) { rescan = req }

	body := `{"reader":"gmail"}`
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/daemon/rescan", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.Rescan(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["status"] != "rescanning" {
		t.Errorf("expected status=rescanning, got %q", resp["status"])
	}
	if rescan.Reader != "gmail" {
		t.Fatalf("rescanFn reader = %q, want gmail", rescan.Reader)
	}
	if rescan.Tenant.ID != "tenant-a" {
		t.Fatalf("rescanFn tenant = %q, want tenant-a", rescan.Tenant.ID)
	}
}

func TestRescan_DaemonNotRunning_Returns202Rescanning(t *testing.T) {
	var rescan daemon.RunRequest
	ms := &mockStore{}
	dm := &mockDaemon{status: DaemonStatus{Running: false}}
	h := newTestHandlers(t, ms, dm)
	h.daemon.(*mockDaemon).rescanFn = func(req daemon.RunRequest) { rescan = req }

	body := `{"reader":"gmail"}`
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/daemon/rescan", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.Rescan(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["status"] != "rescanning" {
		t.Errorf("expected status=rescanning, got %q", resp["status"])
	}
	if rescan.Reader != "gmail" {
		t.Fatalf("rescanFn reader = %q, want gmail", rescan.Reader)
	}
	if rescan.Tenant.ID != "tenant-a" {
		t.Fatalf("rescanFn tenant = %q, want tenant-a", rescan.Tenant.ID)
	}
}
