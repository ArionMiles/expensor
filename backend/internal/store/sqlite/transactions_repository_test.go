package sqlite

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

func TestStoreImplementsTransactionStore(t *testing.T) {
	t.Parallel()

	var _ store.TransactionStore = (*Store)(nil)
}

func TestTransactionMutationsUseWriteGate(t *testing.T) {
	tests := []struct {
		name string
		run  func(*Store, context.Context, store.Tenant) error
	}{
		{name: "update description", run: func(st *Store, ctx context.Context, tenant store.Tenant) error {
			return st.UpdateDescription(ctx, tenant, "missing", "updated")
		}},
		{name: "mute transaction", run: func(st *Store, ctx context.Context, tenant store.Tenant) error {
			return st.MuteTransaction(ctx, tenant, "missing", true, "reason")
		}},
		{name: "update mute reason", run: func(st *Store, ctx context.Context, tenant store.Tenant) error {
			return st.UpdateMuteReason(ctx, tenant, "missing", "reason")
		}},
		{name: "update merchant reason", run: func(st *Store, ctx context.Context, tenant store.Tenant) error {
			return st.UpdateMerchantReason(ctx, tenant, "missing", "reason")
		}},
		{name: "delete muted merchant", run: func(st *Store, ctx context.Context, tenant store.Tenant) error {
			return st.DeleteMutedMerchant(ctx, tenant, "missing")
		}},
		{name: "unmute by pattern", run: func(st *Store, ctx context.Context, tenant store.Tenant) error {
			return st.UnmuteByPattern(ctx, tenant, "merchant")
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := openTestStore(t, filepath.Join(privateTestDir(t), "write-gate.db"))
			<-st.writeGate
			done := make(chan error, 1)
			go func() {
				done <- tt.run(st, t.Context(), store.Tenant{ID: "tenant"})
			}()

			select {
			case err := <-done:
				t.Errorf("mutation completed before write gate was released: %v", err)
			case <-time.After(50 * time.Millisecond):
			}
			st.writeGate <- struct{}{}

			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("mutation did not complete after write gate was released")
			}
		})
	}
}

func TestTransactionPageQueryAppliesPaginationInSQLite(t *testing.T) {
	query, args, err := buildTransactionPageQuery(
		store.Tenant{ID: "tenant"},
		"coffee",
		store.ListFilter{Page: 3, PageSize: 25, SortDir: "asc"},
	)
	if err != nil {
		t.Fatalf("build transaction page query: %v", err)
	}
	if !strings.Contains(query, "LIMIT ? OFFSET ?") {
		t.Fatalf("query does not apply pagination in SQLite: %s", query)
	}
	if got, want := args[len(args)-2:], []any{25, 50}; !equalAnyValues(got, want) {
		t.Fatalf("pagination args = %#v, want %#v", got, want)
	}
}

func equalAnyValues(left, right []any) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
