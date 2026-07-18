package errors

import (
	"errors"
	"net/http"
	"slices"
	"testing"
)

func TestEWrapsSentinelAndPromotesKind(t *testing.T) {
	base := errors.New("not found")
	err := E("store.transactions.get", NotFound, base)

	if !errors.Is(err, base) {
		t.Fatalf("errors.Is(%v, base) = false", err)
	}
	if got := WhatKind(err); got != NotFound {
		t.Fatalf("WhatKind() = %#v, want %#v", got, NotFound)
	}
	if got := StatusCode(err); got != http.StatusNotFound {
		t.Fatalf("StatusCode() = %d, want %d", got, http.StatusNotFound)
	}
	if got := err.Error(); got != "store.transactions.get: not found" {
		t.Fatalf("Error() = %q", got)
	}
}

func TestNestedOps(t *testing.T) {
	err := E("http.rule_draft", E("assistant.rule_draft", InvalidInput, "bad input"))

	var appErr *Error
	if !errors.As(err, &appErr) {
		t.Fatal("error is not *Error")
	}
	if got := appErr.Ops(); !slices.Equal(got, []string{"http.rule_draft", "assistant.rule_draft"}) {
		t.Fatalf("Ops() = %#v", got)
	}
}

func TestUserMsgFindsSafeMessageInWrappedApplicationError(t *testing.T) {
	err := E("http.rule_draft", E("assistant.rule_draft", InvalidInput, User("add at least one email sample")))

	if got := UserMsg(err); got != "add at least one email sample" {
		t.Fatalf("UserMsg() = %q, want safe inner message", got)
	}
}

func TestEJoinsSentinelAndCause(t *testing.T) {
	base := errors.New("invalid output")
	cause := errors.New("json parse failed")
	err := E("assistant.request", InvalidInput, base, cause)

	if !errors.Is(err, base) {
		t.Fatalf("errors.Is(%v, base) = false", err)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("errors.Is(%v, cause) = false", err)
	}
	if got := WhatKind(err); got != InvalidInput {
		t.Fatalf("WhatKind() = %#v, want %#v", got, InvalidInput)
	}
}

func TestLogDetailAttrsIncludesKindAndOps(t *testing.T) {
	err := E("assistant.rule_draft", InvalidInput, "bad input")

	attrs := LogDetailAttrs(err)
	got := map[string]any{}
	for _, attr := range attrs {
		got[attr.Key] = attr.Value.Any()
	}

	if got["error_kind"] != InvalidInput.Code {
		t.Fatalf("error_kind = %#v, want %q", got["error_kind"], InvalidInput.Code)
	}
}
