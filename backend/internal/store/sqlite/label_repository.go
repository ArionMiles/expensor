package sqlite

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

const (
	taxonomyCategory   = "category"
	taxonomyCategories = "categories"
)

func (s *Store) ListLabels(ctx context.Context, tenant store.Tenant) ([]store.Label, error) {
	rows, err := s.repositories.query.QueryContext(ctx, `SELECT name, color, created_at FROM labels WHERE tenant_id=? ORDER BY name`, tenant.ID)
	if err != nil {
		return nil, mapSQLiteError("sqlite.taxonomy.list_labels", err)
	}
	defer rows.Close()
	result := []store.Label{}
	for rows.Next() {
		var label store.Label
		var createdAt string
		if err := rows.Scan(&label.Name, &label.Color, &createdAt); err != nil {
			return nil, errors.B.Op("sqlite.taxonomy.list_labels").Text("scanning label").Err(err).Build()
		}
		label.CreatedAt, err = timeFromText(createdAt)
		if err != nil {
			return nil, err
		}
		result = append(result, label)
	}
	return result, mapSQLiteError("sqlite.taxonomy.list_labels", rows.Err())
}

func (s *Store) CreateLabel(ctx context.Context, tenant store.Tenant, name, color string) error {
	return s.repositories.writeTx(ctx, func(q queryer) error {
		_, err := q.ExecContext(ctx, `INSERT INTO labels (tenant_id, name, color, created_at) VALUES (?, ?, ?, ?)
			ON CONFLICT(tenant_id, name) WHERE tenant_id IS NOT NULL DO NOTHING`,
			tenant.ID, name, color, timeToText(s.repositories.now()))
		return mapSQLiteError("sqlite.taxonomy.create_label", err)
	})
}

func (s *Store) UpdateLabel(ctx context.Context, tenant store.Tenant, name, color string) error {
	var affected int64
	err := s.repositories.writeTx(ctx, func(q queryer) error {
		result, err := q.ExecContext(ctx, `UPDATE labels SET color=? WHERE name=? AND tenant_id=?`, color, name, tenant.ID)
		if err == nil {
			affected, err = result.RowsAffected()
		}
		return mapSQLiteError("sqlite.taxonomy.update_label", err)
	})
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.B.Op("store.taxonomy.update_label").KindNotFound().UserMsg("label not found").Build()
	}
	return nil
}

func (s *Store) DeleteLabel(ctx context.Context, tenant store.Tenant, name string, removeFromTransactions bool) error {
	return s.repositories.writeTx(ctx, func(q queryer) error {
		if removeFromTransactions {
			transactionIDs := `SELECT id FROM transactions WHERE tenant_id=?`
			if _, err := q.ExecContext(ctx, `DELETE FROM transaction_label_sources
				WHERE label=? AND transaction_id IN (`+transactionIDs+`)`, name, tenant.ID); err != nil {
				return mapSQLiteError("sqlite.taxonomy.delete_label", err)
			}
			if _, err := q.ExecContext(ctx, `DELETE FROM transaction_labels WHERE label=? AND transaction_id IN (`+transactionIDs+`)`, name, tenant.ID); err != nil {
				return mapSQLiteError("sqlite.taxonomy.delete_label", err)
			}
		}
		if _, err := q.ExecContext(ctx, `DELETE FROM label_merchants WHERE label=? AND tenant_id=?`, name, tenant.ID); err != nil {
			return mapSQLiteError("sqlite.taxonomy.delete_label", err)
		}
		_, err := q.ExecContext(ctx, `DELETE FROM labels WHERE name=? AND tenant_id=?`, name, tenant.ID)
		return mapSQLiteError("sqlite.taxonomy.delete_label", err)
	})
}

