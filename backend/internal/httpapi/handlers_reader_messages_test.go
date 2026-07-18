package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/auth"
	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/pkg/api"
)

func TestSearchReaderMessages_ReturnsSamples(t *testing.T) {
	receivedAt := time.Date(2026, 6, 1, 10, 30, 0, 0, time.UTC)
	reader := &testSearchReader{result: []api.EmailSearchResult{
		{
			ID:          "message-1",
			SenderEmail: "alerts@example.com",
			Subject:     "Card spend approved",
			Body:        "INR 42.00 at Coffee",
			ReceivedAt:  &receivedAt,
		},
	}}
	provider := &testProvider{
		name:     "sample",
		authType: plugins.AuthTypeConfig,
		reader:   reader,
	}
	registry := plugins.NewRegistry()
	if err := registry.RegisterProvider(provider.provider()); err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	h.registry = registry

	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/providers/sample/messages?subject=spend&limit=3", nil)
	req.SetPathValue("name", "sample")
	rr := httptest.NewRecorder()
	h.SearchProviderMessages(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if reader.query.SubjectQuery != "spend" {
		t.Fatalf("subject search = %q, want spend", reader.query.SubjectQuery)
	}
	if reader.query.Limit != 3 {
		t.Fatalf("limit = %d, want 3", reader.query.Limit)
	}
	var resp struct {
		Results []ProviderSearchResultResponse `json:"results"`
	}
	decodeJSON(t, rr.Body.String(), &resp)
	if len(resp.Results) != 1 {
		t.Fatalf("results len = %d, want 1", len(resp.Results))
	}
	if resp.Results[0].SenderEmail != "alerts@example.com" || resp.Results[0].Body != "INR 42.00 at Coffee" {
		t.Fatalf("unexpected message response: %#v", resp.Results[0])
	}
	if resp.Results[0].ReceivedAt == nil || !resp.Results[0].ReceivedAt.Equal(receivedAt) {
		t.Fatalf("received_at = %v, want %v", resp.Results[0].ReceivedAt, receivedAt)
	}
}

func TestSearchReaderMessages_MissingSubjectReturns422(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/providers/gmail/messages", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()

	h.SearchProviderMessages(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}
