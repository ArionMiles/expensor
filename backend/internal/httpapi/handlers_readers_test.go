package httpapi

import (
	"net/http"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/plugins"
)

func TestListProviders(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	rr := get(h.ListProviders, "/api/providers")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var readers []ProviderInfo
	decodeJSON(t, rr.Body.String(), &readers)
	if len(readers) != 2 {
		t.Fatalf("expected 2 readers, got %d", len(readers))
	}
	// Verify gmail metadata.
	var gmail ProviderInfo
	for _, r := range readers {
		if r.Name == "gmail" {
			gmail = r
		}
	}
	if gmail.AuthType != plugins.AuthTypeOAuth {
		t.Errorf("gmail auth_type: want oauth, got %s", gmail.AuthType)
	}
	if !gmail.RequiresCredentialsUpload {
		t.Errorf("gmail should require credentials upload")
	}
}

func TestListReaders_NormalizesNilConfigSchema(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	registry := plugins.NewRegistry()
	if err := registry.RegisterProvider((&testProvider{
		name:              "nil-schema",
		authType:          plugins.AuthTypeConfig,
		preserveNilSchema: true,
	}).provider()); err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}
	h.registry = registry

	rr := get(h.ListProviders, "/api/providers")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var readers []ProviderInfo
	decodeJSON(t, rr.Body.String(), &readers)
	if len(readers) != 1 {
		t.Fatalf("expected 1 reader, got %d", len(readers))
	}
	if readers[0].ConfigSchema == nil {
		t.Fatalf("config_schema = nil, want non-nil empty slice; body = %s", rr.Body.String())
	}
	if len(readers[0].ConfigSchema) != 0 {
		t.Fatalf("config_schema len = %d, want 0", len(readers[0].ConfigSchema))
	}
}
