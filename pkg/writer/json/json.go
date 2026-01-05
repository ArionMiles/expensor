// Package json implements a Writer that writes transactions to a JSON file.
package json

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/ArionMiles/expensor/pkg/api"
	"github.com/ArionMiles/expensor/pkg/writer/buffered"
)

// Writer writes transactions to a JSON file with buffered batching.
type Writer struct {
	filePath     string
	transactions []*api.TransactionDetails
	mu           sync.Mutex
	buffered     *buffered.Writer
	logger       *slog.Logger
}

// Config holds configuration for the JSON writer.
type Config struct {
	// FilePath is the path to the JSON output file.
	FilePath string
	// BatchSize is the number of transactions to buffer before writing.
	BatchSize int
	// FlushInterval is the interval between automatic flushes (seconds).
	FlushInterval int
}

// New creates a new JSON writer.
func New(cfg Config, logger *slog.Logger) (*Writer, error) {
	if logger == nil {
		logger = slog.Default()
	}

	w := &Writer{
		filePath:     cfg.FilePath,
		transactions: make([]*api.TransactionDetails, 0),
		logger:       logger,
	}

	// Load existing transactions if file exists
	if err := w.loadExisting(); err != nil {
		logger.Warn("could not load existing transactions", "error", err)
	}

	// Create buffered writer
	bufCfg := buffered.Config{
		BatchSize: cfg.BatchSize,
	}
	if cfg.FlushInterval > 0 {
		bufCfg.FlushInterval = buffered.DefaultFlushInterval
	}

	w.buffered = buffered.New(w.flushBatch, bufCfg, logger.With("component", "json_buffer"))

	logger.Info("json writer initialized", "file", cfg.FilePath, "existing_count", len(w.transactions))
	return w, nil
}

// loadExisting loads existing transactions from the JSON file if it exists.
func (w *Writer) loadExisting() error {
	data, err := os.ReadFile(w.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if len(data) == 0 {
		return nil
	}

	return json.Unmarshal(data, &w.transactions)
}

// Write consumes transactions from the input channel and writes them to JSON.
func (w *Writer) Write(ctx context.Context, in <-chan *api.TransactionDetails, ackChan chan<- string) error {
	return w.buffered.Write(ctx, in, ackChan)
}

// flushBatch appends a batch of transactions and writes to the JSON file.
func (w *Writer) flushBatch(transactions []*api.TransactionDetails) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Append new transactions
	w.transactions = append(w.transactions, transactions...)

	// Write entire array to file (JSON doesn't support appending)
	data, err := json.MarshalIndent(w.transactions, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling json: %w", err)
	}

	if err := os.WriteFile(w.filePath, data, 0o600); err != nil {
		return fmt.Errorf("writing json file: %w", err)
	}

	w.logger.Debug("wrote transactions to json",
		"batch_count", len(transactions),
		"total_count", len(w.transactions),
	)
	return nil
}

// TransactionCount returns the total number of transactions written.
func (w *Writer) TransactionCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.transactions)
}
