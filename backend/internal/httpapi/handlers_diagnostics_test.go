package httpapi

import (
	"context"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

func TestListExtractionDiagnostics_DefaultsStatusOpen(t *testing.T) {
	st := &mockStore{diagnostics: []store.ExtractionDiagnosticRow{{ID: "diag-1", Status: store.DiagnosticStatusOpen}}}
	h := newTestHandlers(t, st, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/extraction-diagnostics", nil)
	rr := httptest.NewRecorder()
	h.ListExtractionDiagnostics(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	if st.diagnosticFilter.Status != store.DiagnosticStatusOpen {
		t.Fatalf("expected status open, got %q", st.diagnosticFilter.Status)
	}
	var resp []store.ExtractionDiagnosticRow
	decodeJSON(t, rr.Body.String(), &resp)
	if len(resp) != 1 || resp[0].ID != "diag-1" {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestListExtractionDiagnostics_StatusAllAndLimit(t *testing.T) {
	st := &mockStore{}
	h := newTestHandlers(t, st, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/extraction-diagnostics?status=all&limit=25", nil)
	rr := httptest.NewRecorder()
	h.ListExtractionDiagnostics(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	if st.diagnosticFilter.Status != store.DiagnosticStatusAll {
		t.Fatalf("expected status all, got %q", st.diagnosticFilter.Status)
	}
	if st.diagnosticFilter.Limit != 25 {
		t.Fatalf("expected limit 25, got %d", st.diagnosticFilter.Limit)
	}
}

func TestListExtractionDiagnostics_RejectsInvalidQuery(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		field   string
		message string
	}{
		{name: "status", query: "status=pending", field: "status", message: "must be one of: open, resolved, ignored, all"},
		{name: "limit syntax", query: "limit=bad", field: "limit", message: "must be an integer"},
		{name: "limit range", query: "limit=0", field: "limit", message: "must be at least 1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
			req := httptest.NewRequestWithContext(
				context.Background(),
				http.MethodGet,
				"/api/extraction-diagnostics?"+tt.query,
				nil,
			)
			rr := httptest.NewRecorder()
			h.ListExtractionDiagnostics(rr, req)

			assertValidationError(t, rr, tt.field, "query", tt.message)
		})
	}
}

func TestGetExtractionDiagnostic_Found(t *testing.T) {
	row := &store.ExtractionDiagnosticRow{ID: "11111111-1111-1111-1111-111111111111", Status: store.DiagnosticStatusOpen}
	h := newTestHandlers(t, &mockStore{diagnosticResult: row}, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/extraction-diagnostics/11111111-1111-1111-1111-111111111111", nil)
	req.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
	rr := httptest.NewRecorder()
	h.GetExtractionDiagnostic(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	var resp store.ExtractionDiagnosticRow
	decodeJSON(t, rr.Body.String(), &resp)
	if resp.ID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("expected UUID id, got %q", resp.ID)
	}
}

func TestGetExtractionDiagnostic_NotFound(t *testing.T) {
	h := newTestHandlers(t, &mockStore{diagnosticErr: errStoreNotFound}, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/extraction-diagnostics/22222222-2222-2222-2222-222222222222", nil)
	req.SetPathValue("id", "22222222-2222-2222-2222-222222222222")
	rr := httptest.NewRecorder()
	h.GetExtractionDiagnostic(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestGetExtractionDiagnostic_InvalidID(t *testing.T) {
	h := newTestHandlers(t, &mockStore{diagnosticErr: stderrors.New("store should not be called")}, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/extraction-diagnostics/not-a-uuid", nil)
	req.SetPathValue("id", "not-a-uuid")
	rr := httptest.NewRecorder()
	h.GetExtractionDiagnostic(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestUpdateExtractionDiagnosticStatus_InvalidStatus(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPatch,
		"/api/extraction-diagnostics/11111111-1111-1111-1111-111111111111",
		strings.NewReader(`{"status":"all"}`),
	)
	req.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
	rr := httptest.NewRecorder()
	h.UpdateExtractionDiagnosticStatus(rr, req)

	assertValidationError(t, rr, "status", "body", "must be one of: open, resolved, ignored")
}

func TestUpdateExtractionDiagnosticStatus_MissingStatus(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPatch,
		"/api/extraction-diagnostics/11111111-1111-1111-1111-111111111111",
		strings.NewReader(`{}`),
	)
	req.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
	rr := httptest.NewRecorder()
	h.UpdateExtractionDiagnosticStatus(rr, req)

	assertValidationError(t, rr, "status", "body", "is required")
}

func TestUpdateExtractionDiagnosticStatus_InvalidJSON(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPatch,
		"/api/extraction-diagnostics/11111111-1111-1111-1111-111111111111",
		strings.NewReader("not-json"),
	)
	req.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
	rr := httptest.NewRecorder()
	h.UpdateExtractionDiagnosticStatus(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body=%s)", rr.Code, rr.Body.String())
	}
}

func TestUpdateExtractionDiagnosticStatus_NotFound(t *testing.T) {
	h := newTestHandlers(t, &mockStore{diagnosticErr: errStoreNotFound}, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPatch,
		"/api/extraction-diagnostics/33333333-3333-3333-3333-333333333333",
		strings.NewReader(`{"status":"resolved"}`),
	)
	req.SetPathValue("id", "33333333-3333-3333-3333-333333333333")
	rr := httptest.NewRecorder()
	h.UpdateExtractionDiagnosticStatus(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestUpdateExtractionDiagnosticStatus_InvalidID(t *testing.T) {
	h := newTestHandlers(t, &mockStore{diagnosticErr: stderrors.New("store should not be called")}, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPatch,
		"/api/extraction-diagnostics/not-a-uuid",
		strings.NewReader(`{"status":"resolved"}`),
	)
	req.SetPathValue("id", "not-a-uuid")
	rr := httptest.NewRecorder()
	h.UpdateExtractionDiagnosticStatus(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestUpdateExtractionDiagnosticStatus_Conflict(t *testing.T) {
	h := newTestHandlers(t, &mockStore{diagnosticErr: errStoreDiagnosticConflict}, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPatch,
		"/api/extraction-diagnostics/44444444-4444-4444-4444-444444444444",
		strings.NewReader(`{"status":"open"}`),
	)
	req.SetPathValue("id", "44444444-4444-4444-4444-444444444444")
	rr := httptest.NewRecorder()
	h.UpdateExtractionDiagnosticStatus(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}
