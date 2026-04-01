package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/pkg/api"
)

// TestNewWriter_ConnectionFailure tests that the writer returns an error when connection fails.
func TestNewWriter_ConnectionFailure(t *testing.T) {
	cfg := Config{
		Host:     "nonexistent-host",
		Port:     5432,
		Database: "expensor",
		User:     "expensor",
		Password: "password",
		SSLMode:  "disable",
	}

	_, err := New(cfg, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	if err == nil {
		t.Error("expected error when connecting to nonexistent host, got nil")
	}
}

// TestNewWriter_Defaults tests that default values are set correctly.
func TestNewWriter_Defaults(t *testing.T) {
	// Skip if no test database available
	if os.Getenv("TEST_POSTGRES_HOST") == "" {
		t.Skip("TEST_POSTGRES_HOST not set, skipping integration test")
	}

	cfg := Config{
		Host:     os.Getenv("TEST_POSTGRES_HOST"),
		Database: os.Getenv("TEST_POSTGRES_DB"),
		User:     os.Getenv("TEST_POSTGRES_USER"),
		Password: os.Getenv("TEST_POSTGRES_PASSWORD"),
	}

	writer, err := New(cfg, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Check defaults
	if writer.batchSize != 10 {
		t.Errorf("expected default batchSize=10, got %d", writer.batchSize)
	}
	if writer.flushInterval != 30*time.Second {
		t.Errorf("expected default flushInterval=30s, got %v", writer.flushInterval)
	}
}

// TestWrite_SingleTransaction tests writing a single transaction.
func TestWrite_SingleTransaction(t *testing.T) {
	// Skip if no test database available
	if os.Getenv("TEST_POSTGRES_HOST") == "" {
		t.Skip("TEST_POSTGRES_HOST not set, skipping integration test")
	}

	cfg := Config{
		Host:          os.Getenv("TEST_POSTGRES_HOST"),
		Database:      os.Getenv("TEST_POSTGRES_DB"),
		User:          os.Getenv("TEST_POSTGRES_USER"),
		Password:      os.Getenv("TEST_POSTGRES_PASSWORD"),
		BatchSize:     1, // Force immediate write
		FlushInterval: 1 * time.Second,
	}

	writer, err := New(cfg, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Create test transaction
	txn := &api.TransactionDetails{
		MessageID:    fmt.Sprintf("test-msg-%d", time.Now().Unix()),
		Amount:       1234.56,
		Currency:     "INR",
		Timestamp:    time.Now().Format(time.RFC3339),
		MerchantInfo: "Test Merchant",
		Category:     "Test Category",
		Bucket:       "Wants",
		Source:       "Test Source",
		Description:  "Test transaction",
	}

	// Create channels
	in := make(chan *api.TransactionDetails, 1)
	ackChan := make(chan string, 1)

	// Start writer
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- writer.Write(ctx, in, ackChan)
	}()

	// Send transaction
	in <- txn
	close(in)

	// Wait for acknowledgment
	select {
	case msgID := <-ackChan:
		if msgID != txn.MessageID {
			t.Errorf("expected ack for %s, got %s", txn.MessageID, msgID)
		}
	case <-time.After(3 * time.Second):
		t.Error("timeout waiting for acknowledgment")
	}

	// Wait for writer to finish
	if err := <-errChan; err != nil {
		t.Errorf("writer returned error: %v", err)
	}
}

// TestWrite_MultiCurrency tests writing a transaction with currency conversion.
func TestWrite_MultiCurrency(t *testing.T) {
	// Skip if no test database available
	if os.Getenv("TEST_POSTGRES_HOST") == "" {
		t.Skip("TEST_POSTGRES_HOST not set, skipping integration test")
	}

	cfg := Config{
		Host:          os.Getenv("TEST_POSTGRES_HOST"),
		Database:      os.Getenv("TEST_POSTGRES_DB"),
		User:          os.Getenv("TEST_POSTGRES_USER"),
		Password:      os.Getenv("TEST_POSTGRES_PASSWORD"),
		BatchSize:     1,
		FlushInterval: 1 * time.Second,
	}

	writer, err := New(cfg, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Create test transaction with currency conversion
	originalAmount := 100.00
	originalCurrency := "USD"
	exchangeRate := 83.50

	txn := &api.TransactionDetails{
		MessageID:        fmt.Sprintf("test-usd-%d", time.Now().Unix()),
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

	// Create channels
	in := make(chan *api.TransactionDetails, 1)
	ackChan := make(chan string, 1)

	// Start writer
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- writer.Write(ctx, in, ackChan)
	}()

	// Send transaction
	in <- txn
	close(in)

	// Wait for acknowledgment
	select {
	case msgID := <-ackChan:
		if msgID != txn.MessageID {
			t.Errorf("expected ack for %s, got %s", txn.MessageID, msgID)
		}
	case <-time.After(3 * time.Second):
		t.Error("timeout waiting for acknowledgment")
	}

	// Wait for writer to finish
	if err := <-errChan; err != nil {
		t.Errorf("writer returned error: %v", err)
	}
}

// TestWrite_WithLabels tests writing a transaction with labels.
func TestWrite_WithLabels(t *testing.T) {
	// Skip if no test database available
	if os.Getenv("TEST_POSTGRES_HOST") == "" {
		t.Skip("TEST_POSTGRES_HOST not set, skipping integration test")
	}

	cfg := Config{
		Host:          os.Getenv("TEST_POSTGRES_HOST"),
		Database:      os.Getenv("TEST_POSTGRES_DB"),
		User:          os.Getenv("TEST_POSTGRES_USER"),
		Password:      os.Getenv("TEST_POSTGRES_PASSWORD"),
		BatchSize:     1,
		FlushInterval: 1 * time.Second,
	}

	writer, err := New(cfg, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Create test transaction with labels
	txn := &api.TransactionDetails{
		MessageID:    fmt.Sprintf("test-labels-%d", time.Now().Unix()),
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

	// Create channels
	in := make(chan *api.TransactionDetails, 1)
	ackChan := make(chan string, 1)

	// Start writer
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- writer.Write(ctx, in, ackChan)
	}()

	// Send transaction
	in <- txn
	close(in)

	// Wait for acknowledgment
	select {
	case msgID := <-ackChan:
		if msgID != txn.MessageID {
			t.Errorf("expected ack for %s, got %s", txn.MessageID, msgID)
		}
	case <-time.After(3 * time.Second):
		t.Error("timeout waiting for acknowledgment")
	}

	// Wait for writer to finish
	if err := <-errChan; err != nil {
		t.Errorf("writer returned error: %v", err)
	}
}

// TestWrite_Batch tests writing multiple transactions in a batch.
func TestWrite_Batch(t *testing.T) {
	// Skip if no test database available
	if os.Getenv("TEST_POSTGRES_HOST") == "" {
		t.Skip("TEST_POSTGRES_HOST not set, skipping integration test")
	}

	cfg := Config{
		Host:          os.Getenv("TEST_POSTGRES_HOST"),
		Database:      os.Getenv("TEST_POSTGRES_DB"),
		User:          os.Getenv("TEST_POSTGRES_USER"),
		Password:      os.Getenv("TEST_POSTGRES_PASSWORD"),
		BatchSize:     5,
		FlushInterval: 1 * time.Second,
	}

	writer, err := New(cfg, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Create channels
	in := make(chan *api.TransactionDetails, 10)
	ackChan := make(chan string, 10)

	// Start writer
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- writer.Write(ctx, in, ackChan)
	}()

	// Send multiple transactions
	numTxns := 10
	for i := 0; i < numTxns; i++ {
		txn := &api.TransactionDetails{
			MessageID:    fmt.Sprintf("test-batch-%d-%d", time.Now().Unix(), i),
			Amount:       float64(100 * (i + 1)),
			Currency:     "INR",
			Timestamp:    time.Now().Format(time.RFC3339),
			MerchantInfo: fmt.Sprintf("Merchant %d", i),
			Category:     "Test",
			Bucket:       "Wants",
			Source:       "Test Source",
		}
		in <- txn
	}
	close(in)

	// Wait for all acknowledgments
	ackCount := 0
	timeout := time.After(5 * time.Second)
	for ackCount < numTxns {
		select {
		case <-ackChan:
			ackCount++
		case <-timeout:
			t.Errorf("timeout waiting for acknowledgments, got %d/%d", ackCount, numTxns)
			return
		}
	}

	// Wait for writer to finish
	if err := <-errChan; err != nil {
		t.Errorf("writer returned error: %v", err)
	}
}
