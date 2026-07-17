package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ArionMiles/expensor/backend/pkg/api"
)

// AuthStore persists authentication, account, and bootstrap state.
type AuthStore interface {
	BootstrapRequired(ctx context.Context) (bool, error)
	CreateBootstrapAdmin(ctx context.Context, input CreateBootstrapAdminInput) (*User, error)
	CreateUser(ctx context.Context, input CreateUserInput) (*User, error)
	ListUsers(ctx context.Context) ([]User, error)
	UpdateUser(ctx context.Context, id string, input UpdateUserInput) (*User, error)
	UpdateUserPassword(ctx context.Context, id string, input UpdateUserPasswordInput) error
	DeleteUser(ctx context.Context, id string) error
	FindUserByEmail(ctx context.Context, email string) (*User, error)
	FindUserByID(ctx context.Context, id string) (*User, error)
	CreateSession(ctx context.Context, input CreateSessionInput) (*Session, error)
	FindSessionByHash(ctx context.Context, tokenHash string) (*Session, error)
	RevokeSession(ctx context.Context, id string) error
	CreateAccessToken(ctx context.Context, input CreateAccessTokenInput) (*AccessToken, error)
	ListAccessTokens(ctx context.Context, userID string) ([]AccessToken, error)
	FindAccessTokenByHash(ctx context.Context, tokenHash string) (*AccessToken, error)
	RevokeAccessToken(ctx context.Context, id, userID string) error
	CreateAccountSetupToken(ctx context.Context, input CreateAccountSetupTokenInput) (*AccountSetupToken, error)
	FindAccountSetupTokenByHash(ctx context.Context, tokenHash string) (*AccountSetupToken, error)
	MarkAccountSetupTokenUsed(ctx context.Context, id string) error
	CompleteAccountSetup(ctx context.Context, input CompleteAccountSetupInput) (*User, error)
}

// AnalyticsStore reads dashboard and reporting projections.
type AnalyticsStore interface {
	GetStats(ctx context.Context, tenant Tenant, baseCurrency string) (*Stats, error)
	GetChartData(ctx context.Context, tenant Tenant) (*ChartData, error)
	GetDashboardData(ctx context.Context, tenant Tenant) (*DashboardData, error)
	GetSpendingHeatmap(ctx context.Context, tenant Tenant, from, to *time.Time) (*HeatmapData, error)
	GetAnnualSpend(ctx context.Context, tenant Tenant, year int) ([]DailyBucket, error)
	GetMonthlyBreakdownSpend(ctx context.Context, tenant Tenant, dimension string, months int) (*MonthlyBreakdownData, error)
}

// CommunityStore persists shared community taxonomy content and mappings.
type CommunityStore interface {
	SeedMCCCodes(ctx context.Context, entries []MCCEntry) error
	SeedMerchantCategories(ctx context.Context, entries []MerchantCategoryEntry) (int64, error)
	LoadCategorySnapshot(ctx context.Context) (api.CategoryResolver, error)
	SeedMCCCategories(ctx context.Context, names []string) error
	CategorizeMerchant(ctx context.Context, tenant Tenant, merchant, category, bucket string) (int64, error)
}

// DiagnosticStore persists extraction diagnostics.
type DiagnosticStore interface {
	ListExtractionDiagnostics(ctx context.Context, tenant Tenant, filter DiagnosticFilter) ([]ExtractionDiagnosticRow, error)
	GetExtractionDiagnostic(ctx context.Context, tenant Tenant, id string) (*ExtractionDiagnosticRow, error)
	UpdateExtractionDiagnosticStatus(ctx context.Context, tenant Tenant, id, status string) (*ExtractionDiagnosticRow, error)
	RecordExtractionDiagnostic(ctx context.Context, tenant Tenant, diagnostic api.ExtractionDiagnostic) error
}

// RuleStore persists system and user extraction rules.
type RuleStore interface {
	ListRules(ctx context.Context, tenant Tenant) ([]RuleRow, error)
	GetRule(ctx context.Context, tenant Tenant, id string) (*RuleRow, error)
	CreateRule(ctx context.Context, tenant Tenant, r RuleRow) (*RuleRow, error)
	UpdateRule(ctx context.Context, tenant Tenant, id string, r RuleRow) (*RuleRow, error)
	DeleteRule(ctx context.Context, tenant Tenant, id string) error
	SeedPredefinedRules(ctx context.Context, rules []RuleRow) error
	ImportUserRules(ctx context.Context, tenant Tenant, rules []RuleRow) error
}

