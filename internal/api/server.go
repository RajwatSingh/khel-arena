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
	"strings"
	"time"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/platform/payment"
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

// ArenaAPI is the part of service.ArenaService the handlers reach for.
//
// Reads only. Owner-facing writes are separate work with an authorization
// story of their own, and this interface should not quietly grow them.
type ArenaAPI interface {
	List(ctx context.Context) ([]postgres.ArenaListing, error)
	BySlug(ctx context.Context, slug string) (postgres.ArenaDetail, error)
	ListAreas(ctx context.Context) ([]string, error)
	CityLedger(ctx context.Context, date time.Time, sport domain.Sport, area string) (service.Ledger, error)
}

// PaymentAPI is the part of service.PaymentService the handlers reach for.
//
// Note what is absent: nothing here lets a caller assert that a payment
// succeeded. Settle takes a callback reference and reports what the gateway
// said; there is no method that marks a booking paid on a client's word.
type PaymentAPI interface {
	Providers() []domain.PaymentProvider
	Checkout(ctx context.Context, bookingID, userID uuid.UUID, provider domain.PaymentProvider) (payment.Checkout, domain.Payment, error)
	Settle(ctx context.Context, provider domain.PaymentProvider, ref payment.CallbackRef) (domain.Payment, error)
	Status(ctx context.Context, bookingID, userID uuid.UUID) (domain.Payment, error)
}

// OwnerAPI is the part of service.OwnerService the handlers reach for.
//
// The first interface here whose methods all take an owner id. Every write
// behind it reaches SQL carrying the owner predicate, so a handler that forgot
// its own check still could not touch another owner's arena.
type OwnerAPI interface {
	MyArenas(ctx context.Context, ownerID uuid.UUID) ([]postgres.ArenaListing, error)
	CreateArena(ctx context.Context, ownerID uuid.UUID, a domain.Arena) (domain.Arena, error)
	UpdateArena(ctx context.Context, arenaID, ownerID uuid.UUID, a domain.Arena) (domain.Arena, error)
	SetArenaActive(ctx context.Context, arenaID, ownerID uuid.UUID, active bool) error

	CreateCourt(ctx context.Context, ownerID uuid.UUID, c domain.Court, format string) (postgres.CourtWithRules, error)
	UpdateCourt(ctx context.Context, courtID, ownerID uuid.UUID, c domain.Court, format string) (postgres.CourtWithRules, error)
	SetCourtActive(ctx context.Context, courtID, ownerID uuid.UUID, active bool) error

	CreatePricingRule(ctx context.Context, ownerID uuid.UUID, rule domain.PricingRule) (domain.PricingRule, error)
	DeletePricingRule(ctx context.Context, ruleID, ownerID uuid.UUID) error

	Payments(ctx context.Context, arenaID, ownerID uuid.UUID, limit int) ([]postgres.OwnerPayment, error)
	MarkCashReceived(ctx context.Context, paymentID, ownerID uuid.UUID) (domain.Payment, error)
}

// TeamAPI is the part of service.TeamService the handlers reach for.
type TeamAPI interface {
	MyTeams(ctx context.Context, userID uuid.UUID) ([]domain.Team, error)
	Create(ctx context.Context, captainID uuid.UUID, t domain.Team) (domain.Team, error)
	Get(ctx context.Context, teamID, viewerID uuid.UUID) (service.TeamWithRoster, error)
	Update(ctx context.Context, teamID, actorID uuid.UUID, t domain.Team) (domain.Team, error)
	Join(ctx context.Context, userID uuid.UUID, code string) (domain.Team, error)
	AddMember(ctx context.Context, teamID, actorID, userID uuid.UUID) error
	RemoveMember(ctx context.Context, teamID, actorID, targetID uuid.UUID) error
	TransferCaptaincy(ctx context.Context, teamID, actorID, targetID uuid.UUID) error
	RotateJoinCode(ctx context.Context, teamID, actorID uuid.UUID) (string, error)
	Disband(ctx context.Context, teamID, actorID uuid.UUID) error
}

// MatchmakingAPI is the part of service.MatchmakingService the handlers use.
type MatchmakingAPI interface {
	Feed(ctx context.Context, f postgres.CallFilter) ([]domain.Call, error)
	MyCalls(ctx context.Context, userID uuid.UUID) ([]domain.Call, error)
	Create(ctx context.Context, authorID uuid.UUID, c domain.Call) (domain.Call, error)
	Get(ctx context.Context, callID, viewerID uuid.UUID) (service.CallWithResponses, error)
	Update(ctx context.Context, callID, actorID uuid.UUID, c domain.Call) (domain.Call, error)
	Cancel(ctx context.Context, callID, actorID uuid.UUID) error
	Delete(ctx context.Context, callID, actorID uuid.UUID) error
	Respond(ctx context.Context, callID, userID uuid.UUID, message string) error
	Accept(ctx context.Context, callID, actorID, userID uuid.UUID) error
	Withdraw(ctx context.Context, callID, userID uuid.UUID) error
}

