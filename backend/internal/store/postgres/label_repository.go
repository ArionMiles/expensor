package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

type taxonomyRepository struct {
	pool *pgxpool.Pool
}

type taxonomyItem struct {
	Name        string
	Description string
	IsDefault   bool
}

func newTaxonomyRepository(deps repositoryDependencies) *taxonomyRepository {
	return &taxonomyRepository{
		pool: deps.pool,
	}
}

func (r *taxonomyRepository) ListLabels(ctx context.Context, tenant store.Tenant) ([]store.Label, error) {
	labels := []store.Label{}
	rows, err := r.pool.Query(ctx,
		`SELECT name, color, created_at FROM labels WHERE tenant_id = $1 ORDER BY name`,
		tenant.ID,
	)
	if err != nil {
		return nil, errors.E("postgres.label.list_labels", "listing labels: querying labels", err)
	}
	defer rows.Close()

	for rows.Next() {
		var label store.Label
		if err := rows.Scan(&label.Name, &label.Color, &label.CreatedAt); err != nil {
			return nil, errors.E("postgres.label.list_labels", "listing labels: scanning label", err)
		}
		labels = append(labels, label)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.E("postgres.label.list_labels", "listing labels: iterating labels", err)
	}
	return labels, nil
}

func (r *taxonomyRepository) CreateLabel(ctx context.Context, tenant store.Tenant, name, color string) error {
	_, err := r.pool.Exec(ctx,
		labelUpsertSQL,
		tenant.ID, name, color,
	)
	if err != nil {
		return errors.E("postgres.label.create_label", "creating label: executing label insert", err)
	}
	return nil
}

const labelUpsertSQL = `
	INSERT INTO labels (tenant_id, name, color)
	VALUES ($1, $2, $3)
	ON CONFLICT (tenant_id, name) WHERE tenant_id IS NOT NULL
	DO NOTHING
`

func (r *taxonomyRepository) UpdateLabel(ctx context.Context, tenant store.Tenant, name, color string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE labels SET color = $1 WHERE name = $2 AND tenant_id = $3`,
		color, name, tenant.ID,
	)
	if err != nil {
		return errors.E("postgres.label.update_label", "updating label: executing label update", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.E("store.taxonomy.update_label", errors.NotFound, errors.User("label not found"))
	}
	return nil
}

func (r *taxonomyRepository) DeleteLabel(ctx context.Context, tenant store.Tenant, name string, removeFromTransactions bool) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return errors.E("postgres.label.delete_label", "deleting label: beginning delete-label transaction", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if removeFromTransactions {
		if _, err := tx.Exec(ctx, `
			DELETE FROM transaction_label_sources tls
			USING transactions t
			WHERE tls.transaction_id = t.id AND tls.label = $1 AND t.tenant_id = $2
		`, name, tenant.ID); err != nil {
			return errors.E("postgres.label.delete_label", "deleting label: deleting label sources", err)
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM transaction_labels tl
			USING transactions t
			WHERE tl.transaction_id = t.id AND tl.label = $1 AND t.tenant_id = $2
		`, name, tenant.ID); err != nil {
			return errors.E("postgres.label.delete_label", "deleting label: deleting transaction labels", err)
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM label_merchants WHERE label = $1 AND tenant_id = $2`, name, tenant.ID); err != nil {
		return errors.E("postgres.label.delete_label", "deleting label: deleting merchant mappings", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM labels WHERE name = $1 AND tenant_id = $2`, name, tenant.ID); err != nil {
		return errors.E("postgres.label.delete_label", "deleting label: executing label delete", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return errors.E("postgres.label.delete_label", "deleting label: committing delete-label transaction", err)
	}
	return nil
}

