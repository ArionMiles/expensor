package instrumented

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/observability"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
)

// Store records telemetry around the full store surface.
type Store struct {
	auth         store.AuthStore
	analytics    store.AnalyticsStore
	community    store.CommunityStore
	diagnostics  store.DiagnosticStore
	rules        store.RuleStore
	runtime      store.RuntimeStore
	scanning     store.ScanningStore
	taxonomy     store.TaxonomyStore
	transactions store.TransactionStore
	scope        *observability.Scope
}

// StoreDeps groups backend capabilities by the behavior boundaries
// that the instrumentation wrapper decorates.
type StoreDeps struct {
	Auth         store.AuthStore
	Analytics    store.AnalyticsStore
	Community    store.CommunityStore
	Diagnostics  store.DiagnosticStore
	Rules        store.RuleStore
	Runtime      store.RuntimeStore
	Scanning     store.ScanningStore
	Taxonomy     store.TaxonomyStore
	Transactions store.TransactionStore
}

func NewStore(deps StoreDeps, scope *observability.Scope, logger *slog.Logger) *Store {
	if logger == nil {
		logger = slog.Default()
	}
	if scope == nil {
		scope = observability.NewScope(logger, "store")
	}
	return &Store{
		auth:         deps.Auth,
		analytics:    deps.Analytics,
		community:    deps.Community,
		diagnostics:  deps.Diagnostics,
		rules:        deps.Rules,
		runtime:      deps.Runtime,
		scanning:     deps.Scanning,
		taxonomy:     deps.Taxonomy,
		transactions: deps.Transactions,
		scope:        scope,
	}
}

func (s *Store) recordOperation(ctx context.Context, name string, err error) {
	s.scope.RecordOperation(ctx, observability.Operation{
		Namespace: "store",
		Name:      name,
		Err:       err,
	})
}

func (s *Store) BootstrapRequired(ctx context.Context) (bool, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.bootstrap_required")
	defer span.End()

	required, err := s.auth.BootstrapRequired(ctx)
	s.recordOperation(ctx, "auth.bootstrap_required", err)
	return required, err
}

func (s *Store) CreateBootstrapAdmin(ctx context.Context, input store.CreateBootstrapAdminInput) (*store.User, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.create_bootstrap_admin")
	defer span.End()

	user, err := s.auth.CreateBootstrapAdmin(ctx, input)
	s.recordOperation(ctx, "auth.create_bootstrap_admin", err)
	return user, err
}

func (s *Store) CreateUser(ctx context.Context, input store.CreateUserInput) (*store.User, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.create_user")
	defer span.End()

	user, err := s.auth.CreateUser(ctx, input)
	s.recordOperation(ctx, "auth.create_user", err)
	return user, err
}

func (s *Store) ListUsers(ctx context.Context) ([]store.User, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.list_users")
	defer span.End()

	users, err := s.auth.ListUsers(ctx)
	s.recordOperation(ctx, "auth.list_users", err)
	return users, err
}

func (s *Store) UpdateUser(ctx context.Context, id string, input store.UpdateUserInput) (*store.User, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.update_user")
	defer span.End()

	user, err := s.auth.UpdateUser(ctx, id, input)
	s.recordOperation(ctx, "auth.update_user", err)
	return user, err
}

func (s *Store) UpdateUserPassword(ctx context.Context, id string, input store.UpdateUserPasswordInput) error {
	ctx, span := s.scope.Start(ctx, "store.auth.update_user_password")
	defer span.End()

	err := s.auth.UpdateUserPassword(ctx, id, input)
	s.recordOperation(ctx, "auth.update_user_password", err)
	return err
}

func (s *Store) DeleteUser(ctx context.Context, id string) error {
	ctx, span := s.scope.Start(ctx, "store.auth.delete_user")
	defer span.End()

	err := s.auth.DeleteUser(ctx, id)
	s.recordOperation(ctx, "auth.delete_user", err)
	return err
}

func (s *Store) FindUserByEmail(ctx context.Context, email string) (*store.User, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.find_user_by_email")
	defer span.End()

	user, err := s.auth.FindUserByEmail(ctx, email)
	s.recordOperation(ctx, "auth.find_user_by_email", err)
	return user, err
}

func (s *Store) FindUserByID(ctx context.Context, id string) (*store.User, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.find_user_by_id")
	defer span.End()

	user, err := s.auth.FindUserByID(ctx, id)
	s.recordOperation(ctx, "auth.find_user_by_id", err)
	return user, err
}

func (s *Store) CreateSession(ctx context.Context, input store.CreateSessionInput) (*store.Session, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.create_session")
	defer span.End()

	session, err := s.auth.CreateSession(ctx, input)
	s.recordOperation(ctx, "auth.create_session", err)
	return session, err
}

func (s *Store) FindSessionByHash(ctx context.Context, tokenHash string) (*store.Session, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.find_session_by_hash")
	defer span.End()

	session, err := s.auth.FindSessionByHash(ctx, tokenHash)
	s.recordOperation(ctx, "auth.find_session_by_hash", err)
	return session, err
}

func (s *Store) RevokeSession(ctx context.Context, id string) error {
	ctx, span := s.scope.Start(ctx, "store.auth.revoke_session")
	defer span.End()

	err := s.auth.RevokeSession(ctx, id)
	s.recordOperation(ctx, "auth.revoke_session", err)
	return err
}

