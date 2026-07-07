package llm

import "fmt"

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
