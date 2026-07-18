// Package daemon provides the core daemon runner for Expensor.
package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/ArionMiles/expensor/backend/internal/observability"
	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/internal/state"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	apperrors "github.com/ArionMiles/expensor/backend/pkg/errors"
)

// Runner manages the expense tracking daemon lifecycle.
type Runner struct {
	registry          *plugins.Registry
	transactionWriter store.TransactionBatchWriter
	diagnostics       DiagnosticStore
	httpClient        *http.Client
	logger            *slog.Logger
	scope             *observability.Scope
}

// RunnerDeps contains the application dependencies for a daemon runner.
type RunnerDeps struct {
	Registry          *plugins.Registry
	TransactionWriter store.TransactionBatchWriter
	Diagnostics       DiagnosticStore
	HTTPClient        *http.Client
	Logger            *slog.Logger
	Scope             *observability.Scope
}

// New creates a new daemon runner.
func New(deps RunnerDeps) *Runner {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if deps.Scope == nil {
		deps.Scope = observability.NewScope(deps.Logger, "github.com/ArionMiles/expensor/backend/internal/daemon")
	}

	return &Runner{
		registry:          deps.Registry,
		transactionWriter: deps.TransactionWriter,
		diagnostics:       deps.Diagnostics,
		httpClient:        deps.HTTPClient,
		logger:            deps.Logger,
		scope:             deps.Scope,
	}
}

// TransactionSink consumes extracted transactions and acknowledges successfully persisted message IDs.
type TransactionSink interface {
	Write(ctx context.Context, in <-chan *api.TransactionDetails, ackChan chan<- string) error
}

// DiagnosticStore persists extraction diagnostics for a tenant-scoped run.
type DiagnosticStore interface {
	RecordExtractionDiagnostic(ctx context.Context, tenant store.Tenant, diagnostic api.ExtractionDiagnostic) error
}

type tenantDiagnosticSink struct {
	store  DiagnosticStore
	tenant store.Tenant
}

func (s tenantDiagnosticSink) RecordExtractionDiagnostic(ctx context.Context, diagnostic api.ExtractionDiagnostic) error {
	return s.store.RecordExtractionDiagnostic(ctx, s.tenant, diagnostic)
}

func (r *Runner) diagnosticSink(tenant store.Tenant) api.DiagnosticSink {
	if r.diagnostics == nil {
		return nil
	}
	return tenantDiagnosticSink{store: r.diagnostics, tenant: tenant}
}

// RunConfig holds the configuration for running the daemon.
type RunConfig struct {
	// ReaderName is the provider name of the reader to use (e.g. "gmail").
	// Set by the web UI via POST /api/daemon/start.
	ReaderName   string
	Tenant       store.Tenant
	Config       *config.App
	Rules        []api.Rule
	Resolver     api.CategoryResolver
	StateManager *state.Manager
	RuntimeStore ReaderRuntimeStore
	// ForceRescan bypasses state deduplication for the current run.
	// When true, StateManager should be nil — readers handle nil gracefully.
	ForceRescan bool
}

// ReaderRuntimeStore loads reader runtime configuration persisted by the API.
type ReaderRuntimeStore interface {
	GetReaderConfig(ctx context.Context, tenant store.Tenant, reader string) (json.RawMessage, bool, error)
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
	)

	provider, err := r.registry.GetProvider(runCfg.ReaderName)
	if err != nil {
		runErr = err
		return apperrors.E("daemon.run", apperrors.Internal, "creating reader", err)
	}
	reader, err := provider.NewReader(plugins.ProviderInput{
		HTTPClient:     r.httpClient,
		AppConfig:      cfg,
		ReaderConfig:   r.loadReaderConfig(ctx, runCfg.Tenant, runCfg.ReaderName, runCfg.RuntimeStore),
		Rules:          runCfg.Rules,
		Resolver:       runCfg.Resolver,
		StateManager:   runCfg.StateManager,
		DiagnosticSink: r.diagnosticSink(runCfg.Tenant),
		Logger:         r.logger.With("component", "reader", "provider", runCfg.ReaderName),
	})
	if err != nil {
		runErr = err
		return apperrors.E("daemon.run", apperrors.Internal, "creating reader", err)
	}

	ingestionCfg := store.IngestionConfig{Tenant: runCfg.Tenant}
	if cfg != nil {
		ingestionCfg.BatchSize = cfg.Database.BatchSize
		ingestionCfg.FlushInterval = time.Duration(cfg.Database.FlushInterval) * time.Second
	}
	sink, err := newTransactionSink(r.transactionWriter, ingestionCfg, r.logger.With("component", "transaction_ingestion"))
	if err != nil {
		runErr = err
		return apperrors.E("daemon.run", apperrors.Internal, "creating transaction sink", err)
	}

	// Create transaction and acknowledgment channels
	transactions := make(chan *api.TransactionDetails, 100)
	ackChan := make(chan string, 100)

	r.logger.Info("daemon started")

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		defer close(ackChan)
		return sink.Write(gctx, transactions, ackChan)
	})
	g.Go(func() error { return reader.Read(gctx, transactions, ackChan) })

	if err := g.Wait(); err != nil &&
		!errors.Is(err, context.Canceled) &&
		!errors.Is(err, context.DeadlineExceeded) {
		runErr = err
		r.logger.Error("daemon error", "error", err)
	}

	r.logger.Info("daemon stopped")
	return runErr
}

func (r *Runner) loadReaderConfig(ctx context.Context, tenant store.Tenant, readerName string, runtimeStore ReaderRuntimeStore) json.RawMessage {
	if runtimeStore == nil {
		return nil
	}
	data, ok, err := runtimeStore.GetReaderConfig(ctx, tenant, readerName)
	if err != nil {
		r.logger.Warn("failed to read persisted reader config", "reader", readerName, "error", err)
		return nil
	}
	if !ok {
		return nil
	}
	return data
}
