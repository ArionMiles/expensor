package store

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/observability"
	"github.com/ArionMiles/expensor/backend/pkg/api"
)

// InstrumentedStore records telemetry around the full store surface.
type InstrumentedStore struct {
	next  Backend
	scope *observability.Scope
}

func NewInstrumentedStore(next Backend, scope *observability.Scope, logger *slog.Logger) *InstrumentedStore {
	if logger == nil {
		logger = slog.Default()
	}
	if scope == nil {
		scope = observability.NewScope(logger, "store")
	}
	return &InstrumentedStore{next: next, scope: scope}
}

func (s *InstrumentedStore) recordOperation(ctx context.Context, name string, err error) {
	s.scope.RecordOperation(ctx, observability.Operation{
		Namespace: "store",
		Name:      name,
		Err:       err,
	})
}

func (s *InstrumentedStore) BootstrapRequired(ctx context.Context) (bool, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.bootstrap_required")
	defer span.End()

	required, err := s.next.BootstrapRequired(ctx)
	s.recordOperation(ctx, "auth.bootstrap_required", err)
	return required, err
}

func (s *InstrumentedStore) CreateBootstrapAdmin(ctx context.Context, input CreateBootstrapAdminInput) (*User, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.create_bootstrap_admin")
	defer span.End()

	user, err := s.next.CreateBootstrapAdmin(ctx, input)
	s.recordOperation(ctx, "auth.create_bootstrap_admin", err)
	return user, err
}

func (s *InstrumentedStore) CreateUser(ctx context.Context, input CreateUserInput) (*User, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.create_user")
	defer span.End()

	user, err := s.next.CreateUser(ctx, input)
	s.recordOperation(ctx, "auth.create_user", err)
	return user, err
}

func (s *InstrumentedStore) ListUsers(ctx context.Context) ([]User, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.list_users")
	defer span.End()

	users, err := s.next.ListUsers(ctx)
	s.recordOperation(ctx, "auth.list_users", err)
	return users, err
}

func (s *InstrumentedStore) UpdateUser(ctx context.Context, id string, input UpdateUserInput) (*User, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.update_user")
	defer span.End()

	user, err := s.next.UpdateUser(ctx, id, input)
	s.recordOperation(ctx, "auth.update_user", err)
	return user, err
}

func (s *InstrumentedStore) UpdateUserPassword(ctx context.Context, id string, input UpdateUserPasswordInput) error {
	ctx, span := s.scope.Start(ctx, "store.auth.update_user_password")
	defer span.End()

	err := s.next.UpdateUserPassword(ctx, id, input)
	s.recordOperation(ctx, "auth.update_user_password", err)
	return err
}

func (s *InstrumentedStore) DeleteUser(ctx context.Context, id string) error {
	ctx, span := s.scope.Start(ctx, "store.auth.delete_user")
	defer span.End()

	err := s.next.DeleteUser(ctx, id)
	s.recordOperation(ctx, "auth.delete_user", err)
	return err
}

func (s *InstrumentedStore) FindUserByEmail(ctx context.Context, email string) (*User, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.find_user_by_email")
	defer span.End()

	user, err := s.next.FindUserByEmail(ctx, email)
	s.recordOperation(ctx, "auth.find_user_by_email", err)
	return user, err
}

func (s *InstrumentedStore) FindUserByID(ctx context.Context, id string) (*User, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.find_user_by_id")
	defer span.End()

	user, err := s.next.FindUserByID(ctx, id)
	s.recordOperation(ctx, "auth.find_user_by_id", err)
	return user, err
}

func (s *InstrumentedStore) CreateSession(ctx context.Context, input CreateSessionInput) (*Session, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.create_session")
	defer span.End()

	session, err := s.next.CreateSession(ctx, input)
	s.recordOperation(ctx, "auth.create_session", err)
	return session, err
}

func (s *InstrumentedStore) FindSessionByHash(ctx context.Context, tokenHash string) (*Session, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.find_session_by_hash")
	defer span.End()

	session, err := s.next.FindSessionByHash(ctx, tokenHash)
	s.recordOperation(ctx, "auth.find_session_by_hash", err)
	return session, err
}

func (s *InstrumentedStore) RevokeSession(ctx context.Context, id string) error {
	ctx, span := s.scope.Start(ctx, "store.auth.revoke_session")
	defer span.End()

	err := s.next.RevokeSession(ctx, id)
	s.recordOperation(ctx, "auth.revoke_session", err)
	return err
}

func (s *InstrumentedStore) CreateAccessToken(ctx context.Context, input CreateAccessTokenInput) (*AccessToken, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.create_access_token")
	defer span.End()

	token, err := s.next.CreateAccessToken(ctx, input)
	s.recordOperation(ctx, "auth.create_access_token", err)
	return token, err
}

func (s *InstrumentedStore) ListAccessTokens(ctx context.Context, userID string) ([]AccessToken, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.list_access_tokens")
	defer span.End()

	tokens, err := s.next.ListAccessTokens(ctx, userID)
	s.recordOperation(ctx, "auth.list_access_tokens", err)
	return tokens, err
}

func (s *InstrumentedStore) FindAccessTokenByHash(ctx context.Context, tokenHash string) (*AccessToken, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.find_access_token_by_hash")
	defer span.End()

	token, err := s.next.FindAccessTokenByHash(ctx, tokenHash)
	s.recordOperation(ctx, "auth.find_access_token_by_hash", err)
	return token, err
}

