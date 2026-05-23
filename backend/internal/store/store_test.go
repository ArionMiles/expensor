package store_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/migrations"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	pgwriter "github.com/ArionMiles/expensor/backend/pkg/writer/postgres"
)

// testStore holds a live *store.Store and the container DSN for teardown.
type testStore struct {
	*store.Store
	cleanup func()
	logs    *bytes.Buffer
}

// newTestStore spins up a Postgres container, runs the schema migration, and
// returns a connected Store. The returned cleanup function terminates the container.
func newTestStore(t *testing.T) *testStore {
	t.Helper()
	return newTestStoreWithLogger(t, nil)
}

func newTestStoreWithLogger(t *testing.T, logs *bytes.Buffer) *testStore {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("expensor_test"),
		tcpostgres.WithUsername("expensor"),
		tcpostgres.WithPassword("expensor"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = ctr.Terminate(ctx)
		t.Fatalf("failed to get connection string: %v", err)
	}

	// Run migrations via pgxpool so the schema is ready before the Store connects.
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		_ = ctr.Terminate(ctx)
		t.Fatalf("failed to connect for migration: %v", err)
	}
	if err := pgwriter.RunMigrations(ctx, pool); err != nil {
		pool.Close()
		_ = ctr.Terminate(ctx)
		t.Fatalf("migration failed: %v", err)
	}
	pool.Close()

	cfg := config.PostgresConfig{
		Host:        "localhost",
		Database:    "expensor_test",
		User:        "expensor",
		Password:    "expensor",
		SSLMode:     "disable",
		MaxPoolSize: 2,
	}
	// Extract the host port from the mapped container port.
	mappedPort, err := ctr.MappedPort(ctx, "5432")
	if err != nil {
		_ = ctr.Terminate(ctx)
		t.Fatalf("failed to get mapped port: %v", err)
	}
	cfg.Port = int(mappedPort.Num())

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	if logs != nil {
		logger = slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}

	st, err := store.New(cfg, logger)
	if err != nil {
		_ = ctr.Terminate(ctx)
		t.Fatalf("failed to create store: %v", err)
	}

	return &testStore{
		Store: st,
		logs:  logs,
		cleanup: func() {
			st.Close()
			_ = ctr.Terminate(context.Background())
		},
	}
}

// seedTransaction inserts one transaction directly via pgxpool and returns its UUID.
func seedTransaction(t *testing.T, ctx context.Context, st *store.Store, msg string, amount float64, currency, merchant, category string) string { //nolint:revive // test helper requires many params to cover all fixture dimensions
	t.Helper()
	// We use the store's exported pool indirectly by going through a helper writer query.
	// Instead, expose a small seed helper in the test package using a raw pgx query.
	// Since store.Store wraps a pgxpool that we cannot access, we use the ListTransactions
	// round-trip pattern: insert via the postgres writer package.
	// For simplicity in tests, we insert directly via a separate pool.
	return seedTxn(t, ctx, st, msg, amount, currency, merchant, category, "")
}

// seedTxn uses a hack-free approach: it calls the store's internal pool via a
// package-level helper exposed only in tests (see seed_test.go).
func seedTxn(t *testing.T, ctx context.Context, st *store.Store, msgID string, amount float64, currency, merchant, category, description string) string { //nolint:revive // test helper requires many params to cover all fixture dimensions
	t.Helper()
	id, err := st.InsertForTest(ctx, store.InsertParams{
		MessageID:    msgID,
		Amount:       amount,
		Currency:     currency,
		MerchantInfo: merchant,
		Category:     category,
		Description:  description,
		Timestamp:    time.Now(),
	})
	if err != nil {
		t.Fatalf("seedTxn: %v", err)
	}
	return id
}

func ptrInt(v int) *int {
	return &v
}

// --- tests ---

func TestRuntimeState_ActiveReader(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	if err := ts.SetActiveReader(ctx, "gmail"); err != nil {
		t.Fatalf("SetActiveReader: %v", err)
	}
	got, err := ts.GetActiveReader(ctx)
	if err != nil {
		t.Fatalf("GetActiveReader: %v", err)
	}
	if got != "gmail" {
		t.Fatalf("active reader = %q", got)
	}
}

func TestRuntimeState_ReaderBlobAndConfig(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	if err := ts.SetReaderSecret(ctx, "gmail", []byte(`{"installed":{}}`)); err != nil {
		t.Fatalf("SetReaderSecret: %v", err)
	}
	secret, ok, err := ts.GetReaderSecret(ctx, "gmail")
	if err != nil || !ok {
		t.Fatalf("GetReaderSecret: ok=%v err=%v", ok, err)
	}
	if string(secret) != `{"installed": {}}` {
		t.Fatalf("secret = %s", secret)
	}

	if err := ts.SetReaderToken(ctx, "gmail", []byte(`{"access_token":"a","token_type":"Bearer"}`)); err != nil {
		t.Fatalf("SetReaderToken: %v", err)
	}
	token, ok, err := ts.GetReaderToken(ctx, "gmail")
	if err != nil || !ok {
		t.Fatalf("GetReaderToken: ok=%v err=%v", ok, err)
	}
	if !bytes.Contains(token, []byte(`"access_token": "a"`)) {
		t.Fatalf("token = %s", token)
	}

	if err := ts.SetReaderConfig(ctx, "thunderbird", json.RawMessage(`{"config":{"mailboxes":"Inbox"}}`)); err != nil {
		t.Fatalf("SetReaderConfig: %v", err)
	}
	cfg, ok, err := ts.GetReaderConfig(ctx, "thunderbird")
	if err != nil || !ok {
		t.Fatalf("GetReaderConfig: ok=%v err=%v", ok, err)
	}
	if !bytes.Contains(cfg, []byte(`"Inbox"`)) {
		t.Fatalf("config = %s", cfg)
	}
}

func TestRuntimeState_DeleteReaderRuntime(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	if err := ts.SetReaderSecret(ctx, "gmail", []byte(`{"installed":{}}`)); err != nil {
		t.Fatalf("SetReaderSecret: %v", err)
	}
	if err := ts.SetReaderToken(ctx, "gmail", []byte(`{"access_token":"a"}`)); err != nil {
		t.Fatalf("SetReaderToken: %v", err)
	}
	if err := ts.DeleteReaderRuntime(ctx, "gmail"); err != nil {
		t.Fatalf("DeleteReaderRuntime: %v", err)
	}
	if _, ok, err := ts.GetReaderSecret(ctx, "gmail"); err != nil || ok {
		t.Fatalf("GetReaderSecret after delete: ok=%v err=%v", ok, err)
	}
	if _, ok, err := ts.GetReaderToken(ctx, "gmail"); err != nil || ok {
		t.Fatalf("GetReaderToken after delete: ok=%v err=%v", ok, err)
	}
}

func TestRuntimeState_ProcessedMessages(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	processed, err := ts.IsMessageProcessed(ctx, "msg-1")
	if err != nil {
		t.Fatalf("IsMessageProcessed: %v", err)
	}
	if processed {
		t.Fatal("message should not start processed")
	}

	processedAt := time.Date(2026, time.April, 28, 10, 30, 0, 0, time.UTC)
	if err := ts.MarkMessageProcessed(ctx, "msg-1", processedAt); err != nil {
		t.Fatalf("MarkMessageProcessed: %v", err)
	}
	processed, err = ts.IsMessageProcessed(ctx, "msg-1")
	if err != nil {
		t.Fatalf("IsMessageProcessed after mark: %v", err)
	}
	if !processed {
		t.Fatal("message should be processed")
	}
}

