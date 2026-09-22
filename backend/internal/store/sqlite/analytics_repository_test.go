package sqlite

import (
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

func TestStoreImplementsAnalyticsStore(t *testing.T) {
	t.Parallel()

	var _ store.AnalyticsStore = (*Store)(nil)
}
