package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransactionsRepository interface {
	ListTransactions(ctx context.Context, f ListFilter) ([]Transaction, TransactionListResult, error)
	GetTransaction(ctx context.Context, id string) (*Transaction, error)
	UpdateDescription(ctx context.Context, id, description string) error
	AddLabel(ctx context.Context, transactionID, label string) error
	AddLabels(ctx context.Context, transactionID string, labels []string) error
	RemoveLabel(ctx context.Context, transactionID, label string) error
	SearchTransactions(ctx context.Context, query string, f ListFilter) ([]Transaction, TransactionListResult, error)
	GetFacets(ctx context.Context) (*Facets, error)
	UpdateTransaction(ctx context.Context, id string, u TransactionUpdate) error
	MuteTransaction(ctx context.Context, id string, muted bool, reason string) error
	UpdateMuteReason(ctx context.Context, id, reason string) error
	UpdateMerchantReason(ctx context.Context, id, reason string) error
	MuteByMerchant(ctx context.Context, pattern, reason string) error
	ListMutedMerchants(ctx context.Context) ([]MutedMerchant, error)
	GetMutedMerchantsWithCount(ctx context.Context) ([]MutedMerchantWithCount, error)
	DeleteMutedMerchant(ctx context.Context, id string) error
	UnmuteByPattern(ctx context.Context, pattern string) error
	DeleteMutedMerchantAndUnmute(ctx context.Context, id string) error
	GetMutedMerchantPatterns(ctx context.Context) ([]string, error)
}

type pgTransactionsRepository struct {
	pool *pgxpool.Pool
}

func NewTransactionsRepository(deps repositoryDependencies) TransactionsRepository {
	return &pgTransactionsRepository{
		pool: deps.pool,
	}
}

func (r *pgTransactionsRepository) ListTransactions(ctx context.Context, f ListFilter) ([]Transaction, TransactionListResult, error) {
	return r.listTransactionsQuery(ctx, f)
}

func (r *pgTransactionsRepository) listTransactionsQuery(ctx context.Context, f ListFilter) ([]Transaction, TransactionListResult, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = 20
	}

	where, args := buildListWhere(f)
	offset := (f.Page - 1) * f.PageSize
	join := joinLabel(f.Label)

	result, err := r.queryTransactionTotals(ctx, join, where, args)
	if err != nil {
		return nil, TransactionListResult{}, fmt.Errorf("counting transactions: %w", err)
	}

	args = append(args, f.PageSize, offset)
	limitArg := len(args) - 1
	offsetArg := len(args)

	orderClause := "t.timestamp DESC"
	if strings.ToLower(f.SortDir) == "asc" {
		orderClause = "t.timestamp ASC"
	}

	dataSQL := fmt.Sprintf(`
		SELECT DISTINCT t.id, t.message_id, t.amount, t.currency,
		       t.original_amount, t.original_currency, t.exchange_rate,
		       t.timestamp, t.merchant_info,
		       COALESCE(t.category, ''), COALESCE(t.bucket, ''),
		       t.source, COALESCE(t.source_type, ''), COALESCE(t.source_label, ''), COALESCE(t.bank, ''),
		       COALESCE(t.description, ''), t.muted, t.muted_by_merchant, COALESCE(t.mute_reason,''), t.created_at, t.updated_at
		FROM transactions t%s%s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, join, where, orderClause, limitArg, offsetArg)

	rows, err := r.pool.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, TransactionListResult{}, fmt.Errorf("listing transactions: %w", err)
	}
	defer rows.Close()

	txns, err := scanTransactions(rows)
	if err != nil {
		return nil, TransactionListResult{}, err
	}

	if err := r.loadLabels(ctx, txns); err != nil {
		return nil, TransactionListResult{}, err
	}

	return txns, result, nil
}

func (r *pgTransactionsRepository) GetTransaction(ctx context.Context, id string) (*Transaction, error) {
	return r.getTransactionQuery(ctx, id)
}

func (r *pgTransactionsRepository) getTransactionQuery(ctx context.Context, id string) (*Transaction, error) {
	const q = `
		SELECT t.id, t.message_id, t.amount, t.currency,
		       t.original_amount, t.original_currency, t.exchange_rate,
		       t.timestamp, t.merchant_info,
		       COALESCE(t.category, ''), COALESCE(t.bucket, ''),
		       t.source, COALESCE(t.source_type, ''), COALESCE(t.source_label, ''), COALESCE(t.bank, ''),
		       COALESCE(t.description, ''), t.muted, t.muted_by_merchant, COALESCE(t.mute_reason,''), t.created_at, t.updated_at
		FROM transactions t
		WHERE t.id = $1
	`
	rows, err := r.pool.Query(ctx, q, id)
	if err != nil {
		return nil, fmt.Errorf("fetching transaction: %w", err)
	}
	defer rows.Close()

	txns, err := scanTransactions(rows)
	if err != nil {
		return nil, err
	}
	if len(txns) == 0 {
		return nil, ErrNotFound
	}

	if err := r.loadLabels(ctx, txns); err != nil {
		return nil, err
	}
	return &txns[0], nil
}

func (r *pgTransactionsRepository) UpdateDescription(ctx context.Context, id, description string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE transactions SET description = $1 WHERE id = $2`,
		description, id,
	)
	if err != nil {
		return fmt.Errorf("updating description: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgTransactionsRepository) AddLabel(ctx context.Context, transactionID, label string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning add-label transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`INSERT INTO transaction_label_sources (transaction_id, label, source_type, merchant_pattern)
			 VALUES ($1, $2, 'manual', '')
			 ON CONFLICT (transaction_id, label, source_type, merchant_pattern) DO NOTHING`,
		transactionID, label,
	); err != nil {
		return fmt.Errorf("adding label source: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO transaction_labels (transaction_id, label) VALUES ($1, $2)
			 ON CONFLICT (transaction_id, label) DO NOTHING`,
		transactionID, label,
	); err != nil {
		return fmt.Errorf("adding label: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing add-label transaction: %w", err)
	}
	return nil
}

