package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

type communityRepository struct {
	pool *pgxpool.Pool
}

type categorySnapshotEntry struct {
	fragment string
	category string
	bucket   string
}

func newCommunityRepository(deps repositoryDependencies) *communityRepository {
	return &communityRepository{
		pool: deps.pool,
	}
}

func (r *communityRepository) ApplyCategoryByMerchant(ctx context.Context, tenant store.Tenant, category, merchant string) (int64, error) {
	return r.applyTaxonomyByMerchant(ctx, tenant, merchant, "category", category)
}

func (r *communityRepository) ApplyBucketByMerchant(ctx context.Context, tenant store.Tenant, bucket, merchant string) (int64, error) {
	return r.applyTaxonomyByMerchant(ctx, tenant, merchant, "bucket", bucket)
}

func (r *communityRepository) RemoveCategoryByMerchant(ctx context.Context, tenant store.Tenant, category, merchant string) (int64, error) {
	return r.removeTaxonomyByMerchant(ctx, tenant, merchant, "category", category)
}

func (r *communityRepository) RemoveBucketByMerchant(ctx context.Context, tenant store.Tenant, bucket, merchant string) (int64, error) {
	return r.removeTaxonomyByMerchant(ctx, tenant, merchant, "bucket", bucket)
}

func (r *communityRepository) GetCategoryMappings(ctx context.Context, tenant store.Tenant) (map[string][]string, error) {
	return r.getTaxonomyMappings(ctx, tenant, "category")
}

func (r *communityRepository) GetBucketMappings(ctx context.Context, tenant store.Tenant) (map[string][]string, error) {
	return r.getTaxonomyMappings(ctx, tenant, "bucket")
}

func (r *communityRepository) CategorizeMerchant(ctx context.Context, tenant store.Tenant, merchant, category, bucket string) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, errors.E("postgres.community.categorize_merchant", "beginning categorize-merchant transaction", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx,
		`UPDATE transactions
		 SET category = $2, bucket = $3, updated_at = NOW()
		 WHERE merchant_info = $1 AND tenant_id IS NOT DISTINCT FROM $4`,
		merchant, category, bucket, tenantIDParam(tenant),
	)
	if err != nil {
		return 0, errors.E("postgres.community.categorize_merchant", fmt.Sprintf("updating transactions for merchant %q", merchant), err)
	}
	rowsUpdated := tag.RowsAffected()

	_, err = tx.Exec(ctx,
		fmt.Sprintf(`INSERT INTO merchant_categories (tenant_id, fragment, category, bucket, user_locked)
		 VALUES ($1, $2, $3, $4, true)
		 %s DO UPDATE
		 SET category    = EXCLUDED.category,
		     bucket      = EXCLUDED.bucket,
		     user_locked = true,
		     updated_at  = NOW()`, merchantCategoryConflict(tenant)),
		tenantIDParam(tenant), merchant, category, bucket,
	)
	if err != nil {
		return 0, errors.E("postgres.community.categorize_merchant", fmt.Sprintf("upserting merchant category for %q", merchant), err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, errors.E("postgres.community.categorize_merchant", "committing categorize-merchant transaction", err)
	}
	return rowsUpdated, nil
}

func merchantCategoryConflict(tenant store.Tenant) string {
	if tenantIDParam(tenant) == nil {
		return "ON CONFLICT (fragment) WHERE tenant_id IS NULL"
	}
	return "ON CONFLICT (tenant_id, fragment) WHERE tenant_id IS NOT NULL"
}

func (r *communityRepository) applyTaxonomyByMerchant(
	ctx context.Context,
	tenant store.Tenant,
	merchant string,
	column string,
	value string,
) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, errors.E("postgres.community.apply_taxonomy_by_merchant", "beginning taxonomy merchant transaction", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx,
		fmt.Sprintf(`UPDATE transactions SET %s = $2, updated_at = NOW() WHERE merchant_info = $1 AND tenant_id IS NOT DISTINCT FROM $3`, column),
		merchant, value, tenantIDParam(tenant),
	)
	if err != nil {
		return 0, errors.E("postgres.community.apply_taxonomy_by_merchant", fmt.Sprintf("updating transactions for merchant %q", merchant), err)
	}
	rowsUpdated := tag.RowsAffected()

	_, err = tx.Exec(ctx,
		fmt.Sprintf(`INSERT INTO merchant_categories (tenant_id, fragment, %s, user_locked)
		 VALUES ($1, $2, $3, true)
		 %s DO UPDATE
		 SET %s = EXCLUDED.%s,
		     user_locked = true,
		     updated_at = NOW()`, column, merchantCategoryConflict(tenant), column, column),
		tenantIDParam(tenant), merchant, value,
	)
	if err != nil {
		return 0, errors.E("postgres.community.apply_taxonomy_by_merchant", fmt.Sprintf("upserting merchant taxonomy for %q", merchant), err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, errors.E("postgres.community.apply_taxonomy_by_merchant", "committing taxonomy merchant transaction", err)
	}
	return rowsUpdated, nil
}

