package postgres

import (
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/store/storetest"
)

func TestStoreConformance(t *testing.T) {
	ts := newTestStore(t)
	defer ts.cleanup()

	storetest.Run(t, ts.Store)
}
