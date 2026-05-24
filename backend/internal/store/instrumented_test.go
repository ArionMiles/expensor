package store_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/observability"
)

type fakeTransactionStore struct {
	called bool
	err    error
}

func (f *fakeTransactionStore) ListTransactions(ctx context.Context, filter store.ListFilter) ([]store.Transaction, store.TransactionListResult, error) {
	f.called = true
	if f.err != nil {
		return nil, store.TransactionListResult{}, f.err
	}
	return []store.Transaction{{ID: "tx-1", MessageID: "msg-1"}}, store.TransactionListResult{Total: 1, TotalAmount: 42}, nil
}

func TestInstrumentedTransactionStoreDelegatesSuccess(t *testing.T) {
	next := &fakeTransactionStore{}
	scope := observability.NewScope(slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	instrumented := store.NewInstrumentedTransactionStore(next, scope, slog.Default())

	rows, result, err := instrumented.ListTransactions(context.Background(), store.ListFilter{})
	if err != nil {
		t.Fatalf("ListTransactions() error = %v", err)
	}
	if !next.called {
		t.Fatal("next ListTransactions was not called")
	}
	if len(rows) != 1 || rows[0].ID != "tx-1" || result.Total != 1 || result.TotalAmount != 42 {
		t.Fatalf("unexpected response rows=%#v result=%#v", rows, result)
	}
}

func TestInstrumentedTransactionStoreDelegatesError(t *testing.T) {
	wantErr := errors.New("db down")
	next := &fakeTransactionStore{err: wantErr}
	scope := observability.NewScope(slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	instrumented := store.NewInstrumentedTransactionStore(next, scope, slog.Default())

	_, _, err := instrumented.ListTransactions(context.Background(), store.ListFilter{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("ListTransactions() error = %v, want %v", err, wantErr)
	}
	if !next.called {
		t.Fatal("next ListTransactions was not called")
	}
}
