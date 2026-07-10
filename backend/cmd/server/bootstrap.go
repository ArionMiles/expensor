package main

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/internal/store/postgres"
	"github.com/ArionMiles/expensor/backend/internal/store/postgres/migrations"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	apperrors "github.com/ArionMiles/expensor/backend/pkg/errors"
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
		return storeRuntime{}, apperrors.E(
			"server.open_store",
			apperrors.FailedPrecondition,
			"sqlite database backend is not supported yet",
		)
	default:
		return storeRuntime{}, apperrors.E("server.open_store", apperrors.InvalidArgument, "unsupported database backend")
	}
}

func openPostgresStore(ctx context.Context, cfg config.App, logger *slog.Logger) (storeRuntime, error) {
	if err := waitForPostgres(ctx, cfg.Postgres, logger); err != nil {
		return storeRuntime{}, err
	}
	if err := runPostgresMigrations(ctx, cfg.Postgres, logger); err != nil {
		return storeRuntime{}, err
	}
	pgStore, err := postgres.NewWithSecurity(cfg.Postgres, cfg.Security, logger.With("component", "store"))
	if err != nil {
		return storeRuntime{}, err
	}
	return storeRuntime{Postgres: pgStore}, nil
}

// runPostgresMigrations applies migrations through a short-lived startup pool.
func runPostgresMigrations(ctx context.Context, pgCfg config.Postgres, logger *slog.Logger) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s pool_max_conns=1",
		pgCfg.Host, pgCfg.Port, pgCfg.User, pgCfg.Password, pgCfg.Database, pgCfg.SSLMode,
	)
	poolCfg, err := postgres.ParsePoolConfig(connStr)
	if err != nil {
		return apperrors.E("server.postgres_migrations", apperrors.InvalidArgument, "parsing migration connection string", err)
	}
	poolCfg.MaxConns = 1

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return apperrors.E("server.postgres_migrations", apperrors.Unavailable, "creating migration pool", err)
	}
	defer pool.Close()

	return migrations.Run(ctx, pool, logger)
}

// waitForPostgres retries connectivity until the configured startup deadline.
func waitForPostgres(ctx context.Context, pgCfg config.Postgres, logger *slog.Logger) error {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s pool_max_conns=1",
		pgCfg.Host, pgCfg.Port, pgCfg.User, pgCfg.Password, pgCfg.Database, pgCfg.SSLMode,
	)
	deadline := time.Now().Add(pgCfg.ConnectTimeout)
	for attempt := 1; ; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		pool, err := pgxpool.New(attemptCtx, connStr)
		cancel()
		if err == nil {
			pingCtx, pingCancel := context.WithTimeout(ctx, 3*time.Second)
			err = pool.Ping(pingCtx)
			pingCancel()
			pool.Close()
			if err == nil {
				logger.Info("postgres is ready", "host", pgCfg.Host, "port", pgCfg.Port)
				return nil
			}
		}
		if time.Now().After(deadline) {
			return apperrors.E("server.wait_for_postgres", apperrors.Unavailable, fmt.Sprintf("postgres not ready after %s", pgCfg.ConnectTimeout), err)
		}
		logger.Info("waiting for postgres", "attempt", attempt, "error", err)
		timer := time.NewTimer(pgCfg.RetryInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return apperrors.E("server.wait_for_postgres", apperrors.Canceled, "waiting for postgres", ctx.Err())
		case <-timer.C:
		}
	}
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
			return apperrors.E("server.register_plugins", apperrors.Internal, fmt.Sprintf("registering provider %s", provider.Metadata.Name), err)
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
		return nil, apperrors.E("server.seed_startup_data", apperrors.Internal, "seeding predefined rules", err)
	}
	logger.Info("predefined rules seeded", "count", len(systemRuleRows))

	if err := runtimeStore.SeedMCCCodes(ctx, content.mccEntries); err != nil {
		return nil, apperrors.E("server.seed_startup_data", apperrors.Internal, "seeding MCC codes", err)
	}
	if _, err := runtimeStore.SeedMerchantCategories(ctx, content.catEntries); err != nil {
		return nil, apperrors.E("server.seed_startup_data", apperrors.Internal, "seeding merchant categories", err)
	}
	mccCategoryNames := uniqueCategoryNames(content.mccEntries)
	if err := runtimeStore.SeedMCCCategories(ctx, mccCategoryNames); err != nil {
		return nil, apperrors.E("server.seed_startup_data", apperrors.Internal, "seeding MCC category names", err)
	}

	resolver, err := runtimeStore.LoadCategorySnapshot(ctx)
	if err != nil {
		return nil, apperrors.E("server.seed_startup_data", apperrors.Internal, "loading category snapshot", err)
	}
	logger.Info("category resolver loaded")
	return resolver, nil
}
