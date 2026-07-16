package llm

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/ArionMiles/expensor/backend/internal/observability"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

// InstrumentedClient records provider-neutral telemetry around an LLM client.
type InstrumentedClient struct {
	next     Client
	provider string
	scope    *observability.Scope
	logger   *slog.Logger
	meter    metric.Meter
}

func NewInstrumentedClient(next Client, provider string, scope *observability.Scope, logger *slog.Logger) Client {
	if logger == nil {
		logger = slog.Default()
	}
	if scope == nil {
		scope = observability.NewScope(logger, "github.com/ArionMiles/expensor/backend/internal/llm")
	}
	return &InstrumentedClient{
		next:     next,
		provider: provider,
		scope:    scope,
		logger:   logger,
		meter:    otel.Meter("github.com/ArionMiles/expensor/backend/internal/llm"),
	}
}

func (c *InstrumentedClient) Complete(ctx context.Context, req Request) (Response, error) {
	start := time.Now()
	ctx, span := c.scope.Start(ctx, "llm.complete")
	defer span.End()

	attrs := c.requestAttrs(req)
	span.SetAttributes(attrs...)
	resp, err := c.next.Complete(ctx, req)
	duration := time.Since(start)

	recordAttrs := append([]attribute.KeyValue(nil), attrs...)
	if err != nil {
		if kind := errors.WhatKind(err); kind.Code != "" {
			recordAttrs = append(recordAttrs, attribute.String("error_kind", kind.Code))
		}
		c.logError(ctx, "llm provider call failed", "complete", err, attrs)
	}
	span.SetAttributes(recordAttrs...)
	c.scope.RecordDuration(ctx, observability.DurationOperation{
		Namespace:  "llm",
		Name:       "complete",
		Duration:   duration,
		Err:        err,
		Attributes: recordAttrs,
	})
	if err == nil {
		c.recordUsage(ctx, attrs, resp.Usage)
	}
	return resp, err
}

func (c *InstrumentedClient) HealthCheck(ctx context.Context) error {
	start := time.Now()
	ctx, span := c.scope.Start(ctx, "llm.healthcheck")
	defer span.End()

	attrs := []attribute.KeyValue{attribute.String("llm.provider", c.provider)}
	span.SetAttributes(attrs...)
	err := c.next.HealthCheck(ctx)
	recordAttrs := append([]attribute.KeyValue(nil), attrs...)
	if err != nil {
		if kind := errors.WhatKind(err); kind.Code != "" {
			recordAttrs = append(recordAttrs, attribute.String("error_kind", kind.Code))
		}
		c.logError(ctx, "llm provider healthcheck failed", "healthcheck", err, attrs)
	}
	span.SetAttributes(recordAttrs...)
	c.scope.RecordDuration(ctx, observability.DurationOperation{
		Namespace:  "llm",
		Name:       "healthcheck",
		Duration:   time.Since(start),
		Err:        err,
		Attributes: recordAttrs,
	})
	return err
}

func (c *InstrumentedClient) requestAttrs(req Request) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("llm.provider", c.provider),
		attribute.String("llm.workflow", safeAttr(req.Workflow, "unspecified")),
		attribute.String("llm.purpose", safeAttr(req.Purpose, "unspecified")),
	}
	if req.ResponseFormat.Type != "" {
		attrs = append(attrs, attribute.String("llm.response_format", string(req.ResponseFormat.Type)))
	}
	return attrs
}

func (c *InstrumentedClient) recordUsage(ctx context.Context, attrs []attribute.KeyValue, usage Usage) {
	counter, _ := c.meter.Int64Counter("llm.tokens")
	addTokens := func(kind string, count int) {
		if count <= 0 {
			return
		}
		tokenAttrs := append([]attribute.KeyValue(nil), attrs...)
		tokenAttrs = append(tokenAttrs, attribute.String("llm.token_type", kind))
		counter.Add(ctx, int64(count), metric.WithAttributes(tokenAttrs...))
	}
	addTokens("input", usage.InputTokens)
	addTokens("output", usage.OutputTokens)
	addTokens("total", usage.TotalTokens)
}

func (c *InstrumentedClient) logError(ctx context.Context, message, operation string, err error, attrs []attribute.KeyValue) {
	logAttrs := []slog.Attr{
		slog.String("namespace", "llm"),
		slog.String("operation", operation),
		slog.String("provider", c.provider),
	}
	for _, attr := range attrs {
		if attr.Key == "llm.workflow" {
			logAttrs = append(logAttrs, slog.String("workflow", attr.Value.AsString()))
		}
		if attr.Key == "llm.purpose" {
			logAttrs = append(logAttrs, slog.String("purpose", attr.Value.AsString()))
		}
	}
	logAttrs = append(logAttrs, errors.LogDetailAttrs(err)...)
	if spanContext := trace.SpanFromContext(ctx).SpanContext(); spanContext.IsValid() {
		logAttrs = append(logAttrs,
			slog.String("trace_id", spanContext.TraceID().String()),
			slog.String("span_id", spanContext.SpanID().String()),
		)
	}
	c.logger.LogAttrs(ctx, slog.LevelError, message, logAttrs...)
}

func safeAttr(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

var _ Client = (*InstrumentedClient)(nil)
