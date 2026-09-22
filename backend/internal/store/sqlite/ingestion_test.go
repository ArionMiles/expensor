package sqlite

import (
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

func TestStoreImplementsTransactionBatchWriter(t *testing.T) {
	t.Parallel()

	var _ store.TransactionBatchWriter = (*Store)(nil)
}