func (s *Store) CreateAccessToken(ctx context.Context, input store.CreateAccessTokenInput) (*store.AccessToken, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.create_access_token")
	defer span.End()

	token, err := s.auth.CreateAccessToken(ctx, input)
	s.recordOperation(ctx, "auth.create_access_token", err)
	return token, err
}

func (s *Store) ListAccessTokens(ctx context.Context, userID string) ([]store.AccessToken, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.list_access_tokens")
	defer span.End()

	tokens, err := s.auth.ListAccessTokens(ctx, userID)
	s.recordOperation(ctx, "auth.list_access_tokens", err)
	return tokens, err
}

func (s *Store) FindAccessTokenByHash(ctx context.Context, tokenHash string) (*store.AccessToken, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.find_access_token_by_hash")
	defer span.End()

	token, err := s.auth.FindAccessTokenByHash(ctx, tokenHash)
	s.recordOperation(ctx, "auth.find_access_token_by_hash", err)
	return token, err
}

func (s *Store) RevokeAccessToken(ctx context.Context, id, userID string) error {
	ctx, span := s.scope.Start(ctx, "store.auth.revoke_access_token")
	defer span.End()

	err := s.auth.RevokeAccessToken(ctx, id, userID)
	s.recordOperation(ctx, "auth.revoke_access_token", err)
	return err
}

func (s *Store) CreateAccountSetupToken(ctx context.Context, input store.CreateAccountSetupTokenInput) (*store.AccountSetupToken, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.create_account_setup_token")
	defer span.End()

	token, err := s.auth.CreateAccountSetupToken(ctx, input)
	s.recordOperation(ctx, "auth.create_account_setup_token", err)
	return token, err
}

func (s *Store) FindAccountSetupTokenByHash(ctx context.Context, tokenHash string) (*store.AccountSetupToken, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.find_account_setup_token_by_hash")
	defer span.End()

	token, err := s.auth.FindAccountSetupTokenByHash(ctx, tokenHash)
	s.recordOperation(ctx, "auth.find_account_setup_token_by_hash", err)
	return token, err
}

func (s *Store) MarkAccountSetupTokenUsed(ctx context.Context, id string) error {
	ctx, span := s.scope.Start(ctx, "store.auth.mark_account_setup_token_used")
	defer span.End()

	err := s.auth.MarkAccountSetupTokenUsed(ctx, id)
	s.recordOperation(ctx, "auth.mark_account_setup_token_used", err)
	return err
}

func (s *Store) CompleteAccountSetup(ctx context.Context, input store.CompleteAccountSetupInput) (*store.User, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.complete_account_setup")
	defer span.End()

	user, err := s.auth.CompleteAccountSetup(ctx, input)
	s.recordOperation(ctx, "auth.complete_account_setup", err)
	return user, err
}

func (s *Store) ListTransactions(ctx context.Context, tenant store.Tenant, f store.ListFilter) ([]store.Transaction, store.TransactionListResult, error) {
	ctx, span := s.scope.Start(ctx, "store.transactions.list")
	defer span.End()

	rows, result, err := s.transactions.ListTransactions(ctx, tenant, f)
	s.recordOperation(ctx, "transactions.list", err)
	return rows, result, err
}

func (s *Store) GetTransaction(ctx context.Context, tenant store.Tenant, id string) (*store.Transaction, error) {
	ctx, span := s.scope.Start(ctx, "store.transactions.get")
	defer span.End()

	transaction, err := s.transactions.GetTransaction(ctx, tenant, id)
	s.recordOperation(ctx, "transactions.get", err)
	return transaction, err
}

func (s *Store) UpdateDescription(ctx context.Context, tenant store.Tenant, id, description string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.update_description")
	defer span.End()

	err := s.transactions.UpdateDescription(ctx, tenant, id, description)
	s.recordOperation(ctx, "transactions.update_description", err)
	return err
}

func (s *Store) AddLabel(ctx context.Context, tenant store.Tenant, transactionID, label string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.add_label")
	defer span.End()

	err := s.transactions.AddLabel(ctx, tenant, transactionID, label)
	s.recordOperation(ctx, "transactions.add_label", err)
	return err
}

func (s *Store) AddLabels(ctx context.Context, tenant store.Tenant, transactionID string, labels []string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.add_labels")
	defer span.End()

	err := s.transactions.AddLabels(ctx, tenant, transactionID, labels)
	s.recordOperation(ctx, "transactions.add_labels", err)
	return err
}

func (s *Store) RemoveLabel(ctx context.Context, tenant store.Tenant, transactionID, label string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.remove_label")
	defer span.End()

	err := s.transactions.RemoveLabel(ctx, tenant, transactionID, label)
	s.recordOperation(ctx, "transactions.remove_label", err)
	return err
}

func (s *Store) SearchTransactions(
	ctx context.Context,
	tenant store.Tenant,
	query string,
	f store.ListFilter,
) ([]store.Transaction, store.TransactionListResult, error) {
	ctx, span := s.scope.Start(ctx, "store.transactions.search")
	defer span.End()

	rows, result, err := s.transactions.SearchTransactions(ctx, tenant, query, f)
	s.recordOperation(ctx, "transactions.search", err)
	return rows, result, err
}

