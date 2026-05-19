package api

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/store"
	pkgapi "github.com/ArionMiles/expensor/backend/pkg/api"
)

// Storer is the subset of store.Store operations used by the API handlers.
// Using an interface allows handler unit tests to inject a mock without a real database.
type Storer interface {
	ListTransactions(ctx context.Context, f store.ListFilter) ([]store.Transaction, store.TransactionListResult, error)
	GetTransaction(ctx context.Context, id string) (*store.Transaction, error)
	UpdateDescription(ctx context.Context, id, description string) error
	AddLabel(ctx context.Context, transactionID, label string) error
	AddLabels(ctx context.Context, transactionID string, labels []string) error
	RemoveLabel(ctx context.Context, transactionID, label string) error
	SearchTransactions(ctx context.Context, query string, f store.ListFilter) ([]store.Transaction, store.TransactionListResult, error)
	GetStats(ctx context.Context, baseCurrency string) (*store.Stats, error)
	GetChartData(ctx context.Context) (*store.ChartData, error)
	GetDashboardData(ctx context.Context) (*store.DashboardData, error)
	GetSpendingHeatmap(ctx context.Context, from, to *time.Time) (*store.HeatmapData, error)
	GetAnnualSpend(ctx context.Context, year int) ([]store.DailyBucket, error)
	GetAppConfig(ctx context.Context, key string) (string, error)
	SetAppConfig(ctx context.Context, key, value string) error
	IsMessageProcessed(ctx context.Context, key string) (bool, error)
	MarkMessageProcessed(ctx context.Context, key string, at time.Time) error
	SetActiveReader(ctx context.Context, reader string) error
	GetActiveReader(ctx context.Context) (string, error)
	SetReaderSecret(ctx context.Context, reader string, secret []byte) error
	GetReaderSecret(ctx context.Context, reader string) ([]byte, bool, error)
	SetReaderToken(ctx context.Context, reader string, token []byte) error
	GetReaderToken(ctx context.Context, reader string) ([]byte, bool, error)
	DeleteReaderToken(ctx context.Context, reader string) error
	SetReaderConfig(ctx context.Context, reader string, config json.RawMessage) error
	GetReaderConfig(ctx context.Context, reader string) (json.RawMessage, bool, error)
	DeleteReaderRuntime(ctx context.Context, reader string) error
	GetFacets(ctx context.Context) (*store.Facets, error)
	// Labels
	ListLabels(ctx context.Context) ([]store.Label, error)
	CreateLabel(ctx context.Context, name, color string) error
	UpdateLabel(ctx context.Context, name, color string) error
	DeleteLabel(ctx context.Context, name string, removeFromTransactions bool) error
	ApplyLabelByMerchant(ctx context.Context, label, pattern string) (int64, error)
	RemoveLabelByMerchant(ctx context.Context, label, pattern string) (int64, error)
	GetLabelMappings(ctx context.Context) (map[string][]string, error)
	GetMonthlyBreakdownSpend(ctx context.Context, dimension string, months int) (*store.MonthlyBreakdownData, error)
	// Categories
	ListCategories(ctx context.Context) ([]store.Category, error)
	CreateCategory(ctx context.Context, name, description string) error
	DeleteCategory(ctx context.Context, name string, removeFromTransactions bool) error
	ApplyCategoryByMerchant(ctx context.Context, category, pattern string) (int64, error)
	RemoveCategoryByMerchant(ctx context.Context, category, pattern string) (int64, error)
	GetCategoryMappings(ctx context.Context) (map[string][]string, error)
	// Buckets
	ListBuckets(ctx context.Context) ([]store.Bucket, error)
	CreateBucket(ctx context.Context, name, description string) error
	DeleteBucket(ctx context.Context, name string, removeFromTransactions bool) error
	ApplyBucketByMerchant(ctx context.Context, bucket, pattern string) (int64, error)
	RemoveBucketByMerchant(ctx context.Context, bucket, pattern string) (int64, error)
	GetBucketMappings(ctx context.Context) (map[string][]string, error)
	// Extended transaction update
	UpdateTransaction(ctx context.Context, id string, u store.TransactionUpdate) error
	// Muted transactions
	MuteTransaction(ctx context.Context, id string, muted bool, reason string) error
	UpdateMuteReason(ctx context.Context, id, reason string) error
	MuteByMerchant(ctx context.Context, pattern, reason string) error
	UpdateMerchantReason(ctx context.Context, id, reason string) error
	ListMutedMerchants(ctx context.Context) ([]store.MutedMerchant, error)
	GetMutedMerchantsWithCount(ctx context.Context) ([]store.MutedMerchantWithCount, error)
	DeleteMutedMerchant(ctx context.Context, id string) error
	UnmuteByPattern(ctx context.Context, pattern string) error
	DeleteMutedMerchantAndUnmute(ctx context.Context, id string) error
	// Merchant-wide categorization
	CategorizeMerchant(ctx context.Context, merchant, category, bucket string) (int, error)
	// Rules
	ListRules(ctx context.Context) ([]store.RuleRow, error)
	GetRule(ctx context.Context, id string) (*store.RuleRow, error)
	CreateRule(ctx context.Context, r store.RuleRow) (*store.RuleRow, error)
	UpdateRule(ctx context.Context, id string, r store.RuleRow) (*store.RuleRow, error)
	DeleteRule(ctx context.Context, id string) error
	SeedPredefinedRules(ctx context.Context, rules []store.RuleRow) error
	ImportUserRules(ctx context.Context, rules []store.RuleRow) error
	// MCC / Community content
	SeedMCCCodes(ctx context.Context, entries []store.MCCEntry) error
	SeedMerchantCategories(ctx context.Context, entries []store.MerchantCategoryEntry) (int, error)
	LoadCategorySnapshot(ctx context.Context) (pkgapi.CategoryResolver, error)
	SeedMCCCategories(ctx context.Context, names []string) error
	GetSyncStatus(ctx context.Context) (store.SyncStatus, error)
	SetSyncStatus(ctx context.Context, status store.SyncStatus) error
	// Extraction diagnostics
	ListExtractionDiagnostics(ctx context.Context, filter store.DiagnosticFilter) ([]store.ExtractionDiagnosticRow, error)
	GetExtractionDiagnostic(ctx context.Context, id string) (*store.ExtractionDiagnosticRow, error)
	UpdateExtractionDiagnosticStatus(ctx context.Context, id, status string) (*store.ExtractionDiagnosticRow, error)
}

// compile-time check: *store.Store must satisfy Storer.
var _ Storer = (*store.Store)(nil)