func (s *InstrumentedStore) RevokeAccessToken(ctx context.Context, id, userID string) error {
	ctx, span := s.scope.Start(ctx, "store.auth.revoke_access_token")
	defer span.End()

	err := s.next.RevokeAccessToken(ctx, id, userID)
	s.recordOperation(ctx, "auth.revoke_access_token", err)
	return err
}

func (s *InstrumentedStore) CreateAccountSetupToken(ctx context.Context, input CreateAccountSetupTokenInput) (*AccountSetupToken, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.create_account_setup_token")
	defer span.End()

	token, err := s.next.CreateAccountSetupToken(ctx, input)
	s.recordOperation(ctx, "auth.create_account_setup_token", err)
	return token, err
}

func (s *InstrumentedStore) FindAccountSetupTokenByHash(ctx context.Context, tokenHash string) (*AccountSetupToken, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.find_account_setup_token_by_hash")
	defer span.End()

	token, err := s.next.FindAccountSetupTokenByHash(ctx, tokenHash)
	s.recordOperation(ctx, "auth.find_account_setup_token_by_hash", err)
	return token, err
}

func (s *InstrumentedStore) MarkAccountSetupTokenUsed(ctx context.Context, id string) error {
	ctx, span := s.scope.Start(ctx, "store.auth.mark_account_setup_token_used")
	defer span.End()

	err := s.next.MarkAccountSetupTokenUsed(ctx, id)
	s.recordOperation(ctx, "auth.mark_account_setup_token_used", err)
	return err
}

func (s *InstrumentedStore) CompleteAccountSetup(ctx context.Context, input CompleteAccountSetupInput) (*User, error) {
	ctx, span := s.scope.Start(ctx, "store.auth.complete_account_setup")
	defer span.End()

	user, err := s.next.CompleteAccountSetup(ctx, input)
	s.recordOperation(ctx, "auth.complete_account_setup", err)
	return user, err
}

func (s *InstrumentedStore) ListTransactions(ctx context.Context, tenant Tenant, f ListFilter) ([]Transaction, TransactionListResult, error) {
	ctx, span := s.scope.Start(ctx, "store.transactions.list")
	defer span.End()

	rows, result, err := s.next.ListTransactions(ctx, tenant, f)
	s.recordOperation(ctx, "transactions.list", err)
	return rows, result, err
}

func (s *InstrumentedStore) GetTransaction(ctx context.Context, tenant Tenant, id string) (*Transaction, error) {
	ctx, span := s.scope.Start(ctx, "store.transactions.get")
	defer span.End()

	transaction, err := s.next.GetTransaction(ctx, tenant, id)
	s.recordOperation(ctx, "transactions.get", err)
	return transaction, err
}

func (s *InstrumentedStore) UpdateDescription(ctx context.Context, tenant Tenant, id, description string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.update_description")
	defer span.End()

	err := s.next.UpdateDescription(ctx, tenant, id, description)
	s.recordOperation(ctx, "transactions.update_description", err)
	return err
}

func (s *InstrumentedStore) AddLabel(ctx context.Context, tenant Tenant, transactionID, label string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.add_label")
	defer span.End()

	err := s.next.AddLabel(ctx, tenant, transactionID, label)
	s.recordOperation(ctx, "transactions.add_label", err)
	return err
}

func (s *InstrumentedStore) AddLabels(ctx context.Context, tenant Tenant, transactionID string, labels []string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.add_labels")
	defer span.End()

	err := s.next.AddLabels(ctx, tenant, transactionID, labels)
	s.recordOperation(ctx, "transactions.add_labels", err)
	return err
}

func (s *InstrumentedStore) RemoveLabel(ctx context.Context, tenant Tenant, transactionID, label string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.remove_label")
	defer span.End()

	err := s.next.RemoveLabel(ctx, tenant, transactionID, label)
	s.recordOperation(ctx, "transactions.remove_label", err)
	return err
}

func (s *InstrumentedStore) SearchTransactions(ctx context.Context, tenant Tenant, query string, f ListFilter) ([]Transaction, TransactionListResult, error) {
	ctx, span := s.scope.Start(ctx, "store.transactions.search")
	defer span.End()

	rows, result, err := s.next.SearchTransactions(ctx, tenant, query, f)
	s.recordOperation(ctx, "transactions.search", err)
	return rows, result, err
}

func (s *InstrumentedStore) GetStats(ctx context.Context, tenant Tenant, baseCurrency string) (*Stats, error) {
	ctx, span := s.scope.Start(ctx, "store.read_model.get_stats")
	defer span.End()

	stats, err := s.next.GetStats(ctx, tenant, baseCurrency)
	s.recordOperation(ctx, "read_model.get_stats", err)
	return stats, err
}

func (s *InstrumentedStore) GetChartData(ctx context.Context, tenant Tenant) (*ChartData, error) {
	ctx, span := s.scope.Start(ctx, "store.read_model.get_chart_data")
	defer span.End()

	data, err := s.next.GetChartData(ctx, tenant)
	s.recordOperation(ctx, "read_model.get_chart_data", err)
	return data, err
}