func (s *Store) GetStats(ctx context.Context, tenant store.Tenant, baseCurrency string) (*store.Stats, error) {
	ctx, span := s.scope.Start(ctx, "store.read_model.get_stats")
	defer span.End()

	stats, err := s.analytics.GetStats(ctx, tenant, baseCurrency)
	s.recordOperation(ctx, "read_model.get_stats", err)
	return stats, err
}

func (s *Store) GetChartData(ctx context.Context, tenant store.Tenant) (*store.ChartData, error) {
	ctx, span := s.scope.Start(ctx, "store.read_model.get_chart_data")
	defer span.End()

	data, err := s.analytics.GetChartData(ctx, tenant)
	s.recordOperation(ctx, "read_model.get_chart_data", err)
	return data, err
}

func (s *Store) GetDashboardData(ctx context.Context, tenant store.Tenant) (*store.DashboardData, error) {
	ctx, span := s.scope.Start(ctx, "store.read_model.get_dashboard_data")
	defer span.End()

	data, err := s.analytics.GetDashboardData(ctx, tenant)
	s.recordOperation(ctx, "read_model.get_dashboard_data", err)
	return data, err
}

func (s *Store) GetSpendingHeatmap(ctx context.Context, tenant store.Tenant, from, to *time.Time) (*store.HeatmapData, error) {
	ctx, span := s.scope.Start(ctx, "store.read_model.get_spending_heatmap")
	defer span.End()

	data, err := s.analytics.GetSpendingHeatmap(ctx, tenant, from, to)
	s.recordOperation(ctx, "read_model.get_spending_heatmap", err)
	return data, err
}

func (s *Store) GetAnnualSpend(ctx context.Context, tenant store.Tenant, year int) ([]store.DailyBucket, error) {
	ctx, span := s.scope.Start(ctx, "store.read_model.get_annual_spend")
	defer span.End()

	buckets, err := s.analytics.GetAnnualSpend(ctx, tenant, year)
	s.recordOperation(ctx, "read_model.get_annual_spend", err)
	return buckets, err
}

func (s *Store) GetAppConfig(ctx context.Context, tenant store.Tenant, key string) (string, error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.get_app_config")
	defer span.End()

	value, err := s.runtime.GetAppConfig(ctx, tenant, key)
	s.recordOperation(ctx, "runtime.get_app_config", err)
	return value, err
}

func (s *Store) SetAppConfig(ctx context.Context, tenant store.Tenant, key, value string) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.set_app_config")
	defer span.End()

	err := s.runtime.SetAppConfig(ctx, tenant, key, value)
	s.recordOperation(ctx, "runtime.set_app_config", err)
	return err
}

func (s *Store) IsMessageProcessed(ctx context.Context, tenant store.Tenant, key string) (bool, error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.is_message_processed")
	defer span.End()

	processed, err := s.runtime.IsMessageProcessed(ctx, tenant, key)
	s.recordOperation(ctx, "runtime.is_message_processed", err)
	return processed, err
}

func (s *Store) MarkMessageProcessed(ctx context.Context, tenant store.Tenant, key string, at time.Time) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.mark_message_processed")
	defer span.End()

	err := s.runtime.MarkMessageProcessed(ctx, tenant, key, at)
	s.recordOperation(ctx, "runtime.mark_message_processed", err)
	return err
}

func (s *Store) GetSchedulerConfig(ctx context.Context) (store.SchedulerConfig, error) {
	ctx, span := s.scope.Start(ctx, "store.scanning.get_scheduler_config")
	defer span.End()

	cfg, err := s.scanning.GetSchedulerConfig(ctx)
	s.recordOperation(ctx, "scanning.get_scheduler_config", err)
	return cfg, err
}

func (s *Store) PatchSchedulerConfig(ctx context.Context, patch store.SchedulerConfigPatch) (store.SchedulerConfig, error) {
	ctx, span := s.scope.Start(ctx, "store.scanning.patch_scheduler_config")
	defer span.End()

	cfg, err := s.scanning.PatchSchedulerConfig(ctx, patch)
	s.recordOperation(ctx, "scanning.patch_scheduler_config", err)
	return cfg, err
}

func (s *Store) EnsureScanningStateForTenant(ctx context.Context, tenant store.Tenant) error {
	ctx, span := s.scope.Start(ctx, "store.scanning.ensure_tenant_state")
	defer span.End()

	err := s.scanning.EnsureScanningStateForTenant(ctx, tenant)
	s.recordOperation(ctx, "scanning.ensure_tenant_state", err)
	return err
}

func (s *Store) GetScanningState(ctx context.Context, tenant store.Tenant) (store.TenantScanningState, error) {
	ctx, span := s.scope.Start(ctx, "store.scanning.get_state")
	defer span.End()

	state, err := s.scanning.GetScanningState(ctx, tenant)
	s.recordOperation(ctx, "scanning.get_state", err)
	return state, err
}

func (s *Store) ListRunnableScanningStates(ctx context.Context) ([]store.TenantScanningState, error) {
	ctx, span := s.scope.Start(ctx, "store.scanning.list_runnable_states")
	defer span.End()

	states, err := s.scanning.ListRunnableScanningStates(ctx)
	s.recordOperation(ctx, "scanning.list_runnable_states", err)
	return states, err
}

func (s *Store) ListScanningStates(ctx context.Context) ([]store.TenantScanningState, error) {
	ctx, span := s.scope.Start(ctx, "store.scanning.list_states")
	defer span.End()

	states, err := s.scanning.ListScanningStates(ctx)
	s.recordOperation(ctx, "scanning.list_states", err)
	return states, err
}

