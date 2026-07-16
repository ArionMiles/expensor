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
	args = append(args, request.tenant.ID)
	where = combineWhere(fmt.Sprintf("t.tenant_id = $%d", len(args)), where)
	if query != "" {
		searchCond := buildSearchCondition(query, &args)
		where = combineWhere(searchCond, where)
	}

	join := joinLabel(f.Label)

	totalResult, err := r.queryTransactionTotals(ctx, join, where, args)
	if err != nil {
		return nil, store.TransactionListResult{}, errors.E("postgres.transactions.query_transactions", request.countError, err)
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
		return nil, store.TransactionListResult{}, errors.E("postgres.transactions.query_transactions", request.dataError, err)
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
			fmt.Sprintf("pagination offset overflow: page=%d page_size=%d", filter.Page, filter.PageSize),
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
		WHERE t.id = $1 AND t.tenant_id = $2
	`
	rows, err := r.pool.Query(ctx, q, id, tenant.ID)
	if err != nil {
		return nil, errors.E("postgres.transactions.get_transaction_query", "fetching transaction", err)
	}
	defer rows.Close()

	txns, err := scanTransactions(rows)
	if err != nil {
		return nil, err
	}
	if len(txns) == 0 {
		return nil, errors.E("store.transactions.get", errors.NotFound)
	}

	if err := r.loadLabels(ctx, txns); err != nil {
		return nil, err
	}
	return &txns[0], nil
}

func (r *transactionsRepository) UpdateDescription(ctx context.Context, tenant store.Tenant, id, description string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE transactions SET description = $1 WHERE id = $2 AND tenant_id = $3`,
		description, id, tenant.ID,
	)
	if err != nil {
		return errors.E("postgres.transactions.update_description", "updating description", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.E("store.transactions.update_description", errors.NotFound)
	}
	return nil
}

