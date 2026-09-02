package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/google/uuid"
)

// okHandler records that it ran, so a test can tell "the middleware rejected
// the request" from "the middleware let it through and the handler said no".
func okHandler(reached *bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		*reached = true
		w.WriteHeader(http.StatusOK)
	}
}

func TestWithAuth(t *testing.T) {
	const good = "valid.access.token"

	cases := []struct {
		name       string
		header     string
		wantStatus int
		wantThru   bool
	}{
		{"valid token", "Bearer " + good, http.StatusOK, true},
		{"lowercase scheme", "bearer " + good, http.StatusOK, true},
		{"expired token", "Bearer expired", http.StatusUnauthorized, false},
		{"malformed token", "Bearer not-a-jwt", http.StatusUnauthorized, false},
		{"missing header", "", http.StatusUnauthorized, false},
		{"wrong scheme", "Basic " + good, http.StatusUnauthorized, false},
		{"bare token, no scheme", good, http.StatusUnauthorized, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			auth := &fakeAuth{t: t, authenticate: func(token string) (uuid.UUID, domain.AccountType, error) {
				switch token {
				case good:
					return testUser.ID, testUser.AccountType, nil
				case "expired":
					return uuid.Nil, "", domain.Unauthenticated("Your session has expired.")
				default:
					return uuid.Nil, "", domain.Unauthenticated("Please sign in.")
				}
			}}

			reached := false
			h := withAuth(auth)(okHandler(&reached))

			r := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
			if tc.header != "" {
				r.Header.Set("Authorization", tc.header)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)

			if w.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tc.wantStatus)
			}
			if reached != tc.wantThru {
				t.Errorf("handler reached = %v, want %v", reached, tc.wantThru)
			}
		})
	}
}

// The identity has to arrive in the context, or every protected handler would
// have to re-parse the token itself.
func TestWithAuthPutsCallerInContext(t *testing.T) {
	auth := &fakeAuth{t: t, authenticate: signedIn("token")}

	var (
		gotUserID  uuid.UUID
		gotAccount domain.AccountType
		okUser     bool
		okAccount  bool
	)
	h := withAuth(auth)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID, okUser = userIDFromContext(r.Context())
		gotAccount, okAccount = accountTypeFromContext(r.Context())
	}))

	r := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	r.Header.Set("Authorization", "Bearer token")
	h.ServeHTTP(httptest.NewRecorder(), r)

	if !okUser || gotUserID != testUser.ID {
		t.Errorf("user id in context = %v (ok=%v), want %v", gotUserID, okUser, testUser.ID)
	}
	if !okAccount || gotAccount != testUser.AccountType {
		t.Errorf("account type in context = %v (ok=%v), want %v", gotAccount, okAccount, testUser.AccountType)
	}
}

// Context values only travel inward, so the logging middleware wrapped around
// withAuth can only learn who the caller is through the shared caller record.
func TestWithAuthReportsCallerOutward(t *testing.T) {
	auth := &fakeAuth{t: t, authenticate: signedIn("token")}

	var seen *caller
	outer := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, c := withCaller(r.Context())
			seen = c
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	reached := false
	h := chain(okHandler(&reached), outer, withAuth(auth))

	r := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	r.Header.Set("Authorization", "Bearer token")
	h.ServeHTTP(httptest.NewRecorder(), r)

	if seen == nil || seen.userID != testUser.ID {
		t.Fatalf("caller seen by the outer middleware = %+v, want user %v", seen, testUser.ID)
	}
}

func TestWithRecoveryTurnsPanicIntoCleanError(t *testing.T) {
	cases := map[string]func(){
		"error value":  func() { panic(domain.Internal(nil, "deliberate")) },
		"string value": func() { panic("deliberate: postgres://khel:hunter2@db") },
	}

	for name, boom := range cases {
		t.Run(name, func(t *testing.T) {
			h := withRecovery(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { boom() }))

			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/bookings", nil))

			if w.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500", w.Code)
			}

			var env errorEnvelope
			if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
				t.Fatalf("panic response is not the error envelope: %q", w.Body.String())
			}
			if env.Error.Code != string(domain.CodeInternal) {
				t.Errorf("code = %q, want internal", env.Error.Code)
			}
			// The panic value reaches the log, never the client.
			if got := w.Body.String(); strings.Contains(got, "hunter2") || strings.Contains(got, "deliberate") {
				t.Errorf("panic detail leaked into the response: %s", got)
			}
		})
	}
}

func TestWithRequestID(t *testing.T) {
	t.Run("generates one when absent", func(t *testing.T) {
		var inHandler string
		h := withRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			inHandler = requestIDFromContext(r.Context())
		}))

		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))

		if inHandler == "" {
			t.Error("no request id in the handler's context")
		}
		if got := w.Header().Get("X-Request-Id"); got != inHandler {
			t.Errorf("echoed id = %q, want the one in context %q", got, inHandler)
		}
	})

	t.Run("keeps an inbound one", func(t *testing.T) {
		const given = "trace-from-the-edge"

		var inHandler string
		h := withRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			inHandler = requestIDFromContext(r.Context())
		}))

		r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		r.Header.Set("X-Request-Id", given)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)

		if inHandler != given {
			t.Errorf("request id = %q, want the inbound %q", inHandler, given)
		}
		if got := w.Header().Get("X-Request-Id"); got != given {
			t.Errorf("echoed id = %q, want %q", got, given)
		}
	})
}

func TestWithCORS(t *testing.T) {
	const allowed = "http://localhost:5173"

	t.Run("allowed origin gets credentials", func(t *testing.T) {
		reached := false
		h := withCORS([]string{allowed})(okHandler(&reached))

		r := httptest.NewRequest(http.MethodGet, "/v1/bookings", nil)
		r.Header.Set("Origin", allowed)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)

		if got := w.Header().Get("Access-Control-Allow-Origin"); got != allowed {
			t.Errorf("Allow-Origin = %q, want %q", got, allowed)
		}
		// A wildcard would be incompatible with the refresh cookie, so the
		// exact origin plus this header is the only combination that works.
		if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
			t.Errorf("Allow-Credentials = %q, want true", got)
		}
		if !reached {
			t.Error("a normal request should still reach the handler")
		}
	})

	t.Run("unknown origin gets nothing", func(t *testing.T) {
		reached := false
		h := withCORS([]string{allowed})(okHandler(&reached))

		r := httptest.NewRequest(http.MethodGet, "/v1/bookings", nil)
		r.Header.Set("Origin", "https://not-us.example")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)

		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("Allow-Origin = %q, want it unset", got)
		}
		if w.Header().Get("Vary") != "Origin" {
			t.Error("Vary: Origin must be set either way, or a cache will cross the two answers")
		}
	})

	t.Run("preflight is answered without reaching the handler", func(t *testing.T) {
		reached := false
		h := withCORS([]string{allowed})(okHandler(&reached))

		r := httptest.NewRequest(http.MethodOptions, "/v1/bookings", nil)
		r.Header.Set("Origin", allowed)
		r.Header.Set("Access-Control-Request-Method", http.MethodPost)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)

		if w.Code != http.StatusNoContent {
			t.Errorf("status = %d, want 204", w.Code)
		}
		if reached {
			t.Error("a preflight should not reach the handler")
		}
		if got := w.Header().Get("Access-Control-Allow-Headers"); got == "" {
			t.Error("preflight did not say which headers are allowed")
		}
	})
}
