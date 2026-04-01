// Package postgres provides a PostgreSQL writer for transaction storage.
package postgres

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArionMiles/expensor/backend/pkg/api"
)

//go:embed 001_create_transactions.sql
var migrationSQL string

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
}

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
	}

	// Run migrations
	if err := w.runMigrations(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return w, nil
}

// runMigrations runs the database migrations.
func (w *Writer) runMigrations(ctx context.Context) error {
	w.logger.Info("running database migrations")

	// Execute migration SQL
	if _, err := w.pool.Exec(ctx, migrationSQL); err != nil {
		return fmt.Errorf("executing migration: %w", err)
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

	// Start a transaction
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Prepare batch insert for transactions
	batch := &pgx.Batch{}
	for _, txn := range transactions {
		// Set default currency if not specified
		currency := txn.Currency
		if currency == "" {
			currency = "INR"
		}

		// Parse timestamp
		timestamp, err := time.Parse(time.RFC3339, txn.Timestamp)
		if err != nil {
			w.logger.Warn("invalid timestamp format, using current time",
				"timestamp", txn.Timestamp,
				"error", err,
			)
			timestamp = time.Now()
		}

		// Insert transaction (upsert on conflict)
		batch.Queue(`
			INSERT INTO transactions (
				message_id, amount, currency, original_amount, original_currency,
				exchange_rate, timestamp, merchant_info, category, bucket, source, description
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			ON CONFLICT (message_id) DO UPDATE SET
				amount = EXCLUDED.amount,
				currency = EXCLUDED.currency,
				original_amount = EXCLUDED.original_amount,
				original_currency = EXCLUDED.original_currency,
				exchange_rate = EXCLUDED.exchange_rate,
				timestamp = EXCLUDED.timestamp,
				merchant_info = EXCLUDED.merchant_info,
				category = EXCLUDED.category,
				bucket = EXCLUDED.bucket,
				source = EXCLUDED.source,
				description = EXCLUDED.description,
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
			txn.Source,
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
			return fmt.Errorf("inserting transaction %d: %w", i, err)
		}
	}

	// Close batch results before executing more queries on the transaction
	if err := batchResults.Close(); err != nil {
		return fmt.Errorf("closing batch results: %w", err)
	}

	// Second: Insert labels (now safe to use tx.Exec)
	for i, txn := range transactions {
		if len(txn.Labels) > 0 {
			if err := w.insertLabels(ctx, tx, txnIDs[i], txn.Labels); err != nil {
				return fmt.Errorf("inserting labels for transaction %s: %w", txnIDs[i], err)
			}
		}
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// insertLabels inserts labels for a transaction.
func (w *Writer) insertLabels(ctx context.Context, tx pgx.Tx, txnID string, labels []string) error {
	if len(labels) == 0 {
		return nil
	}

	// Build multi-row insert with ON CONFLICT to handle duplicates
	valueStrings := make([]string, 0, len(labels))
	valueArgs := make([]any, 0, len(labels)*2)
	argIndex := 1

	for _, label := range labels {
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d)", argIndex, argIndex+1))
		valueArgs = append(valueArgs, txnID, label)
		argIndex += 2
	}

	query := fmt.Sprintf(`
		INSERT INTO transaction_labels (transaction_id, label)
		VALUES %s
		ON CONFLICT (transaction_id, label) DO NOTHING
	`, strings.Join(valueStrings, ","))

	_, err := tx.Exec(ctx, query, valueArgs...)
	if err != nil {
		return fmt.Errorf("executing label insert: %w", err)
	}

	return nil
}

// Close closes the database connection pool.
func (w *Writer) Close() {
	if w.pool != nil {
		w.pool.Close()
		w.logger.Info("closed PostgreSQL connection pool")
	}
}
