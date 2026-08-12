package api

import (
	"net"
	"net/http"
	"time"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/postgres"
	"github.com/RajwatSingh/khel-arena/internal/service"
)

type api struct {
	authService    *service.AuthService
	bookingService *service.BookingService
}

func chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

func (a *api) login(w http.ResponseWriter, r *http.Request) {
	req, err := decode[loginRequest](w, r)

	if err != nil {
		writeError(w, r, err)
		return
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)

	if err != nil {
		writeError(w, r, err)
		return
	}

	sc := postgres.SessionContext{
		UserAgent: r.UserAgent(),
		IP:        host,
	}

	session, err := a.authService.Login(r.Context(), req.Email, req.Password, sc)

	if err != nil {
		writeError(w, r, err)
		return
	}

	resp := sessionDTOFromDomain(session)
	encode(w, http.StatusOK, resp)
}

func (a *api) createBooking(w http.ResponseWriter, r *http.Request) {
	req, err := decode[bookingInput](w, r)

	if err != nil {
		writeError(w, r, err)
		return
	}

	userID, ok := userIDFromContext(r.Context())

	if !ok {
		msg := "userID missing from context on protected route"
		writeError(w, r, domain.Internal(nil, msg))
		return
	}

	bookingInput := service.CreateBookingInput{
		UserID:  userID,
		TeamID:  req.TeamID,
		CourtID: req.CourtID,
		Start:   req.StartsAt,
		End:     req.EndsAt,
		Note:    req.Note,
	}

	booking, err := a.bookingService.Create(r.Context(), bookingInput)

	if err != nil {
		writeError(w, r, err)
		return
	}

	resp := bookingDTOFromDomain(booking)
	encode(w, http.StatusCreated, resp)
}

func newAPI(authService *service.AuthService, bookingService *service.BookingService) *api {
	return &api{
		authService: authService,
		bookingService: bookingService,
	}
}

func NewRouter(allowedOrigins []string, authService *service.AuthService, bookingService *service.BookingService) http.Handler {
	mux := http.NewServeMux()
	a := newAPI(authService, bookingService)

	mux.HandleFunc("POST /v1/auth/login", a.login)

	protected := http.NewServeMux()
	protected.HandleFunc("POST /v1/bookings", a.createBooking)
	mux.Handle("/v1/bookings", chain(protected, withAuth(authService)))

	return chain(mux,
		withRecovery,
		withRequestID,
		withLogging,
		withCORS(allowedOrigins), // only if you decide you need it
		withTimeout(10*time.Second),
	)
}
