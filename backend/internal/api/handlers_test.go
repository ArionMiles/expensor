package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/internal/store"
	pkgapi "github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	pkgstate "github.com/ArionMiles/expensor/backend/pkg/state"
)

// --- mocks ---

type mockDaemon struct {
	status DaemonStatus
}

func (m *mockDaemon) Status() DaemonStatus { return m.status }

type mockStore struct {
	transactions          []store.Transaction
	listResult            store.TransactionListResult
	listErr               error
	listFilter            store.ListFilter
	getResult             *store.Transaction
	getErr                error
	updateErr             error
	addLabelErr           error
	addLabelsErr          error
	removeLblErr          error
	searchResult          []store.Transaction
	searchListResult      store.TransactionListResult
	searchErr             error
	searchFilter          store.ListFilter
	stats                 *store.Stats
	statsErr              error
	dashboardData         *store.DashboardData
	dashboardErr          error
	appConfig             map[string]string
	setConfigErr          error
	processedMessages     map[string]time.Time
	activeReader          string
	readerSecrets         map[string][]byte
	readerTokens          map[string][]byte
	readerConfigs         map[string]json.RawMessage
	getFacetsErr          error
	facets                *store.Facets
	labels                []store.Label
	labelsErr             error
	deleteLabelCleanup    bool
	categoryMappings      map[string][]string
	categories            []store.Category
	catsErr               error
	deleteCategoryCleanup bool
	bucketMappings        map[string][]string
	buckets               []store.Bucket
	bucketsErr            error
	deleteBucketCleanup   bool
	updateTxErr           error
	rules                 []store.RuleRow
	rulesErr              error
	ruleResult            *store.RuleRow
	ruleErr               error
	importErr             error
	heatmapData           *store.HeatmapData
	heatmapErr            error
	annualData            []store.DailyBucket
	annualErr             error
	monthlyBreakdown      *store.MonthlyBreakdownData
	monthlyBreakdownErr   error
	categorizeMerchantN   int
	diagnostics           []store.ExtractionDiagnosticRow
	diagnosticFilter      store.DiagnosticFilter
	diagnosticResult      *store.ExtractionDiagnosticRow
	diagnosticErr         error
	updateDiagnosticID    string
	updateDiagnosticStat  string
	syncStatus            store.SyncStatus
	syncStatusErr         error
}

func (m *mockStore) ListTransactions(
	_ context.Context,
	f store.ListFilter,
) ([]store.Transaction, store.TransactionListResult, error) {
	m.listFilter = f
	if m.listErr != nil {
		return nil, store.TransactionListResult{}, m.listErr
	}
	return m.transactions, m.listResult, nil
}

func (m *mockStore) GetTransaction(_ context.Context, _ string) (*store.Transaction, error) {
	return m.getResult, m.getErr
}

func (m *mockStore) UpdateDescription(_ context.Context, _, _ string) error {
	return m.updateErr
}

func (m *mockStore) AddLabel(_ context.Context, _, _ string) error {
	return m.addLabelErr
}

func (m *mockStore) AddLabels(_ context.Context, _ string, _ []string) error {
	return m.addLabelsErr
}

func (m *mockStore) RemoveLabel(_ context.Context, _, _ string) error {
	return m.removeLblErr
}

func (m *mockStore) SearchTransactions(
	_ context.Context,
	_ string,
	f store.ListFilter,
) ([]store.Transaction, store.TransactionListResult, error) {
	m.searchFilter = f
	if m.searchErr != nil {
		return nil, store.TransactionListResult{}, m.searchErr
	}
	return m.searchResult, m.searchListResult, nil
}

func (m *mockStore) GetStats(_ context.Context, _ string) (*store.Stats, error) {
	return m.stats, m.statsErr
}

func (m *mockStore) GetChartData(_ context.Context) (*store.ChartData, error) {
	return &store.ChartData{
		MonthlySpend: []store.TimeBucket{},
		DailySpend:   []store.TimeBucket{},
		ByCategory:   map[string]float64{},
		ByBucket:     map[string]float64{},
		ByLabel:      map[string]float64{},
		BySource:     map[string]float64{},
	}, nil
}