func (s *InstrumentedStore) GetDashboardData(ctx context.Context, tenant Tenant) (*DashboardData, error) {
	ctx, span := s.scope.Start(ctx, "store.read_model.get_dashboard_data")
	defer span.End()

	data, err := s.next.GetDashboardData(ctx, tenant)
	s.recordOperation(ctx, "read_model.get_dashboard_data", err)
	return data, err
}

func (s *InstrumentedStore) GetSpendingHeatmap(ctx context.Context, tenant Tenant, from, to *time.Time) (*HeatmapData, error) {
	ctx, span := s.scope.Start(ctx, "store.read_model.get_spending_heatmap")
	defer span.End()

	data, err := s.next.GetSpendingHeatmap(ctx, tenant, from, to)
	s.recordOperation(ctx, "read_model.get_spending_heatmap", err)
	return data, err
}

func (s *InstrumentedStore) GetAnnualSpend(ctx context.Context, tenant Tenant, year int) ([]DailyBucket, error) {
	ctx, span := s.scope.Start(ctx, "store.read_model.get_annual_spend")
	defer span.End()

	buckets, err := s.next.GetAnnualSpend(ctx, tenant, year)
	s.recordOperation(ctx, "read_model.get_annual_spend", err)
	return buckets, err
}

func (s *InstrumentedStore) GetAppConfig(ctx context.Context, tenant Tenant, key string) (string, error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.get_app_config")
	defer span.End()

	value, err := s.next.GetAppConfig(ctx, tenant, key)
	s.recordOperation(ctx, "runtime.get_app_config", err)
	return value, err
}

func (s *InstrumentedStore) SetAppConfig(ctx context.Context, tenant Tenant, key, value string) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.set_app_config")
	defer span.End()

	err := s.next.SetAppConfig(ctx, tenant, key, value)
	s.recordOperation(ctx, "runtime.set_app_config", err)
	return err
}

func (s *InstrumentedStore) IsMessageProcessed(ctx context.Context, tenant Tenant, key string) (bool, error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.is_message_processed")
	defer span.End()

	processed, err := s.next.IsMessageProcessed(ctx, tenant, key)
	s.recordOperation(ctx, "runtime.is_message_processed", err)
	return processed, err
}

func (s *InstrumentedStore) MarkMessageProcessed(ctx context.Context, tenant Tenant, key string, at time.Time) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.mark_message_processed")
	defer span.End()

	err := s.next.MarkMessageProcessed(ctx, tenant, key, at)
	s.recordOperation(ctx, "runtime.mark_message_processed", err)
	return err
}

func (s *InstrumentedStore) GetSchedulerConfig(ctx context.Context) (SchedulerConfig, error) {
	ctx, span := s.scope.Start(ctx, "store.scanning.get_scheduler_config")
	defer span.End()

	cfg, err := s.next.GetSchedulerConfig(ctx)
	s.recordOperation(ctx, "scanning.get_scheduler_config", err)
	return cfg, err
}

func (s *InstrumentedStore) PatchSchedulerConfig(ctx context.Context, patch SchedulerConfigPatch) (SchedulerConfig, error) {
	ctx, span := s.scope.Start(ctx, "store.scanning.patch_scheduler_config")
	defer span.End()

	cfg, err := s.next.PatchSchedulerConfig(ctx, patch)
	s.recordOperation(ctx, "scanning.patch_scheduler_config", err)
	return cfg, err
}

func (s *InstrumentedStore) EnsureScanningStateForTenant(ctx context.Context, tenant Tenant) error {
	ctx, span := s.scope.Start(ctx, "store.scanning.ensure_tenant_state")
	defer span.End()

	err := s.next.EnsureScanningStateForTenant(ctx, tenant)
	s.recordOperation(ctx, "scanning.ensure_tenant_state", err)
	return err
}

func (s *InstrumentedStore) GetScanningState(ctx context.Context, tenant Tenant) (TenantScanningState, error) {
	ctx, span := s.scope.Start(ctx, "store.scanning.get_state")
	defer span.End()

	state, err := s.next.GetScanningState(ctx, tenant)
	s.recordOperation(ctx, "scanning.get_state", err)
	return state, err
}

func (s *InstrumentedStore) ListRunnableScanningStates(ctx context.Context) ([]TenantScanningState, error) {
	ctx, span := s.scope.Start(ctx, "store.scanning.list_runnable_states")
	defer span.End()

	states, err := s.next.ListRunnableScanningStates(ctx)
	s.recordOperation(ctx, "scanning.list_runnable_states", err)
	return states, err
}

func (s *InstrumentedStore) ListScanningStates(ctx context.Context) ([]TenantScanningState, error) {
	ctx, span := s.scope.Start(ctx, "store.scanning.list_states")
	defer span.End()

	states, err := s.next.ListScanningStates(ctx)
	s.recordOperation(ctx, "scanning.list_states", err)
	return states, err
}

func (s *InstrumentedStore) SetActiveScanningReader(ctx context.Context, tenant Tenant, reader string) error {
	ctx, span := s.scope.Start(ctx, "store.scanning.set_active_reader")
	defer span.End()

	err := s.next.SetActiveScanningReader(ctx, tenant, reader)
	s.recordOperation(ctx, "scanning.set_active_reader", err)
	return err
}

func (s *InstrumentedStore) ClearActiveScanningReader(ctx context.Context, tenant Tenant) error {
	ctx, span := s.scope.Start(ctx, "store.scanning.clear_active_reader")
	defer span.End()

	err := s.next.ClearActiveScanningReader(ctx, tenant)
	s.recordOperation(ctx, "scanning.clear_active_reader", err)
	return err
}

