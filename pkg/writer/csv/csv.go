// Package csv implements a Writer that writes transactions to a CSV file.
package csv

import (
	"context"
	"encoding/csv"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"sync"

	"github.com/ArionMiles/expensor/pkg/api"
	"github.com/ArionMiles/expensor/pkg/writer/buffered"
)

// Writer writes transactions to a CSV file with buffered batching.
type Writer struct {
	filePath string
	file     *os.File
	writer   *csv.Writer
	mu       sync.Mutex
	buffered *buffered.Writer
	logger   *slog.Logger
}

// Config holds configuration for the CSV writer.
type Config struct {
	// FilePath is the path to the CSV output file.
	FilePath string
	// BatchSize is the number of transactions to buffer before writing.
	BatchSize int
	// FlushInterval is the interval between automatic flushes.
	FlushInterval int // seconds
}

// New creates a new CSV writer.
func New(cfg Config, logger *slog.Logger) (*Writer, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Create or open file
	file, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("opening csv file: %w", err)
	}

	w := &Writer{
		filePath: cfg.FilePath,
		file:     file,
		writer:   csv.NewWriter(file),
		logger:   logger,
	}

	// Write headers if file is new/empty
	stat, err := file.Stat()
	if err != nil {
		if closeErr := file.Close(); closeErr != nil {
			return nil, fmt.Errorf("stat csv file: %w (close error: %w)", err, closeErr)
		}
		return nil, fmt.Errorf("stat csv file: %w", err)
	}

	if stat.Size() == 0 {
		if err := w.writeHeaders(); err != nil {
			if closeErr := file.Close(); closeErr != nil {
				return nil, fmt.Errorf("writing headers: %w (close error: %w)", err, closeErr)
			}
			return nil, fmt.Errorf("writing headers: %w", err)
		}
	}

	// Create buffered writer
	bufCfg := buffered.Config{
		BatchSize: cfg.BatchSize,
	}
	if cfg.FlushInterval > 0 {
		bufCfg.FlushInterval = buffered.DefaultFlushInterval
	}

	w.buffered = buffered.New(w.flushBatch, bufCfg, logger.With("component", "csv_buffer"))

	logger.Info("csv writer initialized", "file", cfg.FilePath)
	return w, nil
}

func (w *Writer) writeHeaders() error {
	headers := []string{"Timestamp", "Merchant", "Amount", "Category", "Bucket", "Source"}
	if err := w.writer.Write(headers); err != nil {
		return err
	}
	w.writer.Flush()
	return w.writer.Error()
}

// Write consumes transactions from the input channel and writes them to CSV.
func (w *Writer) Write(ctx context.Context, in <-chan *api.TransactionDetails, ackChan chan<- string) error {
	defer w.Close()
	return w.buffered.Write(ctx, in, ackChan)
}

// flushBatch writes a batch of transactions to the CSV file.
func (w *Writer) flushBatch(transactions []*api.TransactionDetails) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, t := range transactions {
		record := []string{
			t.Timestamp,
			t.MerchantInfo,
			strconv.FormatFloat(t.Amount, 'f', 2, 64),
			t.Category,
			t.Bucket,
			t.Source,
		}
		if err := w.writer.Write(record); err != nil {
			return fmt.Errorf("writing csv record: %w", err)
		}
	}

	w.writer.Flush()
	if err := w.writer.Error(); err != nil {
		return fmt.Errorf("flushing csv: %w", err)
	}

	w.logger.Debug("wrote transactions to csv", "count", len(transactions))
	return nil
}

// Close closes the CSV file.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.writer.Flush()
	if err := w.file.Close(); err != nil {
		return fmt.Errorf("closing csv file: %w", err)
	}

	w.logger.Info("csv writer closed", "file", w.filePath)
	return nil
}