func (r *transactionsRepository) AddLabel(ctx context.Context, tenant store.Tenant, transactionID, label string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return errors.E("postgres.transactions.add_label", "beginning add-label transaction", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`INSERT INTO transaction_label_sources (transaction_id, label, source_type, merchant_pattern)
			 SELECT id, $2, 'manual', ''
			 FROM transactions
			 WHERE id = $1 AND tenant_id = $3
			 ON CONFLICT (transaction_id, label, source_type, merchant_pattern) DO NOTHING`,
		transactionID, label, tenant.ID,
	); err != nil {
		return errors.E("postgres.transactions.add_label", "adding label source", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO transaction_labels (transaction_id, label)
			 SELECT id, $2
			 FROM transactions
			 WHERE id = $1 AND tenant_id = $3
			 ON CONFLICT (transaction_id, label) DO NOTHING`,
		transactionID, label, tenant.ID,
	); err != nil {
		return errors.E("postgres.transactions.add_label", "adding label", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return errors.E("postgres.transactions.add_label", "committing add-label transaction", err)
	}
	return nil
}

func (r *transactionsRepository) AddLabels(ctx context.Context, tenant store.Tenant, transactionID string, labels []string) error {
	if len(labels) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return errors.E("postgres.transactions.add_labels", "beginning add-labels transaction", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`INSERT INTO transaction_label_sources (transaction_id, label, source_type, merchant_pattern)
			 SELECT t.id, label, 'manual', ''
			 FROM transactions t, unnest($2::text[]) AS labels(label)
			 WHERE t.id = $1 AND t.tenant_id = $3
			 ON CONFLICT (transaction_id, label, source_type, merchant_pattern) DO NOTHING`,
		transactionID, labels, tenant.ID,
	); err != nil {
		return errors.E("postgres.transactions.add_labels", "adding label sources", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO transaction_labels (transaction_id, label)
			 SELECT t.id, label
			 FROM transactions t, unnest($2::text[]) AS labels(label)
			 WHERE t.id = $1 AND t.tenant_id = $3
			 ON CONFLICT (transaction_id, label) DO NOTHING`,
		transactionID, labels, tenant.ID,
	); err != nil {
		return errors.E("postgres.transactions.add_labels", "adding labels", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return errors.E("postgres.transactions.add_labels", "committing add-labels transaction", err)
	}
	return nil
}

func (r *transactionsRepository) RemoveLabel(ctx context.Context, tenant store.Tenant, transactionID, label string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return errors.E("postgres.transactions.remove_label", "beginning remove-label transaction", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`DELETE FROM transaction_label_sources tls
		 USING transactions t
		 WHERE tls.transaction_id = t.id
		   AND tls.transaction_id = $1
		   AND tls.label = $2
		   AND t.tenant_id = $3`,
		transactionID, label, tenant.ID,
	); err != nil {
		return errors.E("postgres.transactions.remove_label", "removing label sources", err)
	}

	tag, err := tx.Exec(ctx,
		`DELETE FROM transaction_labels tl
		 USING transactions t
		 WHERE tl.transaction_id = t.id
		   AND tl.transaction_id = $1
		   AND tl.label = $2
		   AND t.tenant_id = $3`,
		transactionID, label, tenant.ID,
	)
	if err != nil {
		return errors.E("postgres.transactions.remove_label", "removing label", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.E("store.transactions.remove_label", errors.NotFound)
	}

	if err := tx.Commit(ctx); err != nil {
		return errors.E("postgres.transactions.remove_label", "committing remove-label transaction", err)
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
             WHERE tenant_id = $1 AND source IS NOT NULL AND source != ''
             ORDER BY source`,
			&f.Sources,
		},
		{
			`SELECT DISTINCT source_type FROM transactions
             WHERE tenant_id = $1 AND source_type IS NOT NULL AND source_type != ''
             ORDER BY source_type`,
			&f.SourceTypes,
		},
		{
			`SELECT DISTINCT bank FROM transactions
             WHERE tenant_id = $1 AND bank IS NOT NULL AND bank != ''
             ORDER BY bank`,
			&f.Banks,
		},
		{
			`SELECT DISTINCT category FROM transactions
             WHERE tenant_id = $1 AND category IS NOT NULL AND category != ''
             ORDER BY category`,
			&f.Categories,
		},
		{
			`SELECT DISTINCT currency FROM transactions
             WHERE tenant_id = $1 AND currency IS NOT NULL AND currency != ''
             ORDER BY currency`,
			&f.Currencies,
		},
		{
			`SELECT DISTINCT merchant_info FROM transactions
             WHERE tenant_id = $1 AND merchant_info IS NOT NULL AND merchant_info != ''
             ORDER BY merchant_info`,
			&f.Merchants,
		},
		{
			`SELECT DISTINCT tl.label FROM transaction_labels tl
			 JOIN transactions t ON t.id = tl.transaction_id
			 WHERE t.tenant_id = $1
             ORDER BY label`,
			&f.Labels,
		},
		{
			`SELECT DISTINCT bucket FROM transactions
             WHERE tenant_id = $1 AND bucket IS NOT NULL AND bucket != ''
             ORDER BY bucket`,
			&f.Buckets,
		},
	}

	for _, q := range queries {
		rows, err := r.pool.Query(ctx, q.sql, tenant.ID)
		if err != nil {
			return errors.E("postgres.transactions.load_facet_values", "fetching facets", err)
		}
		var vals []string
		for rows.Next() {
			var v string
			if err := rows.Scan(&v); err != nil {
				rows.Close()
				return errors.E("postgres.transactions.load_facet_values", "scanning facet value", err)
			}
			vals = append(vals, v)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return errors.E("postgres.transactions.load_facet_values", "iterating facet rows", err)
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
		WHERE t.tenant_id = $1
		GROUP BY tl.label
		ORDER BY label
	`, "label", f.LabelCounts, tenant.ID); err != nil {
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
			 WHERE tenant_id = $1 AND category IS NOT NULL AND category != ''
			 GROUP BY category
			 ORDER BY category`,
			f.CategoryCounts,
			"category",
		},
		{
			`SELECT bucket, COUNT(*)::int
			 FROM transactions
			 WHERE tenant_id = $1 AND bucket IS NOT NULL AND bucket != ''
			 GROUP BY bucket
			 ORDER BY bucket`,
			f.BucketCounts,
			"bucket",
		},
	}
	for _, q := range countQueries {
		if err := r.scanFacetCountMap(ctx, q.sql, q.name, q.dest, tenant.ID); err != nil {
			return err
		}
	}
	return nil
}

func (r *transactionsRepository) scanFacetCountMap(ctx context.Context, sql, name string, dest map[string]int, args ...any) error {
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return errors.E("postgres.transactions.scan_facet_count_map", fmt.Sprintf("fetching %s counts", name), err)
	}
	defer rows.Close()

	for rows.Next() {
		var label string
		var count int
		if err := rows.Scan(&label, &count); err != nil {
			return errors.E("postgres.transactions.scan_facet_count_map", fmt.Sprintf("scanning %s count", name), err)
		}
		dest[label] = count
	}
	if err := rows.Err(); err != nil {
		return errors.E("postgres.transactions.scan_facet_count_map", fmt.Sprintf("iterating %s counts", name), err)
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
	args = append(args, id, tenant.ID)
	q := fmt.Sprintf(
		"UPDATE transactions SET %s, updated_at = NOW() WHERE id = $%d AND tenant_id = $%d",
		strings.Join(setClauses, ", "), len(args)-1, len(args),
	)
	tag, err := r.pool.Exec(ctx, q, args...)
	if err != nil {
		return errors.E("postgres.transactions.update_transaction", "updating transaction", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.E("store.transactions.update", errors.NotFound)
	}
	return nil
}

func (r *transactionsRepository) MuteTransaction(ctx context.Context, tenant store.Tenant, id string, muted bool, reason string) error {
	var tag pgconn.CommandTag
	var err error
	if muted {
		tag, err = r.pool.Exec(ctx,
			`UPDATE transactions SET muted=true, muted_by_merchant=false, mute_reason=NULLIF($2,''), updated_at=NOW()
			 WHERE id=$1 AND tenant_id = $3`,
			id, reason, tenant.ID,
		)
	} else {
		tag, err = r.pool.Exec(ctx,
			`UPDATE transactions SET muted=false, muted_by_merchant=false, mute_reason=NULL, updated_at=NOW()
			 WHERE id=$1 AND tenant_id = $2`,
			id, tenant.ID,
		)
	}
	if err != nil {
		return errors.E("postgres.transactions.mute_transaction", "muting transaction", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.E("store.transactions.mute", errors.NotFound)
	}
	return nil
}

func (r *transactionsRepository) UpdateMuteReason(ctx context.Context, tenant store.Tenant, id, reason string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE transactions SET mute_reason=NULLIF($2,''), updated_at=NOW()
		 WHERE id=$1 AND muted=true AND tenant_id = $3`,
		id, reason, tenant.ID,
	)
	if err != nil {
		return errors.E("postgres.transactions.update_mute_reason", "updating mute reason", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.E("store.transactions.update_mute_reason", errors.NotFound)
	}
	return nil
}

func (r *transactionsRepository) UpdateMerchantReason(ctx context.Context, tenant store.Tenant, id, reason string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE muted_merchants SET reason=NULLIF($2,'') WHERE id=$1 AND tenant_id = $3`,
		id, reason, tenant.ID,
	)
	if err != nil {
		return errors.E("postgres.transactions.update_merchant_reason", "updating merchant reason", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.E("store.transactions.update_merchant_reason", errors.NotFound)
	}
	return nil
}

func (r *transactionsRepository) MuteByMerchant(ctx context.Context, tenant store.Tenant, pattern, reason string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return errors.E("postgres.transactions.mute_by_merchant", "beginning mute-by-merchant transaction", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx,
		mutedMerchantUpsertSQL,
		tenant.ID, pattern, reason,
	)
	if err != nil {
		return errors.E("postgres.transactions.mute_by_merchant", "storing muted merchant pattern", err)
	}

	_, err = tx.Exec(ctx,
		`UPDATE transactions
			 SET muted=true, muted_by_merchant=true, mute_reason=NULLIF($2,''), updated_at=NOW()
			 WHERE merchant_info ILIKE $1 AND tenant_id = $3`,
		"%"+pattern+"%", reason, tenant.ID,
	)
	if err != nil {
		return errors.E("postgres.transactions.mute_by_merchant", "muting transactions by merchant", err)
	}

	return tx.Commit(ctx)
}

const mutedMerchantUpsertSQL = `
	INSERT INTO muted_merchants (tenant_id, pattern, reason)
	VALUES ($1, $2, NULLIF($3,''))
	ON CONFLICT (tenant_id, pattern) WHERE tenant_id IS NOT NULL
	DO UPDATE SET reason = EXCLUDED.reason
`

func (r *transactionsRepository) ListMutedMerchants(ctx context.Context, tenant store.Tenant) ([]store.MutedMerchant, error) {
	return r.listMutedMerchantsQuery(ctx, tenant)
}

func (r *transactionsRepository) listMutedMerchantsQuery(ctx context.Context, tenant store.Tenant) ([]store.MutedMerchant, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, pattern, COALESCE(reason,''), created_at
		 FROM muted_merchants
		 WHERE tenant_id = $1
		 ORDER BY created_at DESC`,
		tenant.ID)
	if err != nil {
		return nil, errors.E("postgres.transactions.list_muted_merchants_query", "listing muted merchants", err)
	}
	defer rows.Close()
	var result []store.MutedMerchant
	for rows.Next() {
		var m store.MutedMerchant
		if err := rows.Scan(&m.ID, &m.Pattern, &m.Reason, &m.CreatedAt); err != nil {
			return nil, errors.E("postgres.transactions.list_muted_merchants_query", "scanning muted merchant", err)
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
		 AND t.tenant_id = mm.tenant_id
		WHERE mm.tenant_id = $1
		GROUP BY mm.id
		ORDER BY mm.created_at DESC
	`, tenant.ID)
	if err != nil {
		return nil, errors.E("postgres.transactions.get_muted_merchants_with_count_query", "listing muted merchants with count", err)
	}
	defer rows.Close()
	var result []store.MutedMerchantWithCount
	for rows.Next() {
		var m store.MutedMerchantWithCount
		if err := rows.Scan(&m.ID, &m.Pattern, &m.Reason, &m.CreatedAt, &m.MutedCount); err != nil {
			return nil, errors.E("postgres.transactions.get_muted_merchants_with_count_query", "scanning muted merchant with count", err)
		}
		result = append(result, m)
	}
	if result == nil {
		result = []store.MutedMerchantWithCount{}
	}
	return result, rows.Err()
}

func (r *transactionsRepository) DeleteMutedMerchant(ctx context.Context, tenant store.Tenant, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM muted_merchants WHERE id=$1 AND tenant_id = $2`, id, tenant.ID)
	if err != nil {
		return errors.E("postgres.transactions.delete_muted_merchant", "deleting muted merchant", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.E("store.transactions.delete_muted_merchant", errors.NotFound)
	}
	return nil
}

func (r *transactionsRepository) UnmuteByPattern(ctx context.Context, tenant store.Tenant, pattern string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE transactions SET muted=false, muted_by_merchant=false, mute_reason=NULL, updated_at=NOW()
			 WHERE merchant_info ILIKE $1 AND tenant_id = $2`,
		"%"+pattern+"%", tenant.ID,
	)
	if err != nil {
		return errors.E("postgres.transactions.unmute_by_pattern", "unmuting by pattern", err)
	}
	return nil
}

func (r *transactionsRepository) DeleteMutedMerchantAndUnmute(ctx context.Context, tenant store.Tenant, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return errors.E("postgres.transactions.delete_muted_merchant_and_unmute", "beginning transaction", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var pattern string
	if err := tx.QueryRow(ctx,
		`DELETE FROM muted_merchants WHERE id=$1 AND tenant_id = $2 RETURNING pattern`, id, tenant.ID,
	).Scan(&pattern); err != nil {
		if errorsIsNoRows(err) {
			return errors.E("store.transactions.delete_muted_merchant_and_unmute", errors.NotFound)
		}
		return errors.E("postgres.transactions.delete_muted_merchant_and_unmute", "deleting muted merchant", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE transactions
			 SET muted=false, muted_by_merchant=false, mute_reason=NULL, updated_at=NOW()
			 WHERE merchant_info ILIKE $1 AND tenant_id = $2`,
		"%"+pattern+"%", tenant.ID,
	); err != nil {
		return errors.E("postgres.transactions.delete_muted_merchant_and_unmute", "unmuting transactions", err)
	}

	return tx.Commit(ctx)
}

func (r *transactionsRepository) GetMutedMerchantPatterns(ctx context.Context, tenant store.Tenant) ([]string, error) {
	var patterns []string
	rows, err := r.pool.Query(ctx, `SELECT pattern FROM muted_merchants WHERE tenant_id = $1`, tenant.ID)
	if err != nil {
		return nil, errors.E("postgres.transactions.get_muted_merchant_patterns", "fetching muted merchant patterns", err)
	}
	defer rows.Close()
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, errors.E("postgres.transactions.get_muted_merchant_patterns", "scanning pattern", err)
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
		return errors.E("postgres.transactions.load_labels", "fetching labels", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tid, label string
		if err := rows.Scan(&tid, &label); err != nil {
			return errors.E("postgres.transactions.load_labels", "scanning label row", err)
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
