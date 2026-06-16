// Package bootstrapdb prepares the backend database schema at startup.
package bootstrapdb

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArionMiles/expensor/backend/internal/dbconn"
	"github.com/ArionMiles/expensor/backend/migrations"
	"github.com/ArionMiles/expensor/backend/pkg/config"
)

// Hooks provides test seams for the one-time bridge.
type Hooks struct {
	ValidateCopy func(context.Context) error
}

var bridgeTables = []string{
	"app_config",
	"buckets",
	"categories",
	"extraction_diagnostics",
	"label_merchants",
	"labels",
	"mcc_codes",
	"merchant_categories",
	"muted_merchants",
	"processed_messages",
	"reader_runtime",
	"rules",
	"transaction_label_sources",
	"transaction_labels",
	"transactions",
}

// Prepare opens a short-lived startup pool, prepares the database schema, and
// returns once the backend can use expensor as the active application schema.
func Prepare(ctx context.Context, cfg config.Postgres, logger *slog.Logger) error {
	return PrepareWithHooks(ctx, cfg, logger, Hooks{})
}

// PrepareWithHooks behaves like Prepare, but allows tests to inject a failure
// after the bridge validates the copied schema and before the transaction
// commits.
func PrepareWithHooks(ctx context.Context, cfg config.Postgres, logger *slog.Logger, hooks Hooks) error {
	if logger == nil {
		logger = slog.Default()
	}

	pool, err := openPool(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	expensorExists, err := schemaExists(ctx, pool, "expensor")
	if err != nil {
		return err
	}
	if expensorExists {
		logger.Debug("expensor schema already exists; running embedded migrations")
		return migrations.Run(ctx, pool, logger)
	}

	legacyExists, err := legacyAppExists(ctx, pool)
	if err != nil {
		return err
	}
	if !legacyExists {
		logger.Debug("fresh database detected; running embedded migrations")
		return migrations.Run(ctx, pool, logger)
	}

	logger.Info("legacy public schema detected; preparing expensor bridge")
	if err := copyLegacySchema(ctx, pool, logger, hooks); err != nil {
		return err
	}
	if err := migrations.Baseline(ctx, pool, logger); err != nil {
		return err
	}
	logger.Info("database bridged to expensor schema")
	return nil
}

func openPool(ctx context.Context, cfg config.Postgres) (*pgxpool.Pool, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s pool_max_conns=1",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode,
	)
	poolCfg, err := dbconn.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("opening bootstrap pool: %w", err)
	}
	poolCfg.MaxConns = 1
	return pgxpool.NewWithConfig(ctx, poolCfg)
}

func schemaExists(ctx context.Context, pool *pgxpool.Pool, schema string) (bool, error) {
	var exists bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.schemata
			WHERE schema_name = $1
		)
	`, schema).Scan(&exists); err != nil {
		return false, fmt.Errorf("checking schema %s: %w", schema, err)
	}
	return exists, nil
}

func legacyAppExists(ctx context.Context, pool *pgxpool.Pool) (bool, error) {
	return tableExists(ctx, pool, "public", "transactions")
}

func tableExists(ctx context.Context, pool *pgxpool.Pool, schema, table string) (bool, error) {
	var exists bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = $1
			  AND table_name = $2
		)
	`, schema, table).Scan(&exists); err != nil {
		return false, fmt.Errorf("checking table %s.%s: %w", schema, table, err)
	}
	return exists, nil
}

func copyLegacySchema(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger, hooks Hooks) (err error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting bridge transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	copiedTables, err := copyLegacyTables(ctx, tx)
	if err != nil {
		return err
	}
	if err := installLegacyTriggers(ctx, tx); err != nil {
		return err
	}
	if err := validateLegacyCopy(ctx, tx, copiedTables); err != nil {
		return err
	}
	if hooks.ValidateCopy != nil {
		if err = hooks.ValidateCopy(ctx); err != nil {
			return fmt.Errorf("validating copied schema: %w", err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing bridge transaction: %w", err)
	}
	logger.Info("legacy schema copied into expensor", "tables", strings.Join(copiedTables, ","))
	return nil
}

func copyLegacyTables(ctx context.Context, tx pgx.Tx) ([]string, error) {
	if _, err := tx.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS expensor`); err != nil {
		return nil, fmt.Errorf("creating expensor schema: %w", err)
	}
	if _, err := tx.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS pg_trgm`); err != nil {
		return nil, fmt.Errorf("creating pg_trgm extension: %w", err)
	}

	copiedTables := make([]string, 0, len(bridgeTables))
	for _, table := range bridgeTables {
		exists, err := tableExistsTx(ctx, tx, "public", table)
		if err != nil {
			return nil, err
		}
		if !exists {
			continue
		}
		if err := copyLegacyTable(ctx, tx, table); err != nil {
			return nil, err
		}
		copiedTables = append(copiedTables, table)
	}
	return copiedTables, nil
}

func copyLegacyTable(ctx context.Context, tx pgx.Tx, table string) error {
	if _, err := tx.Exec(ctx, fmt.Sprintf(
		`CREATE TABLE expensor.%s (LIKE public.%s INCLUDING ALL)`, table, table,
	)); err != nil {
		return fmt.Errorf("creating expensor.%s from public.%s: %w", table, table, err)
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(
		`INSERT INTO expensor.%s SELECT * FROM public.%s ON CONFLICT DO NOTHING`, table, table,
	)); err != nil {
		return fmt.Errorf("copying public.%s into expensor.%s: %w", table, table, err)
	}
	return nil
}

func installLegacyTriggers(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, `
		CREATE OR REPLACE FUNCTION expensor.update_updated_at_column()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = NOW();
			RETURN NEW;
		END;
		$$ language 'plpgsql'
	`); err != nil {
		return fmt.Errorf("creating expensor update_updated_at_column function: %w", err)
	}
	if _, err := tx.Exec(ctx, `DROP TRIGGER IF EXISTS update_transactions_updated_at ON expensor.transactions`); err != nil {
		return fmt.Errorf("dropping expensor transaction trigger: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		CREATE TRIGGER update_transactions_updated_at
			BEFORE UPDATE ON expensor.transactions
			FOR EACH ROW
			EXECUTE FUNCTION expensor.update_updated_at_column()
	`); err != nil {
		return fmt.Errorf("creating expensor transaction trigger: %w", err)
	}
	return nil
}

func validateLegacyCopy(ctx context.Context, tx pgx.Tx, tables []string) error {
	for _, table := range tables {
		var publicCount, expensorCount int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM public.%s`, table)).Scan(&publicCount); err != nil {
			return fmt.Errorf("counting public.%s rows: %w", table, err)
		}
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM expensor.%s`, table)).Scan(&expensorCount); err != nil {
			return fmt.Errorf("counting expensor.%s rows: %w", table, err)
		}
		if publicCount != expensorCount {
			return fmt.Errorf("row count mismatch for %s: public=%d expensor=%d", table, publicCount, expensorCount)
		}
	}
	return nil
}

func tableExistsTx(ctx context.Context, tx pgx.Tx, schema, table string) (bool, error) {
	var exists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = $1
			  AND table_name = $2
		)
	`, schema, table).Scan(&exists); err != nil {
		return false, fmt.Errorf("checking table %s.%s: %w", schema, table, err)
	}
	return exists, nil
}
