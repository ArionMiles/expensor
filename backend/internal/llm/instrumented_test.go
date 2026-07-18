package llm

import (
	"bytes"
	"context"
	stderrors "errors"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/ArionMiles/expensor/backend/internal/observability"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

type instrumentedClientStub struct {
	response Response
	err      error
}

func (c instrumentedClientStub) Complete(context.Context, Request) (Response, error) {
	return c.response, c.err
}

func (c instrumentedClientStub) HealthCheck(context.Context) error {
	return c.err
}

func TestInstrumentedClientRecordsCompleteSpanAndSanitizedErrorLog(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(provider)
	defer otel.SetTracerProvider(noop.NewTracerProvider())

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scope := observability.NewScope(logger, "test/llm")
	client := NewInstrumentedClient(instrumentedClientStub{
		err: errors.E(errors.ResourceExhausted, errors.User("OpenAI API quota is unavailable."), stderrors.New("quota failed for sensitive account detail")),
	}, "openai", scope, logger)

	_, err := client.Complete(context.Background(), Request{
		Workflow:       "rule_drafting",
		Purpose:        "draft_rule",
		ResponseFormat: ResponseFormat{Type: ResponseFormatJSONSchema},
		Messages:       []Message{{Role: RoleUser, Content: "body must not be logged"}},
	})
	if err == nil {
		t.Fatal("Complete() error = nil, want provider error")
	}

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(spans))
	}
	if spans[0].Name() != "llm.complete" {
		t.Fatalf("span name = %q, want llm.complete", spans[0].Name())
	}
	attrs := spanAttrs(spans[0].Attributes())
	if attrs["llm.provider"] != "openai" || attrs["llm.workflow"] != "rule_drafting" || attrs["llm.purpose"] != "draft_rule" {
		t.Fatalf("span attrs = %#v, want provider/workflow/purpose", attrs)
	}
	if attrs["error_kind"] != errors.ResourceExhausted.Code {
		t.Fatalf("span error attrs = %#v, want structured error kind", attrs)
	}

	gotLogs := logs.String()
	if !strings.Contains(gotLogs, "llm provider call failed") || !strings.Contains(gotLogs, "error_kind=resource_exhausted") {
		t.Fatalf("logs = %q, want structured provider failure", gotLogs)
	}
	if strings.Contains(gotLogs, "sensitive account detail") || strings.Contains(gotLogs, "body must not be logged") {
		t.Fatalf("logs = %q, want no raw provider message or prompt content", gotLogs)
	}
}

func TestInstrumentedClientRecordsHealthcheckFailure(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(provider)
	defer otel.SetTracerProvider(noop.NewTracerProvider())

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scope := observability.NewScope(logger, "test/llm")
	client := NewInstrumentedClient(instrumentedClientStub{err: context.DeadlineExceeded}, "openai", scope, logger)

	err := client.HealthCheck(context.Background())
	if !stderrors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("HealthCheck() error = %v, want deadline exceeded", err)
	}

	spans := recorder.Ended()
	if len(spans) != 1 || spans[0].Name() != "llm.healthcheck" {
		t.Fatalf("spans = %#v, want one llm.healthcheck span", spans)
	}
	attrs := spanAttrs(spans[0].Attributes())
	if attrs["llm.provider"] != "openai" || attrs["error_kind"] != errors.DeadlineExceeded.Code {
		t.Fatalf("span attrs = %#v, want provider and structured error kind", attrs)
	}
	if !strings.Contains(logs.String(), "llm provider healthcheck failed") {
		t.Fatalf("logs = %q, want healthcheck failure log", logs.String())
	}
}

func spanAttrs(attrs []attribute.KeyValue) map[string]string {
	out := make(map[string]string, len(attrs))
	for _, attr := range attrs {
		out[string(attr.Key)] = attr.Value.Emit()
	}
	return out
}
