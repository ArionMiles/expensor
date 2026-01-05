// Package buffered provides a buffered writer base for batch writes.
package buffered

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/ArionMiles/expensor/pkg/api"
)

// DefaultBatchSize is the default number of transactions to buffer before flushing.
const DefaultBatchSize = 10

// DefaultFlushInterval is the default interval between automatic flushes.
const DefaultFlushInterval = 30 * time.Second

// Flusher is called when the buffer needs to be flushed.
type Flusher func(transactions []*api.TransactionDetails) error

// Config holds configuration for buffered writing.
type Config struct {
	// BatchSize is the number of transactions to buffer before flushing.
	// Defaults to DefaultBatchSize.
	BatchSize int
	// FlushInterval is the interval between automatic flushes.
	// Defaults to DefaultFlushInterval.
	FlushInterval time.Duration
}

// Writer buffers transactions and flushes them in batches.
type Writer struct {
	buffer   []*api.TransactionDetails
	mu       sync.Mutex
	flusher  Flusher
	config   Config
	logger   *slog.Logger
	doneChan chan struct{}
}

// New creates a new buffered writer with the given flusher function.
func New(flusher Flusher, cfg Config, logger *slog.Logger) *Writer {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = DefaultBatchSize
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = DefaultFlushInterval
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &Writer{
		buffer:   make([]*api.TransactionDetails, 0, cfg.BatchSize),
		flusher:  flusher,
		config:   cfg,
		logger:   logger,
		doneChan: make(chan struct{}),
	}
}

// Write consumes transactions from the input channel and buffers them for batch writes.
func (w *Writer) Write(ctx context.Context, in <-chan *api.TransactionDetails) error {
	ticker := time.NewTicker(w.config.FlushInterval)
	defer ticker.Stop()

	w.logger.Info("buffered writer started",
		"batch_size", w.config.BatchSize,
		"flush_interval", w.config.FlushInterval,
	)

	for {
		select {
		case <-ctx.Done():
			return w.handleShutdown()
		case <-ticker.C:
			w.handleTimerFlush()
		case transaction, ok := <-in:
			if done, err := w.handleTransaction(transaction, ok); done {
				return err
			}
		}
	}
}

func (w *Writer) handleShutdown() error {
	w.logger.Info("buffered writer stopping, flushing remaining buffer")
	if err := w.flush(); err != nil {
		w.logger.Error("failed to flush on shutdown", "error", err)
	}
	return context.Canceled
}

func (w *Writer) handleTimerFlush() {
	if err := w.flush(); err != nil {
		w.logger.Error("failed to flush on interval", "error", err)
	}
}

func (w *Writer) handleTransaction(transaction *api.TransactionDetails, ok bool) (bool, error) {
	if !ok {
		w.logger.Info("input channel closed, flushing remaining buffer")
		if err := w.flush(); err != nil {
			w.logger.Error("failed to flush on close", "error", err)
			return true, err
		}
		return true, nil
	}

	w.mu.Lock()
	w.buffer = append(w.buffer, transaction)
	shouldFlush := len(w.buffer) >= w.config.BatchSize
	w.mu.Unlock()

	if shouldFlush {
		if err := w.flush(); err != nil {
			w.logger.Error("failed to flush on batch size", "error", err)
		}
	}
	return false, nil
}

// flush writes all buffered transactions using the flusher function.
func (w *Writer) flush() error {
	w.mu.Lock()
	if len(w.buffer) == 0 {
		w.mu.Unlock()
		return nil
	}

	// Copy buffer and reset
	toFlush := make([]*api.TransactionDetails, len(w.buffer))
	copy(toFlush, w.buffer)
	w.buffer = w.buffer[:0]
	w.mu.Unlock()

	w.logger.Debug("flushing buffer", "count", len(toFlush))

	if err := w.flusher(toFlush); err != nil {
		return err
	}

	w.logger.Info("flushed transactions", "count", len(toFlush))
	return nil
}

// BufferLen returns the current number of buffered transactions.
func (w *Writer) BufferLen() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.buffer)
}