func (s *InstrumentedStore) SetScanningEnabled(ctx context.Context, tenant Tenant, enabled bool) error {
	ctx, span := s.scope.Start(ctx, "store.scanning.set_enabled")
	defer span.End()

	err := s.next.SetScanningEnabled(ctx, tenant, enabled)
	s.recordOperation(ctx, "scanning.set_enabled", err)
	return err
}

func (s *InstrumentedStore) UpdateScanningState(ctx context.Context, tenant Tenant, update ScanningStateUpdate) error {
	ctx, span := s.scope.Start(ctx, "store.scanning.update_state")
	defer span.End()

	err := s.next.UpdateScanningState(ctx, tenant, update)
	s.recordOperation(ctx, "scanning.update_state", err)
	return err
}

func (s *InstrumentedStore) SetReaderSecret(ctx context.Context, tenant Tenant, reader string, secret []byte) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.set_reader_secret")
	defer span.End()

	err := s.next.SetReaderSecret(ctx, tenant, reader, secret)
	s.recordOperation(ctx, "runtime.set_reader_secret", err)
	return err
}

func (s *InstrumentedStore) GetReaderSecret(ctx context.Context, tenant Tenant, reader string) (secret []byte, found bool, err error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.get_reader_secret")
	defer span.End()

	secret, found, err = s.next.GetReaderSecret(ctx, tenant, reader)
	s.recordOperation(ctx, "runtime.get_reader_secret", err)
	return secret, found, err
}

func (s *InstrumentedStore) SetReaderToken(ctx context.Context, tenant Tenant, reader string, token []byte) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.set_reader_token")
	defer span.End()

	err := s.next.SetReaderToken(ctx, tenant, reader, token)
	s.recordOperation(ctx, "runtime.set_reader_token", err)
	return err
}

func (s *InstrumentedStore) GetReaderToken(ctx context.Context, tenant Tenant, reader string) (token []byte, found bool, err error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.get_reader_token")
	defer span.End()

	token, found, err = s.next.GetReaderToken(ctx, tenant, reader)
	s.recordOperation(ctx, "runtime.get_reader_token", err)
	return token, found, err
}

func (s *InstrumentedStore) DeleteReaderToken(ctx context.Context, tenant Tenant, reader string) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.delete_reader_token")
	defer span.End()

	err := s.next.DeleteReaderToken(ctx, tenant, reader)
	s.recordOperation(ctx, "runtime.delete_reader_token", err)
	return err
}

func (s *InstrumentedStore) SetReaderConfig(ctx context.Context, tenant Tenant, reader string, config json.RawMessage) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.set_reader_config")
	defer span.End()

	err := s.next.SetReaderConfig(ctx, tenant, reader, config)
	s.recordOperation(ctx, "runtime.set_reader_config", err)
	return err
}

func (s *InstrumentedStore) GetReaderConfig(ctx context.Context, tenant Tenant, reader string) (json.RawMessage, bool, error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.get_reader_config")
	defer span.End()

	config, found, err := s.next.GetReaderConfig(ctx, tenant, reader)
	s.recordOperation(ctx, "runtime.get_reader_config", err)
	return config, found, err
}

func (s *InstrumentedStore) SetLLMProviderConfig(ctx context.Context, tenant Tenant, provider string, config json.RawMessage) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.set_llm_provider_config")
	defer span.End()

	err := s.next.SetLLMProviderConfig(ctx, tenant, provider, config)
	s.recordOperation(ctx, "runtime.set_llm_provider_config", err)
	return err
}

func (s *InstrumentedStore) GetLLMProviderConfig(ctx context.Context, tenant Tenant, provider string) (json.RawMessage, bool, error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.get_llm_provider_config")
	defer span.End()

	config, found, err := s.next.GetLLMProviderConfig(ctx, tenant, provider)
	s.recordOperation(ctx, "runtime.get_llm_provider_config", err)
	return config, found, err
}

func (s *InstrumentedStore) SetLLMProviderCredentials(ctx context.Context, tenant Tenant, provider string, credentials []byte) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.set_llm_provider_credentials")
	defer span.End()

	err := s.next.SetLLMProviderCredentials(ctx, tenant, provider, credentials)
	s.recordOperation(ctx, "runtime.set_llm_provider_credentials", err)
	return err
}

func (s *InstrumentedStore) GetLLMProviderCredentials(
	ctx context.Context,
	tenant Tenant,
	provider string,
) (credentials []byte, found bool, err error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.get_llm_provider_credentials")
	defer span.End()

	credentials, found, err = s.next.GetLLMProviderCredentials(ctx, tenant, provider)
	s.recordOperation(ctx, "runtime.get_llm_provider_credentials", err)
	return credentials, found, err
}

func (s *InstrumentedStore) DeleteLLMProviderRuntime(ctx context.Context, tenant Tenant, provider string) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.delete_llm_provider_runtime")
	defer span.End()

	err := s.next.DeleteLLMProviderRuntime(ctx, tenant, provider)
	s.recordOperation(ctx, "runtime.delete_llm_provider_runtime", err)
	return err
}

