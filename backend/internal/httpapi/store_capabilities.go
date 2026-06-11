package httpapi

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

type settingsStore interface {
	GetAppConfig(ctx context.Context, key string) (string, error)
	SetAppConfig(ctx context.Context, key, value string) error
}

type analyticsStore interface {
	GetStats(ctx context.Context, baseCurrency string) (*store.Stats, error)
	GetChartData(ctx context.Context) (*store.ChartData, error)
	GetDashboardData(ctx context.Context) (*store.DashboardData, error)
	GetSpendingHeatmap(ctx context.Context, from, to *time.Time) (*store.HeatmapData, error)
	GetAnnualSpend(ctx context.Context, year int) ([]store.DailyBucket, error)
	GetMonthlyBreakdownSpend(ctx context.Context, dimension string, months int) (*store.MonthlyBreakdownData, error)
}

type transactionStore interface {
	ListTransactions(ctx context.Context, f store.ListFilter) ([]store.Transaction, store.TransactionListResult, error)
	SearchTransactions(ctx context.Context, query string, f store.ListFilter) ([]store.Transaction, store.TransactionListResult, error)
	GetTransaction(ctx context.Context, id string) (*store.Transaction, error)
	UpdateTransaction(ctx context.Context, id string, u store.TransactionUpdate) error
	AddLabels(ctx context.Context, transactionID string, labels []string) error
	RemoveLabel(ctx context.Context, transactionID, label string) error
	GetFacets(ctx context.Context) (*store.Facets, error)
}

type muteStore interface {
	MuteTransaction(ctx context.Context, id string, muted bool, reason string) error
	UpdateMuteReason(ctx context.Context, id, reason string) error
	MuteByMerchant(ctx context.Context, pattern, reason string) error
	UpdateMerchantReason(ctx context.Context, id, reason string) error
	GetMutedMerchantsWithCount(ctx context.Context) ([]store.MutedMerchantWithCount, error)
	DeleteMutedMerchant(ctx context.Context, id string) error
	DeleteMutedMerchantAndUnmute(ctx context.Context, id string) error
	CategorizeMerchant(ctx context.Context, merchant, category, bucket string) (int, error)
}

type taxonomyStore interface {
	ListLabels(ctx context.Context) ([]store.Label, error)
	CreateLabel(ctx context.Context, name, color string) error
	UpdateLabel(ctx context.Context, name, color string) error
	DeleteLabel(ctx context.Context, name string, removeFromTransactions bool) error
	ApplyLabelByMerchant(ctx context.Context, label, pattern string) (int64, error)
	RemoveLabelByMerchant(ctx context.Context, label, pattern string) (int64, error)
	GetLabelMappings(ctx context.Context) (map[string][]string, error)
	ListCategories(ctx context.Context) ([]store.Category, error)
	CreateCategory(ctx context.Context, name, description string) error
	DeleteCategory(ctx context.Context, name string, removeFromTransactions bool) error
	ApplyCategoryByMerchant(ctx context.Context, category, pattern string) (int64, error)
	RemoveCategoryByMerchant(ctx context.Context, category, pattern string) (int64, error)
	GetCategoryMappings(ctx context.Context) (map[string][]string, error)
	ListBuckets(ctx context.Context) ([]store.Bucket, error)
	CreateBucket(ctx context.Context, name, description string) error
	DeleteBucket(ctx context.Context, name string, removeFromTransactions bool) error
	ApplyBucketByMerchant(ctx context.Context, bucket, pattern string) (int64, error)
	RemoveBucketByMerchant(ctx context.Context, bucket, pattern string) (int64, error)
	GetBucketMappings(ctx context.Context) (map[string][]string, error)
}

type readerRuntimeStore interface {
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
}

type ruleStore interface {
	ListRules(ctx context.Context) ([]store.RuleRow, error)
	GetRule(ctx context.Context, id string) (*store.RuleRow, error)
	CreateRule(ctx context.Context, r store.RuleRow) (*store.RuleRow, error)
	UpdateRule(ctx context.Context, id string, r store.RuleRow) (*store.RuleRow, error)
	DeleteRule(ctx context.Context, id string) error
	ImportUserRules(ctx context.Context, rules []store.RuleRow) error
}

type syncStore interface {
	GetSyncStatus(ctx context.Context) (store.SyncStatus, error)
}

type diagnosticStore interface {
	ListExtractionDiagnostics(ctx context.Context, filter store.DiagnosticFilter) ([]store.ExtractionDiagnosticRow, error)
	GetExtractionDiagnostic(ctx context.Context, id string) (*store.ExtractionDiagnosticRow, error)
	UpdateExtractionDiagnosticStatus(ctx context.Context, id, status string) (*store.ExtractionDiagnosticRow, error)
}
