package postgres

import (
	"context"
	"testing"
)

func TestNewWriterUsesSchemaMigrationsInExpensor(t *testing.T) {
	w := newTestWriter(t, Config{})
	defer w.Close()

	ctx := context.Background()
	var schemaMigrationsExists bool
	if err := w.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'expensor'
			  AND table_name = 'schema_migrations'
		)
	`).Scan(&schemaMigrationsExists); err != nil {
		t.Fatalf("check schema_migrations table exists: %v", err)
	}
	if !schemaMigrationsExists {
		t.Fatal("schema_migrations table not present in expensor after writer bootstrap")
	}

	var legacyExists bool
	if err := w.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public'
			  AND table_name = 'legacy_schema_migrations'
		)
	`).Scan(&legacyExists); err != nil {
		t.Fatalf("check legacy table exists: %v", err)
	}
	if legacyExists {
		t.Fatal("legacy_schema_migrations table should not be present after phase 2")
	}
}