func (s *InstrumentedStore) SetActiveLLMProvider(ctx context.Context, tenant Tenant, provider string) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.set_active_llm_provider")
	defer span.End()

	err := s.next.SetActiveLLMProvider(ctx, tenant, provider)
	s.recordOperation(ctx, "runtime.set_active_llm_provider", err)
	return err
}

func (s *InstrumentedStore) ClearActiveLLMProvider(ctx context.Context, tenant Tenant) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.clear_active_llm_provider")
	defer span.End()

	err := s.next.ClearActiveLLMProvider(ctx, tenant)
	s.recordOperation(ctx, "runtime.clear_active_llm_provider", err)
	return err
}

func (s *InstrumentedStore) GetActiveLLMProviderRuntime(ctx context.Context, tenant Tenant) (LLMProviderRuntime, bool, error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.get_active_llm_provider")
	defer span.End()

	runtime, found, err := s.next.GetActiveLLMProviderRuntime(ctx, tenant)
	s.recordOperation(ctx, "runtime.get_active_llm_provider", err)
	return runtime, found, err
}

func (s *InstrumentedStore) DeleteReaderRuntime(ctx context.Context, tenant Tenant, reader string) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.delete_reader_runtime")
	defer span.End()

	err := s.next.DeleteReaderRuntime(ctx, tenant, reader)
	s.recordOperation(ctx, "runtime.delete_reader_runtime", err)
	return err
}

func (s *InstrumentedStore) GetCommunityURL(ctx context.Context) (string, error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.get_community_url")
	defer span.End()

	url, err := s.next.GetCommunityURL(ctx)
	s.recordOperation(ctx, "runtime.get_community_url", err)
	return url, err
}

func (s *InstrumentedStore) SetCommunityURL(ctx context.Context, url string) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.set_community_url")
	defer span.End()

	err := s.next.SetCommunityURL(ctx, url)
	s.recordOperation(ctx, "runtime.set_community_url", err)
	return err
}

func (s *InstrumentedStore) GetFacets(ctx context.Context, tenant Tenant) (*Facets, error) {
	ctx, span := s.scope.Start(ctx, "store.transactions.get_facets")
	defer span.End()

	facets, err := s.next.GetFacets(ctx, tenant)
	s.recordOperation(ctx, "transactions.get_facets", err)
	return facets, err
}

func (s *InstrumentedStore) ListLabels(ctx context.Context, tenant Tenant) ([]Label, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.list_labels")
	defer span.End()

	labels, err := s.next.ListLabels(ctx, tenant)
	s.recordOperation(ctx, "taxonomy.list_labels", err)
	return labels, err
}

func (s *InstrumentedStore) CreateLabel(ctx context.Context, tenant Tenant, name, color string) error {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.create_label")
	defer span.End()

	err := s.next.CreateLabel(ctx, tenant, name, color)
	s.recordOperation(ctx, "taxonomy.create_label", err)
	return err
}

func (s *InstrumentedStore) UpdateLabel(ctx context.Context, tenant Tenant, name, color string) error {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.update_label")
	defer span.End()

	err := s.next.UpdateLabel(ctx, tenant, name, color)
	s.recordOperation(ctx, "taxonomy.update_label", err)
	return err
}

func (s *InstrumentedStore) DeleteLabel(ctx context.Context, tenant Tenant, name string, removeFromTransactions bool) error {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.delete_label")
	defer span.End()

	err := s.next.DeleteLabel(ctx, tenant, name, removeFromTransactions)
	s.recordOperation(ctx, "taxonomy.delete_label", err)
	return err
}

func (s *InstrumentedStore) ApplyLabelByMerchant(ctx context.Context, tenant Tenant, label, pattern string) (int64, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.apply_label_by_merchant")
	defer span.End()

	affected, err := s.next.ApplyLabelByMerchant(ctx, tenant, label, pattern)
	s.recordOperation(ctx, "taxonomy.apply_label_by_merchant", err)
	return affected, err
}

func (s *InstrumentedStore) RemoveLabelByMerchant(ctx context.Context, tenant Tenant, label, pattern string) (int64, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.remove_label_by_merchant")
	defer span.End()

	removed, err := s.next.RemoveLabelByMerchant(ctx, tenant, label, pattern)
	s.recordOperation(ctx, "taxonomy.remove_label_by_merchant", err)
	return removed, err
}

func (s *InstrumentedStore) GetLabelMappings(ctx context.Context, tenant Tenant) (map[string][]string, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.get_label_mappings")
	defer span.End()

	mappings, err := s.next.GetLabelMappings(ctx, tenant)
	s.recordOperation(ctx, "taxonomy.get_label_mappings", err)
	return mappings, err
}

func (s *InstrumentedStore) GetMonthlyBreakdownSpend(ctx context.Context, tenant Tenant, dimension string, months int) (*MonthlyBreakdownData, error) {
	ctx, span := s.scope.Start(ctx, "store.read_model.get_monthly_breakdown_spend")
	defer span.End()

	data, err := s.next.GetMonthlyBreakdownSpend(ctx, tenant, dimension, months)
	s.recordOperation(ctx, "read_model.get_monthly_breakdown_spend", err)
	return data, err
}

func (s *InstrumentedStore) ListCategories(ctx context.Context, tenant Tenant) ([]Category, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.list_categories")
	defer span.End()

	categories, err := s.next.ListCategories(ctx, tenant)
	s.recordOperation(ctx, "taxonomy.list_categories", err)
	return categories, err
}

