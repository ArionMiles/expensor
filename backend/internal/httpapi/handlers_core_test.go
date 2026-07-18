package httpapi

import (
	"context"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

func TestHealth(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	rr := get(h.Health, "/api/health")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", resp["status"])
	}
}

func TestStatus_WithStats(t *testing.T) {
	st := &mockStore{stats: &store.Stats{TotalCount: 42, TotalBase: 99999, BaseCurrency: "INR"}}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.Status, "/api/status")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	stats := resp["stats"].(map[string]any)
	if stats["total_count"] != float64(42) {
		t.Errorf("expected stats.total_count=42, got %v", stats["total_count"])
	}
}

func TestStatus_StatsError(t *testing.T) {
	st := &mockStore{statsErr: stderrors.New("stats failed")}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.Status, "/api/status")

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["message"] != unexpectedErrorMessage {
		t.Errorf("message = %q, want %q", resp["message"], unexpectedErrorMessage)
	}
}

func TestStatusWithoutControllerReturnsStoppedStatus(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	h.daemon = nil
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/status", nil)
	rr := httptest.NewRecorder()
	h.Status(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var response StatusResponse
	decodeJSON(t, rr.Body.String(), &response)
	if response.Daemon.Running {
		t.Fatal("daemon status = running, want stopped")
	}
}
