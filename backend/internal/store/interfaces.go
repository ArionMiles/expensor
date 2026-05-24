package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ArionMiles/expensor/backend/pkg/api"
)

// TransactionStore is the transaction subset wrapped by store decorators.
type TransactionStore interface {
	ListTransactions(ctx context.Context, f ListFilter) ([]Transaction, TransactionListResult, error)
}

var _ TransactionStore = (*Store)(nil)

// FullStore is the store surface wrapped by the instrumentation facade.
type FullStore interface {
	ListTransactions(ctx context.Context, f ListFilter) ([]Transaction, TransactionListResult, error)
	GetTransaction(ctx context.Context, id string) (*Transaction, error)
	UpdateDescription(ctx context.Context, id, description string) error
	AddLabel(ctx context.Context, transactionID, label string) error
	AddLabels(ctx context.Context, transactionID string, labels []string) error
	RemoveLabel(ctx context.Context, transactionID, label string) error
	SearchTransactions(ctx context.Context, query string, f ListFilter) ([]Transaction, TransactionListResult, error)
	GetStats(ctx context.Context, baseCurrency string) (*Stats, error)
	GetChartData(ctx context.Context) (*ChartData, error)
	GetDashboardData(ctx context.Context) (*DashboardData, error)
	GetSpendingHeatmap(ctx context.Context, from, to *time.Time) (*HeatmapData, error)
	GetAnnualSpend(ctx context.Context, year int) ([]DailyBucket, error)
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
	GetFacets(ctx context.Context) (*Facets, error)
	ListLabels(ctx context.Context) ([]Label, error)
	CreateLabel(ctx context.Context, name, color string) error
	UpdateLabel(ctx context.Context, name, color string) error
	DeleteLabel(ctx context.Context, name string, removeFromTransactions bool) error
	ApplyLabelByMerchant(ctx context.Context, label, pattern string) (int64, error)
	RemoveLabelByMerchant(ctx context.Context, label, pattern string) (int64, error)
	GetLabelMappings(ctx context.Context) (map[string][]string, error)
	GetMonthlyBreakdownSpend(ctx context.Context, dimension string, months int) (*MonthlyBreakdownData, error)
	ListCategories(ctx context.Context) ([]Category, error)
	CreateCategory(ctx context.Context, name, description string) error
	DeleteCategory(ctx context.Context, name string, removeFromTransactions bool) error
	ApplyCategoryByMerchant(ctx context.Context, category, pattern string) (int64, error)
	RemoveCategoryByMerchant(ctx context.Context, category, pattern string) (int64, error)
	GetCategoryMappings(ctx context.Context) (map[string][]string, error)
	ListBuckets(ctx context.Context) ([]Bucket, error)
	CreateBucket(ctx context.Context, name, description string) error
	DeleteBucket(ctx context.Context, name string, removeFromTransactions bool) error
	ApplyBucketByMerchant(ctx context.Context, bucket, pattern string) (int64, error)
	RemoveBucketByMerchant(ctx context.Context, bucket, pattern string) (int64, error)
	GetBucketMappings(ctx context.Context) (map[string][]string, error)
	UpdateTransaction(ctx context.Context, id string, u TransactionUpdate) error
	MuteTransaction(ctx context.Context, id string, muted bool, reason string) error
	UpdateMuteReason(ctx context.Context, id, reason string) error
	UpdateMerchantReason(ctx context.Context, id, reason string) error
	MuteByMerchant(ctx context.Context, pattern, reason string) error
	ListMutedMerchants(ctx context.Context) ([]MutedMerchant, error)
	GetMutedMerchantsWithCount(ctx context.Context) ([]MutedMerchantWithCount, error)
	DeleteMutedMerchant(ctx context.Context, id string) error
	UnmuteByPattern(ctx context.Context, pattern string) error
	DeleteMutedMerchantAndUnmute(ctx context.Context, id string) error
	GetMutedMerchantPatterns(ctx context.Context) ([]string, error)
	CategorizeMerchant(ctx context.Context, merchant, category, bucket string) (int, error)
	ListRules(ctx context.Context) ([]RuleRow, error)
	GetRule(ctx context.Context, id string) (*RuleRow, error)
	CreateRule(ctx context.Context, r RuleRow) (*RuleRow, error)
	UpdateRule(ctx context.Context, id string, r RuleRow) (*RuleRow, error)
	DeleteRule(ctx context.Context, id string) error
	SeedPredefinedRules(ctx context.Context, rules []RuleRow) error
	ImportUserRules(ctx context.Context, rules []RuleRow) error
	SeedMCCCodes(ctx context.Context, entries []MCCEntry) error
	SeedMerchantCategories(ctx context.Context, entries []MerchantCategoryEntry) (int, error)
	LoadCategorySnapshot(ctx context.Context) (api.CategoryResolver, error)
	SeedMCCCategories(ctx context.Context, names []string) error
	GetSyncStatus(ctx context.Context) (SyncStatus, error)
	SetSyncStatus(ctx context.Context, status SyncStatus) error
	ListExtractionDiagnostics(ctx context.Context, filter DiagnosticFilter) ([]ExtractionDiagnosticRow, error)
	GetExtractionDiagnostic(ctx context.Context, id string) (*ExtractionDiagnosticRow, error)
	UpdateExtractionDiagnosticStatus(ctx context.Context, id, status string) (*ExtractionDiagnosticRow, error)
	RecordExtractionDiagnostic(ctx context.Context, diagnostic api.ExtractionDiagnostic) error
}

var _ FullStore = (*Store)(nil)
