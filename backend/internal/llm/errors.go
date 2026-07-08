package llm

import (
	"fmt"
	"net/http"

	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

var (
	KindNoProviderConfigured  = errors.Kind{Code: "llm_no_provider_configured", Status: http.StatusConflict}
	KindProviderNotFound      = errors.Kind{Code: "llm_provider_not_found", Status: http.StatusNotFound}
	KindProviderConflict      = errors.Kind{Code: "llm_provider_conflict", Status: http.StatusConflict}
	KindCapabilityUnsupported = errors.Kind{Code: "llm_capability_unsupported", Status: http.StatusConflict}
	KindResultTooLarge        = errors.Kind{Code: "llm_result_too_large", Status: http.StatusUnprocessableEntity}
	KindUnsafeMutation        = errors.Kind{Code: "llm_unsafe_mutation", Status: http.StatusUnprocessableEntity}
)

// ProviderError carries a provider-specific failure in a form HTTP handlers can map safely.
type ProviderError struct {
	Provider   string
	StatusCode int
	Code       string
	Message    string
}

func (e *ProviderError) Error() string {
	if e.Code == "" {
		return fmt.Sprintf("%s provider error: %s", e.Provider, e.Message)
	}
	return fmt.Sprintf("%s provider error %s: %s", e.Provider, e.Code, e.Message)
}

// ErrorKind maps provider failures onto the application's error taxonomy.
func (e *ProviderError) ErrorKind() errors.Kind {
	switch e.StatusCode {
	case 0:
		return errors.BadGateway
	default:
		return errors.KindFromStatus(e.StatusCode)
	}
}
