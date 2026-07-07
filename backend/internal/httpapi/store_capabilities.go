package httpapi

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

type authStore interface {
	BootstrapRequired(ctx context.Context) (bool, error)
	CreateBootstrapAdmin(ctx context.Context, input store.CreateBootstrapAdminInput) (*store.User, error)
	CreateUser(ctx context.Context, input store.CreateUserInput) (*store.User, error)
	ListUsers(ctx context.Context) ([]store.User, error)
	UpdateUser(ctx context.Context, id string, input store.UpdateUserInput) (*store.User, error)
	UpdateUserPassword(ctx context.Context, id string, input store.UpdateUserPasswordInput) error
	DeleteUser(ctx context.Context, id string) error
	FindUserByEmail(ctx context.Context, email string) (*store.User, error)
	FindUserByID(ctx context.Context, id string) (*store.User, error)
	CreateSession(ctx context.Context, input store.CreateSessionInput) (*store.Session, error)
	FindSessionByHash(ctx context.Context, tokenHash string) (*store.Session, error)
	RevokeSession(ctx context.Context, id string) error
	CreateAccessToken(ctx context.Context, input store.CreateAccessTokenInput) (*store.AccessToken, error)
	ListAccessTokens(ctx context.Context, userID string) ([]store.AccessToken, error)
	FindAccessTokenByHash(ctx context.Context, tokenHash string) (*store.AccessToken, error)
	RevokeAccessToken(ctx context.Context, id, userID string) error
	CreateAccountSetupToken(ctx context.Context, input store.CreateAccountSetupTokenInput) (*store.AccountSetupToken, error)
	FindAccountSetupTokenByHash(ctx context.Context, tokenHash string) (*store.AccountSetupToken, error)
	MarkAccountSetupTokenUsed(ctx context.Context, id string) error
	CompleteAccountSetup(ctx context.Context, input store.CompleteAccountSetupInput) (*store.User, error)
}

type settingsStore interface {
	GetAppConfig(ctx context.Context, tenant store.Tenant, key string) (string, error)
	SetAppConfig(ctx context.Context, tenant store.Tenant, key, value string) error
}

type scanningStore interface {
	GetSchedulerConfig(ctx context.Context) (store.SchedulerConfig, error)
	PatchSchedulerConfig(ctx context.Context, patch store.SchedulerConfigPatch) (store.SchedulerConfig, error)
	EnsureScanningStateForTenant(ctx context.Context, tenant store.Tenant) error
	GetScanningState(ctx context.Context, tenant store.Tenant) (store.TenantScanningState, error)
	ListScanningStates(ctx context.Context) ([]store.TenantScanningState, error)
	SetActiveScanningReader(ctx context.Context, tenant store.Tenant, reader string) error
	ClearActiveScanningReader(ctx context.Context, tenant store.Tenant) error
	SetScanningEnabled(ctx context.Context, tenant store.Tenant, enabled bool) error
	UpdateScanningState(ctx context.Context, tenant store.Tenant, update store.ScanningStateUpdate) error
}

type analyticsStore interface {
	GetStats(ctx context.Context, tenant store.Tenant, baseCurrency string) (*store.Stats, error)
	GetChartData(ctx context.Context, tenant store.Tenant) (*store.ChartData, error)
	GetDashboardData(ctx context.Context, tenant store.Tenant) (*store.DashboardData, error)
	GetSpendingHeatmap(ctx context.Context, tenant store.Tenant, from, to *time.Time) (*store.HeatmapData, error)
	GetAnnualSpend(ctx context.Context, tenant store.Tenant, year int) ([]store.DailyBucket, error)
	GetMonthlyBreakdownSpend(ctx context.Context, tenant store.Tenant, dimension string, months int) (*store.MonthlyBreakdownData, error)
}

type transactionStore interface {
	ListTransactions(ctx context.Context, tenant store.Tenant, f store.ListFilter) ([]store.Transaction, store.TransactionListResult, error)
	SearchTransactions(ctx context.Context, tenant store.Tenant, query string, f store.ListFilter) ([]store.Transaction, store.TransactionListResult, error)
	GetTransaction(ctx context.Context, tenant store.Tenant, id string) (*store.Transaction, error)
	UpdateTransaction(ctx context.Context, tenant store.Tenant, id string, u store.TransactionUpdate) error
	AddLabels(ctx context.Context, tenant store.Tenant, transactionID string, labels []string) error
	RemoveLabel(ctx context.Context, tenant store.Tenant, transactionID, label string) error
	GetFacets(ctx context.Context, tenant store.Tenant) (*store.Facets, error)
}

type muteStore interface {
	MuteTransaction(ctx context.Context, tenant store.Tenant, id string, muted bool, reason string) error
	UpdateMuteReason(ctx context.Context, tenant store.Tenant, id, reason string) error
	MuteByMerchant(ctx context.Context, tenant store.Tenant, pattern, reason string) error
	UpdateMerchantReason(ctx context.Context, tenant store.Tenant, id, reason string) error
	GetMutedMerchantsWithCount(ctx context.Context, tenant store.Tenant) ([]store.MutedMerchantWithCount, error)
	DeleteMutedMerchant(ctx context.Context, tenant store.Tenant, id string) error
	DeleteMutedMerchantAndUnmute(ctx context.Context, tenant store.Tenant, id string) error
	CategorizeMerchant(ctx context.Context, tenant store.Tenant, merchant, category, bucket string) (int64, error)
}