func (s *InstrumentedStore) CreateCategory(ctx context.Context, tenant Tenant, name, description string) error {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.create_category")
	defer span.End()

	err := s.next.CreateCategory(ctx, tenant, name, description)
	s.recordOperation(ctx, "taxonomy.create_category", err)
	return err
}

func (s *InstrumentedStore) DeleteCategory(ctx context.Context, tenant Tenant, name string, removeFromTransactions bool) error {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.delete_category")
	defer span.End()

	err := s.next.DeleteCategory(ctx, tenant, name, removeFromTransactions)
	s.recordOperation(ctx, "taxonomy.delete_category", err)
	return err
}

func (s *InstrumentedStore) ApplyCategoryByMerchant(ctx context.Context, tenant Tenant, category, pattern string) (int64, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.apply_category_by_merchant")
	defer span.End()

	affected, err := s.next.ApplyCategoryByMerchant(ctx, tenant, category, pattern)
	s.recordOperation(ctx, "taxonomy.apply_category_by_merchant", err)
	return affected, err
}

func (s *InstrumentedStore) RemoveCategoryByMerchant(ctx context.Context, tenant Tenant, category, pattern string) (int64, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.remove_category_by_merchant")
	defer span.End()

	removed, err := s.next.RemoveCategoryByMerchant(ctx, tenant, category, pattern)
	s.recordOperation(ctx, "taxonomy.remove_category_by_merchant", err)
	return removed, err
}

func (s *InstrumentedStore) GetCategoryMappings(ctx context.Context, tenant Tenant) (map[string][]string, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.get_category_mappings")
	defer span.End()

	mappings, err := s.next.GetCategoryMappings(ctx, tenant)
	s.recordOperation(ctx, "taxonomy.get_category_mappings", err)
	return mappings, err
}

func (s *InstrumentedStore) ListBuckets(ctx context.Context, tenant Tenant) ([]Bucket, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.list_buckets")
	defer span.End()

	buckets, err := s.next.ListBuckets(ctx, tenant)
	s.recordOperation(ctx, "taxonomy.list_buckets", err)
	return buckets, err
}

func (s *InstrumentedStore) CreateBucket(ctx context.Context, tenant Tenant, name, description string) error {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.create_bucket")
	defer span.End()

	err := s.next.CreateBucket(ctx, tenant, name, description)
	s.recordOperation(ctx, "taxonomy.create_bucket", err)
	return err
}

func (s *InstrumentedStore) DeleteBucket(ctx context.Context, tenant Tenant, name string, removeFromTransactions bool) error {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.delete_bucket")
	defer span.End()

	err := s.next.DeleteBucket(ctx, tenant, name, removeFromTransactions)
	s.recordOperation(ctx, "taxonomy.delete_bucket", err)
	return err
}

func (s *InstrumentedStore) ApplyBucketByMerchant(ctx context.Context, tenant Tenant, bucket, pattern string) (int64, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.apply_bucket_by_merchant")
	defer span.End()

	affected, err := s.next.ApplyBucketByMerchant(ctx, tenant, bucket, pattern)
	s.recordOperation(ctx, "taxonomy.apply_bucket_by_merchant", err)
	return affected, err
}

func (s *InstrumentedStore) RemoveBucketByMerchant(ctx context.Context, tenant Tenant, bucket, pattern string) (int64, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.remove_bucket_by_merchant")
	defer span.End()

	removed, err := s.next.RemoveBucketByMerchant(ctx, tenant, bucket, pattern)
	s.recordOperation(ctx, "taxonomy.remove_bucket_by_merchant", err)
	return removed, err
}

func (s *InstrumentedStore) GetBucketMappings(ctx context.Context, tenant Tenant) (map[string][]string, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.get_bucket_mappings")
	defer span.End()

	mappings, err := s.next.GetBucketMappings(ctx, tenant)
	s.recordOperation(ctx, "taxonomy.get_bucket_mappings", err)
	return mappings, err
}

func (s *InstrumentedStore) UpdateTransaction(ctx context.Context, tenant Tenant, id string, u TransactionUpdate) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.update")
	defer span.End()

	err := s.next.UpdateTransaction(ctx, tenant, id, u)
	s.recordOperation(ctx, "transactions.update", err)
	return err
}

func (s *InstrumentedStore) MuteTransaction(ctx context.Context, tenant Tenant, id string, muted bool, reason string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.mute")
	defer span.End()

	err := s.next.MuteTransaction(ctx, tenant, id, muted, reason)
	s.recordOperation(ctx, "transactions.mute", err)
	return err
}

func (s *InstrumentedStore) UpdateMuteReason(ctx context.Context, tenant Tenant, id, reason string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.update_mute_reason")
	defer span.End()

	err := s.next.UpdateMuteReason(ctx, tenant, id, reason)
	s.recordOperation(ctx, "transactions.update_mute_reason", err)
	return err
}

func (s *InstrumentedStore) UpdateMerchantReason(ctx context.Context, tenant Tenant, id, reason string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.update_merchant_reason")
	defer span.End()

	err := s.next.UpdateMerchantReason(ctx, tenant, id, reason)
	s.recordOperation(ctx, "transactions.update_merchant_reason", err)
	return err
}

