package postgres

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

type transactionsRepository struct {
	pool *pgxpool.Pool
}

type transactionQueryRequest struct {
	tenant     store.Tenant
	filter     store.ListFilter
	search     string
	countError string
	dataError  string
}

func newTransactionsRepository(deps repositoryDependencies) *transactionsRepository {
	return &transactionsRepository{
		pool: deps.pool,
	}
}

func (r *transactionsRepository) ListTransactions(
	ctx context.Context,
	tenant store.Tenant,
	f store.ListFilter,
) (transactions []store.Transaction, result store.TransactionListResult, err error) {
	return r.listTransactionsQuery(ctx, tenant, f)
}

func (r *transactionsRepository) listTransactionsQuery(
	ctx context.Context,
	tenant store.Tenant,
	f store.ListFilter,
) (transactions []store.Transaction, result store.TransactionListResult, err error) {
	return r.queryTransactions(ctx, transactionQueryRequest{
		tenant:     tenant,
		filter:     f,
		countError: "counting transactions",
		dataError:  "listing transactions",
	})
}

func (r *transactionsRepository) queryTransactions(
	ctx context.Context,
	request transactionQueryRequest,
) (transactions []store.Transaction, result store.TransactionListResult, err error) {
	f := normalizeTransactionListFilter(request.filter)
	offset, err := transactionOffset(f)
	if err != nil {
		return nil, store.TransactionListResult{}, err
	}
	query := strings.TrimSpace(request.search)

	where, args := buildListWhere(f)
	args = append(args, tenantIDParam(request.tenant))
	where = combineWhere(tenantWhere("t.tenant_id", fmt.Sprintf("$%d", len(args))), where)
	if query != "" {
		searchCond := buildSearchCondition(query, &args)
		where = combineWhere(searchCond, where)
	}

	join := joinLabel(f.Label)

	totalResult, err := r.queryTransactionTotals(ctx, join, where, args)
	if err != nil {
		return nil, store.TransactionListResult{}, fmt.Errorf("%s: %w", request.countError, err)
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
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, join, where, transactionOrderClause(f), limitArg, offsetArg)

	rows, err := r.pool.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, store.TransactionListResult{}, fmt.Errorf("%s: %w", request.dataError, err)
	}
	defer rows.Close()

	txns, err := scanTransactions(rows)
	if err != nil {
		return nil, store.TransactionListResult{}, err
	}

	if err := r.loadLabels(ctx, txns); err != nil {
		return nil, store.TransactionListResult{}, err
	}

	return txns, totalResult, nil
}

func transactionOffset(filter store.ListFilter) (int, error) {
	if filter.Page-1 > math.MaxInt/filter.PageSize {
		return 0, errors.E(
			"store.transactions.list",
			errors.InvalidInput,
			fmt.Sprintf("%s: page=%d page_size=%d", messagePaginationOverflow, filter.Page, filter.PageSize),
		)
	}
	return (filter.Page - 1) * filter.PageSize, nil
}

func normalizeTransactionListFilter(f store.ListFilter) store.ListFilter {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = 20
	}
	return f
}

func transactionOrderClause(f store.ListFilter) string {
	if strings.ToLower(f.SortDir) == "asc" {
		return "t.timestamp ASC"
	}
	return "t.timestamp DESC"
}

func (r *transactionsRepository) GetTransaction(ctx context.Context, tenant store.Tenant, id string) (*store.Transaction, error) {
	return r.getTransactionQuery(ctx, tenant, id)
}