func (r *taxonomyRepository) ApplyLabelByMerchant(ctx context.Context, tenant store.Tenant, label, pattern string) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, errors.E("postgres.label.apply_label_by_merchant", "beginning apply-label-by-merchant transaction", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx,
		labelMerchantUpsertSQL,
		tenant.ID, label, pattern,
	)
	if err != nil {
		return 0, errors.E("postgres.label.apply_label_by_merchant", "storing label merchant mapping", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO transaction_label_sources (transaction_id, label, source_type, merchant_pattern)
		 SELECT id, $1, 'merchant', $2
		 FROM transactions
		 WHERE merchant_info ILIKE '%' || $2 || '%'
		   AND tenant_id = $3
		 ON CONFLICT (transaction_id, label, source_type, merchant_pattern) DO NOTHING`,
		label, pattern, tenant.ID,
	); err != nil {
		return 0, errors.E("postgres.label.apply_label_by_merchant", "storing merchant label sources", err)
	}

	tag, err := tx.Exec(ctx,
		`INSERT INTO transaction_labels (transaction_id, label)
		 SELECT id, $1 FROM transactions
		 WHERE merchant_info ILIKE '%' || $2 || '%'
		   AND tenant_id = $3
		 ON CONFLICT (transaction_id, label) DO NOTHING`,
		label, pattern, tenant.ID,
	)
	if err != nil {
		return 0, errors.E("postgres.label.apply_label_by_merchant", "applying label by merchant", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, errors.E("postgres.label.apply_label_by_merchant", "committing apply-label-by-merchant transaction", err)
	}
	return tag.RowsAffected(), nil
}

const labelMerchantUpsertSQL = `
	INSERT INTO label_merchants (tenant_id, label, merchant_pattern)
	VALUES ($1, $2, $3)
	ON CONFLICT (tenant_id, label, merchant_pattern) WHERE tenant_id IS NOT NULL
	DO NOTHING
`

func (r *taxonomyRepository) RemoveLabelByMerchant(ctx context.Context, tenant store.Tenant, label, pattern string) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, errors.E("postgres.label.remove_label_by_merchant", "beginning remove-label-by-merchant transaction", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`DELETE FROM label_merchants
		 WHERE label = $1 AND merchant_pattern = $2 AND tenant_id = $3`,
		label, pattern, tenant.ID,
	); err != nil {
		return 0, errors.E("postgres.label.remove_label_by_merchant", "removing merchant label mapping", err)
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
		   AND t.tenant_id = $3`,
		label, pattern, tenant.ID,
	); err != nil {
		return 0, errors.E("postgres.label.remove_label_by_merchant", "removing merchant label sources", err)
	}

	removed, err := removeOrphanedTransactionLabels(ctx, tx, affectedTxnIDs, label)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, errors.E("postgres.label.remove_label_by_merchant", "committing remove-label-by-merchant transaction", err)
	}
	return removed, nil
}

func loadMerchantLabelTransactionIDs(ctx context.Context, tx pgx.Tx, tenant store.Tenant, label, pattern string) ([]string, error) {
	rows, err := tx.Query(ctx, `
		SELECT DISTINCT tls.transaction_id
		FROM transaction_label_sources tls
		JOIN transactions t ON t.id = tls.transaction_id
		WHERE tls.label = $1
		  AND tls.source_type = 'merchant'
		  AND tls.merchant_pattern = $2
		  AND t.tenant_id = $3
	`, label, pattern, tenant.ID)
	if err != nil {
		return nil, errors.E("postgres.label.load_merchant_label_transaction_i_ds", "loading affected transactions", err)
	}
	defer rows.Close()

	var transactionIDs []string
	for rows.Next() {
		var transactionID string
		if err := rows.Scan(&transactionID); err != nil {
			return nil, errors.E("postgres.label.load_merchant_label_transaction_i_ds", "scanning affected transaction", err)
		}
		transactionIDs = append(transactionIDs, transactionID)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.E("postgres.label.load_merchant_label_transaction_i_ds", "iterating affected transactions", err)
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
			return 0, errors.E("postgres.label.remove_orphaned_transaction_labels", "removing orphaned transaction label", err)
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
		return false, errors.E("postgres.label.transaction_label_has_sources", "counting remaining label sources", err)
	}
	return remaining > 0, nil
}

func (r *taxonomyRepository) GetLabelMappings(ctx context.Context, tenant store.Tenant) (map[string][]string, error) {
	result := make(map[string][]string)
	rows, err := r.pool.Query(ctx, `
			SELECT label, merchant_pattern
			FROM label_merchants
			WHERE tenant_id = $1
			ORDER BY label, merchant_pattern
		`, tenant.ID)
	if err != nil {
		return nil, errors.E("postgres.label.get_label_mappings", "fetching label mappings", err)
	}
	defer rows.Close()

	for rows.Next() {
		var label, merchantPattern string
		if err := rows.Scan(&label, &merchantPattern); err != nil {
			return nil, errors.E("postgres.label.get_label_mappings", "scanning label mapping", err)
		}
		result[label] = append(result[label], merchantPattern)
	}
	return result, rows.Err()
}