func TestExtractionDiagnostics_RecordListUpdateAndGet(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()
	receivedAt := time.Date(2026, 4, 28, 10, 30, 0, 0, time.UTC)

	diagnostic := api.ExtractionDiagnostic{
		Reader:         "gmail",
		MessageID:      "diag-msg-1",
		Source:         "Credit Card - HDFC",
		Sender:         "alerts-display@example.com",
		SenderEmail:    "alerts@example.com",
		Subject:        "Card transaction",
		EmailBody:      "Spent INR 123 at Example Store",
		ReceivedAt:     &receivedAt,
		Snippet:        "Spent INR 123",
		RuleID:         "11111111-1111-1111-1111-111111111111",
		RuleName:       "Example Bank",
		AmountRegex:    `INR\s+([0-9.]+)`,
		MerchantRegex:  `at (.+)`,
		CurrencyRegex:  `(INR)`,
		FailureReasons: []string{"amount_not_found", "merchant_not_found"},
	}

	if err := ts.RecordExtractionDiagnostic(ctx, diagnostic); err != nil {
		t.Fatalf("RecordExtractionDiagnostic: %v", err)
	}

	rows, err := ts.ListExtractionDiagnostics(ctx, store.DiagnosticFilter{Status: store.DiagnosticStatusOpen})
	if err != nil {
		t.Fatalf("ListExtractionDiagnostics: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 open diagnostic, got %d", len(rows))
	}
	row := rows[0]
	if row.Status != store.DiagnosticStatusOpen {
		t.Fatalf("want status open, got %q", row.Status)
	}
	if row.Reader != diagnostic.Reader || row.MessageID != diagnostic.MessageID || row.RuleName != diagnostic.RuleName {
		t.Fatalf("unexpected identity fields: %+v", row)
	}
	if row.Source != diagnostic.Source || row.Sender != diagnostic.Sender || row.Snippet != diagnostic.Snippet {
		t.Fatalf("unexpected diagnostic metadata fields: %+v", row)
	}
	if row.ReceivedAt == nil || !row.ReceivedAt.Equal(receivedAt) {
		t.Fatalf("want received_at %v, got %v", receivedAt, row.ReceivedAt)
	}
	if row.RuleID == nil || *row.RuleID != diagnostic.RuleID {
		t.Fatalf("want rule id %q, got %v", diagnostic.RuleID, row.RuleID)
	}
	if row.AmountRegex != diagnostic.AmountRegex || row.MerchantRegex != diagnostic.MerchantRegex || row.CurrencyRegex != diagnostic.CurrencyRegex {
		t.Fatalf("unexpected regex fields: %+v", row)
	}
	if len(row.FailureReasons) != 2 || row.FailureReasons[0] != "amount_not_found" || row.FailureReasons[1] != "merchant_not_found" {
		t.Fatalf("unexpected failure reasons: %v", row.FailureReasons)
	}
	if row.ResolvedAt != nil {
		t.Fatalf("open diagnostic should not have resolved_at: %v", row.ResolvedAt)
	}

	resolved, err := ts.UpdateExtractionDiagnosticStatus(ctx, row.ID, store.DiagnosticStatusResolved)
	if err != nil {
		t.Fatalf("UpdateExtractionDiagnosticStatus: %v", err)
	}
	if resolved.Status != store.DiagnosticStatusResolved {
		t.Fatalf("want resolved status, got %q", resolved.Status)
	}
	if resolved.ResolvedAt == nil {
		t.Fatal("resolved diagnostic should have resolved_at")
	}

	got, err := ts.GetExtractionDiagnostic(ctx, row.ID)
	if err != nil {
		t.Fatalf("GetExtractionDiagnostic: %v", err)
	}
	if got.ID != row.ID || got.Status != store.DiagnosticStatusResolved {
		t.Fatalf("unexpected fetched diagnostic: %+v", got)
	}

	openAgain, err := ts.UpdateExtractionDiagnosticStatus(ctx, row.ID, store.DiagnosticStatusOpen)
	if err != nil {
		t.Fatalf("UpdateExtractionDiagnosticStatus open: %v", err)
	}
	if openAgain.ResolvedAt != nil {
		t.Fatalf("reopened diagnostic should clear resolved_at: %v", openAgain.ResolvedAt)
	}
}

func TestExtractionDiagnostics_SenderFallsBackToSenderEmail(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	diagnostic := api.ExtractionDiagnostic{
		Reader:      "gmail",
		MessageID:   "sender-fallback",
		SenderEmail: "fallback@example.com",
		RuleName:    "Sender Fallback Rule",
	}
	if err := ts.RecordExtractionDiagnostic(ctx, diagnostic); err != nil {
		t.Fatalf("RecordExtractionDiagnostic: %v", err)
	}

	rows, err := ts.ListExtractionDiagnostics(ctx, store.DiagnosticFilter{Status: store.DiagnosticStatusOpen})
	if err != nil {
		t.Fatalf("ListExtractionDiagnostics: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 diagnostic, got %d", len(rows))
	}
	if rows[0].Sender != diagnostic.SenderEmail {
		t.Fatalf("sender should fall back to sender_email, got %q", rows[0].Sender)
	}
}

func TestExtractionDiagnostics_ListLimit(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	for i := range 3 {
		if err := ts.RecordExtractionDiagnostic(ctx, api.ExtractionDiagnostic{
			Reader:    "gmail",
			MessageID: fmt.Sprintf("limit-%d", i),
			Subject:   fmt.Sprintf("Diagnostic %d", i),
			RuleName:  "Limit Rule",
		}); err != nil {
			t.Fatalf("record diagnostic %d: %v", i, err)
		}
	}

	rows, err := ts.ListExtractionDiagnostics(ctx, store.DiagnosticFilter{Status: store.DiagnosticStatusOpen, Limit: 2})
	if err != nil {
		t.Fatalf("ListExtractionDiagnostics: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 limited diagnostics, got %d", len(rows))
	}
	if rows[0].CreatedAt.Before(rows[1].CreatedAt) {
		t.Fatalf("diagnostics should be ordered newest first: %+v", rows)
	}
}

func TestExtractionDiagnostics_OpenDedupeUpdatesExistingRow(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	first := api.ExtractionDiagnostic{
		Reader:         "gmail",
		MessageID:      "dup-msg",
		SenderEmail:    "old@example.com",
		Subject:        "Old subject",
		RuleName:       "Duplicate Rule",
		AmountRegex:    "old-amount",
		FailureReasons: []string{"old_reason"},
	}
	second := api.ExtractionDiagnostic{
		Reader:         "gmail",
		MessageID:      "dup-msg",
		SenderEmail:    "new@example.com",
		Subject:        "New subject",
		EmailBody:      "new body",
		RuleName:       "Duplicate Rule",
		AmountRegex:    "new-amount",
		MerchantRegex:  "new-merchant",
		CurrencyRegex:  "new-currency",
		FailureReasons: []string{"new_reason"},
	}

	if err := ts.RecordExtractionDiagnostic(ctx, first); err != nil {
		t.Fatalf("record first diagnostic: %v", err)
	}
	if err := ts.RecordExtractionDiagnostic(ctx, second); err != nil {
		t.Fatalf("record second diagnostic: %v", err)
	}

	rows, err := ts.ListExtractionDiagnostics(ctx, store.DiagnosticFilter{Status: store.DiagnosticStatusOpen})
	if err != nil {
		t.Fatalf("ListExtractionDiagnostics: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want one deduped open diagnostic, got %d", len(rows))
	}
	row := rows[0]
	if row.SenderEmail != second.SenderEmail || row.Subject != second.Subject || row.EmailBody != second.EmailBody {
		t.Fatalf("dedupe did not update email fields: %+v", row)
	}
	if row.AmountRegex != second.AmountRegex || row.MerchantRegex != second.MerchantRegex || row.CurrencyRegex != second.CurrencyRegex {
		t.Fatalf("dedupe did not update regex fields: %+v", row)
	}
	if len(row.FailureReasons) != 1 || row.FailureReasons[0] != "new_reason" {
		t.Fatalf("dedupe did not update failure reasons: %v", row.FailureReasons)
	}
}

func TestExtractionDiagnostics_ReopenConflictReturnsSentinelError(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()
	diagnostic := api.ExtractionDiagnostic{
		Reader:    "gmail",
		MessageID: "reopen-conflict",
		RuleName:  "Conflict Rule",
		Subject:   "Original subject",
	}

	if err := ts.RecordExtractionDiagnostic(ctx, diagnostic); err != nil {
		t.Fatalf("record original diagnostic: %v", err)
	}
	rows, err := ts.ListExtractionDiagnostics(ctx, store.DiagnosticFilter{Status: store.DiagnosticStatusOpen})
	if err != nil {
		t.Fatalf("list original diagnostic: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 original diagnostic, got %d", len(rows))
	}
	originalID := rows[0].ID

	if _, err := ts.UpdateExtractionDiagnosticStatus(ctx, originalID, store.DiagnosticStatusResolved); err != nil {
		t.Fatalf("resolve original diagnostic: %v", err)
	}
	diagnostic.Subject = "Current open subject"
	if err := ts.RecordExtractionDiagnostic(ctx, diagnostic); err != nil {
		t.Fatalf("record current diagnostic: %v", err)
	}

	_, err = ts.UpdateExtractionDiagnosticStatus(ctx, originalID, store.DiagnosticStatusOpen)
	if !errors.Is(err, store.ErrDiagnosticConflict) {
		t.Fatalf("reopen should return ErrDiagnosticConflict, got %v", err)
	}
	openRows, err := ts.ListExtractionDiagnostics(ctx, store.DiagnosticFilter{Status: store.DiagnosticStatusOpen})
	if err != nil {
		t.Fatalf("list open diagnostics: %v", err)
	}
	if len(openRows) != 1 || openRows[0].Subject != "Current open subject" {
		t.Fatalf("reopen collision should leave existing open row unchanged: %+v", openRows)
	}
}

func TestExtractionDiagnostics_EmptyRuleIDScansAsNil(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	if err := ts.RecordExtractionDiagnostic(ctx, api.ExtractionDiagnostic{
		Reader:    "gmail",
		MessageID: "empty-rule-id",
		RuleName:  "No Rule ID",
	}); err != nil {
		t.Fatalf("RecordExtractionDiagnostic: %v", err)
	}

	rows, err := ts.ListExtractionDiagnostics(ctx, store.DiagnosticFilter{Status: store.DiagnosticStatusOpen})
	if err != nil {
		t.Fatalf("ListExtractionDiagnostics: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 diagnostic, got %d", len(rows))
	}
	if rows[0].RuleID != nil {
		t.Fatalf("empty RuleID should scan as nil, got %v", rows[0].RuleID)
	}
}

