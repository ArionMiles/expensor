package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"go.opentelemetry.io/otel/trace"

	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

const unexpectedErrorMessage = "Something went wrong."

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	requestID := responseRequestID(w)
	status := errors.StatusCode(err)
	message := errors.UserMsg(err)
	if message == "" || status >= http.StatusInternalServerError {
		logError(r, requestID, err)
	}
	if message == "" {
		message = unexpectedErrorMessage
	}
	writeErrorResponse(w, status, message)
}

// writeErrorResponse serializes a safe, already-decided HTTP error response.
// Prefer writeError when an application error is available so its status, safe
// message, request ID, and log context are derived consistently.
func writeErrorResponse(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{
		Message:   message,
		RequestID: responseRequestID(w),
	})
}

func logError(r *http.Request, requestID string, err error) {
	attrs := []slog.Attr{
		slog.String("request_id", requestID),
		slog.Any("error", err),
	}
	attrs = append(attrs, errors.LogDetailAttrs(err)...)
	if spanContext := trace.SpanFromContext(r.Context()).SpanContext(); spanContext.IsValid() {
		attrs = append(attrs,
			slog.String("trace_id", spanContext.TraceID().String()),
			slog.String("span_id", spanContext.SpanID().String()),
		)
	}
	slog.LogAttrs(r.Context(), slog.LevelError, "request failed", attrs...)
}
