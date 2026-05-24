package store

import (
	"context"
	"log/slog"
	"time"

	"github.com/ArionMiles/expensor/backend/pkg/observability"
)

// InstrumentedTransactionStore records telemetry around transaction store calls.
type InstrumentedTransactionStore struct {
	next  TransactionStore
	scope *observability.Scope
	now   func() time.Time
}

func NewInstrumentedTransactionStore(next TransactionStore, scope *observability.Scope, logger *slog.Logger) *InstrumentedTransactionStore {
	if logger == nil {
		logger = slog.Default()
	}
	if scope == nil {
		scope = observability.NewScope(logger, "store")
	}
	return &InstrumentedTransactionStore{
		next:  next,
		scope: scope,
		now:   time.Now,
	}
}

func (s *InstrumentedTransactionStore) SetNowForTest(now func() time.Time) {
	if now != nil {
		s.now = now
	}
}

func (s *InstrumentedTransactionStore) ListTransactions(ctx context.Context, f ListFilter) ([]Transaction, TransactionListResult, error) {
	ctx, span := s.scope.Start(ctx, "store.transactions.list")
	defer span.End()

	start := s.now()
	rows, result, err := s.next.ListTransactions(ctx, f)
	s.scope.RecordOperation(ctx, observability.Operation{
		Namespace: "store",
		Name:      "transactions.list",
		Duration:  time.Since(start),
		Err:       err,
	})
	return rows, result, err
}
