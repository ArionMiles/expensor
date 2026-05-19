// Package daemon provides the core daemon runner for Expensor.
package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"golang.org/x/sync/errgroup"

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
	WriterName     string
	Config         *config.Config
	Rules          []api.Rule
	Resolver       api.CategoryResolver
	StateManager   *state.Manager
	DiagnosticSink api.DiagnosticSink
	RuntimeStore   ReaderRuntimeStore
	// ForceRescan bypasses state deduplication for the current run.
	// When true, StateManager should be nil — readers handle nil gracefully.
	ForceRescan bool
}

// ReaderRuntimeStore loads reader runtime configuration persisted by the API.
type ReaderRuntimeStore interface {
	GetReaderConfig(ctx context.Context, reader string) (json.RawMessage, bool, error)
}

// Run starts the expense tracking daemon with the given configuration.
// It blocks until the context is canceled or an error occurs.
func (r *Runner) Run(ctx context.Context, runCfg RunConfig) error {
	cfg := runCfg.Config

	r.logger.Info("starting expensor daemon",
		"reader", runCfg.ReaderName,
		"writer", runCfg.WriterName,
	)

	// Overlay persisted web-UI config onto cfg for readers that use ConfigApplier.
	applyPersistedReaderConfig(ctx, persistedReaderConfigInput{
		registry:   r.registry,
		readerName: runCfg.ReaderName,
		config:     runCfg.Config,
		store:      runCfg.RuntimeStore,
		logger:     r.logger,
	})

	// Create reader from plugin
	reader, err := r.registry.CreateReader(
		runCfg.ReaderName,
		r.httpClient,
		cfg,
		runCfg.Rules,
		runCfg.Resolver,
		runCfg.StateManager,
		runCfg.DiagnosticSink,
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

	r.logger.Info("daemon started")

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return writer.Write(gctx, transactions, ackChan) })
	g.Go(func() error { return reader.Read(gctx, transactions, ackChan) })

	if err := g.Wait(); err != nil &&
		!errors.Is(err, context.Canceled) &&
		!errors.Is(err, context.DeadlineExceeded) {
		r.logger.Error("daemon error", "error", err)
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

type persistedReaderConfigInput struct {
	registry   *plugins.Registry
	readerName string
	config     *config.Config
	store      ReaderRuntimeStore
	logger     *slog.Logger
}

// applyPersistedReaderConfig loads DB-backed reader config and, if the plugin
// implements ConfigApplier, overlays its values onto cfg. Errors are non-fatal:
// if config is absent the reader falls back to env vars.
func applyPersistedReaderConfig(ctx context.Context, input persistedReaderConfigInput) {
	plugin, err := input.registry.GetReader(input.readerName)
	if err != nil {
		return
	}
	applier, ok := plugin.(plugins.ConfigApplier)
	if !ok {
		return
	}
	if input.store == nil {
		return
	}
	data, ok, err := input.store.GetReaderConfig(ctx, input.readerName)
	if err != nil {
		input.logger.Warn("failed to read persisted reader config", "reader", input.readerName, "error", err)
		return
	}
	if !ok {
		return
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		input.logger.Warn("failed to parse persisted reader config", "reader", input.readerName, "error", err)
		return
	}
	applier.ApplyConfig(input.config, raw)
	input.logger.Debug("applied persisted reader config", "reader", input.readerName)
}
