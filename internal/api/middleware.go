package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/google/uuid"
)

// Middleware is a function that wraps one handler in another. That is the
// whole pattern: because http.Handler is a single-method interface, anything
// that can decorate a request or a response is just a function of this shape,
// and a chain of them is function composition.
type Middleware func(http.Handler) http.Handler

// Authenticator is the one method the auth middleware needs. Narrower than
// AuthAPI on purpose: a test for this middleware should not have to fake
// seven methods it never calls.
type Authenticator interface {
	Authenticate(accessToken string) (uuid.UUID, domain.AccountType, error)
}

// statusRecorder remembers what status went out, which the logging middleware
// otherwise has no way to see -- http.ResponseWriter is write-only.
//
// It starts at 200 because a handler that writes a body without calling
// WriteHeader has sent a 200, and the recorder should say the same thing the
// client received.
type statusRecorder struct {
	http.ResponseWriter
	status  int
	written bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.written {
		r.status = code
		r.written = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	r.written = true
	return r.ResponseWriter.Write(b)
}

// withRecovery turns a panic into a 500 instead of a dropped connection.
//
// Outermost in the chain, so it covers every other middleware as well as the
// handlers. It writes through the same writeError as everything else: a
// second, hand-rolled error path here would be the one place the error
// envelope could drift.
func withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			cause, ok := rec.(error)
			if !ok {
				// A panic value is any, and panic("boom") is common enough
				// that it must not crash the recovery path itself.
				cause = fmt.Errorf("%v", rec)
			}
			// http.ErrAbortHandler is the documented way for a handler to
			// give up on a connection deliberately. Swallowing it would turn
			// an intentional abort into a spurious 500 in the log.
			if ok && errors.Is(cause, http.ErrAbortHandler) {
				panic(rec)
			}

			slog.ErrorContext(r.Context(), "panic recovered",
				"error", cause,
				"request_id", requestIDFromContext(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
				"stack", string(debug.Stack()),
			)

			writeError(w, r, domain.Internal(cause, "panic in handler"))
		}()

		next.ServeHTTP(w, r)
	})
}

// withRequestID gives every request an identifier, taken from the caller's
// X-Request-Id when there is one so a trace survives across services, and
// echoes it back so a client can quote it in a bug report.
func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-Id", id)

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

// withLogging writes one line per request.
//
// It sits outside the auth middleware, so it cannot read the user id off its
// own context -- context values only travel inward. Instead it puts an empty
// caller in the context on the way down for withAuth to fill, and reads it
// back after the handler has returned.
func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		ctx, c := withCaller(r.Context())
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r.WithContext(ctx))

		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", requestIDFromContext(ctx),
		}
		if c.userID != uuid.Nil {
			attrs = append(attrs, "user_id", c.userID)
		}

		slog.LogAttrs(ctx, levelForStatus(rec.status), "request", toAttrs(attrs)...)
	})
}

// levelForStatus keeps routine 4xx noise out of the error stream. A 401 on a
// protected route is the system working; a 500 is not, and the two should not
// have to be told apart by reading the status field.
func levelForStatus(status int) slog.Level {
	switch {
	case status >= 500:
		return slog.LevelError
	case status >= 400:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}

func toAttrs(kv []any) []slog.Attr {
	attrs := make([]slog.Attr, 0, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		key, _ := kv[i].(string)
		attrs = append(attrs, slog.Any(key, kv[i+1]))
	}
	return attrs
}

// withCORS grants cross-origin access to the configured origins, and to
// nobody else.
//
// The allow list is echoed back one origin at a time rather than answered
// with "*", because credentials: 'include' -- which the frontend sends, so
// the refresh cookie travels -- is incompatible with a wildcard. Vary: Origin
// keeps a cache from serving one origin's response to another.
func withCORS(allowedOrigins []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowed := origin != "" && slices.Contains(allowedOrigins, origin)

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			// Set whether or not this origin is allowed: the response differs
			// by Origin either way, and a cache that does not know that will
			// serve the wrong one.
			w.Header().Add("Vary", "Origin")

			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				if allowed {
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-Id")
					w.Header().Set("Access-Control-Max-Age", "600")
				}
				// 204 either way. A preflight that was refused is answered by
				// the absence of the allow headers, which is what the browser
				// checks; there is no body to send in either case.
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// withTimeout caps how long a request may run, so one slow query cannot pin a
// connection indefinitely. Every layer beneath this already threads
// context.Context through, so the deadline reaches the database driver
// without anything else having to check for it.
func withTimeout(d time.Duration) Middleware {
	const body = `{"error":{"code":"unavailable","message":"That took too long. Try again."}}`
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, d, body)
	}
}

// withAuth admits only requests carrying a valid access token, and records
// who the caller is for the handlers behind it.
//
// Authenticate verifies the token's signature in process and touches no
// database. That is what makes it cheap enough to sit in front of every
// protected route, and adding a lookup here would give that up for a check
// that a fifteen-minute token barely benefits from.
func withAuth(authenticator Authenticator) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, accountType, err := authenticator.Authenticate(bearerToken(r))
			if err != nil {
				// The message comes from the service, which says the same
				// thing for a missing, malformed and expired token. Telling
				// them apart tells an attacker which of their guesses was
				// shaped correctly.
				writeError(w, r, err)
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			ctx = context.WithValue(ctx, accountTypeKey, accountType)

			// Hand the identity back out to the logging middleware wrapped
			// around this one, which cannot see inward.
			if c, ok := callerFromContext(ctx); ok {
				c.userID = userID
				c.accountType = accountType
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// bearerToken extracts the credential from an Authorization header, or
// returns "" when there isn't one in the expected form. The scheme match is
// case-insensitive because RFC 7235 says it is.
func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}
