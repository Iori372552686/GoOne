// Package gerr is the canonical framework error model for GoOne
// (modelled after kratos/errors, adapted to g1_protocol.ErrorCode).
//
// It unifies the previously coexisting styles:
//   - business error codes (g1_protocol.ErrorCode) carried in packets,
//   - plain Go errors with %w wrapping,
//   - transport-level errors (timeout, closed channels, ...).
//
// A *gerr.Error carries a wire code, a machine-readable reason, a human
// message and an optional cause, so a single value serves logging, metrics
// labelling and packet Ret.Code filling at the same time.
//
// Usage:
//
//	return gerr.New(pb.ErrorCode_ERR_NOT_EXIST_PLAYER, "role_not_found", "role %d not loaded", uid)
//	return gerr.Wrap(pb.ErrorCode_ERR_DB, "redis_set_failed", err)
//	if gerr.Code(err) == pb.ErrorCode_ERR_TIMEOUT { ... }
package gerr

import (
	"errors"
	"fmt"

	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
)

// Error is the canonical GoOne error value.
type Error struct {
	// Code is the wire error code written into packet Ret.Code.
	Code g1_protocol.ErrorCode
	// Reason is a short, stable, machine-readable identifier
	// (snake_case), suitable for metrics labels and log searching.
	Reason string
	// Message is the human-readable description.
	Message string
	// cause is the wrapped underlying error, if any.
	cause error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	switch {
	case e.cause != nil && e.Message != "":
		return fmt.Sprintf("[%s] %s: %s | %v", e.Code, e.Reason, e.Message, e.cause)
	case e.cause != nil:
		return fmt.Sprintf("[%s] %s | %v", e.Code, e.Reason, e.cause)
	case e.Message != "":
		return fmt.Sprintf("[%s] %s: %s", e.Code, e.Reason, e.Message)
	default:
		return fmt.Sprintf("[%s] %s", e.Code, e.Reason)
	}
}

// Unwrap supports errors.Is / errors.As chains.
func (e *Error) Unwrap() error { return e.cause }

// Is treats two *Error values with the same Code and Reason as equal, so
// sentinel errors created by New can be matched with errors.Is.
func (e *Error) Is(target error) bool {
	var t *Error
	if !errors.As(target, &t) {
		return false
	}
	if t.Reason != "" && t.Reason != e.Reason {
		return false
	}
	return t.Code == e.Code
}

// New creates an *Error with a formatted message.
func New(code g1_protocol.ErrorCode, reason, format string, args ...interface{}) *Error {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	return &Error{Code: code, Reason: reason, Message: msg}
}

// Wrap attaches a wire code and reason to an underlying error.
func Wrap(code g1_protocol.ErrorCode, reason string, cause error) *Error {
	return &Error{Code: code, Reason: reason, cause: cause}
}

// WithMessage returns a copy of e with the human message replaced.
func (e *Error) WithMessage(format string, args ...interface{}) *Error {
	if e == nil {
		return nil
	}
	clone := *e
	clone.Message = fmt.Sprintf(format, args...)
	return &clone
}

// Code extracts the wire code from any error:
//   - nil            -> ERR_OK
//   - *gerr.Error    -> its Code
//   - anything else  -> ERR_INTERNAL
func Code(err error) g1_protocol.ErrorCode {
	if err == nil {
		return g1_protocol.ErrorCode_ERR_OK
	}
	var e *Error
	if errors.As(err, &e) && e.Code != 0 {
		return e.Code
	}
	return g1_protocol.ErrorCode_ERR_INTERNAL
}

// Reason extracts the machine-readable reason, or "" when absent.
func Reason(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Reason
	}
	return ""
}

// Common transport/framework sentinel errors. Match with errors.Is.
var (
	// ErrTimeout indicates an RPC or queue wait exceeded its deadline.
	ErrTimeout = New(g1_protocol.ErrorCode_ERR_TIMEOUT, "timeout", "operation timed out")
	// ErrClosed indicates the underlying channel/connection is closed.
	ErrClosed = New(g1_protocol.ErrorCode_ERR_INTERNAL, "closed", "underlying resource is closed")
	// ErrNotFound is the generic "entity does not exist" error.
	ErrNotFound = New(g1_protocol.ErrorCode_ERR_NOT_EXIST, "not_found", "entity not found")
)
