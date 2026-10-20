// Package sqlite provides SQLite query and persistence operations for Expensor.
package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/auth"
	"github.com/ArionMiles/expensor/backend/internal/store/sqlite/migrations"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

// Options configures the SQLite store.
type Options struct {
	Config   config.SQLite
	Security config.Security
}

// Store owns the SQLite connection pool and shared repository dependencies.
type Store struct {
	db                     *sql.DB
	writeGate              chan struct{}
	acquireWriteConnection func(context.Context) (writeConnection, error)
	repositories           repositoryDependencies
}

type lifecycleHooks struct {
	health            func(context.Context, *Store) error
	migrate           func(context.Context, *Store) error
	initRepositories  func(*Store, config.Security) error
	pathCreation      pathCreationHooks
	afterDatabaseOpen func()
}

type pathCreationHooks struct {
	afterDirectoryCreate func()
	afterFileCreate      func()
	beforeRetry          func()
}

// New opens, checks, migrates, and initializes a SQLite store.
func New(ctx context.Context, opts Options) (*Store, error) {
	return newWithLifecycle(ctx, opts, defaultLifecycleHooks())
}

func defaultLifecycleHooks() lifecycleHooks {
	return lifecycleHooks{
		health: func(ctx context.Context, store *Store) error {
			return store.HealthCheck(ctx)
		},
		migrate: func(ctx context.Context, store *Store) error {
			return migrations.Run(ctx, store.db)
		},
		initRepositories: func(store *Store, security config.Security) error {
			return store.initRepositories(security)
		},
	}
}

func newWithLifecycle(ctx context.Context, opts Options, hooks lifecycleHooks) (result *Store, resultErr error) {
	prepared, err := prepareDatabasePath(ctx, opts.Config.Path, hooks.pathCreation)
	if err != nil {
		return nil, errors.B.Op("sqlite.store.new").Text("preparing store path").Err(err).Build()
	}
	defer func() {
		if closeErr := prepared.close(); closeErr != nil {
			if result != nil {
				result.Close()
				result = nil
			}
			resultErr = errors.Join(resultErr, closeErr)
		}
	}()

	db, err := openDatabase(ctx, prepared.dsn, opts.Config.BusyTimeout)
	if err != nil {
		return nil, errors.B.Op("sqlite.store.new").Text("opening store database").Err(err).Build()
	}
	if hooks.afterDatabaseOpen != nil {
		hooks.afterDatabaseOpen()
	}
	if err := prepared.verifyLiveIdentity(ctx, db); err != nil {
		_ = db.Close()
		return nil, errors.B.Op("sqlite.store.new").Text("verifying opened database").Err(err).Build()
	}
	if err := prepared.close(); err != nil {
		_ = db.Close()
		return nil, errors.B.Op("sqlite.store.new").Text("releasing database validation handle").Err(err).Build()
	}
	store := &Store{
		db:        db,
		writeGate: make(chan struct{}, 1),
	}
	store.acquireWriteConnection = func(ctx context.Context) (writeConnection, error) {
		return store.db.Conn(ctx)
	}
	store.writeGate <- struct{}{}

	if err := hooks.health(ctx, store); err != nil {
		store.Close()
		return nil, err
	}
	if err := hooks.migrate(ctx, store); err != nil {
		store.Close()
		return nil, errors.B.Op("sqlite.store.new").Text("migrating store database").Err(err).Build()
	}
	if err := hooks.initRepositories(store, opts.Security); err != nil {
		store.Close()
		return nil, err
	}
	return store, nil
}

// HealthCheck verifies that the SQLite database is reachable.
func (s *Store) HealthCheck(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return errors.B.Op("sqlite.store.health_check").KindUnavailable().Text("pinging store database").Err(err).Build()
	}
	return nil
}

// Close releases the SQLite connection pool.
func (s *Store) Close() {
	if s != nil && s.db != nil {
		_ = s.db.Close()
	}
}

func (s *Store) initRepositories(security config.Security) error {
	var secretBox *auth.SecretBox
	if len(security.SecretKey) > 0 {
		var err error
		secretBox, err = auth.NewSecretBox(security.SecretKey)
		if err != nil {
			return errors.B.Op("sqlite.store.init_repositories").KindInvalidArgument().Text("creating store secret box").Err(err).Build()
		}
	}
	s.repositories = repositoryDependencies{
		db:        s.db,
		query:     s.db,
		writeTx:   s.withImmediateWrite,
		now:       time.Now,
		secretBox: secretBox,
	}
	return nil
}