func (r *pgTransactionsRepository) AddLabels(ctx context.Context, transactionID string, labels []string) error {
	if len(labels) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning add-labels transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`INSERT INTO transaction_label_sources (transaction_id, label, source_type, merchant_pattern)
			 SELECT $1, unnest($2::text[]), 'manual', ''
			 ON CONFLICT (transaction_id, label, source_type, merchant_pattern) DO NOTHING`,
		transactionID, labels,
	); err != nil {
		return fmt.Errorf("adding label sources: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO transaction_labels (transaction_id, label)
			 SELECT $1, unnest($2::text[])
			 ON CONFLICT (transaction_id, label) DO NOTHING`,
		transactionID, labels,
	); err != nil {
		return fmt.Errorf("adding labels: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing add-labels transaction: %w", err)
	}
	return nil
}

func (r *pgTransactionsRepository) RemoveLabel(ctx context.Context, transactionID, label string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning remove-label transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`DELETE FROM transaction_label_sources WHERE transaction_id = $1 AND label = $2`,
		transactionID, label,
	); err != nil {
		return fmt.Errorf("removing label sources: %w", err)
	}

	tag, err := tx.Exec(ctx,
		`DELETE FROM transaction_labels WHERE transaction_id = $1 AND label = $2`,
		transactionID, label,
	)
	if err != nil {
		return fmt.Errorf("removing label: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing remove-label transaction: %w", err)
	}
	return nil
}

func (r *pgTransactionsRepository) SearchTransactions(
	ctx context.Context,
	query string,
	f ListFilter,
) ([]Transaction, TransactionListResult, error) {
	return r.searchTransactionsQuery(ctx, query, f)
}

func (r *pgTransactionsRepository) searchTransactionsQuery(
	ctx context.Context,
	query string,
	f ListFilter,
) ([]Transaction, TransactionListResult, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = 20
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return r.listTransactionsQuery(ctx, f)
	}

	where, args := buildListWhere(f)
	searchCond := buildSearchCondition(query, &args)
	fullWhere := combineWhere(searchCond, where)
	offset := (f.Page - 1) * f.PageSize

	join := joinLabel(f.Label)
	result, err := r.queryTransactionTotals(ctx, join, fullWhere, args)
	if err != nil {
		return nil, TransactionListResult{}, fmt.Errorf("counting search results: %w", err)
	}

	args = append(args, f.PageSize, offset)
	limitArg := len(args) - 1
	offsetArg := len(args)

	dataSQL := fmt.Sprintf(`
		SELECT DISTINCT t.id, t.message_id, t.amount, t.currency,
		       t.original_amount, t.original_currency, t.exchange_rate,
		       t.timestamp, t.merchant_info,
		       COALESCE(t.category, ''), COALESCE(t.bucket, ''),
		       t.source, COALESCE(t.source_type, ''), COALESCE(t.source_label, ''), COALESCE(t.bank, ''),
		       COALESCE(t.description, ''), t.muted, t.muted_by_merchant, COALESCE(t.mute_reason,''), t.created_at, t.updated_at
		FROM transactions t%s%s
		ORDER BY t.timestamp DESC
		LIMIT $%d OFFSET $%d
	`, join, fullWhere, limitArg, offsetArg)

	rows, err := r.pool.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, TransactionListResult{}, fmt.Errorf("searching transactions: %w", err)
	}
	defer rows.Close()

	txns, err := scanTransactions(rows)
	if err != nil {
		return nil, TransactionListResult{}, err
	}
	if err := r.loadLabels(ctx, txns); err != nil {
		return nil, TransactionListResult{}, err
	}
	return txns, result, nil
}

func (r *pgTransactionsRepository) GetFacets(ctx context.Context) (*Facets, error) {
	return r.getFacetsQuery(ctx)
}

func (r *pgTransactionsRepository) getFacetsQuery(ctx context.Context) (*Facets, error) {
	var f Facets
	if err := r.loadFacetValues(ctx, &f); err != nil {
		return nil, err
	}

	f.LabelCounts = map[string]int{}
	f.CategoryCounts = map[string]int{}
	f.BucketCounts = map[string]int{}
	if err := r.loadFacetCounts(ctx, &f); err != nil {
		return nil, err
	}
	normalizeFacetSlices(&f)
	return &f, nil
}

func (r *pgTransactionsRepository) loadFacetValues(ctx context.Context, f *Facets) error {
	queries := []struct {
		sql  string
		dest *[]string
	}{
		{
			`SELECT DISTINCT source FROM transactions
             WHERE source IS NOT NULL AND source != ''
             ORDER BY source`,
			&f.Sources,
		},
		{
			`SELECT DISTINCT source_type FROM transactions
             WHERE source_type IS NOT NULL AND source_type != ''
             ORDER BY source_type`,
			&f.SourceTypes,
		},
		{
			`SELECT DISTINCT bank FROM transactions
             WHERE bank IS NOT NULL AND bank != ''
             ORDER BY bank`,
			&f.Banks,
		},
		{
			`SELECT DISTINCT category FROM transactions
             WHERE category IS NOT NULL AND category != ''
             ORDER BY category`,
			&f.Categories,
		},
		{
			`SELECT DISTINCT currency FROM transactions
             WHERE currency IS NOT NULL AND currency != ''
             ORDER BY currency`,
			&f.Currencies,
		},
		{
			`SELECT DISTINCT merchant_info FROM transactions
             WHERE merchant_info IS NOT NULL AND merchant_info != ''
             ORDER BY merchant_info`,
			&f.Merchants,
		},
		{
			`SELECT DISTINCT label FROM transaction_labels
             ORDER BY label`,
			&f.Labels,
		},
		{
			`SELECT DISTINCT bucket FROM transactions
             WHERE bucket IS NOT NULL AND bucket != ''
             ORDER BY bucket`,
			&f.Buckets,
		},
	}

	for _, q := range queries {
		rows, err := r.pool.Query(ctx, q.sql)
		if err != nil {
			return fmt.Errorf("fetching facets: %w", err)
		}
		var vals []string
		for rows.Next() {
			var v string
			if err := rows.Scan(&v); err != nil {
				rows.Close()
				return fmt.Errorf("scanning facet value: %w", err)
			}
			vals = append(vals, v)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterating facet rows: %w", err)
		}
		*q.dest = vals
	}
	return nil
}

func (r *pgTransactionsRepository) loadFacetCounts(ctx context.Context, f *Facets) error {
	if err := r.scanFacetCountMap(ctx, `
		SELECT label, COUNT(*)::int
		FROM transaction_labels
		GROUP BY label
		ORDER BY label
	`, "label", f.LabelCounts); err != nil {
		return err
	}

	countQueries := []struct {
		sql  string
		dest map[string]int
		name string
	}{
		{
			`SELECT category, COUNT(*)::int FROM transactions WHERE category IS NOT NULL AND category != '' GROUP BY category ORDER BY category`,
			f.CategoryCounts,
			"category",
		},
		{
			`SELECT bucket, COUNT(*)::int FROM transactions WHERE bucket IS NOT NULL AND bucket != '' GROUP BY bucket ORDER BY bucket`,
			f.BucketCounts,
			"bucket",
		},
	}
	for _, q := range countQueries {
		if err := r.scanFacetCountMap(ctx, q.sql, q.name, q.dest); err != nil {
			return err
		}
	}
	return nil
}

func (r *pgTransactionsRepository) scanFacetCountMap(ctx context.Context, sql, name string, dest map[string]int) error {
	rows, err := r.pool.Query(ctx, sql)
	if err != nil {
		return fmt.Errorf("fetching %s counts: %w", name, err)
	}
	defer rows.Close()

	for rows.Next() {
		var label string
		var count int
		if err := rows.Scan(&label, &count); err != nil {
			return fmt.Errorf("scanning %s count: %w", name, err)
		}
		dest[label] = count
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating %s counts: %w", name, err)
	}
	return nil
}

func normalizeFacetSlices(f *Facets) {
	if f.Sources == nil {
		f.Sources = []string{}
	}
	if f.SourceTypes == nil {
		f.SourceTypes = []string{}
	}
	if f.Banks == nil {
		f.Banks = []string{}
	}
	if f.Categories == nil {
		f.Categories = []string{}
	}
	if f.Currencies == nil {
		f.Currencies = []string{}
	}
	if f.Merchants == nil {
		f.Merchants = []string{}
	}
	if f.Labels == nil {
		f.Labels = []string{}
	}
	if f.Buckets == nil {
		f.Buckets = []string{}
	}
}

func (r *pgTransactionsRepository) UpdateTransaction(ctx context.Context, id string, u TransactionUpdate) error {
	if u.Description == nil && u.Category == nil && u.Bucket == nil {
		return nil
	}
	var setClauses []string
	var args []any
	n := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	if u.Description != nil {
		setClauses = append(setClauses, "description = "+n(*u.Description))
	}
	if u.Category != nil {
		setClauses = append(setClauses, "category = "+n(*u.Category))
	}
	if u.Bucket != nil {
		setClauses = append(setClauses, "bucket = "+n(*u.Bucket))
	}
	args = append(args, id)
	q := fmt.Sprintf(
		"UPDATE transactions SET %s, updated_at = NOW() WHERE id = $%d",
		strings.Join(setClauses, ", "), len(args),
	)
	tag, err := r.pool.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("updating transaction: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgTransactionsRepository) MuteTransaction(ctx context.Context, id string, muted bool, reason string) error {
	var tag pgconn.CommandTag
	var err error
	if muted {
		tag, err = r.pool.Exec(ctx,
			`UPDATE transactions SET muted=true, muted_by_merchant=false, mute_reason=NULLIF($2,''), updated_at=NOW() WHERE id=$1`,
			id, reason,
		)
	} else {
		tag, err = r.pool.Exec(ctx,
			`UPDATE transactions SET muted=false, muted_by_merchant=false, mute_reason=NULL, updated_at=NOW() WHERE id=$1`,
			id,
		)
	}
	if err != nil {
		return fmt.Errorf("muting transaction: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgTransactionsRepository) UpdateMuteReason(ctx context.Context, id, reason string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE transactions SET mute_reason=NULLIF($2,''), updated_at=NOW() WHERE id=$1 AND muted=true`,
		id, reason,
	)
	if err != nil {
		return fmt.Errorf("updating mute reason: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgTransactionsRepository) UpdateMerchantReason(ctx context.Context, id, reason string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE muted_merchants SET reason=NULLIF($2,'') WHERE id=$1`,
		id, reason,
	)
	if err != nil {
		return fmt.Errorf("updating merchant reason: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgTransactionsRepository) MuteByMerchant(ctx context.Context, pattern, reason string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning mute-by-merchant transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx,
		`INSERT INTO muted_merchants (pattern, reason)
			 VALUES ($1, NULLIF($2,''))
			 ON CONFLICT (pattern) DO UPDATE SET reason=EXCLUDED.reason`,
		pattern, reason,
	)
	if err != nil {
		return fmt.Errorf("storing muted merchant pattern: %w", err)
	}

	_, err = tx.Exec(ctx,
		`UPDATE transactions
			 SET muted=true, muted_by_merchant=true, mute_reason=NULLIF($2,''), updated_at=NOW()
			 WHERE merchant_info ILIKE $1`,
		"%"+pattern+"%", reason,
	)
	if err != nil {
		return fmt.Errorf("muting transactions by merchant: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *pgTransactionsRepository) ListMutedMerchants(ctx context.Context) ([]MutedMerchant, error) {
	return r.listMutedMerchantsQuery(ctx)
}

func (r *pgTransactionsRepository) listMutedMerchantsQuery(ctx context.Context) ([]MutedMerchant, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, pattern, COALESCE(reason,''), created_at FROM muted_merchants ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("listing muted merchants: %w", err)
	}
	defer rows.Close()
	var result []MutedMerchant
	for rows.Next() {
		var m MutedMerchant
		if err := rows.Scan(&m.ID, &m.Pattern, &m.Reason, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning muted merchant: %w", err)
		}
		result = append(result, m)
	}
	if result == nil {
		result = []MutedMerchant{}
	}
	return result, rows.Err()
}

func (r *pgTransactionsRepository) GetMutedMerchantsWithCount(ctx context.Context) ([]MutedMerchantWithCount, error) {
	return r.getMutedMerchantsWithCountQuery(ctx)
}

func (r *pgTransactionsRepository) getMutedMerchantsWithCountQuery(ctx context.Context) ([]MutedMerchantWithCount, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT mm.id, mm.pattern, COALESCE(mm.reason,''), mm.created_at,
		       COUNT(t.id) AS muted_count
		FROM muted_merchants mm
		LEFT JOIN transactions t
		  ON t.muted_by_merchant = true
		 AND t.merchant_info ILIKE '%' || mm.pattern || '%'
		GROUP BY mm.id
		ORDER BY mm.created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("listing muted merchants with count: %w", err)
	}
	defer rows.Close()
	var result []MutedMerchantWithCount
	for rows.Next() {
		var m MutedMerchantWithCount
		if err := rows.Scan(&m.ID, &m.Pattern, &m.Reason, &m.CreatedAt, &m.MutedCount); err != nil {
			return nil, fmt.Errorf("scanning muted merchant with count: %w", err)
		}
		result = append(result, m)
	}
	if result == nil {
		result = []MutedMerchantWithCount{}
	}
	return result, rows.Err()
}

func (r *pgTransactionsRepository) DeleteMutedMerchant(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM muted_merchants WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("deleting muted merchant: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgTransactionsRepository) UnmuteByPattern(ctx context.Context, pattern string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE transactions SET muted=false, muted_by_merchant=false, mute_reason=NULL, updated_at=NOW()
			 WHERE merchant_info ILIKE $1`,
		"%"+pattern+"%",
	)
	if err != nil {
		return fmt.Errorf("unmuting by pattern: %w", err)
	}
	return nil
}

func (r *pgTransactionsRepository) DeleteMutedMerchantAndUnmute(ctx context.Context, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var pattern string
	if err := tx.QueryRow(ctx,
		`DELETE FROM muted_merchants WHERE id=$1 RETURNING pattern`, id,
	).Scan(&pattern); err != nil {
		if errorsIsNoRows(err) {
			return ErrNotFound
		}
		return fmt.Errorf("deleting muted merchant: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE transactions
			 SET muted=false, muted_by_merchant=false, mute_reason=NULL, updated_at=NOW()
			 WHERE merchant_info ILIKE $1`,
		"%"+pattern+"%",
	); err != nil {
		return fmt.Errorf("unmuting transactions: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *pgTransactionsRepository) GetMutedMerchantPatterns(ctx context.Context) ([]string, error) {
	var patterns []string
	rows, err := r.pool.Query(ctx, `SELECT pattern FROM muted_merchants`)
	if err != nil {
		return nil, fmt.Errorf("fetching muted merchant patterns: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("scanning pattern: %w", err)
		}
		patterns = append(patterns, p)
	}
	return patterns, rows.Err()
}

func (r *pgTransactionsRepository) queryTransactionTotals(
	ctx context.Context,
	join string,
	where string,
	args []any,
) (TransactionListResult, error) {
	aggregateSQL := `
		SELECT COUNT(*), COALESCE(SUM(filtered.amount), 0)
		FROM (
			SELECT DISTINCT t.id, t.amount
			FROM transactions t` + join + where + `
		) AS filtered
	`

	var result TransactionListResult
	if err := r.pool.QueryRow(ctx, aggregateSQL, args...).Scan(&result.Total, &result.TotalAmount); err != nil {
		return TransactionListResult{}, err
	}

	return result, nil
}

func (r *pgTransactionsRepository) loadLabels(ctx context.Context, txns []Transaction) error {
	if len(txns) == 0 {
		return nil
	}

	ids := make([]string, len(txns))
	idx := make(map[string]int, len(txns))
	for i, t := range txns {
		ids[i] = t.ID
		idx[t.ID] = i
	}

	rows, err := r.pool.Query(ctx,
		`SELECT transaction_id, label FROM transaction_labels WHERE transaction_id = ANY($1) ORDER BY label`,
		ids,
	)
	if err != nil {
		return fmt.Errorf("fetching labels: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tid, label string
		if err := rows.Scan(&tid, &label); err != nil {
			return fmt.Errorf("scanning label row: %w", err)
		}
		if i, ok := idx[tid]; ok {
			txns[i].Labels = append(txns[i].Labels, label)
		}
	}
	return rows.Err()
}

func errorsIsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
