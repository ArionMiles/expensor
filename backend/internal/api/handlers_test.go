package api

import (
	"context"
	"encoding/json"
	"errors"
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
	return NewHandlers(registry, st, dm, "http://localhost:8080", "http://localhost:5173", t.TempDir(), "INR", nil, slog.Default())
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
	st := &mockStore{updateErr: store.ErrNotFound}
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
