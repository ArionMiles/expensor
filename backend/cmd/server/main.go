package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/app"
	"github.com/ArionMiles/expensor/backend/internal/assistant"
	"github.com/ArionMiles/expensor/backend/internal/catalog"
	"github.com/ArionMiles/expensor/backend/internal/daemon/scheduler"
	"github.com/ArionMiles/expensor/backend/internal/httpapi"
	"github.com/ArionMiles/expensor/backend/internal/llm"
	openaiProvider "github.com/ArionMiles/expensor/backend/internal/llm/openai"
	"github.com/ArionMiles/expensor/backend/internal/observability"
	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
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

	content, err := catalog.Load()
	if err != nil {
		logger.Error("failed to load embedded content", "error", err)
		return 1
	}

	registry := plugins.NewRegistry()
	if err := registerPlugins(registry, content.ReaderGuides); err != nil {
		logger.Error("failed to register plugins", "error", err)
		return 1
	}
	logger.Info("plugins registered",
		"providers", len(registry.ListProviders()),
	)

	logger.Info("loaded embedded content", "rules", len(content.SystemRules), "mcc_codes", len(content.Seed.MCCEntries),
		"merchant_categories", len(content.Seed.MerchantCategories))
	logger.Info("loaded llm prompt catalog", "prompts", content.PromptCatalog.Len())

	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		logger.Info("received shutdown signal", "signal", sig)
		cancel()
	}()

	storeRuntime, storeErr := app.NewStore(ctx, app.StoreOptions{
		Database: cfg.Database,
		Security: cfg.Security,
		Logger:   logger,
	})
	if storeErr != nil {
		logger.Error("failed to connect store", "error", storeErr)
		return 1
	}
	defer storeRuntime.Close()

	resolver, err := storeRuntime.Seed(ctx, content.Seed)
	if err != nil {
		logger.Error("startup seeding failed", "error", err)
		return 1
	}

	instrumentedStore := storeRuntime.Store
	var st httpapi.Storer = instrumentedStore

	llmRegistry := llm.NewRegistry()
	if err := llmRegistry.RegisterProvider(openaiProvider.Provider(content.OpenAIModelOptions)); err != nil {
		logger.Error("failed to register llm provider", "error", err)
		return 1
	}
	llmLogger := logger.With("component", "llm")
	llmScope := observability.NewScope(llmLogger, "github.com/ArionMiles/expensor/backend/internal/llm")
	llmRouter := llm.NewRouter(llm.RouterConfig{
		Registry: llmRegistry,
		Runtime:  instrumentedStore,
		Prompts:  content.PromptCatalog,
		Scope:    llmScope,
		Logger:   llmLogger,
	})
	assistantLogger := logger.With("component", "assistant")
	assistantScope := observability.NewScope(assistantLogger, "github.com/ArionMiles/expensor/backend/internal/assistant")
	ruleDrafts := assistant.NewInstrumentedRuleDrafter(assistant.NewRuleDraftService(llmRouter), assistantScope, assistantLogger)
	logger.Info("LLM router initialized", "providers", len(llmRegistry.ListProviders()), "prompts", llmRouter.PromptCatalog().Len())

	dm := &daemonManager{}
	dc := &daemonCoordinator{
		ctx: ctx, registry: registry, cfg: cfg,
		systemRules: content.SystemRules, resolver: resolver,
		st: st, runtimeStore: instrumentedStore, resolverStore: instrumentedStore,
		diagnostics: instrumentedStore, transactionWriter: storeRuntime.Ingestion, dm: dm, logger: logger,
	}

	schedulerScope := observability.NewScope(logger.With("component", "scheduler"), "github.com/ArionMiles/expensor/backend/internal/daemon/scheduler")
	scanRunner := &scheduledScanRunner{
		registry: registry, cfg: cfg, systemRules: content.SystemRules, resolver: resolver,
		st: st, runtimeStore: instrumentedStore, diagnostics: instrumentedStore, transactionWriter: storeRuntime.Ingestion, logger: logger,
	}
	sched := scheduler.New(scheduler.Config{
		Store:  instrumentedStore,
		Runner: scheduler.NewInstrumentedRunner(scanRunner, schedulerScope),
		Logger: logger.With("component", "scheduler"),
	})
	go func() {
		if err := sched.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("scheduler stopped with error", "error", err)
		}
	}()
	logger.Info("multi-tenant scanning scheduler started")

	syncFn := startCommunitySync(ctx, communitySyncDependencies{
		config:      cfg.Community,
		store:       instrumentedStore,
		runtime:     instrumentedStore,
		coordinator: dc,
		logger:      logger,
	})
	handlers := httpapi.NewHandlers(httpapi.HandlersConfig{
		Registry:           registry,
		LLMRegistry:        llmRegistry,
		LLMRouter:          llmRouter,
		RuleDrafts:         ruleDrafts,
		LLMScope:           llmScope,
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
		BanksData:          content.BanksJSON,
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