func (r *taxonomyRepository) listTaxonomyItems(ctx context.Context, tenant store.Tenant, query, itemName string) ([]taxonomyItem, error) {
	items := []taxonomyItem{}
	rows, err := r.pool.Query(ctx, query, tenant.ID)
	if err != nil {
		return nil, errors.E("postgres.label.list_taxonomy_items", fmt.Sprintf("listing %ss", itemName), err)
	}
	defer rows.Close()

	for rows.Next() {
		var item taxonomyItem
		if err := rows.Scan(&item.Name, &item.Description, &item.IsDefault); err != nil {
			return nil, errors.E("postgres.label.list_taxonomy_items", fmt.Sprintf("scanning %s", itemName), err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *taxonomyRepository) ListCategories(ctx context.Context, tenant store.Tenant) ([]store.Category, error) {
	items, err := r.listTaxonomyItems(
		ctx,
		tenant,
		globalOrTenantTaxonomyQuery("categories"),
		"category",
	)
	if err != nil {
		return nil, err
	}
	cats := make([]store.Category, 0, len(items))
	for _, item := range items {
		cats = append(cats, store.Category(item))
	}
	return cats, nil
}

func (r *taxonomyRepository) CreateCategory(ctx context.Context, tenant store.Tenant, name, description string) error {
	_, err := r.pool.Exec(ctx,
		taxonomyItemUpsertSQL("categories"),
		tenant.ID, name, description,
	)
	if err != nil {
		return errors.E("postgres.label.create_category", "creating category", err)
	}
	return nil
}

func (r *taxonomyRepository) DeleteCategory(ctx context.Context, tenant store.Tenant, name string, removeFromTransactions bool) error {
	return r.deleteNamedTaxonomy(ctx, taxonomyDeleteInput{
		tenant:                 tenant,
		name:                   name,
		removeFromTransactions: removeFromTransactions,
		spec: taxonomyDeleteSpec{
			kind:                   "category",
			selectDefaultSQL:       `SELECT is_default FROM categories WHERE name = $1 AND tenant_id = $2`,
			deleteEmptyMappingsSQL: `DELETE FROM merchant_categories WHERE category = $1 AND bucket IS NULL AND mcc_code IS NULL AND tenant_id = $2`,
			clearMappingsSQL:       `UPDATE merchant_categories SET category = NULL, updated_at = NOW() WHERE category = $1 AND tenant_id = $2`,
			clearTransactionsSQL:   `UPDATE transactions SET category = '', updated_at = NOW() WHERE category = $1 AND tenant_id = $2`,
			deleteSQL:              `DELETE FROM categories WHERE name = $1 AND tenant_id = $2`,
		},
	})
}

func (r *taxonomyRepository) ListBuckets(ctx context.Context, tenant store.Tenant) ([]store.Bucket, error) {
	items, err := r.listTaxonomyItems(
		ctx,
		tenant,
		globalOrTenantTaxonomyQuery("buckets"),
		"bucket",
	)
	if err != nil {
		return nil, err
	}
	buckets := make([]store.Bucket, 0, len(items))
	for _, item := range items {
		buckets = append(buckets, store.Bucket(item))
	}
	return buckets, nil
}

func (r *taxonomyRepository) CreateBucket(ctx context.Context, tenant store.Tenant, name, description string) error {
	_, err := r.pool.Exec(ctx,
		taxonomyItemUpsertSQL("buckets"),
		tenant.ID, name, description,
	)
	if err != nil {
		return errors.E("postgres.label.create_bucket", "creating bucket", err)
	}
	return nil
}

func (r *taxonomyRepository) DeleteBucket(ctx context.Context, tenant store.Tenant, name string, removeFromTransactions bool) error {
	return r.deleteNamedTaxonomy(ctx, taxonomyDeleteInput{
		tenant:                 tenant,
		name:                   name,
		removeFromTransactions: removeFromTransactions,
		spec: taxonomyDeleteSpec{
			kind:                   "bucket",
			selectDefaultSQL:       `SELECT is_default FROM buckets WHERE name = $1 AND tenant_id = $2`,
			deleteEmptyMappingsSQL: `DELETE FROM merchant_categories WHERE bucket = $1 AND category IS NULL AND mcc_code IS NULL AND tenant_id = $2`,
			clearMappingsSQL:       `UPDATE merchant_categories SET bucket = NULL, updated_at = NOW() WHERE bucket = $1 AND tenant_id = $2`,
			clearTransactionsSQL:   `UPDATE transactions SET bucket = '', updated_at = NOW() WHERE bucket = $1 AND tenant_id = $2`,
			deleteSQL:              `DELETE FROM buckets WHERE name = $1 AND tenant_id = $2`,
		},
	})
}

func taxonomyItemUpsertSQL(table string) string {
	return fmt.Sprintf(`INSERT INTO %s (tenant_id, name, description) VALUES ($1, $2, NULLIF($3,''))
		 ON CONFLICT (tenant_id, name) WHERE tenant_id IS NOT NULL DO NOTHING`, table)
}

func globalOrTenantTaxonomyQuery(table string) string {
	return fmt.Sprintf(`
		SELECT DISTINCT ON (name) name, COALESCE(description,''), is_default
		FROM %s
		WHERE tenant_id IS NULL OR tenant_id = $1
		ORDER BY name, (tenant_id = $1) DESC
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
	tenant                 store.Tenant
	name                   string
	removeFromTransactions bool
	spec                   taxonomyDeleteSpec
}

func (r *taxonomyRepository) deleteNamedTaxonomy(
	ctx context.Context,
	input taxonomyDeleteInput,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return errors.E("postgres.label.delete_named_taxonomy", fmt.Sprintf("beginning delete %s transaction", input.spec.kind), err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ensureTaxonomyCanBeDeleted(ctx, tx, input.tenant, input.name, input.spec); err != nil {
		return err
	}
	if err := execTaxonomyDelete(ctx, tx, input); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return errors.E("postgres.label.delete_named_taxonomy", fmt.Sprintf("committing delete %s transaction", input.spec.kind), err)
	}
	return nil
}

func ensureTaxonomyCanBeDeleted(ctx context.Context, tx pgx.Tx, tenant store.Tenant, name string, spec taxonomyDeleteSpec) error {
	var isDefault bool
	err := tx.QueryRow(ctx, spec.selectDefaultSQL, name, tenant.ID).Scan(&isDefault)
	if err != nil {
		return errors.E(
			"store.taxonomy.delete_"+spec.kind,
			errors.NotFound,
			errors.User(spec.kind+" not found"),
			err,
		)
	}
	if isDefault {
		return errors.E(
			"store.taxonomy.delete_"+spec.kind,
			errors.Conflict,
			errors.User("The default "+spec.kind+" cannot be deleted."),
			fmt.Sprintf("cannot delete default %s %q", spec.kind, name),
		)
	}
	return nil
}

func execTaxonomyDelete(ctx context.Context, tx pgx.Tx, input taxonomyDeleteInput) error {
	tenantParam := input.tenant.ID
	name := input.name
	spec := input.spec
	if _, err := tx.Exec(ctx, spec.deleteEmptyMappingsSQL, name, tenantParam); err != nil {
		return errors.E("postgres.label.exec_taxonomy_delete", fmt.Sprintf("deleting %s merchant mappings", spec.kind), err)
	}
	if _, err := tx.Exec(ctx, spec.clearMappingsSQL, name, tenantParam); err != nil {
		return errors.E("postgres.label.exec_taxonomy_delete", fmt.Sprintf("clearing %s merchant mappings", spec.kind), err)
	}
	if input.removeFromTransactions {
		if _, err := tx.Exec(ctx, spec.clearTransactionsSQL, name, tenantParam); err != nil {
			return errors.E("postgres.label.exec_taxonomy_delete", fmt.Sprintf("clearing %s from transactions", spec.kind), err)
		}
	}
	if _, err := tx.Exec(ctx, spec.deleteSQL, name, tenantParam); err != nil {
		return errors.E("postgres.label.exec_taxonomy_delete", fmt.Sprintf("deleting %s", spec.kind), err)
	}
	return nil
}
