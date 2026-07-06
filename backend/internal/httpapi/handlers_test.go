package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/ArionMiles/expensor/backend/internal/auth"
	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
)

const (
	testTransactionID   = "11111111-1111-1111-1111-111111111111"
	testRuleID          = "22222222-2222-2222-2222-222222222222"
	testMutedMerchantID = "33333333-3333-3333-3333-333333333333"
)

// --- mocks ---

type mockDaemon struct {
	status DaemonStatus
}

func (m *mockDaemon) Status() DaemonStatus { return m.status }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

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
		return nil, m.createUserErr
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
			return store.ErrNotFound
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
	return nil, store.ErrNotFound
}

func (m *mockStore) FindUserByID(_ context.Context, id string) (*store.User, error) {
	if m.usersByID != nil {
		if user, ok := m.usersByID[id]; ok {
			return user, nil
		}
	}
	return nil, store.ErrNotFound
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
	return nil, store.ErrNotFound
}

func (m *mockStore) RevokeSession(_ context.Context, id string) error {
	m.revokedSessionID = id
	return nil
}

func (m *mockStore) CreateAccessToken(_ context.Context, input store.CreateAccessTokenInput) (*store.AccessToken, error) {
	m.createdAccessToken = input
	if m.createAccessTokenErr != nil {
		return nil, m.createAccessTokenErr
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
	return nil, store.ErrNotFound
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
	return nil, store.ErrNotFound
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
	return nil, store.ErrNotFound
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
	return m.getResult, m.getErr
}

func (m *mockStore) AddLabels(_ context.Context, _ store.Tenant, _ string, _ []string) error {
	return m.addLabelsErr
}

func (m *mockStore) RemoveLabel(_ context.Context, _ store.Tenant, _, _ string) error {
	return m.removeLblErr
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
	return "", errors.New("not found")
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

func (m *mockStore) GetFacets(_ context.Context, _ store.Tenant) (*store.Facets, error) {
	if m.getFacetsErr != nil {
		return nil, m.getFacetsErr
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
		return nil, m.labelsErr
	}
	if m.labels == nil {
		return []store.Label{}, nil
	}
	return m.labels, nil
}

func (m *mockStore) CreateLabel(_ context.Context, _ store.Tenant, _, _ string) error {
	return m.labelsErr
}

func (m *mockStore) UpdateLabel(_ context.Context, _ store.Tenant, _, _ string) error {
	return m.updateErr
}

func (m *mockStore) DeleteLabel(_ context.Context, _ store.Tenant, _ string, removeFromTransactions bool) error {
	m.deleteLabelCleanup = removeFromTransactions
	return m.labelsErr
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
		return nil, m.catsErr
	}
	if m.categories == nil {
		return []store.Category{}, nil
	}
	return m.categories, nil
}

func (m *mockStore) CreateCategory(_ context.Context, _ store.Tenant, _, _ string) error {
	return m.catsErr
}

func (m *mockStore) DeleteCategory(_ context.Context, _ store.Tenant, _ string, removeFromTransactions bool) error {
	m.deleteCategoryCleanup = removeFromTransactions
	return m.catsErr
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
		return nil, m.bucketsErr
	}
	if m.buckets == nil {
		return []store.Bucket{}, nil
	}
	return m.buckets, nil
}

func (m *mockStore) CreateBucket(_ context.Context, _ store.Tenant, _, _ string) error {
	return m.bucketsErr
}

func (m *mockStore) DeleteBucket(_ context.Context, _ store.Tenant, _ string, removeFromTransactions bool) error {
	m.deleteBucketCleanup = removeFromTransactions
	return m.bucketsErr
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
	return m.updateTxErr
}

func (m *mockStore) ListRules(_ context.Context, _ store.Tenant) ([]store.RuleRow, error) {
	if m.rulesErr != nil {
		return nil, m.rulesErr
	}
	if m.rules != nil {
		return m.rules, nil
	}
	return []store.RuleRow{}, nil
}

func (m *mockStore) GetRule(_ context.Context, _ store.Tenant, _ string) (*store.RuleRow, error) {
	return m.ruleResult, m.ruleErr
}

func (m *mockStore) CreateRule(_ context.Context, _ store.Tenant, r store.RuleRow) (*store.RuleRow, error) {
	if m.ruleErr != nil {
		return nil, m.ruleErr
	}
	r.ID = "new-id"
	return &r, nil
}

func (m *mockStore) UpdateRule(_ context.Context, _ store.Tenant, _ string, r store.RuleRow) (*store.RuleRow, error) {
	if m.ruleErr != nil {
		return nil, m.ruleErr
	}
	return &r, nil
}

func (m *mockStore) DeleteRule(_ context.Context, _ store.Tenant, _ string) error {
	return m.ruleErr
}

func (m *mockStore) ImportUserRules(_ context.Context, _ store.Tenant, rows []store.RuleRow) error {
	m.importedRules = rows
	return m.importErr
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
		return nil, m.diagnosticErr
	}
	if m.diagnostics != nil {
		return m.diagnostics, nil
	}
	return []store.ExtractionDiagnosticRow{}, nil
}

func (m *mockStore) GetExtractionDiagnostic(_ context.Context, _ store.Tenant, _ string) (*store.ExtractionDiagnosticRow, error) {
	return m.diagnosticResult, m.diagnosticErr
}

func (m *mockStore) UpdateExtractionDiagnosticStatus(
	_ context.Context,
	_ store.Tenant,
	id string,
	status string,
) (*store.ExtractionDiagnosticRow, error) {
	m.updateDiagnosticID = id
	m.updateDiagnosticStat = status
	return m.diagnosticResult, m.diagnosticErr
}

// newTestHandlers returns a Handlers wired with a real (minimal) plugin registry,
// the given store mock, and a mock daemon.
func newTestHandlers(t *testing.T, st Storer, dm DaemonStatusProvider, banksData ...[]byte) *Handlers {
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

// --- minimal plugin stubs ---

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
	return nil, errors.New("not implemented in test stub")
}

func (p *testProvider) NewEmailSearcher(input plugins.ProviderInput) (api.EmailSearcher, error) {
	reader, err := p.NewReader(input)
	if err != nil {
		return nil, err
	}
	searcher, ok := reader.(api.EmailSearcher)
	if !ok {
		return nil, errors.New("not implemented in test stub")
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
	var response ValidationErrorResponse
	decodeJSON(t, rr.Body.String(), &response)
	if response.Error != "request validation failed" {
		t.Fatalf("error = %q", response.Error)
	}
	want := []ValidationErrorDetail{{Field: field, Location: location, Message: message}}
	if !reflect.DeepEqual(response.Details, want) {
		t.Fatalf("details = %#v, want %#v", response.Details, want)
	}
}

// --- health ---

func TestHealth(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	rr := get(h.Health, "/api/health")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", resp["status"])
	}
}

// --- status ---

func TestStatus_WithStats(t *testing.T) {
	st := &mockStore{stats: &store.Stats{TotalCount: 42, TotalBase: 99999, BaseCurrency: "INR"}}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.Status, "/api/status")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	stats := resp["stats"].(map[string]any)
	if stats["total_count"] != float64(42) {
		t.Errorf("expected stats.total_count=42, got %v", stats["total_count"])
	}
}

func TestStatus_StatsError(t *testing.T) {
	st := &mockStore{statsErr: errors.New("stats failed")}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.Status, "/api/status")

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["error"] != "failed to fetch stats" {
		t.Errorf("expected failed to fetch stats error, got %q", resp["error"])
	}
}

// --- plugin listing ---

func TestListProviders(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	rr := get(h.ListProviders, "/api/providers")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var readers []ProviderInfo
	decodeJSON(t, rr.Body.String(), &readers)
	if len(readers) != 2 {
		t.Fatalf("expected 2 readers, got %d", len(readers))
	}
	// Verify gmail metadata.
	var gmail ProviderInfo
	for _, r := range readers {
		if r.Name == "gmail" {
			gmail = r
		}
	}
	if gmail.AuthType != plugins.AuthTypeOAuth {
		t.Errorf("gmail auth_type: want oauth, got %s", gmail.AuthType)
	}
	if !gmail.RequiresCredentialsUpload {
		t.Errorf("gmail should require credentials upload")
	}
}

func TestListReaders_NormalizesNilConfigSchema(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	registry := plugins.NewRegistry()
	if err := registry.RegisterProvider((&testProvider{
		name:              "nil-schema",
		authType:          plugins.AuthTypeConfig,
		preserveNilSchema: true,
	}).provider()); err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}
	h.registry = registry

	rr := get(h.ListProviders, "/api/providers")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var readers []ProviderInfo
	decodeJSON(t, rr.Body.String(), &readers)
	if len(readers) != 1 {
		t.Fatalf("expected 1 reader, got %d", len(readers))
	}
	if readers[0].ConfigSchema == nil {
		t.Fatalf("config_schema = nil, want non-nil empty slice; body = %s", rr.Body.String())
	}
	if len(readers[0].ConfigSchema) != 0 {
		t.Fatalf("config_schema len = %d, want 0", len(readers[0].ConfigSchema))
	}
}

// --- credentials status ---

func TestCredentialsStatus_Missing(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/providers/gmail/credentials/status", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.CredentialsStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]bool
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["exists"] {
		t.Error("expected exists=false")
	}
}

func TestCredentialsStatus_Present(t *testing.T) {
	ms := &mockStore{
		readerSecrets: map[string][]byte{"tenant-a/gmail": []byte(`{"installed":{}}`)},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})

	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/providers/gmail/credentials/status", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.CredentialsStatus(rr, req)

	var resp map[string]bool
	decodeJSON(t, rr.Body.String(), &resp)
	if !resp["exists"] {
		t.Error("expected exists=true")
	}
}

