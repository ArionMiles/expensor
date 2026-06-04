package api

import (
	"github.com/ArionMiles/expensor/backend/internal/store"
)

// Compile-time checks that the concrete store implementations satisfy the
// smaller capability interfaces used by API handlers.
var (
	_ settingsStore      = (*store.Store)(nil)
	_ analyticsStore     = (*store.Store)(nil)
	_ transactionStore   = (*store.Store)(nil)
	_ muteStore          = (*store.Store)(nil)
	_ taxonomyStore      = (*store.Store)(nil)
	_ readerRuntimeStore = (*store.Store)(nil)
	_ ruleStore          = (*store.Store)(nil)
	_ syncStore          = (*store.Store)(nil)
	_ diagnosticStore    = (*store.Store)(nil)

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
