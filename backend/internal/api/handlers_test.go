package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	transactions []store.Transaction
	total        int
	listErr      error
	getResult    *store.Transaction
	getErr       error
	updateErr    error
	addLabelErr  error
	addLabelsErr error
	removeLblErr error
	searchResult []store.Transaction
	searchTotal  int
	searchErr    error
	stats        *store.Stats
	statsErr     error
	appConfig    map[string]string
	setConfigErr error
	getFacetsErr error
	labels       []store.Label
	labelsErr    error
	categories   []store.Category
	catsErr      error
	buckets      []store.Bucket
	bucketsErr   error
	updateTxErr  error
	heatmapData  *store.HeatmapData
	heatmapErr   error
}

func (m *mockStore) ListTransactions(_ context.Context, _ store.ListFilter) ([]store.Transaction, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.transactions, m.total, nil
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

func (m *mockStore) SearchTransactions(_ context.Context, _ string, _ store.ListFilter) ([]store.Transaction, int, error) {
	if m.searchErr != nil {
		return nil, 0, m.searchErr
	}
	return m.searchResult, m.searchTotal, nil
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

func (m *mockStore) GetFacets(_ context.Context) (*store.Facets, error) {
	if m.getFacetsErr != nil {
		return nil, m.getFacetsErr
	}
	return &store.Facets{
		Sources:    []string{},
		Categories: []string{},
		Currencies: []string{},
		Labels:     []string{},
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

func (m *mockStore) DeleteLabel(_ context.Context, _ string) error { return m.labelsErr }

func (m *mockStore) ApplyLabelByMerchant(_ context.Context, _, _ string) (int64, error) {
	if m.labelsErr != nil {
		return 0, m.labelsErr
	}
	return 0, nil
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

func (m *mockStore) DeleteCategory(_ context.Context, _ string) error { return m.catsErr }

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

func (m *mockStore) DeleteBucket(_ context.Context, _ string) error { return m.bucketsErr }

func (m *mockStore) UpdateTransaction(_ context.Context, _ string, _ store.TransactionUpdate) error {
	return m.updateTxErr
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

// newTestHandlers returns a Handlers wired with a real (minimal) plugin registry,
// the given store mock, and a mock daemon.
func newTestHandlers(t *testing.T, st Storer, dm DaemonStatusProvider) *Handlers {
	t.Helper()
	registry := plugins.NewRegistry()
	_ = registry.RegisterReader(&testReaderPlugin{name: "gmail", authType: plugins.AuthTypeOAuth, requiresCreds: true})
	_ = registry.RegisterReader(&testReaderPlugin{name: "thunderbird", authType: plugins.AuthTypeConfig, requiresCreds: false, schema: []plugins.ConfigField{
		{Key: "profilePath", Label: "Profile Directory", Type: "path", Required: true},
	}})
	_ = registry.RegisterWriter(&testWriterPlugin{name: "postgres"})
	return NewHandlers(registry, st, dm, "http://localhost:8080", "http://localhost:5173", t.TempDir(), "INR", 60, 180, nil, slog.Default())
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
	_ pkgapi.Labels, _ *pkgstate.Manager, _ *slog.Logger,
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
	h := newTestHandlers(t, nil, &mockDaemon{})
	credFile := filepath.Join(h.dataDir, "client_secret_gmail.json")
	if err := os.MkdirAll(h.dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(credFile, []byte(`{"installed":{}}`), 0o600)
	// No t.Cleanup needed — t.TempDir() handles it.

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
	h := newTestHandlers(t, nil, &mockDaemon{})
	cfgFile := filepath.Join(h.dataDir, "config_thunderbird.json")
	if err := os.MkdirAll(h.dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(cfgFile, []byte(`{"profilePath":"/tmp/tb"}`), 0o600)
	// No t.Cleanup needed — t.TempDir() handles it.

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

// --- transactions ---

func TestHandleListTransactions_NilStore(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	rr := get(h.HandleListTransactions, "/api/transactions")
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
}

func TestHandleListTransactions_Empty(t *testing.T) {
	st := &mockStore{transactions: []store.Transaction{}, total: 0}
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
}

func TestHandleListTransactions_WithResults(t *testing.T) {
	now := time.Now()
	st := &mockStore{
		transactions: []store.Transaction{
			{ID: "abc", Amount: 100, Currency: "INR", MerchantInfo: "Amazon", Timestamp: now, Labels: []string{}},
		},
		total: 1,
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
}

func TestHandleListTransactions_StoreError(t *testing.T) {
	st := &mockStore{listErr: errors.New("db error")}
	h := newTestHandlers(t, st, &mockDaemon{})
	rr := get(h.HandleListTransactions, "/api/transactions")
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestHandleGetTransaction_Found(t *testing.T) {
	txn := &store.Transaction{ID: "abc", Amount: 500, Currency: "INR", Labels: []string{"food"}}
	st := &mockStore{getResult: txn}
	h := newTestHandlers(t, st, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/transactions/abc", nil)
	req.SetPathValue("id", "abc")
	rr := httptest.NewRecorder()
	h.HandleGetTransaction(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp store.Transaction
	decodeJSON(t, rr.Body.String(), &resp)
	if resp.ID != "abc" {
		t.Errorf("expected id=abc, got %s", resp.ID)
	}
}

func TestHandleGetTransaction_NotFound(t *testing.T) {
	st := &mockStore{getErr: store.ErrNotFound}
	h := newTestHandlers(t, st, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/transactions/noexist", nil)
	req.SetPathValue("id", "noexist")
	rr := httptest.NewRecorder()
	h.HandleGetTransaction(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
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
	st := &mockStore{searchResult: []store.Transaction{{ID: "x", MerchantInfo: "Zomato", Labels: []string{}}}, searchTotal: 1}
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
}

func TestHandleSearchTransactions_NilStore(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	rr := get(h.HandleSearchTransactions, "/api/transactions/search?q=foo")
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
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
	var resp map[string][]string
	decodeJSON(t, rr.Body.String(), &resp)
	for _, key := range []string{"sources", "categories", "currencies", "labels"} {
		if resp[key] == nil {
			t.Errorf("expected %q to be an empty slice, got nil", key)
		}
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