func (s *Store) SetActiveScanningReader(ctx context.Context, tenant store.Tenant, reader string) error {
	ctx, span := s.scope.Start(ctx, "store.scanning.set_active_reader")
	defer span.End()

	err := s.scanning.SetActiveScanningReader(ctx, tenant, reader)
	s.recordOperation(ctx, "scanning.set_active_reader", err)
	return err
}

func (s *Store) ClearActiveScanningReader(ctx context.Context, tenant store.Tenant) error {
	ctx, span := s.scope.Start(ctx, "store.scanning.clear_active_reader")
	defer span.End()

	err := s.scanning.ClearActiveScanningReader(ctx, tenant)
	s.recordOperation(ctx, "scanning.clear_active_reader", err)
	return err
}

func (s *Store) SetScanningEnabled(ctx context.Context, tenant store.Tenant, enabled bool) error {
	ctx, span := s.scope.Start(ctx, "store.scanning.set_enabled")
	defer span.End()

	err := s.scanning.SetScanningEnabled(ctx, tenant, enabled)
	s.recordOperation(ctx, "scanning.set_enabled", err)
	return err
}

func (s *Store) UpdateScanningState(ctx context.Context, tenant store.Tenant, update store.ScanningStateUpdate) error {
	ctx, span := s.scope.Start(ctx, "store.scanning.update_state")
	defer span.End()

	err := s.scanning.UpdateScanningState(ctx, tenant, update)
	s.recordOperation(ctx, "scanning.update_state", err)
	return err
}

func (s *Store) SetReaderSecret(ctx context.Context, tenant store.Tenant, reader string, secret []byte) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.set_reader_secret")
	defer span.End()

	err := s.runtime.SetReaderSecret(ctx, tenant, reader, secret)
	s.recordOperation(ctx, "runtime.set_reader_secret", err)
	return err
}

func (s *Store) GetReaderSecret(ctx context.Context, tenant store.Tenant, reader string) (secret []byte, found bool, err error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.get_reader_secret")
	defer span.End()

	secret, found, err = s.runtime.GetReaderSecret(ctx, tenant, reader)
	s.recordOperation(ctx, "runtime.get_reader_secret", err)
	return secret, found, err
}

func (s *Store) SetReaderToken(ctx context.Context, tenant store.Tenant, reader string, token []byte) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.set_reader_token")
	defer span.End()

	err := s.runtime.SetReaderToken(ctx, tenant, reader, token)
	s.recordOperation(ctx, "runtime.set_reader_token", err)
	return err
}

func (s *Store) GetReaderToken(ctx context.Context, tenant store.Tenant, reader string) (token []byte, found bool, err error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.get_reader_token")
	defer span.End()

	token, found, err = s.runtime.GetReaderToken(ctx, tenant, reader)
	s.recordOperation(ctx, "runtime.get_reader_token", err)
	return token, found, err
}

func (s *Store) DeleteReaderToken(ctx context.Context, tenant store.Tenant, reader string) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.delete_reader_token")
	defer span.End()

	err := s.runtime.DeleteReaderToken(ctx, tenant, reader)
	s.recordOperation(ctx, "runtime.delete_reader_token", err)
	return err
}

func (s *Store) SetReaderConfig(ctx context.Context, tenant store.Tenant, reader string, config json.RawMessage) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.set_reader_config")
	defer span.End()

	err := s.runtime.SetReaderConfig(ctx, tenant, reader, config)
	s.recordOperation(ctx, "runtime.set_reader_config", err)
	return err
}

func (s *Store) GetReaderConfig(ctx context.Context, tenant store.Tenant, reader string) (json.RawMessage, bool, error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.get_reader_config")
	defer span.End()

	config, found, err := s.runtime.GetReaderConfig(ctx, tenant, reader)
	s.recordOperation(ctx, "runtime.get_reader_config", err)
	return config, found, err
}

func (s *Store) SetLLMProviderConfig(ctx context.Context, tenant store.Tenant, provider string, config json.RawMessage) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.set_llm_provider_config")
	defer span.End()

	err := s.runtime.SetLLMProviderConfig(ctx, tenant, provider, config)
	s.recordOperation(ctx, "runtime.set_llm_provider_config", err)
	return err
}

func (s *Store) GetLLMProviderConfig(ctx context.Context, tenant store.Tenant, provider string) (json.RawMessage, bool, error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.get_llm_provider_config")
	defer span.End()

	config, found, err := s.runtime.GetLLMProviderConfig(ctx, tenant, provider)
	s.recordOperation(ctx, "runtime.get_llm_provider_config", err)
	return config, found, err
}

func (s *Store) SetLLMProviderCredentials(ctx context.Context, tenant store.Tenant, provider string, credentials []byte) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.set_llm_provider_credentials")
	defer span.End()

	err := s.runtime.SetLLMProviderCredentials(ctx, tenant, provider, credentials)
	s.recordOperation(ctx, "runtime.set_llm_provider_credentials", err)
	return err
}

func (s *Store) GetLLMProviderCredentials(
	ctx context.Context,
	tenant store.Tenant,
	provider string,
) (credentials []byte, found bool, err error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.get_llm_provider_credentials")
	defer span.End()

	credentials, found, err = s.runtime.GetLLMProviderCredentials(ctx, tenant, provider)
	s.recordOperation(ctx, "runtime.get_llm_provider_credentials", err)
	return credentials, found, err
}

