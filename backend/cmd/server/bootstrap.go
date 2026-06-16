package main

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArionMiles/expensor/backend/internal/bootstrapdb"
	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	"github.com/ArionMiles/expensor/backend/pkg/reader/gmail"
	"github.com/ArionMiles/expensor/backend/pkg/reader/thunderbird"
	"github.com/ArionMiles/expensor/backend/pkg/writer/postgres"
)

// runMigrations applies migrations through a short-lived startup pool.
func runMigrations(pgCfg config.Postgres, logger *slog.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return bootstrapdb.Prepare(ctx, pgCfg, logger)
}

// waitForPostgres retries connectivity until the configured startup deadline.
func waitForPostgres(pgCfg config.Postgres, logger *slog.Logger) error {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s pool_max_conns=1",
		pgCfg.Host, pgCfg.Port, pgCfg.User, pgCfg.Password, pgCfg.Database, pgCfg.SSLMode,
	)
	deadline := time.Now().Add(pgCfg.ConnectTimeout)
	for attempt := 1; ; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		pool, err := pgxpool.New(ctx, connStr)
		cancel()
		if err == nil {
			pingCtx, pingCancel := context.WithTimeout(context.Background(), 3*time.Second)
			err = pool.Ping(pingCtx)
			pingCancel()
			pool.Close()
			if err == nil {
				logger.Info("postgres is ready", "host", pgCfg.Host, "port", pgCfg.Port)
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("postgres not ready after %s: %w", pgCfg.ConnectTimeout, err)
		}
		logger.Info("waiting for postgres", "attempt", attempt, "error", err)
		time.Sleep(pgCfg.RetryInterval)
	}
}

// registerPlugins assembles the application's concrete reader and writer catalog.
func registerPlugins(registry *plugins.Registry, fs embed.FS, logger *slog.Logger) error {
	gmailPlugin := &gmail.Plugin{}
	tbPlugin := &thunderbird.Plugin{}

	if data, err := fs.ReadFile("content/readers/gmail/guide.json"); err == nil {
		gmailPlugin.SetGuideData(data)
	} else {
		logger.Warn("could not load gmail guide", "error", err)
	}
	if data, err := fs.ReadFile("content/readers/thunderbird/guide.json"); err == nil {
		tbPlugin.SetGuideData(data)
	} else {
		logger.Warn("could not load thunderbird guide", "error", err)
	}

	for _, p := range []plugins.ReaderPlugin{gmailPlugin, tbPlugin} {
		if err := registry.RegisterReader(p); err != nil {
			return fmt.Errorf("registering reader %s: %w", p.Metadata().Name, err)
		}
	}
	postgresPlugin := &postgres.Plugin{}
	if err := registry.RegisterWriter(postgresPlugin); err != nil {
		return fmt.Errorf("registering writer %s: %w", postgresPlugin.Metadata().Name, err)
	}
	return nil
}

// seedStartupData persists bundled content and builds the category resolver.
func seedStartupData(ctx context.Context, pgStore *store.Store, content embeddedContent, logger *slog.Logger) (api.CategoryResolver, error) {
	systemRuleRows := buildSystemRuleRows(content.rawRules)
	if err := pgStore.SeedPredefinedRules(ctx, systemRuleRows); err != nil {
		return nil, fmt.Errorf("seeding predefined rules: %w", err)
	}
	logger.Info("predefined rules seeded", "count", len(systemRuleRows))

	if err := pgStore.SeedMCCCodes(ctx, content.mccEntries); err != nil {
		return nil, fmt.Errorf("seeding MCC codes: %w", err)
	}
	if _, err := pgStore.SeedMerchantCategories(ctx, content.catEntries); err != nil {
		return nil, fmt.Errorf("seeding merchant categories: %w", err)
	}
	mccCategoryNames := uniqueCategoryNames(content.mccEntries)
	if err := pgStore.SeedMCCCategories(ctx, mccCategoryNames); err != nil {
		return nil, fmt.Errorf("seeding MCC category names: %w", err)
	}

	resolver, err := pgStore.LoadCategorySnapshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading category snapshot: %w", err)
	}
	logger.Info("category resolver loaded")
	return resolver, nil
}