func (s *InstrumentedStore) MuteByMerchant(ctx context.Context, tenant Tenant, pattern, reason string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.mute_by_merchant")
	defer span.End()

	err := s.next.MuteByMerchant(ctx, tenant, pattern, reason)
	s.recordOperation(ctx, "transactions.mute_by_merchant", err)
	return err
}

func (s *InstrumentedStore) ListMutedMerchants(ctx context.Context, tenant Tenant) ([]MutedMerchant, error) {
	ctx, span := s.scope.Start(ctx, "store.transactions.list_muted_merchants")
	defer span.End()

	merchants, err := s.next.ListMutedMerchants(ctx, tenant)
	s.recordOperation(ctx, "transactions.list_muted_merchants", err)
	return merchants, err
}

func (s *InstrumentedStore) GetMutedMerchantsWithCount(ctx context.Context, tenant Tenant) ([]MutedMerchantWithCount, error) {
	ctx, span := s.scope.Start(ctx, "store.transactions.get_muted_merchants_with_count")
	defer span.End()

	merchants, err := s.next.GetMutedMerchantsWithCount(ctx, tenant)
	s.recordOperation(ctx, "transactions.get_muted_merchants_with_count", err)
	return merchants, err
}

func (s *InstrumentedStore) DeleteMutedMerchant(ctx context.Context, tenant Tenant, id string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.delete_muted_merchant")
	defer span.End()

	err := s.next.DeleteMutedMerchant(ctx, tenant, id)
	s.recordOperation(ctx, "transactions.delete_muted_merchant", err)
	return err
}

func (s *InstrumentedStore) UnmuteByPattern(ctx context.Context, tenant Tenant, pattern string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.unmute_by_pattern")
	defer span.End()

	err := s.next.UnmuteByPattern(ctx, tenant, pattern)
	s.recordOperation(ctx, "transactions.unmute_by_pattern", err)
	return err
}

func (s *InstrumentedStore) DeleteMutedMerchantAndUnmute(ctx context.Context, tenant Tenant, id string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.delete_muted_merchant_and_unmute")
	defer span.End()

	err := s.next.DeleteMutedMerchantAndUnmute(ctx, tenant, id)
	s.recordOperation(ctx, "transactions.delete_muted_merchant_and_unmute", err)
	return err
}

func (s *InstrumentedStore) GetMutedMerchantPatterns(ctx context.Context, tenant Tenant) ([]string, error) {
	ctx, span := s.scope.Start(ctx, "store.transactions.get_muted_merchant_patterns")
	defer span.End()

	patterns, err := s.next.GetMutedMerchantPatterns(ctx, tenant)
	s.recordOperation(ctx, "transactions.get_muted_merchant_patterns", err)
	return patterns, err
}

func (s *InstrumentedStore) CategorizeMerchant(ctx context.Context, tenant Tenant, merchant, category, bucket string) (int64, error) {
	ctx, span := s.scope.Start(ctx, "store.community.categorize_merchant")
	defer span.End()

	updated, err := s.next.CategorizeMerchant(ctx, tenant, merchant, category, bucket)
	s.recordOperation(ctx, "community.categorize_merchant", err)
	return updated, err
}

func (s *InstrumentedStore) ListRules(ctx context.Context, tenant Tenant) ([]RuleRow, error) {
	ctx, span := s.scope.Start(ctx, "store.rules.list")
	defer span.End()

	rules, err := s.next.ListRules(ctx, tenant)
	s.recordOperation(ctx, "rules.list", err)
	return rules, err
}

func (s *InstrumentedStore) GetRule(ctx context.Context, tenant Tenant, id string) (*RuleRow, error) {
	ctx, span := s.scope.Start(ctx, "store.rules.get")
	defer span.End()

	rule, err := s.next.GetRule(ctx, tenant, id)
	s.recordOperation(ctx, "rules.get", err)
	return rule, err
}

func (s *InstrumentedStore) CreateRule(ctx context.Context, tenant Tenant, r RuleRow) (*RuleRow, error) {
	ctx, span := s.scope.Start(ctx, "store.rules.create")
	defer span.End()

	rule, err := s.next.CreateRule(ctx, tenant, r)
	s.recordOperation(ctx, "rules.create", err)
	return rule, err
}

func (s *InstrumentedStore) UpdateRule(ctx context.Context, tenant Tenant, id string, r RuleRow) (*RuleRow, error) {
	ctx, span := s.scope.Start(ctx, "store.rules.update")
	defer span.End()

	rule, err := s.next.UpdateRule(ctx, tenant, id, r)
	s.recordOperation(ctx, "rules.update", err)
	return rule, err
}

func (s *InstrumentedStore) DeleteRule(ctx context.Context, tenant Tenant, id string) error {
	ctx, span := s.scope.Start(ctx, "store.rules.delete")
	defer span.End()

	err := s.next.DeleteRule(ctx, tenant, id)
	s.recordOperation(ctx, "rules.delete", err)
	return err
}

func (s *InstrumentedStore) SeedPredefinedRules(ctx context.Context, rules []RuleRow) error {
	ctx, span := s.scope.Start(ctx, "store.rules.seed_predefined")
	defer span.End()

	err := s.next.SeedPredefinedRules(ctx, rules)
	s.recordOperation(ctx, "rules.seed_predefined", err)
	return err
}

func (s *InstrumentedStore) ImportUserRules(ctx context.Context, tenant Tenant, rules []RuleRow) error {
	ctx, span := s.scope.Start(ctx, "store.rules.import_user")
	defer span.End()

	err := s.next.ImportUserRules(ctx, tenant, rules)
	s.recordOperation(ctx, "rules.import_user", err)
	return err
}

