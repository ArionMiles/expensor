package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/ArionMiles/expensor/backend/pkg/api"
)

// testDB holds the shared container config for all integration tests.
// It is populated by TestMain before any test runs.
var testDB *Config

// TestMain starts a single Postgres container for all integration tests in this
// package, avoiding the overhead of spinning up a new container per test.
func TestMain(m *testing.M) {
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
		fmt.Fprintf(os.Stderr, "failed to start postgres container: %v\n", err)
		os.Exit(1)
	}

	mappedPort, err := ctr.MappedPort(ctx, "5432")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get mapped port: %v\n", err)
		_ = ctr.Terminate(ctx)
		os.Exit(1)
	}

	testDB = &Config{
		Host:     "localhost",
		Port:     mappedPort.Int(),
		Database: "expensor_test",
		User:     "expensor",
		Password: "expensor",
		SSLMode:  "disable",
	}

	code := m.Run()

	_ = ctr.Terminate(ctx)
	os.Exit(code)
}

// newTestWriter creates a writer using the shared test container.
// Each call creates a fresh Writer (and re-runs idempotent migrations).
func newTestWriter(t *testing.T, overrides Config) *Writer {
	t.Helper()
	if testing.Short() || testDB == nil {
		t.Skip("skipping integration test (-short or container unavailable)")
	}

	cfg := *testDB
	if overrides.BatchSize > 0 {
		cfg.BatchSize = overrides.BatchSize
	}
	if overrides.FlushInterval > 0 {
		cfg.FlushInterval = overrides.FlushInterval
	}

	w, err := New(cfg, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })
	return w
}

// TestNewWriter_ConnectionFailure verifies the writer returns an error for an unreachable host.
func TestNewWriter_ConnectionFailure(t *testing.T) {
	cfg := Config{
		Host:     "nonexistent-host",
		Port:     5432,
		Database: "expensor",
		User:     "expensor",
		Password: "password",
		SSLMode:  "disable",
	}
	_, err := New(cfg, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err == nil {
		t.Error("expected error when connecting to nonexistent host, got nil")
	}
}

// TestNewWriter_Defaults verifies that zero-value config fields are populated with defaults.
func TestNewWriter_Defaults(t *testing.T) {
	w := newTestWriter(t, Config{})

	if w.batchSize != 10 {
		t.Errorf("expected default batchSize=10, got %d", w.batchSize)
	}
	if w.flushInterval != 30*time.Second {
		t.Errorf("expected default flushInterval=30s, got %v", w.flushInterval)
	}
}

// TestWrite_SingleTransaction verifies a single transaction is written and acknowledged.
func TestWrite_SingleTransaction(t *testing.T) {
	w := newTestWriter(t, Config{BatchSize: 1, FlushInterval: time.Second})

	txn := &api.TransactionDetails{
		MessageID:    fmt.Sprintf("test-msg-%d", time.Now().UnixNano()),
		Amount:       1234.56,
		Currency:     "INR",
		Timestamp:    time.Now().Format(time.RFC3339),
		MerchantInfo: "Test Merchant",
		Category:     "Test Category",
		Bucket:       "Wants",
		Source:       "Test Source",
		Description:  "Test transaction",
	}

	assertWrite(t, w, []*api.TransactionDetails{txn}, 5*time.Second)
}

// TestWrite_MultiCurrency verifies a transaction with currency conversion fields is stored correctly.
func TestWrite_MultiCurrency(t *testing.T) {
	w := newTestWriter(t, Config{BatchSize: 1, FlushInterval: time.Second})

	originalAmount := 100.00
	originalCurrency := "USD"
	exchangeRate := 83.50

	txn := &api.TransactionDetails{
		MessageID:        fmt.Sprintf("test-usd-%d", time.Now().UnixNano()),
		Amount:           originalAmount * exchangeRate,
		Currency:         "INR",
		OriginalAmount:   &originalAmount,
		OriginalCurrency: &originalCurrency,
		ExchangeRate:     &exchangeRate,
		Timestamp:        time.Now().Format(time.RFC3339),
		MerchantInfo:     "Amazon.com",
		Category:         "Shopping",
		Bucket:           "Wants",
		Source:           "Credit Card - ICICI",
	}

	assertWrite(t, w, []*api.TransactionDetails{txn}, 5*time.Second)
}

// TestWrite_WithLabels verifies that labels are persisted alongside the transaction.
func TestWrite_WithLabels(t *testing.T) {
	w := newTestWriter(t, Config{BatchSize: 1, FlushInterval: time.Second})

	txn := &api.TransactionDetails{
		MessageID:    fmt.Sprintf("test-labels-%d", time.Now().UnixNano()),
		Amount:       500.00,
		Currency:     "INR",
		Timestamp:    time.Now().Format(time.RFC3339),
		MerchantInfo: "Starbucks",
		Category:     "Food",
		Bucket:       "Wants",
		Source:       "Credit Card - ICICI",
		Description:  "Coffee with team",
		Labels:       []string{"work", "coffee", "team-expense"},
	}

	assertWrite(t, w, []*api.TransactionDetails{txn}, 5*time.Second)
}

// TestWrite_Batch verifies that multiple transactions are written and all acknowledged.
func TestWrite_Batch(t *testing.T) {
	w := newTestWriter(t, Config{BatchSize: 5, FlushInterval: time.Second})

	txns := make([]*api.TransactionDetails, 10)
	for i := range txns {
		txns[i] = &api.TransactionDetails{
			MessageID:    fmt.Sprintf("test-batch-%d-%d", time.Now().UnixNano(), i),
			Amount:       float64(100 * (i + 1)),
			Currency:     "INR",
			Timestamp:    time.Now().Format(time.RFC3339),
			MerchantInfo: fmt.Sprintf("Merchant %d", i),
			Category:     "Test",
			Bucket:       "Wants",
			Source:       "Test Source",
		}
	}

	assertWrite(t, w, txns, 10*time.Second)
}

// assertWrite sends txns through the writer and verifies every message is acknowledged.
func assertWrite(t *testing.T, w *Writer, txns []*api.TransactionDetails, timeout time.Duration) {
	t.Helper()

	in := make(chan *api.TransactionDetails, len(txns))
	ackChan := make(chan string, len(txns))

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- w.Write(ctx, in, ackChan) }()

	for _, txn := range txns {
		in <- txn
	}
	close(in)

	expected := make(map[string]bool, len(txns))
	for _, txn := range txns {
		expected[txn.MessageID] = false
	}

	deadline := time.After(timeout - 500*time.Millisecond)
	for range txns {
		select {
		case msgID := <-ackChan:
			if _, ok := expected[msgID]; !ok {
				t.Errorf("received ack for unexpected message ID %q", msgID)
			}
			expected[msgID] = true
		case <-deadline:
			missing := 0
			for _, acked := range expected {
				if !acked {
					missing++
				}
			}
			t.Errorf("timeout: %d/%d transactions not acknowledged", missing, len(txns))
			return
		}
	}

	if err := <-errCh; err != nil {
		t.Errorf("writer returned error: %v", err)
	}
}
