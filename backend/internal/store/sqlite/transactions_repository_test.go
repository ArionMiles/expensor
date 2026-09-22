package sqlite

import (
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

func TestStoreImplementsTransactionStore(t *testing.T) {
	t.Parallel()

	var _ store.TransactionStore = (*Store)(nil)
}
