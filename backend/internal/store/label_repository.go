package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgTaxonomyRepository struct {
	pool *pgxpool.Pool
}

type taxonomyItem struct {
	Name        string
	Description string
	IsDefault   bool
}

func newPGTaxonomyRepository(deps repositoryDependencies) *pgTaxonomyRepository {
	return &pgTaxonomyRepository{
		pool: deps.pool,
	}
}

func (r *pgTaxonomyRepository) ListLabels(ctx context.Context, tenant Tenant) ([]Label, error) {
	labels := []Label{}
	rows, err := r.pool.Query(ctx,
		`SELECT name, color, created_at FROM labels WHERE tenant_id IS NOT DISTINCT FROM $1 ORDER BY name`,
		tenantIDParam(tenant),
	)
	if err != nil {
		return nil, fmt.Errorf("listing labels: querying labels: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var label Label
		if err := rows.Scan(&label.Name, &label.Color, &label.CreatedAt); err != nil {
			return nil, fmt.Errorf("listing labels: scanning label: %w", err)
		}
		labels = append(labels, label)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing labels: iterating labels: %w", err)
	}
	return labels, nil
}

func (r *pgTaxonomyRepository) CreateLabel(ctx context.Context, tenant Tenant, name, color string) error {
	_, err := r.pool.Exec(ctx,
		labelUpsertSQL(tenant),
		tenantIDParam(tenant), name, color,
	)
	if err != nil {
		return fmt.Errorf("creating label: executing label insert: %w", err)
	}
	return nil
}

func labelUpsertSQL(tenant Tenant) string {
	if tenantIDParam(tenant) == nil {
		return `INSERT INTO labels (tenant_id, name, color) VALUES ($1, $2, $3)
			ON CONFLICT (name) WHERE tenant_id IS NULL DO NOTHING`
	}
	return `INSERT INTO labels (tenant_id, name, color) VALUES ($1, $2, $3)
			ON CONFLICT (tenant_id, name) WHERE tenant_id IS NOT NULL DO NOTHING`
}

func (r *pgTaxonomyRepository) UpdateLabel(ctx context.Context, tenant Tenant, name, color string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE labels SET color = $1 WHERE name = $2 AND tenant_id IS NOT DISTINCT FROM $3`,
		color, name, tenantIDParam(tenant),
	)
	if err != nil {
		return fmt.Errorf("updating label: executing label update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return notFound("store.taxonomy.update_label")
	}
	return nil
}

func (r *pgTaxonomyRepository) DeleteLabel(ctx context.Context, tenant Tenant, name string, removeFromTransactions bool) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("deleting label: beginning delete-label transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if removeFromTransactions {
		if _, err := tx.Exec(ctx, `
			DELETE FROM transaction_label_sources tls
			USING transactions t
			WHERE tls.transaction_id = t.id AND tls.label = $1 AND t.tenant_id IS NOT DISTINCT FROM $2
		`, name, tenantIDParam(tenant)); err != nil {
			return fmt.Errorf("deleting label: deleting label sources: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM transaction_labels tl
			USING transactions t
			WHERE tl.transaction_id = t.id AND tl.label = $1 AND t.tenant_id IS NOT DISTINCT FROM $2
		`, name, tenantIDParam(tenant)); err != nil {
			return fmt.Errorf("deleting label: deleting transaction labels: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM label_merchants WHERE label = $1 AND tenant_id IS NOT DISTINCT FROM $2`, name, tenantIDParam(tenant)); err != nil {
		return fmt.Errorf("deleting label: deleting merchant mappings: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM labels WHERE name = $1 AND tenant_id IS NOT DISTINCT FROM $2`, name, tenantIDParam(tenant)); err != nil {
		return fmt.Errorf("deleting label: executing label delete: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("deleting label: committing delete-label transaction: %w", err)
	}
	return nil
}

func (r *pgTaxonomyRepository) ApplyLabelByMerchant(ctx context.Context, tenant Tenant, label, pattern string) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("beginning apply-label-by-merchant transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx,
		labelMerchantUpsertSQL(tenant),
		tenantIDParam(tenant), label, pattern,
	)
	if err != nil {
		return 0, fmt.Errorf("storing label merchant mapping: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO transaction_label_sources (transaction_id, label, source_type, merchant_pattern)
		 SELECT id, $1, 'merchant', $2
		 FROM transactions
		 WHERE merchant_info ILIKE '%' || $2 || '%'
		   AND tenant_id IS NOT DISTINCT FROM $3
		 ON CONFLICT (transaction_id, label, source_type, merchant_pattern) DO NOTHING`,
		label, pattern, tenantIDParam(tenant),
	); err != nil {
		return 0, fmt.Errorf("storing merchant label sources: %w", err)
	}

	tag, err := tx.Exec(ctx,
		`INSERT INTO transaction_labels (transaction_id, label)
		 SELECT id, $1 FROM transactions
		 WHERE merchant_info ILIKE '%' || $2 || '%'
		   AND tenant_id IS NOT DISTINCT FROM $3
		 ON CONFLICT (transaction_id, label) DO NOTHING`,
		label, pattern, tenantIDParam(tenant),
	)
	if err != nil {
		return 0, fmt.Errorf("applying label by merchant: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("committing apply-label-by-merchant transaction: %w", err)
	}
	return tag.RowsAffected(), nil
}

func labelMerchantUpsertSQL(tenant Tenant) string {
	if tenantIDParam(tenant) == nil {
		return `INSERT INTO label_merchants (tenant_id, label, merchant_pattern)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (label, merchant_pattern) WHERE tenant_id IS NULL DO NOTHING`
	}
	return `INSERT INTO label_merchants (tenant_id, label, merchant_pattern)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (tenant_id, label, merchant_pattern) WHERE tenant_id IS NOT NULL DO NOTHING`
}

func (r *pgTaxonomyRepository) RemoveLabelByMerchant(ctx context.Context, tenant Tenant, label, pattern string) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("beginning remove-label-by-merchant transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`DELETE FROM label_merchants
		 WHERE label = $1 AND merchant_pattern = $2 AND tenant_id IS NOT DISTINCT FROM $3`,
		label, pattern, tenantIDParam(tenant),
	); err != nil {
		return 0, fmt.Errorf("removing merchant label mapping: %w", err)
	}

	affectedTxnIDs, err := loadMerchantLabelTransactionIDs(ctx, tx, tenant, label, pattern)
	if err != nil {
		return 0, err
	}

	if _, err := tx.Exec(ctx,
		`DELETE FROM transaction_label_sources tls
		 USING transactions t
		 WHERE tls.transaction_id = t.id
		   AND tls.label = $1
		   AND tls.source_type = 'merchant'
		   AND tls.merchant_pattern = $2
		   AND t.tenant_id IS NOT DISTINCT FROM $3`,
		label, pattern, tenantIDParam(tenant),
	); err != nil {
		return 0, fmt.Errorf("removing merchant label sources: %w", err)
	}

	removed, err := removeOrphanedTransactionLabels(ctx, tx, affectedTxnIDs, label)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("committing remove-label-by-merchant transaction: %w", err)
	}
	return removed, nil
}

func loadMerchantLabelTransactionIDs(ctx context.Context, tx pgx.Tx, tenant Tenant, label, pattern string) ([]string, error) {
	rows, err := tx.Query(ctx, `
		SELECT DISTINCT tls.transaction_id
		FROM transaction_label_sources tls
		JOIN transactions t ON t.id = tls.transaction_id
		WHERE tls.label = $1
		  AND tls.source_type = 'merchant'
		  AND tls.merchant_pattern = $2
		  AND t.tenant_id IS NOT DISTINCT FROM $3
	`, label, pattern, tenantIDParam(tenant))
	if err != nil {
		return nil, fmt.Errorf("loading affected transactions: %w", err)
	}
	defer rows.Close()

	var transactionIDs []string
	for rows.Next() {
		var transactionID string
		if err := rows.Scan(&transactionID); err != nil {
			return nil, fmt.Errorf("scanning affected transaction: %w", err)
		}
		transactionIDs = append(transactionIDs, transactionID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating affected transactions: %w", err)
	}
	return transactionIDs, nil
}

func removeOrphanedTransactionLabels(ctx context.Context, tx pgx.Tx, transactionIDs []string, label string) (int64, error) {
	var removed int64
	for _, transactionID := range transactionIDs {
		hasSources, err := transactionLabelHasSources(ctx, tx, transactionID, label)
		if err != nil {
			return 0, err
		}
		if hasSources {
			continue
		}

		tag, err := tx.Exec(ctx,
			`DELETE FROM transaction_labels
			 WHERE transaction_id = $1 AND label = $2`,
			transactionID, label,
		)
		if err != nil {
			return 0, fmt.Errorf("removing orphaned transaction label: %w", err)
		}
		removed += tag.RowsAffected()
	}
	return removed, nil
}

func transactionLabelHasSources(ctx context.Context, tx pgx.Tx, transactionID, label string) (bool, error) {
	var remaining int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*)
		 FROM transaction_label_sources
		 WHERE transaction_id = $1 AND label = $2`,
		transactionID, label,
	).Scan(&remaining); err != nil {
		return false, fmt.Errorf("counting remaining label sources: %w", err)
	}
	return remaining > 0, nil
}

func (r *pgTaxonomyRepository) GetLabelMappings(ctx context.Context, tenant Tenant) (map[string][]string, error) {
	result := make(map[string][]string)
	rows, err := r.pool.Query(ctx, `
			SELECT label, merchant_pattern
			FROM label_merchants
			WHERE tenant_id IS NOT DISTINCT FROM $1
			ORDER BY label, merchant_pattern
		`, tenantIDParam(tenant))
	if err != nil {
		return nil, fmt.Errorf("fetching label mappings: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var label, merchantPattern string
		if err := rows.Scan(&label, &merchantPattern); err != nil {
			return nil, fmt.Errorf("scanning label mapping: %w", err)
		}
		result[label] = append(result[label], merchantPattern)
	}
	return result, rows.Err()
}

func (r *pgTaxonomyRepository) listTaxonomyItems(ctx context.Context, tenant Tenant, query, itemName string) ([]taxonomyItem, error) {
	items := []taxonomyItem{}
	rows, err := r.pool.Query(ctx, query, tenantIDParam(tenant))
	if err != nil {
		return nil, fmt.Errorf("listing %ss: %w", itemName, err)
	}
	defer rows.Close()

	for rows.Next() {
		var item taxonomyItem
		if err := rows.Scan(&item.Name, &item.Description, &item.IsDefault); err != nil {
			return nil, fmt.Errorf("scanning %s: %w", itemName, err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *pgTaxonomyRepository) ListCategories(ctx context.Context, tenant Tenant) ([]Category, error) {
	items, err := r.listTaxonomyItems(
		ctx,
		tenant,
		globalOrTenantTaxonomyQuery("categories"),
		"category",
	)
	if err != nil {
		return nil, err
	}
	cats := make([]Category, 0, len(items))
	for _, item := range items {
		cats = append(cats, Category(item))
	}
	return cats, nil
}

func (r *pgTaxonomyRepository) CreateCategory(ctx context.Context, tenant Tenant, name, description string) error {
	_, err := r.pool.Exec(ctx,
		taxonomyItemUpsertSQL(tenant, "categories"),
		tenantIDParam(tenant), name, description,
	)
	if err != nil {
		return fmt.Errorf("creating category: %w", err)
	}
	return nil
}

func (r *pgTaxonomyRepository) DeleteCategory(ctx context.Context, tenant Tenant, name string, removeFromTransactions bool) error {
	return r.deleteNamedTaxonomy(ctx, taxonomyDeleteInput{
		tenant:                 tenant,
		name:                   name,
		removeFromTransactions: removeFromTransactions,
		spec: taxonomyDeleteSpec{
			kind:                   "category",
			selectDefaultSQL:       `SELECT is_default FROM categories WHERE name = $1 AND tenant_id IS NOT DISTINCT FROM $2`,
			deleteEmptyMappingsSQL: `DELETE FROM merchant_categories WHERE category = $1 AND bucket IS NULL AND mcc_code IS NULL AND tenant_id IS NOT DISTINCT FROM $2`,
			clearMappingsSQL:       `UPDATE merchant_categories SET category = NULL, updated_at = NOW() WHERE category = $1 AND tenant_id IS NOT DISTINCT FROM $2`,
			clearTransactionsSQL:   `UPDATE transactions SET category = '', updated_at = NOW() WHERE category = $1 AND tenant_id IS NOT DISTINCT FROM $2`,
			deleteSQL:              `DELETE FROM categories WHERE name = $1 AND tenant_id IS NOT DISTINCT FROM $2`,
		},
	})
}

func (r *pgTaxonomyRepository) ListBuckets(ctx context.Context, tenant Tenant) ([]Bucket, error) {
	items, err := r.listTaxonomyItems(
		ctx,
		tenant,
		globalOrTenantTaxonomyQuery("buckets"),
		"bucket",
	)
	if err != nil {
		return nil, err
	}
	buckets := make([]Bucket, 0, len(items))
	for _, item := range items {
		buckets = append(buckets, Bucket(item))
	}
	return buckets, nil
}

func (r *pgTaxonomyRepository) CreateBucket(ctx context.Context, tenant Tenant, name, description string) error {
	_, err := r.pool.Exec(ctx,
		taxonomyItemUpsertSQL(tenant, "buckets"),
		tenantIDParam(tenant), name, description,
	)
	if err != nil {
		return fmt.Errorf("creating bucket: %w", err)
	}
	return nil
}

func (r *pgTaxonomyRepository) DeleteBucket(ctx context.Context, tenant Tenant, name string, removeFromTransactions bool) error {
	return r.deleteNamedTaxonomy(ctx, taxonomyDeleteInput{
		tenant:                 tenant,
		name:                   name,
		removeFromTransactions: removeFromTransactions,
		spec: taxonomyDeleteSpec{
			kind:                   "bucket",
			selectDefaultSQL:       `SELECT is_default FROM buckets WHERE name = $1 AND tenant_id IS NOT DISTINCT FROM $2`,
			deleteEmptyMappingsSQL: `DELETE FROM merchant_categories WHERE bucket = $1 AND category IS NULL AND mcc_code IS NULL AND tenant_id IS NOT DISTINCT FROM $2`,
			clearMappingsSQL:       `UPDATE merchant_categories SET bucket = NULL, updated_at = NOW() WHERE bucket = $1 AND tenant_id IS NOT DISTINCT FROM $2`,
			clearTransactionsSQL:   `UPDATE transactions SET bucket = '', updated_at = NOW() WHERE bucket = $1 AND tenant_id IS NOT DISTINCT FROM $2`,
			deleteSQL:              `DELETE FROM buckets WHERE name = $1 AND tenant_id IS NOT DISTINCT FROM $2`,
		},
	})
}

func taxonomyItemUpsertSQL(tenant Tenant, table string) string {
	if tenantIDParam(tenant) == nil {
		return fmt.Sprintf(`INSERT INTO %s (tenant_id, name, description) VALUES ($1, $2, NULLIF($3,''))
		 ON CONFLICT (name) WHERE tenant_id IS NULL DO NOTHING`, table)
	}
	return fmt.Sprintf(`INSERT INTO %s (tenant_id, name, description) VALUES ($1, $2, NULLIF($3,''))
		 ON CONFLICT (tenant_id, name) WHERE tenant_id IS NOT NULL DO NOTHING`, table)
}

func globalOrTenantTaxonomyQuery(table string) string {
	return fmt.Sprintf(`
		SELECT DISTINCT ON (name) name, COALESCE(description,''), is_default
		FROM %s
		WHERE tenant_id IS NULL OR tenant_id IS NOT DISTINCT FROM $1
		ORDER BY name, (tenant_id IS NOT DISTINCT FROM $1) DESC
	`, table)
}

type taxonomyDeleteSpec struct {
	kind                   string
	selectDefaultSQL       string
	deleteEmptyMappingsSQL string
	clearMappingsSQL       string
	clearTransactionsSQL   string
	deleteSQL              string
}

type taxonomyDeleteInput struct {
	tenant                 Tenant
	name                   string
	removeFromTransactions bool
	spec                   taxonomyDeleteSpec
}

func (r *pgTaxonomyRepository) deleteNamedTaxonomy(
	ctx context.Context,
	input taxonomyDeleteInput,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning delete %s transaction: %w", input.spec.kind, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ensureTaxonomyCanBeDeleted(ctx, tx, input.tenant, input.name, input.spec); err != nil {
		return err
	}
	if err := execTaxonomyDelete(ctx, tx, input); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing delete %s transaction: %w", input.spec.kind, err)
	}
	return nil
}

func ensureTaxonomyCanBeDeleted(ctx context.Context, tx pgx.Tx, tenant Tenant, name string, spec taxonomyDeleteSpec) error {
	var isDefault bool
	err := tx.QueryRow(ctx, spec.selectDefaultSQL, name, tenantIDParam(tenant)).Scan(&isDefault)
	if err != nil {
		return notFound("store.taxonomy.delete_" + spec.kind)
	}
	if isDefault {
		return fmt.Errorf("cannot delete default %s %q", spec.kind, name)
	}
	return nil
}

func execTaxonomyDelete(ctx context.Context, tx pgx.Tx, input taxonomyDeleteInput) error {
	tenantParam := tenantIDParam(input.tenant)
	name := input.name
	spec := input.spec
	if _, err := tx.Exec(ctx, spec.deleteEmptyMappingsSQL, name, tenantParam); err != nil {
		return fmt.Errorf("deleting %s merchant mappings: %w", spec.kind, err)
	}
	if _, err := tx.Exec(ctx, spec.clearMappingsSQL, name, tenantParam); err != nil {
		return fmt.Errorf("clearing %s merchant mappings: %w", spec.kind, err)
	}
	if input.removeFromTransactions {
		if _, err := tx.Exec(ctx, spec.clearTransactionsSQL, name, tenantParam); err != nil {
			return fmt.Errorf("clearing %s from transactions: %w", spec.kind, err)
		}
	}
	if _, err := tx.Exec(ctx, spec.deleteSQL, name, tenantParam); err != nil {
		return fmt.Errorf("deleting %s: %w", spec.kind, err)
	}
	return nil
}
