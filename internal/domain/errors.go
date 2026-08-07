// Package domain holds the vocabulary of the business: the entities, the
// values they are built from, and the rules that hold regardless of how the
// service is deployed.
//
// Nothing here imports a database driver, an HTTP framework, or a logger.
// That is the point: these rules can be read and tested on their own, and the
// layers around them can be replaced without renegotiating what a booking is.
package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Code classifies a failure by what the caller should do about it, not by
// where it happened. The transport layer maps codes to status codes; no
// handler needs to know which business rule produced one.
type Code string

const (
	// CodeInvalid means the request was malformed or violated a rule that
	// the caller can see and correct.
	CodeInvalid Code = "invalid"
	// CodeUnauthenticated means no valid identity was presented.
	CodeUnauthenticated Code = "unauthenticated"
	// CodeForbidden means the identity is valid but not allowed to do this.
	CodeForbidden Code = "forbidden"
	// CodeNotFound means the addressed thing does not exist, or the caller
	// is not permitted to know that it does.
	CodeNotFound Code = "not_found"
	// CodeConflict means the request collided with the current state --
	// a duplicate username, a slot someone else just took.
	CodeConflict Code = "conflict"
	// CodeRateLimited means the caller is going too fast.
	CodeRateLimited Code = "rate_limited"
	// CodeUnavailable means a dependency this request needed is down. It is
	// the only code that invites a retry of the identical request.
	CodeUnavailable Code = "unavailable"
	// CodeInternal means a defect. The message is never shown to a user.
	CodeInternal Code = "internal"
)

// Error is the single error type crossing layer boundaries.
//
// Message is written for the person who will read it and is safe to return
// over the wire. Anything sensitive belongs in the wrapped cause, which is
// logged but never serialised.
type Error struct {
	Code    Code
	Message string
	// Field names the offending input for validation failures, so a client
	// can attach the message to the right form control.
	Field string
	// Fields carries the individual failures when one submission was wrong
	// in several places. Message is their readable summary; this is what a
	// client renders against each control. Empty for single-field errors,
	// where Field and Message already say everything.
	Fields []*Error
	cause  error
}

// FieldErrors returns the per-field failures behind this error. A single-field
// error reports itself, so callers can treat both shapes the same way.
func (e *Error) FieldErrors() []*Error {
	if len(e.Fields) > 0 {
		out := make([]*Error, len(e.Fields))
		copy(out, e.Fields)
		return out
	}
	if e.Field != "" {
		return []*Error{e}
	}
	return nil
}

func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString(string(e.Code))
	b.WriteString(": ")
	b.WriteString(e.Message)
	if e.Field != "" {
		b.WriteString(" (field: " + e.Field + ")")
	}
	if e.cause != nil {
		b.WriteString(": " + e.cause.Error())
	}
	return b.String()
}

func (e *Error) Unwrap() error { return e.cause }

// Is lets errors.Is compare by code alone, so callers can write
// errors.Is(err, domain.ErrNotFound) without constructing a full Error.
func (e *Error) Is(target error) bool {
	var t *Error
	if !errors.As(target, &t) {
		return false
	}
	return t.Code == e.Code && (t.Message == "" || t.Message == e.Message)
}

// WithCause attaches the underlying failure. The cause is for operators;
// it never reaches the client.
func (e *Error) WithCause(err error) *Error {
	clone := *e
	clone.cause = err
	return &clone
}

// Sentinels for errors.Is comparison. These carry a code and no message, so
// they match any Error sharing the code.
var (
	ErrInvalid         = &Error{Code: CodeInvalid}
	ErrUnauthenticated = &Error{Code: CodeUnauthenticated}
	ErrForbidden       = &Error{Code: CodeForbidden}
	ErrNotFound        = &Error{Code: CodeNotFound}
	ErrConflict        = &Error{Code: CodeConflict}
	ErrRateLimited     = &Error{Code: CodeRateLimited}
	ErrUnavailable     = &Error{Code: CodeUnavailable}
	ErrInternal        = &Error{Code: CodeInternal}
)

