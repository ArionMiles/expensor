package store

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TaxonomyRepository owns label, category, and bucket taxonomy persistence.
type TaxonomyRepository interface {
	InitLabels(ctx context.Context) error
	InitCategoriesBuckets(ctx context.Context) error
	ListLabels(ctx context.Context) ([]Label, error)
	CreateLabel(ctx context.Context, name, color string) error
	UpdateLabel(ctx context.Context, name, color string) error
	DeleteLabel(ctx context.Context, name string, removeFromTransactions bool) error
	ApplyLabelByMerchant(ctx context.Context, label, pattern string) (int64, error)
	RemoveLabelByMerchant(ctx context.Context, label, pattern string) (int64, error)
	GetLabelMappings(ctx context.Context) (map[string][]string, error)
	ListCategories(ctx context.Context) ([]Category, error)
	CreateCategory(ctx context.Context, name, description string) error
	DeleteCategory(ctx context.Context, name string, removeFromTransactions bool) error
	ListBuckets(ctx context.Context) ([]Bucket, error)
	CreateBucket(ctx context.Context, name, description string) error
	DeleteBucket(ctx context.Context, name string, removeFromTransactions bool) error

	// Compatibility methods retained for label repository callers.
	List(ctx context.Context) ([]Label, error)
	Create(ctx context.Context, name, color string) error
	Update(ctx context.Context, name, color string) error
	Delete(ctx context.Context, name string) error
}

// LabelRepository is retained as a compatibility alias while the taxonomy repository grows.
type LabelRepository = TaxonomyRepository

type pgTaxonomyRepository struct {
	pool *pgxpool.Pool
}

type taxonomyItem struct {
	Name        string
	Description string
	IsDefault   bool
}

// NewTaxonomyRepository returns a PostgreSQL-backed taxonomy repository.
func NewTaxonomyRepository(deps repositoryDependencies) TaxonomyRepository {
	return &pgTaxonomyRepository{
		pool: deps.pool,
	}
}

// NewLabelRepository returns a PostgreSQL-backed label repository.
func NewLabelRepository(pool *pgxpool.Pool, logger *slog.Logger) LabelRepository {
	_ = logger
	return &pgTaxonomyRepository{
		pool: pool,
	}
}

func (r *pgTaxonomyRepository) List(ctx context.Context) ([]Label, error) {
	return r.ListLabels(ctx)
}

func (r *pgTaxonomyRepository) Create(ctx context.Context, name, color string) error {
	return r.CreateLabel(ctx, name, color)
}

func (r *pgTaxonomyRepository) Update(ctx context.Context, name, color string) error {
	return r.UpdateLabel(ctx, name, color)
}

func (r *pgTaxonomyRepository) Delete(ctx context.Context, name string) error {
	return r.DeleteLabel(ctx, name, false)
}

func (r *pgTaxonomyRepository) InitLabels(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
			CREATE TABLE IF NOT EXISTS labels (
				name        TEXT PRIMARY KEY,
				color       TEXT NOT NULL DEFAULT '#6366f1',
				created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
			);
			CREATE TABLE IF NOT EXISTS label_merchants (
				id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				label            TEXT NOT NULL REFERENCES labels(name) ON DELETE CASCADE,
				merchant_pattern TEXT NOT NULL,
				created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				UNIQUE(label, merchant_pattern)
			);
			CREATE TABLE IF NOT EXISTS transaction_label_sources (
				id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				transaction_id   UUID NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
				label            TEXT NOT NULL,
				source_type      TEXT NOT NULL,
				merchant_pattern TEXT NOT NULL DEFAULT '',
				created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				CHECK (source_type IN ('manual', 'merchant')),
				UNIQUE(transaction_id, label, source_type, merchant_pattern)
			);
		`)
	if err != nil {
		return fmt.Errorf("initializing labels: executing labels initialization: %w", err)
	}
	return nil
}

func (r *pgTaxonomyRepository) InitCategoriesBuckets(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
			CREATE TABLE IF NOT EXISTS categories (
				name        TEXT PRIMARY KEY,
				description TEXT,
				is_default  BOOLEAN NOT NULL DEFAULT false,
				created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
			);
			CREATE TABLE IF NOT EXISTS buckets (
				name        TEXT PRIMARY KEY,
				description TEXT,
				is_default  BOOLEAN NOT NULL DEFAULT false,
				created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
			);
			INSERT INTO categories (name, is_default) VALUES
				('Food & Dining', true),('Transport', true),('Shopping', true),
				('Utilities', true),('Healthcare', true),('Entertainment', true),
				('Travel', true),('Finance', true)
			ON CONFLICT (name) DO NOTHING;
			INSERT INTO buckets (name, is_default) VALUES
				('Needs', true),('Wants', true),('Investments', true),('Income', true)
			ON CONFLICT (name) DO NOTHING;
		`)
	if err != nil {
		return fmt.Errorf("initializing categories and buckets: executing categories and buckets initialization: %w", err)
	}
	return nil
}

