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

	"github.com/ArionMiles/expensor/backend/internal/observability"
	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/internal/state"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
)

// Runner manages the expense tracking daemon lifecycle.
type Runner struct {
	registry   *plugins.Registry
	httpClient *http.Client
	logger     *slog.Logger
	scope      *observability.Scope
}

// New creates a new daemon runner.
func New(registry *plugins.Registry, httpClient *http.Client, logger *slog.Logger) *Runner {
	return NewWithScope(registry, httpClient, logger, nil)
}

// NewWithScope creates a daemon runner with an explicit observability scope.
func NewWithScope(registry *plugins.Registry, httpClient *http.Client, logger *slog.Logger, scope *observability.Scope) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	if scope == nil {
		scope = observability.NewScope(logger, "github.com/ArionMiles/expensor/backend/internal/daemon")
	}

	return &Runner{
		registry:   registry,
		httpClient: httpClient,
		logger:     logger,
		scope:      scope,
	}
}

// RunConfig holds the configuration for running the daemon.
type RunConfig struct {
	// ReaderName is the plugin name of the reader to use (e.g. "gmail").
	// Set by the web UI via POST /api/daemon/start.
	ReaderName string
	// WriterName is the plugin name of the writer to use (e.g. "postgres").
	WriterName     string
	Config         *config.App
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
	ctx, span := r.scope.Start(ctx, "daemon.run")
	defer span.End()

	cfg := runCfg.Config
	var runErr error
	defer func() {
		r.scope.RecordOperation(ctx, observability.Operation{
			Namespace: "daemon",
			Name:      "run",
			Err:       runErr,
		})
	}()

	r.logger.Info("starting expensor daemon",
		"reader", runCfg.ReaderName,
		"writer", runCfg.WriterName,
	)

	readerPlugin, err := r.registry.GetReader(runCfg.ReaderName)
	if err != nil {
		runErr = err
		return fmt.Errorf("creating reader: %w", err)
	}
	reader, err := readerPlugin.NewReader(plugins.ReaderInput{
		HTTPClient:     r.httpClient,
		AppConfig:      cfg,
		ReaderConfig:   r.loadReaderConfig(ctx, runCfg.ReaderName, runCfg.RuntimeStore),
		Rules:          runCfg.Rules,
		Resolver:       runCfg.Resolver,
		StateManager:   runCfg.StateManager,
		DiagnosticSink: runCfg.DiagnosticSink,
		Logger:         r.logger.With("component", "reader", "plugin", runCfg.ReaderName),
	})
	if err != nil {
		runErr = err
		return fmt.Errorf("creating reader: %w", err)
	}

	writerPlugin, err := r.registry.GetWriter(runCfg.WriterName)
	if err != nil {
		runErr = err
		return fmt.Errorf("creating writer: %w", err)
	}
	writer, err := writerPlugin.NewWriter(plugins.WriterInput{
		HTTPClient: r.httpClient,
		AppConfig:  cfg,
		Logger:     r.logger.With("component", "writer", "plugin", runCfg.WriterName),
	})
	if err != nil {
		runErr = err
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
		runErr = err
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
	return runErr
}

func (r *Runner) loadReaderConfig(ctx context.Context, readerName string, store ReaderRuntimeStore) json.RawMessage {
	if store == nil {
		return nil
	}
	data, ok, err := store.GetReaderConfig(ctx, readerName)
	if err != nil {
		r.logger.Warn("failed to read persisted reader config", "reader", readerName, "error", err)
		return nil
	}
	if !ok {
		return nil
	}
	return data
}
