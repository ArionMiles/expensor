package bootstrap_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/ArionMiles/expensor/backend/internal/bootstrap"
	"github.com/ArionMiles/expensor/backend/migrations"
)

func TestLegacyClaimPreviewCountsRows(t *testing.T) {
	pool, cleanup := newLegacyTestPool(t)
	defer cleanup()
	ctx := context.Background()
	seedLegacyRows(ctx, t, pool)

	preview, err := bootstrap.PreviewLegacyClaim(ctx, pool)
	if err != nil {
		t.Fatalf("PreviewLegacyClaim() error = %v", err)
	}
	if preview.Transactions != 1 || preview.Labels != 1 || preview.Rules != 1 ||
		preview.ReaderRuntime != 1 || preview.ProcessedMessages != 1 {
		t.Fatalf("preview = %#v", preview)
	}
	if len(preview.BlockingReasons) != 0 {
		t.Fatalf("blocking reasons = %#v, want none", preview.BlockingReasons)
	}
}

func TestLegacyClaimAssignsRowsToInitialTenant(t *testing.T) {
	pool, cleanup := newLegacyTestPool(t)
	defer cleanup()
	ctx := context.Background()
	seedLegacyRows(ctx, t, pool)
	tenantID := createLegacyTestUser(ctx, t, pool)

	if err := bootstrap.ClaimLegacyData(ctx, pool, tenantID); err != nil {
		t.Fatalf("ClaimLegacyData() error = %v", err)
	}
	assertNoNullTenantUserRows(ctx, t, pool)
}

func newLegacyTestPool(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	ctr, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("expensor_test"),
		tcpostgres.WithUsername("expensor"),
		tcpostgres.WithPassword("expensor"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}

	mappedPort, err := ctr.MappedPort(ctx, "5432")
	if err != nil {
		_ = ctr.Terminate(ctx)
		t.Fatalf("mapped port: %v", err)
	}
	conn := fmt.Sprintf("host=localhost port=%d user=expensor password=expensor dbname=expensor_test sslmode=disable pool_max_conns=1", mappedPort.Num())
	pool, err := pgxpool.New(ctx, conn)
	if err != nil {
		_ = ctr.Terminate(ctx)
		t.Fatalf("open pool: %v", err)
	}
	if err := migrations.Run(ctx, pool, slog.Default()); err != nil {
		pool.Close()
		_ = ctr.Terminate(ctx)
		t.Fatalf("run migrations: %v", err)
	}

	return pool, func() {
		pool.Close()
		_ = ctr.Terminate(context.Background())
	}
}

func seedLegacyRows(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	var transactionID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO transactions (message_id, amount, currency, timestamp, merchant_info, category, bucket, source, source_type, source_label, bank)
		VALUES ('legacy-message', 42.50, 'INR', NOW(), 'Legacy Merchant', 'Food', 'Needs', 'gmail', 'credit_card', 'Legacy Card', 'Legacy Bank')
		RETURNING id
	`).Scan(&transactionID); err != nil {
		t.Fatalf("seed transaction: %v", err)
	}
	seedStatements := []struct {
		name string
		sql  string
		args []any
	}{
		{"label", `INSERT INTO labels (name, color) VALUES ('legacy-label', '#f59e0b')`, nil},
		{"label merchant", `INSERT INTO label_merchants (label, merchant_pattern) VALUES ('legacy-label', 'Legacy Merchant')`, nil},
		{"transaction label", `INSERT INTO transaction_labels (transaction_id, label) VALUES ($1, 'legacy-label')`, []any{transactionID}},
		{"rule", `INSERT INTO rules (name, amount_regex, merchant_regex, predefined) VALUES ('legacy-rule', 'INR ([0-9.]+)', 'at (.+)', false)`, nil},
		{"muted merchant", `INSERT INTO muted_merchants (pattern, reason) VALUES ('Legacy Merchant', 'legacy mute')`, nil},
		{
			"merchant category",
			`INSERT INTO merchant_categories (fragment, category, bucket, source, user_locked) VALUES ('Legacy Merchant', 'Food', 'Needs', 'user', true)`,
			nil,
		},
		{
			"diagnostic",
			`INSERT INTO extraction_diagnostics (reader, message_id, source, sender, subject, email_body, rule_name)
			 VALUES ('gmail', 'legacy-message', 'gmail', 'Legacy Sender', 'Legacy Subject', 'Legacy body', 'legacy-rule')`,
			nil,
		},
		{
			"runtime",
			`INSERT INTO reader_runtime (reader, client_secret, oauth_token, config)
			 VALUES ('gmail', '{"installed":{}}'::jsonb, '{"access_token":"legacy"}'::jsonb, '{"mailboxes":"Inbox"}'::jsonb)`,
			nil,
		},
		{"processed message", `INSERT INTO processed_messages (message_key) VALUES ('legacy-message-key')`, nil},
	}
	for _, statement := range seedStatements {
		if _, err := pool.Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatalf("seed %s: %v", statement.name, err)
		}
	}
}

func createLegacyTestUser(ctx context.Context, t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name, role, avatar_key)
		VALUES ('admin@example.com', 'hash', 'Admin', 'admin', 'default')
		RETURNING id
	`).Scan(&id); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return id
}

func assertNoNullTenantUserRows(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	queries := map[string]string{
		"transactions":           `SELECT COUNT(*) FROM transactions WHERE tenant_id IS NULL`,
		"app_config":             `SELECT COUNT(*) FROM app_config WHERE tenant_id IS NULL`,
		"labels":                 `SELECT COUNT(*) FROM labels WHERE tenant_id IS NULL`,
		"rules":                  `SELECT COUNT(*) FROM rules WHERE tenant_id IS NULL AND predefined = false`,
		"muted_merchants":        `SELECT COUNT(*) FROM muted_merchants WHERE tenant_id IS NULL`,
		"merchant_categories":    `SELECT COUNT(*) FROM merchant_categories WHERE tenant_id IS NULL AND user_locked = true`,
		"label_merchants":        `SELECT COUNT(*) FROM label_merchants WHERE tenant_id IS NULL`,
		"extraction_diagnostics": `SELECT COUNT(*) FROM extraction_diagnostics WHERE tenant_id IS NULL`,
		"reader_runtime":         `SELECT COUNT(*) FROM reader_runtime WHERE tenant_id IS NULL`,
		"processed_messages":     `SELECT COUNT(*) FROM processed_messages WHERE tenant_id IS NULL`,
	}
	for table, query := range queries {
		var count int
		if err := pool.QueryRow(ctx, query).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("%s has %d NULL tenant rows, want 0", table, count)
		}
	}
}