func (s *Store) DeleteLLMProviderRuntime(ctx context.Context, tenant store.Tenant, provider string) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.delete_llm_provider_runtime")
	defer span.End()

	err := s.runtime.DeleteLLMProviderRuntime(ctx, tenant, provider)
	s.recordOperation(ctx, "runtime.delete_llm_provider_runtime", err)
	return err
}

func (s *Store) SetActiveLLMProvider(ctx context.Context, tenant store.Tenant, provider string) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.set_active_llm_provider")
	defer span.End()

	err := s.runtime.SetActiveLLMProvider(ctx, tenant, provider)
	s.recordOperation(ctx, "runtime.set_active_llm_provider", err)
	return err
}

func (s *Store) ClearActiveLLMProvider(ctx context.Context, tenant store.Tenant) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.clear_active_llm_provider")
	defer span.End()

	err := s.runtime.ClearActiveLLMProvider(ctx, tenant)
	s.recordOperation(ctx, "runtime.clear_active_llm_provider", err)
	return err
}

func (s *Store) GetActiveLLMProviderRuntime(ctx context.Context, tenant store.Tenant) (store.LLMProviderRuntime, bool, error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.get_active_llm_provider")
	defer span.End()

	runtime, found, err := s.runtime.GetActiveLLMProviderRuntime(ctx, tenant)
	s.recordOperation(ctx, "runtime.get_active_llm_provider", err)
	return runtime, found, err
}

func (s *Store) DeleteReaderRuntime(ctx context.Context, tenant store.Tenant, reader string) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.delete_reader_runtime")
	defer span.End()

	err := s.runtime.DeleteReaderRuntime(ctx, tenant, reader)
	s.recordOperation(ctx, "runtime.delete_reader_runtime", err)
	return err
}

func (s *Store) GetCommunityURL(ctx context.Context) (string, error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.get_community_url")
	defer span.End()

	url, err := s.runtime.GetCommunityURL(ctx)
	s.recordOperation(ctx, "runtime.get_community_url", err)
	return url, err
}

func (s *Store) SetCommunityURL(ctx context.Context, url string) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.set_community_url")
	defer span.End()

	err := s.runtime.SetCommunityURL(ctx, url)
	s.recordOperation(ctx, "runtime.set_community_url", err)
	return err
}

func (s *Store) GetFacets(ctx context.Context, tenant store.Tenant) (*store.Facets, error) {
	ctx, span := s.scope.Start(ctx, "store.transactions.get_facets")
	defer span.End()

	facets, err := s.transactions.GetFacets(ctx, tenant)
	s.recordOperation(ctx, "transactions.get_facets", err)
	return facets, err
}

func (s *Store) ListLabels(ctx context.Context, tenant store.Tenant) ([]store.Label, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.list_labels")
	defer span.End()

	labels, err := s.taxonomy.ListLabels(ctx, tenant)
	s.recordOperation(ctx, "taxonomy.list_labels", err)
	return labels, err
}

func (s *Store) CreateLabel(ctx context.Context, tenant store.Tenant, name, color string) error {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.create_label")
	defer span.End()

	err := s.taxonomy.CreateLabel(ctx, tenant, name, color)
	s.recordOperation(ctx, "taxonomy.create_label", err)
	return err
}

func (s *Store) UpdateLabel(ctx context.Context, tenant store.Tenant, name, color string) error {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.update_label")
	defer span.End()

	err := s.taxonomy.UpdateLabel(ctx, tenant, name, color)
	s.recordOperation(ctx, "taxonomy.update_label", err)
	return err
}

func (s *Store) DeleteLabel(ctx context.Context, tenant store.Tenant, name string, removeFromTransactions bool) error {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.delete_label")
	defer span.End()

	err := s.taxonomy.DeleteLabel(ctx, tenant, name, removeFromTransactions)
	s.recordOperation(ctx, "taxonomy.delete_label", err)
	return err
}

func (s *Store) ApplyLabelByMerchant(ctx context.Context, tenant store.Tenant, label, pattern string) (int64, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.apply_label_by_merchant")
	defer span.End()

	affected, err := s.taxonomy.ApplyLabelByMerchant(ctx, tenant, label, pattern)
	s.recordOperation(ctx, "taxonomy.apply_label_by_merchant", err)
	return affected, err
}

func (s *Store) RemoveLabelByMerchant(ctx context.Context, tenant store.Tenant, label, pattern string) (int64, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.remove_label_by_merchant")
	defer span.End()

	removed, err := s.taxonomy.RemoveLabelByMerchant(ctx, tenant, label, pattern)
	s.recordOperation(ctx, "taxonomy.remove_label_by_merchant", err)
	return removed, err
}

func (s *Store) GetLabelMappings(ctx context.Context, tenant store.Tenant) (map[string][]string, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.get_label_mappings")
	defer span.End()

	mappings, err := s.taxonomy.GetLabelMappings(ctx, tenant)
	s.recordOperation(ctx, "taxonomy.get_label_mappings", err)
	return mappings, err
}

