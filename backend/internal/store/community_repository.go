package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArionMiles/expensor/backend/pkg/api"
)

type CommunityRepository interface {
	CategorizeMerchant(ctx context.Context, merchant, category, bucket string) (int, error)
	ApplyCategoryByMerchant(ctx context.Context, category, merchant string) (int64, error)
	ApplyBucketByMerchant(ctx context.Context, bucket, merchant string) (int64, error)
	RemoveCategoryByMerchant(ctx context.Context, category, merchant string) (int64, error)
	RemoveBucketByMerchant(ctx context.Context, bucket, merchant string) (int64, error)
	GetCategoryMappings(ctx context.Context) (map[string][]string, error)
	GetBucketMappings(ctx context.Context) (map[string][]string, error)
	SeedMCCCodes(ctx context.Context, entries []MCCEntry) error
	SeedMerchantCategories(ctx context.Context, entries []MerchantCategoryEntry) (int, error)
	LoadCategorySnapshot(ctx context.Context) (api.CategoryResolver, error)
	SeedMCCCategories(ctx context.Context, names []string) error
}

type pgCommunityRepository struct {
	pool    *pgxpool.Pool
	metrics *QueryInstrumentation
}

type categorySnapshotEntry struct {
	fragment string
	category string
	bucket   string
}

func NewCommunityRepository(deps repositoryDependencies) CommunityRepository {
	metrics := deps.metrics
	if metrics == nil {
		metrics = NewQueryInstrumentation(deps.logger)
	}
	return &pgCommunityRepository{
		pool:    deps.pool,
		metrics: metrics,
	}
}

func (r *pgCommunityRepository) ApplyCategoryByMerchant(ctx context.Context, category, merchant string) (int64, error) {
	return r.applyTaxonomyByMerchant(ctx, "community.apply_category_by_merchant", merchant, "category", category)
}

func (r *pgCommunityRepository) ApplyBucketByMerchant(ctx context.Context, bucket, merchant string) (int64, error) {
	return r.applyTaxonomyByMerchant(ctx, "community.apply_bucket_by_merchant", merchant, "bucket", bucket)
}

func (r *pgCommunityRepository) RemoveCategoryByMerchant(ctx context.Context, category, merchant string) (int64, error) {
	return r.removeTaxonomyByMerchant(ctx, "community.remove_category_by_merchant", merchant, "category", category)
}

func (r *pgCommunityRepository) RemoveBucketByMerchant(ctx context.Context, bucket, merchant string) (int64, error) {
	return r.removeTaxonomyByMerchant(ctx, "community.remove_bucket_by_merchant", merchant, "bucket", bucket)
}

func (r *pgCommunityRepository) GetCategoryMappings(ctx context.Context) (map[string][]string, error) {
	return r.getTaxonomyMappings(ctx, "community.get_category_mappings", "category")
}

func (r *pgCommunityRepository) GetBucketMappings(ctx context.Context) (map[string][]string, error) {
	return r.getTaxonomyMappings(ctx, "community.get_bucket_mappings", "bucket")
}

