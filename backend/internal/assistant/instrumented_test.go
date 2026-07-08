package assistant

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
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

type instrumentedRuleDrafterStub struct {
	result RuleDraftResult
	err    error
}

func (s instrumentedRuleDrafterStub) DraftRule(context.Context, store.Tenant, RuleDraftInput) (RuleDraftResult, error) {
	return s.result, s.err
}

func TestInstrumentedRuleDrafterRecordsValidationIssueOutcome(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(provider)
	defer otel.SetTracerProvider(noop.NewTracerProvider())

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scope := observability.NewScope(logger, "test/assistant")
	drafter := NewInstrumentedRuleDrafter(instrumentedRuleDrafterStub{
		result: RuleDraftResult{
			ValidationIssues: []RuleDraftSampleIssue{{
				SampleIndex: 0,
				Field:       "amount",
				Expected:    "100",
				Actual:      "99",
				Message:     "Amount matched sensitive value.",
			}},
		},
	}, scope, logger)

	_, err := drafter.DraftRule(context.Background(), store.Tenant{ID: "tenant-a"}, RuleDraftInput{
		Samples: []Sample{{Body: "email body with merchant detail"}},
	})
	if err != nil {
		t.Fatalf("DraftRule() error = %v", err)
	}

	spans := recorder.Ended()
	if len(spans) != 1 || spans[0].Name() != "assistant.rule_draft" {
		t.Fatalf("spans = %#v, want one assistant.rule_draft span", spans)
	}
	attrs := assistantSpanAttrs(spans[0].Attributes())
	if attrs["assistant.workflow"] != "rule_drafting" ||
		attrs["assistant.outcome"] != "needs_review" ||
		attrs["assistant.sample_count"] != "1" ||
		attrs["assistant.validation_issue_count"] != "1" {
		t.Fatalf("span attrs = %#v, want workflow outcome and counts", attrs)
	}

	gotLogs := logs.String()
	if !strings.Contains(gotLogs, "rule draft needs review") || !strings.Contains(gotLogs, "validation_issue_count=1") {
		t.Fatalf("logs = %q, want validation issue summary", gotLogs)
	}
	if strings.Contains(gotLogs, "Amount matched sensitive value") || strings.Contains(gotLogs, "email body with merchant detail") {
		t.Fatalf("logs = %q, want no raw validation messages or email body", gotLogs)
	}
}

func TestInstrumentedRuleDrafterRecordsSanitizedErrors(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(provider)
	defer otel.SetTracerProvider(noop.NewTracerProvider())

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scope := observability.NewScope(logger, "test/assistant")
	drafter := NewInstrumentedRuleDrafter(instrumentedRuleDrafterStub{
		err: errors.E(
			"assistant.RuleDraftService.requestDraft",
			KindRuleDraftInvalidOutput,
			stderrors.New("raw provider output contained sensitive data"),
		),
	}, scope, logger)

	_, err := drafter.DraftRule(context.Background(), store.Tenant{ID: "tenant-a"}, RuleDraftInput{
		Samples: []Sample{{Body: "email body must not be logged"}},
	})
	if errors.WhatKind(err) != KindRuleDraftInvalidOutput {
		t.Fatalf("DraftRule() error = %v, want KindRuleDraftInvalidOutput", err)
	}

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(spans))
	}
	attrs := assistantSpanAttrs(spans[0].Attributes())
	if attrs["assistant.outcome"] != "error" || attrs["error_class"] != "invalid_output" {
		t.Fatalf("span attrs = %#v, want sanitized error outcome", attrs)
	}

	gotLogs := logs.String()
	if !strings.Contains(gotLogs, "rule draft failed") || !strings.Contains(gotLogs, "error_class=invalid_output") {
		t.Fatalf("logs = %q, want sanitized rule draft error", gotLogs)
	}
	if strings.Contains(gotLogs, "raw provider output") || strings.Contains(gotLogs, "email body must not be logged") {
		t.Fatalf("logs = %q, want no raw error or email body", gotLogs)
	}
}

func assistantSpanAttrs(attrs []attribute.KeyValue) map[string]string {
	out := make(map[string]string, len(attrs))
	for _, attr := range attrs {
		out[string(attr.Key)] = attr.Value.Emit()
	}
	return out
}
