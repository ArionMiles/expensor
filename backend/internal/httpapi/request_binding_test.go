package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type requestBindingFixture struct {
	Name string `json:"name" validate:"required,no_control_chars"`
}

type nestedRequestBindingFixture struct {
	Source RuleSourceResponse `json:"source"`
}

func TestDecodeAndValidateJSON_RejectsMalformedJSON(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/example", strings.NewReader("not-json"))
	rr := httptest.NewRecorder()

	if _, ok := decodeAndValidateJSON[requestBindingFixture](h, rr, req); ok {
		t.Fatal("expected JSON decoding to fail")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (body=%s)", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestDecodeAndValidateJSON_ReportsSemanticValidationErrors(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/example", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()

	if _, ok := decodeAndValidateJSON[requestBindingFixture](h, rr, req); ok {
		t.Fatal("expected request validation to fail")
	}
	assertValidationError(t, rr, "name", "body", "is required")
}

func TestDecodeAndValidateJSON_ReportsNestedFieldPaths(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/example",
		strings.NewReader(`{"source":{"type":"Email"}}`),
	)
	rr := httptest.NewRecorder()

	if _, ok := decodeAndValidateJSON[nestedRequestBindingFixture](h, rr, req); ok {
		t.Fatal("expected request validation to fail")
	}
	assertValidationError(t, rr, "source.bank", "body", "is required")
}
