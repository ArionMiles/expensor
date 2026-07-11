package instrumented

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/ArionMiles/expensor/backend/internal/observability"
	"github.com/ArionMiles/expensor/backend/internal/store"
)

// TransactionBatchWriter records telemetry around transaction ingestion writes.
type TransactionBatchWriter struct {
	next  store.TransactionBatchWriter
	scope *observability.Scope
}

func NewTransactionBatchWriter(next store.TransactionBatchWriter, scope *observability.Scope, logger *slog.Logger) *TransactionBatchWriter {
	return &TransactionBatchWriter{
		next:  next,
		scope: scope,
	}
}

func (w *TransactionBatchWriter) Write(ctx context.Context, batch store.IngestionBatch) error {
	ctx, span := w.scope.Start(ctx, "store.ingestion.write")
	defer span.End()

	start := time.Now()
	batchSize := len(batch.Transactions)
	span.SetAttributes(attribute.Int("store.ingestion.batch_size", batchSize))

	err := w.next.Write(ctx, batch)
	w.scope.RecordDuration(ctx, observability.DurationOperation{
		Namespace: "store",
		Name:      "ingestion.write",
		Duration:  time.Since(start),
		Err:       err,
		Attributes: []attribute.KeyValue{
			attribute.Int("batch_size", batchSize),
		},
	})
	return err
}
