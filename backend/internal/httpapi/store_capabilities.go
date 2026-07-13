package httpapi

import (
	"context"
	"encoding/json"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

type authStore interface {
	store.AuthStore
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
	store.AnalyticsStore
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
	store.TaxonomyStore
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