// TournamentAPI is the part of service.TournamentService the handlers use.
type TournamentAPI interface {
	List(ctx context.Context, limit int) ([]domain.Tournament, error)
	Get(ctx context.Context, slug string) (service.TournamentWithEntries, error)
	Create(ctx context.Context, organizerID uuid.UUID, t domain.Tournament) (domain.Tournament, error)
	Register(ctx context.Context, tournamentID, teamID, actorID uuid.UUID) error
	Withdraw(ctx context.Context, tournamentID, teamID, actorID uuid.UUID) error
	SetEntryPaid(ctx context.Context, tournamentID, teamID, actorID uuid.UUID, paid bool) error
	SetStatus(ctx context.Context, tournamentID, actorID uuid.UUID, status domain.TournamentStatus) error
}

// Mailer sends the reset link. One method, because that is the only message
// this layer triggers.
type Mailer interface {
	SendPasswordReset(ctx context.Context, user domain.User, token string)
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
	Auth        AuthAPI
	Bookings    BookingAPI
	Profiles    ProfileAPI
	Arenas      ArenaAPI
	Payments    PaymentAPI
	Owner       OwnerAPI
	Teams       TeamAPI
	Calls       MatchmakingAPI
	Tournaments TournamentAPI

	// Mailer delivers the password-reset link. Optional: with none, the token
	// is only logged (see LogResetTokens), which is the development path.
	Mailer Mailer

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

	// AppURL is the interface's own origin. A payer coming back from a
	// gateway is redirected here -- built from configuration and never from
	// the request, because a redirect target taken from a query parameter is
	// an open redirect on a URL gateways will send anyone to.
	AppURL string

	// LoginRateLimit throttles the two endpoints an attacker can grind on.
	// The zero value applies defaultLoginRate; set Disabled to turn it off,
	// which is what the handler tests want.
	LoginRateLimit RateLimit
}

// defaultRequestTimeout is generous enough for the slowest of these handlers
// (a login, which deliberately spends time hashing) and far short of the
// write timeout in cmd/api, so the timeout handler is what answers a stuck
// request rather than the connection dying under it.
const defaultRequestTimeout = 10 * time.Second

