package observability

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Operation describes one low-cardinality operation measurement.
type Operation struct {
	Namespace string
	Name      string
	Err       error
}

// RecordOperation records a span result, metrics, and a trace-aware log entry.
func (s *Scope) RecordOperation(ctx context.Context, op Operation) {
	status := "ok"
	if op.Err != nil {
		status = "error"
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
		span.RecordError(op.Err)
		span.SetStatus(codes.Error, op.Err.Error())
		logAttrs = append(logAttrs, slog.Any("error", op.Err))
		s.logger.LogAttrs(ctx, slog.LevelError, "operation failed", logAttrs...)
		return
	}

	s.logger.LogAttrs(ctx, slog.LevelDebug, "operation completed", logAttrs...)
}
