// Package api is the HTTP transport layer: it turns requests into service
// calls and service results into JSON.
//
// It imports internal/service and internal/domain, and never internal/postgres
// for anything but the postgres.SessionContext type the service layer already
// insists on (see sessionContext in auth.go). The transport layer does not get
// to know how anything is stored, the same rule internal/service follows about
// pgx one layer down.
//
// The package is named api rather than http so that net/http can keep its own
// name at every use site inside it.
package api

import (
	"context"
	"net/http"
	"time"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/postgres"
	"github.com/RajwatSingh/khel-arena/internal/service"
	"github.com/google/uuid"
)

// The three interfaces below are declared here, next to the handlers that
// call them, rather than in the packages that implement them. Each names only
// the methods this layer actually uses, so *service.AuthService satisfies
// AuthAPI without an adapter and a test fake satisfies it without a database,
// a token issuer, or a clock.

// AuthAPI is the part of service.AuthService the handlers reach for.
type AuthAPI interface {
	Register(ctx context.Context, reg domain.Registration, sc postgres.SessionContext) (service.Session, error)
	Login(ctx context.Context, email, password string, sc postgres.SessionContext) (service.Session, error)
	Refresh(ctx context.Context, refreshToken string, sc postgres.SessionContext) (service.Session, error)
	Logout(ctx context.Context, refreshToken string) error
	Authenticate(accessToken string) (uuid.UUID, domain.AccountType, error)
	BeginPasswordReset(ctx context.Context, email string) (string, domain.User, error)
	CompletePasswordReset(ctx context.Context, resetToken, newPassword string) error
	ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error
}

// BookingAPI is the part of service.BookingService the handlers reach for.
//
// ReleaseStaleHolds is absent on purpose: it belongs to the janitor's ticker,
// not to any request, and nothing reachable over HTTP should be able to
// trigger a full sweep.
type BookingAPI interface {
	Create(ctx context.Context, in service.CreateBookingInput) (domain.Booking, error)
	Availability(ctx context.Context, courtID uuid.UUID, date time.Time) ([]domain.GridSlot, error)
	ListMine(ctx context.Context, userID uuid.UUID, limit int) ([]domain.BookingDetail, error)
	Cancel(ctx context.Context, bookingID, userID uuid.UUID) error
}

// ProfileAPI is the part of service.ProfileService the handlers reach for.
type ProfileAPI interface {
	Me(ctx context.Context, userID uuid.UUID) (domain.User, error)
	Update(ctx context.Context, userID uuid.UUID, p domain.ProfileUpdate) (domain.User, error)
}

// Options is everything a Server needs. A struct rather than a long
// positional signature: six of these are booleans and string slices, and at
// a call site NewServer(a, b, c, nil, true, false, true) says nothing about
// which flag is which.
type Options struct {
	Auth     AuthAPI
	Bookings BookingAPI
	Profiles ProfileAPI

	// Pinger backs /readyz. Optional: leave it nil and readiness reports the
	// process alone, which is the honest answer when there is nothing else to
	// check.
	Pinger Pinger

	// AllowedOrigins are the browser origins CORS accepts. Empty means no
	// cross-origin request is granted, which is correct when the frontend is
	// served from the same origin (the vite proxy in development, one host in
	// production).
	AllowedOrigins []string

	// SecureCookies marks the refresh cookie Secure. Off in development
	// because http://localhost has no TLS; on everywhere else.
	SecureCookies bool

	// LogResetTokens prints password-reset tokens to the log, which is the
	// only way to complete a reset until email delivery exists. Must stay
	// false in production -- see handlePasswordForgot.
	LogResetTokens bool

	// RequestTimeout caps how long a single request may run. Zero picks
	// defaultRequestTimeout.
	RequestTimeout time.Duration
}

// defaultRequestTimeout is generous enough for the slowest of these handlers
// (a login, which deliberately spends time hashing) and far short of the
// write timeout in cmd/api, so the timeout handler is what answers a stuck
// request rather than the connection dying under it.
const defaultRequestTimeout = 10 * time.Second

type Server struct {
	auth     AuthAPI
	bookings BookingAPI
	profiles ProfileAPI
	pinger   Pinger

	allowedOrigins []string
	secureCookies  bool
	logResetTokens bool
	requestTimeout time.Duration
}

func NewServer(opts Options) *Server {
	timeout := opts.RequestTimeout
	if timeout <= 0 {
		timeout = defaultRequestTimeout
	}
	return &Server{
		auth:           opts.Auth,
		bookings:       opts.Bookings,
		profiles:       opts.Profiles,
		pinger:         opts.Pinger,
		allowedOrigins: opts.AllowedOrigins,
		secureCookies:  opts.SecureCookies,
		logResetTokens: opts.LogResetTokens,
		requestTimeout: timeout,
	}
}

// Handler is the whole API as one http.Handler, ready to hand to http.Server.
func (s *Server) Handler() http.Handler { return s.routes() }

// routes is the only place the endpoint list is written out.
func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// Health probes sit outside /v1: they describe the process, not the API,
	// and their shape must not change when the API version does.
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)

	// Public.
	mux.HandleFunc("POST /v1/auth/register", s.handleRegister)
	mux.HandleFunc("POST /v1/auth/login", s.handleLogin)
	mux.HandleFunc("POST /v1/auth/refresh", s.handleRefresh)
	mux.HandleFunc("POST /v1/auth/logout", s.handleLogout)
	mux.HandleFunc("POST /v1/auth/password/forgot", s.handlePasswordForgot)
	mux.HandleFunc("POST /v1/auth/password/reset", s.handlePasswordReset)
	mux.HandleFunc("GET /v1/courts/{courtID}/availability", s.handleAvailability)

	// Authenticated. Each pattern is registered on this same mux, wrapped
	// individually, rather than on a nested mux mounted under a prefix:
	// r.PathValue reads from the pattern that matched, so {bookingID} only
	// resolves if the pattern carrying it is the one this mux matched.
	authed := withAuth(s.auth)
	protected := func(h http.HandlerFunc) http.Handler { return authed(h) }

	mux.Handle("GET /v1/me", protected(s.handleMe))
	mux.Handle("POST /v1/auth/password/change", protected(s.handlePasswordChange))
	mux.Handle("POST /v1/bookings", protected(s.handleCreateBooking))
	mux.Handle("GET /v1/bookings", protected(s.handleListBookings))
	mux.Handle("DELETE /v1/bookings/{bookingID}", protected(s.handleCancelBooking))

	// Outermost first. Recovery has to be able to catch a panic thrown by any
	// of the others, request ID has to exist before anything logs, and CORS
	// has to answer a preflight before the timeout handler starts a clock on
	// a request that will never reach a handler.
	return chain(mux,
		withRecovery,
		withRequestID,
		withLogging,
		withCORS(s.allowedOrigins),
		withTimeout(s.requestTimeout),
	)
}

// chain wraps h in mws, applying them so that mws[0] is outermost.
func chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// currentUser reads the caller's id off the context the auth middleware
// populated.
//
// A miss is not a client error -- it means a route was registered without
// withAuth, which is a wiring bug in routes() above. It reports 500 rather
// than 401 so it reads as "we are broken", not "you are not signed in".
func (s *Server) currentUser(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		writeError(w, r, domain.Internal(nil, "no user id in context on a protected route: %s %s", r.Method, r.URL.Path))
		return uuid.Nil, false
	}
	return userID, true
}
