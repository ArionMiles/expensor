package main

import (
	"context"
	"embed"
	"fmt"
	"log/slog"

	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/internal/store/postgres"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
	"github.com/ArionMiles/expensor/backend/pkg/reader/gmail"
	"github.com/ArionMiles/expensor/backend/pkg/reader/thunderbird"
)

type storeRuntime struct {
	Postgres *postgres.Store
}

func (r storeRuntime) Close() {
	if r.Postgres != nil {
		r.Postgres.Close()
	}
}

func logDatabaseBackend(database config.Database, logger *slog.Logger) {
	if !database.BackendConfigured {
		logger.Info("No DB backend configured. Defaulting to sqlite.")
		return
	}
	switch database.Backend {
	case config.DatabaseBackendPostgres:
		logger.Info("PostgreSQL configured as the DB backend.")
	case config.DatabaseBackendSQLite:
		logger.Info("SQLite configured as the DB backend.")
	}
}

func openConfiguredStore(ctx context.Context, cfg config.App, logger *slog.Logger) (storeRuntime, error) {
	switch cfg.Database.Backend {
	case config.DatabaseBackendPostgres:
		return openPostgresStore(ctx, cfg, logger)
	case config.DatabaseBackendSQLite:
		return storeRuntime{}, errors.E(
			"server.open_store",
			errors.FailedPrecondition,
			"sqlite database backend is not supported yet",
		)
	default:
		return storeRuntime{}, errors.E("server.open_store", errors.InvalidArgument, "unsupported database backend")
	}
}

func openPostgresStore(ctx context.Context, cfg config.App, logger *slog.Logger) (storeRuntime, error) {
	pgStore, err := postgres.New(ctx, postgres.Options{
		Config:   cfg.Postgres,
		Security: cfg.Security,
		Logger:   logger.With("component", "store"),
	})
	if err != nil {
		return storeRuntime{}, err
	}
	return storeRuntime{Postgres: pgStore}, nil
}

// registerPlugins assembles the application's concrete provider catalog.
func registerPlugins(registry *plugins.Registry, fs embed.FS, logger *slog.Logger) error {
	var gmailGuide []byte
	if data, err := fs.ReadFile("content/readers/gmail/guide.json"); err == nil {
		gmailGuide = data
	} else {
		logger.Warn("could not load gmail guide", "error", err)
	}
	var thunderbirdGuide []byte
	if data, err := fs.ReadFile("content/readers/thunderbird/guide.json"); err == nil {
		thunderbirdGuide = data
	} else {
		logger.Warn("could not load thunderbird guide", "error", err)
	}

	for _, provider := range []plugins.Provider{gmail.Provider(gmailGuide), thunderbird.Provider(thunderbirdGuide)} {
		if err := registry.RegisterProvider(provider); err != nil {
			return errors.E("server.register_plugins", errors.Internal, fmt.Sprintf("registering provider %s", provider.Metadata.Name), err)
		}
	}
	return nil
}

type startupSeedStore interface {
	SeedPredefinedRules(ctx context.Context, rules []store.RuleRow) error
	SeedMCCCodes(ctx context.Context, entries []store.MCCEntry) error
	SeedMerchantCategories(ctx context.Context, entries []store.MerchantCategoryEntry) (int64, error)
	SeedMCCCategories(ctx context.Context, names []string) error
	LoadCategorySnapshot(ctx context.Context) (api.CategoryResolver, error)
}

// seedStartupData persists bundled content and builds the category resolver.
func seedStartupData(ctx context.Context, runtimeStore startupSeedStore, content embeddedContent, logger *slog.Logger) (api.CategoryResolver, error) {
	systemRuleRows := buildSystemRuleRows(content.rawRules)
	if err := runtimeStore.SeedPredefinedRules(ctx, systemRuleRows); err != nil {
		return nil, errors.E("server.seed_startup_data", errors.Internal, "seeding predefined rules", err)
	}
	logger.Info("predefined rules seeded", "count", len(systemRuleRows))

	if err := runtimeStore.SeedMCCCodes(ctx, content.mccEntries); err != nil {
		return nil, errors.E("server.seed_startup_data", errors.Internal, "seeding MCC codes", err)
	}
	if _, err := runtimeStore.SeedMerchantCategories(ctx, content.catEntries); err != nil {
		return nil, errors.E("server.seed_startup_data", errors.Internal, "seeding merchant categories", err)
	}
	mccCategoryNames := uniqueCategoryNames(content.mccEntries)
	if err := runtimeStore.SeedMCCCategories(ctx, mccCategoryNames); err != nil {
		return nil, errors.E("server.seed_startup_data", errors.Internal, "seeding MCC category names", err)
	}

	resolver, err := runtimeStore.LoadCategorySnapshot(ctx)
	if err != nil {
		return nil, errors.E("server.seed_startup_data", errors.Internal, "loading category snapshot", err)
	}
	logger.Info("category resolver loaded")
	return resolver, nil
}