// RuntimeStore persists runtime settings and provider state.
type RuntimeStore interface {
	GetAppConfig(ctx context.Context, tenant Tenant, key string) (string, error)
	SetAppConfig(ctx context.Context, tenant Tenant, key, value string) error
	IsMessageProcessed(ctx context.Context, tenant Tenant, key string) (bool, error)
	MarkMessageProcessed(ctx context.Context, tenant Tenant, key string, at time.Time) error
	SetReaderSecret(ctx context.Context, tenant Tenant, reader string, secret []byte) error
	GetReaderSecret(ctx context.Context, tenant Tenant, reader string) ([]byte, bool, error)
	SetReaderToken(ctx context.Context, tenant Tenant, reader string, token []byte) error
	GetReaderToken(ctx context.Context, tenant Tenant, reader string) ([]byte, bool, error)
	DeleteReaderToken(ctx context.Context, tenant Tenant, reader string) error
	SetReaderConfig(ctx context.Context, tenant Tenant, reader string, config json.RawMessage) error
	GetReaderConfig(ctx context.Context, tenant Tenant, reader string) (json.RawMessage, bool, error)
	DeleteReaderRuntime(ctx context.Context, tenant Tenant, reader string) error
	SetLLMProviderConfig(ctx context.Context, tenant Tenant, provider string, config json.RawMessage) error
	GetLLMProviderConfig(ctx context.Context, tenant Tenant, provider string) (json.RawMessage, bool, error)
	SetLLMProviderCredentials(ctx context.Context, tenant Tenant, provider string, credentials []byte) error
	GetLLMProviderCredentials(ctx context.Context, tenant Tenant, provider string) ([]byte, bool, error)
	DeleteLLMProviderRuntime(ctx context.Context, tenant Tenant, provider string) error
	SetActiveLLMProvider(ctx context.Context, tenant Tenant, provider string) error
	ClearActiveLLMProvider(ctx context.Context, tenant Tenant) error
	GetActiveLLMProviderRuntime(ctx context.Context, tenant Tenant) (LLMProviderRuntime, bool, error)
	GetCommunityURL(ctx context.Context) (string, error)
	SetCommunityURL(ctx context.Context, url string) error
	GetSyncStatus(ctx context.Context) (SyncStatus, error)
	SetSyncStatus(ctx context.Context, status SyncStatus) error
	GetCommunitySyncSettings(ctx context.Context) (CommunitySyncSettings, error)
	PatchCommunitySyncSettings(ctx context.Context, patch CommunitySyncSettingsPatch) (CommunitySyncSettings, error)
}

// ScanningStore persists scanner scheduler and tenant scanning state.
type ScanningStore interface {
	GetSchedulerConfig(ctx context.Context) (SchedulerConfig, error)
	PatchSchedulerConfig(ctx context.Context, patch SchedulerConfigPatch) (SchedulerConfig, error)
	EnsureScanningStateForTenant(ctx context.Context, tenant Tenant) error
	GetScanningState(ctx context.Context, tenant Tenant) (TenantScanningState, error)
	ListRunnableScanningStates(ctx context.Context) ([]TenantScanningState, error)
	ListScanningStates(ctx context.Context) ([]TenantScanningState, error)
	SetActiveScanningReader(ctx context.Context, tenant Tenant, reader string) error
	ClearActiveScanningReader(ctx context.Context, tenant Tenant) error
	SetScanningEnabled(ctx context.Context, tenant Tenant, enabled bool) error
	UpdateScanningState(ctx context.Context, tenant Tenant, update ScanningStateUpdate) error
}

