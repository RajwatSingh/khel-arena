package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

func TestCodeToStatus(t *testing.T) {
	cases := map[domain.Code]int{
		domain.CodeInvalid:         http.StatusBadRequest,
		domain.CodeUnauthenticated: http.StatusUnauthorized,
		domain.CodeForbidden:       http.StatusForbidden,
		domain.CodeNotFound:        http.StatusNotFound,
		domain.CodeConflict:        http.StatusConflict,
		domain.CodeRateLimited:     http.StatusTooManyRequests,
		domain.CodeUnavailable:     http.StatusServiceUnavailable,
		domain.CodeInternal:        http.StatusInternalServerError,
		// A code added to the domain without a line added here must not
		// become a 200 by accident.
		domain.Code("bogus"): http.StatusInternalServerError,
	}

	for code, want := range cases {
		if got := codeToStatus(code); got != want {
			t.Errorf("codeToStatus(%q) = %d, want %d", code, got, want)
		}
	}
}

// writeErrorFor runs writeError over one error and decodes what came out.
func writeErrorFor(t *testing.T, err error) (*httptest.ResponseRecorder, errorEnvelope) {
	t.Helper()

	w := httptest.NewRecorder()
	writeError(w, httptest.NewRequest(http.MethodGet, "/v1/anything", nil), err)

	var got errorEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding error envelope %q: %v", w.Body.String(), err)
	}
	return w, got
}

func TestWriteErrorStatusAndCode(t *testing.T) {
	w, got := writeErrorFor(t, domain.NotFound("No booking with that reference."))

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if got.Error.Code != string(domain.CodeNotFound) {
		t.Errorf("code = %q, want %q", got.Error.Code, domain.CodeNotFound)
	}
	if got.Error.Message != "No booking with that reference." {
		t.Errorf("message = %q, want the domain error's own message", got.Error.Message)
	}
}

// The leak this test guards is the whole reason writeError reads
// domain.UserMessage instead of err.Error(): a driver failure carries a query
// and sometimes a connection string, and neither may reach a client.
func TestWriteErrorNeverLeaksInternalCause(t *testing.T) {
	const secret = "pgx: dial postgres://khel:hunter2@10.0.0.4:5432"

	cases := map[string]error{
		"wrapped internal": domain.Internal(errors.New(secret), "loading court context"),
		"unclassified":     errors.New(secret),
	}

	for name, err := range cases {
		t.Run(name, func(t *testing.T) {
			w, got := writeErrorFor(t, err)

			if w.Code != http.StatusInternalServerError {
				t.Errorf("status = %d, want 500", w.Code)
			}
			if strings.Contains(w.Body.String(), "hunter2") || strings.Contains(w.Body.String(), "pgx") {
				t.Errorf("response body leaked the cause: %s", w.Body.String())
			}
			if got.Error.Message != domain.UserMessage(err) {
				t.Errorf("message = %q, want the generic apology", got.Error.Message)
			}
			if got.Error.Fields != nil {
				t.Errorf("fields = %v, want none for an internal error", got.Error.Fields)
			}
		})
	}
}

func TestWriteErrorRendersSingleFieldError(t *testing.T) {
	_, got := writeErrorFor(t, domain.Invalid("password", "Use at least 10 characters."))

	if len(got.Error.Fields) != 1 {
		t.Fatalf("fields = %v, want exactly one", got.Error.Fields)
	}
	if got.Error.Fields[0].Field != "password" {
		t.Errorf("field = %q, want password", got.Error.Fields[0].Field)
	}
	if got.Error.Fields[0].Message != "Use at least 10 characters." {
		t.Errorf("message = %q", got.Error.Fields[0].Message)
	}
}

func TestWriteErrorRendersMultiFieldError(t *testing.T) {
	v := &domain.Validation{}
	v.Add("password", "Use at least 10 characters.")
	v.Add("username", "Usernames need at least 3 characters.")

	_, got := writeErrorFor(t, v.Err())

	if len(got.Error.Fields) != 2 {
		t.Fatalf("fields = %v, want two", got.Error.Fields)
	}
	want := map[string]string{
		"password": "Use at least 10 characters.",
		"username": "Usernames need at least 3 characters.",
	}
	for _, f := range got.Error.Fields {
		if want[f.Field] != f.Message {
			t.Errorf("field %q = %q, want %q", f.Field, f.Message, want[f.Field])
		}
	}
}

// omitempty on a nil slice is what keeps the key out entirely. A `"fields":
// []` would make every client check a key that never means anything.
func TestWriteErrorOmitsEmptyFields(t *testing.T) {
	w, _ := writeErrorFor(t, domain.Unauthenticated("Please sign in."))

	if strings.Contains(w.Body.String(), "fields") {
		t.Errorf("body carries a fields key with nothing in it: %s", w.Body.String())
	}
}
