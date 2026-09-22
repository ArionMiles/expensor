package sqlite

import (
	"context"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

var _ store.TaxonomyStore = (*Store)(nil)

func TestTaxonomyRepositoryVisibilityAndOverride(t *testing.T) {
	st := newRepositoryTestStore(t)
	tenant := newRepositoryTestTenant(t, st)
	ctx := context.Background()
	if err := st.CreateCategory(ctx, tenant, "Food & Dining", "tenant override"); err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	categories, err := st.ListCategories(ctx, tenant)
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	var matches int
	for _, category := range categories {
		if category.Name == "Food & Dining" {
			matches++
			if category.Description != "tenant override" || category.IsDefault {
				t.Fatalf("override = %#v", category)
			}
		}
	}
	if matches != 1 {
		t.Fatalf("Food & Dining matches = %d", matches)
	}
}
