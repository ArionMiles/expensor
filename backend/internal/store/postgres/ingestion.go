package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	apperrors "github.com/ArionMiles/expensor/backend/pkg/errors"
)

type ingestionRepository struct {
	pool   poolBeginner
	logger *slog.Logger
}

const defaultTransactionCurrency = "INR"

func newIngestionRepository(deps repositoryDependencies) *ingestionRepository {
	return &ingestionRepository{
		pool:   deps.pool,
		logger: deps.logger,
	}
}

type poolBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// writeBatch writes a batch of transactions to the database.
// Uses INSERT ON CONFLICT to handle duplicate message_id (upsert logic).
func (w *ingestionRepository) Write(ctx context.Context, batch store.IngestionBatch) error {
	transactions := batch.Transactions
	if len(transactions) == 0 {
		return nil
	}

	// Start a transaction
	tx, err := w.pool.Begin(ctx) // TODO: Can we have a WithTx(...) context manager pattern here?
	if err != nil {
		return apperrors.E("postgres.ingestion.write", apperrors.Internal, "beginning transaction", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Prepare batch insert for transactions
	pgBatch := &pgx.Batch{}
	const conflictClause = "ON CONFLICT (tenant_id, message_id) WHERE tenant_id IS NOT NULL"
	for _, txn := range transactions {
		currency, timestamp := w.normalizeWriteInput(txn)
		pgBatch.Queue(fmt.Sprintf(`
			INSERT INTO transactions (
				tenant_id, message_id, amount, currency, original_amount, original_currency,
				exchange_rate, timestamp, merchant_info, category, bucket, source,
				source_type, source_label, bank, description
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
			%s DO UPDATE SET
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
		`, conflictClause),
			batch.Tenant.ID,
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
	batchResults := tx.SendBatch(ctx, pgBatch)

	// First: Collect all transaction IDs (fully consume batch results)
	txnIDs := make([]string, len(transactions))
	for i := 0; i < len(transactions); i++ {
		if err := batchResults.QueryRow().Scan(&txnIDs[i]); err != nil {
			_ = batchResults.Close()
			return apperrors.E("postgres.ingestion.write", apperrors.Internal, fmt.Sprintf("inserting transaction %d", i), err)
		}
	}

	// Close batch results before executing more queries on the transaction
	if err := batchResults.Close(); err != nil {
		return apperrors.E("postgres.ingestion.write", apperrors.Internal, "closing batch results", err)
	}

	// Second: Insert labels (now safe to use tx.Exec)
	for i, txn := range transactions {
		if len(txn.Labels) > 0 {
			if err := w.insertLabels(ctx, tx, txnIDs[i], txn.Labels); err != nil {
				return apperrors.E("postgres.ingestion.write", apperrors.Internal, fmt.Sprintf("inserting labels for transaction %s", txnIDs[i]), err)
			}
		}
	}

	if err := w.applyMerchantLabels(ctx, tx, txnIDs); err != nil {
		return apperrors.E("postgres.ingestion.write", apperrors.Internal, "auto-applying merchant labels", err)
	}
	if err := w.applyMerchantCategories(ctx, tx, txnIDs); err != nil {
		return apperrors.E("postgres.ingestion.write", apperrors.Internal, "auto-applying merchant categories", err)
	}
	if err := w.applyMutedMerchants(ctx, tx, txnIDs); err != nil {
		return apperrors.E("postgres.ingestion.write", apperrors.Internal, "auto-muting transactions", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return apperrors.E("postgres.ingestion.write", apperrors.Internal, "committing transaction", err)
	}

	return nil
}

func (w *ingestionRepository) normalizeWriteInput(txn *api.TransactionDetails) (string, time.Time) {
	currency := txn.Currency
	if currency == "" {
		currency = defaultTransactionCurrency
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
func (w *ingestionRepository) applyMerchantLabels(ctx context.Context, tx pgx.Tx, txnIDs []string) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO transaction_label_sources (transaction_id, label, source_type, merchant_pattern)
		SELECT t.id, lm.label, 'merchant', lm.merchant_pattern
		FROM transactions t
		JOIN label_merchants lm
		  ON t.merchant_info ILIKE '%' || lm.merchant_pattern || '%'
		 AND lm.tenant_id = t.tenant_id
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
func (w *ingestionRepository) applyMerchantCategories(ctx context.Context, tx pgx.Tx, txnIDs []string) error {
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
			 AND mc.tenant_id = t.tenant_id
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
func (w *ingestionRepository) applyMutedMerchants(ctx context.Context, tx pgx.Tx, txnIDs []string) error {
	_, err := tx.Exec(ctx, `
		UPDATE transactions t
		SET muted = true,
		    muted_by_merchant = true,
		    mute_reason = mm.reason,
		    updated_at = NOW()
		FROM muted_merchants mm
		WHERE t.id = ANY($1)
		  AND mm.tenant_id = t.tenant_id
		  AND t.merchant_info ILIKE '%' || mm.pattern || '%'
	`, txnIDs)
	return err
}

// insertLabels inserts labels for a transaction.
func (w *ingestionRepository) insertLabels(ctx context.Context, tx pgx.Tx, txnID string, labels []string) error {
	if len(labels) == 0 {
		return nil
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO transaction_label_sources (transaction_id, label, source_type, merchant_pattern)
		SELECT $1, unnest($2::text[]), 'manual', ''
		ON CONFLICT (transaction_id, label, source_type, merchant_pattern) DO NOTHING
	`, txnID, labels); err != nil {
		return apperrors.E("postgres.ingestion.labels", apperrors.Internal, "executing label source insert", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO transaction_labels (transaction_id, label)
		SELECT $1, unnest($2::text[])
		ON CONFLICT (transaction_id, label) DO NOTHING
	`, txnID, labels); err != nil {
		return apperrors.E("postgres.ingestion.labels", apperrors.Internal, "executing label insert", err)
	}

	return nil
}
