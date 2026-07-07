package assistant

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/ArionMiles/expensor/backend/internal/llm"
	"github.com/ArionMiles/expensor/backend/internal/observability"
	"github.com/ArionMiles/expensor/backend/internal/store"
)

const ruleDraftOutcomeError = "error"

// RuleDrafter is implemented by services that generate rule drafts from email samples.
type RuleDrafter interface {
	DraftRule(ctx context.Context, tenant store.Tenant, input RuleDraftInput) (RuleDraftResult, error)
}

// InstrumentedRuleDrafter records workflow telemetry around rule drafting.
type InstrumentedRuleDrafter struct {
	next   RuleDrafter
	scope  *observability.Scope
	logger *slog.Logger
}

func NewInstrumentedRuleDrafter(next RuleDrafter, scope *observability.Scope, logger *slog.Logger) *InstrumentedRuleDrafter {
	if logger == nil {
		logger = slog.Default()
	}
	if scope == nil {
		scope = observability.NewScope(logger, "github.com/ArionMiles/expensor/backend/internal/assistant")
	}
	return &InstrumentedRuleDrafter{next: next, scope: scope, logger: logger}
}

func (d *InstrumentedRuleDrafter) DraftRule(ctx context.Context, tenant store.Tenant, input RuleDraftInput) (RuleDraftResult, error) {
	start := time.Now()
	ctx, span := d.scope.Start(ctx, "assistant.rule_draft")
	defer span.End()

	sampleCount := countDraftSamples(input)
	attrs := []attribute.KeyValue{
		attribute.String("assistant.workflow", ruleDraftWorkflow),
		attribute.String("assistant.purpose", ruleDraftPurpose),
		attribute.Int("assistant.sample_count", sampleCount),
	}
	span.SetAttributes(attrs...)

	result, err := d.next.DraftRule(ctx, tenant, input)
	outcome := "ok"
	issueCount := len(result.ValidationIssues)
	if err != nil {
		outcome = ruleDraftOutcomeError
		attrs = append(attrs, attribute.String("error_class", ruleDraftErrorClass(err)))
		d.logError(ctx, err, sampleCount)
	} else if issueCount > 0 {
		outcome = "needs_review"
		d.logger.LogAttrs(ctx, slog.LevelInfo, "rule draft needs review",
			slog.String("namespace", "assistant"),
			slog.String("operation", "rule_draft"),
			slog.Int("sample_count", sampleCount),
			slog.Int("validation_issue_count", issueCount),
		)
	}
	attrs = append(attrs,
		attribute.String("assistant.outcome", outcome),
		attribute.Int("assistant.validation_issue_count", issueCount),
	)
	span.SetAttributes(attrs...)

	d.scope.RecordDuration(ctx, observability.DurationOperation{
		Namespace:  "assistant",
		Name:       "rule_draft",
		Duration:   time.Since(start),
		Err:        err,
		Attributes: attrs,
	})
	return result, err
}

func (d *InstrumentedRuleDrafter) logError(ctx context.Context, err error, sampleCount int) {
	logAttrs := []slog.Attr{
		slog.String("namespace", "assistant"),
		slog.String("operation", "rule_draft"),
		slog.String("error_class", ruleDraftErrorClass(err)),
		slog.Int("sample_count", sampleCount),
	}
	if spanContext := trace.SpanFromContext(ctx).SpanContext(); spanContext.IsValid() {
		logAttrs = append(logAttrs,
			slog.String("trace_id", spanContext.TraceID().String()),
			slog.String("span_id", spanContext.SpanID().String()),
		)
	}
	d.logger.LogAttrs(ctx, slog.LevelError, "rule draft failed", logAttrs...)
}

func ruleDraftErrorClass(err error) string {
	switch {
	case errors.Is(err, llm.ErrNoProviderConfigured):
		return "no_provider_configured"
	case errors.Is(err, llm.ErrCapabilityUnsupported):
		return "capability_unsupported"
	case errors.Is(err, ErrRuleDraftInvalidInput):
		return "invalid_input"
	case errors.Is(err, ErrRuleDraftInvalidOutput):
		return "invalid_output"
	case errors.Is(err, ErrRuleDraftPromptMissing):
		return "prompt_missing"
	case errors.Is(err, context.Canceled):
		return "context_canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline_exceeded"
	default:
		return "error"
	}
}

func countDraftSamples(input RuleDraftInput) int {
	count := 0
	for _, sample := range input.Samples {
		if sample.Body != "" {
			count++
		}
	}
	return count
}

var (
	_ RuleDrafter = (*RuleDraftService)(nil)
	_ RuleDrafter = (*InstrumentedRuleDrafter)(nil)
)
