package httpapi

import (
	"github.com/ArionMiles/expensor/backend/internal/store/instrumented"
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

	_ settingsStore      = (*instrumented.Store)(nil)
	_ analyticsStore     = (*instrumented.Store)(nil)
	_ transactionStore   = (*instrumented.Store)(nil)
	_ muteStore          = (*instrumented.Store)(nil)
	_ taxonomyStore      = (*instrumented.Store)(nil)
	_ readerRuntimeStore = (*instrumented.Store)(nil)
	_ ruleStore          = (*instrumented.Store)(nil)
	_ syncStore          = (*instrumented.Store)(nil)
	_ diagnosticStore    = (*instrumented.Store)(nil)
)
