package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/daemon"
	scanscheduler "github.com/ArionMiles/expensor/backend/internal/daemon/scheduler"
	"github.com/ArionMiles/expensor/backend/internal/httpapi"
	"github.com/ArionMiles/expensor/backend/internal/llm"
	"github.com/ArionMiles/expensor/backend/internal/observability"
	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/config"
)

func main() {
	os.Exit(run())
}

func run() int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration: %v\n", err)
		return 1
	}

	observabilityRuntime, err := observability.Setup(context.Background(), cfg.Observability)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize observability: %v\n", err)
		return 1
	}
	logger := observabilityRuntime.Logger
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := observabilityRuntime.Shutdown(shutdownCtx); err != nil {
			logger.Warn("failed to shutdown observability", "error", err)
		}
	}()

	registry := plugins.NewRegistry()
	if err := registerPlugins(registry, readersFS, logger); err != nil {
		logger.Error("failed to register plugins", "error", err)
		return 1
	}
	logger.Info("plugins registered",
		"providers", len(registry.ListProviders()),
	)

	if err := waitForPostgres(cfg.Postgres, logger); err != nil {
		logger.Error("postgres not reachable at startup", "error", err)
		return 1
	}

	content, err := parseEmbedded(rulesInput, mccInput, categoriesInput)
	if err != nil {
		logger.Error("failed to parse embedded content", "error", err)
		return 1
	}
	logger.Info("loaded embedded content", "rules", len(content.rules), "mcc_codes", len(content.mccEntries), "merchant_categories", len(content.catEntries))

	promptCatalog, err := llm.LoadPromptCatalog(llmPromptsFS, "content/llm/prompts")
	if err != nil {
		logger.Error("failed to load llm prompt catalog", "error", err)
		return 1
	}
	logger.Info("loaded llm prompt catalog", "prompts", promptCatalog.Len())

	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		logger.Info("received shutdown signal", "signal", sig)
		cancel()
	}()

	if err := runMigrations(cfg.Postgres, logger); err != nil {
		logger.Error("failed to run migrations", "error", err)
		return 1
	}

	pgStore, storeErr := store.NewWithSecurity(cfg.Postgres, cfg.Security, logger.With("component", "store"))
	if storeErr != nil {
		logger.Error("failed to connect store", "error", storeErr)
		return 1
	}

	resolver, err := seedStartupData(ctx, pgStore, content, logger)
	if err != nil {
		pgStore.Close()
		logger.Error("startup seeding failed", "error", err)
		return 1
	}
	defer pgStore.Close()

	storeLogger := logger.With("component", "store")
	storeScope := observability.NewScope(storeLogger, "github.com/ArionMiles/expensor/backend/internal/store")
	instrumentedStore := store.NewInstrumentedStore(pgStore, storeScope, storeLogger)
	var st httpapi.Storer = instrumentedStore

	llmRegistry := llm.NewRegistry()
	llmRouter := llm.NewRouter(llm.RouterConfig{
		Registry: llmRegistry,
		Runtime:  instrumentedStore,
		Prompts:  promptCatalog,
	})
	logger.Info("LLM router initialized", "providers", len(llmRegistry.ListProviders()), "prompts", llmRouter.PromptCatalog().Len())

	dm := &daemonManager{}
	sinkFactory := func(tenant store.Tenant, appCfg *config.App, sinkLogger *slog.Logger) (daemon.TransactionSink, error) {
		return pgStore.NewTransactionIngestor(store.IngestionConfig{
			Tenant:        tenant,
			BatchSize:     appCfg.Postgres.BatchSize,
			FlushInterval: time.Duration(appCfg.Postgres.FlushInterval) * time.Second,
		}, sinkLogger), nil
	}
	dc := &daemonCoordinator{
		ctx: ctx, registry: registry, cfg: cfg,
		systemRules: content.rules, resolver: resolver,
		st: st, runtimeStore: instrumentedStore, resolverStore: instrumentedStore,
		diagnostics: instrumentedStore, sinkFactory: sinkFactory, dm: dm, logger: logger,
	}

	schedulerScope := observability.NewScope(logger.With("component", "scheduler"), "github.com/ArionMiles/expensor/backend/internal/daemon/scheduler")
	scanRunner := &scheduledScanRunner{
		registry: registry, cfg: cfg, systemRules: content.rules, resolver: resolver,
		st: st, runtimeStore: instrumentedStore, diagnostics: instrumentedStore, sinkFactory: sinkFactory, logger: logger,
	}
	scanScheduler := scanscheduler.New(scanscheduler.Config{
		Store:  instrumentedStore,
		Runner: scanscheduler.NewInstrumentedRunner(scanRunner, schedulerScope),
		Logger: logger.With("component", "scheduler"),
	})
	go func() {
		if err := scanScheduler.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("scheduler stopped with error", "error", err)
		}
	}()
	logger.Info("multi-tenant scanning scheduler started")

	syncFn := startCommunitySync(ctx, communitySyncDependencies{
		config:      cfg.Community,
		store:       instrumentedStore,
		pgStore:     pgStore,
		coordinator: dc,
		logger:      logger,
	})
	handlers := httpapi.NewHandlers(httpapi.HandlersConfig{
		Registry:           registry,
		Store:              st,
		Daemon:             dm,
		Version:            config.Version,
		BaseURL:            cfg.BaseURL,
		FrontendURL:        cfg.FrontendURL,
		ThunderbirdDataDir: cfg.Thunderbird.DataDir,
		ScanInterval:       cfg.ScanInterval,
		LookbackDays:       cfg.LookbackDays,
		StartFn:            dc.start,
		StopFn:             dc.stop,
		RescanFn:           dc.rescan,
		RestartFn:          dc.restart,
		SyncFn:             syncFn,
		BanksData:          banksInput,
		Logger:             logger.With("component", "api"),
		LogLevel:           observabilityRuntime.LogLevel,
	})
	server := httpapi.NewServer(cfg.Port, handlers, cfg.StaticDir, logger.With("component", "http"))

	if err := server.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("HTTP server error", "error", err)
		return 1
	}
	logger.Info("shutdown complete")
	return 0
}
