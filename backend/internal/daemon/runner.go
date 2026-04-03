// Package daemon provides the core daemon runner for Expensor.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	"github.com/ArionMiles/expensor/backend/pkg/state"
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

// RunConfig holds the configuration for running the daemon.
type RunConfig struct {
	// ReaderName is the plugin name of the reader to use (e.g. "gmail").
	// Set by the web UI via POST /api/daemon/start.
	ReaderName string
	// WriterName is the plugin name of the writer to use (e.g. "postgres").
	WriterName   string
	Config       *config.Config
	Rules        []api.Rule
	Labels       api.Labels
	StateManager *state.Manager
}

// Run starts the expense tracking daemon with the given configuration.
// It blocks until the context is canceled or an error occurs.
func (r *Runner) Run(ctx context.Context, runCfg RunConfig) error {
	cfg := runCfg.Config

	r.logger.Info("starting expensor daemon",
		"reader", runCfg.ReaderName,
		"writer", runCfg.WriterName,
	)

	// Create reader from plugin
	reader, err := r.registry.CreateReader(
		runCfg.ReaderName,
		r.httpClient,
		cfg,
		runCfg.Rules,
		runCfg.Labels,
		runCfg.StateManager,
		r.logger.With("component", "reader", "plugin", runCfg.ReaderName),
	)
	if err != nil {
		return fmt.Errorf("creating reader: %w", err)
	}

	// Create writer from plugin
	writer, err := r.registry.CreateWriter(
		runCfg.WriterName,
		r.httpClient,
		cfg,
		r.logger.With("component", "writer", "plugin", runCfg.WriterName),
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
	readerDone := make(chan error, 1)
	go func() {
		readerDone <- reader.Read(ctx, transactions, ackChan)
	}()

	// Wait for both reader and writer to complete
	var readerErr, writerErr error
	for i := 0; i < 2; i++ {
		select {
		case readerErr = <-readerDone:
			if readerErr != nil && !errors.Is(readerErr, context.Canceled) && !errors.Is(readerErr, context.DeadlineExceeded) {
				r.logger.Error("reader error", "error", readerErr)
			}
		case writerErr = <-writerDone:
			if writerErr != nil && !errors.Is(writerErr, context.Canceled) && !errors.Is(writerErr, context.DeadlineExceeded) {
				r.logger.Error("writer error", "error", writerErr)
			}
		}
	}

	// Close writer if it implements io.Closer.
	if closer, ok := writer.(io.Closer); ok {
		if err := closer.Close(); err != nil {
			r.logger.Warn("error closing writer", "error", err)
		} else {
			r.logger.Info("closed writer resources")
		}
	}

	r.logger.Info("daemon stopped")
	return nil
}