func (r *transactionsRepository) getTransactionQuery(ctx context.Context, tenant store.Tenant, id string) (*store.Transaction, error) {
	const q = `
		SELECT t.id, t.message_id, t.amount, t.currency,
		       t.original_amount, t.original_currency, t.exchange_rate,
		       t.timestamp, t.merchant_info,
		       COALESCE(t.category, ''), COALESCE(t.bucket, ''),
		       t.source, COALESCE(t.source_type, ''), COALESCE(t.source_label, ''), COALESCE(t.bank, ''),
		       COALESCE(t.description, ''), t.muted, t.muted_by_merchant, COALESCE(t.mute_reason,''), t.created_at, t.updated_at
		FROM transactions t
		WHERE t.id = $1 AND t.tenant_id IS NOT DISTINCT FROM $2
	`
	rows, err := r.pool.Query(ctx, q, id, tenantIDParam(tenant))
	if err != nil {
		return nil, fmt.Errorf("fetching transaction: %w", err)
	}
	defer rows.Close()

	txns, err := scanTransactions(rows)
	if err != nil {
		return nil, err
	}
	if len(txns) == 0 {
		return nil, notFound("store.transactions.get")
	}

	if err := r.loadLabels(ctx, txns); err != nil {
		return nil, err
	}
	return &txns[0], nil
}

func (r *transactionsRepository) UpdateDescription(ctx context.Context, tenant store.Tenant, id, description string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE transactions SET description = $1 WHERE id = $2 AND tenant_id IS NOT DISTINCT FROM $3`,
		description, id, tenantIDParam(tenant),
	)
	if err != nil {
		return fmt.Errorf("updating description: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return notFound("store.transactions.update_description")
	}
	return nil
}

func (r *transactionsRepository) AddLabel(ctx context.Context, tenant store.Tenant, transactionID, label string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning add-label transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`INSERT INTO transaction_label_sources (transaction_id, label, source_type, merchant_pattern)
			 SELECT id, $2, 'manual', ''
			 FROM transactions
			 WHERE id = $1 AND tenant_id IS NOT DISTINCT FROM $3
			 ON CONFLICT (transaction_id, label, source_type, merchant_pattern) DO NOTHING`,
		transactionID, label, tenantIDParam(tenant),
	); err != nil {
		return fmt.Errorf("adding label source: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO transaction_labels (transaction_id, label)
			 SELECT id, $2
			 FROM transactions
			 WHERE id = $1 AND tenant_id IS NOT DISTINCT FROM $3
			 ON CONFLICT (transaction_id, label) DO NOTHING`,
		transactionID, label, tenantIDParam(tenant),
	); err != nil {
		return fmt.Errorf("adding label: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing add-label transaction: %w", err)
	}
	return nil
}

func (r *transactionsRepository) AddLabels(ctx context.Context, tenant store.Tenant, transactionID string, labels []string) error {
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
			 SELECT t.id, label, 'manual', ''
			 FROM transactions t, unnest($2::text[]) AS labels(label)
			 WHERE t.id = $1 AND t.tenant_id IS NOT DISTINCT FROM $3
			 ON CONFLICT (transaction_id, label, source_type, merchant_pattern) DO NOTHING`,
		transactionID, labels, tenantIDParam(tenant),
	); err != nil {
		return fmt.Errorf("adding label sources: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO transaction_labels (transaction_id, label)
			 SELECT t.id, label
			 FROM transactions t, unnest($2::text[]) AS labels(label)
			 WHERE t.id = $1 AND t.tenant_id IS NOT DISTINCT FROM $3
			 ON CONFLICT (transaction_id, label) DO NOTHING`,
		transactionID, labels, tenantIDParam(tenant),
	); err != nil {
		return fmt.Errorf("adding labels: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing add-labels transaction: %w", err)
	}
	return nil
}

func (r *transactionsRepository) RemoveLabel(ctx context.Context, tenant store.Tenant, transactionID, label string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning remove-label transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`DELETE FROM transaction_label_sources tls
		 USING transactions t
		 WHERE tls.transaction_id = t.id
		   AND tls.transaction_id = $1
		   AND tls.label = $2
		   AND t.tenant_id IS NOT DISTINCT FROM $3`,
		transactionID, label, tenantIDParam(tenant),
	); err != nil {
		return fmt.Errorf("removing label sources: %w", err)
	}

	tag, err := tx.Exec(ctx,
		`DELETE FROM transaction_labels tl
		 USING transactions t
		 WHERE tl.transaction_id = t.id
		   AND tl.transaction_id = $1
		   AND tl.label = $2
		   AND t.tenant_id IS NOT DISTINCT FROM $3`,
		transactionID, label, tenantIDParam(tenant),
	)
	if err != nil {
		return fmt.Errorf("removing label: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return notFound("store.transactions.remove_label")
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing remove-label transaction: %w", err)
	}
	return nil
}