func (s *Store) GetMonthlyBreakdownSpend(ctx context.Context, tenant store.Tenant, dimension string, months int) (*store.MonthlyBreakdownData, error) {
	ctx, span := s.scope.Start(ctx, "store.read_model.get_monthly_breakdown_spend")
	defer span.End()

	data, err := s.analytics.GetMonthlyBreakdownSpend(ctx, tenant, dimension, months)
	s.recordOperation(ctx, "read_model.get_monthly_breakdown_spend", err)
	return data, err
}

func (s *Store) ListCategories(ctx context.Context, tenant store.Tenant) ([]store.Category, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.list_categories")
	defer span.End()

	categories, err := s.taxonomy.ListCategories(ctx, tenant)
	s.recordOperation(ctx, "taxonomy.list_categories", err)
	return categories, err
}

func (s *Store) CreateCategory(ctx context.Context, tenant store.Tenant, name, description string) error {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.create_category")
	defer span.End()

	err := s.taxonomy.CreateCategory(ctx, tenant, name, description)
	s.recordOperation(ctx, "taxonomy.create_category", err)
	return err
}

func (s *Store) DeleteCategory(ctx context.Context, tenant store.Tenant, name string, removeFromTransactions bool) error {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.delete_category")
	defer span.End()

	err := s.taxonomy.DeleteCategory(ctx, tenant, name, removeFromTransactions)
	s.recordOperation(ctx, "taxonomy.delete_category", err)
	return err
}

func (s *Store) ApplyCategoryByMerchant(ctx context.Context, tenant store.Tenant, category, pattern string) (int64, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.apply_category_by_merchant")
	defer span.End()

	affected, err := s.taxonomy.ApplyCategoryByMerchant(ctx, tenant, category, pattern)
	s.recordOperation(ctx, "taxonomy.apply_category_by_merchant", err)
	return affected, err
}

func (s *Store) RemoveCategoryByMerchant(ctx context.Context, tenant store.Tenant, category, pattern string) (int64, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.remove_category_by_merchant")
	defer span.End()

	removed, err := s.taxonomy.RemoveCategoryByMerchant(ctx, tenant, category, pattern)
	s.recordOperation(ctx, "taxonomy.remove_category_by_merchant", err)
	return removed, err
}

func (s *Store) GetCategoryMappings(ctx context.Context, tenant store.Tenant) (map[string][]string, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.get_category_mappings")
	defer span.End()

	mappings, err := s.taxonomy.GetCategoryMappings(ctx, tenant)
	s.recordOperation(ctx, "taxonomy.get_category_mappings", err)
	return mappings, err
}

func (s *Store) ListBuckets(ctx context.Context, tenant store.Tenant) ([]store.Bucket, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.list_buckets")
	defer span.End()

	buckets, err := s.taxonomy.ListBuckets(ctx, tenant)
	s.recordOperation(ctx, "taxonomy.list_buckets", err)
	return buckets, err
}

func (s *Store) CreateBucket(ctx context.Context, tenant store.Tenant, name, description string) error {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.create_bucket")
	defer span.End()

	err := s.taxonomy.CreateBucket(ctx, tenant, name, description)
	s.recordOperation(ctx, "taxonomy.create_bucket", err)
	return err
}

func (s *Store) DeleteBucket(ctx context.Context, tenant store.Tenant, name string, removeFromTransactions bool) error {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.delete_bucket")
	defer span.End()

	err := s.taxonomy.DeleteBucket(ctx, tenant, name, removeFromTransactions)
	s.recordOperation(ctx, "taxonomy.delete_bucket", err)
	return err
}

func (s *Store) ApplyBucketByMerchant(ctx context.Context, tenant store.Tenant, bucket, pattern string) (int64, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.apply_bucket_by_merchant")
	defer span.End()

	affected, err := s.taxonomy.ApplyBucketByMerchant(ctx, tenant, bucket, pattern)
	s.recordOperation(ctx, "taxonomy.apply_bucket_by_merchant", err)
	return affected, err
}

func (s *Store) RemoveBucketByMerchant(ctx context.Context, tenant store.Tenant, bucket, pattern string) (int64, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.remove_bucket_by_merchant")
	defer span.End()

	removed, err := s.taxonomy.RemoveBucketByMerchant(ctx, tenant, bucket, pattern)
	s.recordOperation(ctx, "taxonomy.remove_bucket_by_merchant", err)
	return removed, err
}

func (s *Store) GetBucketMappings(ctx context.Context, tenant store.Tenant) (map[string][]string, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.get_bucket_mappings")
	defer span.End()

	mappings, err := s.taxonomy.GetBucketMappings(ctx, tenant)
	s.recordOperation(ctx, "taxonomy.get_bucket_mappings", err)
	return mappings, err
}

func (s *Store) UpdateTransaction(ctx context.Context, tenant store.Tenant, id string, u store.TransactionUpdate) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.update")
	defer span.End()

	err := s.transactions.UpdateTransaction(ctx, tenant, id, u)
	s.recordOperation(ctx, "transactions.update", err)
	return err
}

func (s *Store) MuteTransaction(ctx context.Context, tenant store.Tenant, id string, muted bool, reason string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.mute")
	defer span.End()

	err := s.transactions.MuteTransaction(ctx, tenant, id, muted, reason)
	s.recordOperation(ctx, "transactions.mute", err)
	return err
}

func (s *Store) UpdateMuteReason(ctx context.Context, tenant store.Tenant, id, reason string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.update_mute_reason")
	defer span.End()

	err := s.transactions.UpdateMuteReason(ctx, tenant, id, reason)
	s.recordOperation(ctx, "transactions.update_mute_reason", err)
	return err
}