// Constructors. Each takes a message already written for a human.

func Invalid(field, format string, args ...any) *Error {
	return &Error{Code: CodeInvalid, Field: field, Message: fmt.Sprintf(format, args...)}
}

func Unauthenticated(format string, args ...any) *Error {
	return &Error{Code: CodeUnauthenticated, Message: fmt.Sprintf(format, args...)}
}

func Forbidden(format string, args ...any) *Error {
	return &Error{Code: CodeForbidden, Message: fmt.Sprintf(format, args...)}
}

func NotFound(format string, args ...any) *Error {
	return &Error{Code: CodeNotFound, Message: fmt.Sprintf(format, args...)}
}

func Conflict(format string, args ...any) *Error {
	return &Error{Code: CodeConflict, Message: fmt.Sprintf(format, args...)}
}

func RateLimited(format string, args ...any) *Error {
	return &Error{Code: CodeRateLimited, Message: fmt.Sprintf(format, args...)}
}

func Unavailable(format string, args ...any) *Error {
	return &Error{Code: CodeUnavailable, Message: fmt.Sprintf(format, args...)}
}

// Internal wraps a defect. The message is deliberately generic because it may
// be shown to a user; the real detail travels in the cause, for the log.
func Internal(cause error, format string, args ...any) *Error {
	return &Error{
		Code:    CodeInternal,
		Message: "Something went wrong on our end. Please try again.",
		cause:   fmt.Errorf(format+": %w", append(args, cause)...),
	}
}

// CodeOf reports the classification of any error. Errors that did not come
// from this package are treated as defects, which is the safe default: an
// unclassified error is one nobody decided was safe to show.
func CodeOf(err error) Code {
	if err == nil {
		return ""
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return CodeInternal
}

// UserMessage returns text safe to show the caller. Unclassified errors and
// internal defects collapse to a generic apology so a stray driver error can
// never leak a connection string or a query.
func UserMessage(err error) string {
	if err == nil {
		return ""
	}
	var e *Error
	if errors.As(err, &e) && e.Code != CodeInternal {
		return e.Message
	}
	return "Something went wrong on our end. Please try again."
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

// Validation collects field errors so a caller learns everything wrong with a
// submission at once, rather than fixing one field per round-trip.
type Validation struct {
	fields []*Error
}

// Check records a failure for `field` when cond is false. It returns the
// Validation so checks can be chained.
func (v *Validation) Check(cond bool, field, format string, args ...any) *Validation {
	if !cond {
		v.fields = append(v.fields, Invalid(field, format, args...))
	}
	return v
}

// Add records an already-built field error.
func (v *Validation) Add(field, format string, args ...any) *Validation {
	v.fields = append(v.fields, Invalid(field, format, args...))
	return v
}

func (v *Validation) OK() bool { return len(v.fields) == 0 }

// Fields exposes the individual failures so a transport layer can render them
// per-control. The slice is a copy; mutating it does not affect the Validation.
func (v *Validation) Fields() []*Error {
	out := make([]*Error, len(v.fields))
	copy(out, v.fields)
	return out
}

// Err returns nil when every check passed, otherwise an Error summarising
// them. The first failure supplies Field, so single-field submissions behave
// exactly like a direct Invalid().
func (v *Validation) Err() error {
	switch len(v.fields) {
	case 0:
		return nil
	case 1:
		return v.fields[0]
	default:
		msgs := make([]string, 0, len(v.fields))
		for _, f := range v.fields {
			msgs = append(msgs, f.Message)
		}
		return &Error{
			Code:    CodeInvalid,
			Field:   v.fields[0].Field,
			Message: strings.Join(msgs, " "),
			Fields:  v.Fields(),
		}
	}
}
