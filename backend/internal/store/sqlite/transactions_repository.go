package sqlite

import (
	"context"
	"database/sql"
	"math"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

const transactionColumns = `
	id,message_id,amount,currency,original_amount,original_currency,exchange_rate,
	timestamp,merchant_info,coalesce(category,''),coalesce(bucket,''),source,
	source_type,source_label,bank,coalesce(description,''),muted,muted_by_merchant,
	coalesce(mute_reason,''),created_at,updated_at`

type storedTransaction struct {
	transaction store.Transaction
	amount      int64
}

func (s *Store) ListTransactions(ctx context.Context, tenant store.Tenant, filter store.ListFilter) ([]store.Transaction, store.TransactionListResult, error) {
	return s.queryTransactions(ctx, tenant, "", filter)
}

func (s *Store) SearchTransactions(
	ctx context.Context,
	tenant store.Tenant,
	query string,
	filter store.ListFilter,
) ([]store.Transaction, store.TransactionListResult, error) {
	return s.queryTransactions(ctx, tenant, strings.TrimSpace(query), filter)
}

func (s *Store) queryTransactions(
	ctx context.Context,
	tenant store.Tenant,
	search string,
	filter store.ListFilter,
) ([]store.Transaction, store.TransactionListResult, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 20
	}
	if filter.Page-1 > math.MaxInt/filter.PageSize {
		return nil, store.TransactionListResult{}, errors.B.Op("store.transactions.list").KindInvalidInput().Text("pagination offset overflow").Build()
	}
	records, err := s.loadStoredTransactions(ctx, tenant)
	if err != nil {
		return nil, store.TransactionListResult{}, err
	}
	filtered := make([]storedTransaction, 0, len(records))
	var total scaledAmountTotal
	for _, record := range records {
		if matchesListFilter(record.transaction, filter) && matchesSearch(record.transaction, search) {
			filtered = append(filtered, record)
			total.add(record.amount)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].transaction.Timestamp.Equal(filtered[j].transaction.Timestamp) {
			if strings.EqualFold(filter.SortDir, "asc") {
				return filtered[i].transaction.ID < filtered[j].transaction.ID
			}
			return filtered[i].transaction.ID > filtered[j].transaction.ID
		}
		if strings.EqualFold(filter.SortDir, "asc") {
			return filtered[i].transaction.Timestamp.Before(filtered[j].transaction.Timestamp)
		}
		return filtered[i].transaction.Timestamp.After(filtered[j].transaction.Timestamp)
	})
	result := store.TransactionListResult{Total: len(filtered), TotalAmount: total.float64()}
	offset := (filter.Page - 1) * filter.PageSize
	if offset >= len(filtered) {
		return []store.Transaction{}, result, nil
	}
	end := min(offset+filter.PageSize, len(filtered))
	transactions := make([]store.Transaction, end-offset)
	for i := offset; i < end; i++ {
		transactions[i-offset] = filtered[i].transaction
	}
	return transactions, result, nil
}