func (r *pgCommunityRepository) CategorizeMerchant(ctx context.Context, merchant, category, bucket string) (int, error) {
	var rowsUpdated int
	err := r.metrics.Observe(ctx, "community.categorize_merchant", func(ctx context.Context) error {
		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("beginning categorize-merchant transaction: %w", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		tag, err := tx.Exec(ctx,
			`UPDATE transactions
			 SET category = $2, bucket = $3, updated_at = NOW()
			 WHERE merchant_info = $1`,
			merchant, category, bucket,
		)
		if err != nil {
			return fmt.Errorf("updating transactions for merchant %q: %w", merchant, err)
		}
		rowsUpdated = int(tag.RowsAffected())

		_, err = tx.Exec(ctx,
			`INSERT INTO merchant_categories (fragment, category, bucket, user_locked)
			 VALUES ($1, $2, $3, true)
			 ON CONFLICT (fragment) DO UPDATE
			 SET category    = EXCLUDED.category,
			     bucket      = EXCLUDED.bucket,
			     user_locked = true,
			     updated_at  = NOW()`,
			merchant, category, bucket,
		)
		if err != nil {
			return fmt.Errorf("upserting merchant category for %q: %w", merchant, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("committing categorize-merchant transaction: %w", err)
		}
		return nil
	})
	return rowsUpdated, err
}

func (r *pgCommunityRepository) applyTaxonomyByMerchant(
	ctx context.Context,
	metric string,
	merchant string,
	column string,
	value string,
) (int64, error) {
	var rowsUpdated int64
	err := r.metrics.Observe(ctx, metric, func(ctx context.Context) error {
		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("beginning taxonomy merchant transaction: %w", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		tag, err := tx.Exec(ctx,
			fmt.Sprintf(`UPDATE transactions SET %s = $2, updated_at = NOW() WHERE merchant_info = $1`, column),
			merchant, value,
		)
		if err != nil {
			return fmt.Errorf("updating transactions for merchant %q: %w", merchant, err)
		}
		rowsUpdated = tag.RowsAffected()

		_, err = tx.Exec(ctx,
			fmt.Sprintf(`INSERT INTO merchant_categories (fragment, %s, user_locked)
			 VALUES ($1, $2, true)
			 ON CONFLICT (fragment) DO UPDATE
			 SET %s = EXCLUDED.%s,
			     user_locked = true,
			     updated_at = NOW()`, column, column, column),
			merchant, value,
		)
		if err != nil {
			return fmt.Errorf("upserting merchant taxonomy for %q: %w", merchant, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("committing taxonomy merchant transaction: %w", err)
		}
		return nil
	})
	return rowsUpdated, err
}

func (r *pgCommunityRepository) removeTaxonomyByMerchant(
	ctx context.Context,
	metric string,
	merchant string,
	column string,
	value string,
) (int64, error) {
	var rowsUpdated int64
	err := r.metrics.Observe(ctx, metric, func(ctx context.Context) error {
		tag, err := r.pool.Exec(ctx,
			fmt.Sprintf(`UPDATE merchant_categories SET %s = NULL, updated_at = NOW() WHERE fragment = $1 AND %s = $2`, column, column),
			merchant, value,
		)
		if err != nil {
			return fmt.Errorf("removing merchant taxonomy for %q: %w", merchant, err)
		}
		rowsUpdated = tag.RowsAffected()
		_, err = r.pool.Exec(ctx, `DELETE FROM merchant_categories WHERE fragment = $1 AND category IS NULL AND bucket IS NULL AND mcc_code IS NULL`, merchant)
		if err != nil {
			return fmt.Errorf("pruning empty merchant taxonomy for %q: %w", merchant, err)
		}
		return nil
	})
	return rowsUpdated, err
}

func (r *pgCommunityRepository) getTaxonomyMappings(ctx context.Context, metric, column string) (map[string][]string, error) {
	mappings := map[string][]string{}
	err := r.metrics.Observe(ctx, metric, func(ctx context.Context) error {
		rows, err := r.pool.Query(ctx, fmt.Sprintf(
			`SELECT %s, fragment FROM merchant_categories WHERE %s IS NOT NULL ORDER BY %s, fragment`,
			column,
			column,
			column,
		))
		if err != nil {
			return fmt.Errorf("listing merchant taxonomy mappings: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			var merchant string
			if err := rows.Scan(&name, &merchant); err != nil {
				return fmt.Errorf("scanning merchant taxonomy mapping: %w", err)
			}
			mappings[name] = append(mappings[name], merchant)
		}
		return rows.Err()
	})
	return mappings, err
}

func (r *pgCommunityRepository) SeedMCCCodes(ctx context.Context, entries []MCCEntry) error {
	return r.metrics.Observe(ctx, "community.seed_mcc_codes", func(ctx context.Context) error {
		for _, entry := range entries {
			_, err := r.pool.Exec(ctx, `
				INSERT INTO mcc_codes (code, description, category, bucket, updated_at)
				VALUES ($1, $2, $3, $4, NOW())
				ON CONFLICT (code) DO UPDATE SET
					description = EXCLUDED.description,
					category    = EXCLUDED.category,
					bucket      = EXCLUDED.bucket,
					updated_at  = NOW()
			`, entry.Code, entry.Description, entry.Category, entry.Bucket)
			if err != nil {
				return fmt.Errorf("upserting mcc code %s: %w", entry.Code, err)
			}
		}
		return nil
	})
}

func (r *pgCommunityRepository) SeedMerchantCategories(ctx context.Context, entries []MerchantCategoryEntry) (int, error) {
	updated := 0
	err := r.metrics.Observe(ctx, "community.seed_merchant_categories", func(ctx context.Context) error {
		for _, entry := range entries {
			tag, err := r.pool.Exec(ctx, `
				INSERT INTO merchant_categories (fragment, mcc_code, category, bucket, source)
				VALUES ($1, $2, $3, $4, 'community')
				ON CONFLICT (fragment) DO UPDATE SET
					mcc_code   = EXCLUDED.mcc_code,
					category   = EXCLUDED.category,
					bucket     = EXCLUDED.bucket,
					updated_at = NOW()
				WHERE merchant_categories.user_locked = false
			`, entry.Fragment, entry.MCC, entry.Category, entry.Bucket)
			if err != nil {
				return fmt.Errorf("upserting merchant category %s: %w", entry.Fragment, err)
			}
			updated += int(tag.RowsAffected())
		}
		return nil
	})
	return updated, err
}

func (r *pgCommunityRepository) LoadCategorySnapshot(ctx context.Context) (api.CategoryResolver, error) {
	var resolver api.CategoryResolver
	err := r.metrics.Observe(ctx, "community.load_category_snapshot", func(ctx context.Context) error {
		entries, err := r.loadCategorySnapshotEntries(ctx)
		if err != nil {
			return err
		}
		resolver = categoryResolverFromEntries(entries)
		return nil
	})
	return resolver, err
}

func (r *pgCommunityRepository) loadCategorySnapshotEntries(ctx context.Context) ([]categorySnapshotEntry, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT mc.fragment,
		       COALESCE(m.category, mc.category, '') AS category,
		       COALESCE(m.bucket,   mc.bucket,   '') AS bucket
		FROM merchant_categories mc
		LEFT JOIN mcc_codes m ON m.code = mc.mcc_code
		WHERE COALESCE(m.category, mc.category) IS NOT NULL
		   OR COALESCE(m.bucket, mc.bucket) IS NOT NULL
	`)
	if err != nil {
		return nil, fmt.Errorf("loading category snapshot: %w", err)
	}
	defer rows.Close()

	var entries []categorySnapshotEntry
	for rows.Next() {
		var entry categorySnapshotEntry
		if err := rows.Scan(&entry.fragment, &entry.category, &entry.bucket); err != nil {
			return nil, fmt.Errorf("scanning category row: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating category rows: %w", err)
	}
	return entries, nil
}

func categoryResolverFromEntries(entries []categorySnapshotEntry) api.CategoryResolver {
	return func(merchantInfo string) (string, string) {
		lower := strings.ToLower(merchantInfo)
		best := categorySnapshotEntry{}
		for _, entry := range entries {
			if strings.Contains(lower, strings.ToLower(entry.fragment)) {
				if len(entry.fragment) > len(best.fragment) {
					best = entry
				}
			}
		}
		return best.category, best.bucket
	}
}

func (r *pgCommunityRepository) SeedMCCCategories(ctx context.Context, names []string) error {
	return r.metrics.Observe(ctx, "community.seed_mcc_categories", func(ctx context.Context) error {
		for _, name := range names {
			_, err := r.pool.Exec(ctx, `
				INSERT INTO categories (name, is_default)
				VALUES ($1, true)
				ON CONFLICT (name) DO NOTHING
			`, name)
			if err != nil {
				return fmt.Errorf("seeding category %s: %w", name, err)
			}
		}
		return nil
	})
}
