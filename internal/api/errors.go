package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

// This file is the only place an internal error becomes an HTTP status. Every
// handler returns the error it got and calls writeError; none of them chooses
// a status. That is what keeps the mapping consistent, and what makes adding
// a domain.Code a one-line change here rather than a hunt through handlers.

// errorEnvelope is the one shape every failure takes, matching what
// web/src/lib/api/client.js decodes.
type errorEnvelope struct {
	Error errorDTO `json:"error"`
}

type errorDTO struct {
	Code    string     `json:"code"`
	Message string     `json:"message"`
	Fields  []fieldDTO `json:"fields,omitempty"`
}

type fieldDTO struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// codeToStatus maps a domain error code to its HTTP status. Pure, so it can
// be tested exhaustively without a request.
//
// Anything unrecognised falls through to 500 -- domain.CodeOf already
// collapses an unknown error to CodeInternal, and this default covers a code
// added to the domain without a line added here.
func codeToStatus(code domain.Code) int {
	switch code {
	case domain.CodeInvalid:
		return http.StatusBadRequest
	case domain.CodeUnauthenticated:
		return http.StatusUnauthorized
	case domain.CodeForbidden:
		return http.StatusForbidden
	case domain.CodeNotFound:
		return http.StatusNotFound
	case domain.CodeConflict:
		return http.StatusConflict
	case domain.CodeRateLimited:
		return http.StatusTooManyRequests
	case domain.CodeUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// writeError writes the error envelope and sets the status.
//
// The message always comes from domain.UserMessage, never from err.Error().
// UserMessage returns a generic apology for CodeInternal and for anything it
// does not recognise, which is what stops a raw pgx failure putting a
// connection string in a response body. The cause is logged instead, and only
// for CodeInternal: logging every 401 at error level buries real defects
// under routine noise.
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	code := domain.CodeOf(err)
	status := codeToStatus(code)

	if code == domain.CodeInternal {
		slog.ErrorContext(r.Context(), "internal error",
			"error", causeOf(err),
			"request_id", requestIDFromContext(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
		)
	} else {
		slog.DebugContext(r.Context(), "request rejected",
			"code", code,
			"error", causeOf(err),
			"request_id", requestIDFromContext(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
		)
	}

	encode(w, status, errorEnvelope{Error: errorDTO{
		Code:    string(code),
		Message: domain.UserMessage(err),
		Fields:  fieldsFromError(err),
	}})
}

// causeOf unwraps to the underlying failure, for the log only. It is
// deliberately not part of the serialized JSON: the cause is where the
// database driver's own text lives.
func causeOf(err error) error {
	var de *domain.Error
	if errors.As(err, &de) {
		if cause := de.Unwrap(); cause != nil {
			return cause
		}
	}
	return err
}

// fieldsFromError renders per-field validation messages, so a form can put
// each one next to the input it belongs to.
//
// Returning nil rather than an empty slice matters: the `omitempty` tag on
// Fields drops the key entirely for nil, so an error with nothing to say
// about individual fields serializes without a "fields": [] a client would
// have to check.
func fieldsFromError(err error) []fieldDTO {
	var de *domain.Error
	if !errors.As(err, &de) || de.Code == domain.CodeInternal {
		// Nothing about an internal defect is safe to describe to a client,
		// field names included -- UserMessage already refuses to, and this
		// keeps a second route to the same information closed.
		return nil
	}

	fieldErrors := de.FieldErrors()
	if len(fieldErrors) == 0 {
		return nil
	}

	out := make([]fieldDTO, 0, len(fieldErrors))
	for _, fe := range fieldErrors {
		out = append(out, fieldDTO{Field: fe.Field, Message: fe.Message})
	}
	return out
}
