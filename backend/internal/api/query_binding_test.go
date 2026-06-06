package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type queryBindingFixture struct {
	Page     *int       `form:"page"`
	DateFrom *time.Time `form:"date_from"`
	Ignored  string
}

func TestDecodeQuery_DecodesTypedValues(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	var query queryBindingFixture
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/example?page=3&date_from=2026-04-30T18:30:00.000Z&Ignored=not-bound",
		nil,
	)
	rr := httptest.NewRecorder()

	if !h.decodeQuery(rr, req, &query) {
		t.Fatalf("decode failed: status=%d body=%s", rr.Code, rr.Body.String())
	}
	if query.Page == nil || *query.Page != 3 {
		t.Fatalf("page = %#v", query.Page)
	}
	if query.DateFrom == nil || query.DateFrom.Format(time.RFC3339Nano) != "2026-04-30T18:30:00Z" {
		t.Fatalf("date_from = %#v", query.DateFrom)
	}
	if query.Ignored != "" {
		t.Fatalf("untagged field was bound: %q", query.Ignored)
	}
}

func TestDecodeQuery_LeavesAbsentPointerValuesNil(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	var query queryBindingFixture
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/example", nil)
	rr := httptest.NewRecorder()

	if !h.decodeQuery(rr, req, &query) {
		t.Fatalf("decode failed: status=%d body=%s", rr.Code, rr.Body.String())
	}
	if query.Page != nil || query.DateFrom != nil {
		t.Fatalf("query = %#v", query)
	}
}

func TestDecodeQuery_ReportsTypedConversionFailures(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		field   string
		message string
	}{
		{
			name:    "integer",
			query:   "page=many",
			field:   "page",
			message: "must be an integer",
		},
		{
			name:    "timestamp",
			query:   "date_from=yesterday",
			field:   "date_from",
			message: "must be an RFC3339 timestamp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
			var query queryBindingFixture
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/example?"+tt.query, nil)
			rr := httptest.NewRecorder()

			if h.decodeQuery(rr, req, &query) {
				t.Fatal("expected decode failure")
			}
			assertValidationError(t, rr, tt.field, "query", tt.message)
		})
	}
}
