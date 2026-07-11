package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ArionMiles/expensor/backend/internal/observability"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/internal/store/instrumented"
	"github.com/ArionMiles/expensor/backend/internal/store/postgres"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

// StoreOptions configures the application store.
type StoreOptions struct {
	Database config.Database
	Security config.Security
	Logger   *slog.Logger
}

// Store contains backend-neutral store dependencies used by the application.
type Store struct {
	Store     *instrumented.Store
	Seeder    store.Seeder
	Ingestion store.TransactionBatchWriter

	close func()
}

// NewStore opens the configured database backend and wraps it with instrumentation.
func NewStore(ctx context.Context, opts StoreOptions) (Store, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	logDatabaseBackend(opts.Database, logger)

	backend, err := openStoreBackend(ctx, opts)
	if err != nil {
		return Store{}, err
	}

	storeLogger := logger.With("component", "store")
	storeScope := observability.NewScope(storeLogger, "github.com/ArionMiles/expensor/backend/internal/store")
	instrumentedStore := instrumented.NewStore(instrumented.StoreDeps{
		Auth:         backend,
		Analytics:    backend,
		Community:    backend,
		Diagnostics:  backend,
		Rules:        backend,
		Runtime:      backend,
		Scanning:     backend,
		Taxonomy:     backend,
		Transactions: backend,
	}, storeScope, storeLogger)
	instrumentedIngestion := instrumented.NewTransactionBatchWriter(backend, storeScope, storeLogger)

	return Store{
		Store:     instrumentedStore,
		Seeder:    backend,
		Ingestion: instrumentedIngestion,
		close:     backend.Close,
	}, nil
}

// Close releases backend store resources.
func (s Store) Close() {
	if s.close != nil {
		s.close()
	}
}

// Seed persists bundled startup content through the configured backend.
func (s Store) Seed(ctx context.Context, content store.SeedContent) (api.CategoryResolver, error) {
	if s.Seeder == nil {
		return nil, errors.E("app.store.seed", errors.FailedPrecondition, "store backend does not support startup seeding")
	}
	return s.Seeder.Seed(ctx, content)
}

func openStoreBackend(ctx context.Context, opts StoreOptions) (store.Backend, error) {
	switch opts.Database.Backend {
	case config.DatabaseBackendPostgres:
		return postgres.New(ctx, postgres.Options{
			Config:   opts.Database.Postgres,
			Security: opts.Security,
			Logger:   opts.Logger,
		})
	case "", config.DatabaseBackendSQLite:
		return nil, errors.E("app.store.new", errors.FailedPrecondition, "sqlite database backend is not supported yet")
	default:
		return nil, errors.E("app.store.new", errors.InvalidArgument, fmt.Sprintf("unsupported database backend %q", opts.Database.Backend))
	}
}

func logDatabaseBackend(database config.Database, logger *slog.Logger) {
	if database.Backend == "" {
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
