package llm

import (
	"net/http"

	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

var (
	KindNoProviderConfigured  = errors.Kind{Code: "llm_no_provider_configured", Status: http.StatusConflict}
	KindCapabilityUnsupported = errors.Kind{Code: "llm_capability_unsupported", Status: http.StatusConflict}
)