func (s *InstrumentedStore) SeedMCCCodes(ctx context.Context, entries []MCCEntry) error {
	ctx, span := s.scope.Start(ctx, "store.community.seed_mcc_codes")
	defer span.End()

	err := s.next.SeedMCCCodes(ctx, entries)
	s.recordOperation(ctx, "community.seed_mcc_codes", err)
	return err
}

func (s *InstrumentedStore) SeedMerchantCategories(ctx context.Context, entries []MerchantCategoryEntry) (int64, error) {
	ctx, span := s.scope.Start(ctx, "store.community.seed_merchant_categories")
	defer span.End()

	updated, err := s.next.SeedMerchantCategories(ctx, entries)
	s.recordOperation(ctx, "community.seed_merchant_categories", err)
	return updated, err
}

func (s *InstrumentedStore) LoadCategorySnapshot(ctx context.Context) (api.CategoryResolver, error) {
	ctx, span := s.scope.Start(ctx, "store.community.load_category_snapshot")
	defer span.End()

	resolver, err := s.next.LoadCategorySnapshot(ctx)
	s.recordOperation(ctx, "community.load_category_snapshot", err)
	return resolver, err
}

func (s *InstrumentedStore) SeedMCCCategories(ctx context.Context, names []string) error {
	ctx, span := s.scope.Start(ctx, "store.community.seed_mcc_categories")
	defer span.End()

	err := s.next.SeedMCCCategories(ctx, names)
	s.recordOperation(ctx, "community.seed_mcc_categories", err)
	return err
}

func (s *InstrumentedStore) GetSyncStatus(ctx context.Context) (SyncStatus, error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.get_sync_status")
	defer span.End()

	status, err := s.next.GetSyncStatus(ctx)
	s.recordOperation(ctx, "runtime.get_sync_status", err)
	return status, err
}

func (s *InstrumentedStore) SetSyncStatus(ctx context.Context, status SyncStatus) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.set_sync_status")
	defer span.End()

	err := s.next.SetSyncStatus(ctx, status)
	s.recordOperation(ctx, "runtime.set_sync_status", err)
	return err
}

func (s *InstrumentedStore) GetCommunitySyncSettings(ctx context.Context) (CommunitySyncSettings, error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.get_community_sync_settings")
	defer span.End()

	settings, err := s.next.GetCommunitySyncSettings(ctx)
	s.recordOperation(ctx, "runtime.get_community_sync_settings", err)
	return settings, err
}

func (s *InstrumentedStore) PatchCommunitySyncSettings(
	ctx context.Context,
	patch CommunitySyncSettingsPatch,
) (CommunitySyncSettings, error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.patch_community_sync_settings")
	defer span.End()

	settings, err := s.next.PatchCommunitySyncSettings(ctx, patch)
	s.recordOperation(ctx, "runtime.patch_community_sync_settings", err)
	return settings, err
}

func (s *InstrumentedStore) ListExtractionDiagnostics(ctx context.Context, tenant Tenant, filter DiagnosticFilter) ([]ExtractionDiagnosticRow, error) {
	ctx, span := s.scope.Start(ctx, "store.diagnostics.list_extraction")
	defer span.End()

	rows, err := s.next.ListExtractionDiagnostics(ctx, tenant, filter)
	s.recordOperation(ctx, "diagnostics.list_extraction", err)
	return rows, err
}

func (s *InstrumentedStore) GetExtractionDiagnostic(ctx context.Context, tenant Tenant, id string) (*ExtractionDiagnosticRow, error) {
	ctx, span := s.scope.Start(ctx, "store.diagnostics.get_extraction")
	defer span.End()

	row, err := s.next.GetExtractionDiagnostic(ctx, tenant, id)
	s.recordOperation(ctx, "diagnostics.get_extraction", err)
	return row, err
}

func (s *InstrumentedStore) UpdateExtractionDiagnosticStatus(ctx context.Context, tenant Tenant, id, status string) (*ExtractionDiagnosticRow, error) {
	ctx, span := s.scope.Start(ctx, "store.diagnostics.update_extraction_status")
	defer span.End()

	row, err := s.next.UpdateExtractionDiagnosticStatus(ctx, tenant, id, status)
	s.recordOperation(ctx, "diagnostics.update_extraction_status", err)
	return row, err
}

func (s *InstrumentedStore) RecordExtractionDiagnostic(ctx context.Context, diagnostic api.ExtractionDiagnostic) error {
	ctx, span := s.scope.Start(ctx, "store.diagnostics.record_extraction")
	defer span.End()

	err := s.next.RecordExtractionDiagnostic(ctx, diagnostic)
	s.recordOperation(ctx, "diagnostics.record_extraction", err)
	return err
}

func (s *InstrumentedStore) RecordTenantExtractionDiagnostic(ctx context.Context, tenant Tenant, diagnostic api.ExtractionDiagnostic) error {
	ctx, span := s.scope.Start(ctx, "store.diagnostics.record_tenant_extraction")
	defer span.End()

	err := s.next.RecordTenantExtractionDiagnostic(ctx, tenant, diagnostic)
	s.recordOperation(ctx, "diagnostics.record_tenant_extraction", err)
	return err
}
