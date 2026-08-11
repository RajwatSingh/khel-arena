package api

import (
	"net/http"
)

func chain(h http.Handler, mws ...Middleware) http.Handler {
      for i := len(mws) - 1; i >= 0; i-- {
              h = mws[i](h)
      }
      return h
}

func NewRouter(/* services, config */) http.Handler {
      mux := http.NewServeMux()

      // register public routes (login, register) directly on mux
      mux.HandleFunc("POST /v1/auth/login", loginHandler)

      // protected routes get the auth middleware wrapped around them individually,
      // or grouped under a sub-mux — e.g.:
      protected := http.NewServeMux()
      protected.HandleFunc("POST /v1/bookings", createBookingHandler)
      mux.Handle("/v1/bookings", chain(protected, withAuth(authService)))

      // the global stack, outermost to innermost, wraps everything:
      return chain(mux,
              withRecovery,
              withRequestID,
              withLogging,
              withCORS(allowedOrigins), // only if you decide you need it
              withTimeout(10*time.Second),
      )
}
