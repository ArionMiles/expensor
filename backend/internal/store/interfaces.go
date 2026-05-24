package store

import "context"

// TransactionStore is the transaction subset wrapped by store decorators.
type TransactionStore interface {
	ListTransactions(ctx context.Context, f ListFilter) ([]Transaction, TransactionListResult, error)
}

var _ TransactionStore = (*Store)(nil)
