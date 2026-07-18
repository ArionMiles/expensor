package httpapi

import (
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	apperrors "github.com/ArionMiles/expensor/backend/pkg/errors"
)

func TestWriteErrorUsesUserMessageAndRequestID(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.Header().Set(requestIDHeader, "7b08e51d-8e8f-4b4a-9c14-fd1ff4c823b3")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/transactions/1", nil)

	writeError(rr, req, apperrors.E(apperrors.NotFound, apperrors.User("transaction not found")))

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
	var response ErrorResponse
	decodeJSON(t, rr.Body.String(), &response)
	if response.Message != "transaction not found" {
		t.Fatalf("message = %q, want transaction not found", response.Message)
	}
	if response.RequestID != "7b08e51d-8e8f-4b4a-9c14-fd1ff4c823b3" {
		t.Fatalf("request_id = %q", response.RequestID)
	}
	if len(response.ValidationErrors) != 0 {
		t.Fatalf("validation_errors = %#v, want empty", response.ValidationErrors)
	}
}

func TestWriteErrorHidesUnexpectedError(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/transactions/1", nil)

	writeError(rr, req, apperrors.E(apperrors.Internal, stderrors.New("database password leaked")))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
	var response ErrorResponse
	decodeJSON(t, rr.Body.String(), &response)
	if response.Message != "Something went wrong." {
		t.Fatalf("message = %q", response.Message)
	}
	if _, err := uuid.Parse(response.RequestID); err != nil {
		t.Fatalf("request_id = %q, want UUID: %v", response.RequestID, err)
	}
}

func TestWriteErrorHidesClientErrorWithoutUserMessage(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/transactions/1", nil)

	writeError(rr, req, apperrors.E(apperrors.InvalidArgument, stderrors.New("invalid internal state")))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
	var response ErrorResponse
	decodeJSON(t, rr.Body.String(), &response)
	if response.Message != unexpectedErrorMessage {
		t.Fatalf("message = %q", response.Message)
	}
}

func TestWriteValidationErrorsUsesUnifiedErrorResponse(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.Header().Set(requestIDHeader, "7b08e51d-8e8f-4b4a-9c14-fd1ff4c823b3")
	validationErrors := []ValidationError{{Field: "page_size", Location: "query", Message: "must be at most 100"}}

	writeValidationErrors(rr, validationErrors)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
	}
	var response ErrorResponse
	decodeJSON(t, rr.Body.String(), &response)
	if response.Message != "Request validation failed." {
		t.Fatalf("message = %q", response.Message)
	}
	if response.RequestID != "7b08e51d-8e8f-4b4a-9c14-fd1ff4c823b3" {
		t.Fatalf("request_id = %q", response.RequestID)
	}
	if len(response.ValidationErrors) != 1 || response.ValidationErrors[0] != validationErrors[0] {
		t.Fatalf("validation_errors = %#v, want %#v", response.ValidationErrors, validationErrors)
	}
}

func TestRequestIDMiddlewareGeneratesAndPropagatesServerRequestID(t *testing.T) {
	var requestID string
	handler := requestIDMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		requestID = requestIDFromContext(r.Context())
	}))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/health", nil)
	req.Header.Set(requestIDHeader, "client-controlled")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if _, err := uuid.Parse(requestID); err != nil {
		t.Fatalf("request ID = %q, want UUID: %v", requestID, err)
	}
	if requestID == "client-controlled" {
		t.Fatal("request ID must not trust client input")
	}
	if rr.Header().Get(requestIDHeader) != requestID {
		t.Fatalf("response request ID = %q, want %q", rr.Header().Get(requestIDHeader), requestID)
	}
}

func TestAPIErrorFallbackUsesUnifiedErrorResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/widgets", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := apiErrorFallback(mux)

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantMsg    string
	}{
		{
			name:       "unknown API endpoint",
			method:     http.MethodGet,
			path:       "/api/missing",
			wantStatus: http.StatusNotFound,
			wantMsg:    "API endpoint not found.",
		},
		{
			name:       "unsupported API method",
			method:     http.MethodPost,
			path:       "/api/widgets",
			wantStatus: http.StatusMethodNotAllowed,
			wantMsg:    "Method not allowed.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), tt.method, tt.path, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
			var response ErrorResponse
			decodeJSON(t, rr.Body.String(), &response)
			if response.Message != tt.wantMsg {
				t.Fatalf("message = %q, want %q", response.Message, tt.wantMsg)
			}
			if response.RequestID == "" || rr.Header().Get(requestIDHeader) != response.RequestID {
				t.Fatalf("request ID header = %q, response = %q", rr.Header().Get(requestIDHeader), response.RequestID)
			}
		})
	}
}

func TestAPIErrorFallbackPreservesPathValuesForMatchedRoutes(t *testing.T) {
	mux := http.NewServeMux()
	var gotID string
	mux.HandleFunc("GET /api/widgets/{id}", func(w http.ResponseWriter, r *http.Request) {
		gotID = r.PathValue("id")
		w.WriteHeader(http.StatusNoContent)
	})
	handler := apiErrorFallback(mux)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/widgets/widget-123", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if gotID != "widget-123" {
		t.Fatalf("path value = %q, want %q", gotID, "widget-123")
	}
}
