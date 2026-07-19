package httpapi

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/assistant"
	"github.com/ArionMiles/expensor/backend/internal/daemon"
	"github.com/ArionMiles/expensor/backend/internal/llm"
	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

// Shared HTTP handler test fixtures.

const (
	testTransactionID   = "11111111-1111-1111-1111-111111111111"
	testRuleID          = "22222222-2222-2222-2222-222222222222"
	testMutedMerchantID = "33333333-3333-3333-3333-333333333333"
)

var (
	errStoreNotFound                = errors.E(errors.NotFound, "not found")
	errStoreConflict                = errors.E(errors.Conflict, "conflict")
	errStoreAccessTokenNameConflict = errors.E(errors.Conflict, errors.User("Token test already exists."), "access token name conflict")
	errStoreUserEmailConflict       = errors.E(errors.Conflict, errors.User("User b@example.com already exists."), "user email conflict")
	errStoreRuleNameConflict        = errors.E(errors.Conflict, errors.User("rule name already exists"), "rule name conflict")
	errStoreDiagnosticConflict      = errors.E(errors.Conflict, errors.User("open extraction diagnostic already exists"), "diagnostic conflict")
)

type mockDaemon struct {
	status    DaemonStatus
	startFn   func(daemon.RunRequest)
	stopFn    func()
	rescanFn  func(daemon.RunRequest)
	restartFn func(daemon.RunRequest)
}

func (m *mockDaemon) Start(request daemon.RunRequest) {
	if m.startFn != nil {
		m.startFn(request)
	}
}

func (m *mockDaemon) Stop() {
	if m.stopFn != nil {
		m.stopFn()
	}
}

func (m *mockDaemon) Rescan(request daemon.RunRequest) {
	if m.rescanFn != nil {
		m.rescanFn(request)
	}
}

func (m *mockDaemon) Restart(request daemon.RunRequest) {
	if m.restartFn != nil {
		m.restartFn(request)
	}
}

func (m *mockDaemon) Status() daemon.Status {
	return daemon.Status{Running: m.status.Running, StartedAt: m.status.StartedAt, LastError: m.status.LastError}
}

