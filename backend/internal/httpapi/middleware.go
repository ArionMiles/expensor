package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/ArionMiles/expensor/backend/internal/observability"
)

// loggingMiddleware logs method, path, status, and duration for every request.
func loggingMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		logger.Debug("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

// observabilityMiddleware records low-cardinality HTTP request telemetry.
func observabilityMiddleware(scope *observability.Scope, next http.Handler) http.Handler {
	if scope == nil {
		scope = observability.NewScope(slog.Default(), "github.com/ArionMiles/expensor/backend/internal/httpapi")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := scope.Start(r.Context(), "http.server.request")
		defer span.End()

		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		req := r.WithContext(ctx)
		next.ServeHTTP(rw, req)

		route := req.Pattern
		if route == "" {
			route = "unmatched"
		}
		span.SetAttributes(
			attribute.String("http.request.method", r.Method),
			attribute.String("http.route", route),
			attribute.Int("http.response.status_code", rw.status),
		)
		scope.RecordDuration(ctx, observability.DurationOperation{
			Namespace:  "http",
			Name:       "server.request",
			Duration:   time.Since(start),
			StatusCode: rw.status,
			Attributes: []attribute.KeyValue{
				attribute.String("method", r.Method),
				attribute.String("route", route),
			},
		})
	})
}

// corsMiddleware adds permissive CORS headers for local development (Vite dev server).
// In production the frontend is served by the same origin, so these headers are no-ops.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// recoveryMiddleware catches panics and returns a 500 response.
func recoveryMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error("panic recovered", "panic", rec)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":"internal server error"}`))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}
