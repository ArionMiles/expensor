package api

import (
	"context"
	"time"

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
	GetSpendingHeatmap(ctx context.Context, from, to *time.Time) (*store.HeatmapData, error)
	GetAnnualSpend(ctx context.Context, year int) ([]store.DailyBucket, error)
	GetAppConfig(ctx context.Context, key string) (string, error)
	SetAppConfig(ctx context.Context, key, value string) error
	GetFacets(ctx context.Context) (*store.Facets, error)
	// Labels
	ListLabels(ctx context.Context) ([]store.Label, error)
	CreateLabel(ctx context.Context, name, color string) error
	UpdateLabel(ctx context.Context, name, color string) error
	DeleteLabel(ctx context.Context, name string) error
	ApplyLabelByMerchant(ctx context.Context, label, pattern string) (int64, error)
	// Categories
	ListCategories(ctx context.Context) ([]store.Category, error)
	CreateCategory(ctx context.Context, name, description string) error
	DeleteCategory(ctx context.Context, name string) error
	// Buckets
	ListBuckets(ctx context.Context) ([]store.Bucket, error)
	CreateBucket(ctx context.Context, name, description string) error
	DeleteBucket(ctx context.Context, name string) error
	// Extended transaction update
	UpdateTransaction(ctx context.Context, id string, u store.TransactionUpdate) error
}

// compile-time check: *store.Store must satisfy Storer.
var _ Storer = (*store.Store)(nil)
