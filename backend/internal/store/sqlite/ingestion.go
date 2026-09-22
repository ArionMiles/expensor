package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

const defaultTransactionCurrency = "INR"

type preparedTransaction struct {
	transaction    *api.TransactionDetails
	amount         int64
	originalAmount any
	exchangeRate   any
	currency       string
	timestamp      string
}

type ingestionTransactionContext struct {
	tenantID      string
	transactionID string
	merchant      string
	now           string
}

type transactionLabelInput struct {
	transactionID string
	label         string
	sourceType    string
	pattern       string
	now           string
}

// Write atomically persists one tenant-scoped ingestion batch.
func (s *Store) Write(ctx context.Context, batch store.IngestionBatch) error {
	if len(batch.Transactions) == 0 {
		return nil
	}
	prepared := make([]preparedTransaction, len(batch.Transactions))
	for i, transaction := range batch.Transactions {
		if transaction == nil {
			return errors.B.Op("sqlite.ingestion.write").KindInvalidInput().Text("transaction is required").Build()
		}
		value, err := prepareIngestionTransaction(transaction, s.repositories.now())
		if err != nil {
			return err
		}
		prepared[i] = value
	}

	return s.repositories.writeTx(ctx, func(q queryer) error {
		for i := range prepared {
			if err := s.writeIngestionTransaction(ctx, q, batch.Tenant, prepared[i]); err != nil {
				return err
			}
		}
		return nil
	})
}

func prepareIngestionTransaction(transaction *api.TransactionDetails, now time.Time) (preparedTransaction, error) {
	amount, err := amountToScaled(transaction.Amount)
	if err != nil {
		return preparedTransaction{}, err
	}
	var originalAmount any
	if transaction.OriginalAmount != nil {
		originalAmount, err = amountToScaled(*transaction.OriginalAmount)
		if err != nil {
			return preparedTransaction{}, err
		}
	}
	var exchangeRate any
	if transaction.ExchangeRate != nil {
		exchangeRate, err = exchangeRateToScaled(*transaction.ExchangeRate)
		if err != nil {
			return preparedTransaction{}, err
		}
	}
	currency := transaction.Currency
	if currency == "" {
		currency = defaultTransactionCurrency
	}
	timestamp, err := time.Parse(time.RFC3339, transaction.Timestamp)
	if err != nil {
		timestamp = now
	}
	return preparedTransaction{
		transaction: transaction, amount: amount, originalAmount: originalAmount,
		exchangeRate: exchangeRate, currency: currency, timestamp: timeToText(timestamp),
	}, nil
}

func (s *Store) writeIngestionTransaction(ctx context.Context, q queryer, tenant store.Tenant, prepared preparedTransaction) error {
	transaction := prepared.transaction
	now := timeToText(s.repositories.now())
	var id string
	err := q.QueryRowContext(ctx, `INSERT INTO transactions (
		id,tenant_id,message_id,amount,currency,original_amount,original_currency,exchange_rate,
		timestamp,merchant_info,category,bucket,source,source_type,source_label,bank,description,created_at,updated_at
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	ON CONFLICT(tenant_id,message_id) WHERE tenant_id IS NOT NULL DO UPDATE SET
		amount=excluded.amount,currency=excluded.currency,original_amount=excluded.original_amount,
		original_currency=excluded.original_currency,exchange_rate=excluded.exchange_rate,
		timestamp=excluded.timestamp,merchant_info=excluded.merchant_info,source=excluded.source,
		source_type=excluded.source_type,source_label=excluded.source_label,bank=excluded.bank,
		category=COALESCE(NULLIF(transactions.category,''),excluded.category),
		bucket=COALESCE(NULLIF(transactions.bucket,''),excluded.bucket),updated_at=excluded.updated_at
	RETURNING id`, uuid.NewString(), tenant.ID, transaction.MessageID, prepared.amount, prepared.currency,
		prepared.originalAmount, transaction.OriginalCurrency, prepared.exchangeRate, prepared.timestamp,
		transaction.MerchantInfo, transaction.Category, transaction.Bucket, transaction.Source.Display(),
		transaction.Source.Type, transaction.Source.Label, transaction.Source.Bank, transaction.Description,
		now, now).Scan(&id)
	if err != nil {
		return mapSQLiteError("sqlite.ingestion.write_transaction", err)
	}
	for _, label := range transaction.Labels {
		if err := insertTransactionLabel(ctx, q, transactionLabelInput{
			transactionID: id,
			label:         label,
			sourceType:    "manual",
			now:           now,
		}); err != nil {
			return err
		}
	}
	writeContext := ingestionTransactionContext{
		tenantID:      tenant.ID,
		transactionID: id,
		merchant:      transaction.MerchantInfo,
		now:           now,
	}
	if err := s.applyIngestionMerchantLabels(ctx, q, writeContext); err != nil {
		return err
	}
	if err := s.applyIngestionMerchantCategory(ctx, q, writeContext); err != nil {
		return err
	}
	return s.applyIngestionMute(ctx, q, writeContext)
}

