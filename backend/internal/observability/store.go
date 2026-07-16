package observability

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

const errorStatus = "error"

// Operation describes one low-cardinality operation measurement.
type Operation struct {
	Namespace string
	Name      string
	Err       error
}

// DurationOperation describes one low-cardinality duration measurement.
type DurationOperation struct {
	Namespace  string
	Name       string
	Duration   time.Duration
	StatusCode int
	Err        error
	Attributes []attribute.KeyValue
}

// RecordOperation records a span result, metrics, and a trace-aware log entry.
func (s *Scope) RecordOperation(ctx context.Context, op Operation) {
	status := "ok"
	if op.Err != nil {
		status = errorStatus
	}

	attrs := []attribute.KeyValue{
		attribute.String("namespace", op.Namespace),
		attribute.String("operation", op.Name),
		attribute.String("status", status),
	}

	counter, _ := s.meter.Int64Counter(op.Namespace + ".operations")
	counter.Add(ctx, 1, metric.WithAttributes(attrs...))

	span := trace.SpanFromContext(ctx)
	logAttrs := []slog.Attr{
		slog.String("namespace", op.Namespace),
		slog.String("operation", op.Name),
	}
	if spanContext := span.SpanContext(); spanContext.IsValid() {
		logAttrs = append(logAttrs,
			slog.String("trace_id", spanContext.TraceID().String()),
			slog.String("span_id", spanContext.SpanID().String()),
		)
	}

	if op.Err != nil {
		span.SetStatus(codes.Error, errorStatus)
		logAttrs = append(logAttrs, slog.Any("error", op.Err))
		logAttrs = append(logAttrs, errors.LogDetailAttrs(op.Err)...)
		s.logger.LogAttrs(ctx, slog.LevelError, "operation failed", logAttrs...)
		return
	}

	s.logger.LogAttrs(ctx, slog.LevelDebug, "operation completed", logAttrs...)
}

// RecordDuration records request or batch latency with low-cardinality attributes.
func (s *Scope) RecordDuration(ctx context.Context, op DurationOperation) {
	status := "ok"
	if op.Err != nil || op.StatusCode >= 500 {
		status = errorStatus
	}
	attrs := []attribute.KeyValue{
		attribute.String("namespace", op.Namespace),
		attribute.String("operation", op.Name),
		attribute.String("status", status),
	}
	attrs = append(attrs, op.Attributes...)
	if op.StatusCode > 0 {
		attrs = append(attrs, attribute.Int("status_code", op.StatusCode))
	}

	counter, _ := s.meter.Int64Counter(op.Namespace + ".operations")
	counter.Add(ctx, 1, metric.WithAttributes(attrs...))
	histogram, _ := s.meter.Float64Histogram(op.Namespace + ".duration_ms")
	histogram.Record(ctx, float64(op.Duration.Milliseconds()), metric.WithAttributes(attrs...))

	span := trace.SpanFromContext(ctx)
	if status == errorStatus {
		span.SetStatus(codes.Error, errorStatus)
	}
}
