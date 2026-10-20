package sqlite

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

func TestTransactionMutationsUseWriteTransaction(t *testing.T) {
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
			wantErr := stderrors.New("write transaction called")
			st := &Store{repositories: repositoryDependencies{
				now: time.Now,
				writeTx: func(context.Context, func(queryer) error) error {
					return wantErr
				},
			}}
			if err := tt.run(st, t.Context(), store.Tenant{ID: "tenant"}); !stderrors.Is(err, wantErr) {
				t.Fatalf("mutation error = %v, want write transaction error", err)
			}
		})
	}
}
