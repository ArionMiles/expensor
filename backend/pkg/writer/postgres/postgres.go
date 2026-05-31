// Package postgres provides a PostgreSQL writer for transaction storage.
package postgres

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/attribute"

	"github.com/ArionMiles/expensor/backend/internal/migration"
	"github.com/ArionMiles/expensor/backend/migrations"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/observability"
)

// RunMigrations applies all numbered SQL migrations from the embedded migrations
// directory. Exported so integration tests can bootstrap a schema without
// importing the full Writer.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	return migration.Run(ctx, pool, migrations.FS, log)
}

// log is a package-level logger used only for RunMigrations bootstrap calls
// (e.g. from tests). The Writer itself uses the logger injected via New.
var log = slog.Default()

// Config holds the PostgreSQL writer configuration.
type Config struct {
	Host     string
	Port     int
	Database string
	User     string
	Password string
	SSLMode  string

	// BatchSize is the number of transactions to buffer before writing.
	BatchSize int
	// FlushInterval is the time between automatic flushes.
	FlushInterval time.Duration

	// MaxPoolSize is the maximum number of connections in the pool.
	MaxPoolSize int
}

// Writer writes transactions to a PostgreSQL database.
type Writer struct {
	pool          *pgxpool.Pool
	logger        *slog.Logger
	batchSize     int
	flushInterval time.Duration
	scope         *observability.Scope
}

// compile-time check: *Writer must satisfy io.Closer.
var _ io.Closer = (*Writer)(nil)

// New creates a new PostgreSQL writer.
func New(cfg Config, logger *slog.Logger) (*Writer, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Set defaults
	if cfg.Port == 0 {
		cfg.Port = 5432
	}
	if cfg.SSLMode == "" {
		cfg.SSLMode = "disable"
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 10
	}
	if cfg.FlushInterval == 0 {
		cfg.FlushInterval = 30 * time.Second
	}
	if cfg.MaxPoolSize == 0 {
		cfg.MaxPoolSize = 10
	}

	// Build connection string
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode,
	)

	// Configure connection pool
	poolConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("parsing connection string: %w", err)
	}

	maxConns := min(cfg.MaxPoolSize, math.MaxInt32)
	poolConfig.MaxConns = int32(maxConns) //nolint:gosec // G115: value is bounded by min(cfg.MaxPoolSize, math.MaxInt32)
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = 1 * time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute
	poolConfig.HealthCheckPeriod = 1 * time.Minute

	// Create connection pool
	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	logger.Info("connected to PostgreSQL",
		"host", cfg.Host,
		"port", cfg.Port,
		"database", cfg.Database,
	)

	w := &Writer{
		pool:          pool,
		logger:        logger,
		batchSize:     cfg.BatchSize,
		flushInterval: cfg.FlushInterval,
		scope:         observability.NewScope(logger, "github.com/ArionMiles/expensor/backend/pkg/writer/postgres"),
	}

	// Run migrations
	if err := w.runMigrations(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return w, nil
}

// runMigrations applies all pending numbered SQL migrations.
func (w *Writer) runMigrations(ctx context.Context) error {
	w.logger.Info("running database migrations")
	if err := migration.Run(ctx, w.pool, migrations.FS, w.logger); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}
	w.logger.Info("migrations completed successfully")
	return nil
}

// flushBatch writes the current batch to PostgreSQL and sends acknowledgments.
// It resets the batch slice to empty on success.
func (w *Writer) flushBatch(ctx context.Context, batch *[]*api.TransactionDetails, ackChan chan<- string) error {
	if len(*batch) == 0 {
		return nil
	}

	if err := w.writeBatch(ctx, *batch); err != nil {
		return err
	}

	for _, txn := range *batch {
		if txn.MessageID == "" {
			continue
		}
		select {
		case ackChan <- txn.MessageID:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	w.logger.Info("wrote transaction batch", "count", len(*batch))
	*batch = (*batch)[:0]
	return nil
}

// Write consumes transactions from the channel and writes them to PostgreSQL.
// It implements batch writing with periodic flushes for performance.
func (w *Writer) Write(ctx context.Context, in <-chan *api.TransactionDetails, ackChan chan<- string) error {
	batch := make([]*api.TransactionDetails, 0, w.batchSize)
	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if err := w.flushBatch(ctx, &batch, ackChan); err != nil {
				w.logger.Error("failed to flush final batch", "error", err)
			}
			return ctx.Err()

		case txn, ok := <-in:
			if !ok {
				return w.flushBatch(ctx, &batch, ackChan)
			}
			batch = append(batch, txn)
			if len(batch) >= w.batchSize {
				if err := w.flushBatch(ctx, &batch, ackChan); err != nil {
					return err
				}
			}

		case <-ticker.C:
			if err := w.flushBatch(ctx, &batch, ackChan); err != nil {
				return err
			}
		}
	}
}

