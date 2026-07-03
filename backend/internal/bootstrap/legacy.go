// Package bootstrap contains removable first-admin migration helpers.
package bootstrap

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// LegacyPreview reports legacy single-user rows that will be claimed by the first admin tenant.
type LegacyPreview struct {
	Transactions          int64    `json:"transactions"`
	AppConfig             int64    `json:"app_config"`
	Labels                int64    `json:"labels"`
	Categories            int64    `json:"categories"`
	Buckets               int64    `json:"buckets"`
	Rules                 int64    `json:"rules"`
	MutedMerchants        int64    `json:"muted_merchants"`
	MerchantCategories    int64    `json:"merchant_categories"`
	LabelMerchants        int64    `json:"label_merchants"`
	ExtractionDiagnostics int64    `json:"extraction_diagnostics"`
	ReaderRuntime         int64    `json:"reader_runtime"`
	ProcessedMessages     int64    `json:"processed_messages"`
	BlockingReasons       []string `json:"blocking_reasons"`
}

type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type txBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// PreviewLegacyClaim counts NULL-tenant rows that belong to the removable legacy migration path.
func PreviewLegacyClaim(ctx context.Context, db queryRower) (LegacyPreview, error) {
	var preview LegacyPreview
	counts := []struct {
		name string
		dest *int64
		sql  string
	}{
		{"transactions", &preview.Transactions, `SELECT COUNT(*) FROM transactions WHERE tenant_id IS NULL`},
		{"app_config", &preview.AppConfig, `SELECT COUNT(*) FROM app_config WHERE tenant_id IS NULL`},
		{"labels", &preview.Labels, `SELECT COUNT(*) FROM labels WHERE tenant_id IS NULL`},
		{"categories", &preview.Categories, `SELECT COUNT(*) FROM categories WHERE tenant_id IS NULL AND is_default = false`},
		{"buckets", &preview.Buckets, `SELECT COUNT(*) FROM buckets WHERE tenant_id IS NULL AND is_default = false`},
		{"rules", &preview.Rules, `SELECT COUNT(*) FROM rules WHERE tenant_id IS NULL AND predefined = false`},
		{"muted_merchants", &preview.MutedMerchants, `SELECT COUNT(*) FROM muted_merchants WHERE tenant_id IS NULL`},
		{"merchant_categories", &preview.MerchantCategories, legacyMerchantCategoriesCountSQL},
		{"label_merchants", &preview.LabelMerchants, `SELECT COUNT(*) FROM label_merchants WHERE tenant_id IS NULL`},
		{"extraction_diagnostics", &preview.ExtractionDiagnostics, `SELECT COUNT(*) FROM extraction_diagnostics WHERE tenant_id IS NULL`},
		{"reader_runtime", &preview.ReaderRuntime, `SELECT COUNT(*) FROM reader_runtime WHERE tenant_id IS NULL`},
		{"processed_messages", &preview.ProcessedMessages, `SELECT COUNT(*) FROM processed_messages WHERE tenant_id IS NULL`},
	}
	for _, count := range counts {
		if err := db.QueryRow(ctx, count.sql).Scan(count.dest); err != nil {
			return LegacyPreview{}, fmt.Errorf("counting legacy %s rows: %w", count.name, err)
		}
	}
	return preview, nil
}

// ClaimLegacyData assigns legacy single-user rows to tenantID inside one transaction.
func ClaimLegacyData(ctx context.Context, db txBeginner, tenantID string) error {
	if tenantID == "" {
		return errors.New("legacy claim tenant id is required")
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning legacy claim: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := ClaimLegacyDataTx(ctx, tx, tenantID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing legacy claim: %w", err)
	}
	return nil
}

// ClaimLegacyDataTx assigns legacy rows using an existing transaction.
func ClaimLegacyDataTx(ctx context.Context, tx pgx.Tx, tenantID string) error {
	if tenantID == "" {
		return errors.New("legacy claim tenant id is required")
	}
	updates := []struct {
		name string
		sql  string
	}{
		{"transactions", `UPDATE transactions SET tenant_id = $1 WHERE tenant_id IS NULL`},
		{"app_config", `UPDATE app_config SET tenant_id = $1 WHERE tenant_id IS NULL`},
		{"labels", `UPDATE labels SET tenant_id = $1 WHERE tenant_id IS NULL`},
		{"categories", `UPDATE categories SET tenant_id = $1 WHERE tenant_id IS NULL AND is_default = false`},
		{"buckets", `UPDATE buckets SET tenant_id = $1 WHERE tenant_id IS NULL AND is_default = false`},
		{"rules", `UPDATE rules SET tenant_id = $1 WHERE tenant_id IS NULL AND predefined = false`},
		{"muted_merchants", `UPDATE muted_merchants SET tenant_id = $1 WHERE tenant_id IS NULL`},
		{"merchant_categories", `UPDATE merchant_categories SET tenant_id = $1 WHERE tenant_id IS NULL AND (user_locked = true OR source = 'user')`},
		{"label_merchants", `UPDATE label_merchants SET tenant_id = $1 WHERE tenant_id IS NULL`},
		{"extraction_diagnostics", `UPDATE extraction_diagnostics SET tenant_id = $1 WHERE tenant_id IS NULL`},
		{"reader_runtime", `UPDATE reader_runtime SET tenant_id = $1 WHERE tenant_id IS NULL`},
		{"processed_messages", `UPDATE processed_messages SET tenant_id = $1 WHERE tenant_id IS NULL`},
	}
	for _, update := range updates {
		if _, err := tx.Exec(ctx, update.sql, tenantID); err != nil {
			return fmt.Errorf("claiming legacy %s rows: %w", update.name, err)
		}
	}
	return verifyLegacyClaimComplete(ctx, tx)
}

func verifyLegacyClaimComplete(ctx context.Context, db queryRower) error {
	preview, err := PreviewLegacyClaim(ctx, db)
	if err != nil {
		return err
	}
	if preview.Transactions > 0 || preview.AppConfig > 0 || preview.Labels > 0 || preview.Categories > 0 ||
		preview.Buckets > 0 || preview.Rules > 0 || preview.MutedMerchants > 0 || preview.MerchantCategories > 0 ||
		preview.LabelMerchants > 0 || preview.ExtractionDiagnostics > 0 || preview.ReaderRuntime > 0 ||
		preview.ProcessedMessages > 0 {
		return errors.New("legacy claim left unclaimed tenant-owned rows")
	}
	return nil
}

const legacyMerchantCategoriesCountSQL = `
	SELECT COUNT(*)
	FROM merchant_categories
	WHERE tenant_id IS NULL AND (user_locked = true OR source = 'user')
`
