package errors

import (
	"errors"
	"net/http"
	"reflect"
	"slices"
	"testing"
)

func TestBuilderPopulatesError(t *testing.T) {
	cause := errors.New("database failed")
	data := map[string]string{"field": "email"}

	err := B.
		Op("store.users.create").
		KindInvalidArgument().
		Textf("creating user %q", "a@example.com").
		UserMsg("user could not be created").
		Data(data).
		Err(cause).
		Build()

	if err.Op != "store.users.create" {
		t.Errorf("Op = %q, want %q", err.Op, "store.users.create")
	}
	if err.Kind != InvalidArgument {
		t.Errorf("Kind = %#v, want %#v", err.Kind, InvalidArgument)
	}
	if err.Text != `creating user "a@example.com"` {
		t.Errorf("Text = %q, want formatted diagnostic text", err.Text)
	}
	if err.UserMsg != "user could not be created" {
		t.Errorf("UserMsg = %q, want %q", err.UserMsg, "user could not be created")
	}
	if !reflect.DeepEqual(err.Data, data) {
		t.Errorf("Data = %#v, want %#v", err.Data, data)
	}
	if !errors.Is(err, cause) {
		t.Errorf("errors.Is(%v, cause) = false", err)
	}
}

func TestBuilderValueReceiversDoNotRetainState(t *testing.T) {
	first := B.Op("first").Text("first text").Build()
	second := B.Op("second").Build()

	if first.Op != "first" || first.Text != "first text" {
		t.Fatalf("first error = %#v", first)
	}
	if second.Op != "second" || second.Text != "" {
		t.Fatalf("second error retained builder state: %#v", second)
	}
}

func TestBuilderJoinsRepeatedCauses(t *testing.T) {
	first := errors.New("first")
	second := errors.New("second")

	err := B.Err(first).Err(nil).Err(second).Build()

	if !errors.Is(err, first) {
		t.Errorf("errors.Is(%v, first) = false", err)
	}
	if !errors.Is(err, second) {
		t.Errorf("errors.Is(%v, second) = false", err)
	}
}

func TestBuilderPromotesWrappedKindAndUserMessage(t *testing.T) {
	inner := B.
		Op("assistant.rule_draft").
		KindInvalidInput().
		UserMsg("add at least one email sample").
		Build()
	err := B.Op("http.rule_draft").Err(inner).Build()

	if err.Kind != InvalidInput {
		t.Errorf("Kind = %#v, want %#v", err.Kind, InvalidInput)
	}
	if err.UserMsg != "add at least one email sample" {
		t.Errorf("UserMsg = %q, want promoted user message", err.UserMsg)
	}
	if got := err.Ops(); !slices.Equal(got, []string{"http.rule_draft", "assistant.rule_draft"}) {
		t.Errorf("Ops() = %#v", got)
	}
}

func TestBuilderKindMethods(t *testing.T) {
	tests := []struct {
		name  string
		build func() *Error
		want  Kind
	}{
		{name: "invalid argument", build: func() *Error { return B.KindInvalidArgument().Build() }, want: InvalidArgument},
		{name: "invalid input", build: func() *Error { return B.KindInvalidInput().Build() }, want: InvalidInput},
		{name: "unauthenticated", build: func() *Error { return B.KindUnauthenticated().Build() }, want: Unauthenticated},
		{name: "permission denied", build: func() *Error { return B.KindPermissionDenied().Build() }, want: PermissionDenied},
		{name: "not found", build: func() *Error { return B.KindNotFound().Build() }, want: NotFound},
		{name: "method not allowed", build: func() *Error { return B.KindMethodNotAllowed().Build() }, want: MethodNotAllowed},
		{name: "conflict", build: func() *Error { return B.KindConflict().Build() }, want: Conflict},
		{name: "failed precondition", build: func() *Error { return B.KindFailedPrecondition().Build() }, want: FailedPrecondition},
		{name: "payload too large", build: func() *Error { return B.KindPayloadTooLarge().Build() }, want: PayloadTooLarge},
		{name: "resource exhausted", build: func() *Error { return B.KindResourceExhausted().Build() }, want: ResourceExhausted},
		{name: "internal", build: func() *Error { return B.KindInternal().Build() }, want: Internal},
		{name: "unimplemented", build: func() *Error { return B.KindUnimplemented().Build() }, want: Unimplemented},
		{name: "unavailable", build: func() *Error { return B.KindUnavailable().Build() }, want: Unavailable},
		{name: "canceled", build: func() *Error { return B.KindCanceled().Build() }, want: Canceled},
		{name: "deadline exceeded", build: func() *Error { return B.KindDeadlineExceeded().Build() }, want: DeadlineExceeded},
		{name: "bad gateway", build: func() *Error { return B.KindBadGateway().Build() }, want: BadGateway},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.build().Kind; got != tt.want {
				t.Errorf("Kind = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestBuilderWrapsSentinelAndPromotesKind(t *testing.T) {
	base := errors.New("not found")
	err := B.Op("store.transactions.get").KindNotFound().Err(base).Build()

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
	err := B.Op("http.rule_draft").Err(B.Op("assistant.rule_draft").KindInvalidInput().Text("bad input").Build()).Build()

	var appErr *Error
	if !errors.As(err, &appErr) {
		t.Fatal("error is not *Error")
	}
	if got := appErr.Ops(); !slices.Equal(got, []string{"http.rule_draft", "assistant.rule_draft"}) {
		t.Fatalf("Ops() = %#v", got)
	}
}

func TestUserMsgFindsSafeMessageInWrappedApplicationError(t *testing.T) {
	err := B.Op("http.rule_draft").Err(B.Op("assistant.rule_draft").KindInvalidInput().UserMsg("add at least one email sample").Build()).Build()

	if got := UserMsg(err); got != "add at least one email sample" {
		t.Fatalf("UserMsg() = %q, want safe inner message", got)
	}
}

func TestBuilderJoinsSentinelAndCause(t *testing.T) {
	base := errors.New("invalid output")
	cause := errors.New("json parse failed")
	err := B.Op("assistant.request").KindInvalidInput().Err(base).Err(cause).Build()

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
	err := B.Op("assistant.rule_draft").KindInvalidInput().Text("bad input").Build()

	attrs := LogDetailAttrs(err)
	got := map[string]any{}
	for _, attr := range attrs {
		got[attr.Key] = attr.Value.Any()
	}

	if got["error_kind"] != InvalidInput.Code {
		t.Fatalf("error_kind = %#v, want %q", got["error_kind"], InvalidInput.Code)
	}
}
