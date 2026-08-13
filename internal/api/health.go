package api

import (
	"context"
	"net/http"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

type Pinger interface {
	Ping(context.Context) error
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	err := s.pinger.Ping(r.Context())

	if err != nil {
		w.Header().Set("Retry-After", "2")
		writeError(w, r, domain.Unavailable("database unavailable"))
		return
	}
	w.WriteHeader(http.StatusOK)
}
