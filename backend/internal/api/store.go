package api

import (
	"context"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

// Storer is the subset of store.Store operations used by the API handlers.
// Using an interface allows handler unit tests to inject a mock without a real database.
type Storer interface {
	ListTransactions(ctx context.Context, f store.ListFilter) ([]store.Transaction, int, error)
	GetTransaction(ctx context.Context, id string) (*store.Transaction, error)
	UpdateDescription(ctx context.Context, id, description string) error
	AddLabel(ctx context.Context, transactionID, label string) error
	AddLabels(ctx context.Context, transactionID string, labels []string) error
	RemoveLabel(ctx context.Context, transactionID, label string) error
	SearchTransactions(ctx context.Context, query string, f store.ListFilter) ([]store.Transaction, int, error)
	GetStats(ctx context.Context, baseCurrency string) (*store.Stats, error)
	GetChartData(ctx context.Context) (*store.ChartData, error)
	GetAppConfig(ctx context.Context, key string) (string, error)
	SetAppConfig(ctx context.Context, key, value string) error
}

// compile-time check: *store.Store must satisfy Storer.
var _ Storer = (*store.Store)(nil)
