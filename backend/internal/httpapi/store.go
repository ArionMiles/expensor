package httpapi

import (
	"github.com/ArionMiles/expensor/backend/internal/store"
)

// Storer is the subset of store.Store operations used by the API handlers.
// Using an interface allows handler unit tests to inject a mock without a real database.
type Storer interface {
	authStore
	settingsStore
	scanningStore
	analyticsStore
	transactionStore
	muteStore
	taxonomyStore
	readerRuntimeStore
	llmRuntimeStore
	ruleStore
	syncStore
	diagnosticStore
}

// compile-time checks for the shared backend and instrumentation surfaces.
var (
	_ Storer = (store.Backend)(nil)
	_ Storer = (*store.InstrumentedStore)(nil)
)
