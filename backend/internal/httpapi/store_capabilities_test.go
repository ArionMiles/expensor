package httpapi

import (
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/internal/store/postgres"
)

// Compile-time checks that the concrete store implementations satisfy the
// smaller capability interfaces used by API handlers.
var (
	_ settingsStore      = (*postgres.Store)(nil)
	_ analyticsStore     = (*postgres.Store)(nil)
	_ transactionStore   = (*postgres.Store)(nil)
	_ muteStore          = (*postgres.Store)(nil)
	_ taxonomyStore      = (*postgres.Store)(nil)
	_ readerRuntimeStore = (*postgres.Store)(nil)
	_ ruleStore          = (*postgres.Store)(nil)
	_ syncStore          = (*postgres.Store)(nil)
	_ diagnosticStore    = (*postgres.Store)(nil)

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