func (s *Store) ApplyLabelByMerchant(ctx context.Context, tenant store.Tenant, label, pattern string) (int64, error) {
	var added int64
	err := s.repositories.writeTx(ctx, func(q queryer) error {
		_, err := q.ExecContext(ctx, `INSERT INTO label_merchants (id, tenant_id, label, merchant_pattern, created_at)
			VALUES (?, ?, ?, ?, ?) ON CONFLICT(tenant_id, label, merchant_pattern)
			WHERE tenant_id IS NOT NULL DO NOTHING`, uuid.NewString(), tenant.ID, label, pattern, timeToText(s.repositories.now()))
		if err != nil {
			return mapSQLiteError("sqlite.taxonomy.apply_label", err)
		}
		ids, err := matchingTransactionIDs(ctx, q, tenant, pattern)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if _, err := q.ExecContext(ctx, `INSERT INTO transaction_label_sources
				(id, transaction_id, label, source_type, merchant_pattern, created_at) VALUES (?, ?, ?, 'merchant', ?, ?)
				ON CONFLICT(transaction_id, label, source_type, merchant_pattern) DO NOTHING`,
				uuid.NewString(), id, label, pattern, timeToText(s.repositories.now())); err != nil {
				return mapSQLiteError("sqlite.taxonomy.apply_label", err)
			}
			result, err := q.ExecContext(ctx, `INSERT INTO transaction_labels (id, transaction_id, label, created_at)
				VALUES (?, ?, ?, ?) ON CONFLICT(transaction_id, label) DO NOTHING`,
				uuid.NewString(), id, label, timeToText(s.repositories.now()))
			if err != nil {
				return mapSQLiteError("sqlite.taxonomy.apply_label", err)
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return mapSQLiteError("sqlite.taxonomy.apply_label", err)
			}
			added += affected
		}
		return nil
	})
	return added, err
}

func matchingTransactionIDs(ctx context.Context, q queryer, tenant store.Tenant, pattern string) ([]string, error) {
	rows, err := q.QueryContext(ctx, `SELECT id FROM transactions WHERE tenant_id=?
		AND instr(expensor_casefold(merchant_info), expensor_casefold(?)) > 0`, tenant.ID, pattern)
	if err != nil {
		return nil, mapSQLiteError("sqlite.taxonomy.matching_transactions", err)
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, errors.B.Op("sqlite.taxonomy.matching_transactions").Text("scanning transaction ID").Err(err).Build()
		}
		ids = append(ids, id)
	}
	return ids, mapSQLiteError("sqlite.taxonomy.matching_transactions", rows.Err())
}

func (s *Store) RemoveLabelByMerchant(ctx context.Context, tenant store.Tenant, label, pattern string) (int64, error) {
	var removed int64
	err := s.repositories.writeTx(ctx, func(q queryer) error {
		if _, err := q.ExecContext(ctx, `DELETE FROM label_merchants WHERE tenant_id=? AND label=? AND merchant_pattern=?`, tenant.ID, label, pattern); err != nil {
			return mapSQLiteError("sqlite.taxonomy.remove_label", err)
		}
		ids, err := merchantLabelTransactionIDs(ctx, q, tenant, label, pattern)
		if err != nil {
			return err
		}
		if err := deleteMerchantLabelSources(ctx, q, tenant, label, pattern); err != nil {
			return err
		}
		removed, err = removeOrphanedSQLiteLabels(ctx, q, ids, label)
		if err != nil {
			return err
		}
		return nil
	})
	return removed, err
}

func merchantLabelTransactionIDs(
	ctx context.Context,
	q queryer,
	tenant store.Tenant,
	label string,
	pattern string,
) (ids []string, resultErr error) {
	const operation = "sqlite.taxonomy.remove_label"

	rows, err := q.QueryContext(ctx, `SELECT DISTINCT tls.transaction_id FROM transaction_label_sources tls
		JOIN transactions t ON t.id=tls.transaction_id WHERE t.tenant_id=? AND tls.label=?
		AND tls.source_type='merchant' AND tls.merchant_pattern=?`, tenant.ID, label, pattern)
	if err != nil {
		return nil, mapSQLiteError(operation, err)
	}
	defer func() {
		resultErr = errors.Join(resultErr, mapSQLiteError(operation, rows.Close()))
	}()

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, errors.B.Op(operation).Text("scanning transaction ID").Err(err).Build()
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, mapSQLiteError(operation, err)
	}
	return ids, nil
}

func deleteMerchantLabelSources(ctx context.Context, q queryer, tenant store.Tenant, label, pattern string) error {
	_, err := q.ExecContext(ctx, `DELETE FROM transaction_label_sources WHERE label=?
		AND source_type='merchant' AND merchant_pattern=? AND transaction_id IN
		(SELECT id FROM transactions WHERE tenant_id=?)`, label, pattern, tenant.ID)
	return mapSQLiteError("sqlite.taxonomy.remove_label", err)
}

