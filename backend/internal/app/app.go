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
	communityClose  func(context.Context) error
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
	return newApp(ctx, opts, NewStore)
}

func newApp(ctx context.Context, opts Options, openStore func(context.Context, StoreOptions) (Store, error)) (_ *App, err error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	content, err := catalog.Load()
	if err != nil {
		return nil, errors.B.Op("app.new").KindInternal().Text("loading bundled content").Err(err).Build()
	}
	registry := plugins.NewRegistry()
	if err := registerReaders(registry, content.ReaderGuides); err != nil {
		return nil, errors.B.Op("app.new").Err(err).Build()
	}
	logger.Info("plugins registered", "providers", len(registry.ListProviders()))
	logger.Info("loaded embedded content", "rules", len(content.SystemRules), "mcc_codes", len(content.Seed.MCCEntries),
		"merchant_categories", len(content.Seed.MerchantCategories))
	logger.Info("loaded llm prompt catalog", "prompts", content.PromptCatalog.Len())

	storeRuntime, err := openStore(ctx, StoreOptions{Database: opts.Config.Database, Security: opts.Config.Security, Logger: logger})
	if err != nil {
		return nil, errors.B.Op("app.new").Text("opening store").Err(err).Build()
	}
	constructed := false
	defer func() {
		if !constructed {
			storeRuntime.Close()
		}
	}()
	resolver, err := storeRuntime.Seed(ctx, content.Seed)
	if err != nil {
		return nil, errors.B.Op("app.new").KindInternal().Text("seeding startup content").Err(err).Build()
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
		return nil, errors.B.Op("app.new").Err(err).Build()
	}
	controller, err := daemon.NewController(daemon.ControllerDependencies{Context: ctx, Scanner: scanService, Store: st, Logger: logger})
	if err != nil {
		return nil, errors.B.Op("app.new").Err(err).Build()
	}
	schedulerScope := observability.NewScope(logger.With("component", "scheduler"),
		"github.com/ArionMiles/expensor/backend/internal/daemon/scheduler")
	scheduledScans := scheduler.NewScanRunner(scanService)
	sched, err := scheduler.New(scheduler.Config{
		Store:          st,
		Runner:         scheduler.NewInstrumentedRunner(scheduledScans, schedulerScope),
		PollInterval:   opts.Config.Scheduler.PollInterval,
		BaseRetryDelay: opts.Config.Scheduler.BaseRetryDelay,
		MaxRetryDelay:  opts.Config.Scheduler.MaxRetryDelay,
		Logger:         logger.With("component", "scheduler"),
	})
	if err != nil {
		return nil, errors.B.Op("app.new").Text("constructing scan scheduler").Err(err).Build()
	}
	communityService, err := community.New(ctx, community.Dependencies{
		Config: opts.Config.Community, Store: st, Runtime: st, Resolver: controller, Logger: logger,
	})
	if err != nil {
		return nil, errors.B.Op("app.new").Err(err).Build()
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
		communityClose:  communityService.Close,
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
		return errors.B.Op("app.run").KindFailedPrecondition().Text("application already started").Build()
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
		return errors.B.Op("app.run").KindUnavailable().Text("HTTP server failed").Err(err).Build()
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

		if a.communityClose != nil {
			a.closeErr = errors.Join(a.closeErr, a.communityClose(ctx))
		}
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