func mockStoreErr(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.WhatKind(err) != errors.Unknown {
		return errors.E(op, err)
	}
	return err
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// mockStore implements the HTTP handler persistence surface for unit tests.
type mockStore struct {
	transactions               []store.Transaction
	listResult                 store.TransactionListResult
	listErr                    error
	listFilter                 store.ListFilter
	listCalls                  int
	getResult                  *store.Transaction
	getErr                     error
	updateErr                  error
	updatedTransaction         store.TransactionUpdate
	muteTransactionID          string
	muteTransactionValue       bool
	muteTransactionReason      string
	updateMuteReasonID         string
	updateMuteReasonValue      string
	updateMerchantID           string
	updateMerchantReason       string
	addLabelsErr               error
	removeLblErr               error
	searchResult               []store.Transaction
	searchListResult           store.TransactionListResult
	searchErr                  error
	searchFilter               store.ListFilter
	searchCalls                int
	stats                      *store.Stats
	statsErr                   error
	dashboardData              *store.DashboardData
	dashboardErr               error
	appConfig                  map[string]string
	appConfigByTenant          map[string]map[string]string
	setConfigErr               error
	schedulerConfig            store.SchedulerConfig
	scanningState              store.TenantScanningState
	scanningStates             []store.TenantScanningState
	readerSecrets              map[string][]byte
	readerTokens               map[string][]byte
	readerConfigs              map[string]json.RawMessage
	llmProviderConfigs         map[string]json.RawMessage
	llmProviderCredentials     map[string][]byte
	activeLLMProvider          string
	getFacetsErr               error
	facets                     *store.Facets
	labels                     []store.Label
	labelsErr                  error
	deleteLabelCleanup         bool
	categoryMappings           map[string][]string
	categories                 []store.Category
	catsErr                    error
	deleteCategoryCleanup      bool
	bucketMappings             map[string][]string
	buckets                    []store.Bucket
	bucketsErr                 error
	deleteBucketCleanup        bool
	updateTxErr                error
	rules                      []store.RuleRow
	rulesErr                   error
	ruleResult                 *store.RuleRow
	ruleErr                    error
	importedRules              []store.RuleRow
	importErr                  error
	heatmapData                *store.HeatmapData
	heatmapErr                 error
	annualData                 []store.DailyBucket
	annualErr                  error
	monthlyBreakdown           *store.MonthlyBreakdownData
	monthlyBreakdownErr        error
	categorizeMerchantN        int64
	diagnostics                []store.ExtractionDiagnosticRow
	diagnosticFilter           store.DiagnosticFilter
	diagnosticResult           *store.ExtractionDiagnosticRow
	diagnosticErr              error
	updateDiagnosticID         string
	updateDiagnosticStat       string
	syncStatus                 store.SyncStatus
	syncStatusErr              error
	communitySyncSettings      store.CommunitySyncSettings
	communitySyncSettingsPatch store.CommunitySyncSettingsPatch
	communitySyncSettingsErr   error
	bootstrapRequired          bool
	createdBootstrapAdmin      store.CreateBootstrapAdminInput
	createdUser                store.CreateUserInput
	createUserErr              error
	users                      []store.User
	updatedUserID              string
	updatedUser                store.UpdateUserInput
	updatedUserResult          *store.User
	updatedPasswordUserID      string
	updatedPasswordHash        string
	deletedUserID              string
	usersByEmail               map[string]*store.User
	usersByID                  map[string]*store.User
	createdSession             store.CreateSessionInput
	sessionsByHash             map[string]*store.Session
	revokedSessionID           string
	createdAccessToken         store.CreateAccessTokenInput
	createAccessTokenErr       error
	accessTokens               []store.AccessToken
	listedAccessTokensUserID   string
	accessTokensByHash         map[string]*store.AccessToken
	revokedAccessTokenID       string
	revokedAccessUserID        string
	createdSetupToken          store.CreateAccountSetupTokenInput
	setupTokensByHash          map[string]*store.AccountSetupToken
	usedSetupTokenID           string
	completedSetupTokenHash    string
	completedSetupPasswordHash string
	completedSetupDisplayName  string
	completedSetupAvatarKey    string
	completedSetupUser         *store.User
	lastAppConfigTenant        store.Tenant
}

func (m *mockStore) BootstrapRequired(_ context.Context) (bool, error) {
	return m.bootstrapRequired, nil
}

func (m *mockStore) CreateBootstrapAdmin(_ context.Context, input store.CreateBootstrapAdminInput) (*store.User, error) {
	m.createdBootstrapAdmin = input
	return &store.User{
		ID:           "admin-user-id",
		TenantID:     "admin-user-id",
		Email:        input.Email,
		DisplayName:  input.DisplayName,
		PasswordHash: input.PasswordHash,
		Role:         store.UserRoleAdmin,
		AvatarKey:    input.AvatarKey,
	}, nil
}

func (m *mockStore) CreateUser(_ context.Context, input store.CreateUserInput) (*store.User, error) {
	m.createdUser = input
	if m.createUserErr != nil {
		return nil, mockStoreErr("store.auth.create_user", m.createUserErr)
	}
	return &store.User{
		ID:           "user-id",
		TenantID:     "user-id",
		Email:        input.Email,
		DisplayName:  input.DisplayName,
		PasswordHash: input.PasswordHash,
		Role:         input.Role,
		AvatarKey:    input.AvatarKey,
	}, nil
}

func (m *mockStore) ListUsers(_ context.Context) ([]store.User, error) {
	return m.users, nil
}

func (m *mockStore) UpdateUser(_ context.Context, id string, input store.UpdateUserInput) (*store.User, error) {
	m.updatedUserID = id
	m.updatedUser = input
	if m.updatedUserResult != nil {
		return m.updatedUserResult, nil
	}
	if m.usersByID != nil {
		if user, ok := m.usersByID[id]; ok {
			return user, nil
		}
	}
	return &store.User{
		ID:          id,
		TenantID:    id,
		Email:       "updated@example.com",
		DisplayName: "Updated",
		Role:        store.UserRoleUser,
		AvatarKey:   "default",
	}, nil
}

func (m *mockStore) UpdateUserPassword(_ context.Context, id string, input store.UpdateUserPasswordInput) error {
	m.updatedPasswordUserID = id
	m.updatedPasswordHash = input.PasswordHash
	if m.usersByID != nil {
		if user, ok := m.usersByID[id]; ok {
			user.PasswordHash = input.PasswordHash
			return nil
		}
	}
	return nil
}

func (m *mockStore) DeleteUser(_ context.Context, id string) error {
	m.deletedUserID = id
	if m.usersByID != nil {
		if _, ok := m.usersByID[id]; !ok {
			return mockStoreErr("store.auth.delete_user", errStoreNotFound)
		}
		delete(m.usersByID, id)
	}
	return nil
}

func (m *mockStore) FindUserByEmail(_ context.Context, email string) (*store.User, error) {
	if m.usersByEmail != nil {
		if user, ok := m.usersByEmail[strings.ToLower(email)]; ok {
			return user, nil
		}
	}
	return nil, mockStoreErr("store.auth.find_user_by_email", errStoreNotFound)
}

func (m *mockStore) FindUserByID(_ context.Context, id string) (*store.User, error) {
	if m.usersByID != nil {
		if user, ok := m.usersByID[id]; ok {
			return user, nil
		}
	}
	return nil, mockStoreErr("store.auth.find_user_by_id", errStoreNotFound)
}

func (m *mockStore) CreateSession(_ context.Context, input store.CreateSessionInput) (*store.Session, error) {
	m.createdSession = input
	return &store.Session{ID: "session-id", UserID: input.UserID, TokenHash: input.TokenHash, ExpiresAt: input.ExpiresAt}, nil
}

func (m *mockStore) FindSessionByHash(_ context.Context, tokenHash string) (*store.Session, error) {
	if m.sessionsByHash != nil {
		if session, ok := m.sessionsByHash[tokenHash]; ok {
			return session, nil
		}
	}
	return nil, mockStoreErr("store.auth.find_session_by_hash", errStoreNotFound)
}

func (m *mockStore) RevokeSession(_ context.Context, id string) error {
	m.revokedSessionID = id
	return nil
}

func (m *mockStore) CreateAccessToken(_ context.Context, input store.CreateAccessTokenInput) (*store.AccessToken, error) {
	m.createdAccessToken = input
	if m.createAccessTokenErr != nil {
		return nil, mockStoreErr("store.auth.create_access_token", m.createAccessTokenErr)
	}
	return &store.AccessToken{ID: "access-token-id", UserID: input.UserID, Name: input.Name, TokenHash: input.TokenHash, ExpiresAt: input.ExpiresAt}, nil
}

func (m *mockStore) ListAccessTokens(_ context.Context, userID string) ([]store.AccessToken, error) {
	m.listedAccessTokensUserID = userID
	return m.accessTokens, nil
}

func (m *mockStore) FindAccessTokenByHash(_ context.Context, tokenHash string) (*store.AccessToken, error) {
	if m.accessTokensByHash != nil {
		if token, ok := m.accessTokensByHash[tokenHash]; ok {
			return token, nil
		}
	}
	return nil, mockStoreErr("store.auth.find_access_token_by_hash", errStoreNotFound)
}

func (m *mockStore) RevokeAccessToken(_ context.Context, id, userID string) error {
	m.revokedAccessTokenID = id
	m.revokedAccessUserID = userID
	return nil
}

func (m *mockStore) CreateAccountSetupToken(_ context.Context, input store.CreateAccountSetupTokenInput) (*store.AccountSetupToken, error) {
	m.createdSetupToken = input
	return &store.AccountSetupToken{ID: "setup-token-id", UserID: input.UserID, TokenHash: input.TokenHash, ExpiresAt: input.ExpiresAt}, nil
}

func (m *mockStore) FindAccountSetupTokenByHash(_ context.Context, tokenHash string) (*store.AccountSetupToken, error) {
	if m.setupTokensByHash != nil {
		if token, ok := m.setupTokensByHash[tokenHash]; ok {
			return token, nil
		}
	}
	return nil, mockStoreErr("store.auth.find_account_setup_token_by_hash", errStoreNotFound)
}

func (m *mockStore) MarkAccountSetupTokenUsed(_ context.Context, id string) error {
	m.usedSetupTokenID = id
	return nil
}

func (m *mockStore) CompleteAccountSetup(_ context.Context, input store.CompleteAccountSetupInput) (*store.User, error) {
	m.completedSetupTokenHash = input.TokenHash
	m.completedSetupPasswordHash = input.PasswordHash
	m.completedSetupDisplayName = input.DisplayName
	m.completedSetupAvatarKey = input.AvatarKey
	if m.completedSetupUser != nil {
		return m.completedSetupUser, nil
	}
	return nil, mockStoreErr("store.auth.complete_account_setup", errStoreNotFound)
}

func (m *mockStore) ListTransactions(
	_ context.Context,
	_ store.Tenant,
	f store.ListFilter,
) ([]store.Transaction, store.TransactionListResult, error) {
	m.listCalls++
	m.listFilter = f
	if m.listErr != nil {
		return nil, store.TransactionListResult{}, m.listErr
	}
	return m.transactions, m.listResult, nil
}

func (m *mockStore) GetTransaction(_ context.Context, _ store.Tenant, _ string) (*store.Transaction, error) {
	return m.getResult, mockStoreErr("store.transactions.get", m.getErr)
}

func (m *mockStore) AddLabels(_ context.Context, _ store.Tenant, _ string, _ []string) error {
	return mockStoreErr("store.transactions.add_labels", m.addLabelsErr)
}

func (m *mockStore) RemoveLabel(_ context.Context, _ store.Tenant, _, _ string) error {
	return mockStoreErr("store.transactions.remove_label", m.removeLblErr)
}

func (m *mockStore) SearchTransactions(
	_ context.Context,
	_ store.Tenant,
	_ string,
	f store.ListFilter,
) ([]store.Transaction, store.TransactionListResult, error) {
	m.searchCalls++
	m.searchFilter = f
	if m.searchErr != nil {
		return nil, store.TransactionListResult{}, m.searchErr
	}
	return m.searchResult, m.searchListResult, nil
}

func (m *mockStore) GetStats(_ context.Context, _ store.Tenant, _ string) (*store.Stats, error) {
	return m.stats, m.statsErr
}

func (m *mockStore) GetChartData(_ context.Context, _ store.Tenant) (*store.ChartData, error) {
	return &store.ChartData{
		MonthlySpend: []store.TimeBucket{},
		DailySpend:   []store.TimeBucket{},
		ByCategory:   map[string]float64{},
		ByBucket:     map[string]float64{},
		ByLabel:      map[string]float64{},
		BySource:     map[string]float64{},
	}, nil
}

func (m *mockStore) GetDashboardData(_ context.Context, _ store.Tenant) (*store.DashboardData, error) {
	if m.dashboardErr != nil {
		return nil, m.dashboardErr
	}
	if m.dashboardData != nil {
		return m.dashboardData, nil
	}
	return &store.DashboardData{
		CurrentMonth: store.DashboardSection{
			Label: "April 2026",
			Stats: store.Stats{TotalCount: 1, TotalBase: 1000, BaseCurrency: "INR"},
			Charts: store.ChartData{
				MonthlySpend:      []store.TimeBucket{},
				DailySpend:        []store.TimeBucket{},
				ByCategory:        map[string]float64{},
				ByBucket:          map[string]float64{},
				ByLabel:           map[string]float64{},
				BySource:          map[string]float64{},
				ByCategoryMonthly: map[string]store.CategoryMonthlyEntry{},
			},
		},
		AllTime: store.DashboardSection{
			Label: "All Time",
			Stats: store.Stats{TotalCount: 2, TotalBase: 2000, BaseCurrency: "INR"},
			Charts: store.ChartData{
				MonthlySpend:      []store.TimeBucket{},
				DailySpend:        []store.TimeBucket{},
				ByCategory:        map[string]float64{},
				ByBucket:          map[string]float64{},
				ByLabel:           map[string]float64{},
				BySource:          map[string]float64{},
				ByCategoryMonthly: map[string]store.CategoryMonthlyEntry{},
			},
		},
	}, nil
}

func (m *mockStore) GetAppConfig(_ context.Context, tenant store.Tenant, key string) (string, error) {
	m.lastAppConfigTenant = tenant
	if m.appConfigByTenant != nil {
		if values, ok := m.appConfigByTenant[tenant.ID]; ok {
			if v, ok := values[key]; ok {
				return v, nil
			}
		}
	}
	if m.appConfig != nil {
		if v, ok := m.appConfig[key]; ok {
			return v, nil
		}
	}
	return "", stderrors.New("not found")
}

func (m *mockStore) SetAppConfig(_ context.Context, _ store.Tenant, key, value string) error {
	if m.setConfigErr != nil {
		return m.setConfigErr
	}
	if m.appConfig == nil {
		m.appConfig = make(map[string]string)
	}
	m.appConfig[key] = value
	return nil
}

func (m *mockStore) GetSchedulerConfig(context.Context) (store.SchedulerConfig, error) {
	if m.schedulerConfig.MaxConcurrentScans == 0 {
		m.schedulerConfig.MaxConcurrentScans = 4
	}
	return m.schedulerConfig, nil
}

func (m *mockStore) PatchSchedulerConfig(_ context.Context, patch store.SchedulerConfigPatch) (store.SchedulerConfig, error) {
	if patch.MaxConcurrentScans != nil {
		m.schedulerConfig.MaxConcurrentScans = *patch.MaxConcurrentScans
	}
	if m.schedulerConfig.MaxConcurrentScans == 0 {
		m.schedulerConfig.MaxConcurrentScans = 4
	}
	m.schedulerConfig.UpdatedAt = time.Now()
	return m.schedulerConfig, nil
}

func (m *mockStore) EnsureScanningStateForTenant(_ context.Context, tenant store.Tenant) error {
	if m.scanningState.TenantID == "" {
		m.scanningState = store.TenantScanningState{TenantID: tenant.ID, Enabled: true, State: store.ScanningStateStopped}
	}
	return nil
}

func (m *mockStore) GetScanningState(ctx context.Context, tenant store.Tenant) (store.TenantScanningState, error) {
	if err := m.EnsureScanningStateForTenant(ctx, tenant); err != nil {
		return store.TenantScanningState{}, err
	}
	return m.scanningState, nil
}

func (m *mockStore) ListScanningStates(context.Context) ([]store.TenantScanningState, error) {
	return append([]store.TenantScanningState(nil), m.scanningStates...), nil
}

func (m *mockStore) SetActiveScanningReader(_ context.Context, tenant store.Tenant, reader string) error {
	m.scanningState = store.TenantScanningState{TenantID: tenant.ID, ActiveReader: reader, Enabled: true, State: store.ScanningStateQueued}
	return nil
}

func (m *mockStore) ClearActiveScanningReader(_ context.Context, tenant store.Tenant) error {
	m.scanningState = store.TenantScanningState{TenantID: tenant.ID, Enabled: false, State: store.ScanningStateStopped}
	return nil
}

func (m *mockStore) SetScanningEnabled(_ context.Context, tenant store.Tenant, enabled bool) error {
	if m.scanningState.TenantID == "" {
		m.scanningState.TenantID = tenant.ID
	}
	m.scanningState.Enabled = enabled
	switch {
	case enabled && m.scanningState.ActiveReader != "":
		m.scanningState.State = store.ScanningStateQueued
	case enabled:
		m.scanningState.State = store.ScanningStateStopped
	default:
		m.scanningState.State = store.ScanningStatePaused
	}
	return nil
}

func (m *mockStore) UpdateScanningState(_ context.Context, tenant store.Tenant, update store.ScanningStateUpdate) error {
	m.scanningState.TenantID = tenant.ID
	m.scanningState.State = update.State
	m.scanningState.ReasonCode = update.ReasonCode
	m.scanningState.PublicMessage = update.PublicMessage
	if update.LastStartedAt != nil {
		m.scanningState.LastStartedAt = update.LastStartedAt
	}
	if update.LastStoppedAt != nil {
		m.scanningState.LastStoppedAt = update.LastStoppedAt
	}
	if update.LastFailedAt != nil {
		m.scanningState.LastFailedAt = update.LastFailedAt
	}
	m.scanningState.NextRetryAt = update.NextRetryAt
	if update.RetryCount != nil {
		m.scanningState.RetryCount = *update.RetryCount
	}
	return nil
}

func (m *mockStore) readerRuntimeKey(tenant store.Tenant, reader string) string {
	if tenant.ID == "" {
		panic("tenant id is required for reader runtime mock")
	}
	return tenant.ID + "/" + reader
}

func (m *mockStore) SetReaderSecret(_ context.Context, tenant store.Tenant, reader string, secret []byte) error {
	if m.readerSecrets == nil {
		m.readerSecrets = make(map[string][]byte)
	}
	m.readerSecrets[m.readerRuntimeKey(tenant, reader)] = append([]byte(nil), secret...)
	return nil
}

func (m *mockStore) GetReaderSecret(_ context.Context, tenant store.Tenant, reader string) (secret []byte, found bool, err error) {
	secret, ok := m.readerSecrets[m.readerRuntimeKey(tenant, reader)]
	return append([]byte(nil), secret...), ok, nil
}

func (m *mockStore) SetReaderToken(_ context.Context, tenant store.Tenant, reader string, token []byte) error {
	if m.readerTokens == nil {
		m.readerTokens = make(map[string][]byte)
	}
	m.readerTokens[m.readerRuntimeKey(tenant, reader)] = append([]byte(nil), token...)
	return nil
}

func (m *mockStore) GetReaderToken(_ context.Context, tenant store.Tenant, reader string) (token []byte, found bool, err error) {
	token, ok := m.readerTokens[m.readerRuntimeKey(tenant, reader)]
	return append([]byte(nil), token...), ok, nil
}

func (m *mockStore) DeleteReaderToken(_ context.Context, tenant store.Tenant, reader string) error {
	delete(m.readerTokens, m.readerRuntimeKey(tenant, reader))
	return nil
}

func (m *mockStore) SetReaderConfig(_ context.Context, tenant store.Tenant, reader string, readerConfig json.RawMessage) error {
	if m.readerConfigs == nil {
		m.readerConfigs = make(map[string]json.RawMessage)
	}
	m.readerConfigs[m.readerRuntimeKey(tenant, reader)] = append(json.RawMessage(nil), readerConfig...)
	return nil
}

func (m *mockStore) GetReaderConfig(_ context.Context, tenant store.Tenant, reader string) (json.RawMessage, bool, error) {
	cfg, ok := m.readerConfigs[m.readerRuntimeKey(tenant, reader)]
	return append(json.RawMessage(nil), cfg...), ok, nil
}

func (m *mockStore) DeleteReaderRuntime(_ context.Context, tenant store.Tenant, reader string) error {
	key := m.readerRuntimeKey(tenant, reader)
	delete(m.readerSecrets, key)
	delete(m.readerTokens, key)
	delete(m.readerConfigs, key)
	return nil
}

func (m *mockStore) llmProviderRuntimeKey(tenant store.Tenant, provider string) string {
	if tenant.ID == "" {
		panic("tenant id is required for llm provider runtime mock")
	}
	return tenant.ID + "/" + provider
}

func (m *mockStore) SetLLMProviderConfig(_ context.Context, tenant store.Tenant, provider string, config json.RawMessage) error {
	if m.llmProviderConfigs == nil {
		m.llmProviderConfigs = make(map[string]json.RawMessage)
	}
	m.llmProviderConfigs[m.llmProviderRuntimeKey(tenant, provider)] = append(json.RawMessage(nil), config...)
	return nil
}

func (m *mockStore) GetLLMProviderConfig(_ context.Context, tenant store.Tenant, provider string) (json.RawMessage, bool, error) {
	cfg, ok := m.llmProviderConfigs[m.llmProviderRuntimeKey(tenant, provider)]
	return append(json.RawMessage(nil), cfg...), ok, nil
}

func (m *mockStore) SetLLMProviderCredentials(_ context.Context, tenant store.Tenant, provider string, credentials []byte) error {
	if m.llmProviderCredentials == nil {
		m.llmProviderCredentials = make(map[string][]byte)
	}
	m.llmProviderCredentials[m.llmProviderRuntimeKey(tenant, provider)] = append([]byte(nil), credentials...)
	return nil
}

func (m *mockStore) GetLLMProviderCredentials(
	_ context.Context,
	tenant store.Tenant,
	provider string,
) (credentials []byte, found bool, err error) {
	credentials, ok := m.llmProviderCredentials[m.llmProviderRuntimeKey(tenant, provider)]
	return append([]byte(nil), credentials...), ok, nil
}

func (m *mockStore) DeleteLLMProviderRuntime(_ context.Context, tenant store.Tenant, provider string) error {
	key := m.llmProviderRuntimeKey(tenant, provider)
	delete(m.llmProviderConfigs, key)
	delete(m.llmProviderCredentials, key)
	if m.activeLLMProvider == provider {
		m.activeLLMProvider = ""
	}
	return nil
}

func (m *mockStore) SetActiveLLMProvider(_ context.Context, _ store.Tenant, provider string) error {
	m.activeLLMProvider = provider
	return nil
}

func (m *mockStore) ClearActiveLLMProvider(_ context.Context, _ store.Tenant) error {
	m.activeLLMProvider = ""
	return nil
}

func (m *mockStore) GetActiveLLMProviderRuntime(
	_ context.Context,
	tenant store.Tenant,
) (store.LLMProviderRuntime, bool, error) {
	if m.activeLLMProvider == "" {
		return store.LLMProviderRuntime{}, false, nil
	}
	key := m.llmProviderRuntimeKey(tenant, m.activeLLMProvider)
	config := append(json.RawMessage(nil), m.llmProviderConfigs[key]...)
	credentials := append([]byte(nil), m.llmProviderCredentials[key]...)
	return store.LLMProviderRuntime{
		Provider:       m.activeLLMProvider,
		Config:         config,
		Credentials:    credentials,
		HasCredentials: len(credentials) > 0,
		Active:         true,
	}, true, nil
}

func (m *mockStore) GetFacets(_ context.Context, _ store.Tenant) (*store.Facets, error) {
	if m.getFacetsErr != nil {
		return nil, mockStoreErr("store.transactions.facets", m.getFacetsErr)
	}
	if m.facets != nil {
		return m.facets, nil
	}
	return &store.Facets{
		Sources:     []string{},
		SourceTypes: []string{},
		Banks:       []string{},
		Categories:  []string{},
		Currencies:  []string{},
		Merchants:   []string{},
		Labels:      []string{},
		LabelCounts: map[string]int{},
		Buckets:     []string{},
	}, nil
}

func (m *mockStore) ListLabels(_ context.Context, _ store.Tenant) ([]store.Label, error) {
	if m.labelsErr != nil {
		return nil, mockStoreErr("store.taxonomy.labels", m.labelsErr)
	}
	if m.labels == nil {
		return []store.Label{}, nil
	}
	return m.labels, nil
}

func (m *mockStore) CreateLabel(_ context.Context, _ store.Tenant, _, _ string) error {
	return mockStoreErr("store.taxonomy.labels", m.labelsErr)
}

func (m *mockStore) UpdateLabel(_ context.Context, _ store.Tenant, _, _ string) error {
	return mockStoreErr("store.update", m.updateErr)
}

func (m *mockStore) DeleteLabel(_ context.Context, _ store.Tenant, _ string, removeFromTransactions bool) error {
	m.deleteLabelCleanup = removeFromTransactions
	return mockStoreErr("store.taxonomy.labels", m.labelsErr)
}

func (m *mockStore) RemoveLabelByMerchant(_ context.Context, _ store.Tenant, _, _ string) (int64, error) {
	return 0, nil
}

func (m *mockStore) ApplyLabelByMerchant(_ context.Context, _ store.Tenant, _, _ string) (int64, error) {
	if m.labelsErr != nil {
		return 0, m.labelsErr
	}
	return 0, nil
}

func (m *mockStore) GetLabelMappings(_ context.Context, _ store.Tenant) (map[string][]string, error) {
	return map[string][]string{}, nil
}

func (m *mockStore) GetMonthlyBreakdownSpend(_ context.Context, _ store.Tenant, _ string, _ int) (*store.MonthlyBreakdownData, error) {
	if m.monthlyBreakdownErr != nil {
		return nil, m.monthlyBreakdownErr
	}
	if m.monthlyBreakdown != nil {
		return m.monthlyBreakdown, nil
	}
	return &store.MonthlyBreakdownData{
		Labels: []string{},
		Months: []string{},
		Series: []store.MonthlyBreakdownSeries{},
	}, nil
}

func (m *mockStore) ListCategories(_ context.Context, _ store.Tenant) ([]store.Category, error) {
	if m.catsErr != nil {
		return nil, mockStoreErr("store.taxonomy.categories", m.catsErr)
	}
	if m.categories == nil {
		return []store.Category{}, nil
	}
	return m.categories, nil
}

func (m *mockStore) CreateCategory(_ context.Context, _ store.Tenant, _, _ string) error {
	return mockStoreErr("store.taxonomy.categories", m.catsErr)
}

func (m *mockStore) DeleteCategory(_ context.Context, _ store.Tenant, _ string, removeFromTransactions bool) error {
	m.deleteCategoryCleanup = removeFromTransactions
	return mockStoreErr("store.taxonomy.categories", m.catsErr)
}

func (m *mockStore) GetCategoryMappings(_ context.Context, _ store.Tenant) (map[string][]string, error) {
	if m.categoryMappings != nil {
		return m.categoryMappings, nil
	}
	return map[string][]string{}, nil
}

func (m *mockStore) ApplyCategoryByMerchant(_ context.Context, _ store.Tenant, _, _ string) (int64, error) {
	if m.catsErr != nil {
		return 0, m.catsErr
	}
	return 2, nil
}

func (m *mockStore) RemoveCategoryByMerchant(_ context.Context, _ store.Tenant, _, _ string) (int64, error) {
	if m.catsErr != nil {
		return 0, m.catsErr
	}
	return 1, nil
}

func (m *mockStore) ListBuckets(_ context.Context, _ store.Tenant) ([]store.Bucket, error) {
	if m.bucketsErr != nil {
		return nil, mockStoreErr("store.taxonomy.buckets", m.bucketsErr)
	}
	if m.buckets == nil {
		return []store.Bucket{}, nil
	}
	return m.buckets, nil
}

func (m *mockStore) CreateBucket(_ context.Context, _ store.Tenant, _, _ string) error {
	return mockStoreErr("store.taxonomy.buckets", m.bucketsErr)
}

func (m *mockStore) DeleteBucket(_ context.Context, _ store.Tenant, _ string, removeFromTransactions bool) error {
	m.deleteBucketCleanup = removeFromTransactions
	return mockStoreErr("store.taxonomy.buckets", m.bucketsErr)
}

func (m *mockStore) GetBucketMappings(_ context.Context, _ store.Tenant) (map[string][]string, error) {
	if m.bucketMappings != nil {
		return m.bucketMappings, nil
	}
	return map[string][]string{}, nil
}

func (m *mockStore) ApplyBucketByMerchant(_ context.Context, _ store.Tenant, _, _ string) (int64, error) {
	if m.bucketsErr != nil {
		return 0, m.bucketsErr
	}
	return 3, nil
}

func (m *mockStore) RemoveBucketByMerchant(_ context.Context, _ store.Tenant, _, _ string) (int64, error) {
	if m.bucketsErr != nil {
		return 0, m.bucketsErr
	}
	return 1, nil
}

func (m *mockStore) UpdateTransaction(_ context.Context, _ store.Tenant, _ string, update store.TransactionUpdate) error {
	m.updatedTransaction = update
	return mockStoreErr("store.transactions.update", m.updateTxErr)
}

func (m *mockStore) ListRules(_ context.Context, _ store.Tenant) ([]store.RuleRow, error) {
	if m.rulesErr != nil {
		return nil, mockStoreErr("store.rules.list", m.rulesErr)
	}
	if m.rules != nil {
		return m.rules, nil
	}
	return []store.RuleRow{}, nil
}

func (m *mockStore) GetRule(_ context.Context, _ store.Tenant, _ string) (*store.RuleRow, error) {
	return m.ruleResult, mockStoreErr("store.rules.get", m.ruleErr)
}

func (m *mockStore) CreateRule(_ context.Context, _ store.Tenant, r store.RuleRow) (*store.RuleRow, error) {
	if m.ruleErr != nil {
		return nil, mockStoreErr("store.rules.write", m.ruleErr)
	}
	r.ID = "new-id"
	return &r, nil
}

func (m *mockStore) UpdateRule(_ context.Context, _ store.Tenant, _ string, r store.RuleRow) (*store.RuleRow, error) {
	if m.ruleErr != nil {
		return nil, mockStoreErr("store.rules.write", m.ruleErr)
	}
	return &r, nil
}

func (m *mockStore) DeleteRule(_ context.Context, _ store.Tenant, _ string) error {
	return mockStoreErr("store.rules.delete", m.ruleErr)
}

func (m *mockStore) ImportUserRules(_ context.Context, _ store.Tenant, rows []store.RuleRow) error {
	m.importedRules = rows
	return mockStoreErr("store.rules.import", m.importErr)
}

func (m *mockStore) MuteTransaction(_ context.Context, _ store.Tenant, id string, muted bool, reason string) error {
	m.muteTransactionID = id
	m.muteTransactionValue = muted
	m.muteTransactionReason = reason
	return nil
}

func (m *mockStore) UpdateMuteReason(_ context.Context, _ store.Tenant, id, reason string) error {
	m.updateMuteReasonID = id
	m.updateMuteReasonValue = reason
	return nil
}

func (m *mockStore) MuteByMerchant(_ context.Context, _ store.Tenant, _, _ string) error { return nil }

func (m *mockStore) UpdateMerchantReason(_ context.Context, _ store.Tenant, id, reason string) error {
	m.updateMerchantID = id
	m.updateMerchantReason = reason
	return nil
}

func (m *mockStore) ListMutedMerchants(_ context.Context, _ store.Tenant) ([]store.MutedMerchant, error) {
	return []store.MutedMerchant{}, nil
}

func (m *mockStore) GetMutedMerchantsWithCount(_ context.Context, _ store.Tenant) ([]store.MutedMerchantWithCount, error) {
	return []store.MutedMerchantWithCount{}, nil
}

func (m *mockStore) DeleteMutedMerchant(_ context.Context, _ store.Tenant, _ string) error {
	return nil
}

func (m *mockStore) DeleteMutedMerchantAndUnmute(_ context.Context, _ store.Tenant, _ string) error {
	return nil
}

func (m *mockStore) CategorizeMerchant(_ context.Context, _ store.Tenant, _, _, _ string) (int64, error) {
	if m.categorizeMerchantN != 0 {
		return m.categorizeMerchantN, m.updateErr
	}
	return 3, m.updateErr
}

func (m *mockStore) GetSpendingHeatmap(_ context.Context, _ store.Tenant, _, _ *time.Time) (*store.HeatmapData, error) {
	if m.heatmapErr != nil {
		return nil, m.heatmapErr
	}
	if m.heatmapData != nil {
		return m.heatmapData, nil
	}
	return &store.HeatmapData{
		ByWeekdayHour: []store.WeekdayHourBucket{},
		ByDayOfMonth:  []store.DayOfMonthBucket{},
	}, nil
}

func (m *mockStore) GetAnnualSpend(_ context.Context, _ store.Tenant, _ int) ([]store.DailyBucket, error) {
	if m.annualErr != nil {
		return nil, m.annualErr
	}
	if m.annualData != nil {
		return m.annualData, nil
	}
	return []store.DailyBucket{}, nil
}

func (m *mockStore) ListExtractionDiagnostics(
	_ context.Context,
	_ store.Tenant,
	f store.DiagnosticFilter,
) ([]store.ExtractionDiagnosticRow, error) {
	m.diagnosticFilter = f
	if m.diagnosticErr != nil {
		return nil, mockStoreErr("store.diagnostics.list", m.diagnosticErr)
	}
	if m.diagnostics != nil {
		return m.diagnostics, nil
	}
	return []store.ExtractionDiagnosticRow{}, nil
}

func (m *mockStore) GetExtractionDiagnostic(_ context.Context, _ store.Tenant, _ string) (*store.ExtractionDiagnosticRow, error) {
	return m.diagnosticResult, mockStoreErr("store.diagnostics.get", m.diagnosticErr)
}

func (m *mockStore) UpdateExtractionDiagnosticStatus(
	_ context.Context,
	_ store.Tenant,
	id string,
	status string,
) (*store.ExtractionDiagnosticRow, error) {
	m.updateDiagnosticID = id
	m.updateDiagnosticStat = status
	return m.diagnosticResult, mockStoreErr("store.diagnostics.get", m.diagnosticErr)
}

func newTestHandlers(t *testing.T, st Storer, dm DaemonController, banksData ...[]byte) *Handlers {
	t.Helper()
	registry := plugins.NewRegistry()
	_ = registry.RegisterProvider((&testProvider{name: "gmail", authType: plugins.AuthTypeOAuth, requiresCreds: true}).provider())
	_ = registry.RegisterProvider((&testProvider{name: "thunderbird", authType: plugins.AuthTypeConfig, requiresCreds: false, schema: []plugins.ConfigField{
		{Key: "profilePath", Label: "Profile Directory", Type: "path", Required: true},
	}}).provider())
	var banks []byte
	if len(banksData) > 0 {
		banks = banksData[0]
	}
	var logLevel slog.LevelVar
	logLevel.Set(slog.LevelInfo)
	return NewHandlers(HandlersConfig{
		Registry:     registry,
		Store:        st,
		Daemon:       dm,
		Version:      "test",
		BaseURL:      "http://localhost:8080",
		FrontendURL:  "http://localhost:5173",
		ScanInterval: 60,
		LookbackDays: 180,
		BanksData:    banks,
		Logger:       slog.Default(),
		LogLevel:     &logLevel,
	})
}

type testProvider struct {
	name              string
	authType          plugins.AuthType
	requiresCreds     bool
	scopes            []string
	schema            []plugins.ConfigField
	preserveNilSchema bool
	guide             json.RawMessage
	reader            api.Reader
	newReaderErr      error
	input             plugins.ProviderInput
}

func (p *testProvider) Metadata() plugins.ProviderMetadata {
	schema := p.schema
	if schema == nil && !p.preserveNilSchema {
		schema = []plugins.ConfigField{}
	}
	return plugins.ProviderMetadata{
		Name:        p.name,
		Description: p.name + " reader",
		Auth: plugins.AuthSpec{
			Type:                      p.authType,
			RequiredScopes:            p.scopes,
			RequiresCredentialsUpload: p.requiresCreds,
		},
		ConfigSchema: schema,
		SetupGuide:   p.guide,
	}
}

func (p *testProvider) NewReader(input plugins.ProviderInput) (api.Reader, error) {
	p.input = input
	if p.newReaderErr != nil {
		return nil, p.newReaderErr
	}
	if p.reader != nil {
		return p.reader, nil
	}
	return nil, stderrors.New("not implemented in test stub")
}

func (p *testProvider) NewEmailSearcher(input plugins.ProviderInput) (api.EmailSearcher, error) {
	reader, err := p.NewReader(input)
	if err != nil {
		return nil, err
	}
	searcher, ok := reader.(api.EmailSearcher)
	if !ok {
		return nil, stderrors.New("not implemented in test stub")
	}
	return searcher, nil
}

func (p *testProvider) provider() plugins.Provider {
	return plugins.Provider{
		Metadata:         p.Metadata(),
		NewReader:        p.NewReader,
		NewEmailSearcher: p.NewEmailSearcher,
	}
}

type testSearchReader struct {
	query  api.EmailSearchQuery
	result []api.EmailSearchResult
	err    error
}

func (r *testSearchReader) Read(context.Context, chan<- *api.TransactionDetails, <-chan string) error {
	return nil
}

func (r *testSearchReader) Search(_ context.Context, query api.EmailSearchQuery) ([]api.EmailSearchResult, error) {
	r.query = query
	return r.result, r.err
}

type testLLMClient struct {
	healthErr error
}

func (c testLLMClient) Complete(context.Context, llm.Request) (llm.Response, error) {
	return llm.Response{}, stderrors.New("not implemented in test stub")
}

func (c testLLMClient) HealthCheck(context.Context) error {
	return c.healthErr
}

func testLLMProvider(t *testing.T, client llm.Client) *llm.Registry {
	t.Helper()
	return testLLMProviderWithFactory(t, func(llm.ClientConfig) (llm.Client, error) {
		return client, nil
	})
}

func testLLMProviderWithFactory(
	t *testing.T,
	newClient func(llm.ClientConfig) (llm.Client, error),
) *llm.Registry {
	t.Helper()
	registry := llm.NewRegistry()
	if err := registry.RegisterProvider(llm.Provider{
		Metadata: llm.ProviderMetadata{
			Name:           "openai",
			DisplayName:    "OpenAI",
			APIKeyURL:      "https://platform.openai.com/api-keys",
			APIKeyLinkText: "OpenAI dashboard",
			DataUse: llm.DataUseSpec{
				Mode:      llm.DataUseNoTrainingByDefault,
				PolicyURL: "https://platform.openai.com/docs/models/default-usage-policies-by-endpoint",
			},
			Auth: llm.AuthSpec{Type: llm.AuthTypeAPIKey, Required: true},
			ConfigSchema: json.RawMessage(`{
				"type":"object",
				"properties":{
					"model":{"type":"string","default":"gpt-5.4-mini"},
					"base_url":{"type":"string","default":"https://api.openai.com/v1"}
				}
			}`),
			Capabilities: []llm.Capability{llm.CapabilityTextGeneration, llm.CapabilityJSONSchema},
			ModelOptions: []llm.ModelOption{{
				ID:          "gpt-5.4-mini",
				DisplayName: "GPT-5.4 mini",
				Quality:     "Balanced",
				Cost:        "Lower",
				Recommended: true,
			}},
		},
		NewClient: newClient,
	}); err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}
	return registry
}