func (r *transactionsRepository) SearchTransactions(
	ctx context.Context,
	tenant store.Tenant,
	query string,
	f store.ListFilter,
) (transactions []store.Transaction, result store.TransactionListResult, err error) {
	return r.searchTransactionsQuery(ctx, tenant, query, f)
}

func (r *transactionsRepository) searchTransactionsQuery(
	ctx context.Context,
	tenant store.Tenant,
	query string,
	f store.ListFilter,
) (transactions []store.Transaction, result store.TransactionListResult, err error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return r.listTransactionsQuery(ctx, tenant, f)
	}
	return r.queryTransactions(ctx, transactionQueryRequest{
		tenant:     tenant,
		filter:     f,
		search:     query,
		countError: "counting search results",
		dataError:  "searching transactions",
	})
}

func (r *transactionsRepository) GetFacets(ctx context.Context, tenant store.Tenant) (*store.Facets, error) {
	return r.getFacetsQuery(ctx, tenant)
}

func (r *transactionsRepository) getFacetsQuery(ctx context.Context, tenant store.Tenant) (*store.Facets, error) {
	var f store.Facets
	if err := r.loadFacetValues(ctx, tenant, &f); err != nil {
		return nil, err
	}

	f.LabelCounts = map[string]int{}
	f.CategoryCounts = map[string]int{}
	f.BucketCounts = map[string]int{}
	if err := r.loadFacetCounts(ctx, tenant, &f); err != nil {
		return nil, err
	}
	normalizeFacetSlices(&f)
	return &f, nil
}

