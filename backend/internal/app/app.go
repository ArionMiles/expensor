package app

import (
	"context"
	"log/slog"
	"sync"

	"github.com/ArionMiles/expensor/backend/internal/catalog"
	"github.com/ArionMiles/expensor/backend/internal/community"
	"github.com/ArionMiles/expensor/backend/internal/daemon"
	"github.com/ArionMiles/expensor/backend/internal/daemon/scheduler"
	"github.com/ArionMiles/expensor/backend/internal/observability"
	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

var openApplicationStore = NewStore

// Options configures application composition.
type Options struct {
	Config   config.App
	Logger   *slog.Logger
	LogLevel *slog.LevelVar
}

// App owns the fully composed application runtime.
type App struct {
	logger          *slog.Logger
	schedulerRun    func(context.Context) error
	communityRun    func(context.Context) error
	serverRun       func(context.Context) error
	controllerClose func(context.Context) error
	storeClose      func()

	runMu      sync.Mutex
	runStarted bool
	runCancel  context.CancelFunc
	workers    sync.WaitGroup
	closeOnce  sync.Once
	closeErr   error
}

// New fully constructs application dependencies and performs startup seeding without starting goroutines.
func New(ctx context.Context, opts Options) (_ *App, err error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	content, err := catalog.Load()
	if err != nil {
		return nil, errors.E("app.new", errors.Internal, "loading bundled content", err)
	}
	registry := plugins.NewRegistry()
	if err := registerReaders(registry, content.ReaderGuides); err != nil {
		return nil, errors.E("app.new", err)
	}
	logger.Info("plugins registered", "providers", len(registry.ListProviders()))
	logger.Info("loaded embedded content", "rules", len(content.SystemRules), "mcc_codes", len(content.Seed.MCCEntries),
		"merchant_categories", len(content.Seed.MerchantCategories))
	logger.Info("loaded llm prompt catalog", "prompts", content.PromptCatalog.Len())

	storeRuntime, err := openApplicationStore(ctx, StoreOptions{Database: opts.Config.Database, Security: opts.Config.Security, Logger: logger})
	if err != nil {
		return nil, errors.E("app.new", "opening store", err)
	}
	constructed := false
	defer func() {
		if !constructed {
			storeRuntime.Close()
		}
	}()
	resolver, err := storeRuntime.Seed(ctx, content.Seed)
	if err != nil {
		return nil, errors.E("app.new", errors.Internal, "seeding startup content", err)
	}
	st := storeRuntime.Store
	llmComponents, err := newLLMRuntime(content, st, logger)
	if err != nil {
		return nil, err
	}
	logger.Info("LLM router initialized", "providers", len(llmComponents.registry.ListProviders()),
		"prompts", llmComponents.router.PromptCatalog().Len())

	scanService, err := daemon.NewScanService(daemon.ScanDependencies{
		Registry: registry, Config: opts.Config, SystemRules: content.SystemRules, Resolver: resolver,
		Store: st, Diagnostics: st, TransactionWriter: storeRuntime.Ingestion, Logger: logger,
	})
	if err != nil {
		return nil, errors.E("app.new", err)
	}
	controller, err := daemon.NewController(daemon.ControllerDependencies{Context: ctx, Scanner: scanService, Store: st, Logger: logger})
	if err != nil {
		return nil, errors.E("app.new", err)
	}
	schedulerScope := observability.NewScope(logger.With("component", "scheduler"),
		"github.com/ArionMiles/expensor/backend/internal/daemon/scheduler")
	scheduledScans := scheduler.NewScanRunner(scanService)
	sched := scheduler.New(scheduler.Config{
		Store: st, Runner: scheduler.NewInstrumentedRunner(scheduledScans, schedulerScope), Logger: logger.With("component", "scheduler"),
	})
	communityService, err := community.New(ctx, community.Dependencies{
		Config: opts.Config.Community, Store: st, Runtime: st, Resolver: controller, Logger: logger,
	})
	if err != nil {
		return nil, errors.E("app.new", err)
	}
	server := newHTTPServer(httpDependencies{
		config: opts.Config, content: content, registry: registry, llm: llmComponents, store: st,
		controller: controller, community: communityService, logger: logger, logLevel: opts.LogLevel,
	})

	application := &App{
		logger:          logger,
		schedulerRun:    sched.Start,
		communityRun:    communityService.Run,
		serverRun:       server.Start,
		controllerClose: controller.Close,
		storeClose:      storeRuntime.Close,
	}
	constructed = true
	return application, nil
}

// Run starts background workers and blocks in the HTTP server.
func (a *App) Run(ctx context.Context) error {
	a.runMu.Lock()
	if a.runStarted {
		a.runMu.Unlock()
		return errors.E("app.run", errors.FailedPrecondition, "application already started")
	}
	runCtx, cancel := context.WithCancel(ctx)
	a.runStarted = true
	a.runCancel = cancel
	a.workers.Add(2)
	a.runMu.Unlock()

	defer cancel()
	go a.runWorker(runCtx, "scheduler", a.schedulerRun)
	go a.runWorker(runCtx, "community sync", a.communityRun)
	a.logger.Info("multi-tenant scanning scheduler started")
	if err := a.serverRun(runCtx); err != nil && !errors.Is(err, context.Canceled) {
		return errors.E("app.run", errors.Unavailable, "HTTP server failed", err)
	}
	return nil
}

// Close idempotently stops application work and releases store resources.
func (a *App) Close(ctx context.Context) error {
	a.closeOnce.Do(func() {
		a.runMu.Lock()
		if a.runCancel != nil {
			a.runCancel()
		}
		a.runMu.Unlock()

		if a.controllerClose != nil {
			a.closeErr = errors.Join(a.closeErr, a.controllerClose(ctx))
		}
		a.closeErr = errors.Join(a.closeErr, a.waitWorkers(ctx))
		if a.storeClose != nil {
			a.storeClose()
		}
	})
	return a.closeErr
}

func (a *App) runWorker(ctx context.Context, name string, run func(context.Context) error) {
	defer a.workers.Done()
	err := run(ctx)
	if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return
	}
	if err != nil {
		a.logger.Error(name+" stopped with error", "error", err)
		return
	}
	a.logger.Error(name + " stopped unexpectedly")
}

func (a *App) waitWorkers(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		a.workers.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