func TestExtractionDiagnostics_EmptyMessageIDDoesNotDedupe(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()
	first := api.ExtractionDiagnostic{
		Reader:   "gmail",
		Subject:  "First no-message diagnostic",
		RuleName: "No Message Rule",
	}
	second := api.ExtractionDiagnostic{
		Reader:   "gmail",
		Subject:  "Second no-message diagnostic",
		RuleName: "No Message Rule",
	}

	if err := ts.RecordExtractionDiagnostic(ctx, first); err != nil {
		t.Fatalf("record first diagnostic: %v", err)
	}
	if err := ts.RecordExtractionDiagnostic(ctx, second); err != nil {
		t.Fatalf("record second diagnostic: %v", err)
	}

	rows, err := ts.ListExtractionDiagnostics(ctx, store.DiagnosticFilter{Status: store.DiagnosticStatusOpen})
	if err != nil {
		t.Fatalf("ListExtractionDiagnostics: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("empty MessageID diagnostics should not dedupe, got %d rows", len(rows))
	}
	if rows[0].MessageID != "" || rows[1].MessageID != "" {
		t.Fatalf("empty MessageID should scan as empty string: %+v", rows)
	}
}

func TestExtractionDiagnostics_NotFound(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()
	missingID := "22222222-2222-2222-2222-222222222222"

	if _, err := ts.GetExtractionDiagnostic(ctx, missingID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetExtractionDiagnostic should return ErrNotFound, got %v", err)
	}
	if _, err := ts.UpdateExtractionDiagnosticStatus(ctx, missingID, store.DiagnosticStatusResolved); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("UpdateExtractionDiagnosticStatus should return ErrNotFound, got %v", err)
	}
}

func TestExtractionDiagnostics_InvalidStatus(t *testing.T) {
	if err := store.ValidateDiagnosticFilterStatus(store.DiagnosticStatusOpen); err != nil {
		t.Fatalf("open should be valid: %v", err)
	}
	if err := store.ValidateDiagnosticFilterStatus(store.DiagnosticStatusResolved); err != nil {
		t.Fatalf("resolved should be valid: %v", err)
	}
	if err := store.ValidateDiagnosticFilterStatus(store.DiagnosticStatusIgnored); err != nil {
		t.Fatalf("ignored should be valid: %v", err)
	}
	if err := store.ValidateDiagnosticFilterStatus(store.DiagnosticStatusAll); err != nil {
		t.Fatalf("all should be valid: %v", err)
	}
	if err := store.ValidateDiagnosticFilterStatus("pending"); err == nil {
		t.Fatal("pending should be invalid")
	}
}

func TestListTransactions_Pagination(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	for i := range 5 {
		seedTransaction(t, ctx, ts.Store, fmt.Sprintf("msg-%d", i), float64(100*(i+1)), "INR", fmt.Sprintf("Merchant%d", i), "Food")
	}

	// Page 1, size 2.
	txns, result, err := ts.ListTransactions(ctx, store.ListFilter{Page: 1, PageSize: 2})
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if result.Total != 5 {
		t.Errorf("want total=5, got %d", result.Total)
	}
	if len(txns) != 2 {
		t.Errorf("want 2 rows, got %d", len(txns))
	}

	// Page 3, size 2 — only 1 remaining.
	txns, _, err = ts.ListTransactions(ctx, store.ListFilter{Page: 3, PageSize: 2})
	if err != nil {
		t.Fatalf("ListTransactions page 3: %v", err)
	}
	if len(txns) != 1 {
		t.Errorf("want 1 row on last page, got %d", len(txns))
	}
}

func TestListTransactions_FilterByCategory(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	seedTransaction(t, ctx, ts.Store, "food-1", 100, "INR", "Zomato", "Food")
	seedTransaction(t, ctx, ts.Store, "travel-1", 500, "INR", "Uber", "Travel")

	txns, result, err := ts.ListTransactions(
		ctx,
		store.ListFilter{Category: "Food", PageSize: 10},
	)
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("want total=1, got %d", result.Total)
	}
	if len(txns) != 1 || txns[0].Category != "Food" {
		t.Errorf("unexpected transactions: %v", txns)
	}
	if result.TotalAmount != 100 {
		t.Errorf("want totalAmount=100, got %v", result.TotalAmount)
	}
}

func TestListTransactions_FilterMissingCategoryBucketAndLabel(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	missingID, err := ts.InsertForTest(ctx, store.InsertParams{
		MessageID: "missing-taxonomy-1", Amount: 100, Currency: "INR", MerchantInfo: "Unknown",
	})
	if err != nil {
		t.Fatalf("InsertForTest missing: %v", err)
	}
	labeledID, err := ts.InsertForTest(ctx, store.InsertParams{
		MessageID: "labeled-taxonomy-1", Amount: 200, Currency: "INR", MerchantInfo: "Known",
		Category: "Food", Bucket: "Needs",
	})
	if err != nil {
		t.Fatalf("InsertForTest labeled: %v", err)
	}
	if err := ts.AddLabel(ctx, labeledID, "Groceries"); err != nil {
		t.Fatalf("AddLabel: %v", err)
	}

	txns, result, err := ts.ListTransactions(ctx, store.ListFilter{
		CategoryMissing: true,
		BucketMissing:   true,
		LabelMissing:    true,
		PageSize:        10,
	})
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if result.Total != 1 || len(txns) != 1 || txns[0].ID != missingID {
		t.Fatalf("want only missing taxonomy transaction, got total=%d txns=%v", result.Total, txns)
	}
}

func TestDefaultBucketsUseInvestments(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	buckets, err := ts.ListBuckets(ctx)
	if err != nil {
		t.Fatalf("ListBuckets: %v", err)
	}

	foundInvestments := false
	for _, bucket := range buckets {
		if bucket.Name == "Savings" {
			t.Fatalf("default buckets should not include Savings: %v", buckets)
		}
		if bucket.Name == "Investments" && bucket.IsDefault {
			foundInvestments = true
		}
	}
	if !foundInvestments {
		t.Fatalf("default buckets should include Investments: %v", buckets)
	}
}

func TestGetDashboardData_IncludesUncategorizedBreakdowns(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	_, err := ts.InsertForTest(ctx, store.InsertParams{
		MessageID: "dashboard-missing-taxonomy-1", Amount: 125, Currency: "INR", MerchantInfo: "Unknown",
	})
	if err != nil {
		t.Fatalf("InsertForTest missing: %v", err)
	}
	knownID, err := ts.InsertForTest(ctx, store.InsertParams{
		MessageID: "dashboard-known-taxonomy-1", Amount: 250, Currency: "INR", MerchantInfo: "Known",
		Category: "Food", Bucket: "Investments",
	})
	if err != nil {
		t.Fatalf("InsertForTest known: %v", err)
	}
	if err := ts.AddLabel(ctx, knownID, "Groceries"); err != nil {
		t.Fatalf("AddLabel: %v", err)
	}

	data, err := ts.GetDashboardData(ctx)
	if err != nil {
		t.Fatalf("GetDashboardData: %v", err)
	}
	if got := data.AllTime.Charts.ByCategory["Uncategorized"]; got != 125 {
		t.Fatalf("want uncategorized category amount 125, got %v", got)
	}
	if got := data.AllTime.Charts.ByBucket["Uncategorized"]; got != 125 {
		t.Fatalf("want uncategorized bucket amount 125, got %v", got)
	}
	if got := data.AllTime.Charts.ByLabel["Uncategorized"]; got != 125 {
		t.Fatalf("want uncategorized label amount 125, got %v", got)
	}
}

func TestListTransactions_FilterByCurrency(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	seedTransaction(t, ctx, ts.Store, "inr-1", 100, "INR", "Amazon IN", "Shopping")
	seedTransaction(t, ctx, ts.Store, "usd-1", 20, "USD", "Amazon US", "Shopping")

	txns, result, err := ts.ListTransactions(ctx, store.ListFilter{Currency: "USD", PageSize: 10})
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if result.Total != 1 || txns[0].Currency != "USD" {
		t.Errorf("want 1 USD transaction, got total=%d txns=%v", result.Total, txns)
	}
}

func TestListTransactions_FilterByLabel(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	id := seedTransaction(t, ctx, ts.Store, "lbl-1", 200, "INR", "Netflix", "Entertainment")
	seedTransaction(t, ctx, ts.Store, "nolbl-1", 100, "INR", "Spotify", "Entertainment")

	// Add label to first transaction.
	if err := ts.AddLabel(ctx, id, "subscription"); err != nil {
		t.Fatalf("AddLabel: %v", err)
	}

	txns, result, err := ts.ListTransactions(ctx, store.ListFilter{Label: "subscription", PageSize: 10})
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if result.Total != 1 || txns[0].ID != id {
		t.Errorf("want 1 labeled transaction, got total=%d", result.Total)
	}
}