// TaxonomyStore persists labels, categories, buckets, and their mappings.
type TaxonomyStore interface {
	ListLabels(ctx context.Context, tenant Tenant) ([]Label, error)
	CreateLabel(ctx context.Context, tenant Tenant, name, color string) error
	UpdateLabel(ctx context.Context, tenant Tenant, name, color string) error
	DeleteLabel(ctx context.Context, tenant Tenant, name string, removeFromTransactions bool) error
	ApplyLabelByMerchant(ctx context.Context, tenant Tenant, label, pattern string) (int64, error)
	RemoveLabelByMerchant(ctx context.Context, tenant Tenant, label, pattern string) (int64, error)
	GetLabelMappings(ctx context.Context, tenant Tenant) (map[string][]string, error)
	ListCategories(ctx context.Context, tenant Tenant) ([]Category, error)
	CreateCategory(ctx context.Context, tenant Tenant, name, description string) error
	DeleteCategory(ctx context.Context, tenant Tenant, name string, removeFromTransactions bool) error
	ApplyCategoryByMerchant(ctx context.Context, tenant Tenant, category, pattern string) (int64, error)
	RemoveCategoryByMerchant(ctx context.Context, tenant Tenant, category, pattern string) (int64, error)
	GetCategoryMappings(ctx context.Context, tenant Tenant) (map[string][]string, error)
	ListBuckets(ctx context.Context, tenant Tenant) ([]Bucket, error)
	CreateBucket(ctx context.Context, tenant Tenant, name, description string) error
	DeleteBucket(ctx context.Context, tenant Tenant, name string, removeFromTransactions bool) error
	ApplyBucketByMerchant(ctx context.Context, tenant Tenant, bucket, pattern string) (int64, error)
	RemoveBucketByMerchant(ctx context.Context, tenant Tenant, bucket, pattern string) (int64, error)
	GetBucketMappings(ctx context.Context, tenant Tenant) (map[string][]string, error)
}

// TransactionStore persists transactions and transaction annotations.
type TransactionStore interface {
	ListTransactions(ctx context.Context, tenant Tenant, f ListFilter) ([]Transaction, TransactionListResult, error)
	GetTransaction(ctx context.Context, tenant Tenant, id string) (*Transaction, error)
	UpdateDescription(ctx context.Context, tenant Tenant, id, description string) error
	AddLabel(ctx context.Context, tenant Tenant, transactionID, label string) error
	AddLabels(ctx context.Context, tenant Tenant, transactionID string, labels []string) error
	RemoveLabel(ctx context.Context, tenant Tenant, transactionID, label string) error
	SearchTransactions(ctx context.Context, tenant Tenant, query string, f ListFilter) ([]Transaction, TransactionListResult, error)
	GetFacets(ctx context.Context, tenant Tenant) (*Facets, error)
	UpdateTransaction(ctx context.Context, tenant Tenant, id string, u TransactionUpdate) error
	MuteTransaction(ctx context.Context, tenant Tenant, id string, muted bool, reason string) error
	UpdateMuteReason(ctx context.Context, tenant Tenant, id, reason string) error
	UpdateMerchantReason(ctx context.Context, tenant Tenant, id, reason string) error
	MuteByMerchant(ctx context.Context, tenant Tenant, pattern, reason string) error
	ListMutedMerchants(ctx context.Context, tenant Tenant) ([]MutedMerchant, error)
	GetMutedMerchantsWithCount(ctx context.Context, tenant Tenant) ([]MutedMerchantWithCount, error)
	DeleteMutedMerchant(ctx context.Context, tenant Tenant, id string) error
	UnmuteByPattern(ctx context.Context, tenant Tenant, pattern string) error
	DeleteMutedMerchantAndUnmute(ctx context.Context, tenant Tenant, id string) error
	GetMutedMerchantPatterns(ctx context.Context, tenant Tenant) ([]string, error)
}

// Seeder persists startup seed content and returns the resulting category resolver.
type Seeder interface {
	Seed(ctx context.Context, content SeedContent) (api.CategoryResolver, error)
}

// HealthChecker verifies backend reachability.
type HealthChecker interface {
	HealthCheck(ctx context.Context) error
}

// Backend is implemented by concrete store backends.
type Backend interface {
	AuthStore
	AnalyticsStore
	CommunityStore
	DiagnosticStore
	RuleStore
	RuntimeStore
	ScanningStore
	TaxonomyStore
	TransactionStore
	Seeder
	TransactionBatchWriter
	HealthChecker
	Close()
}