func (r *transactionsRepository) loadFacetValues(ctx context.Context, tenant store.Tenant, f *store.Facets) error {
	queries := []struct {
		sql  string
		dest *[]string
	}{
		{
			`SELECT DISTINCT source FROM transactions
             WHERE tenant_id IS NOT DISTINCT FROM $1 AND source IS NOT NULL AND source != ''
             ORDER BY source`,
			&f.Sources,
		},
		{
			`SELECT DISTINCT source_type FROM transactions
             WHERE tenant_id IS NOT DISTINCT FROM $1 AND source_type IS NOT NULL AND source_type != ''
             ORDER BY source_type`,
			&f.SourceTypes,
		},
		{
			`SELECT DISTINCT bank FROM transactions
             WHERE tenant_id IS NOT DISTINCT FROM $1 AND bank IS NOT NULL AND bank != ''
             ORDER BY bank`,
			&f.Banks,
		},
		{
			`SELECT DISTINCT category FROM transactions
             WHERE tenant_id IS NOT DISTINCT FROM $1 AND category IS NOT NULL AND category != ''
             ORDER BY category`,
			&f.Categories,
		},
		{
			`SELECT DISTINCT currency FROM transactions
             WHERE tenant_id IS NOT DISTINCT FROM $1 AND currency IS NOT NULL AND currency != ''
             ORDER BY currency`,
			&f.Currencies,
		},
		{
			`SELECT DISTINCT merchant_info FROM transactions
             WHERE tenant_id IS NOT DISTINCT FROM $1 AND merchant_info IS NOT NULL AND merchant_info != ''
             ORDER BY merchant_info`,
			&f.Merchants,
		},
		{
			`SELECT DISTINCT tl.label FROM transaction_labels tl
			 JOIN transactions t ON t.id = tl.transaction_id
			 WHERE t.tenant_id IS NOT DISTINCT FROM $1
             ORDER BY label`,
			&f.Labels,
		},
		{
			`SELECT DISTINCT bucket FROM transactions
             WHERE tenant_id IS NOT DISTINCT FROM $1 AND bucket IS NOT NULL AND bucket != ''
             ORDER BY bucket`,
			&f.Buckets,
		},
	}

	for _, q := range queries {
		rows, err := r.pool.Query(ctx, q.sql, tenantIDParam(tenant))
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

func (r *transactionsRepository) loadFacetCounts(ctx context.Context, tenant store.Tenant, f *store.Facets) error {
	if err := r.scanFacetCountMap(ctx, `
		SELECT tl.label, COUNT(*)::int
		FROM transaction_labels tl
		JOIN transactions t ON t.id = tl.transaction_id
		WHERE t.tenant_id IS NOT DISTINCT FROM $1
		GROUP BY tl.label
		ORDER BY label
	`, "label", f.LabelCounts, tenantIDParam(tenant)); err != nil {
		return err
	}

	countQueries := []struct {
		sql  string
		dest map[string]int
		name string
	}{
		{
			`SELECT category, COUNT(*)::int
			 FROM transactions
			 WHERE tenant_id IS NOT DISTINCT FROM $1 AND category IS NOT NULL AND category != ''
			 GROUP BY category
			 ORDER BY category`,
			f.CategoryCounts,
			"category",
		},
		{
			`SELECT bucket, COUNT(*)::int
			 FROM transactions
			 WHERE tenant_id IS NOT DISTINCT FROM $1 AND bucket IS NOT NULL AND bucket != ''
			 GROUP BY bucket
			 ORDER BY bucket`,
			f.BucketCounts,
			"bucket",
		},
	}
	for _, q := range countQueries {
		if err := r.scanFacetCountMap(ctx, q.sql, q.name, q.dest, tenantIDParam(tenant)); err != nil {
			return err
		}
	}
	return nil
}

func (r *transactionsRepository) scanFacetCountMap(ctx context.Context, sql, name string, dest map[string]int, args ...any) error {
	rows, err := r.pool.Query(ctx, sql, args...)
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

func normalizeFacetSlices(f *store.Facets) {
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

func (r *transactionsRepository) UpdateTransaction(ctx context.Context, tenant store.Tenant, id string, u store.TransactionUpdate) error {
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
	args = append(args, id, tenantIDParam(tenant))
	q := fmt.Sprintf(
		"UPDATE transactions SET %s, updated_at = NOW() WHERE id = $%d AND tenant_id IS NOT DISTINCT FROM $%d",
		strings.Join(setClauses, ", "), len(args)-1, len(args),
	)
	tag, err := r.pool.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("updating transaction: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return notFound("store.transactions.update")
	}
	return nil
}

func (r *transactionsRepository) MuteTransaction(ctx context.Context, tenant store.Tenant, id string, muted bool, reason string) error {
	var tag pgconn.CommandTag
	var err error
	if muted {
		tag, err = r.pool.Exec(ctx,
			`UPDATE transactions SET muted=true, muted_by_merchant=false, mute_reason=NULLIF($2,''), updated_at=NOW()
			 WHERE id=$1 AND tenant_id IS NOT DISTINCT FROM $3`,
			id, reason, tenantIDParam(tenant),
		)
	} else {
		tag, err = r.pool.Exec(ctx,
			`UPDATE transactions SET muted=false, muted_by_merchant=false, mute_reason=NULL, updated_at=NOW()
			 WHERE id=$1 AND tenant_id IS NOT DISTINCT FROM $2`,
			id, tenantIDParam(tenant),
		)
	}
	if err != nil {
		return fmt.Errorf("muting transaction: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return notFound("store.transactions.mute")
	}
	return nil
}

func (r *transactionsRepository) UpdateMuteReason(ctx context.Context, tenant store.Tenant, id, reason string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE transactions SET mute_reason=NULLIF($2,''), updated_at=NOW()
		 WHERE id=$1 AND muted=true AND tenant_id IS NOT DISTINCT FROM $3`,
		id, reason, tenantIDParam(tenant),
	)
	if err != nil {
		return fmt.Errorf("updating mute reason: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return notFound("store.transactions.update_mute_reason")
	}
	return nil
}

func (r *transactionsRepository) UpdateMerchantReason(ctx context.Context, tenant store.Tenant, id, reason string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE muted_merchants SET reason=NULLIF($2,'') WHERE id=$1 AND tenant_id IS NOT DISTINCT FROM $3`,
		id, reason, tenantIDParam(tenant),
	)
	if err != nil {
		return fmt.Errorf("updating merchant reason: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return notFound("store.transactions.update_merchant_reason")
	}
	return nil
}

func (r *transactionsRepository) MuteByMerchant(ctx context.Context, tenant store.Tenant, pattern, reason string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning mute-by-merchant transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx,
		mutedMerchantUpsertSQL(tenant),
		tenantIDParam(tenant), pattern, reason,
	)
	if err != nil {
		return fmt.Errorf("storing muted merchant pattern: %w", err)
	}

	_, err = tx.Exec(ctx,
		`UPDATE transactions
			 SET muted=true, muted_by_merchant=true, mute_reason=NULLIF($2,''), updated_at=NOW()
			 WHERE merchant_info ILIKE $1 AND tenant_id IS NOT DISTINCT FROM $3`,
		"%"+pattern+"%", reason, tenantIDParam(tenant),
	)
	if err != nil {
		return fmt.Errorf("muting transactions by merchant: %w", err)
	}

	return tx.Commit(ctx)
}

func mutedMerchantUpsertSQL(tenant store.Tenant) string {
	if tenantIDParam(tenant) == nil {
		return `INSERT INTO muted_merchants (tenant_id, pattern, reason)
			 VALUES ($1, $2, NULLIF($3,''))
			 ON CONFLICT (pattern) WHERE tenant_id IS NULL
			 DO UPDATE SET reason=EXCLUDED.reason`
	}
	return `INSERT INTO muted_merchants (tenant_id, pattern, reason)
			 VALUES ($1, $2, NULLIF($3,''))
			 ON CONFLICT (tenant_id, pattern) WHERE tenant_id IS NOT NULL
			 DO UPDATE SET reason=EXCLUDED.reason`
}

func (r *transactionsRepository) ListMutedMerchants(ctx context.Context, tenant store.Tenant) ([]store.MutedMerchant, error) {
	return r.listMutedMerchantsQuery(ctx, tenant)
}

func (r *transactionsRepository) listMutedMerchantsQuery(ctx context.Context, tenant store.Tenant) ([]store.MutedMerchant, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, pattern, COALESCE(reason,''), created_at
		 FROM muted_merchants
		 WHERE tenant_id IS NOT DISTINCT FROM $1
		 ORDER BY created_at DESC`,
		tenantIDParam(tenant))
	if err != nil {
		return nil, fmt.Errorf("listing muted merchants: %w", err)
	}
	defer rows.Close()
	var result []store.MutedMerchant
	for rows.Next() {
		var m store.MutedMerchant
		if err := rows.Scan(&m.ID, &m.Pattern, &m.Reason, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning muted merchant: %w", err)
		}
		result = append(result, m)
	}
	if result == nil {
		result = []store.MutedMerchant{}
	}
	return result, rows.Err()
}

func (r *transactionsRepository) GetMutedMerchantsWithCount(ctx context.Context, tenant store.Tenant) ([]store.MutedMerchantWithCount, error) {
	return r.getMutedMerchantsWithCountQuery(ctx, tenant)
}

func (r *transactionsRepository) getMutedMerchantsWithCountQuery(ctx context.Context, tenant store.Tenant) ([]store.MutedMerchantWithCount, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT mm.id, mm.pattern, COALESCE(mm.reason,''), mm.created_at,
		       COUNT(t.id) AS muted_count
		FROM muted_merchants mm
		LEFT JOIN transactions t
		  ON t.muted_by_merchant = true
		 AND t.merchant_info ILIKE '%' || mm.pattern || '%'
		 AND t.tenant_id IS NOT DISTINCT FROM mm.tenant_id
		WHERE mm.tenant_id IS NOT DISTINCT FROM $1
		GROUP BY mm.id
		ORDER BY mm.created_at DESC
	`, tenantIDParam(tenant))
	if err != nil {
		return nil, fmt.Errorf("listing muted merchants with count: %w", err)
	}
	defer rows.Close()
	var result []store.MutedMerchantWithCount
	for rows.Next() {
		var m store.MutedMerchantWithCount
		if err := rows.Scan(&m.ID, &m.Pattern, &m.Reason, &m.CreatedAt, &m.MutedCount); err != nil {
			return nil, fmt.Errorf("scanning muted merchant with count: %w", err)
		}
		result = append(result, m)
	}
	if result == nil {
		result = []store.MutedMerchantWithCount{}
	}
	return result, rows.Err()
}

func (r *transactionsRepository) DeleteMutedMerchant(ctx context.Context, tenant store.Tenant, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM muted_merchants WHERE id=$1 AND tenant_id IS NOT DISTINCT FROM $2`, id, tenantIDParam(tenant))
	if err != nil {
		return fmt.Errorf("deleting muted merchant: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return notFound("store.transactions.delete_muted_merchant")
	}
	return nil
}

func (r *transactionsRepository) UnmuteByPattern(ctx context.Context, tenant store.Tenant, pattern string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE transactions SET muted=false, muted_by_merchant=false, mute_reason=NULL, updated_at=NOW()
			 WHERE merchant_info ILIKE $1 AND tenant_id IS NOT DISTINCT FROM $2`,
		"%"+pattern+"%", tenantIDParam(tenant),
	)
	if err != nil {
		return fmt.Errorf("unmuting by pattern: %w", err)
	}
	return nil
}

func (r *transactionsRepository) DeleteMutedMerchantAndUnmute(ctx context.Context, tenant store.Tenant, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var pattern string
	if err := tx.QueryRow(ctx,
		`DELETE FROM muted_merchants WHERE id=$1 AND tenant_id IS NOT DISTINCT FROM $2 RETURNING pattern`, id, tenantIDParam(tenant),
	).Scan(&pattern); err != nil {
		if errorsIsNoRows(err) {
			return notFound("store.transactions.delete_muted_merchant_and_unmute")
		}
		return fmt.Errorf("deleting muted merchant: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE transactions
			 SET muted=false, muted_by_merchant=false, mute_reason=NULL, updated_at=NOW()
			 WHERE merchant_info ILIKE $1 AND tenant_id IS NOT DISTINCT FROM $2`,
		"%"+pattern+"%", tenantIDParam(tenant),
	); err != nil {
		return fmt.Errorf("unmuting transactions: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *transactionsRepository) GetMutedMerchantPatterns(ctx context.Context, tenant store.Tenant) ([]string, error) {
	var patterns []string
	rows, err := r.pool.Query(ctx, `SELECT pattern FROM muted_merchants WHERE tenant_id IS NOT DISTINCT FROM $1`, tenantIDParam(tenant))
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

func (r *transactionsRepository) queryTransactionTotals(
	ctx context.Context,
	join string,
	where string,
	args []any,
) (store.TransactionListResult, error) {
	aggregateSQL := `
		SELECT COUNT(*), COALESCE(SUM(filtered.amount), 0)
		FROM (
			SELECT DISTINCT t.id, t.amount
			FROM transactions t` + join + where + `
		) AS filtered
	`

	var result store.TransactionListResult
	if err := r.pool.QueryRow(ctx, aggregateSQL, args...).Scan(&result.Total, &result.TotalAmount); err != nil {
		return store.TransactionListResult{}, err
	}

	return result, nil
}

func (r *transactionsRepository) loadLabels(ctx context.Context, txns []store.Transaction) error {
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
