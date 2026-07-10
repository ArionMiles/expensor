package httpapi

import (
	"github.com/ArionMiles/expensor/backend/internal/store"
)

// Compile-time checks that the concrete store implementations satisfy the
// smaller capability interfaces used by API handlers.
var (
	_ settingsStore      = (store.Backend)(nil)
	_ analyticsStore     = (store.Backend)(nil)
	_ transactionStore   = (store.Backend)(nil)
	_ muteStore          = (store.Backend)(nil)
	_ taxonomyStore      = (store.Backend)(nil)
	_ readerRuntimeStore = (store.Backend)(nil)
	_ ruleStore          = (store.Backend)(nil)
	_ syncStore          = (store.Backend)(nil)
	_ diagnosticStore    = (store.Backend)(nil)

	_ settingsStore      = (*store.InstrumentedStore)(nil)
	_ analyticsStore     = (*store.InstrumentedStore)(nil)
	_ transactionStore   = (*store.InstrumentedStore)(nil)
	_ muteStore          = (*store.InstrumentedStore)(nil)
	_ taxonomyStore      = (*store.InstrumentedStore)(nil)
	_ readerRuntimeStore = (*store.InstrumentedStore)(nil)
	_ ruleStore          = (*store.InstrumentedStore)(nil)
	_ syncStore          = (*store.InstrumentedStore)(nil)
	_ diagnosticStore    = (*store.InstrumentedStore)(nil)
)