// writeBatch writes a batch of transactions to the database.
// Uses INSERT ON CONFLICT to handle duplicate message_id (upsert logic).
func (w *Writer) writeBatch(ctx context.Context, transactions []*api.TransactionDetails) error {
	if len(transactions) == 0 {
		return nil
	}
	ctx, span := w.observabilityScope().Start(ctx, "postgres_writer.batch_write")
	defer span.End()
	start := time.Now()
	span.SetAttributes(attribute.Int("postgres_writer.batch_size", len(transactions)))
	var writeErr error
	defer func() {
		w.observabilityScope().RecordDuration(ctx, observability.DurationOperation{
			Namespace: "postgres_writer",
			Name:      "batch_write",
			Duration:  time.Since(start),
			Err:       writeErr,
			Attributes: []attribute.KeyValue{
				attribute.Int("batch_size", len(transactions)),
			},
		})
	}()

	// Start a transaction
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		writeErr = err
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Prepare batch insert for transactions
	batch := &pgx.Batch{}
	for _, txn := range transactions {
		currency, timestamp := w.normalizeWriteInput(txn)
		batch.Queue(`
			INSERT INTO transactions (
				message_id, amount, currency, original_amount, original_currency,
				exchange_rate, timestamp, merchant_info, category, bucket, source,
				source_type, source_label, bank, description
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
			ON CONFLICT (message_id) DO UPDATE SET
				amount            = EXCLUDED.amount,
				currency          = EXCLUDED.currency,
				original_amount   = EXCLUDED.original_amount,
				original_currency = EXCLUDED.original_currency,
				exchange_rate     = EXCLUDED.exchange_rate,
				timestamp         = EXCLUDED.timestamp,
				merchant_info     = EXCLUDED.merchant_info,
				source            = EXCLUDED.source,
				source_type       = EXCLUDED.source_type,
				source_label      = EXCLUDED.source_label,
				bank              = EXCLUDED.bank,
				-- Preserve user edits: only fall back to extracted value when the
				-- stored value is NULL or empty (user has not yet set it).
				category = COALESCE(NULLIF(transactions.category, ''), EXCLUDED.category),
				bucket   = COALESCE(NULLIF(transactions.bucket, ''), EXCLUDED.bucket),
				-- description is never produced by extraction; never overwrite it.
				updated_at = NOW()
			RETURNING id
		`,
			txn.MessageID,
			txn.Amount,
			currency,
			txn.OriginalAmount,
			txn.OriginalCurrency,
			txn.ExchangeRate,
			timestamp,
			txn.MerchantInfo,
			txn.Category,
			txn.Bucket,
			txn.Source.Display(),
			txn.Source.Type,
			txn.Source.Label,
			txn.Source.Bank,
			txn.Description,
		)
	}

	// Execute batch
	batchResults := tx.SendBatch(ctx, batch)

	// First: Collect all transaction IDs (fully consume batch results)
	txnIDs := make([]string, len(transactions))
	for i := 0; i < len(transactions); i++ {
		if err := batchResults.QueryRow().Scan(&txnIDs[i]); err != nil {
			_ = batchResults.Close()
			writeErr = err
			return fmt.Errorf("inserting transaction %d: %w", i, err)
		}
	}

	// Close batch results before executing more queries on the transaction
	if err := batchResults.Close(); err != nil {
		writeErr = err
		return fmt.Errorf("closing batch results: %w", err)
	}

	// Second: Insert labels (now safe to use tx.Exec)
	for i, txn := range transactions {
		if len(txn.Labels) > 0 {
			if err := w.insertLabels(ctx, tx, txnIDs[i], txn.Labels); err != nil {
				writeErr = err
				return fmt.Errorf("inserting labels for transaction %s: %w", txnIDs[i], err)
			}
		}
	}

	if err := w.applyMerchantLabels(ctx, tx, txnIDs); err != nil {
		writeErr = err
		return fmt.Errorf("auto-applying merchant labels: %w", err)
	}
	if err := w.applyMerchantCategories(ctx, tx, txnIDs); err != nil {
		writeErr = err
		return fmt.Errorf("auto-applying merchant categories: %w", err)
	}
	if err := w.applyMutedMerchants(ctx, tx, txnIDs); err != nil {
		writeErr = err
		return fmt.Errorf("auto-muting transactions: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		writeErr = err
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

func (w *Writer) observabilityScope() *observability.Scope {
	if w.scope == nil {
		w.scope = observability.NewScope(w.logger, "github.com/ArionMiles/expensor/backend/pkg/writer/postgres")
	}
	return w.scope
}

func (w *Writer) normalizeWriteInput(txn *api.TransactionDetails) (string, time.Time) {
	currency := txn.Currency
	if currency == "" {
		currency = "INR"
	}

	timestamp, err := time.Parse(time.RFC3339, txn.Timestamp)
	if err != nil {
		w.logger.Warn("invalid timestamp format, using current time",
			"timestamp", txn.Timestamp,
			"error", err,
		)
		timestamp = time.Now()
	}

	return currency, timestamp
}

// applyMerchantLabels attaches labels from merchant label mappings to transactions.
func (w *Writer) applyMerchantLabels(ctx context.Context, tx pgx.Tx, txnIDs []string) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO transaction_label_sources (transaction_id, label, source_type, merchant_pattern)
		SELECT t.id, lm.label, 'merchant', lm.merchant_pattern
		FROM transactions t
		JOIN label_merchants lm
		  ON t.merchant_info ILIKE '%' || lm.merchant_pattern || '%'
		WHERE t.id = ANY($1)
		ON CONFLICT (transaction_id, label, source_type, merchant_pattern) DO NOTHING
	`, txnIDs); err != nil {
		return err
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO transaction_labels (transaction_id, label)
		SELECT DISTINCT transaction_id, label
		FROM transaction_label_sources
		WHERE transaction_id = ANY($1)
		ON CONFLICT (transaction_id, label) DO NOTHING
	`, txnIDs)
	return err
}

// applyMerchantCategories fills in category and bucket from merchant category mappings.
func (w *Writer) applyMerchantCategories(ctx context.Context, tx pgx.Tx, txnIDs []string) error {
	_, err := tx.Exec(ctx, `
		WITH ranked_matches AS (
			SELECT
				t.id,
				COALESCE(m.category, mc.category) AS category,
				COALESCE(m.bucket, mc.bucket) AS bucket,
				ROW_NUMBER() OVER (
					PARTITION BY t.id
					ORDER BY LENGTH(mc.fragment) DESC, mc.fragment ASC
				) AS rn
			FROM transactions t
			JOIN merchant_categories mc
			  ON t.merchant_info ILIKE '%' || mc.fragment || '%'
			LEFT JOIN mcc_codes m ON m.code = mc.mcc_code
			WHERE t.id = ANY($1)
			  AND COALESCE(m.category, mc.category) IS NOT NULL
		)
		UPDATE transactions t
		SET category = CASE WHEN COALESCE(t.category, '') = '' THEN rm.category ELSE t.category END,
		    bucket   = CASE WHEN COALESCE(t.bucket, '') = '' THEN rm.bucket ELSE t.bucket END,
		    updated_at = NOW()
		FROM ranked_matches rm
		WHERE t.id = rm.id
		  AND rm.rn = 1
	`, txnIDs)
	return err
}

// applyMutedMerchants marks transactions as muted when they match muted merchant patterns.
func (w *Writer) applyMutedMerchants(ctx context.Context, tx pgx.Tx, txnIDs []string) error {
	_, err := tx.Exec(ctx, `
		UPDATE transactions t
		SET muted = true,
		    muted_by_merchant = true,
		    mute_reason = mm.reason,
		    updated_at = NOW()
		FROM muted_merchants mm
		WHERE t.id = ANY($1)
		  AND t.merchant_info ILIKE '%' || mm.pattern || '%'
	`, txnIDs)
	return err
}

// insertLabels inserts labels for a transaction.
func (w *Writer) insertLabels(ctx context.Context, tx pgx.Tx, txnID string, labels []string) error {
	if len(labels) == 0 {
		return nil
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO transaction_label_sources (transaction_id, label, source_type, merchant_pattern)
		SELECT $1, unnest($2::text[]), 'manual', ''
		ON CONFLICT (transaction_id, label, source_type, merchant_pattern) DO NOTHING
	`, txnID, labels); err != nil {
		return fmt.Errorf("executing label source insert: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO transaction_labels (transaction_id, label)
		SELECT $1, unnest($2::text[])
		ON CONFLICT (transaction_id, label) DO NOTHING
	`, txnID, labels); err != nil {
		return fmt.Errorf("executing label insert: %w", err)
	}

	return nil
}

// Close releases the writer's connection pool. It implements io.Closer.
func (w *Writer) Close() error {
	if w.pool != nil {
		w.pool.Close()
		w.logger.Info("closed PostgreSQL connection pool")
	}
	return nil
}
