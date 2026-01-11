// Package daemon provides the core daemon runner for Expensor.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
)

// Runner manages the expense tracking daemon lifecycle.
type Runner struct {
	registry   *plugins.Registry
	httpClient *http.Client
	logger     *slog.Logger
}

// New creates a new daemon runner.
func New(registry *plugins.Registry, httpClient *http.Client, logger *slog.Logger) *Runner {
	if logger == nil {
		logger = slog.Default()
	}

	return &Runner{
		registry:   registry,
		httpClient: httpClient,
		logger:     logger,
	}
}

// Run starts the expense tracking daemon with the given configuration.
// It blocks until the context is canceled or an error occurs.
func (r *Runner) Run(ctx context.Context, cfg config.Config) error {
	// Validate configuration
	if cfg.ReaderPlugin == "" {
		return fmt.Errorf("EXPENSOR_READER environment variable is required")
	}
	if cfg.WriterPlugin == "" {
		return fmt.Errorf("EXPENSOR_WRITER environment variable is required")
	}

	r.logger.Info("starting expensor daemon",
		"reader", cfg.ReaderPlugin,
		"writer", cfg.WriterPlugin,
	)

	// Create reader from plugin
	reader, err := r.registry.CreateReader(
		cfg.ReaderPlugin,
		r.httpClient,
		cfg.ReaderConfig,
		r.logger.With("component", "reader", "plugin", cfg.ReaderPlugin),
	)
	if err != nil {
		return fmt.Errorf("creating reader: %w", err)
	}

	// Create writer from plugin
	writer, err := r.registry.CreateWriter(
		cfg.WriterPlugin,
		r.httpClient,
		cfg.WriterConfig,
		r.logger.With("component", "writer", "plugin", cfg.WriterPlugin),
	)
	if err != nil {
		return fmt.Errorf("creating writer: %w", err)
	}

	// Create transaction and acknowledgment channels
	transactions := make(chan *api.TransactionDetails, 100)
	ackChan := make(chan string, 100)

	// Start writer in background
	writerDone := make(chan error, 1)
	go func() {
		writerDone <- writer.Write(ctx, transactions, ackChan)
	}()

	// Start reader (blocks until context is canceled)
	r.logger.Info("daemon started")
	if err := reader.Read(ctx, transactions, ackChan); err != nil && !errors.Is(err, context.Canceled) {
		r.logger.Error("reader error", "error", err)
	}

	// Wait for writer to finish
	if err := <-writerDone; err != nil && !errors.Is(err, context.Canceled) {
		r.logger.Error("writer error", "error", err)
	}

	r.logger.Info("daemon stopped")
	return nil
}
