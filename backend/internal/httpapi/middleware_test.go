package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/ArionMiles/expensor/backend/internal/observability"
)

func TestRecoveryMiddlewareContentType(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	handler := recoveryMiddleware(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
	if _, ok := body["error"]; !ok {
		t.Errorf("expected JSON body to contain 'error' key, got %v", body)
	}
}

func TestObservabilityMiddlewareCreatesHTTPRequestSpan(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(t.Context())
		otel.SetTracerProvider(noop.NewTracerProvider())
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/transactions/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	scope := observability.NewScope(logger, "test/http")
	handler := observabilityMiddleware(scope, mux)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/transactions/123", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusAccepted)
	}
	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(spans))
	}
	if spans[0].Name() != "http.server.request" {
		t.Fatalf("span name = %q, want http.server.request", spans[0].Name())
	}
	attrs := spans[0].Attributes()
	wantAttrs := map[string]string{
		"http.request.method": "GET",
		"http.route":          "GET /api/transactions/{id}",
	}
	for key, want := range wantAttrs {
		found := false
		for _, attr := range attrs {
			if string(attr.Key) == key && attr.Value.AsString() == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("span attributes missing %s=%q: %#v", key, want, attrs)
		}
	}
}
