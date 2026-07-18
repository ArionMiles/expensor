package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/auth"
)

func TestReaderStatus_Thunderbird_NotConfigured(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/providers/thunderbird/status", nil)
	req.SetPathValue("name", "thunderbird")
	rr := httptest.NewRecorder()
	h.ReaderStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["ready"] != false {
		t.Error("thunderbird without config should not be ready")
	}
}

func TestReaderStatus_Thunderbird_Configured(t *testing.T) {
	ms := &mockStore{
		readerConfigs: map[string]json.RawMessage{
			"tenant-a/thunderbird": json.RawMessage(`{"profilePath":"/tmp/tb"}`),
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})

	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/providers/thunderbird/status", nil)
	req.SetPathValue("name", "thunderbird")
	rr := httptest.NewRecorder()
	h.ReaderStatus(rr, req)

	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["ready"] != true {
		t.Errorf("thunderbird with config should be ready, got %v", resp)
	}
}

func TestSaveReaderConfig_SavesToStore(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(
		ctx,
		http.MethodPut,
		"/api/providers/thunderbird/config",
		strings.NewReader(`{"config":{"mailboxes":"Inbox"}}`),
	)
	req.SetPathValue("name", "thunderbird")
	rr := httptest.NewRecorder()

	h.SaveReaderConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(ms.readerConfigs["tenant-a/thunderbird"], []byte("Inbox")) {
		t.Fatalf("config not persisted: %s", ms.readerConfigs["tenant-a/thunderbird"])
	}
}

func TestGetReaderConfig_LoadsFromStore(t *testing.T) {
	ms := &mockStore{
		readerConfigs: map[string]json.RawMessage{
			"tenant-a/thunderbird": json.RawMessage(`{"config":{"mailboxes":"Inbox"}}`),
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/providers/thunderbird/config", nil)
	req.SetPathValue("name", "thunderbird")
	rr := httptest.NewRecorder()

	h.GetReaderConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte("Inbox")) {
		t.Fatalf("config response = %s", rr.Body.String())
	}
}
