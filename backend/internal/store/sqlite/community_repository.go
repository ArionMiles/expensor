package sqlite

import (
	"context"
	"database/sql"
	"strings"

	"github.com/google/uuid"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

const (
	communityTaxonomyCategory = "category"
	communityTaxonomyBucket   = "bucket"
)

type categorySnapshotEntry struct {
	fragment string
	category string
	bucket   string
}

func (s *Store) SeedMCCCodes(ctx context.Context, entries []store.MCCEntry) error {
	return s.repositories.writeTx(ctx, func(q queryer) error {
		for _, entry := range entries {
			_, err := q.ExecContext(ctx, `INSERT INTO mcc_codes (code, description, category, bucket, updated_at)
				VALUES (?, ?, ?, ?, ?) ON CONFLICT(code) DO UPDATE SET description=excluded.description,
				category=excluded.category, bucket=excluded.bucket, updated_at=excluded.updated_at`,
				entry.Code, entry.Description, entry.Category, entry.Bucket, timeToText(s.repositories.now()))
			if err != nil {
				return mapSQLiteError("sqlite.community.seed_mcc_codes", err)
			}
		}
		return nil
	})
}

func (s *Store) SeedMerchantCategories(ctx context.Context, entries []store.MerchantCategoryEntry) (int64, error) {
	var updated int64
	err := s.repositories.writeTx(ctx, func(q queryer) error {
		for _, entry := range entries {
			now := timeToText(s.repositories.now())
			result, err := q.ExecContext(ctx, `INSERT INTO merchant_categories
				(id, fragment, mcc_code, category, bucket, source, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, 'community', ?, ?)
				ON CONFLICT(fragment) WHERE tenant_id IS NULL DO UPDATE SET
				mcc_code=excluded.mcc_code, category=excluded.category, bucket=excluded.bucket,
				updated_at=excluded.updated_at WHERE merchant_categories.user_locked=0`,
				uuid.NewString(), entry.Fragment, entry.MCC, entry.Category, entry.Bucket, now, now)
			if err != nil {
				return mapSQLiteError("sqlite.community.seed_merchant_categories", err)
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return mapSQLiteError("sqlite.community.seed_merchant_categories", err)
			}
			updated += affected
		}
		return nil
	})
	return updated, err
}

func (s *Store) LoadCategorySnapshot(ctx context.Context) (api.CategoryResolver, error) {
	rows, err := s.repositories.query.QueryContext(ctx, `SELECT mc.fragment,
		COALESCE(m.category, mc.category, ''), COALESCE(m.bucket, mc.bucket, '')
		FROM merchant_categories mc LEFT JOIN mcc_codes m ON m.code=mc.mcc_code
		WHERE mc.tenant_id IS NULL AND (COALESCE(m.category, mc.category) IS NOT NULL
		OR COALESCE(m.bucket, mc.bucket) IS NOT NULL)`)
	if err != nil {
		return nil, mapSQLiteError("sqlite.community.load_snapshot", err)
	}
	defer rows.Close()
	entries := []categorySnapshotEntry{}
	for rows.Next() {
		var entry categorySnapshotEntry
		if err := rows.Scan(&entry.fragment, &entry.category, &entry.bucket); err != nil {
			return nil, errors.B.Op("sqlite.community.load_snapshot").Text("scanning category mapping").Err(err).Build()
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, mapSQLiteError("sqlite.community.load_snapshot", err)
	}
	return categoryResolverFromEntries(entries), nil
}

func categoryResolverFromEntries(entries []categorySnapshotEntry) api.CategoryResolver {
	return func(merchantInfo string) (string, string) {
		lower := strings.ToLower(merchantInfo)
		best := categorySnapshotEntry{}
		for _, entry := range entries {
			if strings.Contains(lower, strings.ToLower(entry.fragment)) && len(entry.fragment) > len(best.fragment) {
				best = entry
			}
		}
		return best.category, best.bucket
	}
}

func (s *Store) SeedMCCCategories(ctx context.Context, names []string) error {
	return s.repositories.writeTx(ctx, func(q queryer) error {
		for _, name := range names {
			_, err := q.ExecContext(ctx, `INSERT INTO categories (name, is_default, created_at)
				VALUES (?, 1, ?) ON CONFLICT(name) WHERE tenant_id IS NULL DO NOTHING`,
				name, timeToText(s.repositories.now()))
			if err != nil {
				return mapSQLiteError("sqlite.community.seed_mcc_categories", err)
			}
		}
		return nil
	})
}

func (s *Store) CategorizeMerchant(ctx context.Context, tenant store.Tenant, merchant, category, bucket string) (int64, error) {
	var updated int64
	err := s.repositories.writeTx(ctx, func(q queryer) error {
		now := timeToText(s.repositories.now())
		result, err := q.ExecContext(ctx, `UPDATE transactions SET category=?, bucket=?, updated_at=?
			WHERE merchant_info=? AND tenant_id=?`, category, bucket, now, merchant, tenant.ID)
		if err != nil {
			return mapSQLiteError("sqlite.community.categorize", err)
		}
		updated, err = result.RowsAffected()
		if err != nil {
			return mapSQLiteError("sqlite.community.categorize", err)
		}
		_, err = q.ExecContext(ctx, `INSERT INTO merchant_categories
			(id, tenant_id, fragment, category, bucket, user_locked, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 1, ?, ?)
			ON CONFLICT(tenant_id, fragment) WHERE tenant_id IS NOT NULL DO UPDATE SET
			category=excluded.category, bucket=excluded.bucket, user_locked=1, updated_at=excluded.updated_at`,
			uuid.NewString(), tenant.ID, merchant, category, bucket, now, now)
		return mapSQLiteError("sqlite.community.categorize", err)
	})
	return updated, err
}

func (s *Store) ApplyCategoryByMerchant(ctx context.Context, tenant store.Tenant, category, merchant string) (int64, error) {
	return s.applyTaxonomyByMerchant(ctx, tenant, merchant, communityTaxonomyCategory, category)
}

func (s *Store) ApplyBucketByMerchant(ctx context.Context, tenant store.Tenant, bucket, merchant string) (int64, error) {
	return s.applyTaxonomyByMerchant(ctx, tenant, merchant, communityTaxonomyBucket, bucket)
}

func (s *Store) applyTaxonomyByMerchant(ctx context.Context, tenant store.Tenant, merchant, kind, value string) (int64, error) {
	var updateSQL, insertSQL string
	switch kind {
	case communityTaxonomyCategory:
		updateSQL = `UPDATE transactions SET category=?, updated_at=? WHERE merchant_info=? AND tenant_id=?`
		insertSQL = `INSERT INTO merchant_categories (id, tenant_id, fragment, category, user_locked, created_at, updated_at)
			VALUES (?, ?, ?, ?, 1, ?, ?) ON CONFLICT(tenant_id, fragment) WHERE tenant_id IS NOT NULL
			DO UPDATE SET category=excluded.category, user_locked=1, updated_at=excluded.updated_at`
	case communityTaxonomyBucket:
		updateSQL = `UPDATE transactions SET bucket=?, updated_at=? WHERE merchant_info=? AND tenant_id=?`
		insertSQL = `INSERT INTO merchant_categories (id, tenant_id, fragment, bucket, user_locked, created_at, updated_at)
			VALUES (?, ?, ?, ?, 1, ?, ?) ON CONFLICT(tenant_id, fragment) WHERE tenant_id IS NOT NULL
			DO UPDATE SET bucket=excluded.bucket, user_locked=1, updated_at=excluded.updated_at`
	default:
		return 0, errors.B.Op("sqlite.community.apply_taxonomy").KindInvalidArgument().Text("invalid taxonomy kind").Build()
	}
	var updated int64
	err := s.repositories.writeTx(ctx, func(q queryer) error {
		now := timeToText(s.repositories.now())
		result, err := q.ExecContext(ctx, updateSQL, value, now, merchant, tenant.ID)
		if err != nil {
			return mapSQLiteError("sqlite.community.apply_taxonomy", err)
		}
		updated, err = result.RowsAffected()
		if err != nil {
			return mapSQLiteError("sqlite.community.apply_taxonomy", err)
		}
		_, err = q.ExecContext(ctx, insertSQL, uuid.NewString(), tenant.ID, merchant, value, now, now)
		return mapSQLiteError("sqlite.community.apply_taxonomy", err)
	})
	return updated, err
}

func (s *Store) RemoveCategoryByMerchant(ctx context.Context, tenant store.Tenant, category, merchant string) (int64, error) {
	return s.removeTaxonomyByMerchant(ctx, tenant, merchant, communityTaxonomyCategory, category)
}

func (s *Store) RemoveBucketByMerchant(ctx context.Context, tenant store.Tenant, bucket, merchant string) (int64, error) {
	return s.removeTaxonomyByMerchant(ctx, tenant, merchant, communityTaxonomyBucket, bucket)
}

func (s *Store) removeTaxonomyByMerchant(ctx context.Context, tenant store.Tenant, merchant, kind, value string) (int64, error) {
	var updateSQL string
	switch kind {
	case communityTaxonomyCategory:
		updateSQL = `UPDATE merchant_categories SET category=NULL, updated_at=? WHERE fragment=? AND category=? AND tenant_id=?`
	case communityTaxonomyBucket:
		updateSQL = `UPDATE merchant_categories SET bucket=NULL, updated_at=? WHERE fragment=? AND bucket=? AND tenant_id=?`
	default:
		return 0, errors.B.Op("sqlite.community.remove_taxonomy").KindInvalidArgument().Text("invalid taxonomy kind").Build()
	}
	var updated int64
	err := s.repositories.writeTx(ctx, func(q queryer) error {
		result, err := q.ExecContext(ctx, updateSQL, timeToText(s.repositories.now()), merchant, value, tenant.ID)
		if err != nil {
			return mapSQLiteError("sqlite.community.remove_taxonomy", err)
		}
		updated, err = result.RowsAffected()
		if err != nil {
			return mapSQLiteError("sqlite.community.remove_taxonomy", err)
		}
		_, err = q.ExecContext(ctx, `DELETE FROM merchant_categories WHERE fragment=? AND category IS NULL
			AND bucket IS NULL AND mcc_code IS NULL AND tenant_id=?`, merchant, tenant.ID)
		return mapSQLiteError("sqlite.community.remove_taxonomy", err)
	})
	return updated, err
}

func (s *Store) GetCategoryMappings(ctx context.Context, tenant store.Tenant) (map[string][]string, error) {
	return s.getTaxonomyMappings(ctx, tenant, communityTaxonomyCategory)
}

func (s *Store) GetBucketMappings(ctx context.Context, tenant store.Tenant) (map[string][]string, error) {
	return s.getTaxonomyMappings(ctx, tenant, communityTaxonomyBucket)
}

func (s *Store) getTaxonomyMappings(ctx context.Context, tenant store.Tenant, kind string) (map[string][]string, error) {
	var query string
	switch kind {
	case communityTaxonomyCategory:
		query = `SELECT category, fragment FROM merchant_categories WHERE tenant_id=? AND category IS NOT NULL ORDER BY category, fragment`
	case communityTaxonomyBucket:
		query = `SELECT bucket, fragment FROM merchant_categories WHERE tenant_id=? AND bucket IS NOT NULL ORDER BY bucket, fragment`
	default:
		return nil, errors.B.Op("sqlite.community.get_taxonomy_mappings").KindInvalidArgument().Text("invalid taxonomy kind").Build()
	}
	rows, err := s.repositories.query.QueryContext(ctx, query, tenant.ID)
	if err != nil {
		return nil, mapSQLiteError("sqlite.community.get_taxonomy_mappings", err)
	}
	defer rows.Close()
	result := map[string][]string{}
	for rows.Next() {
		var name, merchant string
		if err := rows.Scan(&name, &merchant); err != nil {
			return nil, errors.B.Op("sqlite.community.get_taxonomy_mappings").Text("scanning taxonomy mapping").Err(err).Build()
		}
		result[name] = append(result[name], merchant)
	}
	if err := rows.Err(); err != nil {
		return nil, mapSQLiteError("sqlite.community.get_taxonomy_mappings", err)
	}
	return result, nil
}

var _ = sql.ErrNoRows
