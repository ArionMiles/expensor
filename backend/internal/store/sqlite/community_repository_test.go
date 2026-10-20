package sqlite

import (
	"context"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

var _ store.CommunityStore = (*Store)(nil)

func TestCommunityRepositoryLongestMatch(t *testing.T) {
	st := newRepositoryTestStore(t)
	shortCategory, longCategory := "Short", "Long"
	if updated, err := st.SeedMerchantCategories(context.Background(), []store.MerchantCategoryEntry{
		{Fragment: "shop", Category: &shortCategory},
		{Fragment: "example shop", Category: &longCategory},
	}); err != nil || updated != 2 {
		t.Fatalf("SeedMerchantCategories = %d, %v", updated, err)
	}
	resolver, err := st.LoadCategorySnapshot(context.Background())
	if err != nil {
		t.Fatalf("LoadCategorySnapshot: %v", err)
	}
	if category, _ := resolver("PAID AT EXAMPLE SHOP"); category != longCategory {
		t.Fatalf("resolved category = %q, want %q", category, longCategory)
	}
}
