package sqlite

import (
	"context"
	"regexp"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
)

var _ store.Seeder = (*Store)(nil)

func TestSeederRepositoryIsIdempotent(t *testing.T) {
	st := newRepositoryTestStore(t)
	category := "Seed Category"
	content := store.SeedContent{
		Rules:              []api.Rule{{Name: "Seed Rule", Amount: regexp.MustCompile("amount"), MerchantInfo: regexp.MustCompile("merchant")}},
		MerchantCategories: []store.MerchantCategoryEntry{{Fragment: "seed merchant", Category: &category}},
	}
	if _, err := st.Seed(context.Background(), content); err != nil {
		t.Fatalf("Seed first: %v", err)
	}
	resolver, err := st.Seed(context.Background(), content)
	if err != nil {
		t.Fatalf("Seed second: %v", err)
	}
	if got, _ := resolver("seed merchant"); got != category {
		t.Fatalf("resolved category = %q, want %q", got, category)
	}
}