func insertTransactionLabel(ctx context.Context, q queryer, input transactionLabelInput) error {
	if _, err := q.ExecContext(ctx, `INSERT INTO transaction_label_sources
		(id,transaction_id,label,source_type,merchant_pattern,created_at) VALUES (?,?,?,?,?,?)
		ON CONFLICT(transaction_id,label,source_type,merchant_pattern) DO NOTHING`,
		uuid.NewString(), input.transactionID, input.label, input.sourceType, input.pattern, input.now); err != nil {
		return mapSQLiteError("sqlite.ingestion.insert_label_source", err)
	}
	_, err := q.ExecContext(ctx, `INSERT INTO transaction_labels (id,transaction_id,label,created_at)
		VALUES (?,?,?,?) ON CONFLICT(transaction_id,label) DO NOTHING`,
		uuid.NewString(), input.transactionID, input.label, input.now)
	return mapSQLiteError("sqlite.ingestion.insert_label", err)
}

func (s *Store) applyIngestionMerchantLabels(ctx context.Context, q queryer, input ingestionTransactionContext) error {
	rows, err := q.QueryContext(ctx, `SELECT label,merchant_pattern FROM label_merchants
		WHERE tenant_id=? AND instr(expensor_casefold(?),expensor_casefold(merchant_pattern))>0`,
		input.tenantID, input.merchant)
	if err != nil {
		return mapSQLiteError("sqlite.ingestion.find_merchant_labels", err)
	}
	defer rows.Close()
	type mapping struct{ label, pattern string }
	mappings := []mapping{}
	for rows.Next() {
		var value mapping
		if err := rows.Scan(&value.label, &value.pattern); err != nil {
			return mapSQLiteError("sqlite.ingestion.scan_merchant_label", err)
		}
		mappings = append(mappings, value)
	}
	if err := rows.Err(); err != nil {
		return mapSQLiteError("sqlite.ingestion.iterate_merchant_labels", err)
	}
	for _, value := range mappings {
		if err := insertTransactionLabel(ctx, q, transactionLabelInput{
			transactionID: input.transactionID,
			label:         value.label,
			sourceType:    "merchant",
			pattern:       value.pattern,
			now:           input.now,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) applyIngestionMerchantCategory(ctx context.Context, q queryer, input ingestionTransactionContext) error {
	var category, bucket *string
	err := q.QueryRowContext(ctx, `SELECT COALESCE(m.category,mc.category),COALESCE(m.bucket,mc.bucket)
		FROM merchant_categories mc LEFT JOIN mcc_codes m ON m.code=mc.mcc_code
		WHERE mc.tenant_id=? AND instr(expensor_casefold(?),expensor_casefold(mc.fragment))>0
		AND (COALESCE(m.category,mc.category) IS NOT NULL OR COALESCE(m.bucket,mc.bucket) IS NOT NULL)
		ORDER BY length(mc.fragment) DESC,mc.fragment LIMIT 1`, input.tenantID, input.merchant).Scan(&category, &bucket)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return mapSQLiteError("sqlite.ingestion.find_merchant_category", err)
	}
	_, err = q.ExecContext(ctx, `UPDATE transactions SET
		category=CASE WHEN COALESCE(category,'')='' THEN ? ELSE category END,
		bucket=CASE WHEN COALESCE(bucket,'')='' THEN ? ELSE bucket END,updated_at=? WHERE id=?`,
		category, bucket, input.now, input.transactionID)
	return mapSQLiteError("sqlite.ingestion.apply_merchant_category", err)
}

func (s *Store) applyIngestionMute(ctx context.Context, q queryer, input ingestionTransactionContext) error {
	var reason sql.NullString
	err := q.QueryRowContext(ctx, `SELECT reason FROM muted_merchants WHERE tenant_id=?
		AND instr(expensor_casefold(?),expensor_casefold(pattern))>0 ORDER BY length(pattern) DESC LIMIT 1`,
		input.tenantID, input.merchant).Scan(&reason)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return mapSQLiteError("sqlite.ingestion.find_muted_merchant", err)
	}
	_, err = q.ExecContext(
		ctx,
		`UPDATE transactions SET muted=1,muted_by_merchant=1,mute_reason=?,updated_at=? WHERE id=?`,
		reason.String,
		input.now,
		input.transactionID,
	)
	return mapSQLiteError("sqlite.ingestion.apply_muted_merchant", err)
}
