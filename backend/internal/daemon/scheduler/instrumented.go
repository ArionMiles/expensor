package scheduler

import (
	"context"
	"log/slog"

	"github.com/ArionMiles/expensor/backend/internal/observability"
	"github.com/ArionMiles/expensor/backend/internal/store"
)

// InstrumentedRunner records scheduler run telemetry around a runner implementation.
type InstrumentedRunner struct {
	next  Runner
	scope *observability.Scope
}

func NewInstrumentedRunner(next Runner, scope *observability.Scope) *InstrumentedRunner {
	if scope == nil {
		scope = observability.NewScope(slog.Default(), "github.com/ArionMiles/expensor/backend/internal/daemon/scheduler")
	}
	return &InstrumentedRunner{next: next, scope: scope}
}

func (r *InstrumentedRunner) Run(ctx context.Context, tenant store.Tenant, reader string) error {
	ctx, span := r.scope.Start(ctx, "scheduler.run")
	defer span.End()

	err := r.next.Run(ctx, tenant, reader)
	r.scope.RecordOperation(ctx, observability.Operation{
		Namespace: "scheduler",
		Name:      "run",
		Err:       err,
	})
	return err
}
