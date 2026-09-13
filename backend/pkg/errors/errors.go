// Package errors provides structured application errors.
package errors

import (
	"context"
	stderrors "errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

// Kind classifies errors at application boundaries.
type Kind struct {
	Code   string
	Status int
}

var (
	Unknown            = Kind{}
	InvalidArgument    = Kind{Code: "invalid_argument", Status: http.StatusBadRequest}
	InvalidInput       = Kind{Code: "invalid_input", Status: http.StatusUnprocessableEntity}
	Unauthenticated    = Kind{Code: "unauthenticated", Status: http.StatusUnauthorized}
	PermissionDenied   = Kind{Code: "permission_denied", Status: http.StatusForbidden}
	NotFound           = Kind{Code: "not_found", Status: http.StatusNotFound}
	MethodNotAllowed   = Kind{Code: "method_not_allowed", Status: http.StatusMethodNotAllowed}
	Conflict           = Kind{Code: "conflict", Status: http.StatusConflict}
	FailedPrecondition = Kind{Code: "failed_precondition", Status: http.StatusPreconditionFailed}
	PayloadTooLarge    = Kind{Code: "payload_too_large", Status: http.StatusRequestEntityTooLarge}
	ResourceExhausted  = Kind{Code: "resource_exhausted", Status: http.StatusTooManyRequests}
	Internal           = Kind{Code: "internal", Status: http.StatusInternalServerError}
	Unimplemented      = Kind{Code: "unimplemented", Status: http.StatusNotImplemented}
	Unavailable        = Kind{Code: "unavailable", Status: http.StatusServiceUnavailable}
	Canceled           = Kind{Code: "canceled", Status: http.StatusServiceUnavailable}
	DeadlineExceeded   = Kind{Code: "deadline_exceeded", Status: http.StatusServiceUnavailable}
	BadGateway         = Kind{Code: "bad_gateway", Status: http.StatusBadGateway}
)

// Error carries structured context through the error chain.
type Error struct {
	Op      string
	Kind    Kind
	Text    string
	UserMsg string
	Data    any
	Err     error
}

// Builder explicitly assembles an application error.
type Builder struct {
	op      string
	kind    Kind
	text    string
	userMsg string
	data    any
	err     error
}

// B is the empty application error builder.
var B Builder

// Op sets the logical operation.
func (b Builder) Op(op string) Builder {
	b.op = op
	return b
}

// Kind sets the application error kind.
func (b Builder) Kind(kind Kind) Builder {
	b.kind = kind
	return b
}

// Text sets internal diagnostic text.
func (b Builder) Text(text string) Builder {
	b.text = text
	return b
}

// Textf formats and sets internal diagnostic text.
func (b Builder) Textf(format string, args ...any) Builder {
	b.text = fmt.Sprintf(format, args...)
	return b
}

// UserMsg sets text that is safe to expose to callers.
func (b Builder) UserMsg(message string) Builder {
	b.userMsg = message
	return b
}

// Data sets structured error-specific context.
func (b Builder) Data(data any) Builder {
	b.data = data
	return b
}

// Err adds an underlying error.
func (b Builder) Err(err error) Builder {
	b.err = Join(b.err, err)
	return b
}

// Build creates an application error from the builder values.
func (b Builder) Build() *Error {
	err := &Error{
		Op:      b.op,
		Kind:    b.kind,
		Text:    b.text,
		UserMsg: b.userMsg,
		Data:    b.data,
		Err:     b.err,
	}
	err.promote()
	return err
}

func (b Builder) KindInvalidArgument() Builder    { return b.Kind(InvalidArgument) }
func (b Builder) KindInvalidInput() Builder       { return b.Kind(InvalidInput) }
func (b Builder) KindUnauthenticated() Builder    { return b.Kind(Unauthenticated) }
func (b Builder) KindPermissionDenied() Builder   { return b.Kind(PermissionDenied) }
func (b Builder) KindNotFound() Builder           { return b.Kind(NotFound) }
func (b Builder) KindMethodNotAllowed() Builder   { return b.Kind(MethodNotAllowed) }
func (b Builder) KindConflict() Builder           { return b.Kind(Conflict) }
func (b Builder) KindFailedPrecondition() Builder { return b.Kind(FailedPrecondition) }
func (b Builder) KindPayloadTooLarge() Builder    { return b.Kind(PayloadTooLarge) }
func (b Builder) KindResourceExhausted() Builder  { return b.Kind(ResourceExhausted) }
func (b Builder) KindInternal() Builder           { return b.Kind(Internal) }
func (b Builder) KindUnimplemented() Builder      { return b.Kind(Unimplemented) }
func (b Builder) KindUnavailable() Builder        { return b.Kind(Unavailable) }
func (b Builder) KindCanceled() Builder           { return b.Kind(Canceled) }
func (b Builder) KindDeadlineExceeded() Builder   { return b.Kind(DeadlineExceeded) }
func (b Builder) KindBadGateway() Builder         { return b.Kind(BadGateway) }

// KindCarrier is implemented by custom error types that can provide an application kind.
type KindCarrier interface {
	ErrorKind() Kind
}

// Unwrap returns the result of calling err's Unwrap method, if any.
func Unwrap(err error) error {
	return stderrors.Unwrap(err)
}

// Is reports whether any error in err's chain matches target.
func Is(err, target error) bool {
	return stderrors.Is(err, target)
}

// As finds the first error in err's chain that matches target.
func As(err error, target any) bool {
	return stderrors.As(err, target)
}

// Join returns an error that wraps the given errors.
func Join(errs ...error) error {
	return stderrors.Join(errs...)
}

func (e *Error) Error() string {
	parts := make([]string, 0, 4)
	if e.Op != "" {
		parts = append(parts, e.Op)
	}
	if e.Text != "" {
		parts = append(parts, e.Text)
	} else if e.Kind != Unknown {
		kindText := e.Kind.String()
		if e.Err == nil || e.Err.Error() != kindText {
			parts = append(parts, kindText)
		}
	}
	if e.Err != nil {
		parts = append(parts, e.Err.Error())
	}
	if len(parts) == 0 {
		return "unknown error"
	}
	return strings.Join(parts, ": ")
}

// Unwrap returns the underlying error.
func (e *Error) Unwrap() error {
	return e.Err
}

// Ops returns the logical operation chain.
func (e *Error) Ops() []string {
	ops := make([]string, 0, 4)
	for err := error(e); err != nil; {
		var appErr *Error
		if !As(err, &appErr) {
			break
		}
		if appErr.Op != "" {
			ops = append(ops, appErr.Op)
		}
		err = appErr.Err
	}
	return ops
}

func (e *Error) promote() {
	var prev *Error
	if !As(e.Err, &prev) {
		return
	}
	if e.Kind == Unknown {
		e.Kind = prev.Kind
	}
	if e.UserMsg == "" {
		e.UserMsg = prev.UserMsg
	}
}

// WhatKind returns the first structured kind in err's chain.
func WhatKind(err error) Kind {
	if err == nil {
		return Unknown
	}
	switch {
	case Is(err, context.Canceled):
		return Canceled
	case Is(err, context.DeadlineExceeded):
		return DeadlineExceeded
	}
	var appErr *Error
	if As(err, &appErr) && appErr.Kind != Unknown {
		return appErr.Kind
	}
	var kindCarrier KindCarrier
	if As(err, &kindCarrier) && kindCarrier.ErrorKind() != Unknown {
		return kindCarrier.ErrorKind()
	}
	var statusCoder interface{ StatusCode() int }
	if As(err, &statusCoder) && statusCoder.StatusCode() > 0 {
		return KindFromStatus(statusCoder.StatusCode())
	}
	return Unknown
}

// StatusCode returns the HTTP status code implied by err.
func StatusCode(err error) int {
	kind := WhatKind(err)
	if kind.Status == 0 {
		return http.StatusInternalServerError
	}
	return kind.Status
}

// UserMsg returns the safe user-facing message, if any.
func UserMsg(err error) string {
	for err != nil {
		var appErr *Error
		if !As(err, &appErr) {
			return ""
		}
		if appErr.UserMsg != "" {
			return appErr.UserMsg
		}
		err = appErr.Err
	}
	return ""
}

// LogDetailAttrs returns structured error details safe for logs.
func LogDetailAttrs(err error) []slog.Attr {
	attrs := make([]slog.Attr, 0, 2)
	kind := WhatKind(err)
	if kind.Code != "" {
		attrs = append(attrs, slog.String("error_kind", kind.Code))
	}
	var appErr *Error
	if As(err, &appErr) {
		if ops := appErr.Ops(); len(ops) > 0 {
			attrs = append(attrs, slog.Any("error_ops", ops))
		}
	}
	return attrs
}

func (k Kind) String() string {
	if k.Code == "" {
		return "unknown error"
	}
	return strings.ReplaceAll(k.Code, "_", " ")
}

func KindFromStatus(status int) Kind {
	switch status {
	case http.StatusBadRequest:
		return InvalidArgument
	case http.StatusUnprocessableEntity:
		return InvalidInput
	case http.StatusUnauthorized:
		return Unauthenticated
	case http.StatusForbidden:
		return PermissionDenied
	case http.StatusNotFound:
		return NotFound
	case http.StatusMethodNotAllowed:
		return MethodNotAllowed
	case http.StatusConflict:
		return Conflict
	case http.StatusPreconditionFailed:
		return FailedPrecondition
	case http.StatusRequestEntityTooLarge:
		return PayloadTooLarge
	case http.StatusTooManyRequests:
		return ResourceExhausted
	case http.StatusNotImplemented:
		return Unimplemented
	case http.StatusBadGateway:
		return BadGateway
	case http.StatusServiceUnavailable:
		return Unavailable
	default:
		if status >= 500 {
			return Internal
		}
		return Unknown
	}
}