func (m *mockStore) GetDashboardData(_ context.Context) (*store.DashboardData, error) {
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

func (m *mockStore) GetAppConfig(_ context.Context, key string) (string, error) {
	if m.appConfig != nil {
		if v, ok := m.appConfig[key]; ok {
			return v, nil
		}
	}
	return "", errors.New("not found")
}

func (m *mockStore) SetAppConfig(_ context.Context, key, value string) error {
	if m.setConfigErr != nil {
		return m.setConfigErr
	}
	if m.appConfig == nil {
		m.appConfig = make(map[string]string)
	}
	m.appConfig[key] = value
	return nil
}

func (m *mockStore) IsMessageProcessed(_ context.Context, key string) (bool, error) {
	_, ok := m.processedMessages[key]
	return ok, nil
}

func (m *mockStore) MarkMessageProcessed(_ context.Context, key string, at time.Time) error {
	if m.processedMessages == nil {
		m.processedMessages = make(map[string]time.Time)
	}
	m.processedMessages[key] = at
	return nil
}

func (m *mockStore) SetActiveReader(_ context.Context, reader string) error {
	m.activeReader = reader
	return nil
}

func (m *mockStore) GetActiveReader(_ context.Context) (string, error) {
	return m.activeReader, nil
}

func (m *mockStore) SetReaderSecret(_ context.Context, reader string, secret []byte) error {
	if m.readerSecrets == nil {
		m.readerSecrets = make(map[string][]byte)
	}
	m.readerSecrets[reader] = append([]byte(nil), secret...)
	return nil
}

func (m *mockStore) GetReaderSecret(_ context.Context, reader string) (secret []byte, found bool, err error) {
	secret, ok := m.readerSecrets[reader]
	return append([]byte(nil), secret...), ok, nil
}

func (m *mockStore) SetReaderToken(_ context.Context, reader string, token []byte) error {
	if m.readerTokens == nil {
		m.readerTokens = make(map[string][]byte)
	}
	m.readerTokens[reader] = append([]byte(nil), token...)
	return nil
}

func (m *mockStore) GetReaderToken(_ context.Context, reader string) (token []byte, found bool, err error) {
	token, ok := m.readerTokens[reader]
	return append([]byte(nil), token...), ok, nil
}

func (m *mockStore) DeleteReaderToken(_ context.Context, reader string) error {
	delete(m.readerTokens, reader)
	return nil
}

func (m *mockStore) SetReaderConfig(_ context.Context, reader string, readerConfig json.RawMessage) error {
	if m.readerConfigs == nil {
		m.readerConfigs = make(map[string]json.RawMessage)
	}
	m.readerConfigs[reader] = append(json.RawMessage(nil), readerConfig...)
	return nil
}

func (m *mockStore) GetReaderConfig(_ context.Context, reader string) (json.RawMessage, bool, error) {
	cfg, ok := m.readerConfigs[reader]
	return append(json.RawMessage(nil), cfg...), ok, nil
}

func (m *mockStore) DeleteReaderRuntime(_ context.Context, reader string) error {
	delete(m.readerSecrets, reader)
	delete(m.readerTokens, reader)
	delete(m.readerConfigs, reader)
	return nil
}

func (m *mockStore) GetFacets(_ context.Context) (*store.Facets, error) {
	if m.getFacetsErr != nil {
		return nil, m.getFacetsErr
	}
	if m.facets != nil {
		return m.facets, nil
	}
	return &store.Facets{
		Sources:     []string{},
		Categories:  []string{},
		Currencies:  []string{},
		Merchants:   []string{},
		Labels:      []string{},
		LabelCounts: map[string]int{},
		Buckets:     []string{},
	}, nil
}

func (m *mockStore) ListLabels(_ context.Context) ([]store.Label, error) {
	if m.labelsErr != nil {
		return nil, m.labelsErr
	}
	if m.labels == nil {
		return []store.Label{}, nil
	}
	return m.labels, nil
}

func (m *mockStore) CreateLabel(_ context.Context, _, _ string) error { return m.labelsErr }

func (m *mockStore) UpdateLabel(_ context.Context, _, _ string) error { return m.updateErr }

func (m *mockStore) DeleteLabel(_ context.Context, _ string, removeFromTransactions bool) error {
	m.deleteLabelCleanup = removeFromTransactions
	return m.labelsErr
}

func (m *mockStore) RemoveLabelByMerchant(_ context.Context, _, _ string) (int64, error) {
	return 0, nil
}

func (m *mockStore) ApplyLabelByMerchant(_ context.Context, _, _ string) (int64, error) {
	if m.labelsErr != nil {
		return 0, m.labelsErr
	}
	return 0, nil
}

func (m *mockStore) GetLabelMappings(_ context.Context) (map[string][]string, error) {
	return map[string][]string{}, nil
}

func (m *mockStore) GetMonthlyBreakdownSpend(_ context.Context, _ string, _ int) (*store.MonthlyBreakdownData, error) {
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

func (m *mockStore) ListCategories(_ context.Context) ([]store.Category, error) {
	if m.catsErr != nil {
		return nil, m.catsErr
	}
	if m.categories == nil {
		return []store.Category{}, nil
	}
	return m.categories, nil
}

func (m *mockStore) CreateCategory(_ context.Context, _, _ string) error { return m.catsErr }

func (m *mockStore) DeleteCategory(_ context.Context, _ string, removeFromTransactions bool) error {
	m.deleteCategoryCleanup = removeFromTransactions
	return m.catsErr
}

func (m *mockStore) GetCategoryMappings(_ context.Context) (map[string][]string, error) {
	if m.categoryMappings != nil {
		return m.categoryMappings, nil
	}
	return map[string][]string{}, nil
}

func (m *mockStore) ApplyCategoryByMerchant(_ context.Context, _, _ string) (int64, error) {
	if m.catsErr != nil {
		return 0, m.catsErr
	}
	return 2, nil
}

func (m *mockStore) RemoveCategoryByMerchant(_ context.Context, _, _ string) (int64, error) {
	if m.catsErr != nil {
		return 0, m.catsErr
	}
	return 1, nil
}

func (m *mockStore) ListBuckets(_ context.Context) ([]store.Bucket, error) {
	if m.bucketsErr != nil {
		return nil, m.bucketsErr
	}
	if m.buckets == nil {
		return []store.Bucket{}, nil
	}
	return m.buckets, nil
}

func (m *mockStore) CreateBucket(_ context.Context, _, _ string) error { return m.bucketsErr }

func (m *mockStore) DeleteBucket(_ context.Context, _ string, removeFromTransactions bool) error {
	m.deleteBucketCleanup = removeFromTransactions
	return m.bucketsErr
}

func (m *mockStore) GetBucketMappings(_ context.Context) (map[string][]string, error) {
	if m.bucketMappings != nil {
		return m.bucketMappings, nil
	}
	return map[string][]string{}, nil
}

func (m *mockStore) ApplyBucketByMerchant(_ context.Context, _, _ string) (int64, error) {
	if m.bucketsErr != nil {
		return 0, m.bucketsErr
	}
	return 3, nil
}

func (m *mockStore) RemoveBucketByMerchant(_ context.Context, _, _ string) (int64, error) {
	if m.bucketsErr != nil {
		return 0, m.bucketsErr
	}
	return 1, nil
}

func (m *mockStore) UpdateTransaction(_ context.Context, _ string, _ store.TransactionUpdate) error {
	return m.updateTxErr
}

func (m *mockStore) ListRules(_ context.Context) ([]store.RuleRow, error) {
	if m.rulesErr != nil {
		return nil, m.rulesErr
	}
	if m.rules != nil {
		return m.rules, nil
	}
	return []store.RuleRow{}, nil
}

func (m *mockStore) GetRule(_ context.Context, _ string) (*store.RuleRow, error) {
	return m.ruleResult, m.ruleErr
}

func (m *mockStore) CreateRule(_ context.Context, r store.RuleRow) (*store.RuleRow, error) {
	if m.ruleErr != nil {
		return nil, m.ruleErr
	}
	r.ID = "new-id"
	return &r, nil
}

func (m *mockStore) UpdateRule(_ context.Context, _ string, r store.RuleRow) (*store.RuleRow, error) {
	if m.ruleErr != nil {
		return nil, m.ruleErr
	}
	return &r, nil
}

func (m *mockStore) DeleteRule(_ context.Context, _ string) error {
	return m.ruleErr
}

func (m *mockStore) SeedPredefinedRules(_ context.Context, _ []store.RuleRow) error {
	return nil
}

func (m *mockStore) ImportUserRules(_ context.Context, _ []store.RuleRow) error {
	return m.importErr
}

func (m *mockStore) MuteTransaction(_ context.Context, _ string, _ bool, _ string) error { return nil }
func (m *mockStore) UpdateMuteReason(_ context.Context, _, _ string) error               { return nil }
func (m *mockStore) MuteByMerchant(_ context.Context, _, _ string) error                 { return nil }
func (m *mockStore) UpdateMerchantReason(_ context.Context, _, _ string) error           { return nil }
func (m *mockStore) ListMutedMerchants(_ context.Context) ([]store.MutedMerchant, error) {
	return []store.MutedMerchant{}, nil
}

func (m *mockStore) GetMutedMerchantsWithCount(_ context.Context) ([]store.MutedMerchantWithCount, error) {
	return []store.MutedMerchantWithCount{}, nil
}
func (m *mockStore) DeleteMutedMerchant(_ context.Context, _ string) error          { return nil }
func (m *mockStore) UnmuteByPattern(_ context.Context, _ string) error              { return nil }
func (m *mockStore) DeleteMutedMerchantAndUnmute(_ context.Context, _ string) error { return nil }
func (m *mockStore) CategorizeMerchant(_ context.Context, _, _, _ string) (int, error) {
	if m.categorizeMerchantN != 0 {
		return m.categorizeMerchantN, m.updateErr
	}
	return 3, m.updateErr
}

func (m *mockStore) GetSpendingHeatmap(_ context.Context, _, _ *time.Time) (*store.HeatmapData, error) {
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

func (m *mockStore) GetAnnualSpend(_ context.Context, _ int) ([]store.DailyBucket, error) {
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

func (m *mockStore) GetExtractionDiagnostic(_ context.Context, _ string) (*store.ExtractionDiagnosticRow, error) {
	return m.diagnosticResult, m.diagnosticErr
}

func (m *mockStore) UpdateExtractionDiagnosticStatus(
	_ context.Context,
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
	_ = registry.RegisterReader(&testReaderPlugin{name: "gmail", authType: plugins.AuthTypeOAuth, requiresCreds: true})
	_ = registry.RegisterReader(&testReaderPlugin{name: "thunderbird", authType: plugins.AuthTypeConfig, requiresCreds: false, schema: []plugins.ConfigField{
		{Key: "profilePath", Label: "Profile Directory", Type: "path", Required: true},
	}})
	_ = registry.RegisterWriter(&testWriterPlugin{name: "postgres"})
	var banks []byte
	if len(banksData) > 0 {
		banks = banksData[0]
	}
	return NewHandlers(HandlersConfig{
		Registry:     registry,
		Store:        st,
		Daemon:       dm,
		Version:      "test",
		BaseURL:      "http://localhost:8080",
		FrontendURL:  "http://localhost:5173",
		DataDir:      t.TempDir(),
		ScanInterval: 60,
		LookbackDays: 180,
		BanksData:    banks,
		Logger:       slog.Default(),
	})
}

// --- minimal plugin stubs ---

type testReaderPlugin struct {
	name          string
	authType      plugins.AuthType
	requiresCreds bool
	schema        []plugins.ConfigField
}

func (p *testReaderPlugin) Name() string                    { return p.name }
func (p *testReaderPlugin) Description() string             { return p.name + " reader" }
func (p *testReaderPlugin) RequiredScopes() []string        { return []string{} }
func (p *testReaderPlugin) AuthType() plugins.AuthType      { return p.authType }
func (p *testReaderPlugin) RequiresCredentialsUpload() bool { return p.requiresCreds }
func (p *testReaderPlugin) ConfigSchema() []plugins.ConfigField {
	if p.schema == nil {
		return []plugins.ConfigField{}
	}
	return p.schema
}

func (p *testReaderPlugin) NewReader( //nolint:revive
	_ *http.Client, _ *config.Config, _ []pkgapi.Rule,
	_ pkgapi.CategoryResolver, _ *pkgstate.Manager, _ pkgapi.DiagnosticSink, _ *slog.Logger,
) (pkgapi.Reader, error) {
	return nil, errors.New("not implemented in test stub")
}

type testWriterPlugin struct{ name string }

func (p *testWriterPlugin) Name() string             { return p.name }
func (p *testWriterPlugin) Description() string      { return p.name + " writer" }
func (p *testWriterPlugin) RequiredScopes() []string { return []string{} }
func (p *testWriterPlugin) NewWriter(_ *http.Client, _ *config.Config, _ *slog.Logger) (pkgapi.Writer, error) {
	return nil, errors.New("not implemented in test stub")
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

// --- health ---

func TestHandleHealth(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	rr := get(h.HandleHealth, "/api/health")

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

func TestHandleStatus_NilStore(t *testing.T) {
	dm := &mockDaemon{status: DaemonStatus{Running: true}}
	h := newTestHandlers(t, nil, dm)
	rr := get(h.HandleStatus, "/api/status")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	daemon := resp["daemon"].(map[string]any)
	if daemon["running"] != true {
		t.Errorf("expected daemon.running=true")
	}
}

func TestHandleStatus_WithStats(t *testing.T) {
	st := &mockStore{stats: &store.Stats{TotalCount: 42, TotalBase: 99999, BaseCurrency: "INR"}}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.HandleStatus, "/api/status")

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

// --- plugin listing ---

func TestHandleListReaders(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	rr := get(h.HandleListReaders, "/api/plugins/readers")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var readers []ReaderInfo
	decodeJSON(t, rr.Body.String(), &readers)
	if len(readers) != 2 {
		t.Fatalf("expected 2 readers, got %d", len(readers))
	}
	// Verify gmail metadata.
	var gmail ReaderInfo
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

func TestHandleListWriters(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	rr := get(h.HandleListWriters, "/api/plugins/writers")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var writers []WriterInfo
	decodeJSON(t, rr.Body.String(), &writers)
	if len(writers) != 1 || writers[0].Name != "postgres" {
		t.Errorf("expected postgres writer, got %v", writers)
	}
}

// --- credentials status ---

func TestHandleCredentialsStatus_Missing(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	// h.dataDir is t.TempDir() — no credentials file exists there.
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/readers/gmail/credentials/status", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.HandleCredentialsStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]bool
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["exists"] {
		t.Error("expected exists=false")
	}
}

func TestHandleCredentialsStatus_Present(t *testing.T) {
	ms := &mockStore{
		readerSecrets: map[string][]byte{"gmail": []byte(`{"installed":{}}`)},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/readers/gmail/credentials/status", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.HandleCredentialsStatus(rr, req)

	var resp map[string]bool
	decodeJSON(t, rr.Body.String(), &resp)
	if !resp["exists"] {
		t.Error("expected exists=true")
	}
}

func TestHandleCredentialsStatus_UnknownReader(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/readers/noexist/credentials/status", nil)
	req.SetPathValue("name", "noexist")
	rr := httptest.NewRecorder()
	h.HandleCredentialsStatus(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestHandleUploadCredentials_SavesToStore(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/readers/gmail/credentials", strings.NewReader(`{"installed":{}}`))
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()

	h.HandleUploadCredentials(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["path"] != "db://reader_runtime/gmail/client_secret" {
		t.Fatalf("path = %q", resp["path"])
	}
	if string(ms.readerSecrets["gmail"]) != `{"installed":{}}` {
		t.Fatalf("secret was not persisted to store: %s", ms.readerSecrets["gmail"])
	}
	if _, err := os.Stat(filepath.Join(h.dataDir, "client_secret_gmail.json")); !os.IsNotExist(err) {
		t.Fatalf("credentials should not be written to disk, stat err=%v", err)
	}
}

// --- auth status ---

func TestHandleAuthStatus_NoToken(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/readers/gmail/auth/status", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.HandleAuthStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["authenticated"] != false {
		t.Errorf("expected authenticated=false")
	}
}

func TestHandleAuthStatus_ConfigReader(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/readers/thunderbird/auth/status", nil)
	req.SetPathValue("name", "thunderbird")
	rr := httptest.NewRecorder()
	h.HandleAuthStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["authenticated"] != true {
		t.Errorf("config-only reader should always be authenticated, got %v", resp["authenticated"])
	}
}

func TestHandleAuthStatus_UsesStoreToken(t *testing.T) {
	ms := &mockStore{
		readerTokens: map[string][]byte{
			"gmail": []byte(`{"access_token":"a","token_type":"Bearer","expiry":"2999-01-01T00:00:00Z"}`),
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/readers/gmail/auth/status", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()

	h.HandleAuthStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["authenticated"] != true {
		t.Fatalf("expected authenticated=true, got %v", resp)
	}
}

// --- reader status ---

func TestHandleReaderStatus_Thunderbird_NotConfigured(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/readers/thunderbird/status", nil)
	req.SetPathValue("name", "thunderbird")
	rr := httptest.NewRecorder()
	h.HandleReaderStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["ready"] != false {
		t.Error("thunderbird without config should not be ready")
	}
}

func TestHandleReaderStatus_Thunderbird_Configured(t *testing.T) {
	ms := &mockStore{
		readerConfigs: map[string]json.RawMessage{
			"thunderbird": json.RawMessage(`{"profilePath":"/tmp/tb"}`),
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/readers/thunderbird/status", nil)
	req.SetPathValue("name", "thunderbird")
	rr := httptest.NewRecorder()
	h.HandleReaderStatus(rr, req)

	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["ready"] != true {
		t.Errorf("thunderbird with config should be ready, got %v", resp)
	}
}

func TestHandleSaveReaderConfig_SavesToStore(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/api/readers/thunderbird/config",
		strings.NewReader(`{"config":{"mailboxes":"Inbox"}}`),
	)
	req.SetPathValue("name", "thunderbird")
	rr := httptest.NewRecorder()

	h.HandleSaveReaderConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(ms.readerConfigs["thunderbird"], []byte("Inbox")) {
		t.Fatalf("config not persisted: %s", ms.readerConfigs["thunderbird"])
	}
	if _, err := os.Stat(filepath.Join(h.dataDir, "config_thunderbird.json")); !os.IsNotExist(err) {
		t.Fatalf("config should not be written to disk, stat err=%v", err)
	}
}

func TestHandleGetReaderConfig_LoadsFromStore(t *testing.T) {
	ms := &mockStore{
		readerConfigs: map[string]json.RawMessage{
			"thunderbird": json.RawMessage(`{"config":{"mailboxes":"Inbox"}}`),
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/readers/thunderbird/config", nil)
	req.SetPathValue("name", "thunderbird")
	rr := httptest.NewRecorder()

	h.HandleGetReaderConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte("Inbox")) {
		t.Fatalf("config response = %s", rr.Body.String())
	}
}

func TestHandleRevokeToken_DeletesStoreToken(t *testing.T) {
	ms := &mockStore{readerTokens: map[string][]byte{"gmail": []byte(`{"access_token":"a"}`)}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/readers/gmail/auth/token", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()

	h.HandleRevokeToken(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if _, ok := ms.readerTokens["gmail"]; ok {
		t.Fatal("token was not deleted from store")
	}
}

// --- transactions ---

func TestHandleListTransactions_NilStore(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	rr := get(h.HandleListTransactions, "/api/transactions")
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
}

func TestHandleListTransactions_Empty(t *testing.T) {
	st := &mockStore{transactions: []store.Transaction{}, listResult: store.TransactionListResult{Total: 0}}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.HandleListTransactions, "/api/transactions?page=1&page_size=10")

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

func TestHandleListTransactions_NilSliceReturnsEmptyArray(t *testing.T) {
	st := &mockStore{transactions: nil, listResult: store.TransactionListResult{Total: 0}}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.HandleListTransactions, "/api/transactions?page=1&page_size=10")

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

func TestHandleListTransactions_WithResults(t *testing.T) {
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
	rr := get(h.HandleListTransactions, "/api/transactions")

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

func TestHandleListTransactions_StoreError(t *testing.T) {
	st := &mockStore{listErr: errors.New("db error")}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.HandleListTransactions, "/api/transactions")
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestHandleListTransactions_InvalidControlCharFilter(t *testing.T) {
	st := &mockStore{}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.HandleListTransactions, "/api/transactions?currency=%00bad")

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandleListTransactions_HugePaginationFallsBackToDefaults(t *testing.T) {
	st := &mockStore{transactions: []store.Transaction{}, listResult: store.TransactionListResult{Total: 0}}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.HandleListTransactions, "/api/transactions?page=576460752303423488&page_size=999999")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if st.listFilter.Page != 1 {
		t.Fatalf("expected page to fall back to 1, got %d", st.listFilter.Page)
	}
	if st.listFilter.PageSize != 20 {
		t.Fatalf("expected page_size to fall back to 20, got %d", st.listFilter.PageSize)
	}
}

func TestHandleGetTransaction_Found(t *testing.T) {
	txn := &store.Transaction{ID: "11111111-1111-1111-1111-111111111111", Amount: 500, Currency: "INR", Labels: []string{"food"}}
	st := &mockStore{getResult: txn}
	h := newTestHandlers(t, st, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/transactions/11111111-1111-1111-1111-111111111111", nil)
	req.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
	rr := httptest.NewRecorder()
	h.HandleGetTransaction(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp store.Transaction
	decodeJSON(t, rr.Body.String(), &resp)
	if resp.ID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("expected UUID id, got %s", resp.ID)
	}
}

func TestHandleGetTransaction_NotFound(t *testing.T) {
	st := &mockStore{getErr: store.ErrNotFound}
	h := newTestHandlers(t, st, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/transactions/22222222-2222-2222-2222-222222222222", nil)
	req.SetPathValue("id", "22222222-2222-2222-2222-222222222222")
	rr := httptest.NewRecorder()
	h.HandleGetTransaction(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestHandleGetTransaction_InvalidID(t *testing.T) {
	st := &mockStore{}
	h := newTestHandlers(t, st, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/transactions/not-a-uuid", nil)
	req.SetPathValue("id", "not-a-uuid")
	rr := httptest.NewRecorder()
	h.HandleGetTransaction(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandleUpdateTransaction_Success(t *testing.T) {
	txn := &store.Transaction{ID: "abc", Description: "Updated", Labels: []string{}}
	st := &mockStore{getResult: txn}
	h := newTestHandlers(t, st, &mockDaemon{})

	body := `{"description":"Updated"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/transactions/abc", strings.NewReader(body))
	req.SetPathValue("id", "abc")
	rr := httptest.NewRecorder()
	h.HandleUpdateTransaction(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", rr.Code, rr.Body.String())
	}
}

func TestHandleUpdateTransaction_NotFound(t *testing.T) {
	st := &mockStore{updateTxErr: store.ErrNotFound}
	h := newTestHandlers(t, st, &mockDaemon{})

	body := `{"description":"x"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/transactions/noexist", strings.NewReader(body))
	req.SetPathValue("id", "noexist")
	rr := httptest.NewRecorder()
	h.HandleUpdateTransaction(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestHandleUpdateTransaction_InvalidJSON(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/transactions/abc", strings.NewReader("not-json"))
	req.SetPathValue("id", "abc")
	rr := httptest.NewRecorder()
	h.HandleUpdateTransaction(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rr.Code)
	}
}

func TestHandleListExtractionDiagnostics_DefaultsStatusOpen(t *testing.T) {
	st := &mockStore{diagnostics: []store.ExtractionDiagnosticRow{{ID: "diag-1", Status: store.DiagnosticStatusOpen}}}
	h := newTestHandlers(t, st, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/extraction-diagnostics", nil)
	rr := httptest.NewRecorder()
	h.HandleListExtractionDiagnostics(rr, req)

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

func TestHandleListExtractionDiagnostics_StatusAllAndLimit(t *testing.T) {
	st := &mockStore{}
	h := newTestHandlers(t, st, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/extraction-diagnostics?status=all&limit=25", nil)
	rr := httptest.NewRecorder()
	h.HandleListExtractionDiagnostics(rr, req)

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

func TestHandleListExtractionDiagnostics_InvalidStatus(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/extraction-diagnostics?status=pending", nil)
	rr := httptest.NewRecorder()
	h.HandleListExtractionDiagnostics(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rr.Code)
	}
}

func TestHandleListExtractionDiagnostics_InvalidLimit(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/extraction-diagnostics?limit=bad", nil)
	rr := httptest.NewRecorder()
	h.HandleListExtractionDiagnostics(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rr.Code)
	}
}

func TestHandleGetExtractionDiagnostic_Found(t *testing.T) {
	row := &store.ExtractionDiagnosticRow{ID: "11111111-1111-1111-1111-111111111111", Status: store.DiagnosticStatusOpen}
	h := newTestHandlers(t, &mockStore{diagnosticResult: row}, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/extraction-diagnostics/11111111-1111-1111-1111-111111111111", nil)
	req.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
	rr := httptest.NewRecorder()
	h.HandleGetExtractionDiagnostic(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	var resp store.ExtractionDiagnosticRow
	decodeJSON(t, rr.Body.String(), &resp)
	if resp.ID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("expected UUID id, got %q", resp.ID)
	}
}

func TestHandleGetExtractionDiagnostic_NotFound(t *testing.T) {
	h := newTestHandlers(t, &mockStore{diagnosticErr: store.ErrNotFound}, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/extraction-diagnostics/22222222-2222-2222-2222-222222222222", nil)
	req.SetPathValue("id", "22222222-2222-2222-2222-222222222222")
	rr := httptest.NewRecorder()
	h.HandleGetExtractionDiagnostic(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestHandleGetExtractionDiagnostic_InvalidID(t *testing.T) {
	h := newTestHandlers(t, &mockStore{diagnosticErr: errors.New("store should not be called")}, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/extraction-diagnostics/not-a-uuid", nil)
	req.SetPathValue("id", "not-a-uuid")
	rr := httptest.NewRecorder()
	h.HandleGetExtractionDiagnostic(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandleGetExtractionDiagnostic_NilStore(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/extraction-diagnostics/11111111-1111-1111-1111-111111111111", nil)
	req.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
	rr := httptest.NewRecorder()
	h.HandleGetExtractionDiagnostic(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
}

func TestHandleUpdateExtractionDiagnosticStatus_InvalidStatus(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPut,
		"/api/extraction-diagnostics/11111111-1111-1111-1111-111111111111/status",
		strings.NewReader(`{"status":"all"}`),
	)
	req.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
	rr := httptest.NewRecorder()
	h.HandleUpdateExtractionDiagnosticStatus(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rr.Code)
	}
}

func TestHandleUpdateExtractionDiagnosticStatus_NotFound(t *testing.T) {
	h := newTestHandlers(t, &mockStore{diagnosticErr: store.ErrNotFound}, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPut,
		"/api/extraction-diagnostics/33333333-3333-3333-3333-333333333333/status",
		strings.NewReader(`{"status":"resolved"}`),
	)
	req.SetPathValue("id", "33333333-3333-3333-3333-333333333333")
	rr := httptest.NewRecorder()
	h.HandleUpdateExtractionDiagnosticStatus(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestHandleUpdateExtractionDiagnosticStatus_InvalidID(t *testing.T) {
	h := newTestHandlers(t, &mockStore{diagnosticErr: errors.New("store should not be called")}, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPut,
		"/api/extraction-diagnostics/not-a-uuid/status",
		strings.NewReader(`{"status":"resolved"}`),
	)
	req.SetPathValue("id", "not-a-uuid")
	rr := httptest.NewRecorder()
	h.HandleUpdateExtractionDiagnosticStatus(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandleUpdateExtractionDiagnosticStatus_Conflict(t *testing.T) {
	h := newTestHandlers(t, &mockStore{diagnosticErr: store.ErrDiagnosticConflict}, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPut,
		"/api/extraction-diagnostics/44444444-4444-4444-4444-444444444444/status",
		strings.NewReader(`{"status":"open"}`),
	)
	req.SetPathValue("id", "44444444-4444-4444-4444-444444444444")
	rr := httptest.NewRecorder()
	h.HandleUpdateExtractionDiagnosticStatus(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}

func TestHandleUpdateExtractionDiagnosticStatus_NilStore(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPut,
		"/api/extraction-diagnostics/11111111-1111-1111-1111-111111111111/status",
		strings.NewReader(`{"status":"resolved"}`),
	)
	req.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
	rr := httptest.NewRecorder()
	h.HandleUpdateExtractionDiagnosticStatus(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
}

func TestHandleListExtractionDiagnostics_NilStore(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/extraction-diagnostics", nil)
	rr := httptest.NewRecorder()
	h.HandleListExtractionDiagnostics(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
}

func TestHandleAddLabels_Success(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	body := `{"labels":["food","work"]}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/transactions/abc/labels", strings.NewReader(body))
	req.SetPathValue("id", "abc")
	rr := httptest.NewRecorder()
	h.HandleAddLabels(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestHandleAddLabels_InvalidJSON(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/transactions/abc/labels", strings.NewReader("bad"))
	req.SetPathValue("id", "abc")
	rr := httptest.NewRecorder()
	h.HandleAddLabels(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rr.Code)
	}
}

func TestHandleAddLabels_BatchSuccess(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})

	body := `{"labels":["food","work","recurring"]}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/transactions/abc/labels", strings.NewReader(body))
	req.SetPathValue("id", "abc")
	rr := httptest.NewRecorder()
	h.HandleAddLabels(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestHandleAddLabels_StoreError_Returns500(t *testing.T) {
	h := newTestHandlers(t, &mockStore{addLabelsErr: errors.New("db error")}, &mockDaemon{})

	body := `{"labels":["food"]}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/transactions/abc/labels", strings.NewReader(body))
	req.SetPathValue("id", "abc")
	rr := httptest.NewRecorder()
	h.HandleAddLabels(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestHandleRemoveLabel_Success(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/transactions/abc/labels/food", nil)
	req.SetPathValue("id", "abc")
	req.SetPathValue("label", "food")
	rr := httptest.NewRecorder()
	h.HandleRemoveLabel(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestHandleRemoveLabel_NotFound(t *testing.T) {
	h := newTestHandlers(t, &mockStore{removeLblErr: store.ErrNotFound}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/transactions/abc/labels/missing", nil)
	req.SetPathValue("id", "abc")
	req.SetPathValue("label", "missing")
	rr := httptest.NewRecorder()
	h.HandleRemoveLabel(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestHandleSearchTransactions_Basic(t *testing.T) {
	st := &mockStore{
		searchResult: []store.Transaction{{ID: "x", MerchantInfo: "Zomato", Labels: []string{}}},
		searchListResult: store.TransactionListResult{
			Total:       1,
			TotalAmount: 245,
		},
	}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.HandleSearchTransactions, "/api/transactions/search?q=zomato")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["query"] != "zomato" {
		t.Errorf("expected query=zomato, got %v", resp["query"])
	}
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

func TestHandleSearchTransactions_EmptyArray(t *testing.T) {
	st := &mockStore{
		searchResult:     []store.Transaction{},
		searchListResult: store.TransactionListResult{Total: 0},
	}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.HandleSearchTransactions, "/api/transactions/search?q=zomato")

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

func TestHandleSearchTransactions_NilSliceReturnsEmptyArray(t *testing.T) {
	st := &mockStore{
		searchResult:     nil,
		searchListResult: store.TransactionListResult{Total: 0},
	}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.HandleSearchTransactions, "/api/transactions/search?q=zomato")

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

func TestHandleSearchTransactions_NilStore(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	rr := get(h.HandleSearchTransactions, "/api/transactions/search?q=foo")
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
}

func TestHandleSearchTransactions_MutedAndIndividualFlags(t *testing.T) {
	st := &mockStore{
		searchResult:     []store.Transaction{},
		searchListResult: store.TransactionListResult{Total: 0},
	}
	h := newTestHandlers(t, st, &mockDaemon{})

	rr := get(h.HandleSearchTransactions, "/api/transactions/search?q=zomato&muted_only=1&individual_only=1")

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

func TestHandleSearchTransactions_InvalidControlCharQuery(t *testing.T) {
	st := &mockStore{}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.HandleSearchTransactions, "/api/transactions/search?q=%00bad")

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandleSearchTransactions_HugePaginationFallsBackToDefaults(t *testing.T) {
	st := &mockStore{
		searchResult:     []store.Transaction{},
		searchListResult: store.TransactionListResult{Total: 0},
	}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.HandleSearchTransactions, "/api/transactions/search?q=test&page=576460752303423488&page_size=999999")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if st.searchFilter.Page != 1 {
		t.Fatalf("expected page to fall back to 1, got %d", st.searchFilter.Page)
	}
	if st.searchFilter.PageSize != 20 {
		t.Fatalf("expected page_size to fall back to 20, got %d", st.searchFilter.PageSize)
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

func TestHandleAuthCallback_RejectsUnknownState(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/auth/callback?state=doesnotexist&code=xyz", nil)
	rr := httptest.NewRecorder()
	h.HandleAuthCallback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown state, got %d", rr.Code)
	}
}

func TestHandleAuthCallback_RejectsExpiredState(t *testing.T) {
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
	h.HandleAuthCallback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for expired state, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

// --- app config / base currency ---

func TestHandleGetBaseCurrency_DefaultsToConfig(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/config/base-currency", nil)
	rr := httptest.NewRecorder()
	h.HandleGetBaseCurrency(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["base_currency"] != "INR" {
		t.Errorf("expected INR (from config), got %q", resp["base_currency"])
	}
}

func TestHandleGetBaseCurrency_FromDB(t *testing.T) {
	ms := &mockStore{appConfig: map[string]string{"base_currency": "USD"}}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/config/base-currency", nil)
	rr := httptest.NewRecorder()
	h.HandleGetBaseCurrency(rr, req)

	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["base_currency"] != "USD" {
		t.Errorf("expected USD from DB, got %q", resp["base_currency"])
	}
}

func TestHandleGetSetupStatusRequiresMissingPreferences(t *testing.T) {
	h := newTestHandlers(t, &mockStore{appConfig: map[string]string{"scan_interval": "60"}}, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/config/setup-status", nil)
	rr := httptest.NewRecorder()
	h.HandleGetSetupStatus(rr, req)

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

func TestHandleGetSetupStatusCompleteWhenPreferencesExist(t *testing.T) {
	h := newTestHandlers(t, &mockStore{appConfig: map[string]string{
		"base_currency":   "USD",
		"app.timezone":    "America/New_York",
		"app.time_format": "h:mm a",
	}}, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/config/setup-status", nil)
	rr := httptest.NewRecorder()
	h.HandleGetSetupStatus(rr, req)

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

func TestHandleGetDashboardData_Success(t *testing.T) {
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
	h.HandleGetDashboardData(rr, req)

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

func TestHandleGetMonthlyBreakdown_InvalidDimension(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/api/stats/labels/monthly?dimension=nope",
		nil,
	)
	rr := httptest.NewRecorder()
	h.HandleGetLabelMonthlySpend(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandleSetBaseCurrency_Success(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})

	body := strings.NewReader(`{"base_currency":"usd"}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/config/base-currency", body)
	rr := httptest.NewRecorder()
	h.HandleSetBaseCurrency(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["base_currency"] != "USD" {
		t.Errorf("expected normalised USD, got %q", resp["base_currency"])
	}
	if ms.appConfig["base_currency"] != "USD" {
		t.Errorf("store not updated, got %q", ms.appConfig["base_currency"])
	}
}

func TestHandleSetBaseCurrency_InvalidCode(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})

	for _, tc := range []string{`{"base_currency":"US"}`, `{"base_currency":"USDA"}`, `{"base_currency":"12A"}`} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/config/base-currency", strings.NewReader(tc))
		rr := httptest.NewRecorder()
		h.HandleSetBaseCurrency(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("input %s: expected 400, got %d", tc, rr.Code)
		}
	}
}

func TestHandleSetBaseCurrency_NoStore(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/config/base-currency", strings.NewReader(`{"base_currency":"USD"}`))
	rr := httptest.NewRecorder()
	h.HandleSetBaseCurrency(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
}

func TestHandleGetFacets_NoStore(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/transactions/facets", nil)
	rr := httptest.NewRecorder()
	h.HandleGetFacets(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
}

func TestHandleGetFacets_ReturnsEmptySlices(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/transactions/facets", nil)
	rr := httptest.NewRecorder()
	h.HandleGetFacets(rr, req)
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

func TestHandleGetFacets_ReturnsLabelCounts(t *testing.T) {
	h := newTestHandlers(
		t,
		&mockStore{facets: &store.Facets{Labels: []string{"Food"}, LabelCounts: map[string]int{"Food": 3}}},
		&mockDaemon{},
	)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/transactions/facets", nil)
	rr := httptest.NewRecorder()

	h.HandleGetFacets(rr, req)

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

func TestHandleGetFacets_StoreError(t *testing.T) {
	h := newTestHandlers(t, &mockStore{getFacetsErr: errors.New("db error")}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/transactions/facets", nil)
	rr := httptest.NewRecorder()
	h.HandleGetFacets(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

// --- labels ---

func TestHandleListLabels_NoStore(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	rr := get(h.HandleListLabels, "/api/config/labels")
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
}

func TestHandleListLabels_Success(t *testing.T) {
	ms := &mockStore{labels: []store.Label{{Name: "food", Color: "#f59e0b"}}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	rr := get(h.HandleListLabels, "/api/config/labels")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp []store.Label
	decodeJSON(t, rr.Body.String(), &resp)
	if len(resp) != 1 || resp[0].Name != "food" {
		t.Errorf("unexpected response: %v", resp)
	}
}

func TestHandleCreateLabel_Success(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	body := strings.NewReader(`{"name":"groceries","color":"#aabbcc"}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/config/labels", body)
	rr := httptest.NewRecorder()
	h.HandleCreateLabel(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["name"] != "groceries" {
		t.Errorf("expected name=groceries, got %q", resp["name"])
	}
}

func TestHandleDeleteLabel_NotFound(t *testing.T) {
	ms := &mockStore{labelsErr: store.ErrNotFound}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/config/labels/missing", nil)
	req.SetPathValue("name", "missing")
	rr := httptest.NewRecorder()
	// DeleteLabel returns the labelsErr directly; since it's ErrNotFound the handler
	// logs and returns 500 (DeleteLabel has no ErrNotFound branch in the handler).
	// The store just returns the error; handler writes 500. Verify non-204.
	h.HandleDeleteLabel(rr, req)
	if rr.Code == http.StatusNoContent {
		t.Fatalf("expected non-204 on error, got 204")
	}
}

func TestHandleDeleteLabel_RemoveFromTransactionsOption(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})

	body := strings.NewReader(`{"remove_from_transactions":true}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/config/labels/food", body)
	req.SetPathValue("name", "food")
	rr := httptest.NewRecorder()

	h.HandleDeleteLabel(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if !ms.deleteLabelCleanup {
		t.Fatal("expected delete label to request transaction label cleanup")
	}
}

func TestHandleDeleteLabel_RemoveFromTransactionsQueryOption(t *testing.T) {
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

	h.HandleDeleteLabel(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if !ms.deleteLabelCleanup {
		t.Fatal("expected delete label query parameter to request transaction label cleanup")
	}
}

// --- categories ---

func TestHandleListCategories_Success(t *testing.T) {
	ms := &mockStore{categories: []store.Category{{Name: "food & dining", IsDefault: true}}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	rr := get(h.HandleListCategories, "/api/config/categories")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp []store.Category
	decodeJSON(t, rr.Body.String(), &resp)
	if len(resp) != 1 || resp[0].Name != "food & dining" {
		t.Errorf("unexpected response: %v", resp)
	}
}

func TestHandleDeleteCategory_DefaultRejected(t *testing.T) {
	ms := &mockStore{catsErr: fmt.Errorf("cannot delete default category \"food\"")}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/config/categories/food", nil)
	req.SetPathValue("name", "food")
	rr := httptest.NewRecorder()
	h.HandleDeleteCategory(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestHandleGetCategoryMappings_Success(t *testing.T) {
	ms := &mockStore{categoryMappings: map[string][]string{"Food": {"swiggy", "zomato"}}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	rr := get(h.HandleGetCategoryMappings, "/api/config/categories/mappings")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string][]string
	decodeJSON(t, rr.Body.String(), &resp)
	if !reflect.DeepEqual(resp["Food"], []string{"swiggy", "zomato"}) {
		t.Fatalf("unexpected mappings: %#v", resp)
	}
}

func TestHandleApplyCategoryByMerchant_Success(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	body := strings.NewReader(`{"merchant_pattern":"swiggy"}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/config/categories/Food/apply", body)
	req.SetPathValue("name", "Food")
	rr := httptest.NewRecorder()

	h.HandleApplyCategoryByMerchant(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]int
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["applied"] != 2 {
		t.Fatalf("expected applied=2, got %#v", resp)
	}
}

func TestHandleDeleteCategory_RemoveFromTransactionsOption(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	body := strings.NewReader(`{"remove_from_transactions":true}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/config/categories/Food", body)
	req.SetPathValue("name", "Food")
	rr := httptest.NewRecorder()

	h.HandleDeleteCategory(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if !ms.deleteCategoryCleanup {
		t.Fatal("expected delete category to request transaction cleanup")
	}
}

func TestHandleExportCategories_IncludesMerchants(t *testing.T) {
	ms := &mockStore{
		categories:       []store.Category{{Name: "Food"}},
		categoryMappings: map[string][]string{"Food": {"swiggy"}},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})
	rr := get(h.HandleExportCategories, "/api/config/categories/export")
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

// --- buckets ---

func TestHandleListBuckets_Success(t *testing.T) {
	ms := &mockStore{buckets: []store.Bucket{{Name: "needs", IsDefault: true}}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	rr := get(h.HandleListBuckets, "/api/config/buckets")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp []store.Bucket
	decodeJSON(t, rr.Body.String(), &resp)
	if len(resp) != 1 || resp[0].Name != "needs" {
		t.Errorf("unexpected response: %v", resp)
	}
}

func TestHandleGetBucketMappings_Success(t *testing.T) {
	ms := &mockStore{bucketMappings: map[string][]string{"Needs": {"rent"}}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	rr := get(h.HandleGetBucketMappings, "/api/config/buckets/mappings")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string][]string
	decodeJSON(t, rr.Body.String(), &resp)
	if !reflect.DeepEqual(resp["Needs"], []string{"rent"}) {
		t.Fatalf("unexpected mappings: %#v", resp)
	}
}

func TestHandleApplyBucketByMerchant_Success(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	body := strings.NewReader(`{"merchant_pattern":"rent"}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/config/buckets/Needs/apply", body)
	req.SetPathValue("name", "Needs")
	rr := httptest.NewRecorder()

	h.HandleApplyBucketByMerchant(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]int
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["applied"] != 3 {
		t.Fatalf("expected applied=3, got %#v", resp)
	}
}

func TestHandleDeleteBucket_RemoveFromTransactionsOption(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	body := strings.NewReader(`{"remove_from_transactions":true}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/config/buckets/Needs", body)
	req.SetPathValue("name", "Needs")
	rr := httptest.NewRecorder()

	h.HandleDeleteBucket(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if !ms.deleteBucketCleanup {
		t.Fatal("expected delete bucket to request transaction cleanup")
	}
}

func TestHandleExportBuckets_IncludesMerchants(t *testing.T) {
	ms := &mockStore{
		buckets:        []store.Bucket{{Name: "Needs"}},
		bucketMappings: map[string][]string{"Needs": {"rent"}},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})
	rr := get(h.HandleExportBuckets, "/api/config/buckets/export")
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

// --- extended update transaction ---

func TestHandleUpdateTransaction_InvalidCategory(t *testing.T) {
	// Store returns empty category list, so any category name is invalid.
	ms := &mockStore{categories: []store.Category{}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	body := strings.NewReader(`{"category":"nonexistent"}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/transactions/abc", body)
	req.SetPathValue("id", "abc")
	rr := httptest.NewRecorder()
	h.HandleUpdateTransaction(rr, req)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

// --- scan interval ---

func TestHandleGetScanInterval_Default(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/config/scan-interval", nil)
	rr := httptest.NewRecorder()
	h.HandleGetScanInterval(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["scan_interval"] != "60" {
		t.Errorf("expected scan_interval=60, got %q", resp["scan_interval"])
	}
}

func TestHandleSetScanInterval_Valid(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})

	body := strings.NewReader(`{"scan_interval":"120"}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/config/scan-interval", body)
	rr := httptest.NewRecorder()
	h.HandleSetScanInterval(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["scan_interval"] != "120" {
		t.Errorf("expected scan_interval=120, got %q", resp["scan_interval"])
	}
	if ms.appConfig["scan_interval"] != "120" {
		t.Errorf("store not updated, got %q", ms.appConfig["scan_interval"])
	}
}

func TestHandleSetScanInterval_TooLow(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})

	body := strings.NewReader(`{"scan_interval":"5"}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/config/scan-interval", body)
	rr := httptest.NewRecorder()
	h.HandleSetScanInterval(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestHandleSetScanInterval_TooHigh(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})

	body := strings.NewReader(`{"scan_interval":"9999"}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/config/scan-interval", body)
	rr := httptest.NewRecorder()
	h.HandleSetScanInterval(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestHandleGetReaderCheckpoint_EmptyValueReturnsNull(t *testing.T) {
	ms := &mockStore{appConfig: map[string]string{"reader.gmail.last_scan_at": ""}}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/config/readers/gmail/checkpoint", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.HandleGetReaderCheckpoint(rr, req)

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

func TestHandleGetHeatmap_Success(t *testing.T) {
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
	h.HandleGetHeatmap(rr, req)

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

func TestHandleGetHeatmap_StoreError_Returns500(t *testing.T) {
	ms := &mockStore{heatmapErr: errors.New("db connection lost")}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/stats/heatmap", nil)
	rr := httptest.NewRecorder()
	h.HandleGetHeatmap(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestHandleGetHeatmap_NoStore_Returns503(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/stats/heatmap", nil)
	rr := httptest.NewRecorder()
	h.HandleGetHeatmap(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestHandleGetHeatmap_WithFromTo_Returns200(t *testing.T) {
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
	h.HandleGetHeatmap(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp store.HeatmapData
	decodeJSON(t, rr.Body.String(), &resp)
	if len(resp.ByWeekdayHour) != 1 {
		t.Errorf("expected 1 bucket, got %d", len(resp.ByWeekdayHour))
	}
}

func TestHandleGetHeatmap_InvalidFrom_Returns400(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodGet,
		"/api/stats/heatmap?from=not-a-date",
		nil,
	)
	rr := httptest.NewRecorder()
	h.HandleGetHeatmap(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestHandleGetAnnualHeatmap_Success(t *testing.T) {
	ms := &mockStore{
		annualData: []store.DailyBucket{
			{Date: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), Amount: 1500.0, Count: 3},
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/stats/heatmap/annual?year=2026", nil)
	rr := httptest.NewRecorder()
	h.HandleGetAnnualHeatmap(rr, req)

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

func TestHandleGetAnnualHeatmap_StoreError_Returns500(t *testing.T) {
	ms := &mockStore{annualErr: errors.New("db connection lost")}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/stats/heatmap/annual?year=2026", nil)
	rr := httptest.NewRecorder()
	h.HandleGetAnnualHeatmap(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestHandleGetAnnualHeatmap_NoStore_Returns503(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/stats/heatmap/annual?year=2026", nil)
	rr := httptest.NewRecorder()
	h.HandleGetAnnualHeatmap(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

// --- rules ---

func TestHandleListRules_Success(t *testing.T) {
	ms := &mockStore{rules: []store.RuleRow{{ID: "1", Name: "test", AmountRegex: `\d+`, MerchantRegex: `.+`}}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/rules", nil)
	rr := httptest.NewRecorder()
	h.HandleListRules(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp []store.RuleRow
	decodeJSON(t, rr.Body.String(), &resp)
	if len(resp) != 1 {
		t.Errorf("expected 1 rule, got %d", len(resp))
	}
}

func TestHandleListRules_NilStore_Returns503(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/rules", nil)
	rr := httptest.NewRecorder()
	h.HandleListRules(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
}

func TestHandleCreateRule_Success(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	body := `{"name":"test","amount_regex":"\\d+","merchant_regex":".+"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/rules", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.HandleCreateRule(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestHandleCreateRule_ClearsActiveReaderCheckpoint(t *testing.T) {
	ms := &mockStore{
		activeReader: "gmail",
		appConfig:    map[string]string{"reader.gmail.last_scan_at": "2026-04-27T00:00:00Z"},
	}
	dm := &mockDaemon{}
	h := newTestHandlers(t, ms, dm)
	restarted := ""
	h.restartFn = func(reader string) { restarted = reader }

	body := `{"name":"New Rule","amount_regex":"Rs\\.([\\d.]+)","merchant_regex":"at (.*?) on"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/rules", strings.NewReader(body))
	rr := httptest.NewRecorder()

	h.HandleCreateRule(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if got := ms.appConfig["reader.gmail.last_scan_at"]; got != "" {
		t.Fatalf("reader checkpoint = %q, want empty", got)
	}
	if restarted != "" {
		t.Fatalf("restartFn called while daemon stopped: %q", restarted)
	}
}

func TestHandleCreateRule_RestartsRunningDaemonAfterCheckpointClear(t *testing.T) {
	ms := &mockStore{
		activeReader: "gmail",
		appConfig:    map[string]string{"reader.gmail.last_scan_at": "2026-04-27T00:00:00Z"},
	}
	dm := &mockDaemon{status: DaemonStatus{Running: true}}
	h := newTestHandlers(t, ms, dm)
	restarted := ""
	h.restartFn = func(reader string) { restarted = reader }

	body := `{"name":"New Rule","amount_regex":"Rs\\.([\\d.]+)","merchant_regex":"at (.*?) on"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/rules", strings.NewReader(body))
	rr := httptest.NewRecorder()

	h.HandleCreateRule(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if restarted != "gmail" {
		t.Fatalf("restartFn reader = %q, want gmail", restarted)
	}
}

func TestHandleCreateRule_MissingAmountRegex_Returns422(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	body := `{"name":"test","merchant_regex":".+"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/rules", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.HandleCreateRule(rr, req)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rr.Code)
	}
}

func TestHandleCreateRule_InvalidAmountRegex_Returns422(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	body := `{"name":"test","amount_regex":"[invalid","merchant_regex":".+"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/rules", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.HandleCreateRule(rr, req)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rr.Code)
	}
}

func TestHandleUpdateRule_AnyRule_FullUpdate(t *testing.T) {
	ms := &mockStore{ruleResult: &store.RuleRow{ID: "1", Predefined: true}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	body := `{"name":"updated","amount_regex":"\\d+","merchant_regex":".+"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/rules/1", strings.NewReader(body))
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	h.HandleUpdateRule(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestHandleDeleteRule_PredefinedRule_Returns403(t *testing.T) {
	ms := &mockStore{ruleResult: &store.RuleRow{ID: "1", Predefined: true}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/rules/1", nil)
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	h.HandleDeleteRule(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestHandleDeleteRule_UserRule_Returns204(t *testing.T) {
	ms := &mockStore{ruleResult: &store.RuleRow{ID: "1", Predefined: false}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/rules/1", nil)
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	h.HandleDeleteRule(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
}

func TestHandleExportRules_OnlyNonPredefinedRules(t *testing.T) {
	ms := &mockStore{rules: []store.RuleRow{
		{ID: "1", Name: "predefined", Predefined: true, AmountRegex: `\d+`, MerchantRegex: `.+`},
		{ID: "2", Name: "usr", Predefined: false, AmountRegex: `\d+`, MerchantRegex: `.+`},
	}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/rules/export", nil)
	rr := httptest.NewRecorder()
	h.HandleExportRules(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var exported []map[string]any
	decodeJSON(t, rr.Body.String(), &exported)
	if len(exported) != 1 {
		t.Errorf("expected 1 exported rule (user only), got %d", len(exported))
	}
	if exported[0]["name"] != "usr" {
		t.Errorf("expected exported name=usr, got %v", exported[0]["name"])
	}
}

func TestHandleImportRules_InvalidRegex_Returns422(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	body := `[{"name":"bad","amountRegex":"[invalid","merchantInfoRegex":".+"}]`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/rules/import", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.HandleImportRules(rr, req)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

// --- thunderbird discovery + guide ---

func TestHandleDiscoverProfiles_Returns200WithProfilesKey(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/readers/thunderbird/discover/profiles", nil)
	rr := httptest.NewRecorder()
	h.HandleDiscoverProfiles(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string][]string
	decodeJSON(t, rr.Body.String(), &resp)
	if _, ok := resp["profiles"]; !ok {
		t.Error("expected 'profiles' key in response")
	}
}

func TestHandleDiscoverMailboxes_MissingParam_Returns400(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/readers/thunderbird/discover/mailboxes", nil)
	rr := httptest.NewRecorder()
	h.HandleDiscoverMailboxes(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandleDiscoverMailboxes_NonexistentProfile_Returns404(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodGet,
		"/api/readers/thunderbird/discover/mailboxes?profile=/nonexistent/thunderbird/profile",
		nil,
	)
	rr := httptest.NewRecorder()
	h.HandleDiscoverMailboxes(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestHandleGetReaderGuide_NoGuide_Returns404(t *testing.T) {
	// testReaderPlugin (used by newTestHandlers) does not implement GuideProvider.
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/readers/gmail/guide", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.HandleGetReaderGuide(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

// --- rescan ---

func TestHandleRescan_DaemonRunning_Returns202Rescanning(t *testing.T) {
	called := false
	ms := &mockStore{}
	dm := &mockDaemon{status: DaemonStatus{Running: true}}
	h := newTestHandlers(t, ms, dm)
	h.rescanFn = func(_ string) { called = true }

	body := `{"reader":"gmail"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/daemon/rescan", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.HandleRescan(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["status"] != "rescanning" {
		t.Errorf("expected status=rescanning, got %q", resp["status"])
	}
	if !called {
		t.Error("expected rescanFn to be called even when daemon is running")
	}
}

func TestHandleRescan_DaemonNotRunning_Returns202Rescanning(t *testing.T) {
	called := false
	ms := &mockStore{}
	dm := &mockDaemon{status: DaemonStatus{Running: false}}
	h := newTestHandlers(t, ms, dm)
	h.rescanFn = func(_ string) { called = true }

	body := `{"reader":"gmail"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/daemon/rescan", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.HandleRescan(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["status"] != "rescanning" {
		t.Errorf("expected status=rescanning, got %q", resp["status"])
	}
	if !called {
		t.Error("expected rescanFn to be called")
	}
}

// --- HandleAuthExchange ---

func TestHandleAuthExchange_MissingURL_Returns400(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/readers/gmail/auth/exchange", strings.NewReader(`{}`))
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.HandleAuthExchange(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing url, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestHandleAuthExchange_MalformedURL_Returns400(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/readers/gmail/auth/exchange", strings.NewReader(`{"url":":::not-a-url"}`))
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.HandleAuthExchange(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed url, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestHandleAuthExchange_MissingCode_Returns400(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	body := `{"url":"http://localhost:8080/api/auth/callback?state=somestate"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/readers/gmail/auth/exchange", strings.NewReader(body))
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.HandleAuthExchange(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing code, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestHandleAuthExchange_MissingState_Returns400(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	body := `{"url":"http://localhost:8080/api/auth/callback?code=4%2F0Acode"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/readers/gmail/auth/exchange", strings.NewReader(body))
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.HandleAuthExchange(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing state, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestHandleAuthExchange_UnknownState_Returns400(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	body := `{"url":"http://localhost:8080/api/auth/callback?code=4%2F0Acode&state=doesnotexist"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/readers/gmail/auth/exchange", strings.NewReader(body))
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.HandleAuthExchange(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown state, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestHandleAuthExchange_ExpiredState_Returns400(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})

	expiredState := "reader:gmail:expiredtoken"
	h.mu.Lock()
	h.oauthStates[expiredState] = oauthStateEntry{
		readerName: "gmail",
		expiresAt:  time.Now().Add(-1 * time.Second),
	}
	h.mu.Unlock()

	body := `{"url":"http://localhost:8080/api/auth/callback?code=4%2F0Acode&state=` + expiredState + `"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/readers/gmail/auth/exchange", strings.NewReader(body))
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.HandleAuthExchange(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for expired state, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestHandleAuthExchange_RestartsRunningDaemonAfterTokenSaved(t *testing.T) {
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
	st := &mockStore{readerSecrets: map[string][]byte{"gmail": []byte(secretJSON)}}
	dm := &mockDaemon{status: DaemonStatus{Running: true}}
	h := newTestHandlers(t, st, dm)
	restarted := ""
	h.restartFn = func(reader string) { restarted = reader }

	state := "reader:gmail:validtoken"
	h.mu.Lock()
	h.oauthStates[state] = oauthStateEntry{
		readerName: "gmail",
		expiresAt:  time.Now().Add(time.Minute),
	}
	h.mu.Unlock()

	body := `{"url":"http://localhost:8080/api/auth/callback?code=4%2F0Acode&state=` + state + `"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/readers/gmail/auth/exchange", strings.NewReader(body))
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.HandleAuthExchange(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if restarted != "gmail" {
		t.Fatalf("restartFn reader = %q, want gmail", restarted)
	}
	if !strings.Contains(string(st.readerTokens["gmail"]), "new-refresh") {
		t.Fatalf("saved token = %s, want refresh token from re-grant", st.readerTokens["gmail"])
	}
}

func TestHandleListTransactions_BucketParam(t *testing.T) {
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
	rr := get(h.HandleListTransactions, "/api/transactions?bucket=wants")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["total"] != float64(1) {
		t.Errorf("expected total=1, got %v", resp["total"])
	}
}

func TestHandleListTransactions_WeekdayHourTimezoneParams(t *testing.T) {
	st := &mockStore{transactions: []store.Transaction{}, listResult: store.TransactionListResult{Total: 0}}
	h := newTestHandlers(t, st, &mockDaemon{})

	rr := get(h.HandleListTransactions, "/api/transactions?weekday=5&hour_from=9&hour_to=9&tz=Asia/Kolkata")

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

func TestHandleListTransactions_WeekdayHourTimezoneAndDateParams(t *testing.T) {
	st := &mockStore{transactions: []store.Transaction{}, listResult: store.TransactionListResult{Total: 0}}
	h := newTestHandlers(t, st, &mockDaemon{})

	rr := get(
		h.HandleListTransactions,
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

func TestHandleListTransactions_ExcludeParams(t *testing.T) {
	st := &mockStore{transactions: []store.Transaction{}, listResult: store.TransactionListResult{Total: 0}}
	h := newTestHandlers(t, st, &mockDaemon{})

	rr := get(
		h.HandleListTransactions,
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

func TestHandleListTransactions_MissingTaxonomyParams(t *testing.T) {
	st := &mockStore{transactions: []store.Transaction{}, listResult: store.TransactionListResult{Total: 0}}
	h := newTestHandlers(t, st, &mockDaemon{})

	rr := get(
		h.HandleListTransactions,
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

func TestHandleListTransactions_InvalidTimezoneFallsBackToAppTimezone(t *testing.T) {
	st := &mockStore{
		transactions: []store.Transaction{},
		listResult:   store.TransactionListResult{Total: 0},
		appConfig:    map[string]string{"app.timezone": "Asia/Kolkata"},
	}
	h := newTestHandlers(t, st, &mockDaemon{})

	rr := get(h.HandleListTransactions, "/api/transactions?weekday=5&hour_from=9&hour_to=9&tz=Not/AZone")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if st.listFilter.Timezone != "Asia/Kolkata" {
		t.Fatalf("expected fallback timezone Asia/Kolkata, got %q", st.listFilter.Timezone)
	}
}

func TestHandleListTransactions_MissingTimezoneFallsBackToAppTimezone(t *testing.T) {
	st := &mockStore{
		transactions: []store.Transaction{},
		listResult:   store.TransactionListResult{Total: 0},
		appConfig:    map[string]string{"app.timezone": "Asia/Kolkata"},
	}
	h := newTestHandlers(t, st, &mockDaemon{})

	rr := get(h.HandleListTransactions, "/api/transactions?weekday=5&hour_from=9&hour_to=9")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if st.listFilter.Timezone != "Asia/Kolkata" {
		t.Fatalf("expected fallback timezone Asia/Kolkata, got %q", st.listFilter.Timezone)
	}
}

func TestHandleGetTimezoneDefaultsToEmptyWhenUnset(t *testing.T) {
	st := &mockStore{}
	h := newTestHandlers(t, st, &mockDaemon{})

	rr := get(h.HandleGetTimezone, "/api/config/timezone")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["timezone"] != "" {
		t.Fatalf("expected empty timezone default, got %q", resp["timezone"])
	}
}

// --- banks ---

func TestHandleListBanks(t *testing.T) {
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
			h.HandleListBanks(w, req)
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

func (m *mockStore) SeedMCCCodes(_ context.Context, _ []store.MCCEntry) error { return nil }
func (m *mockStore) SeedMerchantCategories(_ context.Context, _ []store.MerchantCategoryEntry) (int, error) {
	return 0, nil
}

func (m *mockStore) LoadCategorySnapshot(_ context.Context) (pkgapi.CategoryResolver, error) {
	return func(_ string) (string, string) { return "", "" }, nil
}
func (m *mockStore) SeedMCCCategories(_ context.Context, _ []string) error { return nil }
func (m *mockStore) GetSyncStatus(_ context.Context) (store.SyncStatus, error) {
	if m.syncStatusErr != nil {
		return store.SyncStatus{}, m.syncStatusErr
	}
	return m.syncStatus, nil
}
func (m *mockStore) SetSyncStatus(_ context.Context, _ store.SyncStatus) error { return nil }

func TestHandleGetSyncStatus_NoStore(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/config/sync/status", nil)
	rr := httptest.NewRecorder()
	h.HandleGetSyncStatus(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["error"] != "database not connected" {
		t.Fatalf("expected standard error payload, got %#v", resp)
	}
}

func TestHandleCategorizeMerchant_OK(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	body := `{"merchant":"Netflix","category":"Entertainment","bucket":"Wants"}`
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/merchants/categorize", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleCategorizeMerchant(w, r)
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

func TestHandleCategorizeMerchant_EmptyMerchant(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	body := `{"merchant":"","category":"Entertainment","bucket":"Wants"}`
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/merchants/categorize", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleCategorizeMerchant(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestHandleCategorizeMerchant_InvalidJSON(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/merchants/categorize", strings.NewReader("not-json"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleCategorizeMerchant(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestHandleCategorizeMerchant_StoreError(t *testing.T) {
	h := newTestHandlers(t, &mockStore{updateErr: errors.New("db down")}, &mockDaemon{})
	body := `{"merchant":"Netflix","category":"Entertainment","bucket":"Wants"}`
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/merchants/categorize", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleCategorizeMerchant(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}
}