type stubRuleDraftService struct {
	result assistant.RuleDraftResult
	err    error
	input  assistant.RuleDraftInput
	tenant store.Tenant
}

func (s *stubRuleDraftService) DraftRule(
	_ context.Context,
	tenant store.Tenant,
	input assistant.RuleDraftInput,
) (assistant.RuleDraftResult, error) {
	s.tenant = tenant
	s.input = input
	return s.result, s.err
}

func get(h http.HandlerFunc, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, target, nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	return rr
}

func decodeJSON(t *testing.T, body string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(body), v); err != nil {
		t.Fatalf("decodeJSON: %v (body=%q)", err, body)
	}
}

func assertValidationError(
	t *testing.T,
	rr *httptest.ResponseRecorder,
	field string,
	location string,
	message string,
) {
	t.Helper()
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	var response ErrorResponse
	decodeJSON(t, rr.Body.String(), &response)
	if response.Message != "Request validation failed." {
		t.Fatalf("message = %q", response.Message)
	}
	if response.RequestID == "" {
		t.Fatal("request_id is empty")
	}
	want := []ValidationError{{Field: field, Location: location, Message: message}}
	if !reflect.DeepEqual(response.ValidationErrors, want) {
		t.Fatalf("validation_errors = %#v, want %#v", response.ValidationErrors, want)
	}
}

