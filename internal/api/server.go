package api

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/postgres"
	"github.com/RajwatSingh/khel-arena/internal/service"
	"github.com/google/uuid"
)

type AuthAPI interface {
	Login(ctx context.Context, email, password string, sc postgres.SessionContext) (service.Session, error)
	Register(ctx context.Context, reg domain.Registration, sc postgres.SessionContext) (service.Session, error)
	Logout(ctx context.Context, refreshToken string) error
	Authenticate(accessToken string) (uuid.UUID, domain.AccountType, error)
	Refresh(ctx context.Context, refreshToken string, sc postgres.SessionContext) (service.Session, error)
	BeginPasswordReset(ctx context.Context, email string) (string, domain.User, error)
	CompletePasswordReset(ctx context.Context, resetToken, newPassword string) error
}

type BookingAPI interface {
	Create(ctx context.Context, in service.CreateBookingInput) (domain.Booking, error)
}

type Server struct {
	authAPI        AuthAPI
	bookingAPI     BookingAPI
	pinger         Pinger
	allowedOrigins []string
	secureCookies  bool
	logResetTokens bool
}

func chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
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

	session, err := s.authAPI.Login(r.Context(), req.Email, req.Password, sc)

	if err != nil {
		writeError(w, r, err)
		return
	}

	s.writeSession(w, session, http.StatusOK)
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	resp, err := decode[domain.Registration](w, r)

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

	session, err := s.authAPI.Register(r.Context(), resp, sc)

	if err != nil {
		writeError(w, r, err)
		return
	}

	s.writeSession(w, session, http.StatusCreated)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	token := readRefreshToken(r)
	err := s.authAPI.Logout(r.Context(), token)

	if err != nil {
		writeError(w, r, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name: "refresh_token",
		Value: "",
		Path: "/",
		MaxAge: -1,
		Expires: time.Unix(0, 0),
		HttpOnly: true,
	})

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) refresh(w http.ResponseWriter, r *http.Request) {
	token := readRefreshToken(r)

	host, _, err := net.SplitHostPort(r.RemoteAddr)

	if err != nil {
		writeError(w, r, err)
		return
	}

	sc := postgres.SessionContext{
		UserAgent: r.UserAgent(),
		IP: host,
	}

	session, err := s.authAPI.Refresh(r.Context(), token, sc)

	if err != nil {
		writeError(w, r, err)
		return
	}

	s.writeSession(w, session, http.StatusOK)
}

func (s *Server) beginForgotPassword(w http.ResponseWriter, r *http.Request) {
	email, err := decode[emailDTO](w, r)

	if err != nil {
		writeError(w, r, err)
		return
	}

	resetToken, user, err := s.authAPI.BeginPasswordReset(r.Context(), email.Email)
	
	if err != nil {
		writeError(w, r, err)
		return
	}

	if s.logResetTokens {
		slog.Info("Logging the reset", "user", user.Username, "email", email.Email, "token", resetToken)
	}

	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) completePasswordReset(w http.ResponseWriter, r *http.Request) {
	resp, err := decode[newPassword](w, r)

	if err != nil {
		writeError(w, r, err)
		return
	}

	err = s.authAPI.CompletePasswordReset(r.Context(), resp.Token, resp.NewPassword)

	if err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) createBooking(w http.ResponseWriter, r *http.Request) {
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

	booking, err := s.bookingAPI.Create(r.Context(), bookingInput)

	if err != nil {
		writeError(w, r, err)
		return
	}

	resp := bookingDTOFromDomain(booking)
	encode(w, http.StatusCreated, resp)
}

func NewServer(authAPI AuthAPI, bookingAPI BookingAPI, pinger Pinger, allowedOrigins []string, secureCookies bool, logResetTokens bool) *Server {
	return &Server{
		authAPI:        authAPI,
		bookingAPI:     bookingAPI,
		pinger:         pinger,
		allowedOrigins: allowedOrigins,
		secureCookies:  secureCookies,
		logResetTokens: logResetTokens,
	}
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /v1/auth/login", s.login)
	mux.HandleFunc("POST /v1/auth/register", s.register)
	mux.HandleFunc("POST /v1/auth/refresh", s.refresh)
	mux.HandleFunc("POST /v1/auth/logout", s.logout)
	mux.HandleFunc("POST /v1/auth/password/forgot", s.beginForgotPassword)
	mux.HandleFunc("POST /v1/auth/password/reset", s.completePasswordReset)

	protected := http.NewServeMux()
	protected.HandleFunc("POST /v1/bookings", s.createBooking)
	mux.Handle("/v1/bookings", chain(protected, withAuth(s.authAPI)))
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)

	return chain(mux,
		withRecovery,
		withRequestID,
		withLogging,
		withCORS(s.allowedOrigins), 
		withTimeout(10*time.Second),
	)
}
