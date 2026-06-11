package httpapi

import (
	"github.com/ArionMiles/expensor/backend/internal/store"
)

// Storer is the subset of store.Store operations used by the API handlers.
// Using an interface allows handler unit tests to inject a mock without a real database.
type Storer interface {
	settingsStore
	analyticsStore
	transactionStore
	muteStore
	taxonomyStore
	readerRuntimeStore
	ruleStore
	syncStore
	diagnosticStore
}

// compile-time check: *store.Store must satisfy Storer.
var (
	_ Storer = (*store.Store)(nil)
	_ Storer = (*store.InstrumentedStore)(nil)
)
