package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/plugins"
)

func TestDiscoverProfiles_Returns200WithProfilesKey(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/providers/thunderbird/discover/profiles", nil)
	rr := httptest.NewRecorder()
	h.DiscoverProfiles(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string][]string
	decodeJSON(t, rr.Body.String(), &resp)
	if _, ok := resp["profiles"]; !ok {
		t.Error("expected 'profiles' key in response")
	}
}

func TestDiscoverMailboxes_MissingParam_Returns400(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/providers/thunderbird/discover/mailboxes", nil)
	rr := httptest.NewRecorder()
	h.DiscoverMailboxes(rr, req)

	assertValidationError(t, rr, "profile", "query", "is required")
}

func TestDiscoverMailboxes_NonexistentProfile_Returns404(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodGet,
		"/api/providers/thunderbird/discover/mailboxes?profile=/nonexistent/thunderbird/profile",
		nil,
	)
	rr := httptest.NewRecorder()
	h.DiscoverMailboxes(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestGetProviderGuide_NoGuide_Returns404(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	registry := plugins.NewRegistry()
	if err := registry.RegisterProvider((&testProvider{name: "noguide", authType: plugins.AuthTypeConfig}).provider()); err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}
	h.registry = registry

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/providers/noguide/guide", nil)
	req.SetPathValue("name", "noguide")
	rr := httptest.NewRecorder()

	h.GetProviderGuide(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestGetProviderGuide_ReturnsMetadataGuide(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	registry := plugins.NewRegistry()
	guide := json.RawMessage(`{"sections":[{"title":"Setup","steps":[{"text":"Do the setup"}]}]}`)
	if err := registry.RegisterProvider((&testProvider{name: "guided", authType: plugins.AuthTypeConfig, guide: guide}).provider()); err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}
	h.registry = registry

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/providers/guided/guide", nil)
	req.SetPathValue("name", "guided")
	rr := httptest.NewRecorder()

	h.GetProviderGuide(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Do the setup") {
		t.Fatalf("body = %s, want setup guide", rr.Body.String())
	}
}