func (s *Store) loadStoredTransactions(ctx context.Context, tenant store.Tenant) ([]storedTransaction, error) {
	rows, err := s.repositories.query.QueryContext(ctx, `SELECT `+transactionColumns+` FROM transactions WHERE tenant_id=?`, tenant.ID)
	if err != nil {
		return nil, mapSQLiteError("sqlite.transactions.list", err)
	}
	defer rows.Close()
	records := []storedTransaction{}
	for rows.Next() {
		record, err := scanStoredTransaction(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, mapSQLiteError("sqlite.transactions.list", err)
	}
	if err := s.loadTransactionLabels(ctx, tenant, records); err != nil {
		return nil, err
	}
	return records, nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanStoredTransaction(row rowScanner) (storedTransaction, error) {
	var record storedTransaction
	var originalAmount, exchangeRate sql.NullInt64
	var originalCurrency sql.NullString
	var timestamp, createdAt, updatedAt string
	var muted, mutedByMerchant int
	t := &record.transaction
	var source string
	if err := row.Scan(&t.ID, &t.MessageID, &record.amount, &t.Currency, &originalAmount, &originalCurrency, &exchangeRate,
		&timestamp, &t.MerchantInfo, &t.Category, &t.Bucket, &source, &t.Source.Type, &t.Source.Label, &t.Source.Bank,
		&t.Description, &muted, &mutedByMerchant, &t.MuteReason, &createdAt, &updatedAt); err != nil {
		return storedTransaction{}, mapSQLiteError("sqlite.transactions.scan", err)
	}
	t.Amount = amountFromScaled(record.amount)
	if originalAmount.Valid {
		value := amountFromScaled(originalAmount.Int64)
		t.OriginalAmount = &value
	}
	if originalCurrency.Valid {
		value := originalCurrency.String
		t.OriginalCurrency = &value
	}
	if exchangeRate.Valid {
		value := exchangeRateFromScaled(exchangeRate.Int64)
		t.ExchangeRate = &value
	}
	var err error
	if t.Timestamp, err = timeFromText(timestamp); err != nil {
		return storedTransaction{}, err
	}
	if t.CreatedAt, err = timeFromText(createdAt); err != nil {
		return storedTransaction{}, err
	}
	if t.UpdatedAt, err = timeFromText(updatedAt); err != nil {
		return storedTransaction{}, err
	}
	t.Muted = muted != 0
	t.MutedByMerchant = mutedByMerchant != 0
	t.Labels = []string{}
	return record, nil
}

func (s *Store) loadTransactionLabels(ctx context.Context, tenant store.Tenant, records []storedTransaction) error {
	if len(records) == 0 {
		return nil
	}
	byID := make(map[string]int, len(records))
	for i := range records {
		byID[records[i].transaction.ID] = i
	}
	rows, err := s.repositories.query.QueryContext(ctx, `
		SELECT tl.transaction_id,tl.label FROM transaction_labels tl
		JOIN transactions t ON t.id=tl.transaction_id WHERE t.tenant_id=? ORDER BY tl.label`, tenant.ID)
	if err != nil {
		return mapSQLiteError("sqlite.transactions.load_labels", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, label string
		if err := rows.Scan(&id, &label); err != nil {
			return mapSQLiteError("sqlite.transactions.load_labels", err)
		}
		if index, ok := byID[id]; ok {
			records[index].transaction.Labels = append(records[index].transaction.Labels, label)
		}
	}
	return mapSQLiteError("sqlite.transactions.load_labels", rows.Err())
}

func (s *Store) GetTransaction(ctx context.Context, tenant store.Tenant, id string) (*store.Transaction, error) {
	row := s.repositories.query.QueryRowContext(ctx, `SELECT `+transactionColumns+` FROM transactions WHERE id=? AND tenant_id=?`, id, tenant.ID)
	record, err := scanStoredTransaction(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.WhatKind(err) == errors.NotFound {
			return nil, transactionNotFound("get")
		}
		return nil, err
	}
	labels, err := s.labelsForTransaction(ctx, tenant, id)
	if err != nil {
		return nil, err
	}
	record.transaction.Labels = labels
	return &record.transaction, nil
}

func (s *Store) labelsForTransaction(ctx context.Context, tenant store.Tenant, id string) ([]string, error) {
	rows, err := s.repositories.query.QueryContext(ctx, `SELECT tl.label FROM transaction_labels tl
		JOIN transactions t ON t.id=tl.transaction_id
		WHERE t.id=? AND t.tenant_id=? ORDER BY tl.label`, id, tenant.ID)
	if err != nil {
		return nil, mapSQLiteError("sqlite.transactions.labels", err)
	}
	defer rows.Close()
	labels := []string{}
	for rows.Next() {
		var label string
		if err := rows.Scan(&label); err != nil {
			return nil, mapSQLiteError("sqlite.transactions.labels", err)
		}
		labels = append(labels, label)
	}
	return labels, mapSQLiteError("sqlite.transactions.labels", rows.Err())
}

func transactionNotFound(operation string) error {
	return errors.B.Op("store.transactions." + operation).KindNotFound().UserMsg("transaction not found").Build()
}

func mutedMerchantNotFound(operation string) error {
	return errors.B.Op("store.transactions." + operation).KindNotFound().UserMsg("muted merchant not found").Build()
}

func (s *Store) UpdateDescription(ctx context.Context, tenant store.Tenant, id, description string) error {
	return s.updateTransactionFields(ctx, tenant, id, transactionFieldUpdate{
		sets:      []string{"description=?"},
		args:      []any{description},
		operation: "update_description",
	})
}

func (s *Store) UpdateTransaction(ctx context.Context, tenant store.Tenant, id string, update store.TransactionUpdate) error {
	sets := []string{}
	args := []any{}
	if update.Description != nil {
		sets, args = append(sets, "description=?"), append(args, *update.Description)
	}
	if update.Category != nil {
		sets, args = append(sets, "category=?"), append(args, *update.Category)
	}
	if update.Bucket != nil {
		sets, args = append(sets, "bucket=?"), append(args, *update.Bucket)
	}
	if len(sets) == 0 {
		return nil
	}
	return s.updateTransactionFields(ctx, tenant, id, transactionFieldUpdate{
		sets:      sets,
		args:      args,
		operation: "update",
	})
}

type transactionFieldUpdate struct {
	sets      []string
	args      []any
	operation string
}

func (s *Store) updateTransactionFields(
	ctx context.Context,
	tenant store.Tenant,
	id string,
	update transactionFieldUpdate,
) error {
	update.args = append(update.args, timeToText(s.repositories.now()), id, tenant.ID)
	result, err := s.repositories.query.ExecContext(ctx,
		`UPDATE transactions SET `+strings.Join(update.sets, ",")+`,updated_at=? WHERE id=? AND tenant_id=?`, update.args...)
	if err != nil {
		return mapSQLiteError("sqlite.transactions."+update.operation, err)
	}
	return requireTransactionRows(result, update.operation)
}

func requireTransactionRows(result sql.Result, operation string) error {
	count, err := result.RowsAffected()
	if err != nil {
		return mapSQLiteError("sqlite.transactions."+operation, err)
	}
	if count == 0 {
		return transactionNotFound(operation)
	}
	return nil
}

func (s *Store) AddLabel(ctx context.Context, tenant store.Tenant, transactionID, label string) error {
	return s.AddLabels(ctx, tenant, transactionID, []string{label})
}

func (s *Store) AddLabels(ctx context.Context, tenant store.Tenant, transactionID string, labels []string) error {
	if len(labels) == 0 {
		return nil
	}
	return s.repositories.writeTx(ctx, func(tx queryer) error {
		if err := ensureSQLiteTransaction(ctx, tx, tenant, transactionID, "add_labels"); err != nil {
			return err
		}
		for _, label := range labels {
			if _, err := tx.ExecContext(ctx, `INSERT INTO transaction_label_sources
				(id,transaction_id,label,source_type,merchant_pattern) VALUES (?,?,?,'manual','')
				ON CONFLICT (transaction_id,label,source_type,merchant_pattern) DO NOTHING`,
				uuid.NewString(), transactionID, label); err != nil {
				return mapSQLiteError("sqlite.transactions.add_labels", err)
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO transaction_labels (id,transaction_id,label)
				VALUES (?,?,?) ON CONFLICT (transaction_id,label) DO NOTHING`,
				uuid.NewString(), transactionID, label); err != nil {
				return mapSQLiteError("sqlite.transactions.add_labels", err)
			}
		}
		return nil
	})
}

func ensureSQLiteTransaction(ctx context.Context, tx queryer, tenant store.Tenant, id, operation string) error {
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM transactions WHERE id=? AND tenant_id=?)`, id, tenant.ID).Scan(&exists); err != nil {
		return mapSQLiteError("sqlite.transactions."+operation, err)
	}
	if exists == 0 {
		return transactionNotFound(operation)
	}
	return nil
}

func (s *Store) RemoveLabel(ctx context.Context, tenant store.Tenant, transactionID, label string) error {
	return s.repositories.writeTx(ctx, func(tx queryer) error {
		if err := ensureSQLiteTransaction(ctx, tx, tenant, transactionID, "remove_label"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM transaction_label_sources WHERE transaction_id=? AND label=?`, transactionID, label); err != nil {
			return mapSQLiteError("sqlite.transactions.remove_label", err)
		}
		result, err := tx.ExecContext(ctx, `DELETE FROM transaction_labels WHERE transaction_id=? AND label=?`, transactionID, label)
		if err != nil {
			return mapSQLiteError("sqlite.transactions.remove_label", err)
		}
		count, err := result.RowsAffected()
		if err != nil {
			return mapSQLiteError("sqlite.transactions.remove_label", err)
		}
		if count == 0 {
			return errors.B.Op("store.transactions.remove_label").KindNotFound().UserMsg("label not found on transaction").Build()
		}
		return nil
	})
}

func (s *Store) GetFacets(ctx context.Context, tenant store.Tenant) (*store.Facets, error) {
	records, err := s.loadStoredTransactions(ctx, tenant)
	if err != nil {
		return nil, err
	}
	facets := &store.Facets{
		Sources:        []string{},
		SourceTypes:    []string{},
		Banks:          []string{},
		Categories:     []string{},
		CategoryCounts: map[string]int{},
		Currencies:     []string{},
		Merchants:      []string{},
		Labels:         []string{},
		LabelCounts:    map[string]int{},
		Buckets:        []string{},
		BucketCounts:   map[string]int{},
	}
	sets := newFacetSets()
	for _, record := range records {
		t := record.transaction
		addFacet(sets.sources, t.Source.Display())
		addFacet(sets.sourceTypes, t.Source.Type)
		addFacet(sets.banks, t.Source.Bank)
		addFacet(sets.categories, t.Category)
		addFacet(sets.currencies, t.Currency)
		addFacet(sets.merchants, t.MerchantInfo)
		addFacet(sets.buckets, t.Bucket)
		if t.Category != "" {
			facets.CategoryCounts[t.Category]++
		}
		if t.Bucket != "" {
			facets.BucketCounts[t.Bucket]++
		}
		for _, label := range t.Labels {
			addFacet(sets.labels, label)
			facets.LabelCounts[label]++
		}
	}
	facets.Sources = sortedFacet(sets.sources)
	facets.SourceTypes = sortedFacet(sets.sourceTypes)
	facets.Banks = sortedFacet(sets.banks)
	facets.Categories = sortedFacet(sets.categories)
	facets.Currencies = sortedFacet(sets.currencies)
	facets.Merchants = sortedFacet(sets.merchants)
	facets.Labels = sortedFacet(sets.labels)
	facets.Buckets = sortedFacet(sets.buckets)
	return facets, nil
}

type facetSets struct {
	sources     map[string]bool
	sourceTypes map[string]bool
	banks       map[string]bool
	categories  map[string]bool
	currencies  map[string]bool
	merchants   map[string]bool
	labels      map[string]bool
	buckets     map[string]bool
}

func newFacetSets() facetSets {
	return facetSets{
		sources:     map[string]bool{},
		sourceTypes: map[string]bool{},
		banks:       map[string]bool{},
		categories:  map[string]bool{},
		currencies:  map[string]bool{},
		merchants:   map[string]bool{},
		labels:      map[string]bool{},
		buckets:     map[string]bool{},
	}
}

func addFacet(values map[string]bool, value string) {
	if value != "" {
		values[value] = true
	}
}

func sortedFacet(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func (s *Store) MuteTransaction(ctx context.Context, tenant store.Tenant, id string, muted bool, reason string) error {
	mutedValue, merchantValue, reasonValue := 0, 0, any(nil)
	if muted {
		mutedValue = 1
		if reason != "" {
			reasonValue = reason
		}
	}
	result, err := s.repositories.query.ExecContext(ctx, `UPDATE transactions
		SET muted=?,muted_by_merchant=?,mute_reason=?,updated_at=? WHERE id=? AND tenant_id=?`,
		mutedValue, merchantValue, reasonValue, timeToText(s.repositories.now()), id, tenant.ID)
	if err != nil {
		return mapSQLiteError("sqlite.transactions.mute", err)
	}
	return requireTransactionRows(result, "mute")
}

func (s *Store) UpdateMuteReason(ctx context.Context, tenant store.Tenant, id, reason string) error {
	var value any
	if reason != "" {
		value = reason
	}
	result, err := s.repositories.query.ExecContext(ctx, `UPDATE transactions
		SET mute_reason=?,updated_at=? WHERE id=? AND tenant_id=? AND muted=1`,
		value, timeToText(s.repositories.now()), id, tenant.ID)
	if err != nil {
		return mapSQLiteError("sqlite.transactions.update_mute_reason", err)
	}
	return requireTransactionRows(result, "update_mute_reason")
}

func (s *Store) UpdateMerchantReason(ctx context.Context, tenant store.Tenant, id, reason string) error {
	var value any
	if reason != "" {
		value = reason
	}
	result, err := s.repositories.query.ExecContext(ctx, `UPDATE muted_merchants SET reason=? WHERE id=? AND tenant_id=?`, value, id, tenant.ID)
	if err != nil {
		return mapSQLiteError("sqlite.transactions.update_merchant_reason", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return mapSQLiteError("sqlite.transactions.update_merchant_reason", err)
	}
	if count == 0 {
		return mutedMerchantNotFound("update_merchant_reason")
	}
	return nil
}

func (s *Store) MuteByMerchant(ctx context.Context, tenant store.Tenant, pattern, reason string) error {
	return s.repositories.writeTx(ctx, func(tx queryer) error {
		var reasonValue any
		if reason != "" {
			reasonValue = reason
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO muted_merchants (id,tenant_id,pattern,reason)
			VALUES (?,?,?,?) ON CONFLICT (tenant_id,pattern) WHERE tenant_id IS NOT NULL
			DO UPDATE SET reason=excluded.reason`, uuid.NewString(), tenant.ID, pattern, reasonValue); err != nil {
			return mapSQLiteError("sqlite.transactions.mute_by_merchant", err)
		}
		_, err := tx.ExecContext(ctx, `UPDATE transactions SET muted=1,muted_by_merchant=1,mute_reason=?,updated_at=?
			WHERE tenant_id=? AND instr(expensor_casefold(merchant_info),expensor_casefold(?))>0`,
			reasonValue, timeToText(s.repositories.now()), tenant.ID, pattern)
		return mapSQLiteError("sqlite.transactions.mute_by_merchant", err)
	})
}

func (s *Store) ListMutedMerchants(ctx context.Context, tenant store.Tenant) ([]store.MutedMerchant, error) {
	rows, err := s.repositories.query.QueryContext(ctx, `SELECT id,pattern,coalesce(reason,''),created_at
		FROM muted_merchants WHERE tenant_id=? ORDER BY created_at DESC`, tenant.ID)
	if err != nil {
		return nil, mapSQLiteError("sqlite.transactions.list_muted_merchants", err)
	}
	defer rows.Close()
	result := []store.MutedMerchant{}
	for rows.Next() {
		var item store.MutedMerchant
		var created string
		if err := rows.Scan(&item.ID, &item.Pattern, &item.Reason, &created); err != nil {
			return nil, mapSQLiteError("sqlite.transactions.list_muted_merchants", err)
		}
		item.CreatedAt, err = timeFromText(created)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, mapSQLiteError("sqlite.transactions.list_muted_merchants", rows.Err())
}

func (s *Store) GetMutedMerchantsWithCount(ctx context.Context, tenant store.Tenant) ([]store.MutedMerchantWithCount, error) {
	rows, err := s.repositories.query.QueryContext(ctx, `
		SELECT mm.id,mm.pattern,coalesce(mm.reason,''),mm.created_at,count(t.id)
		FROM muted_merchants mm
		LEFT JOIN transactions t ON t.tenant_id=mm.tenant_id AND t.muted_by_merchant=1
			AND instr(expensor_casefold(t.merchant_info),expensor_casefold(mm.pattern))>0
		WHERE mm.tenant_id=?
		GROUP BY mm.id,mm.pattern,mm.reason,mm.created_at
		ORDER BY mm.created_at DESC`, tenant.ID)
	if err != nil {
		return nil, mapSQLiteError("sqlite.transactions.muted_counts", err)
	}
	defer rows.Close()
	result := []store.MutedMerchantWithCount{}
	for rows.Next() {
		var item store.MutedMerchantWithCount
		var created string
		if err := rows.Scan(&item.ID, &item.Pattern, &item.Reason, &created, &item.MutedCount); err != nil {
			return nil, mapSQLiteError("sqlite.transactions.muted_counts", err)
		}
		item.CreatedAt, err = timeFromText(created)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, mapSQLiteError("sqlite.transactions.muted_counts", rows.Err())
}

func (s *Store) DeleteMutedMerchant(ctx context.Context, tenant store.Tenant, id string) error {
	result, err := s.repositories.query.ExecContext(ctx, `DELETE FROM muted_merchants WHERE id=? AND tenant_id=?`, id, tenant.ID)
	if err != nil {
		return mapSQLiteError("sqlite.transactions.delete_muted_merchant", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return mapSQLiteError("sqlite.transactions.delete_muted_merchant", err)
	}
	if count == 0 {
		return mutedMerchantNotFound("delete_muted_merchant")
	}
	return nil
}

func (s *Store) UnmuteByPattern(ctx context.Context, tenant store.Tenant, pattern string) error {
	_, err := s.repositories.query.ExecContext(ctx, `UPDATE transactions
		SET muted=0,muted_by_merchant=0,mute_reason=NULL,updated_at=?
		WHERE tenant_id=? AND instr(expensor_casefold(merchant_info),expensor_casefold(?))>0`,
		timeToText(s.repositories.now()), tenant.ID, pattern)
	return mapSQLiteError("sqlite.transactions.unmute_by_pattern", err)
}

func (s *Store) DeleteMutedMerchantAndUnmute(ctx context.Context, tenant store.Tenant, id string) error {
	return s.repositories.writeTx(ctx, func(tx queryer) error {
		var pattern string
		if err := tx.QueryRowContext(ctx, `SELECT pattern FROM muted_merchants WHERE id=? AND tenant_id=?`, id, tenant.ID).Scan(&pattern); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return mutedMerchantNotFound("delete_muted_merchant_and_unmute")
			}
			return mapSQLiteError("sqlite.transactions.delete_muted_merchant_and_unmute", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM muted_merchants WHERE id=? AND tenant_id=?`, id, tenant.ID); err != nil {
			return mapSQLiteError("sqlite.transactions.delete_muted_merchant_and_unmute", err)
		}
		_, err := tx.ExecContext(ctx, `UPDATE transactions
			SET muted=0,muted_by_merchant=0,mute_reason=NULL,updated_at=?
			WHERE tenant_id=? AND instr(expensor_casefold(merchant_info),expensor_casefold(?))>0`,
			timeToText(s.repositories.now()), tenant.ID, pattern)
		return mapSQLiteError("sqlite.transactions.delete_muted_merchant_and_unmute", err)
	})
}

func (s *Store) GetMutedMerchantPatterns(ctx context.Context, tenant store.Tenant) ([]string, error) {
	rows, err := s.repositories.query.QueryContext(ctx, `SELECT pattern FROM muted_merchants WHERE tenant_id=?`, tenant.ID)
	if err != nil {
		return nil, mapSQLiteError("sqlite.transactions.muted_patterns", err)
	}
	defer rows.Close()
	result := []string{}
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, mapSQLiteError("sqlite.transactions.muted_patterns", err)
		}
		result = append(result, value)
	}
	return result, mapSQLiteError("sqlite.transactions.muted_patterns", rows.Err())
}

var _ = api.Source{}