type Server struct {
	auth        AuthAPI
	bookings    BookingAPI
	profiles    ProfileAPI
	arenas      ArenaAPI
	payments    PaymentAPI
	owner       OwnerAPI
	teams       TeamAPI
	calls       MatchmakingAPI
	tournaments TournamentAPI
	mailer      Mailer
	pinger      Pinger

	appURL string

	allowedOrigins []string
	secureCookies  bool
	logResetTokens bool
	requestTimeout time.Duration

	// loginLimiter throttles the two endpoints worth grinding on. Held on the
	// Server rather than built inside routes() so its buckets survive for the
	// life of the process, which is the only way a rate limit means anything.
	loginLimiter *limiter
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
		arenas:         opts.Arenas,
		payments:       opts.Payments,
		owner:          opts.Owner,
		teams:          opts.Teams,
		calls:          opts.Calls,
		tournaments:    opts.Tournaments,
		mailer:         opts.Mailer,
		pinger:         opts.Pinger,
		appURL:         strings.TrimRight(opts.AppURL, "/"),
		allowedOrigins: opts.AllowedOrigins,
		secureCookies:  opts.SecureCookies,
		logResetTokens: opts.LogResetTokens,
		requestTimeout: timeout,
		loginLimiter:   newLimiter(opts.LoginRateLimit, nil),
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
	//
	// Login and password/forgot are throttled: the first is the online
	// password-guessing target, the second sprays somebody's inbox on
	// request. Everything else here is a read whose cost is a cached query.
	throttle := withRateLimit(s.loginLimiter)

	mux.HandleFunc("POST /v1/auth/register", s.handleRegister)
	mux.Handle("POST /v1/auth/login", throttle(s.handleLogin))
	mux.HandleFunc("POST /v1/auth/refresh", s.handleRefresh)
	mux.HandleFunc("POST /v1/auth/logout", s.handleLogout)
	mux.Handle("POST /v1/auth/password/forgot", throttle(s.handlePasswordForgot))
	mux.HandleFunc("POST /v1/auth/password/reset", s.handlePasswordReset)

	mux.HandleFunc("GET /v1/arenas", s.handleListArenas)
	mux.HandleFunc("GET /v1/arenas/{slug}", s.handleGetArena)
	mux.HandleFunc("GET /v1/areas", s.handleListAreas)
	mux.HandleFunc("GET /v1/ledger", s.handleLedger)
	mux.HandleFunc("GET /v1/courts/{courtID}/availability", s.handleAvailability)
	mux.HandleFunc("GET /v1/payments/providers", s.handleListProviders)
	mux.HandleFunc("GET /v1/calls", s.handleCallFeed)
	mux.HandleFunc("GET /v1/tournaments", s.handleListTournaments)
	mux.HandleFunc("GET /v1/tournaments/{slug}", s.handleGetTournament)

	// The gateway sends the player's browser here. Public by necessity -- the
	// redirect carries no access token -- and safe because nothing in the
	// request decides the outcome: see handlePaymentCallback.
	mux.HandleFunc("GET /v1/payments/{provider}/callback", s.handlePaymentCallback)

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
	mux.Handle("POST /v1/bookings/{bookingID}/checkout", protected(s.handleCreateCheckout))
	mux.Handle("GET /v1/bookings/{bookingID}/payment", protected(s.handlePaymentStatus))

	// Owner-facing. Under its own prefix so the boundary is visible in the
	// route table as well as in the handlers: everything here is scoped to
	// arenas the caller owns.
	mux.Handle("GET /v1/owner/arenas", protected(s.handleMyArenas))
	mux.Handle("POST /v1/owner/arenas", protected(s.handleCreateArena))
	mux.Handle("PATCH /v1/owner/arenas/{arenaID}", protected(s.handleUpdateArena))
	mux.Handle("PUT /v1/owner/arenas/{arenaID}/active", protected(s.handleSetArenaActive))
	mux.Handle("POST /v1/owner/arenas/{arenaID}/courts", protected(s.handleCreateCourt))
	mux.Handle("GET /v1/owner/arenas/{arenaID}/payments", protected(s.handleOwnerPayments))
	mux.Handle("PATCH /v1/owner/courts/{courtID}", protected(s.handleUpdateCourt))
	mux.Handle("PUT /v1/owner/courts/{courtID}/active", protected(s.handleSetCourtActive))
	mux.Handle("POST /v1/owner/courts/{courtID}/pricing", protected(s.handleCreatePricingRule))
	mux.Handle("DELETE /v1/owner/pricing/{ruleID}", protected(s.handleDeletePricingRule))
	mux.Handle("POST /v1/owner/payments/{paymentID}/received", protected(s.handleMarkCashReceived))

	// Teams. All authenticated: a squad is a group of people, and there is
	// nothing useful to show somebody who is not one of them.
	mux.Handle("GET /v1/teams", protected(s.handleMyTeams))
	mux.Handle("POST /v1/teams", protected(s.handleCreateTeam))
	mux.Handle("POST /v1/teams/join", protected(s.handleJoinTeam))
	mux.Handle("GET /v1/teams/{teamID}", protected(s.handleGetTeam))
	mux.Handle("PATCH /v1/teams/{teamID}", protected(s.handleUpdateTeam))
	mux.Handle("DELETE /v1/teams/{teamID}", protected(s.handleDisbandTeam))
	mux.Handle("POST /v1/teams/{teamID}/members", protected(s.handleAddTeamMember))
	mux.Handle("DELETE /v1/teams/{teamID}/members/{userID}", protected(s.handleRemoveTeamMember))
	mux.Handle("PUT /v1/teams/{teamID}/captain", protected(s.handleTransferCaptaincy))
	mux.Handle("POST /v1/teams/{teamID}/join-code", protected(s.handleRotateJoinCode))

	// Matchmaking. The feed is public (above); one call is readable either
	// way, because a shared link should open for somebody not signed in --
	// the handler simply omits the viewer-specific fields.
	mux.Handle("GET /v1/calls/mine", protected(s.handleMyCalls))
	mux.Handle("POST /v1/calls", protected(s.handleCreateCall))
	mux.HandleFunc("GET /v1/calls/{callID}", s.handleGetCall)
	mux.Handle("PATCH /v1/calls/{callID}", protected(s.handleUpdateCall))
	mux.Handle("DELETE /v1/calls/{callID}", protected(s.handleDeleteCall))
	mux.Handle("POST /v1/calls/{callID}/cancel", protected(s.handleCancelCall))
	mux.Handle("POST /v1/calls/{callID}/responses", protected(s.handleRespondToCall))
	mux.Handle("DELETE /v1/calls/{callID}/responses", protected(s.handleWithdrawFromCall))
	mux.Handle("POST /v1/calls/{callID}/responses/{userID}/accept", protected(s.handleAcceptResponse))

	// Tournaments. Listing and one bracket are public (above); entering one
	// and running one are not.
	mux.Handle("POST /v1/tournaments", protected(s.handleCreateTournament))
	mux.Handle("POST /v1/tournaments/{tournamentID}/teams", protected(s.handleRegisterTeam))
	mux.Handle("DELETE /v1/tournaments/{tournamentID}/teams/{teamID}", protected(s.handleWithdrawTeam))
	mux.Handle("PUT /v1/tournaments/{tournamentID}/teams/{teamID}/paid", protected(s.handleSetEntryPaid))
	mux.Handle("PUT /v1/tournaments/{tournamentID}/status", protected(s.handleSetTournamentStatus))

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