func TestListTransactions_ExcludeCategoriesAndLabels(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	foodID := seedTransaction(t, ctx, ts.Store, "misc-food", 100, "INR", "Zomato", "Food")
	travelID := seedTransaction(t, ctx, ts.Store, "misc-travel", 200, "INR", "Uber", "Travel")
	booksID := seedTransaction(t, ctx, ts.Store, "misc-books", 300, "INR", "Bookshop", "Books")

	if err := ts.AddLabel(ctx, foodID, "top"); err != nil {
		t.Fatalf("AddLabel food: %v", err)
	}
	if err := ts.AddLabel(ctx, travelID, "top"); err != nil {
		t.Fatalf("AddLabel travel: %v", err)
	}
	if err := ts.AddLabel(ctx, booksID, "misc"); err != nil {
		t.Fatalf("AddLabel books: %v", err)
	}

	txns, result, err := ts.ListTransactions(ctx, store.ListFilter{
		ExcludeCategories: []string{"Food", "Travel"},
		PageSize:          10,
	})
	if err != nil {
		t.Fatalf("ListTransactions exclude categories: %v", err)
	}
	if result.Total != 1 || len(txns) != 1 || txns[0].Category != "Books" {
		t.Fatalf("want only Books after excluding Food/Travel, got total=%d txns=%v", result.Total, txns)
	}

	txns, result, err = ts.ListTransactions(ctx, store.ListFilter{
		ExcludeLabels: []string{"top"},
		PageSize:      10,
	})
	if err != nil {
		t.Fatalf("ListTransactions exclude labels: %v", err)
	}
	if result.Total != 1 || len(txns) != 1 || txns[0].ID != booksID {
		t.Fatalf("want only misc-labeled transaction after excluding top, got total=%d txns=%v", result.Total, txns)
	}
}

func TestGetTransaction_Found(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	id := seedTxn(t, ctx, ts.Store, "get-1", 999, "INR", "Apple Store", "Tech", "iPad Pro")

	txn, err := ts.GetTransaction(ctx, id)
	if err != nil {
		t.Fatalf("GetTransaction: %v", err)
	}
	if txn == nil {
		t.Fatal("expected transaction, got nil")
	}
	if txn.MerchantInfo != "Apple Store" {
		t.Errorf("want MerchantInfo=Apple Store, got %s", txn.MerchantInfo)
	}
	if txn.Description != "iPad Pro" {
		t.Errorf("want Description=iPad Pro, got %s", txn.Description)
	}
}

