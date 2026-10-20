package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

var _ store.RuleStore = (*Store)(nil)

func TestRulesRepositoryRoundTrip(t *testing.T) {
	st := newRepositoryTestStore(t)
	tenant := newRepositoryTestTenant(t, st)
	ctx := context.Background()

	created, err := st.CreateRule(ctx, tenant, store.RuleRow{
		Name: "Card", SenderEmails: []string{"first@example.test", "second@example.test"},
		AmountRegex: "amount", MerchantRegex: "merchant", SourceLabel: "Primary Card",
	})
	if err != nil {
		t.Fatalf("CreateRule: %v", err)
	}
	if _, err := uuid.Parse(created.ID); err != nil {
		t.Fatalf("CreateRule ID = %q: %v", created.ID, err)
	}
	got, err := st.GetRule(ctx, tenant, created.ID)
	if err != nil {
		t.Fatalf("GetRule: %v", err)
	}
	if got.SenderEmail != "first@example.test" || len(got.SenderEmails) != 2 || got.TransactionSource != "Primary Card" {
		t.Fatalf("GetRule = %#v", got)
	}
}

func newRepositoryTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := New(context.Background(), testStoreOptions(filepath.Join(privateTestDir(t), "repository.db")))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(st.Close)
	return st
}

func newRepositoryTestTenant(t *testing.T, st *Store) store.Tenant {
	t.Helper()
	id := uuid.NewString()
	now := timeToText(st.repositories.now())
	if _, err := st.db.ExecContext(context.Background(), `
		INSERT INTO users (id, email, display_name, role, created_at, updated_at)
		VALUES (?, ?, 'Repository Test', 'user', ?, ?)
	`, id, id+"@example.test", now, now); err != nil {
		t.Fatalf("insert test tenant: %v", err)
	}
	return store.Tenant{ID: id}
}