func TestCredentialsStatus_UnknownReader(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/providers/noexist/credentials/status", nil)
	req.SetPathValue("name", "noexist")
	rr := httptest.NewRecorder()
	h.CredentialsStatus(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestUploadCredentials_SavesToStore(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/providers/gmail/credentials", strings.NewReader(`{"installed":{}}`))
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()

	h.UploadCredentials(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["path"] != "db://reader_runtime/gmail/client_secret" {
		t.Fatalf("path = %q", resp["path"])
	}
	if string(ms.readerSecrets["tenant-a/gmail"]) != `{"installed":{}}` {
		t.Fatalf("secret was not persisted to store: %s", ms.readerSecrets["tenant-a/gmail"])
	}
}

func TestAuthStart_UsesMetadataScopes(t *testing.T) {
	const expectedScope = "https://www.googleapis.com/auth/gmail.readonly"
	secretJSON := `{
		"installed": {
			"client_id": "test-client-id.apps.googleusercontent.com",
			"client_secret": "test-client-secret",
			"auth_uri": "https://accounts.google.com/o/oauth2/auth",
			"token_uri": "https://oauth2.googleapis.com/token",
			"redirect_uris": ["http://localhost:8080/api/auth/callback"]
		}
	}`
	st := &mockStore{readerSecrets: map[string][]byte{"tenant-a/scoped": []byte(secretJSON)}}
	h := newTestHandlers(t, st, &mockDaemon{})
	registry := plugins.NewRegistry()
	if err := registry.RegisterProvider((&testProvider{
		name:          "scoped",
		authType:      plugins.AuthTypeOAuth,
		requiresCreds: true,
		scopes:        []string{expectedScope},
	}).provider()); err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}
	h.registry = registry

	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/providers/scoped/auth/start", nil)
	req.SetPathValue("name", "scoped")
	rr := httptest.NewRecorder()

	h.AuthStart(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	authURL, err := url.Parse(resp["url"])
	if err != nil {
		t.Fatalf("parse auth URL: %v", err)
	}
	if got := authURL.Query().Get("scope"); got != expectedScope {
		t.Fatalf("scope = %q, want %q (url: %s)", got, expectedScope, resp["url"])
	}
}

// --- auth status ---

func TestAuthStatus_NoToken(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/providers/gmail/auth/status", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.AuthStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["authenticated"] != false {
		t.Errorf("expected authenticated=false")
	}
}

func TestAuthStatus_ConfigReader(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/providers/thunderbird/auth/status", nil)
	req.SetPathValue("name", "thunderbird")
	rr := httptest.NewRecorder()
	h.AuthStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["authenticated"] != true {
		t.Errorf("config-only reader should always be authenticated, got %v", resp["authenticated"])
	}
}

func TestAuthStatus_UsesStoreToken(t *testing.T) {
	ms := &mockStore{
		readerTokens: map[string][]byte{
			"tenant-a/gmail": []byte(`{"access_token":"a","token_type":"Bearer","expiry":"2999-01-01T00:00:00Z"}`),
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/providers/gmail/auth/status", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()

	h.AuthStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["authenticated"] != true {
		t.Fatalf("expected authenticated=true, got %v", resp)
	}
	if resp["auth_state"] != "connected" {
		t.Fatalf("expected auth_state=connected, got %v", resp)
	}
}

func TestAuthStatus_RefreshesExpiredAccessTokenWithRefreshToken(t *testing.T) {
	tokenClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			return nil, fmt.Errorf("token endpoint method = %s, want POST", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			return nil, fmt.Errorf("ParseForm: %w", err)
		}
		if got := r.Form.Get("grant_type"); got != "refresh_token" {
			return nil, fmt.Errorf("grant_type = %q, want refresh_token", got)
		}
		if got := r.Form.Get("refresh_token"); got != "old-refresh" {
			return nil, fmt.Errorf("refresh_token = %q, want old-refresh", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"access_token":"new-access","token_type":"Bearer","expires_in":3600}`)),
			Request:    r,
		}, nil
	})}

	secretJSON := fmt.Sprintf(`{
		"installed": {
			"client_id": "test-client-id.apps.googleusercontent.com",
			"client_secret": "test-client-secret",
			"auth_uri": "https://accounts.google.com/o/oauth2/auth",
			"token_uri": %q,
			"redirect_uris": ["http://localhost:8080/api/auth/callback"]
		}
	}`, "https://oauth.test/token")
	ms := &mockStore{
		readerSecrets: map[string][]byte{"tenant-a/gmail": []byte(secretJSON)},
		readerTokens: map[string][]byte{
			"tenant-a/gmail": []byte(`{"access_token":"old-access","refresh_token":"old-refresh","token_type":"Bearer","expiry":"2000-01-01T00:00:00Z"}`),
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.WithValue(context.Background(), oauth2.HTTPClient, tokenClient), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/providers/gmail/auth/status", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()

	h.AuthStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["authenticated"] != true {
		t.Fatalf("expected authenticated=true, got %v", resp)
	}
	if resp["auth_state"] != "connected" {
		t.Fatalf("expected auth_state=connected, got %v", resp)
	}
	if !strings.Contains(string(ms.readerTokens["tenant-a/gmail"]), "new-access") {
		t.Fatalf("saved token = %s, want refreshed access token", ms.readerTokens["tenant-a/gmail"])
	}
	if !strings.Contains(string(ms.readerTokens["tenant-a/gmail"]), "old-refresh") {
		t.Fatalf("saved token = %s, want refresh token preserved", ms.readerTokens["tenant-a/gmail"])
	}
}

func TestAuthStatus_ExpiredAccessTokenWithoutRefreshTokenRequiresAuth(t *testing.T) {
	ms := &mockStore{
		readerTokens: map[string][]byte{
			"tenant-a/gmail": []byte(`{"access_token":"old-access","token_type":"Bearer","expiry":"2000-01-01T00:00:00Z"}`),
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/providers/gmail/auth/status", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()

	h.AuthStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["authenticated"] != false {
		t.Fatalf("expected authenticated=false, got %v", resp)
	}
	if resp["auth_state"] != "reauthorization_required" {
		t.Fatalf("expected auth_state=reauthorization_required, got %v", resp)
	}
}

func TestAuthStatus_InvalidRefreshTokenRequiresAuth(t *testing.T) {
	tokenClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":"invalid_grant","error_description":"Token has been expired or revoked."}`)),
			Request:    r,
		}, nil
	})}

	secretJSON := fmt.Sprintf(`{
		"installed": {
			"client_id": "test-client-id.apps.googleusercontent.com",
			"client_secret": "test-client-secret",
			"auth_uri": "https://accounts.google.com/o/oauth2/auth",
			"token_uri": %q,
			"redirect_uris": ["http://localhost:8080/api/auth/callback"]
		}
	}`, "https://oauth.test/token")
	ms := &mockStore{
		readerSecrets: map[string][]byte{"tenant-a/gmail": []byte(secretJSON)},
		readerTokens: map[string][]byte{
			"tenant-a/gmail": []byte(`{"access_token":"old-access","refresh_token":"old-refresh","token_type":"Bearer","expiry":"2000-01-01T00:00:00Z"}`),
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.WithValue(context.Background(), oauth2.HTTPClient, tokenClient), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/providers/gmail/auth/status", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()

	h.AuthStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["authenticated"] != false {
		t.Fatalf("expected authenticated=false, got %v", resp)
	}
	if resp["auth_state"] != "reauthorization_required" {
		t.Fatalf("expected auth_state=reauthorization_required, got %v", resp)
	}
}

// --- reader status ---

func TestReaderStatus_Thunderbird_NotConfigured(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/providers/thunderbird/status", nil)
	req.SetPathValue("name", "thunderbird")
	rr := httptest.NewRecorder()
	h.ReaderStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["ready"] != false {
		t.Error("thunderbird without config should not be ready")
	}
}

func TestReaderStatus_Thunderbird_Configured(t *testing.T) {
	ms := &mockStore{
		readerConfigs: map[string]json.RawMessage{
			"tenant-a/thunderbird": json.RawMessage(`{"profilePath":"/tmp/tb"}`),
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})

	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/providers/thunderbird/status", nil)
	req.SetPathValue("name", "thunderbird")
	rr := httptest.NewRecorder()
	h.ReaderStatus(rr, req)

	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["ready"] != true {
		t.Errorf("thunderbird with config should be ready, got %v", resp)
	}
}

func TestSearchReaderMessages_ReturnsSamples(t *testing.T) {
	receivedAt := time.Date(2026, 6, 1, 10, 30, 0, 0, time.UTC)
	reader := &testSearchReader{result: []api.EmailSearchResult{
		{
			ID:          "message-1",
			SenderEmail: "alerts@example.com",
			Subject:     "Card spend approved",
			Body:        "INR 42.00 at Coffee",
			ReceivedAt:  &receivedAt,
		},
	}}
	provider := &testProvider{
		name:     "sample",
		authType: plugins.AuthTypeConfig,
		reader:   reader,
	}
	registry := plugins.NewRegistry()
	if err := registry.RegisterProvider(provider.provider()); err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	h.registry = registry

	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/providers/sample/messages?subject=spend&limit=3", nil)
	req.SetPathValue("name", "sample")
	rr := httptest.NewRecorder()
	h.SearchProviderMessages(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if reader.query.SubjectQuery != "spend" {
		t.Fatalf("subject search = %q, want spend", reader.query.SubjectQuery)
	}
	if reader.query.Limit != 3 {
		t.Fatalf("limit = %d, want 3", reader.query.Limit)
	}
	var resp struct {
		Results []ProviderSearchResultResponse `json:"results"`
	}
	decodeJSON(t, rr.Body.String(), &resp)
	if len(resp.Results) != 1 {
		t.Fatalf("results len = %d, want 1", len(resp.Results))
	}
	if resp.Results[0].SenderEmail != "alerts@example.com" || resp.Results[0].Body != "INR 42.00 at Coffee" {
		t.Fatalf("unexpected message response: %#v", resp.Results[0])
	}
	if resp.Results[0].ReceivedAt == nil || !resp.Results[0].ReceivedAt.Equal(receivedAt) {
		t.Fatalf("received_at = %v, want %v", resp.Results[0].ReceivedAt, receivedAt)
	}
}

func TestSearchReaderMessages_MissingSubjectReturns422(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/providers/gmail/messages", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()

	h.SearchProviderMessages(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestSaveReaderConfig_SavesToStore(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(
		ctx,
		http.MethodPut,
		"/api/providers/thunderbird/config",
		strings.NewReader(`{"config":{"mailboxes":"Inbox"}}`),
	)
	req.SetPathValue("name", "thunderbird")
	rr := httptest.NewRecorder()

	h.SaveReaderConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(ms.readerConfigs["tenant-a/thunderbird"], []byte("Inbox")) {
		t.Fatalf("config not persisted: %s", ms.readerConfigs["tenant-a/thunderbird"])
	}
}

func TestGetReaderConfig_LoadsFromStore(t *testing.T) {
	ms := &mockStore{
		readerConfigs: map[string]json.RawMessage{
			"tenant-a/thunderbird": json.RawMessage(`{"config":{"mailboxes":"Inbox"}}`),
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/providers/thunderbird/config", nil)
	req.SetPathValue("name", "thunderbird")
	rr := httptest.NewRecorder()

	h.GetReaderConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte("Inbox")) {
		t.Fatalf("config response = %s", rr.Body.String())
	}
}

func TestRevokeToken_DeletesStoreToken(t *testing.T) {
	ms := &mockStore{readerTokens: map[string][]byte{"tenant-a/gmail": []byte(`{"access_token":"a"}`)}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/providers/gmail/auth/token", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()

	h.RevokeToken(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if _, ok := ms.readerTokens["tenant-a/gmail"]; ok {
		t.Fatal("token was not deleted from store")
	}
}

func TestDisconnectReader_StopsDaemonWhenActiveReaderIsRemoved(t *testing.T) {
	ms := &mockStore{
		scanningState: store.TenantScanningState{TenantID: "tenant-a", ActiveReader: "gmail", Enabled: true, State: store.ScanningStateRunning},
		readerSecrets: map[string][]byte{"tenant-a/gmail": []byte(`{"installed":{}}`)},
		readerTokens:  map[string][]byte{"tenant-a/gmail": []byte(`{"access_token":"a"}`)},
	}
	var stopCalls int
	h := newTestHandlers(t, ms, &mockDaemon{status: DaemonStatus{Running: true}})
	h.stopFn = func() { stopCalls++ }
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/providers/gmail", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()

	h.DisconnectReader(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if stopCalls != 1 {
		t.Fatalf("stop calls = %d, want 1", stopCalls)
	}
	if ms.scanningState.ActiveReader != "" {
		t.Fatalf("active scanning reader = %q, want cleared", ms.scanningState.ActiveReader)
	}
}

func TestDisconnectReader_DoesNotStopDaemonWhenInactiveReaderIsRemoved(t *testing.T) {
	ms := &mockStore{
		scanningState: store.TenantScanningState{TenantID: "tenant-a", ActiveReader: "gmail", Enabled: true, State: store.ScanningStateRunning},
		readerConfigs: map[string]json.RawMessage{"tenant-a/thunderbird": json.RawMessage(`{"mailbox":"Inbox"}`)},
		readerSecrets: map[string][]byte{"tenant-a/gmail": []byte(`{"installed":{}}`)},
		readerTokens:  map[string][]byte{"tenant-a/gmail": []byte(`{"access_token":"a"}`)},
	}
	var stopCalls int
	h := newTestHandlers(t, ms, &mockDaemon{status: DaemonStatus{Running: true}})
	h.stopFn = func() { stopCalls++ }
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/providers/thunderbird", nil)
	req.SetPathValue("name", "thunderbird")
	rr := httptest.NewRecorder()

	h.DisconnectReader(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if stopCalls != 0 {
		t.Fatalf("stop calls = %d, want 0", stopCalls)
	}
	if ms.scanningState.ActiveReader != "gmail" {
		t.Fatalf("active scanning reader = %q, want gmail", ms.scanningState.ActiveReader)
	}
}

// --- transactions ---

func TestListTransactions_Empty(t *testing.T) {
	st := &mockStore{transactions: []store.Transaction{}, listResult: store.TransactionListResult{Total: 0}}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.ListTransactions, "/api/transactions?page=1&page_size=10")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["total"] != float64(0) {
		t.Errorf("expected total=0, got %v", resp["total"])
	}
	txns, ok := resp["transactions"].([]any)
	if !ok {
		t.Fatalf("expected transactions array, got %#v", resp["transactions"])
	}
	if len(txns) != 0 {
		t.Fatalf("expected empty transactions array, got %d entries", len(txns))
	}
}

func TestListTransactions_NilSliceReturnsEmptyArray(t *testing.T) {
	st := &mockStore{transactions: nil, listResult: store.TransactionListResult{Total: 0}}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.ListTransactions, "/api/transactions?page=1&page_size=10")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	txns, ok := resp["transactions"].([]any)
	if !ok {
		t.Fatalf("expected transactions array, got %#v", resp["transactions"])
	}
	if len(txns) != 0 {
		t.Fatalf("expected empty transactions array, got %d entries", len(txns))
	}
}

func TestListTransactions_WithResults(t *testing.T) {
	now := time.Now()
	st := &mockStore{
		transactions: []store.Transaction{
			{ID: "abc", Amount: 100, Currency: "INR", MerchantInfo: "Amazon", Timestamp: now, Labels: []string{}},
		},
		listResult: store.TransactionListResult{
			Total:       1,
			TotalAmount: 100,
		},
	}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.ListTransactions, "/api/transactions")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	txns := resp["transactions"].([]any)
	if len(txns) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(txns))
	}
	if resp["total_amount"] != float64(100) {
		t.Fatalf("expected total_amount=100, got %v", resp["total_amount"])
	}
	if resp["base_currency"] != "INR" {
		t.Fatalf("expected base_currency=INR, got %v", resp["base_currency"])
	}
}

func TestListTransactions_StoreError(t *testing.T) {
	st := &mockStore{listErr: errors.New("db error")}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.ListTransactions, "/api/transactions")
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestListTransactions_RejectsInvalidQuery(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		field   string
		message string
	}{
		{name: "page overflow", query: "page=99999999999999999999999999999", field: "page", message: "must be an integer"},
		{name: "negative page", query: "page=-1", field: "page", message: "must be at least 0"},
		{name: "page size too large", query: "page_size=101", field: "page_size", message: "must be at most 100"},
		{name: "invalid date", query: "date_from=yesterday", field: "date_from", message: "must be an RFC3339 timestamp"},
		{name: "invalid weekday", query: "weekday=7", field: "weekday", message: "must be at most 6"},
		{name: "invalid hour", query: "hour_from=24", field: "hour_from", message: "must be at most 23"},
		{name: "invalid boolean flag", query: "show_muted=true", field: "show_muted", message: "must be 1 when present"},
		{name: "invalid sort", query: "sort_dir=sideways", field: "sort_dir", message: "must be one of: asc, desc"},
		{name: "invalid timezone", query: "tz=Mars/Olympus", field: "tz", message: "must be a valid IANA timezone"},
		{name: "control character", query: "currency=%00bad", field: "currency", message: "must not contain control characters"},
		{name: "invalid search query", query: "q=%00bad", field: "q", message: "must not contain control characters"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &mockStore{}
			h := newTestHandlers(t, st, &mockDaemon{})
			rr := get(h.ListTransactions, "/api/transactions?"+tt.query)

			assertValidationError(t, rr, tt.field, "query", tt.message)
			if st.listCalls != 0 || st.searchCalls != 0 {
				t.Fatalf("store calls = list:%d search:%d", st.listCalls, st.searchCalls)
			}
		})
	}
}

func TestListTransactions_AcceptsLargePageAndMaximumPageSize(t *testing.T) {
	st := &mockStore{transactions: []store.Transaction{}}
	h := newTestHandlers(t, st, &mockDaemon{})

	rr := get(h.ListTransactions, "/api/transactions?page=10001&page_size=100")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if st.listFilter.Page != 10001 || st.listFilter.PageSize != 100 {
		t.Fatalf("pagination = page:%d page_size:%d", st.listFilter.Page, st.listFilter.PageSize)
	}
}

func TestListTransactions_RejectsOffsetOverflow(t *testing.T) {
	st := &mockStore{}
	h := newTestHandlers(t, st, &mockDaemon{})

	rr := get(h.ListTransactions, fmt.Sprintf(
		"/api/transactions?page=%d&page_size=100",
		math.MaxInt,
	))

	assertValidationError(t, rr, "page", "query", "is too large for page_size")
	if st.listCalls != 0 || st.searchCalls != 0 {
		t.Fatalf("store calls = list:%d search:%d", st.listCalls, st.searchCalls)
	}
}

func TestListTransactions_DefaultsPagination(t *testing.T) {
	st := &mockStore{transactions: []store.Transaction{}, listResult: store.TransactionListResult{Total: 0}}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.ListTransactions, "/api/transactions")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if st.listFilter.Page != 1 || st.listFilter.PageSize != 20 {
		t.Fatalf("pagination = page:%d page_size:%d", st.listFilter.Page, st.listFilter.PageSize)
	}
}

func TestListTransactions_ZeroPageDefaultsToFirstPage(t *testing.T) {
	st := &mockStore{transactions: []store.Transaction{}}
	h := newTestHandlers(t, st, &mockDaemon{})

	rr := get(h.ListTransactions, "/api/transactions?page=0")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if st.listFilter.Page != 1 {
		t.Fatalf("page = %d, want 1", st.listFilter.Page)
	}
}

func TestGetTransaction_Found(t *testing.T) {
	txn := &store.Transaction{ID: "11111111-1111-1111-1111-111111111111", Amount: 500, Currency: "INR", Labels: []string{"food"}}
	st := &mockStore{getResult: txn}
	h := newTestHandlers(t, st, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/transactions/11111111-1111-1111-1111-111111111111", nil)
	req.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
	rr := httptest.NewRecorder()
	h.GetTransaction(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp store.Transaction
	decodeJSON(t, rr.Body.String(), &resp)
	if resp.ID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("expected UUID id, got %s", resp.ID)
	}
}

func TestGetTransaction_NotFound(t *testing.T) {
	st := &mockStore{getErr: store.ErrNotFound}
	h := newTestHandlers(t, st, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/transactions/22222222-2222-2222-2222-222222222222", nil)
	req.SetPathValue("id", "22222222-2222-2222-2222-222222222222")
	rr := httptest.NewRecorder()
	h.GetTransaction(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestGetTransaction_InvalidID(t *testing.T) {
	st := &mockStore{}
	h := newTestHandlers(t, st, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/transactions/not-a-uuid", nil)
	req.SetPathValue("id", "not-a-uuid")
	rr := httptest.NewRecorder()
	h.GetTransaction(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestUpdateTransaction_Success(t *testing.T) {
	txn := &store.Transaction{ID: testTransactionID, Description: "Updated", Labels: []string{}}
	st := &mockStore{getResult: txn}
	h := newTestHandlers(t, st, &mockDaemon{})

	body := `{"description":"Updated"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/transactions/11111111-1111-1111-1111-111111111111", strings.NewReader(body))
	req.SetPathValue("id", testTransactionID)
	rr := httptest.NewRecorder()
	h.UpdateTransaction(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", rr.Code, rr.Body.String())
	}
}

func TestUpdateTransaction_MuteStateAndReason(t *testing.T) {
	txn := &store.Transaction{ID: testTransactionID, Muted: true, MuteReason: "duplicate", Labels: []string{}}
	st := &mockStore{getResult: txn}
	h := newTestHandlers(t, st, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPatch,
		"/api/transactions/"+testTransactionID,
		strings.NewReader(`{"muted":true,"mute_reason":"duplicate"}`),
	)
	req.SetPathValue("id", testTransactionID)
	rr := httptest.NewRecorder()
	h.UpdateTransaction(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	if st.muteTransactionID != testTransactionID || !st.muteTransactionValue || st.muteTransactionReason != "duplicate" {
		t.Fatalf("mute call = id=%q muted=%v reason=%q", st.muteTransactionID, st.muteTransactionValue, st.muteTransactionReason)
	}
}

func TestUpdateTransaction_MuteReasonOnly(t *testing.T) {
	txn := &store.Transaction{ID: testTransactionID, Muted: true, MuteReason: "updated", Labels: []string{}}
	st := &mockStore{getResult: txn}
	h := newTestHandlers(t, st, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPatch,
		"/api/transactions/"+testTransactionID,
		strings.NewReader(`{"mute_reason":"updated"}`),
	)
	req.SetPathValue("id", testTransactionID)
	rr := httptest.NewRecorder()
	h.UpdateTransaction(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	if st.updateMuteReasonID != testTransactionID || st.updateMuteReasonValue != "updated" {
		t.Fatalf("mute reason call = id=%q reason=%q", st.updateMuteReasonID, st.updateMuteReasonValue)
	}
}

func TestUpdateTransaction_NotFound(t *testing.T) {
	st := &mockStore{updateTxErr: store.ErrNotFound}
	h := newTestHandlers(t, st, &mockDaemon{})

	body := `{"description":"x"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/transactions/"+testTransactionID, strings.NewReader(body))
	req.SetPathValue("id", testTransactionID)
	rr := httptest.NewRecorder()
	h.UpdateTransaction(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestUpdateTransaction_FetchUpdatedNotFound(t *testing.T) {
	st := &mockStore{getErr: store.ErrNotFound}
	h := newTestHandlers(t, st, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/transactions/"+testTransactionID, strings.NewReader(`{}`))
	req.SetPathValue("id", testTransactionID)
	rr := httptest.NewRecorder()
	h.UpdateTransaction(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestUpdateTransaction_InvalidID(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/transactions/not-a-uuid", strings.NewReader(`{}`))
	req.SetPathValue("id", "not-a-uuid")
	rr := httptest.NewRecorder()
	h.UpdateTransaction(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestUpdateTransaction_InvalidJSON(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/transactions/11111111-1111-1111-1111-111111111111", strings.NewReader("not-json"))
	req.SetPathValue("id", testTransactionID)
	rr := httptest.NewRecorder()
	h.UpdateTransaction(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestListExtractionDiagnostics_DefaultsStatusOpen(t *testing.T) {
	st := &mockStore{diagnostics: []store.ExtractionDiagnosticRow{{ID: "diag-1", Status: store.DiagnosticStatusOpen}}}
	h := newTestHandlers(t, st, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/extraction-diagnostics", nil)
	rr := httptest.NewRecorder()
	h.ListExtractionDiagnostics(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	if st.diagnosticFilter.Status != store.DiagnosticStatusOpen {
		t.Fatalf("expected status open, got %q", st.diagnosticFilter.Status)
	}
	var resp []store.ExtractionDiagnosticRow
	decodeJSON(t, rr.Body.String(), &resp)
	if len(resp) != 1 || resp[0].ID != "diag-1" {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestListExtractionDiagnostics_StatusAllAndLimit(t *testing.T) {
	st := &mockStore{}
	h := newTestHandlers(t, st, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/extraction-diagnostics?status=all&limit=25", nil)
	rr := httptest.NewRecorder()
	h.ListExtractionDiagnostics(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	if st.diagnosticFilter.Status != store.DiagnosticStatusAll {
		t.Fatalf("expected status all, got %q", st.diagnosticFilter.Status)
	}
	if st.diagnosticFilter.Limit != 25 {
		t.Fatalf("expected limit 25, got %d", st.diagnosticFilter.Limit)
	}
}

func TestListExtractionDiagnostics_RejectsInvalidQuery(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		field   string
		message string
	}{
		{name: "status", query: "status=pending", field: "status", message: "must be one of: open, resolved, ignored, all"},
		{name: "limit syntax", query: "limit=bad", field: "limit", message: "must be an integer"},
		{name: "limit range", query: "limit=0", field: "limit", message: "must be at least 1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
			req := httptest.NewRequestWithContext(
				context.Background(),
				http.MethodGet,
				"/api/extraction-diagnostics?"+tt.query,
				nil,
			)
			rr := httptest.NewRecorder()
			h.ListExtractionDiagnostics(rr, req)

			assertValidationError(t, rr, tt.field, "query", tt.message)
		})
	}
}

func TestGetExtractionDiagnostic_Found(t *testing.T) {
	row := &store.ExtractionDiagnosticRow{ID: "11111111-1111-1111-1111-111111111111", Status: store.DiagnosticStatusOpen}
	h := newTestHandlers(t, &mockStore{diagnosticResult: row}, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/extraction-diagnostics/11111111-1111-1111-1111-111111111111", nil)
	req.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
	rr := httptest.NewRecorder()
	h.GetExtractionDiagnostic(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	var resp store.ExtractionDiagnosticRow
	decodeJSON(t, rr.Body.String(), &resp)
	if resp.ID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("expected UUID id, got %q", resp.ID)
	}
}

func TestGetExtractionDiagnostic_NotFound(t *testing.T) {
	h := newTestHandlers(t, &mockStore{diagnosticErr: store.ErrNotFound}, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/extraction-diagnostics/22222222-2222-2222-2222-222222222222", nil)
	req.SetPathValue("id", "22222222-2222-2222-2222-222222222222")
	rr := httptest.NewRecorder()
	h.GetExtractionDiagnostic(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestGetExtractionDiagnostic_InvalidID(t *testing.T) {
	h := newTestHandlers(t, &mockStore{diagnosticErr: errors.New("store should not be called")}, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/extraction-diagnostics/not-a-uuid", nil)
	req.SetPathValue("id", "not-a-uuid")
	rr := httptest.NewRecorder()
	h.GetExtractionDiagnostic(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestUpdateExtractionDiagnosticStatus_InvalidStatus(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPatch,
		"/api/extraction-diagnostics/11111111-1111-1111-1111-111111111111",
		strings.NewReader(`{"status":"all"}`),
	)
	req.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
	rr := httptest.NewRecorder()
	h.UpdateExtractionDiagnosticStatus(rr, req)

	assertValidationError(t, rr, "status", "body", "must be one of: open, resolved, ignored")
}

func TestUpdateExtractionDiagnosticStatus_MissingStatus(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPatch,
		"/api/extraction-diagnostics/11111111-1111-1111-1111-111111111111",
		strings.NewReader(`{}`),
	)
	req.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
	rr := httptest.NewRecorder()
	h.UpdateExtractionDiagnosticStatus(rr, req)

	assertValidationError(t, rr, "status", "body", "is required")
}

func TestUpdateExtractionDiagnosticStatus_InvalidJSON(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPatch,
		"/api/extraction-diagnostics/11111111-1111-1111-1111-111111111111",
		strings.NewReader("not-json"),
	)
	req.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
	rr := httptest.NewRecorder()
	h.UpdateExtractionDiagnosticStatus(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body=%s)", rr.Code, rr.Body.String())
	}
}

func TestUpdateExtractionDiagnosticStatus_NotFound(t *testing.T) {
	h := newTestHandlers(t, &mockStore{diagnosticErr: store.ErrNotFound}, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPatch,
		"/api/extraction-diagnostics/33333333-3333-3333-3333-333333333333",
		strings.NewReader(`{"status":"resolved"}`),
	)
	req.SetPathValue("id", "33333333-3333-3333-3333-333333333333")
	rr := httptest.NewRecorder()
	h.UpdateExtractionDiagnosticStatus(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestUpdateExtractionDiagnosticStatus_InvalidID(t *testing.T) {
	h := newTestHandlers(t, &mockStore{diagnosticErr: errors.New("store should not be called")}, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPatch,
		"/api/extraction-diagnostics/not-a-uuid",
		strings.NewReader(`{"status":"resolved"}`),
	)
	req.SetPathValue("id", "not-a-uuid")
	rr := httptest.NewRecorder()
	h.UpdateExtractionDiagnosticStatus(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestUpdateExtractionDiagnosticStatus_Conflict(t *testing.T) {
	h := newTestHandlers(t, &mockStore{diagnosticErr: store.ErrDiagnosticConflict}, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPatch,
		"/api/extraction-diagnostics/44444444-4444-4444-4444-444444444444",
		strings.NewReader(`{"status":"open"}`),
	)
	req.SetPathValue("id", "44444444-4444-4444-4444-444444444444")
	rr := httptest.NewRecorder()
	h.UpdateExtractionDiagnosticStatus(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}

func TestAddLabels_Success(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	body := `{"labels":["food","work"]}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/transactions/11111111-1111-1111-1111-111111111111/labels", strings.NewReader(body))
	req.SetPathValue("id", testTransactionID)
	rr := httptest.NewRecorder()
	h.AddLabels(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestAddLabels_InvalidJSON(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/transactions/11111111-1111-1111-1111-111111111111/labels", strings.NewReader("bad"))
	req.SetPathValue("id", testTransactionID)
	rr := httptest.NewRecorder()
	h.AddLabels(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestAddLabels_RejectsEmptyLabels(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/api/transactions/"+testTransactionID+"/labels",
		strings.NewReader(`{"labels":[]}`),
	)
	req.SetPathValue("id", testTransactionID)
	rr := httptest.NewRecorder()
	h.AddLabels(rr, req)

	assertValidationError(t, rr, "labels", "body", "must be at least 1")
}

func TestAddLabels_InvalidID(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/transactions/not-a-uuid/labels", strings.NewReader(`{"labels":["food"]}`))
	req.SetPathValue("id", "not-a-uuid")
	rr := httptest.NewRecorder()
	h.AddLabels(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestAddLabels_BatchSuccess(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})

	body := `{"labels":["food","work","recurring"]}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/transactions/11111111-1111-1111-1111-111111111111/labels", strings.NewReader(body))
	req.SetPathValue("id", testTransactionID)
	rr := httptest.NewRecorder()
	h.AddLabels(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestAddLabels_StoreError_Returns500(t *testing.T) {
	h := newTestHandlers(t, &mockStore{addLabelsErr: errors.New("db error")}, &mockDaemon{})

	body := `{"labels":["food"]}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/transactions/11111111-1111-1111-1111-111111111111/labels", strings.NewReader(body))
	req.SetPathValue("id", testTransactionID)
	rr := httptest.NewRecorder()
	h.AddLabels(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestRemoveLabel_Success(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/transactions/11111111-1111-1111-1111-111111111111/labels/food", nil)
	req.SetPathValue("id", testTransactionID)
	req.SetPathValue("label", "food")
	rr := httptest.NewRecorder()
	h.RemoveLabel(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestRemoveLabel_NotFound(t *testing.T) {
	h := newTestHandlers(t, &mockStore{removeLblErr: store.ErrNotFound}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/transactions/11111111-1111-1111-1111-111111111111/labels/missing", nil)
	req.SetPathValue("id", testTransactionID)
	req.SetPathValue("label", "missing")
	rr := httptest.NewRecorder()
	h.RemoveLabel(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestRemoveLabel_InvalidID(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/transactions/not-a-uuid/labels/food", nil)
	req.SetPathValue("id", "not-a-uuid")
	req.SetPathValue("label", "food")
	rr := httptest.NewRecorder()
	h.RemoveLabel(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestListTransactions_WithSearchQuery(t *testing.T) {
	st := &mockStore{
		searchResult: []store.Transaction{{ID: "x", MerchantInfo: "Zomato", Labels: []string{}}},
		searchListResult: store.TransactionListResult{
			Total:       1,
			TotalAmount: 245,
		},
	}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.ListTransactions, "/api/transactions?q=zomato")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["total"] != float64(1) {
		t.Errorf("expected total=1, got %v", resp["total"])
	}
	if resp["total_amount"] != float64(245) {
		t.Errorf("expected total_amount=245, got %v", resp["total_amount"])
	}
	if resp["base_currency"] != "INR" {
		t.Errorf("expected base_currency=INR, got %v", resp["base_currency"])
	}
}

func TestListTransactions_SearchEmptyArray(t *testing.T) {
	st := &mockStore{
		searchResult:     []store.Transaction{},
		searchListResult: store.TransactionListResult{Total: 0},
	}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.ListTransactions, "/api/transactions?q=zomato")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	txns, ok := resp["transactions"].([]any)
	if !ok {
		t.Fatalf("expected transactions array, got %#v", resp["transactions"])
	}
	if len(txns) != 0 {
		t.Fatalf("expected empty transactions array, got %d entries", len(txns))
	}
}

func TestListTransactions_SearchNilSliceReturnsEmptyArray(t *testing.T) {
	st := &mockStore{
		searchResult:     nil,
		searchListResult: store.TransactionListResult{Total: 0},
	}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.ListTransactions, "/api/transactions?q=zomato")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	txns, ok := resp["transactions"].([]any)
	if !ok {
		t.Fatalf("expected transactions array, got %#v", resp["transactions"])
	}
	if len(txns) != 0 {
		t.Fatalf("expected empty transactions array, got %d entries", len(txns))
	}
}

func TestListTransactions_SearchMutedAndIndividualFlags(t *testing.T) {
	st := &mockStore{
		searchResult:     []store.Transaction{},
		searchListResult: store.TransactionListResult{Total: 0},
	}
	h := newTestHandlers(t, st, &mockDaemon{})

	rr := get(h.ListTransactions, "/api/transactions?q=zomato&muted_only=1&individual_only=1")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !st.searchFilter.MutedOnly {
		t.Fatal("expected muted_only=1 to set SearchTransactions filter")
	}
	if !st.searchFilter.IndividualOnly {
		t.Fatal("expected individual_only=1 to set SearchTransactions filter")
	}
}

func TestListTransactions_SearchParsesListFilters(t *testing.T) {
	st := &mockStore{
		searchResult:     []store.Transaction{},
		searchListResult: store.TransactionListResult{Total: 0},
	}
	h := newTestHandlers(t, st, &mockDaemon{})

	rr := get(
		h.ListTransactions,
		"/api/transactions?q=instamart&source_type=Credit%20Card&bank=HDFC"+
			"&date_from=2026-04-30T18:30:00.000Z&date_to=2026-05-31T18:29:59.999Z&sort_dir=asc",
	)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if st.searchFilter.SourceType != "Credit Card" {
		t.Fatalf("source_type = %q, want Credit Card", st.searchFilter.SourceType)
	}
	if st.searchFilter.Bank != "HDFC" {
		t.Fatalf("bank = %q, want HDFC", st.searchFilter.Bank)
	}
	if st.searchFilter.From == nil || st.searchFilter.From.UTC().Format(time.RFC3339Nano) != "2026-04-30T18:30:00Z" {
		t.Fatalf("date_from = %#v", st.searchFilter.From)
	}
	if st.searchFilter.To == nil || st.searchFilter.To.UTC().Format(time.RFC3339Nano) != "2026-05-31T18:29:59.999Z" {
		t.Fatalf("date_to = %#v", st.searchFilter.To)
	}
	if st.searchFilter.SortDir != "asc" {
		t.Fatalf("sort_dir = %q, want asc", st.searchFilter.SortDir)
	}
}

// --- generateState ---

func TestGenerateState_IsUnique(t *testing.T) {
	s1, err := generateState("gmail")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s2, err := generateState("gmail")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s1 == s2 {
		t.Error("generateState must return unique values on each call")
	}
}

func TestGenerateState_IsNotPureTimestamp(t *testing.T) {
	s, err := generateState("gmail")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify the suffix after the last colon is a 32-char hex string.
	parts := strings.Split(s, ":")
	suffix := parts[len(parts)-1]
	if len(suffix) != 32 {
		t.Errorf("expected 32-char hex suffix, got %q (len=%d)", suffix, len(suffix))
	}
	for _, c := range suffix {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			t.Errorf("suffix %q contains non-hex character %q", suffix, c)
		}
	}
}

func TestGenerateState_ContainsReaderName(t *testing.T) {
	s, err := generateState("thunderbird")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(s, "thunderbird") {
		t.Errorf("state %q should contain the reader name", s)
	}
}

// --- OAuth state TTL ---

func TestAuthCallback_RejectsUnknownState(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/auth/callback?state=doesnotexist&code=xyz", nil)
	rr := httptest.NewRecorder()
	h.AuthCallback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown state, got %d", rr.Code)
	}
}

func TestAuthCallback_RejectsExpiredState(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})

	// Inject an already-expired entry directly into the map.
	expiredState := "reader:gmail:expiredtoken"
	h.mu.Lock()
	h.oauthStates[expiredState] = oauthStateEntry{
		readerName: "gmail",
		expiresAt:  time.Now().Add(-1 * time.Second), // already expired
	}
	h.mu.Unlock()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/auth/callback?state="+expiredState+"&code=xyz", nil)
	rr := httptest.NewRecorder()
	h.AuthCallback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for expired state, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestAuthCallback_ReturnsClosePageAfterTokenSaved(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("token endpoint method = %s, want POST", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"new-access","refresh_token":"new-refresh","token_type":"Bearer","expires_in":3600}`)
	}))
	defer tokenServer.Close()

	secretJSON := fmt.Sprintf(`{
		"installed": {
			"client_id": "test-client-id.apps.googleusercontent.com",
			"client_secret": "test-client-secret",
			"auth_uri": "https://accounts.google.com/o/oauth2/auth",
			"token_uri": %q,
			"redirect_uris": ["http://localhost:8080/api/auth/callback"]
		}
	}`, tokenServer.URL)
	st := &mockStore{readerSecrets: map[string][]byte{"tenant-a/gmail": []byte(secretJSON)}}
	h := newTestHandlers(t, st, &mockDaemon{})

	state := "reader:gmail:validtoken"
	h.mu.Lock()
	h.oauthStates[state] = oauthStateEntry{
		readerName: "gmail",
		tenant:     store.Tenant{ID: "tenant-a"},
		expiresAt:  time.Now().Add(time.Minute),
	}
	h.mu.Unlock()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/auth/callback?state="+state+"&code=4%2F0Acode", nil)
	rr := httptest.NewRecorder()
	h.AuthCallback(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", got)
	}
	if location := rr.Header().Get("Location"); location != "" {
		t.Fatalf("Location header = %q, want no redirect", location)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "window.close()") {
		t.Fatalf("body should close the OAuth tab, got: %s", body)
	}
	if !strings.Contains(body, "http://localhost:5173/setup?auth=success&amp;reader=gmail") {
		t.Fatalf("body should include escaped fallback setup link, got: %s", body)
	}
	if !strings.Contains(string(st.readerTokens["tenant-a/gmail"]), "new-refresh") {
		t.Fatalf("saved token = %s, want refresh token from re-grant", st.readerTokens["tenant-a/gmail"])
	}
}

func TestAuthCallback_UsesTenantFromOAuthStartState(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("token endpoint method = %s, want POST", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"new-access","refresh_token":"new-refresh","token_type":"Bearer","expires_in":3600}`)
	}))
	defer tokenServer.Close()

	secretJSON := fmt.Sprintf(`{
		"installed": {
			"client_id": "test-client-id.apps.googleusercontent.com",
			"client_secret": "test-client-secret",
			"auth_uri": "https://accounts.google.com/o/oauth2/auth",
			"token_uri": %q,
			"redirect_uris": ["http://localhost:8080/api/auth/callback"]
		}
	}`, tokenServer.URL)
	st := &mockStore{
		readerSecrets: map[string][]byte{"tenant-a/gmail": []byte(secretJSON)},
	}
	h := newTestHandlers(t, st, &mockDaemon{})

	startCtx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	startReq := httptest.NewRequestWithContext(startCtx, http.MethodPost, "/api/providers/gmail/auth/start", nil)
	startReq.SetPathValue("name", "gmail")
	startRR := httptest.NewRecorder()
	h.AuthStart(startRR, startReq)

	if startRR.Code != http.StatusOK {
		t.Fatalf("start status = %d body=%s", startRR.Code, startRR.Body.String())
	}
	var startResp map[string]string
	decodeJSON(t, startRR.Body.String(), &startResp)
	authURL, err := url.Parse(startResp["url"])
	if err != nil {
		t.Fatalf("parse auth URL: %v", err)
	}
	state := authURL.Query().Get("state")
	if state == "" {
		t.Fatalf("auth URL missing state: %s", startResp["url"])
	}

	callbackReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/auth/callback?state="+url.QueryEscape(state)+"&code=4%2F0Acode", nil)
	callbackRR := httptest.NewRecorder()
	h.AuthCallback(callbackRR, callbackReq)

	if callbackRR.Code != http.StatusOK {
		t.Fatalf("callback status = %d body=%s", callbackRR.Code, callbackRR.Body.String())
	}
	if !strings.Contains(string(st.readerTokens["tenant-a/gmail"]), "new-refresh") {
		t.Fatalf("saved tenant token = %s, want refresh token from re-grant", st.readerTokens["tenant-a/gmail"])
	}
}

// --- app preferences ---

func TestGetPreferencesCombinesStoredValuesAndDefaults(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/config/preferences", nil)
	rr := httptest.NewRecorder()
	h.GetPreferences(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp PreferencesResponse
	decodeJSON(t, rr.Body.String(), &resp)
	if resp.BaseCurrency != "INR" || resp.ScanInterval != 60 || resp.LookbackDays != 180 {
		t.Fatalf("unexpected configured defaults: %#v", resp)
	}
	if resp.Timezone != "" || resp.TimeFormat != "HH:mm" {
		t.Fatalf("unexpected display defaults: %#v", resp)
	}
}

func TestGetPreferencesUsesStoredValues(t *testing.T) {
	ms := &mockStore{appConfig: map[string]string{
		"base_currency":   "USD",
		"scan_interval":   "120",
		"lookback_days":   "365",
		"app.timezone":    "Asia/Kolkata",
		"app.time_format": "h:mm a",
	}}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/config/preferences", nil)
	rr := httptest.NewRecorder()
	h.GetPreferences(rr, req)

	var resp PreferencesResponse
	decodeJSON(t, rr.Body.String(), &resp)
	if resp.BaseCurrency != "USD" || resp.ScanInterval != 120 || resp.LookbackDays != 365 {
		t.Fatalf("unexpected stored numeric preferences: %#v", resp)
	}
	if resp.Timezone != "Asia/Kolkata" || resp.TimeFormat != "h:mm a" {
		t.Fatalf("unexpected stored display preferences: %#v", resp)
	}
}

func TestGetSetupStatusRequiresMissingPreferences(t *testing.T) {
	h := newTestHandlers(t, &mockStore{appConfig: map[string]string{"scan_interval": "60"}}, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/config/setup-status", nil)
	rr := httptest.NewRecorder()
	h.GetSetupStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Required bool     `json:"required"`
		Missing  []string `json:"missing"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Required {
		t.Fatalf("required = false, want true")
	}
	want := []string{"base_currency", "timezone", "time_format"}
	if !reflect.DeepEqual(resp.Missing, want) {
		t.Fatalf("missing = %#v, want %#v", resp.Missing, want)
	}
}

func TestGetSetupStatusCompleteWhenPreferencesExist(t *testing.T) {
	h := newTestHandlers(t, &mockStore{appConfig: map[string]string{
		"base_currency":   "USD",
		"app.timezone":    "America/New_York",
		"app.time_format": "h:mm a",
	}}, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/config/setup-status", nil)
	rr := httptest.NewRecorder()
	h.GetSetupStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Required bool     `json:"required"`
		Missing  []string `json:"missing"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Required {
		t.Fatalf("required = true, want false")
	}
	if len(resp.Missing) != 0 {
		t.Fatalf("missing = %#v, want empty", resp.Missing)
	}
}

func TestGetDashboardData_Success(t *testing.T) {
	ms := &mockStore{
		dashboardData: &store.DashboardData{
			CurrentMonth: store.DashboardSection{
				Label: "April 2026",
				Stats: store.Stats{TotalCount: 1, TotalBase: 1000, BaseCurrency: "INR"},
				Charts: store.ChartData{
					MonthlySpend:      []store.TimeBucket{},
					DailySpend:        []store.TimeBucket{},
					ByCategory:        map[string]float64{"Shopping": 1000},
					ByBucket:          map[string]float64{},
					ByLabel:           map[string]float64{},
					BySource:          map[string]float64{},
					ByCategoryMonthly: map[string]store.CategoryMonthlyEntry{},
				},
			},
			AllTime: store.DashboardSection{
				Label: "All Time",
				Stats: store.Stats{TotalCount: 3, TotalBase: 3000, BaseCurrency: "INR"},
				Charts: store.ChartData{
					MonthlySpend:      []store.TimeBucket{},
					DailySpend:        []store.TimeBucket{},
					ByCategory:        map[string]float64{"Shopping": 3000},
					ByBucket:          map[string]float64{},
					ByLabel:           map[string]float64{},
					BySource:          map[string]float64{},
					ByCategoryMonthly: map[string]store.CategoryMonthlyEntry{},
				},
			},
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/stats/dashboard", nil)
	rr := httptest.NewRecorder()
	h.GetDashboardData(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	var resp map[string]json.RawMessage
	decodeJSON(t, rr.Body.String(), &resp)
	if _, ok := resp["current_month"]; !ok {
		t.Fatalf("expected current_month section in response: %s", rr.Body.String())
	}
	if _, ok := resp["all_time"]; !ok {
		t.Fatalf("expected all_time section in response: %s", rr.Body.String())
	}
}

func TestGetMonthlyBreakdown_InvalidDimension(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/api/stats/labels/monthly?dimension=nope",
		nil,
	)
	rr := httptest.NewRecorder()
	h.GetLabelMonthlySpend(rr, req)

	assertValidationError(t, rr, "dimension", "query", "must be one of: labels, categories, buckets")
}

func TestPatchPreferencesUpdatesSuppliedFields(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})

	body := strings.NewReader(
		`{"base_currency":"usd","scan_interval":120,"lookback_days":365,"timezone":"Asia/Kolkata","time_format":"h:mm a"}`,
	)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/config/preferences", body)
	rr := httptest.NewRecorder()
	h.PatchPreferences(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp PreferencesResponse
	decodeJSON(t, rr.Body.String(), &resp)
	if resp.BaseCurrency != "USD" || resp.ScanInterval != 120 || resp.LookbackDays != 365 {
		t.Fatalf("unexpected response: %#v", resp)
	}
	want := map[string]string{
		"base_currency":   "USD",
		"scan_interval":   "120",
		"lookback_days":   "365",
		"app.timezone":    "Asia/Kolkata",
		"app.time_format": "h:mm a",
	}
	if !reflect.DeepEqual(ms.appConfig, want) {
		t.Fatalf("stored preferences = %#v, want %#v", ms.appConfig, want)
	}
}

func TestPatchPreferencesRejectsInvalidFieldsBeforeWriting(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		field   string
		message string
	}{
		{
			name:    "currency",
			body:    `{"base_currency":"US1"}`,
			field:   "base_currency",
			message: "must be a 3-letter ISO 4217 code",
		},
		{
			name:    "scan interval",
			body:    `{"base_currency":"USD","scan_interval":5}`,
			field:   "scan_interval",
			message: "must be at least 10",
		},
		{
			name:    "lookback days",
			body:    `{"lookback_days":3651}`,
			field:   "lookback_days",
			message: "must be at most 3650",
		},
		{
			name:    "timezone",
			body:    `{"timezone":"Mars/Olympus"}`,
			field:   "timezone",
			message: "must be a valid IANA timezone",
		},
		{
			name:    "time format",
			body:    `{"time_format":"24h"}`,
			field:   "time_format",
			message: "must be one of: HH:mm, HH:mm:ss, h:mm a, h:mm:ss a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockStore{}
			h := newTestHandlers(t, ms, &mockDaemon{})
			req := httptest.NewRequestWithContext(
				context.Background(),
				http.MethodPatch,
				"/api/config/preferences",
				strings.NewReader(tt.body),
			)
			rr := httptest.NewRecorder()
			h.PatchPreferences(rr, req)

			assertValidationError(t, rr, tt.field, "body", tt.message)
			if len(ms.appConfig) != 0 {
				t.Fatalf("invalid patch persisted values: %#v", ms.appConfig)
			}
		})
	}
}

func TestPatchPreferencesRejectsInvalidJSON(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPatch,
		"/api/config/preferences",
		strings.NewReader("not-json"),
	)
	rr := httptest.NewRecorder()
	h.PatchPreferences(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	if len(ms.appConfig) != 0 {
		t.Fatalf("invalid patch persisted values: %#v", ms.appConfig)
	}
}

func TestGetFacets_ReturnsEmptySlices(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/transactions/facets", nil)
	rr := httptest.NewRecorder()
	h.GetFacets(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp struct {
		Sources     []string       `json:"sources"`
		Categories  []string       `json:"categories"`
		Currencies  []string       `json:"currencies"`
		Labels      []string       `json:"labels"`
		LabelCounts map[string]int `json:"label_counts"`
		Buckets     []string       `json:"buckets"`
	}
	decodeJSON(t, rr.Body.String(), &resp)
	emptySlices := map[string][]string{
		"sources":    resp.Sources,
		"categories": resp.Categories,
		"currencies": resp.Currencies,
		"labels":     resp.Labels,
		"buckets":    resp.Buckets,
	}
	for key, value := range emptySlices {
		if value == nil {
			t.Errorf("expected %q to be an empty slice, got nil", key)
		}
	}
	if resp.LabelCounts == nil {
		t.Error("expected label_counts to be an empty object, got nil")
	}
}

func TestGetFacets_ReturnsLabelCounts(t *testing.T) {
	h := newTestHandlers(
		t,
		&mockStore{facets: &store.Facets{Labels: []string{"Food"}, LabelCounts: map[string]int{"Food": 3}}},
		&mockDaemon{},
	)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/transactions/facets", nil)
	rr := httptest.NewRecorder()

	h.GetFacets(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp struct {
		LabelCounts map[string]int `json:"label_counts"`
	}
	decodeJSON(t, rr.Body.String(), &resp)
	if got := resp.LabelCounts["Food"]; got != 3 {
		t.Fatalf("expected Food label count 3, got %d", got)
	}
}

func TestGetFacets_StoreError(t *testing.T) {
	h := newTestHandlers(t, &mockStore{getFacetsErr: errors.New("db error")}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/transactions/facets", nil)
	rr := httptest.NewRecorder()
	h.GetFacets(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

// --- labels ---

func TestListLabels_Success(t *testing.T) {
	ms := &mockStore{labels: []store.Label{{Name: "food", Color: "#f59e0b"}}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	rr := get(h.ListLabels, "/api/config/labels")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp []store.Label
	decodeJSON(t, rr.Body.String(), &resp)
	if len(resp) != 1 || resp[0].Name != "food" {
		t.Errorf("unexpected response: %v", resp)
	}
}

func TestCreateLabel_Success(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	body := strings.NewReader(`{"name":"groceries","color":"#aabbcc"}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/config/labels", body)
	rr := httptest.NewRecorder()
	h.CreateLabel(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["name"] != "groceries" {
		t.Errorf("expected name=groceries, got %q", resp["name"])
	}
}

func TestCreateLabel_RejectsInvalidColor(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/api/config/labels",
		strings.NewReader(`{"name":"groceries","color":"blue"}`),
	)
	rr := httptest.NewRecorder()
	h.CreateLabel(rr, req)

	assertValidationError(t, rr, "color", "body", "must be a valid hexadecimal color")
}

func TestDeleteLabel_NotFound(t *testing.T) {
	ms := &mockStore{labelsErr: store.ErrNotFound}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/config/labels/missing", nil)
	req.SetPathValue("name", "missing")
	rr := httptest.NewRecorder()
	// DeleteLabel returns the labelsErr directly; since it's ErrNotFound the handler
	// logs and returns 500 (DeleteLabel has no ErrNotFound branch in the handler).
	// The store just returns the error; handler writes 500. Verify non-204.
	h.DeleteLabel(rr, req)
	if rr.Code == http.StatusNoContent {
		t.Fatalf("expected non-204 on error, got 204")
	}
}

func TestDeleteLabel_RemoveFromTransactionsOption(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})

	body := strings.NewReader(`{"remove_from_transactions":true}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/config/labels/food", body)
	req.SetPathValue("name", "food")
	rr := httptest.NewRecorder()

	h.DeleteLabel(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if !ms.deleteLabelCleanup {
		t.Fatal("expected delete label to request transaction label cleanup")
	}
}

func TestDeleteLabel_RemoveFromTransactionsQueryOption(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodDelete,
		"/api/config/labels/food?remove_from_transactions=true",
		nil,
	)
	req.SetPathValue("name", "food")
	rr := httptest.NewRecorder()

	h.DeleteLabel(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if !ms.deleteLabelCleanup {
		t.Fatal("expected delete label query parameter to request transaction label cleanup")
	}
}

func TestDeleteLabel_RejectsInvalidCleanupFlag(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodDelete,
		"/api/config/labels/food?remove_from_transactions=sometimes",
		nil,
	)
	req.SetPathValue("name", "food")
	rr := httptest.NewRecorder()
	h.DeleteLabel(rr, req)

	assertValidationError(t, rr, "remove_from_transactions", "query", "must be a boolean")
}

// --- categories ---

func TestListCategories_Success(t *testing.T) {
	ms := &mockStore{categories: []store.Category{{Name: "food & dining", IsDefault: true}}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	rr := get(h.ListCategories, "/api/config/categories")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp []store.Category
	decodeJSON(t, rr.Body.String(), &resp)
	if len(resp) != 1 || resp[0].Name != "food & dining" {
		t.Errorf("unexpected response: %v", resp)
	}
}

func TestDeleteCategory_DefaultRejected(t *testing.T) {
	ms := &mockStore{catsErr: fmt.Errorf("cannot delete default category \"food\"")}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/config/categories/food", nil)
	req.SetPathValue("name", "food")
	rr := httptest.NewRecorder()
	h.DeleteCategory(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestGetCategoryMappings_Success(t *testing.T) {
	ms := &mockStore{categoryMappings: map[string][]string{"Food": {"swiggy", "zomato"}}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	rr := get(h.GetCategoryMappings, "/api/config/categories/mappings")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string][]string
	decodeJSON(t, rr.Body.String(), &resp)
	if !reflect.DeepEqual(resp["Food"], []string{"swiggy", "zomato"}) {
		t.Fatalf("unexpected mappings: %#v", resp)
	}
}

func TestApplyCategoryByMerchant_Success(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPut,
		"/api/config/categories/Food/merchant-mappings/swiggy",
		nil,
	)
	req.SetPathValue("name", "Food")
	req.SetPathValue("pattern", "swiggy")
	rr := httptest.NewRecorder()

	h.ApplyCategoryByMerchant(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]int
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["applied"] != 2 {
		t.Fatalf("expected applied=2, got %#v", resp)
	}
}

func TestDeleteCategory_RemoveFromTransactionsOption(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	body := strings.NewReader(`{"remove_from_transactions":true}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/config/categories/Food", body)
	req.SetPathValue("name", "Food")
	rr := httptest.NewRecorder()

	h.DeleteCategory(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if !ms.deleteCategoryCleanup {
		t.Fatal("expected delete category to request transaction cleanup")
	}
}

func TestExportCategories_IncludesMerchants(t *testing.T) {
	ms := &mockStore{
		categories:       []store.Category{{Name: "Food"}},
		categoryMappings: map[string][]string{"Food": {"swiggy"}},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})
	rr := get(h.ExportCategories, "/api/config/categories/export")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp []map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp[0]["name"] != "Food" {
		t.Fatalf("unexpected export: %#v", resp)
	}
	merchants, ok := resp[0]["merchants"].([]any)
	if !ok || len(merchants) != 1 || merchants[0] != "swiggy" {
		t.Fatalf("expected merchants in export, got %#v", resp)
	}
}

func TestGetBucketMappings_Success(t *testing.T) {
	ms := &mockStore{bucketMappings: map[string][]string{"Needs": {"rent"}}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	rr := get(h.GetBucketMappings, "/api/config/buckets/mappings")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string][]string
	decodeJSON(t, rr.Body.String(), &resp)
	if !reflect.DeepEqual(resp["Needs"], []string{"rent"}) {
		t.Fatalf("unexpected mappings: %#v", resp)
	}
}

func TestApplyBucketByMerchant_Success(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPut,
		"/api/config/buckets/Needs/merchant-mappings/rent",
		nil,
	)
	req.SetPathValue("name", "Needs")
	req.SetPathValue("pattern", "rent")
	rr := httptest.NewRecorder()

	h.ApplyBucketByMerchant(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]int
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["applied"] != 3 {
		t.Fatalf("expected applied=3, got %#v", resp)
	}
}

func TestDeleteBucket_RemoveFromTransactionsOption(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	body := strings.NewReader(`{"remove_from_transactions":true}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/config/buckets/Needs", body)
	req.SetPathValue("name", "Needs")
	rr := httptest.NewRecorder()

	h.DeleteBucket(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if !ms.deleteBucketCleanup {
		t.Fatal("expected delete bucket to request transaction cleanup")
	}
}

func TestExportBuckets_IncludesMerchants(t *testing.T) {
	ms := &mockStore{
		buckets:        []store.Bucket{{Name: "Needs"}},
		bucketMappings: map[string][]string{"Needs": {"rent"}},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})
	rr := get(h.ExportBuckets, "/api/config/buckets/export")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp []map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp[0]["name"] != "Needs" {
		t.Fatalf("unexpected export: %#v", resp)
	}
	merchants, ok := resp[0]["merchants"].([]any)
	if !ok || len(merchants) != 1 || merchants[0] != "rent" {
		t.Fatalf("expected merchants in export, got %#v", resp)
	}
}

func TestGetReaderCheckpoint_EmptyValueReturnsNull(t *testing.T) {
	ms := &mockStore{appConfig: map[string]string{"reader.gmail.last_scan_at": ""}}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/config/providers/gmail/checkpoint", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.GetReaderCheckpoint(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if val, ok := resp["last_scan_at"]; !ok || val != nil {
		t.Fatalf("expected last_scan_at to be null, got %#v", resp["last_scan_at"])
	}
}

// --- heatmap ---

func TestGetHeatmap_Success(t *testing.T) {
	ms := &mockStore{
		heatmapData: &store.HeatmapData{
			ByWeekdayHour: []store.WeekdayHourBucket{
				{Weekday: 1, Hour: 14, Amount: 500.0, Count: 3},
			},
			ByDayOfMonth: []store.DayOfMonthBucket{
				{Day: 15, Amount: 1200.0, Count: 5},
			},
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/stats/heatmap", nil)
	rr := httptest.NewRecorder()
	h.GetHeatmap(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp store.HeatmapData
	decodeJSON(t, rr.Body.String(), &resp)
	if len(resp.ByWeekdayHour) != 1 {
		t.Errorf("expected 1 weekday/hour bucket, got %d", len(resp.ByWeekdayHour))
	}
	if resp.ByWeekdayHour[0].Hour != 14 {
		t.Errorf("expected Hour=14, got %d", resp.ByWeekdayHour[0].Hour)
	}
	if len(resp.ByDayOfMonth) != 1 {
		t.Errorf("expected 1 day-of-month bucket, got %d", len(resp.ByDayOfMonth))
	}
}

func TestGetHeatmap_StoreError_Returns500(t *testing.T) {
	ms := &mockStore{heatmapErr: errors.New("db connection lost")}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/stats/heatmap", nil)
	rr := httptest.NewRecorder()
	h.GetHeatmap(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestGetHeatmap_WithFromTo_Returns200(t *testing.T) {
	ms := &mockStore{
		heatmapData: &store.HeatmapData{
			ByWeekdayHour: []store.WeekdayHourBucket{{Weekday: 0, Hour: 10, Amount: 100, Count: 1}},
			ByDayOfMonth:  []store.DayOfMonthBucket{},
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodGet,
		"/api/stats/heatmap?from=2026-04-01T00:00:00Z&to=2026-04-30T23:59:59Z",
		nil,
	)
	rr := httptest.NewRecorder()
	h.GetHeatmap(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp store.HeatmapData
	decodeJSON(t, rr.Body.String(), &resp)
	if len(resp.ByWeekdayHour) != 1 {
		t.Errorf("expected 1 bucket, got %d", len(resp.ByWeekdayHour))
	}
}

func TestGetHeatmap_InvalidFrom_Returns400(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodGet,
		"/api/stats/heatmap?from=not-a-date",
		nil,
	)
	rr := httptest.NewRecorder()
	h.GetHeatmap(rr, req)

	assertValidationError(t, rr, "from", "query", "must be an RFC3339 timestamp")
}

func TestGetHeatmap_RejectsInvalidYear(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	rr := get(h.GetHeatmap, "/api/stats/heatmap?year=invalid")

	assertValidationError(t, rr, "year", "query", "must be an integer")
}

func TestGetHeatmap_WithYear_ReturnsAnnualData(t *testing.T) {
	ms := &mockStore{
		annualData: []store.DailyBucket{
			{Date: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), Amount: 1500.0, Count: 3},
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/stats/heatmap?year=2026", nil)
	rr := httptest.NewRecorder()
	h.GetHeatmap(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp struct {
		Year    int                 `json:"year"`
		Buckets []store.DailyBucket `json:"buckets"`
	}
	decodeJSON(t, rr.Body.String(), &resp)
	if resp.Year != 2026 {
		t.Errorf("expected year=2026, got %d", resp.Year)
	}
	if len(resp.Buckets) != 1 {
		t.Errorf("expected 1 bucket, got %d", len(resp.Buckets))
	}
	if resp.Buckets[0].Amount != 1500.0 {
		t.Errorf("expected Amount=1500, got %f", resp.Buckets[0].Amount)
	}
}

func TestGetHeatmap_WithYearStoreError_Returns500(t *testing.T) {
	ms := &mockStore{annualErr: errors.New("db connection lost")}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/stats/heatmap?year=2026", nil)
	rr := httptest.NewRecorder()
	h.GetHeatmap(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestGetHeatmap_RejectsYearWithRange(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/api/stats/heatmap?year=2026&from=2026-01-01T00:00:00Z",
		nil,
	)
	rr := httptest.NewRecorder()
	h.GetHeatmap(rr, req)

	assertValidationError(t, rr, "year", "query", "cannot be combined with from or to")
}

func TestListRules_ReturnsSourceObjectAndSenderEmails(t *testing.T) {
	ms := &mockStore{rules: []store.RuleRow{{
		ID:              "1",
		Name:            "HDFC Credit Card",
		SenderEmails:    []string{"alerts@hdfcbank.net", "alerts@hdfcbank.bank.in"},
		SubjectContains: "HDFC Credit Card",
		AmountRegex:     `Rs\.([\d.]+)`,
		MerchantRegex:   `at (.*?) on`,
		SourceType:      "Credit Card",
		SourceLabel:     "HDFC Credit Card",
		Bank:            "HDFC",
	}}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/rules", nil)
	rr := httptest.NewRecorder()

	h.ListRules(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp []struct {
		Name         string     `json:"name"`
		SenderEmails []string   `json:"sender_emails"`
		Source       api.Source `json:"source"`
	}
	decodeJSON(t, rr.Body.String(), &resp)
	if len(resp) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(resp))
	}
	if want := []string{"alerts@hdfcbank.net", "alerts@hdfcbank.bank.in"}; !reflect.DeepEqual(want, resp[0].SenderEmails) {
		t.Fatalf("sender_emails = %#v, want %#v", resp[0].SenderEmails, want)
	}
	if want := (api.Source{Type: "Credit Card", Label: "HDFC Credit Card", Bank: "HDFC"}); resp[0].Source != want {
		t.Fatalf("source = %#v, want %#v", resp[0].Source, want)
	}
}

const validRuleBody = `{
	"name":"New Rule",
	"sender_emails":["alerts@example.com"],
	"amount_regex":"Rs\\.([\\d.]+)",
	"merchant_regex":"at (.*?) on",
	"currency_regex":"(INR)",
	"source":{"type":"Credit Card","label":"Example Card","bank":"Example Bank"}
}`

func TestCreateRule_Success(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/rules", strings.NewReader(validRuleBody))
	rr := httptest.NewRecorder()
	h.CreateRule(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestCreateRule_AcceptsSourceObjectAndSenderEmails(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	body := `{
		"name":"HDFC Credit Card",
		"sender_emails":["alerts@hdfcbank.net","alerts@hdfcbank.bank.in"],
		"subject_contains":"HDFC Credit Card",
		"amount_regex":"Rs\\.([\\d.]+)",
		"merchant_regex":"at (.*?) on",
		"source":{"type":"Credit Card","label":"HDFC Credit Card","bank":"HDFC"}
	}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/rules", strings.NewReader(body))
	rr := httptest.NewRecorder()

	h.CreateRule(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp struct {
		SenderEmails []string   `json:"sender_emails"`
		Source       api.Source `json:"source"`
	}
	decodeJSON(t, rr.Body.String(), &resp)
	if want := []string{"alerts@hdfcbank.net", "alerts@hdfcbank.bank.in"}; !reflect.DeepEqual(want, resp.SenderEmails) {
		t.Fatalf("sender_emails = %#v, want %#v", resp.SenderEmails, want)
	}
	if want := (api.Source{Type: "Credit Card", Label: "HDFC Credit Card", Bank: "HDFC"}); resp.Source != want {
		t.Fatalf("source = %#v, want %#v", resp.Source, want)
	}
}

func TestCreateRule_DuplicateNameReturns409(t *testing.T) {
	h := newTestHandlers(t, &mockStore{ruleErr: store.ErrRuleNameConflict}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/rules", strings.NewReader(validRuleBody))
	rr := httptest.NewRecorder()

	h.CreateRule(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["error"] != "rule name already exists" {
		t.Fatalf("error = %q, want rule name already exists", resp["error"])
	}
}

func TestCreateRule_ClearsActiveReaderCheckpoint(t *testing.T) {
	ms := &mockStore{
		scanningState: store.TenantScanningState{TenantID: "tenant-a", ActiveReader: "gmail", Enabled: true, State: store.ScanningStateRunning},
		appConfig:     map[string]string{"reader.gmail.last_scan_at": "2026-04-27T00:00:00Z"},
	}
	dm := &mockDaemon{}
	h := newTestHandlers(t, ms, dm)
	var restarted DaemonRunRequest
	h.restartFn = func(req DaemonRunRequest) { restarted = req }

	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/rules", strings.NewReader(validRuleBody))
	rr := httptest.NewRecorder()

	h.CreateRule(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if got := ms.appConfig["reader.gmail.last_scan_at"]; got != "" {
		t.Fatalf("reader checkpoint = %q, want empty", got)
	}
	if restarted.Reader != "" {
		t.Fatalf("restartFn called while daemon stopped: %q", restarted.Reader)
	}
}

func TestCreateRule_RestartsRunningDaemonAfterCheckpointClear(t *testing.T) {
	ms := &mockStore{
		scanningState: store.TenantScanningState{TenantID: "tenant-a", ActiveReader: "gmail", Enabled: true, State: store.ScanningStateRunning},
		appConfig:     map[string]string{"reader.gmail.last_scan_at": "2026-04-27T00:00:00Z"},
	}
	dm := &mockDaemon{status: DaemonStatus{Running: true}}
	h := newTestHandlers(t, ms, dm)
	var restarted DaemonRunRequest
	h.restartFn = func(req DaemonRunRequest) { restarted = req }

	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/rules", strings.NewReader(validRuleBody))
	rr := httptest.NewRecorder()

	h.CreateRule(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if restarted.Reader != "gmail" {
		t.Fatalf("restartFn reader = %q, want gmail", restarted.Reader)
	}
	if restarted.Tenant.ID != "tenant-a" {
		t.Fatalf("restartFn tenant = %q, want tenant-a", restarted.Tenant.ID)
	}
}

func TestCreateRule_MissingAmountRegex_Returns422(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	body := `{
		"name":"test",
		"sender_emails":["alerts@example.com"],
		"merchant_regex":".+",
		"source":{"type":"Credit Card","label":"Example Credit Card","bank":"Example Bank"}
	}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/rules", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.CreateRule(rr, req)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rr.Code)
	}
}

func TestCreateRule_InvalidAmountRegex_Returns422(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	body := `{
		"name":"test",
		"sender_emails":["alerts@example.com"],
		"amount_regex":"[invalid",
		"merchant_regex":".+",
		"source":{"type":"Credit Card","label":"Example Credit Card","bank":"Example Bank"}
	}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/rules", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.CreateRule(rr, req)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rr.Code)
	}
}

func TestUpdateRule_AnyRule_FullUpdate(t *testing.T) {
	ms := &mockStore{ruleResult: &store.RuleRow{ID: "1", Predefined: true}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	body := `{
		"name":"updated",
		"sender_emails":["alerts@example.com"],
		"amount_regex":"\\d+",
		"merchant_regex":".+",
		"source":{"type":"Credit Card","label":"Example Credit Card","bank":"Example Bank"}
	}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/rules/22222222-2222-2222-2222-222222222222", strings.NewReader(body))
	req.SetPathValue("id", testRuleID)
	rr := httptest.NewRecorder()
	h.UpdateRule(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestUpdateRule_DuplicateNameReturns409(t *testing.T) {
	ms := &mockStore{ruleErr: store.ErrRuleNameConflict}
	h := newTestHandlers(t, ms, &mockDaemon{})
	body := `{
		"name":"duplicate",
		"sender_emails":["alerts@example.com"],
		"amount_regex":"\\d+",
		"merchant_regex":".+",
		"source":{"type":"Credit Card","label":"Example Credit Card","bank":"Example Bank"}
	}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/rules/22222222-2222-2222-2222-222222222222", strings.NewReader(body))
	req.SetPathValue("id", testRuleID)
	rr := httptest.NewRecorder()

	h.UpdateRule(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["error"] != "rule name already exists" {
		t.Fatalf("error = %q, want rule name already exists", resp["error"])
	}
}

func TestUpdateRule_InvalidIDReturns400(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	body := `{
		"name":"updated",
		"sender_emails":["alerts@example.com"],
		"amount_regex":"\\d+",
		"merchant_regex":".+",
		"source":{"type":"Credit Card","label":"Example Credit Card","bank":"Example Bank"}
	}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/rules/not-a-uuid", strings.NewReader(body))
	req.SetPathValue("id", "not-a-uuid")
	rr := httptest.NewRecorder()

	h.UpdateRule(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestDeleteRule_PredefinedRule_Returns403(t *testing.T) {
	ms := &mockStore{ruleResult: &store.RuleRow{ID: "1", Predefined: true}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/rules/22222222-2222-2222-2222-222222222222", nil)
	req.SetPathValue("id", testRuleID)
	rr := httptest.NewRecorder()
	h.DeleteRule(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestDeleteRule_UserRule_Returns204(t *testing.T) {
	ms := &mockStore{ruleResult: &store.RuleRow{ID: "1", Predefined: false}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/rules/22222222-2222-2222-2222-222222222222", nil)
	req.SetPathValue("id", testRuleID)
	rr := httptest.NewRecorder()
	h.DeleteRule(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
}

func TestDeleteRule_InvalidIDReturns400(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/rules/not-a-uuid", nil)
	req.SetPathValue("id", "not-a-uuid")
	rr := httptest.NewRecorder()

	h.DeleteRule(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestExportRules_OnlyNonPredefinedRules(t *testing.T) {
	ms := &mockStore{rules: []store.RuleRow{
		{ID: "1", Name: "predefined", Predefined: true, AmountRegex: `\d+`, MerchantRegex: `.+`},
		{ID: "2", Name: "usr", Predefined: false, AmountRegex: `\d+`, MerchantRegex: `.+`},
	}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/rules/export", nil)
	rr := httptest.NewRecorder()
	h.ExportRules(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var exported struct {
		Rules []struct {
			Name string `json:"name"`
		} `json:"rules"`
	}
	decodeJSON(t, rr.Body.String(), &exported)
	if len(exported.Rules) != 1 {
		t.Errorf("expected 1 exported rule (user only), got %d", len(exported.Rules))
	}
	if exported.Rules[0].Name != "usr" {
		t.Errorf("expected exported name=usr, got %v", exported.Rules[0].Name)
	}
}

func TestExportRules_UsesVersionedDocument(t *testing.T) {
	ms := &mockStore{rules: []store.RuleRow{
		{ID: "1", Name: "predefined", Predefined: true, AmountRegex: `\d+`, MerchantRegex: `.+`},
		{
			ID:              "2",
			Name:            "HDFC Credit Card",
			SenderEmails:    []string{"alerts@hdfcbank.net", "alerts@hdfcbank.bank.in"},
			SubjectContains: "HDFC Credit Card",
			AmountRegex:     `Rs\.([\d.]+)`,
			MerchantRegex:   `at (.*?) on`,
			SourceType:      "Credit Card",
			SourceLabel:     "HDFC Credit Card",
			Bank:            "HDFC",
		},
	}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/rules/export", nil)
	rr := httptest.NewRecorder()

	h.ExportRules(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var exported struct {
		Version int `json:"version"`
		Rules   []struct {
			Name         string     `json:"name"`
			SenderEmails []string   `json:"sender_emails"`
			Source       api.Source `json:"source"`
		} `json:"rules"`
	}
	decodeJSON(t, rr.Body.String(), &exported)
	if exported.Version != 2 {
		t.Fatalf("version = %d, want 2", exported.Version)
	}
	if len(exported.Rules) != 1 {
		t.Fatalf("expected 1 exported user rule, got %d", len(exported.Rules))
	}
	if exported.Rules[0].Name != "HDFC Credit Card" {
		t.Fatalf("exported rule name = %q", exported.Rules[0].Name)
	}
	if want := []string{"alerts@hdfcbank.net", "alerts@hdfcbank.bank.in"}; !reflect.DeepEqual(want, exported.Rules[0].SenderEmails) {
		t.Fatalf("sender_emails = %#v, want %#v", exported.Rules[0].SenderEmails, want)
	}
}

func TestImportRules_AcceptsVersionedDocument(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	body := `{
		"version":2,
		"presets":{"source_types":[{"value":"Credit Card","origin":"predefined"}],"banks":[{"value":"HDFC","origin":"custom"}]},
		"rules":[{
			"name":"HDFC Credit Card",
			"sender_emails":["alerts@hdfcbank.net","alerts@hdfcbank.bank.in"],
			"subject_contains":"HDFC Credit Card",
			"amount_regex":"Rs\\.([\\d.]+)",
			"merchant_regex":"at (.*?) on",
			"source":{"type":"Credit Card","label":"HDFC Credit Card","bank":"HDFC"}
		}]
	}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/rules/import", strings.NewReader(body))
	rr := httptest.NewRecorder()

	h.ImportRules(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if len(ms.importedRules) != 1 {
		t.Fatalf("imported rules = %d, want 1", len(ms.importedRules))
	}
	got := ms.importedRules[0]
	if want := []string{"alerts@hdfcbank.net", "alerts@hdfcbank.bank.in"}; !reflect.DeepEqual(want, got.SenderEmails) {
		t.Fatalf("sender_emails = %#v, want %#v", got.SenderEmails, want)
	}
	if got.SourceType != "Credit Card" || got.SourceLabel != "HDFC Credit Card" || got.Bank != "HDFC" {
		t.Fatalf("source fields = (%q, %q, %q)", got.SourceType, got.SourceLabel, got.Bank)
	}
}

func TestImportRules_InvalidRegex_Returns422(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	body := `[{"name":"bad","amountRegex":"[invalid","merchantInfoRegex":".+"}]`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/rules/import", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.ImportRules(rr, req)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

// --- thunderbird discovery + guide ---

func TestDiscoverProfiles_Returns200WithProfilesKey(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/providers/thunderbird/discover/profiles", nil)
	rr := httptest.NewRecorder()
	h.DiscoverProfiles(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string][]string
	decodeJSON(t, rr.Body.String(), &resp)
	if _, ok := resp["profiles"]; !ok {
		t.Error("expected 'profiles' key in response")
	}
}

func TestDiscoverMailboxes_MissingParam_Returns400(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/providers/thunderbird/discover/mailboxes", nil)
	rr := httptest.NewRecorder()
	h.DiscoverMailboxes(rr, req)

	assertValidationError(t, rr, "profile", "query", "is required")
}

func TestDiscoverMailboxes_NonexistentProfile_Returns404(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodGet,
		"/api/providers/thunderbird/discover/mailboxes?profile=/nonexistent/thunderbird/profile",
		nil,
	)
	rr := httptest.NewRecorder()
	h.DiscoverMailboxes(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestGetProviderGuide_NoGuide_Returns404(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	registry := plugins.NewRegistry()
	if err := registry.RegisterProvider((&testProvider{name: "noguide", authType: plugins.AuthTypeConfig}).provider()); err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}
	h.registry = registry

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/providers/noguide/guide", nil)
	req.SetPathValue("name", "noguide")
	rr := httptest.NewRecorder()

	h.GetProviderGuide(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestGetProviderGuide_ReturnsMetadataGuide(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	registry := plugins.NewRegistry()
	guide := json.RawMessage(`{"sections":[{"title":"Setup","steps":[{"text":"Do the setup"}]}]}`)
	if err := registry.RegisterProvider((&testProvider{name: "guided", authType: plugins.AuthTypeConfig, guide: guide}).provider()); err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}
	h.registry = registry

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/providers/guided/guide", nil)
	req.SetPathValue("name", "guided")
	rr := httptest.NewRecorder()

	h.GetProviderGuide(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Do the setup") {
		t.Fatalf("body = %s, want setup guide", rr.Body.String())
	}
}

// --- rescan ---

func TestStartDaemon_DaemonRunning_CallsStartFnWithRequestedReader(t *testing.T) {
	var started DaemonRunRequest
	ms := &mockStore{}
	dm := &mockDaemon{status: DaemonStatus{Running: true}}
	h := newTestHandlers(t, ms, dm)
	h.startFn = func(req DaemonRunRequest) { started = req }

	body := `{"reader":"thunderbird"}`
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/daemon/start", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.StartDaemon(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if started.Reader != "thunderbird" {
		t.Fatalf("startFn reader = %q, want thunderbird", started.Reader)
	}
	if started.Tenant.ID != "tenant-a" {
		t.Fatalf("startFn tenant = %q, want tenant-a", started.Tenant.ID)
	}
}

func TestStartDaemon_RejectsMissingReader(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	h.startFn = func(DaemonRunRequest) {}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/daemon/start", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	h.StartDaemon(rr, req)

	assertValidationError(t, rr, "reader", "body", "is required")
}

func TestRescan_DaemonRunning_Returns202Rescanning(t *testing.T) {
	var rescan DaemonRunRequest
	ms := &mockStore{}
	dm := &mockDaemon{status: DaemonStatus{Running: true}}
	h := newTestHandlers(t, ms, dm)
	h.rescanFn = func(req DaemonRunRequest) { rescan = req }

	body := `{"reader":"gmail"}`
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/daemon/rescan", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.Rescan(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["status"] != "rescanning" {
		t.Errorf("expected status=rescanning, got %q", resp["status"])
	}
	if rescan.Reader != "gmail" {
		t.Fatalf("rescanFn reader = %q, want gmail", rescan.Reader)
	}
	if rescan.Tenant.ID != "tenant-a" {
		t.Fatalf("rescanFn tenant = %q, want tenant-a", rescan.Tenant.ID)
	}
}

func TestRescan_DaemonNotRunning_Returns202Rescanning(t *testing.T) {
	var rescan DaemonRunRequest
	ms := &mockStore{}
	dm := &mockDaemon{status: DaemonStatus{Running: false}}
	h := newTestHandlers(t, ms, dm)
	h.rescanFn = func(req DaemonRunRequest) { rescan = req }

	body := `{"reader":"gmail"}`
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/daemon/rescan", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.Rescan(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["status"] != "rescanning" {
		t.Errorf("expected status=rescanning, got %q", resp["status"])
	}
	if rescan.Reader != "gmail" {
		t.Fatalf("rescanFn reader = %q, want gmail", rescan.Reader)
	}
	if rescan.Tenant.ID != "tenant-a" {
		t.Fatalf("rescanFn tenant = %q, want tenant-a", rescan.Tenant.ID)
	}
}

// --- AuthExchange ---

func TestAuthExchange_MissingURL_Returns400(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/providers/gmail/auth/exchange", strings.NewReader(`{}`))
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.AuthExchange(rr, req)

	assertValidationError(t, rr, "url", "body", "is required")
}

func TestAuthExchange_MalformedURL_Returns400(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/providers/gmail/auth/exchange", strings.NewReader(`{"url":":::not-a-url"}`))
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.AuthExchange(rr, req)

	assertValidationError(t, rr, "url", "body", "must be a valid URL")
}

func TestAuthExchange_MissingCode_Returns400(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	body := `{"url":"http://localhost:8080/api/auth/callback?state=somestate"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/providers/gmail/auth/exchange", strings.NewReader(body))
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.AuthExchange(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing code, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestAuthExchange_MissingState_Returns400(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	body := `{"url":"http://localhost:8080/api/auth/callback?code=4%2F0Acode"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/providers/gmail/auth/exchange", strings.NewReader(body))
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.AuthExchange(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing state, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestAuthExchange_UnknownState_Returns400(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	body := `{"url":"http://localhost:8080/api/auth/callback?code=4%2F0Acode&state=doesnotexist"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/providers/gmail/auth/exchange", strings.NewReader(body))
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.AuthExchange(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown state, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestAuthExchange_ExpiredState_Returns400(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})

	expiredState := "reader:gmail:expiredtoken"
	h.mu.Lock()
	h.oauthStates[expiredState] = oauthStateEntry{
		readerName: "gmail",
		expiresAt:  time.Now().Add(-1 * time.Second),
	}
	h.mu.Unlock()

	body := `{"url":"http://localhost:8080/api/auth/callback?code=4%2F0Acode&state=` + expiredState + `"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/providers/gmail/auth/exchange", strings.NewReader(body))
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.AuthExchange(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for expired state, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestAuthExchange_RejectsStateFromDifferentTenant(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("token endpoint method = %s, want POST", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"new-access","refresh_token":"new-refresh","token_type":"Bearer","expires_in":3600}`)
	}))
	defer tokenServer.Close()

	secretJSON := fmt.Sprintf(`{
		"installed": {
			"client_id": "test-client-id.apps.googleusercontent.com",
			"client_secret": "test-client-secret",
			"auth_uri": "https://accounts.google.com/o/oauth2/auth",
			"token_uri": %q,
			"redirect_uris": ["http://localhost:8080/api/auth/callback"]
		}
	}`, tokenServer.URL)
	h := newTestHandlers(t, &mockStore{
		readerSecrets: map[string][]byte{"tenant-a/gmail": []byte(secretJSON)},
	}, &mockDaemon{})

	state := "reader:gmail:tenant-a"
	h.mu.Lock()
	h.oauthStates[state] = oauthStateEntry{
		readerName: "gmail",
		tenant:     store.Tenant{ID: "tenant-a"},
		expiresAt:  time.Now().Add(time.Minute),
	}
	h.mu.Unlock()

	body := `{"url":"http://localhost:8080/api/auth/callback?code=4%2F0Acode&state=` + state + `"}`
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-b", TenantID: "tenant-b", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/providers/gmail/auth/exchange", strings.NewReader(body))
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.AuthExchange(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for tenant mismatch, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestAuthExchange_RestartsRunningDaemonAfterTokenSaved(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("token endpoint method = %s, want POST", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"new-access","refresh_token":"new-refresh","token_type":"Bearer","expires_in":3600}`)
	}))
	defer tokenServer.Close()

	secretJSON := fmt.Sprintf(`{
		"installed": {
			"client_id": "test-client-id.apps.googleusercontent.com",
			"client_secret": "test-client-secret",
			"auth_uri": "https://accounts.google.com/o/oauth2/auth",
			"token_uri": %q,
			"redirect_uris": ["http://localhost:8080/api/auth/callback"]
		}
	}`, tokenServer.URL)
	st := &mockStore{readerSecrets: map[string][]byte{"tenant-a/gmail": []byte(secretJSON)}}
	dm := &mockDaemon{status: DaemonStatus{Running: true}}
	h := newTestHandlers(t, st, dm)
	var restarted DaemonRunRequest
	h.restartFn = func(req DaemonRunRequest) { restarted = req }

	state := "reader:gmail:validtoken"
	h.mu.Lock()
	h.oauthStates[state] = oauthStateEntry{
		readerName: "gmail",
		tenant:     store.Tenant{ID: "tenant-a"},
		expiresAt:  time.Now().Add(time.Minute),
	}
	h.mu.Unlock()

	body := `{"url":"http://localhost:8080/api/auth/callback?code=4%2F0Acode&state=` + state + `"}`
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/providers/gmail/auth/exchange", strings.NewReader(body))
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.AuthExchange(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if restarted.Reader != "gmail" {
		t.Fatalf("restartFn reader = %q, want gmail", restarted.Reader)
	}
	if restarted.Tenant.ID != "tenant-a" {
		t.Fatalf("restartFn tenant = %q, want tenant-a", restarted.Tenant.ID)
	}
	if !strings.Contains(string(st.readerTokens["tenant-a/gmail"]), "new-refresh") {
		t.Fatalf("saved token = %s, want refresh token from re-grant", st.readerTokens["tenant-a/gmail"])
	}
}

func TestListTransactions_BucketParam(t *testing.T) {
	now := time.Now()
	st := &mockStore{
		transactions: []store.Transaction{
			{
				ID: "w1", Amount: 100, Currency: "INR", MerchantInfo: "Netflix",
				Bucket: "wants", Timestamp: now, Labels: []string{},
			},
		},
		listResult: store.TransactionListResult{Total: 1},
	}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.ListTransactions, "/api/transactions?bucket=wants")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["total"] != float64(1) {
		t.Errorf("expected total=1, got %v", resp["total"])
	}
}

func TestListTransactions_WeekdayHourTimezoneParams(t *testing.T) {
	st := &mockStore{transactions: []store.Transaction{}, listResult: store.TransactionListResult{Total: 0}}
	h := newTestHandlers(t, st, &mockDaemon{})

	rr := get(h.ListTransactions, "/api/transactions?weekday=5&hour_from=9&hour_to=9&tz=Asia/Kolkata")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if st.listFilter.Weekday == nil || *st.listFilter.Weekday != 5 {
		t.Fatalf("expected weekday filter 5, got %#v", st.listFilter.Weekday)
	}
	if st.listFilter.HourFrom == nil || *st.listFilter.HourFrom != 9 {
		t.Fatalf("expected hour_from 9, got %#v", st.listFilter.HourFrom)
	}
	if st.listFilter.HourTo == nil || *st.listFilter.HourTo != 9 {
		t.Fatalf("expected hour_to 9, got %#v", st.listFilter.HourTo)
	}
	if st.listFilter.Timezone != "Asia/Kolkata" {
		t.Fatalf("expected timezone Asia/Kolkata, got %q", st.listFilter.Timezone)
	}
}

func TestListTransactions_WeekdayHourTimezoneAndDateParams(t *testing.T) {
	st := &mockStore{transactions: []store.Transaction{}, listResult: store.TransactionListResult{Total: 0}}
	h := newTestHandlers(t, st, &mockDaemon{})

	rr := get(
		h.ListTransactions,
		"/api/transactions?weekday=0&hour_from=23&hour_to=23&tz=Asia/Calcutta&date_from=2026-04-01T00:00:00.000Z&date_to=2026-04-30T23:59:59.000Z",
	)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if st.listFilter.Weekday == nil || *st.listFilter.Weekday != 0 {
		t.Fatalf("expected weekday filter 0, got %#v", st.listFilter.Weekday)
	}
	if st.listFilter.HourFrom == nil || *st.listFilter.HourFrom != 23 {
		t.Fatalf("expected hour_from 23, got %#v", st.listFilter.HourFrom)
	}
	if st.listFilter.HourTo == nil || *st.listFilter.HourTo != 23 {
		t.Fatalf("expected hour_to 23, got %#v", st.listFilter.HourTo)
	}
	if st.listFilter.Timezone != "Asia/Calcutta" {
		t.Fatalf("expected timezone Asia/Calcutta, got %q", st.listFilter.Timezone)
	}
	if st.listFilter.From == nil || st.listFilter.From.Format(time.RFC3339) != "2026-04-01T00:00:00Z" {
		t.Fatalf("expected date_from to parse, got %#v", st.listFilter.From)
	}
	if st.listFilter.To == nil || st.listFilter.To.Format(time.RFC3339) != "2026-04-30T23:59:59Z" {
		t.Fatalf("expected date_to to parse, got %#v", st.listFilter.To)
	}
}

func TestListTransactions_ExcludeParams(t *testing.T) {
	st := &mockStore{transactions: []store.Transaction{}, listResult: store.TransactionListResult{Total: 0}}
	h := newTestHandlers(t, st, &mockDaemon{})

	rr := get(
		h.ListTransactions,
		"/api/transactions?exclude_categories=Shopping,Food%20%26%20Dining&exclude_labels=Top,Recurring&exclude_buckets=Needs,Wants&exclude_sources=HDFC,Amex",
	)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if want := []string{"Shopping", "Food & Dining"}; !reflect.DeepEqual(want, st.listFilter.ExcludeCategories) {
		t.Fatalf("expected exclude_categories %v, got %v", want, st.listFilter.ExcludeCategories)
	}
	if want := []string{"Top", "Recurring"}; !reflect.DeepEqual(want, st.listFilter.ExcludeLabels) {
		t.Fatalf("expected exclude_labels %v, got %v", want, st.listFilter.ExcludeLabels)
	}
	if want := []string{"Needs", "Wants"}; !reflect.DeepEqual(want, st.listFilter.ExcludeBuckets) {
		t.Fatalf("expected exclude_buckets %v, got %v", want, st.listFilter.ExcludeBuckets)
	}
	if want := []string{"HDFC", "Amex"}; !reflect.DeepEqual(want, st.listFilter.ExcludeSources) {
		t.Fatalf("expected exclude_sources %v, got %v", want, st.listFilter.ExcludeSources)
	}
}

func TestListTransactions_StructuredSourceParams(t *testing.T) {
	st := &mockStore{transactions: []store.Transaction{}, listResult: store.TransactionListResult{Total: 0}}
	h := newTestHandlers(t, st, &mockDaemon{})

	rr := get(
		h.ListTransactions,
		"/api/transactions?source_type=Credit%20Card&bank=HDFC&exclude_source_types=UPI,NetBanking&exclude_banks=ICICI,SBI",
	)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if st.listFilter.SourceType != "Credit Card" {
		t.Fatalf("source_type = %q, want Credit Card", st.listFilter.SourceType)
	}
	if st.listFilter.Bank != "HDFC" {
		t.Fatalf("bank = %q, want HDFC", st.listFilter.Bank)
	}
	if want := []string{"UPI", "NetBanking"}; !reflect.DeepEqual(want, st.listFilter.ExcludeSourceTypes) {
		t.Fatalf("exclude_source_types = %#v, want %#v", st.listFilter.ExcludeSourceTypes, want)
	}
	if want := []string{"ICICI", "SBI"}; !reflect.DeepEqual(want, st.listFilter.ExcludeBanks) {
		t.Fatalf("exclude_banks = %#v, want %#v", st.listFilter.ExcludeBanks, want)
	}
}

func TestListTransactions_MissingTaxonomyParams(t *testing.T) {
	st := &mockStore{transactions: []store.Transaction{}, listResult: store.TransactionListResult{Total: 0}}
	h := newTestHandlers(t, st, &mockDaemon{})

	rr := get(
		h.ListTransactions,
		"/api/transactions?category_missing=1&bucket_missing=1&label_missing=1",
	)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !st.listFilter.CategoryMissing {
		t.Fatal("expected category_missing=1 to set ListFilter.CategoryMissing")
	}
	if !st.listFilter.BucketMissing {
		t.Fatal("expected bucket_missing=1 to set ListFilter.BucketMissing")
	}
	if !st.listFilter.LabelMissing {
		t.Fatal("expected label_missing=1 to set ListFilter.LabelMissing")
	}
}

func TestListTransactions_MissingTimezoneFallsBackToAppTimezone(t *testing.T) {
	st := &mockStore{
		transactions: []store.Transaction{},
		listResult:   store.TransactionListResult{Total: 0},
		appConfigByTenant: map[string]map[string]string{
			"tenant-a": {"app.timezone": "Asia/Kolkata"},
		},
	}
	h := newTestHandlers(t, st, &mockDaemon{})

	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/transactions?weekday=5&hour_from=9&hour_to=9", nil)
	rr := httptest.NewRecorder()
	h.ListTransactions(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if st.listFilter.Timezone != "Asia/Kolkata" {
		t.Fatalf("expected fallback timezone Asia/Kolkata, got %q", st.listFilter.Timezone)
	}
}

// --- banks ---

func TestListBanks(t *testing.T) {
	banksJSON := []byte(`[{"fragment":"hdfc","color":"#E31837","name":"HDFC Bank"}]`)

	tests := []struct {
		name       string
		banksData  []byte
		wantStatus int
		wantBody   string
	}{
		{
			name:       "returns banks JSON when populated",
			banksData:  banksJSON,
			wantStatus: http.StatusOK,
			wantBody:   string(banksJSON),
		},
		{
			name:       "returns empty array when no banks data",
			banksData:  nil,
			wantStatus: http.StatusOK,
			wantBody:   "[]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHandlers(t, &mockStore{}, &mockDaemon{}, tc.banksData)
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/config/banks", nil)
			w := httptest.NewRecorder()
			h.ListBanks(w, req)
			if w.Code != tc.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tc.wantStatus)
			}
			if ct := w.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("got Content-Type %q, want application/json", ct)
			}
			if got := strings.TrimSpace(w.Body.String()); got != tc.wantBody {
				t.Errorf("got body %q, want %q", got, tc.wantBody)
			}
		})
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

func TestGetCommunitySyncSettingsDefaultsAutomaticSyncEnabled(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/config/sync/settings", nil)
	rr := httptest.NewRecorder()
	h.GetCommunitySyncSettings(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rr.Code, rr.Body.String())
	}
	var resp CommunitySyncSettingsResponse
	decodeJSON(t, rr.Body.String(), &resp)
	if !resp.AutomaticSyncEnabled {
		t.Fatalf("automatic_sync_enabled = false, want true")
	}
}

func TestPatchCommunitySyncSettingsPersistsAutomaticSyncEnabled(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPatch,
		"/api/config/sync/settings",
		strings.NewReader(`{"automatic_sync_enabled":false}`),
	)
	rr := httptest.NewRecorder()
	h.PatchCommunitySyncSettings(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rr.Code, rr.Body.String())
	}
	if ms.communitySyncSettingsPatch.AutomaticSyncEnabled == nil || *ms.communitySyncSettingsPatch.AutomaticSyncEnabled {
		t.Fatalf("stored patch = %#v, want automatic_sync_enabled=false", ms.communitySyncSettingsPatch)
	}
	var resp CommunitySyncSettingsResponse
	decodeJSON(t, rr.Body.String(), &resp)
	if resp.AutomaticSyncEnabled {
		t.Fatalf("automatic_sync_enabled = true, want false")
	}
}

func TestCategorizeMerchant_OK(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	body := `{"merchant":"Netflix","category":"Entertainment","bucket":"Wants"}`
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/merchants/categorize", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CategorizeMerchant(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]int
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp["updated"] != 3 {
		t.Errorf("want updated=3, got %d", resp["updated"])
	}
}

func TestCategorizeMerchant_EmptyMerchant(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	body := `{"merchant":"","category":"Entertainment","bucket":"Wants"}`
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/merchants/categorize", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CategorizeMerchant(w, r)
	assertValidationError(t, w, "merchant", "body", "is required")
}

func TestCategorizeMerchant_InvalidJSON(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/merchants/categorize", strings.NewReader("not-json"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CategorizeMerchant(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestCategorizeMerchant_StoreError(t *testing.T) {
	h := newTestHandlers(t, &mockStore{updateErr: errors.New("db down")}, &mockDaemon{})
	body := `{"merchant":"Netflix","category":"Entertainment","bucket":"Wants"}`
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/merchants/categorize", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CategorizeMerchant(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}
}

func TestUpdateMerchantReason_InvalidID(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/muted-merchants/not-a-uuid", strings.NewReader(`{"reason":"x"}`))
	req.SetPathValue("id", "not-a-uuid")
	rr := httptest.NewRecorder()
	h.UpdateMerchantReason(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestUpdateMerchantReason_Success(t *testing.T) {
	st := &mockStore{}
	h := newTestHandlers(t, st, &mockDaemon{})
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPatch,
		"/api/muted-merchants/"+testTransactionID,
		strings.NewReader(`{"reason":"subscription"}`),
	)
	req.SetPathValue("id", testTransactionID)
	rr := httptest.NewRecorder()
	h.UpdateMerchantReason(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if st.updateMerchantID != testTransactionID || st.updateMerchantReason != "subscription" {
		t.Fatalf("merchant reason call = id=%q reason=%q", st.updateMerchantID, st.updateMerchantReason)
	}
}

func TestDeleteMutedMerchant_InvalidID(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/muted-merchants/not-a-uuid", nil)
	req.SetPathValue("id", "not-a-uuid")
	rr := httptest.NewRecorder()
	h.DeleteMutedMerchant(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestDeleteMutedMerchant_RejectsInvalidUnmuteFlag(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodDelete,
		"/api/muted-merchants/"+testTransactionID+"?unmute=sometimes",
		nil,
	)
	req.SetPathValue("id", testTransactionID)
	rr := httptest.NewRecorder()
	h.DeleteMutedMerchant(rr, req)

	assertValidationError(t, rr, "unmute", "query", "must be a boolean")
}
