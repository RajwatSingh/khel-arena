package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

// maxRequestBody is the largest JSON body any endpoint accepts. Every request
// this API takes is a handful of fields; a body larger than this is either a
// bug or an attempt to make the server buffer something enormous.
const maxRequestBody = 64 << 10 // 64 KiB

// decode reads a JSON request body into T.
//
// Unknown fields are rejected rather than ignored, so a client's typo'd field
// name is a 400 it can see rather than a value that silently never arrives.
// That matters most on this API's write endpoints: a booking sent with
// "start_at" instead of "starts_at" should not quietly become a booking for
// the zero time.
//
// Every failure is domain.CodeInvalid, and none of them repeats the parser's
// own message. encoding/json says things like "invalid character 'x' looking
// for beginning of value" that describe our decoder, not the caller's
// mistake.
func decode[T any](w http.ResponseWriter, r *http.Request) (T, error) {
	var v T

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&v); err != nil {
		return v, decodeError(err)
	}

	// A second token after the first value means the caller sent two JSON
	// documents in one body. Accepting that would mean silently ignoring
	// whatever came after the first.
	if err := dec.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return v, domain.Invalid("", "Send one JSON object.")
	}

	return v, nil
}

func decodeError(err error) error {
	var (
		syntax          *json.SyntaxError
		unmarshalType   *json.UnmarshalTypeError
		maxBytesReached *http.MaxBytesError
	)

	switch {
	case errors.Is(err, io.EOF):
		return domain.Invalid("", "Send a JSON body.")
	case errors.As(err, &maxBytesReached):
		return domain.Invalid("", "That request is too large.")
	case errors.As(err, &unmarshalType):
		// The one case where the parser knows which field went wrong, so the
		// error can be attached to it and the form can highlight it.
		return domain.Invalid(unmarshalType.Field, "That value isn't in the right format.").WithCause(err)
	case errors.As(err, &syntax):
		return domain.Invalid("", "That isn't valid JSON.").WithCause(err)
	default:
		// Includes DisallowUnknownFields' error, which names the offending
		// field only inside an unstructured string. Reflecting that string
		// back would echo client input into a response, so it stays in the
		// cause, where writeError can log it without serializing it.
		return domain.Invalid("", "We couldn't read that request.").WithCause(err)
	}
}

// encode writes a JSON success response.
//
// Success bodies are the bare resource, with no envelope: a booking is
// {"id": ...}, not {"data": {"id": ...}}. The status line already says
// whether this is an error, so a wrapper would only add a key every client
// has to step through.
func encode(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		// The status and headers are already gone, so there is no way to turn
		// this into an error response -- the client will see a truncated
		// body. All that is left is to make sure it is not invisible to us.
		slog.Error("encoding response body", "error", err)
	}
}