type taxonomyStore interface {
	ListLabels(ctx context.Context, tenant store.Tenant) ([]store.Label, error)
	CreateLabel(ctx context.Context, tenant store.Tenant, name, color string) error
	UpdateLabel(ctx context.Context, tenant store.Tenant, name, color string) error
	DeleteLabel(ctx context.Context, tenant store.Tenant, name string, removeFromTransactions bool) error
	ApplyLabelByMerchant(ctx context.Context, tenant store.Tenant, label, pattern string) (int64, error)
	RemoveLabelByMerchant(ctx context.Context, tenant store.Tenant, label, pattern string) (int64, error)
	GetLabelMappings(ctx context.Context, tenant store.Tenant) (map[string][]string, error)
	ListCategories(ctx context.Context, tenant store.Tenant) ([]store.Category, error)
	CreateCategory(ctx context.Context, tenant store.Tenant, name, description string) error
	DeleteCategory(ctx context.Context, tenant store.Tenant, name string, removeFromTransactions bool) error
	ApplyCategoryByMerchant(ctx context.Context, tenant store.Tenant, category, pattern string) (int64, error)
	RemoveCategoryByMerchant(ctx context.Context, tenant store.Tenant, category, pattern string) (int64, error)
	GetCategoryMappings(ctx context.Context, tenant store.Tenant) (map[string][]string, error)
	ListBuckets(ctx context.Context, tenant store.Tenant) ([]store.Bucket, error)
	CreateBucket(ctx context.Context, tenant store.Tenant, name, description string) error
	DeleteBucket(ctx context.Context, tenant store.Tenant, name string, removeFromTransactions bool) error
	ApplyBucketByMerchant(ctx context.Context, tenant store.Tenant, bucket, pattern string) (int64, error)
	RemoveBucketByMerchant(ctx context.Context, tenant store.Tenant, bucket, pattern string) (int64, error)
	GetBucketMappings(ctx context.Context, tenant store.Tenant) (map[string][]string, error)
}

type readerRuntimeStore interface {
	SetReaderSecret(ctx context.Context, tenant store.Tenant, reader string, secret []byte) error
	GetReaderSecret(ctx context.Context, tenant store.Tenant, reader string) ([]byte, bool, error)
	SetReaderToken(ctx context.Context, tenant store.Tenant, reader string, token []byte) error
	GetReaderToken(ctx context.Context, tenant store.Tenant, reader string) ([]byte, bool, error)
	DeleteReaderToken(ctx context.Context, tenant store.Tenant, reader string) error
	SetReaderConfig(ctx context.Context, tenant store.Tenant, reader string, config json.RawMessage) error
	GetReaderConfig(ctx context.Context, tenant store.Tenant, reader string) (json.RawMessage, bool, error)
	DeleteReaderRuntime(ctx context.Context, tenant store.Tenant, reader string) error
}

type llmRuntimeStore interface {
	SetLLMProviderConfig(ctx context.Context, tenant store.Tenant, provider string, config json.RawMessage) error
	GetLLMProviderConfig(ctx context.Context, tenant store.Tenant, provider string) (json.RawMessage, bool, error)
	SetLLMProviderCredentials(ctx context.Context, tenant store.Tenant, provider string, credentials []byte) error
	GetLLMProviderCredentials(ctx context.Context, tenant store.Tenant, provider string) ([]byte, bool, error)
	DeleteLLMProviderRuntime(ctx context.Context, tenant store.Tenant, provider string) error
	SetActiveLLMProvider(ctx context.Context, tenant store.Tenant, provider string) error
	ClearActiveLLMProvider(ctx context.Context, tenant store.Tenant) error
	GetActiveLLMProviderRuntime(ctx context.Context, tenant store.Tenant) (store.LLMProviderRuntime, bool, error)
}

type ruleStore interface {
	ListRules(ctx context.Context, tenant store.Tenant) ([]store.RuleRow, error)
	GetRule(ctx context.Context, tenant store.Tenant, id string) (*store.RuleRow, error)
	CreateRule(ctx context.Context, tenant store.Tenant, r store.RuleRow) (*store.RuleRow, error)
	UpdateRule(ctx context.Context, tenant store.Tenant, id string, r store.RuleRow) (*store.RuleRow, error)
	DeleteRule(ctx context.Context, tenant store.Tenant, id string) error
	ImportUserRules(ctx context.Context, tenant store.Tenant, rules []store.RuleRow) error
}

type syncStore interface {
	GetSyncStatus(ctx context.Context) (store.SyncStatus, error)
	GetCommunitySyncSettings(ctx context.Context) (store.CommunitySyncSettings, error)
	PatchCommunitySyncSettings(ctx context.Context, patch store.CommunitySyncSettingsPatch) (store.CommunitySyncSettings, error)
}

type diagnosticStore interface {
	ListExtractionDiagnostics(ctx context.Context, tenant store.Tenant, filter store.DiagnosticFilter) ([]store.ExtractionDiagnosticRow, error)
	GetExtractionDiagnostic(ctx context.Context, tenant store.Tenant, id string) (*store.ExtractionDiagnosticRow, error)
	UpdateExtractionDiagnosticStatus(ctx context.Context, tenant store.Tenant, id, status string) (*store.ExtractionDiagnosticRow, error)
}
