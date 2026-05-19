package store

import (
	"context"
	"log/slog"
	"time"
)

type QueryInstrumentation struct {
	logger *slog.Logger
	now    func() time.Time
}

func NewQueryInstrumentation(logger *slog.Logger) *QueryInstrumentation {
	if logger == nil {
		logger = slog.Default()
	}
	return &QueryInstrumentation{logger: logger, now: time.Now}
}

func (q *QueryInstrumentation) Observe(
	ctx context.Context,
	operation string,
	fn func(context.Context) error,
) error {
	start := q.now()
	err := fn(ctx)
	q.logger.Debug(
		"store query",
		"operation", operation,
		"duration_ms", time.Since(start).Milliseconds(),
		"error", err,
	)
	return err
}