func (r *communityRepository) removeTaxonomyByMerchant(
	ctx context.Context,
	tenant store.Tenant,
	merchant string,
	column string,
	value string,
) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		fmt.Sprintf(`UPDATE merchant_categories SET %s = NULL, updated_at = NOW()
			WHERE fragment = $1 AND %s = $2 AND tenant_id IS NOT DISTINCT FROM $3`, column, column),
		merchant, value, tenantIDParam(tenant),
	)
	if err != nil {
		return 0, errors.E("postgres.community.remove_taxonomy_by_merchant", fmt.Sprintf("removing merchant taxonomy for %q", merchant), err)
	}
	rowsUpdated := tag.RowsAffected()
	_, err = r.pool.Exec(ctx,
		`DELETE FROM merchant_categories
		 WHERE fragment = $1 AND category IS NULL AND bucket IS NULL AND mcc_code IS NULL AND tenant_id IS NOT DISTINCT FROM $2`,
		merchant, tenantIDParam(tenant),
	)
	if err != nil {
		return 0, errors.E("postgres.community.remove_taxonomy_by_merchant", fmt.Sprintf("pruning empty merchant taxonomy for %q", merchant), err)
	}
	return rowsUpdated, nil
}

func (r *communityRepository) getTaxonomyMappings(ctx context.Context, tenant store.Tenant, column string) (map[string][]string, error) {
	mappings := map[string][]string{}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(
		`SELECT %s, fragment FROM merchant_categories WHERE tenant_id IS NOT DISTINCT FROM $1 AND %s IS NOT NULL ORDER BY %s, fragment`,
		column,
		column,
		column,
	), tenantIDParam(tenant))
	if err != nil {
		return nil, errors.E("postgres.community.get_taxonomy_mappings", "listing merchant taxonomy mappings", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var merchant string
		if err := rows.Scan(&name, &merchant); err != nil {
			return nil, errors.E("postgres.community.get_taxonomy_mappings", "scanning merchant taxonomy mapping", err)
		}
		mappings[name] = append(mappings[name], merchant)
	}
	return mappings, rows.Err()
}

func (r *communityRepository) SeedMCCCodes(ctx context.Context, entries []store.MCCEntry) error {
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
			return errors.E("postgres.community.seed_mcc_codes", fmt.Sprintf("upserting mcc code %s", entry.Code), err)
		}
	}
	return nil
}

func (r *communityRepository) SeedMerchantCategories(ctx context.Context, entries []store.MerchantCategoryEntry) (int64, error) {
	var updated int64
	for _, entry := range entries {
		tag, err := r.pool.Exec(ctx, `
			INSERT INTO merchant_categories (fragment, mcc_code, category, bucket, source)
			VALUES ($1, $2, $3, $4, 'community')
			ON CONFLICT (fragment) WHERE tenant_id IS NULL DO UPDATE SET
				mcc_code   = EXCLUDED.mcc_code,
				category   = EXCLUDED.category,
				bucket     = EXCLUDED.bucket,
				updated_at = NOW()
			WHERE merchant_categories.user_locked = false
		`, entry.Fragment, entry.MCC, entry.Category, entry.Bucket)
		if err != nil {
			return 0, errors.E("postgres.community.seed_merchant_categories", fmt.Sprintf("upserting merchant category %s", entry.Fragment), err)
		}
		updated += tag.RowsAffected()
	}
	return updated, nil
}

func (r *communityRepository) LoadCategorySnapshot(ctx context.Context) (api.CategoryResolver, error) {
	entries, err := r.loadCategorySnapshotEntries(ctx)
	if err != nil {
		return nil, err
	}
	return categoryResolverFromEntries(entries), nil
}

func (r *communityRepository) loadCategorySnapshotEntries(ctx context.Context) ([]categorySnapshotEntry, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT mc.fragment,
		       COALESCE(m.category, mc.category, '') AS category,
		       COALESCE(m.bucket,   mc.bucket,   '') AS bucket
		FROM merchant_categories mc
		LEFT JOIN mcc_codes m ON m.code = mc.mcc_code
		WHERE mc.tenant_id IS NULL
		  AND (
		      COALESCE(m.category, mc.category) IS NOT NULL
		   OR COALESCE(m.bucket, mc.bucket) IS NOT NULL
		  )
	`)
	if err != nil {
		return nil, errors.E("postgres.community.load_category_snapshot_entries", "loading category snapshot", err)
	}
	defer rows.Close()

	var entries []categorySnapshotEntry
	for rows.Next() {
		var entry categorySnapshotEntry
		if err := rows.Scan(&entry.fragment, &entry.category, &entry.bucket); err != nil {
			return nil, errors.E("postgres.community.load_category_snapshot_entries", "scanning category row", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.E("postgres.community.load_category_snapshot_entries", "iterating category rows", err)
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

func (r *communityRepository) SeedMCCCategories(ctx context.Context, names []string) error {
	for _, name := range names {
		_, err := r.pool.Exec(ctx, `
			INSERT INTO categories (name, is_default)
			VALUES ($1, true)
			ON CONFLICT (name) WHERE tenant_id IS NULL DO NOTHING
		`, name)
		if err != nil {
			return errors.E("postgres.community.seed_mcc_categories", fmt.Sprintf("seeding category %s", name), err)
		}
	}
	return nil
}