func removeOrphanedSQLiteLabels(ctx context.Context, q queryer, ids []string, label string) (int64, error) {
	var removed int64
	for _, id := range ids {
		var count int
		if err := q.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM transaction_label_sources WHERE transaction_id=? AND label=?`, id, label,
		).Scan(&count); err != nil {
			return removed, mapSQLiteError("sqlite.taxonomy.remove_label", err)
		}
		if count != 0 {
			continue
		}
		result, err := q.ExecContext(ctx, `DELETE FROM transaction_labels WHERE transaction_id=? AND label=?`, id, label)
		if err != nil {
			return removed, mapSQLiteError("sqlite.taxonomy.remove_label", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return removed, mapSQLiteError("sqlite.taxonomy.remove_label", err)
		}
		removed += affected
	}
	return removed, nil
}

func (s *Store) GetLabelMappings(ctx context.Context, tenant store.Tenant) (map[string][]string, error) {
	rows, err := s.repositories.query.QueryContext(ctx, `SELECT label, merchant_pattern FROM label_merchants
		WHERE tenant_id=? ORDER BY label, merchant_pattern`, tenant.ID)
	if err != nil {
		return nil, mapSQLiteError("sqlite.taxonomy.get_label_mappings", err)
	}
	defer rows.Close()
	result := map[string][]string{}
	for rows.Next() {
		var label, pattern string
		if err := rows.Scan(&label, &pattern); err != nil {
			return nil, errors.B.Op("sqlite.taxonomy.get_label_mappings").Text("scanning label mapping").Err(err).Build()
		}
		result[label] = append(result[label], pattern)
	}
	return result, mapSQLiteError("sqlite.taxonomy.get_label_mappings", rows.Err())
}

func (s *Store) ListCategories(ctx context.Context, tenant store.Tenant) ([]store.Category, error) {
	items, err := s.listTaxonomy(ctx, tenant, taxonomyCategories)
	if err != nil {
		return nil, err
	}
	result := make([]store.Category, len(items))
	for i, item := range items {
		result[i] = store.Category(item)
	}
	return result, nil
}

func (s *Store) ListBuckets(ctx context.Context, tenant store.Tenant) ([]store.Bucket, error) {
	items, err := s.listTaxonomy(ctx, tenant, "buckets")
	if err != nil {
		return nil, err
	}
	result := make([]store.Bucket, len(items))
	for i, item := range items {
		result[i] = store.Bucket(item)
	}
	return result, nil
}

type taxonomyItem struct {
	Name        string
	Description string
	IsDefault   bool
}

func (s *Store) listTaxonomy(ctx context.Context, tenant store.Tenant, table string) ([]taxonomyItem, error) {
	query := `SELECT name, COALESCE(description,''), is_default FROM ` + table + ` visible
		WHERE tenant_id=? OR (tenant_id IS NULL AND NOT EXISTS
		(SELECT 1 FROM ` + table + ` own WHERE own.tenant_id=? AND own.name=visible.name)) ORDER BY name`
	rows, err := s.repositories.query.QueryContext(ctx, query, tenant.ID, tenant.ID)
	if err != nil {
		return nil, mapSQLiteError("sqlite.taxonomy.list", err)
	}
	defer rows.Close()
	result := []taxonomyItem{}
	for rows.Next() {
		var item taxonomyItem
		if err := rows.Scan(&item.Name, &item.Description, &item.IsDefault); err != nil {
			return nil, errors.B.Op("sqlite.taxonomy.list").Text("scanning taxonomy item").Err(err).Build()
		}
		result = append(result, item)
	}
	return result, mapSQLiteError("sqlite.taxonomy.list", rows.Err())
}

func (s *Store) CreateCategory(ctx context.Context, tenant store.Tenant, name, description string) error {
	return s.createTaxonomy(ctx, tenant, taxonomyCategories, name, description)
}

func (s *Store) CreateBucket(ctx context.Context, tenant store.Tenant, name, description string) error {
	return s.createTaxonomy(ctx, tenant, "buckets", name, description)
}

func (s *Store) createTaxonomy(ctx context.Context, tenant store.Tenant, table, name, description string) error {
	return s.repositories.writeTx(ctx, func(q queryer) error {
		_, err := q.ExecContext(ctx, `INSERT INTO `+table+` (tenant_id, name, description, created_at)
			VALUES (?, ?, NULLIF(?,''), ?) ON CONFLICT(tenant_id, name) WHERE tenant_id IS NOT NULL DO NOTHING`,
			tenant.ID, name, description, timeToText(s.repositories.now()))
		return mapSQLiteError("sqlite.taxonomy.create", err)
	})
}

func (s *Store) DeleteCategory(ctx context.Context, tenant store.Tenant, name string, removeFromTransactions bool) error {
	return s.deleteTaxonomy(ctx, tenant, taxonomyCategory, name, removeFromTransactions)
}

func (s *Store) DeleteBucket(ctx context.Context, tenant store.Tenant, name string, removeFromTransactions bool) error {
	return s.deleteTaxonomy(ctx, tenant, "bucket", name, removeFromTransactions)
}

func (s *Store) deleteTaxonomy(ctx context.Context, tenant store.Tenant, kind, name string, removeFromTransactions bool) error {
	table := kind + "s"
	if kind == taxonomyCategory {
		table = taxonomyCategories
	}
	return s.repositories.writeTx(ctx, func(q queryer) error {
		deletion := taxonomyDelete{
			tenant:                 tenant,
			kind:                   kind,
			name:                   name,
			table:                  table,
			removeFromTransactions: removeFromTransactions,
		}
		if err := ensureTaxonomyDeletable(ctx, q, deletion); err != nil {
			return err
		}
		deletion.now = timeToText(s.repositories.now())
		if err := deleteTaxonomyReferences(ctx, q, deletion); err != nil {
			return err
		}
		_, err := q.ExecContext(ctx, `DELETE FROM `+table+` WHERE name=? AND tenant_id=?`, name, tenant.ID)
		return mapSQLiteError("sqlite.taxonomy.delete", err)
	})
}

type taxonomyDelete struct {
	tenant                 store.Tenant
	kind                   string
	name                   string
	table                  string
	now                    string
	removeFromTransactions bool
}

func ensureTaxonomyDeletable(ctx context.Context, q queryer, deletion taxonomyDelete) error {
	var isDefault bool
	err := q.QueryRowContext(ctx, `SELECT is_default FROM `+deletion.table+`
		WHERE name=? AND (tenant_id=? OR tenant_id IS NULL)
		ORDER BY tenant_id IS NULL LIMIT 1`, deletion.name, deletion.tenant.ID).Scan(&isDefault)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.B.Op("store.taxonomy.delete_" + deletion.kind).
			KindNotFound().UserMsg(deletion.kind + " not found").Build()
	}
	if err != nil {
		return mapSQLiteError("sqlite.taxonomy.delete", err)
	}
	if isDefault {
		return errors.B.Op("store.taxonomy.delete_" + deletion.kind).
			KindConflict().UserMsg("The default " + deletion.kind + " cannot be deleted.").Build()
	}
	return nil
}

func deleteTaxonomyReferences(ctx context.Context, q queryer, deletion taxonomyDelete) error {
	if deletion.kind == taxonomyCategory {
		return deleteCategoryReferences(ctx, q, deletion)
	}
	return deleteBucketReferences(ctx, q, deletion)
}

func deleteCategoryReferences(ctx context.Context, q queryer, deletion taxonomyDelete) error {
	if _, err := q.ExecContext(ctx, `DELETE FROM merchant_categories
		WHERE category=? AND bucket IS NULL AND mcc_code IS NULL AND tenant_id=?`, deletion.name, deletion.tenant.ID); err != nil {
		return mapSQLiteError("sqlite.taxonomy.delete", err)
	}
	if _, err := q.ExecContext(ctx, `UPDATE merchant_categories SET category=NULL, updated_at=?
		WHERE category=? AND tenant_id=?`, deletion.now, deletion.name, deletion.tenant.ID); err != nil {
		return mapSQLiteError("sqlite.taxonomy.delete", err)
	}
	if !deletion.removeFromTransactions {
		return nil
	}
	_, err := q.ExecContext(ctx, `UPDATE transactions SET category='', updated_at=?
		WHERE category=? AND tenant_id=?`, deletion.now, deletion.name, deletion.tenant.ID)
	return mapSQLiteError("sqlite.taxonomy.delete", err)
}

func deleteBucketReferences(ctx context.Context, q queryer, deletion taxonomyDelete) error {
	if _, err := q.ExecContext(ctx, `DELETE FROM merchant_categories
		WHERE bucket=? AND category IS NULL AND mcc_code IS NULL AND tenant_id=?`, deletion.name, deletion.tenant.ID); err != nil {
		return mapSQLiteError("sqlite.taxonomy.delete", err)
	}
	if _, err := q.ExecContext(ctx, `UPDATE merchant_categories SET bucket=NULL, updated_at=?
		WHERE bucket=? AND tenant_id=?`, deletion.now, deletion.name, deletion.tenant.ID); err != nil {
		return mapSQLiteError("sqlite.taxonomy.delete", err)
	}
	if !deletion.removeFromTransactions {
		return nil
	}
	_, err := q.ExecContext(ctx, `UPDATE transactions SET bucket='', updated_at=?
		WHERE bucket=? AND tenant_id=?`, deletion.now, deletion.name, deletion.tenant.ID)
	return mapSQLiteError("sqlite.taxonomy.delete", err)
}
