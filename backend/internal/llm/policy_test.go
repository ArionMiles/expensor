package llm

import (
	"errors"
	"strings"
	"testing"
)

func TestRedactTextMasksCommonSensitiveValues(t *testing.T) {
	got := RedactText("email user@example.com card 4111 1111 1111 1111 token", DefaultRedactionPolicy())
	if strings.Contains(got, "user@example.com") {
		t.Fatalf("email was not redacted: %q", got)
	}
	if strings.Contains(got, "4111 1111 1111 1111") {
		t.Fatalf("card number was not redacted: %q", got)
	}
}

func TestEnforceResultLimitsRejectsOversizedPayload(t *testing.T) {
	err := EnforceResultLimits([]byte("abcdef"), ResultLimits{MaxBytes: 3})
	if !errors.Is(err, ErrResultTooLarge) {
		t.Fatalf("EnforceResultLimits() error = %v, want ErrResultTooLarge", err)
	}
}

func TestValidateMutationSafetyRejectsUnexpectedMutations(t *testing.T) {
	err := ValidateMutationSafety(MutationPolicy{AllowMutations: false}, []MutationRequest{
		{Resource: "transactions", Operation: "update"},
	})
	if !errors.Is(err, ErrUnsafeMutation) {
		t.Fatalf("ValidateMutationSafety() error = %v, want ErrUnsafeMutation", err)
	}
}

func TestValidateMutationSafetyAllowsListedMutations(t *testing.T) {
	err := ValidateMutationSafety(MutationPolicy{
		AllowMutations:    true,
		AllowedResources:  []string{"transactions"},
		AllowedOperations: []string{"update"},
	}, []MutationRequest{{Resource: "transactions", Operation: "update"}})
	if err != nil {
		t.Fatalf("ValidateMutationSafety() error = %v", err)
	}
}
