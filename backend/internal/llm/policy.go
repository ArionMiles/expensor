package llm

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

var (
	emailPattern = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)
	cardPattern  = regexp.MustCompile(`\b(?:\d[ -]?){13,19}\b`)
)

// RedactionPolicy controls deterministic prompt redaction.
type RedactionPolicy struct {
	Replacement string
}

// DefaultRedactionPolicy returns the baseline privacy policy shared by workflows.
func DefaultRedactionPolicy() RedactionPolicy {
	return RedactionPolicy{Replacement: "[REDACTED]"}
}

// RedactText masks common high-risk values before a prompt is assembled.
func RedactText(input string, policy RedactionPolicy) string {
	replacement := policy.Replacement
	out := emailPattern.ReplaceAllString(input, replacement)
	return cardPattern.ReplaceAllStringFunc(out, func(candidate string) string {
		digits := 0
		for _, r := range candidate {
			if r >= '0' && r <= '9' {
				digits++
			}
		}
		if digits < 13 {
			return candidate
		}
		return replacement
	})
}

// ResultLimits constrains the amount of model output a workflow may accept.
type ResultLimits struct {
	MaxBytes int `json:"max_bytes,omitempty" yaml:"max_bytes,omitempty"`
	MaxItems int `json:"max_items,omitempty" yaml:"max_items,omitempty"`
}

// EnforceResultLimits validates a raw provider result size.
func EnforceResultLimits(payload []byte, limits ResultLimits) error {
	const op = "llm.EnforceResultLimits"

	if limits.MaxBytes > 0 && len(payload) > limits.MaxBytes {
		return errors.E(op, errors.InvalidInput, fmt.Sprintf("llm result too large: %d bytes exceeds %d", len(payload), limits.MaxBytes))
	}
	return nil
}

// MutationRequest describes a proposed write from an LLM-enabled workflow.
type MutationRequest struct {
	Resource  string
	Operation string
}

// MutationPolicy controls whether a workflow may propose or execute mutations.
type MutationPolicy struct {
	AllowMutations    bool
	AllowedResources  []string
	AllowedOperations []string
}

// ValidateMutationSafety validates proposed mutations before persistence.
func ValidateMutationSafety(policy MutationPolicy, mutations []MutationRequest) error {
	const op = "llm.ValidateMutationSafety"

	if len(mutations) == 0 {
		return nil
	}
	if !policy.AllowMutations {
		return errors.E(op, errors.InvalidInput, "llm mutation is not allowed: mutations are disabled")
	}
	resources := stringSet(policy.AllowedResources)
	operations := stringSet(policy.AllowedOperations)
	for _, mutation := range mutations {
		if len(resources) > 0 {
			if _, ok := resources[strings.TrimSpace(mutation.Resource)]; !ok {
				return errors.E(op, errors.InvalidInput, fmt.Sprintf("llm mutation is not allowed: resource %q", mutation.Resource))
			}
		}
		if len(operations) > 0 {
			if _, ok := operations[strings.TrimSpace(mutation.Operation)]; !ok {
				return errors.E(op, errors.InvalidInput, fmt.Sprintf("llm mutation is not allowed: operation %q", mutation.Operation))
			}
		}
	}
	return nil
}

func stringSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out[value] = struct{}{}
	}
	return out
}
