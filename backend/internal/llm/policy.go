package llm

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrResultTooLarge = errors.New("llm result too large")
	ErrUnsafeMutation = errors.New("llm mutation is not allowed")
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
	if limits.MaxBytes > 0 && len(payload) > limits.MaxBytes {
		return fmt.Errorf("%w: %d bytes exceeds %d", ErrResultTooLarge, len(payload), limits.MaxBytes)
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
	if len(mutations) == 0 {
		return nil
	}
	if !policy.AllowMutations {
		return fmt.Errorf("%w: mutations are disabled", ErrUnsafeMutation)
	}
	resources := stringSet(policy.AllowedResources)
	operations := stringSet(policy.AllowedOperations)
	for _, mutation := range mutations {
		if len(resources) > 0 {
			if _, ok := resources[strings.TrimSpace(mutation.Resource)]; !ok {
				return fmt.Errorf("%w: resource %q", ErrUnsafeMutation, mutation.Resource)
			}
		}
		if len(operations) > 0 {
			if _, ok := operations[strings.TrimSpace(mutation.Operation)]; !ok {
				return fmt.Errorf("%w: operation %q", ErrUnsafeMutation, mutation.Operation)
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
