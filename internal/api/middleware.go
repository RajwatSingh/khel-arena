package api

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/service"
	"github.com/google/uuid"
)

type Middleware func(http.Handler) http.Handler

type Authenticator interface {
	Authenticate(accessToken string) (uuid.UUID, domain.AccountType, error)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				cause, _ := rec.(error)
				slog.ErrorContext(r.Context(), "panic recovered",
					"error", cause,
					"request_id", requestIDFromContext(r.Context()),
					"method", r.Method,
					"path", r.URL.Path,
				)

				writeError(w, r, domain.Internal(cause, "panic in handler"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-Id", id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		userID, _ := userIDFromContext(r.Context())
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration", time.Since(start),
			"request_id", requestIDFromContext(r.Context()),
			"user_id", userID,
		)
	})
}

func withCORS(allowedOrigins []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Origin")
			if slices.Contains(allowedOrigins, header) {
				w.Header().Set("Access-Control-Allow-Origin", header)
				w.Header().Set("Vary", "Origin")
			}

			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.Header().Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func withTimeout(d time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, d, `{"error":{"code":"unavailable","message":"request timed out"}}`)
	}
}

func withAuth(authService *service.AuthService) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerToken(r)
			userID, accountType, err := authService.Authenticate(token)
			if err != nil {
				writeError(w, r, err)
				return
			}
			ctx := context.WithValue(r.Context(), userIDKey, userID)
			ctx = context.WithValue(ctx, accountTypeKey, accountType)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	const prefix = "Bearer "

	if !strings.HasPrefix(header, prefix) {
		return ""
	}

	return strings.TrimPrefix(header, prefix)
}
