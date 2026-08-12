package api
import (
	"context"
	"net/http"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/postgres"
	"github.com/RajwatSingh/khel-arena/internal/service"
	"github.com/google/uuid"
)

type AuthAPI interface {
	Login(ctx context.Context, email, password string, sc postgres.SessionContext) (service.Session, error)
	Authenticate(accessToken string) (uuid.UUID, domain.AccountType, error)
}

type BookingAPI interface {
	Create(ctx context.Context, in service.CreateBookingInput) (domain.Booking, error)
}

type Server struct {
	authAPI AuthAPI
	bookingAPI BookingAPI
	pinger Pinger
	allowedOrigins []string
}

func chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

func (a *Server) login(w http.ResponseWriter, r *http.Request) {
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

	session, err := a.authAPI.Login(r.Context(), req.Email, req.Password, sc)

	if err != nil {
		writeError(w, r, err)
		return
	}

	resp := sessionDTOFromDomain(session)
	encode(w, http.StatusOK, resp)
}

func (a *Server) createBooking(w http.ResponseWriter, r *http.Request) {
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

	booking, err := a.bookingAPI.Create(r.Context(), bookingInput)

	if err != nil {
		writeError(w, r, err)
		return
	}

	resp := bookingDTOFromDomain(booking)
	encode(w, http.StatusCreated, resp)
}

func NewServer(authAPI AuthAPI, bookingAPI BookingAPI, pinger Pinger, allowedOrigins []string) *Server {
	return &Server{
		authAPI: authAPI,
		bookingAPI: bookingAPI,
		pinger: pinger,
		allowedOrigins: allowedOrigins,
	}
}

func (s *Server)routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /v1/auth/login", s.login)

	protected := http.NewServeMux()
	protected.HandleFunc("POST /v1/bookings", s.createBooking)
	mux.Handle("/v1/bookings", chain(protected, withAuth(s.authenticate)))

	return chain(mux,
		withRecovery,
		withRequestID,
		withLogging,
		withCORS(s.allowedOrigins), // only if you decide you need it
		withTimeout(10*time.Second),
	)
}
