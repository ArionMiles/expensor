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
	Conflict           = Kind{Code: "conflict", Status: http.StatusConflict}
	FailedPrecondition = Kind{Code: "failed_precondition", Status: http.StatusPreconditionFailed}
	ResourceExhausted  = Kind{Code: "resource_exhausted", Status: http.StatusTooManyRequests}
	Internal           = Kind{Code: "internal", Status: http.StatusInternalServerError}
	Unavailable        = Kind{Code: "unavailable", Status: http.StatusServiceUnavailable}
	Canceled           = Kind{Code: "canceled", Status: http.StatusServiceUnavailable}
	DeadlineExceeded   = Kind{Code: "deadline_exceeded", Status: http.StatusServiceUnavailable}
	BadGateway         = Kind{Code: "bad_gateway", Status: http.StatusBadGateway}
)

// Error carries structured context through the error chain.
type Error struct {
	Op      string
	Kind    Kind
	Message string
	UserMsg string
	Err     error
}

// KindCarrier is implemented by custom error types that can provide an application kind.
type KindCarrier interface {
	ErrorKind() Kind
}

// UserMessage is safe to expose to callers.
type UserMessage string

// E builds an application error. Arguments are interpreted by type.
func E(args ...any) error {
	var e Error
	for _, arg := range args {
		switch v := arg.(type) {
		case nil:
		case Kind:
			e.Kind = v
		case UserMessage:
			e.UserMsg = string(v)
		case string:
			if e.isEmpty() {
				e.Op = v
			} else {
				e.Message = v
			}
		case error:
			e.Err = Join(e.Err, v)
		default:
			e.Message = fmt.Sprint(v)
		}
	}
	e.promote()
	return &e
}

func (e *Error) isEmpty() bool {
	return e.Op == "" && e.Kind == Unknown && e.Message == "" && e.UserMsg == "" && e.Err == nil
}

// User returns a user-facing message option.
func User(message string) UserMessage {
	return UserMessage(message)
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
	if e.Message != "" {
		parts = append(parts, e.Message)
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
	var appErr *Error
	if As(err, &appErr) {
		return appErr.UserMsg
	}
	return ""
}

// Class returns a low-cardinality error class suitable for logs and metrics.
func Class(err error) string {
	kind := WhatKind(err)
	if kind.Code != "" {
		return kind.Code
	}
	return "error"
}

// LogAttrs returns low-cardinality structured logging attributes for err.
func LogAttrs(err error) []slog.Attr {
	attrs := []slog.Attr{slog.String("error_class", Class(err))}
	return append(attrs, LogDetailAttrs(err)...)
}

// LogDetailAttrs returns structured error details without overriding a caller's error_class.
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
	case http.StatusConflict:
		return Conflict
	case http.StatusPreconditionFailed:
		return FailedPrecondition
	case http.StatusTooManyRequests:
		return ResourceExhausted
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