func TestGetTransaction_NotFound(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	_, err := ts.GetTransaction(ctx, "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdateDescription(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	id := seedTransaction(t, ctx, ts.Store, "upd-1", 50, "INR", "Swiggy", "Food")

	if err := ts.UpdateDescription(ctx, id, "Lunch with team"); err != nil {
		t.Fatalf("UpdateDescription: %v", err)
	}

	txn, err := ts.GetTransaction(ctx, id)
	if err != nil || txn == nil {
		t.Fatalf("GetTransaction after update: %v", err)
	}
	if txn.Description != "Lunch with team" {
		t.Errorf("want description='Lunch with team', got %q", txn.Description)
	}
}

func TestUpdateDescription_NotFound(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	err := ts.UpdateDescription(ctx, "00000000-0000-0000-0000-000000000000", "nope")
	if err == nil {
		t.Fatal("expected ErrNotFound, got nil")
	}
}

func TestAddLabel_Idempotent(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	id := seedTransaction(t, ctx, ts.Store, "addlbl-1", 300, "INR", "BookMyShow", "Entertainment")

	// Add same label twice — should not error.
	if err := ts.AddLabel(ctx, id, "leisure"); err != nil {
		t.Fatalf("first AddLabel: %v", err)
	}
	if err := ts.AddLabel(ctx, id, "leisure"); err != nil {
		t.Fatalf("duplicate AddLabel should not error: %v", err)
	}

	txn, _ := ts.GetTransaction(ctx, id)
	if len(txn.Labels) != 1 || txn.Labels[0] != "leisure" {
		t.Errorf("expected 1 label 'leisure', got %v", txn.Labels)
	}
}

func TestRemoveLabel(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	id := seedTransaction(t, ctx, ts.Store, "rmlbl-1", 250, "INR", "Myntra", "Shopping")
	_ = ts.AddLabel(ctx, id, "clothing")

	if err := ts.RemoveLabel(ctx, id, "clothing"); err != nil {
		t.Fatalf("RemoveLabel: %v", err)
	}

	txn, _ := ts.GetTransaction(ctx, id)
	if len(txn.Labels) != 0 {
		t.Errorf("expected 0 labels after removal, got %v", txn.Labels)
	}
}

func TestRemoveLabel_NotFound(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	id := seedTransaction(t, ctx, ts.Store, "rmlbl-nf", 100, "INR", "Store", "Misc")
	err := ts.RemoveLabel(ctx, id, "nonexistent")
	if err == nil {
		t.Fatal("expected ErrNotFound, got nil")
	}
}

func TestApplyLabelByMerchant_PersistsMapping(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	if err := ts.CreateLabel(ctx, "subscription", "#f59e0b"); err != nil {
		t.Fatalf("CreateLabel: %v", err)
	}

	affected, err := ts.ApplyLabelByMerchant(ctx, "subscription", "Netflix")
	if err != nil {
		t.Fatalf("ApplyLabelByMerchant: %v", err)
	}
	if affected != 0 {
		t.Fatalf("want 0 affected transactions, got %d", affected)
	}

	mappings, err := ts.GetLabelMappings(ctx)
	if err != nil {
		t.Fatalf("GetLabelMappings: %v", err)
	}
	got := mappings["subscription"]
	if len(got) != 1 || got[0] != "Netflix" {
		t.Fatalf("want persisted mapping [Netflix], got %v", got)
	}
}

func TestRemoveLabelByMerchant_RemovesMappingAndBackfilledLabels(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	if err := ts.CreateLabel(ctx, "subscription", "#f59e0b"); err != nil {
		t.Fatalf("CreateLabel: %v", err)
	}

	id := seedTransaction(t, ctx, ts.Store, "lbl-merchant-1", 200, "INR", "Netflix", "Entertainment")

	affected, err := ts.ApplyLabelByMerchant(ctx, "subscription", "Netflix")
	if err != nil {
		t.Fatalf("ApplyLabelByMerchant: %v", err)
	}
	if affected != 1 {
		t.Fatalf("want 1 affected transaction, got %d", affected)
	}

	txn, err := ts.GetTransaction(ctx, id)
	if err != nil {
		t.Fatalf("GetTransaction before remove: %v", err)
	}
	if len(txn.Labels) != 1 || txn.Labels[0] != "subscription" {
		t.Fatalf("want backfilled label [subscription], got %v", txn.Labels)
	}

	removed, err := ts.RemoveLabelByMerchant(ctx, "subscription", "Netflix")
	if err != nil {
		t.Fatalf("RemoveLabelByMerchant: %v", err)
	}
	if removed != 1 {
		t.Fatalf("want 1 removed transaction label, got %d", removed)
	}

	mappings, err := ts.GetLabelMappings(ctx)
	if err != nil {
		t.Fatalf("GetLabelMappings after remove: %v", err)
	}
	if got := mappings["subscription"]; len(got) != 0 {
		t.Fatalf("want mapping removed, got %v", got)
	}

	txn, err = ts.GetTransaction(ctx, id)
	if err != nil {
		t.Fatalf("GetTransaction after remove: %v", err)
	}
	if len(txn.Labels) != 0 {
		t.Fatalf("want label removed from transaction, got %v", txn.Labels)
	}
}

func TestRemoveLabelByMerchant_PreservesManualLabelSources(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	if err := ts.CreateLabel(ctx, "subscription", "#f59e0b"); err != nil {
		t.Fatalf("CreateLabel: %v", err)
	}

	id := seedTransaction(t, ctx, ts.Store, "lbl-manual-merchant-1", 200, "INR", "Netflix", "Entertainment")

	if err := ts.AddLabel(ctx, id, "subscription"); err != nil {
		t.Fatalf("AddLabel: %v", err)
	}

	affected, err := ts.ApplyLabelByMerchant(ctx, "subscription", "Netflix")
	if err != nil {
		t.Fatalf("ApplyLabelByMerchant: %v", err)
	}
	if affected != 0 {
		t.Fatalf("want 0 affected transactions after manual label, got %d", affected)
	}

	removed, err := ts.RemoveLabelByMerchant(ctx, "subscription", "Netflix")
	if err != nil {
		t.Fatalf("RemoveLabelByMerchant: %v", err)
	}
	if removed != 0 {
		t.Fatalf("want 0 removed transaction labels after manual source remains, got %d", removed)
	}

	txn, err := ts.GetTransaction(ctx, id)
	if err != nil {
		t.Fatalf("GetTransaction: %v", err)
	}
	if len(txn.Labels) != 1 || txn.Labels[0] != "subscription" {
		t.Fatalf("manual label should remain, got %v", txn.Labels)
	}

	mappings, err := ts.GetLabelMappings(ctx)
	if err != nil {
		t.Fatalf("GetLabelMappings: %v", err)
	}
	if got := mappings["subscription"]; len(got) != 0 {
		t.Fatalf("merchant mapping should be removed, got %v", got)
	}
}

func TestRemoveLabelByMerchant_PreservesOtherMerchantSources(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	if err := ts.CreateLabel(ctx, "delivery", "#f59e0b"); err != nil {
		t.Fatalf("CreateLabel: %v", err)
	}

	id := seedTransaction(t, ctx, ts.Store, "lbl-overlap-merchant-1", 350, "INR", "Uber Eats Pass", "Entertainment")

	if affected, err := ts.ApplyLabelByMerchant(ctx, "delivery", "Uber"); err != nil {
		t.Fatalf("ApplyLabelByMerchant Uber: %v", err)
	} else if affected != 1 {
		t.Fatalf("want 1 affected transaction for Uber, got %d", affected)
	}
	if affected, err := ts.ApplyLabelByMerchant(ctx, "delivery", "Uber Eats"); err != nil {
		t.Fatalf("ApplyLabelByMerchant Uber Eats: %v", err)
	} else if affected != 0 {
		t.Fatalf("want 0 affected transactions for overlapping mapping, got %d", affected)
	}

	removed, err := ts.RemoveLabelByMerchant(ctx, "delivery", "Uber")
	if err != nil {
		t.Fatalf("RemoveLabelByMerchant: %v", err)
	}
	if removed != 0 {
		t.Fatalf("want 0 removed transaction labels while another mapping still applies, got %d", removed)
	}

	txn, err := ts.GetTransaction(ctx, id)
	if err != nil {
		t.Fatalf("GetTransaction: %v", err)
	}
	if len(txn.Labels) != 1 || txn.Labels[0] != "delivery" {
		t.Fatalf("label should remain because Uber Eats still applies, got %v", txn.Labels)
	}

	mappings, err := ts.GetLabelMappings(ctx)
	if err != nil {
		t.Fatalf("GetLabelMappings: %v", err)
	}
	got := mappings["delivery"]
	if len(got) != 1 || got[0] != "Uber Eats" {
		t.Fatalf("want remaining mapping [Uber Eats], got %v", got)
	}
}

func TestSearchTransactions(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	seedTransaction(t, ctx, ts.Store, "srch-1", 150, "INR", "Starbucks Coffee", "Food")
	seedTransaction(t, ctx, ts.Store, "srch-2", 200, "INR", "Pizza Hut", "Food")

	txns, result, err := ts.SearchTransactions(
		ctx,
		"Starbucks",
		store.ListFilter{PageSize: 10},
	)
	if err != nil {
		t.Fatalf("SearchTransactions: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("want 1 result for 'Starbucks', got %d", result.Total)
	}
	if len(txns) != 1 || txns[0].MerchantInfo != "Starbucks Coffee" {
		t.Errorf("unexpected result: %v", txns)
	}
	if result.TotalAmount != 150 {
		t.Errorf("want totalAmount=150, got %v", result.TotalAmount)
	}
}

func TestSearchTransactions_EmptyQuery(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	seedTransaction(t, ctx, ts.Store, "srch-all-1", 100, "INR", "Any Shop", "Misc")
	seedTransaction(t, ctx, ts.Store, "srch-all-2", 200, "INR", "Another Shop", "Misc")

	// Empty query should return all.
	_, result, err := ts.SearchTransactions(ctx, "", store.ListFilter{PageSize: 10})
	if err != nil {
		t.Fatalf("SearchTransactions empty: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("empty query should return all 2 transactions, got %d", result.Total)
	}
}

func TestSearchTransactions_SpecialCharactersDoNotError(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	seedTransaction(t, ctx, ts.Store, "srch-special-1", 100, "INR", "Cafe Delight", "Food")

	_, _, err := ts.SearchTransactions(ctx, "í)", store.ListFilter{PageSize: 10})
	if err != nil {
		t.Fatalf("SearchTransactions special chars: %v", err)
	}
}

func TestSearchTransactions_SubstringMerchantMatch(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	seedTxn(t, ctx, ts.Store, "srch-substring-1", 250, "INR", "Swiggy Instamart", "Food", "groceries")

	rows, _, err := ts.SearchTransactions(ctx, "insta", store.ListFilter{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("SearchTransactions: %v", err)
	}
	if len(rows) != 1 || rows[0].MerchantInfo != "Swiggy Instamart" {
		t.Fatalf("unexpected search rows: %+v", rows)
	}
}

func TestSearchTransactions_WebStyleQuery(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	seedTxn(t, ctx, ts.Store, "srch-web-1", 1499, "INR", "Amazon Pay", "Shopping", "prime membership")

	rows, _, err := ts.SearchTransactions(ctx, "amazon prime", store.ListFilter{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("SearchTransactions: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one row, got %+v", rows)
	}
}

func TestGetStats(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	seedTransaction(t, ctx, ts.Store, "stats-1", 100, "INR", "M1", "Food")
	seedTransaction(t, ctx, ts.Store, "stats-2", 200, "INR", "M2", "Food")
	seedTransaction(t, ctx, ts.Store, "stats-3", 50, "USD", "M3", "Other") // excluded from INR total

	stats, err := ts.GetStats(ctx, "INR")
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.TotalCount != 3 {
		t.Errorf("want TotalCount=3 (all currencies), got %d", stats.TotalCount)
	}
	if stats.TotalBase != 300 {
		t.Errorf("want TotalBase=300, got %f", stats.TotalBase)
	}
	if stats.BaseCurrency != "INR" {
		t.Errorf("want BaseCurrency=INR, got %s", stats.BaseCurrency)
	}
}

func TestGetSpendingHeatmap_EmptyDB(t *testing.T) {
	ts := newTestStore(t) // skips automatically when -short is passed
	defer ts.cleanup()

	hd, err := ts.GetSpendingHeatmap(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hd.ByWeekdayHour == nil {
		t.Error("ByWeekdayHour must be a non-nil slice, got nil")
	}
	if hd.ByDayOfMonth == nil {
		t.Error("ByDayOfMonth must be a non-nil slice, got nil")
	}
	if len(hd.ByWeekdayHour) != 0 {
		t.Errorf("expected 0 weekday/hour buckets in empty DB, got %d", len(hd.ByWeekdayHour))
	}
	if len(hd.ByDayOfMonth) != 0 {
		t.Errorf("expected 0 day-of-month buckets in empty DB, got %d", len(hd.ByDayOfMonth))
	}
}

func TestHeatmapBucketMatchesListTransactionsForWeekdayHour(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()

	ctx := context.Background()
	if err := ts.SetAppConfig(ctx, "app.timezone", "Asia/Kolkata"); err != nil {
		t.Fatalf("SetAppConfig: %v", err)
	}

	from := time.Date(2026, time.April, 3, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.April, 3, 23, 59, 59, 0, time.UTC)
	timestamps := []time.Time{
		time.Date(2026, time.April, 3, 3, 45, 0, 0, time.UTC),
		time.Date(2026, time.April, 3, 4, 25, 0, 0, time.UTC),
	}

	for i, tsAt := range timestamps {
		if _, err := ts.InsertForTest(ctx, store.InsertParams{
			MessageID:    fmt.Sprintf("heatmap-weekday-hour-%d", i),
			Amount:       100,
			Currency:     "INR",
			MerchantInfo: fmt.Sprintf("Merchant %d", i),
			Category:     "Food",
			Timestamp:    tsAt,
		}); err != nil {
			t.Fatalf("InsertForTest(%d): %v", i, err)
		}
	}

	heatmap, err := ts.GetSpendingHeatmap(ctx, &from, &to)
	if err != nil {
		t.Fatalf("GetSpendingHeatmap: %v", err)
	}

	var heatmapCount int
	for _, bucket := range heatmap.ByWeekdayHour {
		if bucket.Weekday == 5 && bucket.Hour == 9 {
			heatmapCount = bucket.Count
			break
		}
	}

	txns, result, err := ts.ListTransactions(ctx, store.ListFilter{
		Page:     1,
		PageSize: 20,
		From:     &from,
		To:       &to,
		Weekday:  ptrInt(5),
		HourFrom: ptrInt(9),
		HourTo:   ptrInt(9),
		Timezone: "Asia/Kolkata",
	})
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}

	if len(txns) != result.Total {
		t.Fatalf("expected len(txns) == total, got %d and %d", len(txns), result.Total)
	}
	if heatmapCount != result.Total {
		t.Fatalf("expected heatmap count %d to equal drilldown total %d", heatmapCount, result.Total)
	}
}

func TestHeatmapBucketMatchesListTransactionsForWeekdayHour_AllTime(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()

	ctx := context.Background()
	if err := ts.SetAppConfig(ctx, "app.timezone", "Asia/Calcutta"); err != nil {
		t.Fatalf("SetAppConfig: %v", err)
	}

	timestamps := []struct {
		at     time.Time
		amount float64
	}{
		{at: time.Date(2026, time.April, 5, 17, 35, 0, 0, time.UTC), amount: 1234.56},
		{at: time.Date(2026, time.April, 5, 18, 20, 0, 0, time.UTC), amount: 7890.12},
		{at: time.Date(2026, time.April, 5, 18, 40, 0, 0, time.UTC), amount: 50},
	}

	for i, txn := range timestamps {
		if _, err := ts.InsertForTest(ctx, store.InsertParams{
			MessageID:    fmt.Sprintf("heatmap-all-time-%d", i),
			Amount:       txn.amount,
			Currency:     "INR",
			MerchantInfo: fmt.Sprintf("Merchant %d", i),
			Category:     "Food",
			Timestamp:    txn.at,
		}); err != nil {
			t.Fatalf("InsertForTest(%d): %v", i, err)
		}
	}

	heatmap, err := ts.GetSpendingHeatmap(ctx, nil, nil)
	if err != nil {
		t.Fatalf("GetSpendingHeatmap: %v", err)
	}

	var bucket *store.WeekdayHourBucket
	for i := range heatmap.ByWeekdayHour {
		if heatmap.ByWeekdayHour[i].Weekday == 0 && heatmap.ByWeekdayHour[i].Hour == 23 {
			bucket = &heatmap.ByWeekdayHour[i]
			break
		}
	}
	if bucket == nil {
		t.Fatal("expected Sunday 23:00 bucket in all-time heatmap")
	}

	txns, result, err := ts.ListTransactions(ctx, store.ListFilter{
		Page:     1,
		PageSize: 20,
		Weekday:  ptrInt(0),
		HourFrom: ptrInt(23),
		HourTo:   ptrInt(23),
		Timezone: "Asia/Calcutta",
	})
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}

	if bucket.Count != result.Total {
		t.Fatalf("bucket count %d != list total %d", bucket.Count, result.Total)
	}
	var listedAmount float64
	for _, tx := range txns {
		listedAmount += tx.Amount
	}
	if listedAmount != bucket.Amount {
		t.Fatalf("bucket amount %v != listed amount %v", bucket.Amount, listedAmount)
	}
}

func TestGetSpendingHeatmap_DayOfMonthUsesLocalTimezone(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()

	ctx := context.Background()
	if err := ts.SetAppConfig(ctx, "app.timezone", "Asia/Kolkata"); err != nil {
		t.Fatalf("SetAppConfig: %v", err)
	}

	at := time.Date(2026, time.April, 30, 20, 30, 0, 0, time.UTC)
	from := time.Date(2026, time.April, 30, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.May, 1, 23, 59, 59, 0, time.UTC)
	if _, err := ts.InsertForTest(ctx, store.InsertParams{
		MessageID:    "heatmap-day-rollover",
		Amount:       250,
		Currency:     "INR",
		MerchantInfo: "Boundary Merchant",
		Category:     "Food",
		Timestamp:    at,
	}); err != nil {
		t.Fatalf("InsertForTest: %v", err)
	}

	heatmap, err := ts.GetSpendingHeatmap(ctx, &from, &to)
	if err != nil {
		t.Fatalf("GetSpendingHeatmap: %v", err)
	}

	var localDayCount int
	for _, bucket := range heatmap.ByDayOfMonth {
		if bucket.Day == 1 {
			localDayCount = bucket.Count
		}
		if bucket.Day == 30 {
			t.Fatalf("expected UTC timestamp to roll into local day 1, but found day 30 bucket: %+v", bucket)
		}
	}
	if localDayCount != 1 {
		t.Fatalf("expected local day 1 count 1, got %d", localDayCount)
	}
}

func TestGetAnnualSpend_EmptyDB(t *testing.T) {
	ts := newTestStore(t) // skips when -short
	defer ts.cleanup()

	buckets, err := ts.GetAnnualSpend(context.Background(), 2026)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buckets == nil {
		t.Error("GetAnnualSpend must return a non-nil slice, got nil")
	}
	if len(buckets) != 0 {
		t.Errorf("expected 0 buckets in empty DB, got %d", len(buckets))
	}
}

func TestListRules_EmptyDB(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()

	rules, err := ts.ListRules(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rules == nil {
		t.Error("ListRules must return non-nil slice")
	}
	if len(rules) != 0 {
		t.Errorf("expected 0 rules in empty DB, got %d", len(rules))
	}
}

func TestCreateAndGetRule(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()

	row := store.RuleRow{
		Name:          "test rule",
		AmountRegex:   `(\d+)`,
		MerchantRegex: `(.+)`,
	}
	created, err := ts.CreateRule(context.Background(), row)
	if err != nil {
		t.Fatalf("CreateRule: %v", err)
	}
	if created.ID == "" {
		t.Error("expected non-empty ID after create")
	}
	if created.Predefined {
		t.Error("expected predefined=false for user-created rule")
	}

	got, err := ts.GetRule(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetRule: %v", err)
	}
	if got.Name != "test rule" {
		t.Errorf("expected name=test rule, got %q", got.Name)
	}
}

func TestDeleteRule_PredefinedRuleNotDeleted(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()

	err := ts.SeedPredefinedRules(context.Background(), []store.RuleRow{
		{Name: "sys", AmountRegex: `(\d+)`, MerchantRegex: `(.+)`},
	})
	if err != nil {
		t.Fatalf("SeedPredefinedRules: %v", err)
	}
	rules, err := ts.ListRules(context.Background())
	if err != nil {
		t.Fatalf("ListRules: %v", err)
	}
	if len(rules) == 0 {
		t.Fatal("expected seeded predefined rule")
	}
	predefinedRule := rules[0]

	delErr := ts.DeleteRule(context.Background(), predefinedRule.ID)
	if !errors.Is(delErr, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound when deleting predefined rule, got %v", delErr)
	}
}

func TestPredefinedRulesV2MigrationRemovesStaleV1Rules(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	_, err := ts.PoolForTest().Exec(ctx, `
		INSERT INTO rules
			(name, sender_email, subject_contains, amount_regex, merchant_regex, transaction_source, predefined)
		VALUES
			('HDFC Credit Card (debit alert)', '@hdfcbank', 'debited via Credit Card', '(\d+)', '(.+)', 'Credit Card - HDFC', true),
			('HDFC Credit Card', '@hdfcbank', 'Alert : Update on your HDFC Bank Credit Card', '(\d+)', '(.+)', 'Credit Card - HDFC', true),
			('Custom Legacy Rule', '@legacy', 'legacy subject', '(\d+)', '(.+)', 'Legacy Source', false)
	`)
	if err != nil {
		t.Fatalf("seed legacy rules: %v", err)
	}

	migrationSQL, err := fs.ReadFile(migrations.FS, "003_predefined_rules_v2.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err = ts.PoolForTest().Exec(ctx, string(migrationSQL)); err != nil {
		t.Fatalf("run migration: %v", err)
	}

	var exists bool
	if err = ts.PoolForTest().QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM rules WHERE name = 'HDFC Credit Card (debit alert)' AND predefined = true)`,
	).Scan(&exists); err != nil {
		t.Fatalf("check stale predefined rule: %v", err)
	}
	if exists {
		t.Fatal("expected stale v1 predefined rule to be removed")
	}

	if err = ts.PoolForTest().QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM rules WHERE name = 'Custom Legacy Rule' AND predefined = false)`,
	).Scan(&exists); err != nil {
		t.Fatalf("check custom legacy rule: %v", err)
	}
	if !exists {
		t.Fatal("expected custom legacy rule to remain")
	}

	var senderEmails []string
	var sourceType, sourceLabel, bank string
	if err = ts.PoolForTest().QueryRow(ctx, `
		SELECT sender_emails, source_type, source_label, bank
		FROM rules
		WHERE name = 'HDFC Credit Card' AND predefined = true
	`).Scan(&senderEmails, &sourceType, &sourceLabel, &bank); err != nil {
		t.Fatalf("check migrated predefined rule: %v", err)
	}
	wantSenders := []string{"alerts@hdfcbank.bank.in", "alerts@hdfcbank.net"}
	if !reflect.DeepEqual(senderEmails, wantSenders) {
		t.Fatalf("sender_emails = %#v, want %#v", senderEmails, wantSenders)
	}
	if sourceType != "Credit Card" || sourceLabel != "HDFC Credit Card" || bank != "HDFC" {
		t.Fatalf("source fields = (%q, %q, %q), want (Credit Card, HDFC Credit Card, HDFC)", sourceType, sourceLabel, bank)
	}
}

func TestListTransactions_FilterByBucket(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	id1, err := ts.InsertForTest(ctx, store.InsertParams{
		MessageID: "bucket-wants", Amount: 100, Currency: "INR",
		MerchantInfo: "Netflix", Category: "Entertainment", Bucket: "wants",
	})
	if err != nil {
		t.Fatalf("seed wants: %v", err)
	}
	if _, err = ts.InsertForTest(ctx, store.InsertParams{
		MessageID: "bucket-needs", Amount: 200, Currency: "INR",
		MerchantInfo: "Rent", Category: "Housing", Bucket: "needs",
	}); err != nil {
		t.Fatalf("seed needs: %v", err)
	}

	txns, result, err := ts.ListTransactions(ctx, store.ListFilter{Bucket: "wants", PageSize: 10})
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("want total=1, got %d", result.Total)
	}
	if len(txns) != 1 || txns[0].ID != id1 {
		t.Errorf("want txn %s, got %v", id1, txns)
	}
}

func TestGetFacets_IncludesBuckets(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	for _, p := range []store.InsertParams{
		{MessageID: "fct-1", Amount: 100, Currency: "INR", MerchantInfo: "Netflix", Category: "Entertainment", Bucket: "wants"},
		{MessageID: "fct-2", Amount: 200, Currency: "INR", MerchantInfo: "Rent", Category: "Housing", Bucket: "needs"},
	} {
		if _, err := ts.InsertForTest(ctx, p); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	facets, err := ts.GetFacets(ctx)
	if err != nil {
		t.Fatalf("GetFacets: %v", err)
	}
	contains := func(s []string, v string) bool {
		for _, x := range s {
			if x == v {
				return true
			}
		}
		return false
	}
	if !contains(facets.Buckets, "wants") || !contains(facets.Buckets, "needs") {
		t.Errorf("want [needs wants] in Buckets, got %v", facets.Buckets)
	}
	if !contains(facets.Merchants, "Netflix") || !contains(facets.Merchants, "Rent") {
		t.Errorf("want transaction merchants in Merchants, got %v", facets.Merchants)
	}
}

func TestGetFacets_IncludesLabelCounts(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	id1 := seedTransaction(t, ctx, ts.Store, "facet-label-count-1", 100, "INR", "Merchant A", "Food")
	id2 := seedTransaction(t, ctx, ts.Store, "facet-label-count-2", 200, "INR", "Merchant B", "Food")

	if err := ts.AddLabels(ctx, id1, []string{"counted-label"}); err != nil {
		t.Fatalf("AddLabels id1: %v", err)
	}
	if err := ts.AddLabels(ctx, id2, []string{"counted-label"}); err != nil {
		t.Fatalf("AddLabels id2: %v", err)
	}

	facets, err := ts.GetFacets(ctx)
	if err != nil {
		t.Fatalf("GetFacets: %v", err)
	}
	if got := facets.LabelCounts["counted-label"]; got != 2 {
		t.Fatalf("want counted-label count 2, got %d from %#v", got, facets.LabelCounts)
	}
}

func TestRulesRepository_PersistsSenderEmailsAndSourceFields(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	created, err := ts.CreateRule(ctx, store.RuleRow{
		Name:            "Structured HDFC",
		SenderEmails:    []string{"alerts@hdfcbank.net", "alerts@hdfcbank.bank.in"},
		SubjectContains: "HDFC Credit Card",
		AmountRegex:     `Rs\.([\d.]+)`,
		MerchantRegex:   `at (.*?) on`,
		SourceType:      "Credit Card",
		SourceLabel:     "HDFC Credit Card",
		Bank:            "HDFC",
	})
	if err != nil {
		t.Fatalf("CreateRule: %v", err)
	}

	got, err := ts.GetRule(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetRule: %v", err)
	}
	if !reflect.DeepEqual(got.SenderEmails, []string{"alerts@hdfcbank.net", "alerts@hdfcbank.bank.in"}) {
		t.Fatalf("sender emails = %#v", got.SenderEmails)
	}
	if got.SourceType != "Credit Card" || got.SourceLabel != "HDFC Credit Card" || got.Bank != "HDFC" {
		t.Fatalf("source fields = (%q, %q, %q)", got.SourceType, got.SourceLabel, got.Bank)
	}
}

func TestTransactionsStructuredSourceFacetsAndFilters(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	seed := []store.InsertParams{
		{MessageID: "structured-source-1", Amount: 100, MerchantInfo: "Amazon", SourceType: "Credit Card", SourceLabel: "HDFC Credit Card", Bank: "HDFC"},
		{MessageID: "structured-source-2", Amount: 200, MerchantInfo: "Swiggy", SourceType: "UPI", SourceLabel: "ICICI UPI", Bank: "ICICI"},
		{MessageID: "structured-source-3", Amount: 300, MerchantInfo: "Uber", SourceType: "Credit Card", SourceLabel: "ICICI Credit Card", Bank: "ICICI"},
	}
	for _, p := range seed {
		if _, err := ts.InsertForTest(ctx, p); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	facets, err := ts.GetFacets(ctx)
	if err != nil {
		t.Fatalf("GetFacets: %v", err)
	}
	if !reflect.DeepEqual(facets.SourceTypes, []string{"Credit Card", "UPI"}) {
		t.Fatalf("source types = %#v", facets.SourceTypes)
	}
	if !reflect.DeepEqual(facets.Banks, []string{"HDFC", "ICICI"}) {
		t.Fatalf("banks = %#v", facets.Banks)
	}

	txns, _, err := ts.ListTransactions(ctx, store.ListFilter{SourceType: "Credit Card", Bank: "HDFC", PageSize: 10})
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if len(txns) != 1 || txns[0].MessageID != "structured-source-1" {
		t.Fatalf("filtered transactions = %#v", txns)
	}
	if txns[0].Source.Type != "Credit Card" || txns[0].Source.Bank != "HDFC" || txns[0].Source.Label != "HDFC Credit Card" {
		t.Fatalf("transaction source = %#v", txns[0].Source)
	}
}

func TestGetChartData_BySource(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	for _, p := range []store.InsertParams{
		{MessageID: "src-1", Amount: 100, Currency: "INR", MerchantInfo: "Amazon", Category: "Shopping", Source: "HDFC Credit Card", SourceType: "Credit Card", Bank: "HDFC"},
		{MessageID: "src-2", Amount: 200, Currency: "INR", MerchantInfo: "Swiggy", Category: "Food", Source: "SBI Debit Card", SourceType: "Debit Card", Bank: "SBI"},
		{MessageID: "src-3", Amount: 50, Currency: "INR", MerchantInfo: "Netflix", Category: "Entertainment", Source: "HDFC Credit Card", SourceType: "Credit Card", Bank: "HDFC"},
	} {
		if _, err := ts.InsertForTest(ctx, p); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	cd, err := ts.GetChartData(ctx)
	if err != nil {
		t.Fatalf("GetChartData: %v", err)
	}
	if cd.BySource["HDFC Credit Card"] != 150 {
		t.Errorf("want HDFC=150, got %v", cd.BySource["HDFC Credit Card"])
	}
	if cd.BySource["SBI Debit Card"] != 200 {
		t.Errorf("want SBI=200, got %v", cd.BySource["SBI Debit Card"])
	}
	if cd.BySourceType["Credit Card"] != 150 {
		t.Errorf("want Credit Card=150, got %v", cd.BySourceType["Credit Card"])
	}
	if cd.BySourceType["Debit Card"] != 200 {
		t.Errorf("want Debit Card=200, got %v", cd.BySourceType["Debit Card"])
	}
	if cd.ByBank["HDFC"] != 150 {
		t.Errorf("want HDFC bank=150, got %v", cd.ByBank["HDFC"])
	}
	if cd.ByBank["SBI"] != 200 {
		t.Errorf("want SBI bank=200, got %v", cd.ByBank["SBI"])
	}
}

func TestGetCurrentMonthDashboardData_UsesConfiguredTimezone(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	if err := ts.SetAppConfig(ctx, "app.timezone", "Asia/Kolkata"); err != nil {
		t.Fatalf("SetAppConfig: %v", err)
	}

	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}

	nowLocal := time.Now().In(loc)
	startLocal := time.Date(nowLocal.Year(), nowLocal.Month(), 1, 0, 15, 0, 0, loc)
	boundaryUTC := startLocal.UTC()
	if boundaryUTC.Month() == startLocal.Month() {
		t.Fatalf("test setup expected UTC month boundary rollover, got local=%s utc=%s", startLocal, boundaryUTC)
	}

	if _, err := ts.InsertForTest(ctx, store.InsertParams{
		MessageID:    "month-boundary",
		Amount:       1000,
		Currency:     "INR",
		MerchantInfo: "Boundary Shop",
		Category:     "Shopping",
		Bucket:       "Wants",
		Timestamp:    boundaryUTC,
	}); err != nil {
		t.Fatalf("InsertForTest: %v", err)
	}

	data, err := ts.GetDashboardData(ctx)
	if err != nil {
		t.Fatalf("GetDashboardData: %v", err)
	}
	if data.CurrentMonth.Stats.TotalCount != 1 {
		t.Fatalf("expected current-month total_count=1, got %d", data.CurrentMonth.Stats.TotalCount)
	}
	if data.CurrentMonth.Stats.TotalBase != 1000 {
		t.Fatalf("expected current-month total_base=1000, got %f", data.CurrentMonth.Stats.TotalBase)
	}
}

func TestGetChartData_MonthlySpendUsesConfiguredTimezone(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	if err := ts.SetAppConfig(ctx, "app.timezone", "Asia/Kolkata"); err != nil {
		t.Fatalf("SetAppConfig: %v", err)
	}

	restoreNow := ts.SetNowForTestSequence(time.Date(2026, time.April, 15, 12, 0, 0, 0, time.UTC))
	defer restoreNow()

	if _, err := ts.InsertForTest(ctx, store.InsertParams{
		MessageID:    "all-time-month-boundary",
		Amount:       750,
		Currency:     "INR",
		MerchantInfo: "Boundary Monthly Shop",
		Category:     "Shopping",
		Timestamp:    time.Date(2026, time.March, 31, 20, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("InsertForTest: %v", err)
	}

	cd, err := ts.GetChartData(ctx)
	if err != nil {
		t.Fatalf("GetChartData: %v", err)
	}

	if len(cd.MonthlySpend) != 1 {
		t.Fatalf("expected 1 monthly bucket, got %d", len(cd.MonthlySpend))
	}
	if cd.MonthlySpend[0].Period != "2026-04" {
		t.Fatalf("expected monthly bucket 2026-04 in app timezone, got %q", cd.MonthlySpend[0].Period)
	}
	if cd.MonthlySpend[0].Amount != 750 {
		t.Fatalf("expected monthly bucket amount 750, got %f", cd.MonthlySpend[0].Amount)
	}
	if len(cd.DailySpend) != 1 {
		t.Fatalf("expected 1 daily bucket, got %d", len(cd.DailySpend))
	}
	if cd.DailySpend[0].Period != "2026-04-01" {
		t.Fatalf("expected daily bucket 2026-04-01 in app timezone, got %q", cd.DailySpend[0].Period)
	}
	if cd.DailySpend[0].Amount != 750 {
		t.Fatalf("expected daily bucket amount 750, got %f", cd.DailySpend[0].Amount)
	}
}

func TestGetDashboardData_UsesSingleNowAcrossSections(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	if err := ts.SetAppConfig(ctx, "app.timezone", "Asia/Kolkata"); err != nil {
		t.Fatalf("SetAppConfig: %v", err)
	}

	restoreNow := ts.SetNowForTestSequence(
		time.Date(2026, time.March, 31, 18, 0, 0, 0, time.UTC),
		time.Date(2026, time.March, 31, 19, 0, 0, 0, time.UTC),
		time.Date(2026, time.March, 31, 19, 0, 0, 0, time.UTC),
	)
	defer restoreNow()

	if _, err := ts.InsertForTest(ctx, store.InsertParams{
		MessageID:    "dashboard-now-consistency",
		Amount:       750,
		Currency:     "INR",
		MerchantInfo: "Boundary Shop",
		Category:     "Shopping",
		Timestamp:    time.Date(2026, time.March, 31, 20, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("InsertForTest: %v", err)
	}

	data, err := ts.GetDashboardData(ctx)
	if err != nil {
		t.Fatalf("GetDashboardData: %v", err)
	}

	if data.CurrentMonth.Label != "March 2026" {
		t.Fatalf("expected current-month label March 2026, got %q", data.CurrentMonth.Label)
	}
	if data.CurrentMonth.Stats.TotalCount != 0 {
		t.Fatalf("expected current-month total_count=0, got %d", data.CurrentMonth.Stats.TotalCount)
	}
	if got := data.AllTime.Charts.ByCategoryMonthly["Shopping"]; got != (store.CategoryMonthlyEntry{}) {
		t.Fatalf("expected all-time category monthly to exclude April spend for March snapshot, got %+v", got)
	}
}

func TestCategorizeMerchant(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	// Seed 3 Netflix transactions, 1 from a different merchant.
	id1 := seedTransaction(t, ctx, ts.Store, "msg-cat-1", 500, "INR", "Netflix", "")
	id2 := seedTransaction(t, ctx, ts.Store, "msg-cat-2", 500, "INR", "Netflix", "Shopping")
	id3 := seedTransaction(t, ctx, ts.Store, "msg-cat-3", 500, "INR", "Netflix", "")
	_ = seedTransaction(t, ctx, ts.Store, "msg-cat-4", 200, "INR", "Spotify", "")

	n, err := ts.CategorizeMerchant(ctx, "Netflix", "Entertainment", "Wants")
	if err != nil {
		t.Fatalf("CategorizeMerchant: %v", err)
	}
	if n != 3 {
		t.Errorf("want 3 rows updated, got %d", n)
	}

	// Verify all Netflix transactions updated.
	for _, id := range []string{id1, id2, id3} {
		tx, err := ts.GetTransaction(ctx, id)
		if err != nil {
			t.Fatalf("GetTransaction %s: %v", id, err)
		}
		if tx.Category != "Entertainment" {
			t.Errorf("id=%s: want category=Entertainment, got %q", id, tx.Category)
		}
		if tx.Bucket != "Wants" {
			t.Errorf("id=%s: want bucket=Wants, got %q", id, tx.Bucket)
		}
	}

	// Verify Spotify was not touched.
	all, _, err := ts.ListTransactions(ctx, store.ListFilter{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	for _, tx := range all {
		if tx.MerchantInfo == "Spotify" && tx.Category == "Entertainment" {
			t.Error("Spotify transaction was incorrectly categorized as Entertainment")
		}
	}
}

func TestCategorizeMerchant_UpsertsMerchantCategories(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	// No existing transactions — upsert still persists the rule.
	n, err := ts.CategorizeMerchant(ctx, "Hulu", "Entertainment", "Wants")
	if err != nil {
		t.Fatalf("CategorizeMerchant: %v", err)
	}
	if n != 0 {
		t.Errorf("want 0 rows updated (no existing transactions), got %d", n)
	}

	// Verify merchant_categories rule was stored via LoadCategorySnapshot.
	resolver, err := ts.LoadCategorySnapshot(ctx)
	if err != nil {
		t.Fatalf("LoadCategorySnapshot: %v", err)
	}
	cat, bucket := resolver("Hulu")
	if cat == "" {
		t.Fatal("Hulu not found in category snapshot after CategorizeMerchant")
	}
	if cat != "Entertainment" || bucket != "Wants" {
		t.Errorf("want Entertainment/Wants, got %q/%q", cat, bucket)
	}
}

func TestLoadCategorySnapshot_AllowsCategoryOnlyMerchantMappings(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	if _, err := ts.ApplyCategoryByMerchant(ctx, "Food", "Swiggy"); err != nil {
		t.Fatalf("ApplyCategoryByMerchant: %v", err)
	}

	resolver, err := ts.LoadCategorySnapshot(ctx)
	if err != nil {
		t.Fatalf("LoadCategorySnapshot: %v", err)
	}
	cat, bucket := resolver("Swiggy")
	if cat != "Food" || bucket != "" {
		t.Errorf("want Food with no bucket, got %q/%q", cat, bucket)
	}
}

func TestGetMonthlyBreakdownSpend_ByDimension(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	if err := ts.SetAppConfig(ctx, "app.timezone", "Asia/Kolkata"); err != nil {
		t.Fatalf("SetAppConfig: %v", err)
	}

	restoreNow := ts.SetNowForTestSequence(time.Date(2026, time.April, 12, 12, 0, 0, 0, time.UTC))
	defer restoreNow()

	janID, err := ts.InsertForTest(ctx, store.InsertParams{
		MessageID:    "series-jan",
		Amount:       100,
		Currency:     "INR",
		MerchantInfo: "Uber",
		Category:     "Transport",
		Bucket:       "Needs",
		Timestamp:    time.Date(2026, time.January, 5, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("InsertForTest jan: %v", err)
	}
	if err := ts.AddLabel(ctx, janID, "Travel"); err != nil {
		t.Fatalf("AddLabel jan: %v", err)
	}

	if _, err := ts.InsertForTest(ctx, store.InsertParams{
		MessageID:    "series-feb",
		Amount:       250,
		Currency:     "INR",
		MerchantInfo: "BigBasket",
		Category:     "Groceries",
		Bucket:       "Needs",
		Timestamp:    time.Date(2026, time.February, 18, 9, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("InsertForTest feb: %v", err)
	}

	marID, err := ts.InsertForTest(ctx, store.InsertParams{
		MessageID:    "series-mar",
		Amount:       300,
		Currency:     "INR",
		MerchantInfo: "Swiggy",
		Category:     "Food",
		Bucket:       "Wants",
		Timestamp:    time.Date(2026, time.March, 20, 14, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("InsertForTest mar: %v", err)
	}
	if err := ts.AddLabel(ctx, marID, "Dining"); err != nil {
		t.Fatalf("AddLabel mar: %v", err)
	}

	if _, err := ts.InsertForTest(ctx, store.InsertParams{
		MessageID:    "series-apr-uncategorized",
		Amount:       400,
		Currency:     "INR",
		MerchantInfo: "Unknown Merchant",
		Timestamp:    time.Date(2026, time.April, 2, 8, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("InsertForTest uncategorized apr: %v", err)
	}

	categoryData, err := ts.GetMonthlyBreakdownSpend(ctx, "categories", 4)
	if err != nil {
		t.Fatalf("GetMonthlyBreakdownSpend categories: %v", err)
	}
	if got, want := categoryData.Months, []string{"2026-01", "2026-02", "2026-03", "2026-04"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("category months: got %v want %v", got, want)
	}
	if len(categoryData.Series) != 4 {
		t.Fatalf("expected 4 category series, got %d", len(categoryData.Series))
	}
	if data := monthlySeriesData(categoryData, "Uncategorized"); fmt.Sprint(data) != fmt.Sprint([]float64{0, 0, 0, 400}) {
		t.Fatalf("Uncategorized category series: got %v", data)
	}

	bucketData, err := ts.GetMonthlyBreakdownSpend(ctx, "buckets", 4)
	if err != nil {
		t.Fatalf("GetMonthlyBreakdownSpend buckets: %v", err)
	}
	if len(bucketData.Series) != 3 {
		t.Fatalf("expected 3 bucket series, got %d", len(bucketData.Series))
	}
	for _, series := range bucketData.Series {
		switch series.Label {
		case "Needs":
			if fmt.Sprint(series.Data) != fmt.Sprint([]float64{100, 250, 0, 0}) {
				t.Fatalf("Needs series: got %v", series.Data)
			}
		case "Wants":
			if fmt.Sprint(series.Data) != fmt.Sprint([]float64{0, 0, 300, 0}) {
				t.Fatalf("Wants series: got %v", series.Data)
			}
		}
	}
	if data := monthlySeriesData(bucketData, "Uncategorized"); fmt.Sprint(data) != fmt.Sprint([]float64{0, 0, 0, 400}) {
		t.Fatalf("Uncategorized bucket series: got %v", data)
	}

	labelData, err := ts.GetMonthlyBreakdownSpend(ctx, "labels", 4)
	if err != nil {
		t.Fatalf("GetMonthlyBreakdownSpend labels: %v", err)
	}
	if data := monthlySeriesData(labelData, "Uncategorized"); fmt.Sprint(data) != fmt.Sprint([]float64{0, 250, 0, 400}) {
		t.Fatalf("Uncategorized label series: got %v", data)
	}
}

func monthlySeriesData(data *store.MonthlyBreakdownData, label string) []float64 {
	for _, series := range data.Series {
		if series.Label == label {
			return series.Data
		}
	}
	return nil
}
