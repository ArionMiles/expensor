package store_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	pgwriter "github.com/ArionMiles/expensor/backend/pkg/writer/postgres"
)

// testStore holds a live *store.Store and the container DSN for teardown.
type testStore struct {
	*store.Store
	cleanup func()
}

// newTestStore spins up a Postgres container, runs the schema migration, and
// returns a connected Store. The returned cleanup function terminates the container.
func newTestStore(t *testing.T) *testStore {
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
	cfg.Port = mappedPort.Int()

	st, err := store.New(cfg, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))
	if err != nil {
		_ = ctr.Terminate(ctx)
		t.Fatalf("failed to create store: %v", err)
	}

	return &testStore{
		Store: st,
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

// --- tests ---

func TestListTransactions_Pagination(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	for i := range 5 {
		seedTransaction(t, ctx, ts.Store, fmt.Sprintf("msg-%d", i), float64(100*(i+1)), "INR", fmt.Sprintf("Merchant%d", i), "Food")
	}

	// Page 1, size 2.
	txns, total, err := ts.ListTransactions(ctx, store.ListFilter{Page: 1, PageSize: 2})
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if total != 5 {
		t.Errorf("want total=5, got %d", total)
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

	txns, total, err := ts.ListTransactions(ctx, store.ListFilter{Category: "Food", PageSize: 10})
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if total != 1 {
		t.Errorf("want total=1, got %d", total)
	}
	if len(txns) != 1 || txns[0].Category != "Food" {
		t.Errorf("unexpected transactions: %v", txns)
	}
}

func TestListTransactions_FilterByCurrency(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	seedTransaction(t, ctx, ts.Store, "inr-1", 100, "INR", "Amazon IN", "Shopping")
	seedTransaction(t, ctx, ts.Store, "usd-1", 20, "USD", "Amazon US", "Shopping")

	txns, total, err := ts.ListTransactions(ctx, store.ListFilter{Currency: "USD", PageSize: 10})
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if total != 1 || txns[0].Currency != "USD" {
		t.Errorf("want 1 USD transaction, got total=%d txns=%v", total, txns)
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

	txns, total, err := ts.ListTransactions(ctx, store.ListFilter{Label: "subscription", PageSize: 10})
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if total != 1 || txns[0].ID != id {
		t.Errorf("want 1 labeled transaction, got total=%d", total)
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

func TestSearchTransactions(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	seedTransaction(t, ctx, ts.Store, "srch-1", 150, "INR", "Starbucks Coffee", "Food")
	seedTransaction(t, ctx, ts.Store, "srch-2", 200, "INR", "Pizza Hut", "Food")

	txns, total, err := ts.SearchTransactions(ctx, "Starbucks", store.ListFilter{PageSize: 10})
	if err != nil {
		t.Fatalf("SearchTransactions: %v", err)
	}
	if total != 1 {
		t.Errorf("want 1 result for 'Starbucks', got %d", total)
	}
	if len(txns) != 1 || txns[0].MerchantInfo != "Starbucks Coffee" {
		t.Errorf("unexpected result: %v", txns)
	}
}

func TestSearchTransactions_EmptyQuery(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()
	ctx := context.Background()

	seedTransaction(t, ctx, ts.Store, "srch-all-1", 100, "INR", "Any Shop", "Misc")
	seedTransaction(t, ctx, ts.Store, "srch-all-2", 200, "INR", "Another Shop", "Misc")

	// Empty query should return all.
	_, total, err := ts.SearchTransactions(ctx, "", store.ListFilter{PageSize: 10})
	if err != nil {
		t.Fatalf("SearchTransactions empty: %v", err)
	}
	if total != 2 {
		t.Errorf("empty query should return all 2 transactions, got %d", total)
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