func (r *pgTaxonomyRepository) ListLabels(ctx context.Context) ([]Label, error) {
	labels := []Label{}
	rows, err := r.pool.Query(ctx, `SELECT name, color, created_at FROM labels ORDER BY name`)
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

func (r *pgTaxonomyRepository) CreateLabel(ctx context.Context, name, color string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO labels (name, color) VALUES ($1, $2) ON CONFLICT (name) DO NOTHING`,
		name, color,
	)
	if err != nil {
		return fmt.Errorf("creating label: executing label insert: %w", err)
	}
	return nil
}

func (r *pgTaxonomyRepository) UpdateLabel(ctx context.Context, name, color string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE labels SET color = $1 WHERE name = $2`, color, name)
	if err != nil {
		return fmt.Errorf("updating label: executing label update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("updating label: %w", ErrNotFound)
	}
	return nil
}

func (r *pgTaxonomyRepository) DeleteLabel(ctx context.Context, name string, removeFromTransactions bool) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("deleting label: beginning delete-label transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if removeFromTransactions {
		if _, err := tx.Exec(ctx, `DELETE FROM transaction_label_sources WHERE label = $1`, name); err != nil {
			return fmt.Errorf("deleting label: deleting label sources: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM transaction_labels WHERE label = $1`, name); err != nil {
			return fmt.Errorf("deleting label: deleting transaction labels: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM labels WHERE name = $1`, name); err != nil {
		return fmt.Errorf("deleting label: executing label delete: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("deleting label: committing delete-label transaction: %w", err)
	}
	return nil
}

func (r *pgTaxonomyRepository) ApplyLabelByMerchant(ctx context.Context, label, pattern string) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("beginning apply-label-by-merchant transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx,
		`INSERT INTO label_merchants (label, merchant_pattern)
		 VALUES ($1, $2)
		 ON CONFLICT (label, merchant_pattern) DO NOTHING`,
		label, pattern,
	)
	if err != nil {
		return 0, fmt.Errorf("storing label merchant mapping: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO transaction_label_sources (transaction_id, label, source_type, merchant_pattern)
		 SELECT id, $1, 'merchant', $2
		 FROM transactions
		 WHERE merchant_info ILIKE '%' || $2 || '%'
		 ON CONFLICT (transaction_id, label, source_type, merchant_pattern) DO NOTHING`,
		label, pattern,
	); err != nil {
		return 0, fmt.Errorf("storing merchant label sources: %w", err)
	}

	tag, err := tx.Exec(ctx,
		`INSERT INTO transaction_labels (transaction_id, label)
		 SELECT id, $1 FROM transactions
		 WHERE merchant_info ILIKE '%' || $2 || '%'
		 ON CONFLICT (transaction_id, label) DO NOTHING`,
		label, pattern,
	)
	if err != nil {
		return 0, fmt.Errorf("applying label by merchant: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("committing apply-label-by-merchant transaction: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *pgTaxonomyRepository) RemoveLabelByMerchant(ctx context.Context, label, pattern string) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("beginning remove-label-by-merchant transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`DELETE FROM label_merchants
		 WHERE label = $1 AND merchant_pattern = $2`,
		label, pattern,
	); err != nil {
		return 0, fmt.Errorf("removing merchant label mapping: %w", err)
	}

	affectedTxnIDs, err := loadMerchantLabelTransactionIDs(ctx, tx, label, pattern)
	if err != nil {
		return 0, err
	}

	if _, err := tx.Exec(ctx,
		`DELETE FROM transaction_label_sources
		 WHERE label = $1
		   AND source_type = 'merchant'
		   AND merchant_pattern = $2`,
		label, pattern,
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

func loadMerchantLabelTransactionIDs(ctx context.Context, tx pgx.Tx, label, pattern string) ([]string, error) {
	rows, err := tx.Query(ctx, `
		SELECT DISTINCT transaction_id
		FROM transaction_label_sources
		WHERE label = $1
		  AND source_type = 'merchant'
		  AND merchant_pattern = $2
	`, label, pattern)
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

func (r *pgTaxonomyRepository) GetLabelMappings(ctx context.Context) (map[string][]string, error) {
	result := make(map[string][]string)
	rows, err := r.pool.Query(ctx, `
			SELECT label, merchant_pattern
			FROM label_merchants
			ORDER BY label, merchant_pattern
		`)
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

func (r *pgTaxonomyRepository) listTaxonomyItems(ctx context.Context, query, itemName string) ([]taxonomyItem, error) {
	items := []taxonomyItem{}
	rows, err := r.pool.Query(ctx, query)
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

func (r *pgTaxonomyRepository) ListCategories(ctx context.Context) ([]Category, error) {
	items, err := r.listTaxonomyItems(
		ctx,
		`SELECT name, COALESCE(description,''), is_default FROM categories ORDER BY name`,
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

func (r *pgTaxonomyRepository) CreateCategory(ctx context.Context, name, description string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO categories (name, description) VALUES ($1, NULLIF($2,''))
		 ON CONFLICT (name) DO NOTHING`,
		name, description,
	)
	if err != nil {
		return fmt.Errorf("creating category: %w", err)
	}
	return nil
}

func (r *pgTaxonomyRepository) DeleteCategory(ctx context.Context, name string, removeFromTransactions bool) error {
	return r.deleteNamedTaxonomy(ctx, name, removeFromTransactions, taxonomyDeleteSpec{
		kind:                   "category",
		selectDefaultSQL:       `SELECT is_default FROM categories WHERE name = $1`,
		deleteEmptyMappingsSQL: `DELETE FROM merchant_categories WHERE category = $1 AND bucket IS NULL AND mcc_code IS NULL`,
		clearMappingsSQL:       `UPDATE merchant_categories SET category = NULL, updated_at = NOW() WHERE category = $1`,
		clearTransactionsSQL:   `UPDATE transactions SET category = '', updated_at = NOW() WHERE category = $1`,
		deleteSQL:              `DELETE FROM categories WHERE name = $1`,
	})
}

func (r *pgTaxonomyRepository) ListBuckets(ctx context.Context) ([]Bucket, error) {
	items, err := r.listTaxonomyItems(
		ctx,
		`SELECT name, COALESCE(description,''), is_default FROM buckets ORDER BY name`,
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

func (r *pgTaxonomyRepository) CreateBucket(ctx context.Context, name, description string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO buckets (name, description) VALUES ($1, NULLIF($2,''))
		 ON CONFLICT (name) DO NOTHING`,
		name, description,
	)
	if err != nil {
		return fmt.Errorf("creating bucket: %w", err)
	}
	return nil
}

func (r *pgTaxonomyRepository) DeleteBucket(ctx context.Context, name string, removeFromTransactions bool) error {
	return r.deleteNamedTaxonomy(ctx, name, removeFromTransactions, taxonomyDeleteSpec{
		kind:                   "bucket",
		selectDefaultSQL:       `SELECT is_default FROM buckets WHERE name = $1`,
		deleteEmptyMappingsSQL: `DELETE FROM merchant_categories WHERE bucket = $1 AND category IS NULL AND mcc_code IS NULL`,
		clearMappingsSQL:       `UPDATE merchant_categories SET bucket = NULL, updated_at = NOW() WHERE bucket = $1`,
		clearTransactionsSQL:   `UPDATE transactions SET bucket = '', updated_at = NOW() WHERE bucket = $1`,
		deleteSQL:              `DELETE FROM buckets WHERE name = $1`,
	})
}

type taxonomyDeleteSpec struct {
	kind                   string
	selectDefaultSQL       string
	deleteEmptyMappingsSQL string
	clearMappingsSQL       string
	clearTransactionsSQL   string
	deleteSQL              string
}

func (r *pgTaxonomyRepository) deleteNamedTaxonomy(ctx context.Context, name string, removeFromTransactions bool, spec taxonomyDeleteSpec) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning delete %s transaction: %w", spec.kind, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ensureTaxonomyCanBeDeleted(ctx, tx, name, spec); err != nil {
		return err
	}
	if err := execTaxonomyDelete(ctx, tx, name, removeFromTransactions, spec); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing delete %s transaction: %w", spec.kind, err)
	}
	return nil
}

func ensureTaxonomyCanBeDeleted(ctx context.Context, tx pgx.Tx, name string, spec taxonomyDeleteSpec) error {
	var isDefault bool
	err := tx.QueryRow(ctx, spec.selectDefaultSQL, name).Scan(&isDefault)
	if err != nil {
		return ErrNotFound
	}
	if isDefault {
		return fmt.Errorf("cannot delete default %s %q", spec.kind, name)
	}
	return nil
}

func execTaxonomyDelete(ctx context.Context, tx pgx.Tx, name string, removeFromTransactions bool, spec taxonomyDeleteSpec) error {
	if _, err := tx.Exec(ctx, spec.deleteEmptyMappingsSQL, name); err != nil {
		return fmt.Errorf("deleting %s merchant mappings: %w", spec.kind, err)
	}
	if _, err := tx.Exec(ctx, spec.clearMappingsSQL, name); err != nil {
		return fmt.Errorf("clearing %s merchant mappings: %w", spec.kind, err)
	}
	if removeFromTransactions {
		if _, err := tx.Exec(ctx, spec.clearTransactionsSQL, name); err != nil {
			return fmt.Errorf("clearing %s from transactions: %w", spec.kind, err)
		}
	}
	if _, err := tx.Exec(ctx, spec.deleteSQL, name); err != nil {
		return fmt.Errorf("deleting %s: %w", spec.kind, err)
	}
	return nil
}