func (s *Store) UpdateMerchantReason(ctx context.Context, tenant store.Tenant, id, reason string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.update_merchant_reason")
	defer span.End()

	err := s.transactions.UpdateMerchantReason(ctx, tenant, id, reason)
	s.recordOperation(ctx, "transactions.update_merchant_reason", err)
	return err
}

func (s *Store) MuteByMerchant(ctx context.Context, tenant store.Tenant, pattern, reason string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.mute_by_merchant")
	defer span.End()

	err := s.transactions.MuteByMerchant(ctx, tenant, pattern, reason)
	s.recordOperation(ctx, "transactions.mute_by_merchant", err)
	return err
}

func (s *Store) ListMutedMerchants(ctx context.Context, tenant store.Tenant) ([]store.MutedMerchant, error) {
	ctx, span := s.scope.Start(ctx, "store.transactions.list_muted_merchants")
	defer span.End()

	merchants, err := s.transactions.ListMutedMerchants(ctx, tenant)
	s.recordOperation(ctx, "transactions.list_muted_merchants", err)
	return merchants, err
}

func (s *Store) GetMutedMerchantsWithCount(ctx context.Context, tenant store.Tenant) ([]store.MutedMerchantWithCount, error) {
	ctx, span := s.scope.Start(ctx, "store.transactions.get_muted_merchants_with_count")
	defer span.End()

	merchants, err := s.transactions.GetMutedMerchantsWithCount(ctx, tenant)
	s.recordOperation(ctx, "transactions.get_muted_merchants_with_count", err)
	return merchants, err
}

func (s *Store) DeleteMutedMerchant(ctx context.Context, tenant store.Tenant, id string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.delete_muted_merchant")
	defer span.End()

	err := s.transactions.DeleteMutedMerchant(ctx, tenant, id)
	s.recordOperation(ctx, "transactions.delete_muted_merchant", err)
	return err
}

func (s *Store) UnmuteByPattern(ctx context.Context, tenant store.Tenant, pattern string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.unmute_by_pattern")
	defer span.End()

	err := s.transactions.UnmuteByPattern(ctx, tenant, pattern)
	s.recordOperation(ctx, "transactions.unmute_by_pattern", err)
	return err
}

func (s *Store) DeleteMutedMerchantAndUnmute(ctx context.Context, tenant store.Tenant, id string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.delete_muted_merchant_and_unmute")
	defer span.End()

	err := s.transactions.DeleteMutedMerchantAndUnmute(ctx, tenant, id)
	s.recordOperation(ctx, "transactions.delete_muted_merchant_and_unmute", err)
	return err
}

func (s *Store) GetMutedMerchantPatterns(ctx context.Context, tenant store.Tenant) ([]string, error) {
	ctx, span := s.scope.Start(ctx, "store.transactions.get_muted_merchant_patterns")
	defer span.End()

	patterns, err := s.transactions.GetMutedMerchantPatterns(ctx, tenant)
	s.recordOperation(ctx, "transactions.get_muted_merchant_patterns", err)
	return patterns, err
}

func (s *Store) CategorizeMerchant(ctx context.Context, tenant store.Tenant, merchant, category, bucket string) (int64, error) {
	ctx, span := s.scope.Start(ctx, "store.community.categorize_merchant")
	defer span.End()

	updated, err := s.community.CategorizeMerchant(ctx, tenant, merchant, category, bucket)
	s.recordOperation(ctx, "community.categorize_merchant", err)
	return updated, err
}

func (s *Store) ListRules(ctx context.Context, tenant store.Tenant) ([]store.RuleRow, error) {
	ctx, span := s.scope.Start(ctx, "store.rules.list")
	defer span.End()

	rules, err := s.rules.ListRules(ctx, tenant)
	s.recordOperation(ctx, "rules.list", err)
	return rules, err
}

func (s *Store) GetRule(ctx context.Context, tenant store.Tenant, id string) (*store.RuleRow, error) {
	ctx, span := s.scope.Start(ctx, "store.rules.get")
	defer span.End()

	rule, err := s.rules.GetRule(ctx, tenant, id)
	s.recordOperation(ctx, "rules.get", err)
	return rule, err
}

func (s *Store) CreateRule(ctx context.Context, tenant store.Tenant, r store.RuleRow) (*store.RuleRow, error) {
	ctx, span := s.scope.Start(ctx, "store.rules.create")
	defer span.End()

	rule, err := s.rules.CreateRule(ctx, tenant, r)
	s.recordOperation(ctx, "rules.create", err)
	return rule, err
}

func (s *Store) UpdateRule(ctx context.Context, tenant store.Tenant, id string, r store.RuleRow) (*store.RuleRow, error) {
	ctx, span := s.scope.Start(ctx, "store.rules.update")
	defer span.End()

	rule, err := s.rules.UpdateRule(ctx, tenant, id, r)
	s.recordOperation(ctx, "rules.update", err)
	return rule, err
}

func (s *Store) DeleteRule(ctx context.Context, tenant store.Tenant, id string) error {
	ctx, span := s.scope.Start(ctx, "store.rules.delete")
	defer span.End()

	err := s.rules.DeleteRule(ctx, tenant, id)
	s.recordOperation(ctx, "rules.delete", err)
	return err
}