func (m *mockStore) GetSyncStatus(_ context.Context) (store.SyncStatus, error) {
	if m.syncStatusErr != nil {
		return store.SyncStatus{}, m.syncStatusErr
	}
	return m.syncStatus, nil
}

func (m *mockStore) GetCommunitySyncSettings(_ context.Context) (store.CommunitySyncSettings, error) {
	if m.communitySyncSettingsErr != nil {
		return store.CommunitySyncSettings{}, m.communitySyncSettingsErr
	}
	if m.communitySyncSettings.AutomaticSyncEnabled == nil {
		enabled := true
		m.communitySyncSettings.AutomaticSyncEnabled = &enabled
	}
	return m.communitySyncSettings, nil
}

func (m *mockStore) PatchCommunitySyncSettings(
	_ context.Context,
	patch store.CommunitySyncSettingsPatch,
) (store.CommunitySyncSettings, error) {
	if m.communitySyncSettingsErr != nil {
		return store.CommunitySyncSettings{}, m.communitySyncSettingsErr
	}
	m.communitySyncSettingsPatch = patch
	if patch.AutomaticSyncEnabled != nil {
		m.communitySyncSettings.AutomaticSyncEnabled = patch.AutomaticSyncEnabled
	}
	return m.communitySyncSettings, nil
}