func (s *Store) SeedPredefinedRules(ctx context.Context, rules []store.RuleRow) error {
	ctx, span := s.scope.Start(ctx, "store.rules.seed_predefined")
	defer span.End()

	err := s.rules.SeedPredefinedRules(ctx, rules)
	s.recordOperation(ctx, "rules.seed_predefined", err)
	return err
}

func (s *Store) ImportUserRules(ctx context.Context, tenant store.Tenant, rules []store.RuleRow) error {
	ctx, span := s.scope.Start(ctx, "store.rules.import_user")
	defer span.End()

	err := s.rules.ImportUserRules(ctx, tenant, rules)
	s.recordOperation(ctx, "rules.import_user", err)
	return err
}

func (s *Store) SeedMCCCodes(ctx context.Context, entries []store.MCCEntry) error {
	ctx, span := s.scope.Start(ctx, "store.community.seed_mcc_codes")
	defer span.End()

	err := s.community.SeedMCCCodes(ctx, entries)
	s.recordOperation(ctx, "community.seed_mcc_codes", err)
	return err
}

func (s *Store) SeedMerchantCategories(ctx context.Context, entries []store.MerchantCategoryEntry) (int64, error) {
	ctx, span := s.scope.Start(ctx, "store.community.seed_merchant_categories")
	defer span.End()

	updated, err := s.community.SeedMerchantCategories(ctx, entries)
	s.recordOperation(ctx, "community.seed_merchant_categories", err)
	return updated, err
}

func (s *Store) LoadCategorySnapshot(ctx context.Context) (api.CategoryResolver, error) {
	ctx, span := s.scope.Start(ctx, "store.community.load_category_snapshot")
	defer span.End()

	resolver, err := s.community.LoadCategorySnapshot(ctx)
	s.recordOperation(ctx, "community.load_category_snapshot", err)
	return resolver, err
}

func (s *Store) SeedMCCCategories(ctx context.Context, names []string) error {
	ctx, span := s.scope.Start(ctx, "store.community.seed_mcc_categories")
	defer span.End()

	err := s.community.SeedMCCCategories(ctx, names)
	s.recordOperation(ctx, "community.seed_mcc_categories", err)
	return err
}

func (s *Store) GetSyncStatus(ctx context.Context) (store.SyncStatus, error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.get_sync_status")
	defer span.End()

	status, err := s.runtime.GetSyncStatus(ctx)
	s.recordOperation(ctx, "runtime.get_sync_status", err)
	return status, err
}

func (s *Store) SetSyncStatus(ctx context.Context, status store.SyncStatus) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.set_sync_status")
	defer span.End()

	err := s.runtime.SetSyncStatus(ctx, status)
	s.recordOperation(ctx, "runtime.set_sync_status", err)
	return err
}

func (s *Store) GetCommunitySyncSettings(ctx context.Context) (store.CommunitySyncSettings, error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.get_community_sync_settings")
	defer span.End()

	settings, err := s.runtime.GetCommunitySyncSettings(ctx)
	s.recordOperation(ctx, "runtime.get_community_sync_settings", err)
	return settings, err
}

func (s *Store) PatchCommunitySyncSettings(
	ctx context.Context,
	patch store.CommunitySyncSettingsPatch,
) (store.CommunitySyncSettings, error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.patch_community_sync_settings")
	defer span.End()

	settings, err := s.runtime.PatchCommunitySyncSettings(ctx, patch)
	s.recordOperation(ctx, "runtime.patch_community_sync_settings", err)
	return settings, err
}

func (s *Store) ListExtractionDiagnostics(ctx context.Context, tenant store.Tenant, filter store.DiagnosticFilter) ([]store.ExtractionDiagnosticRow, error) {
	ctx, span := s.scope.Start(ctx, "store.diagnostics.list_extraction")
	defer span.End()

	rows, err := s.diagnostics.ListExtractionDiagnostics(ctx, tenant, filter)
	s.recordOperation(ctx, "diagnostics.list_extraction", err)
	return rows, err
}

func (s *Store) GetExtractionDiagnostic(ctx context.Context, tenant store.Tenant, id string) (*store.ExtractionDiagnosticRow, error) {
	ctx, span := s.scope.Start(ctx, "store.diagnostics.get_extraction")
	defer span.End()

	row, err := s.diagnostics.GetExtractionDiagnostic(ctx, tenant, id)
	s.recordOperation(ctx, "diagnostics.get_extraction", err)
	return row, err
}

func (s *Store) UpdateExtractionDiagnosticStatus(ctx context.Context, tenant store.Tenant, id, status string) (*store.ExtractionDiagnosticRow, error) {
	ctx, span := s.scope.Start(ctx, "store.diagnostics.update_extraction_status")
	defer span.End()

	row, err := s.diagnostics.UpdateExtractionDiagnosticStatus(ctx, tenant, id, status)
	s.recordOperation(ctx, "diagnostics.update_extraction_status", err)
	return row, err
}

func (s *Store) RecordExtractionDiagnostic(ctx context.Context, tenant store.Tenant, diagnostic api.ExtractionDiagnostic) error {
	ctx, span := s.scope.Start(ctx, "store.diagnostics.record_tenant_extraction")
	defer span.End()

	err := s.diagnostics.RecordExtractionDiagnostic(ctx, tenant, diagnostic)
	s.recordOperation(ctx, "diagnostics.record_tenant_extraction", err)
	return err
}
