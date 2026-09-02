package api

import (
	"context"
	"net/http"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

// Pinger is the one thing readiness checks. A one-method interface rather
// than a *pgxpool.Pool, so this package keeps its promise not to import the
// storage layer -- and *pgxpool.Pool satisfies it without knowing this
// interface exists.
type Pinger interface {
	Ping(context.Context) error
}

// handleHealthz — GET /healthz
//
// Liveness. It answers "is this process running", nothing more, and checks no
// dependency on purpose: an orchestrator reading this probe restarts the
// container when it fails, and restarting the API does not fix a database
// that is down. That question belongs to /readyz.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// handleReadyz — GET /readyz
//
// Readiness. It answers "can this process serve a request right now", which
// means the database has to answer. A load balancer takes an instance out of
// rotation on a 503 here and puts it back when the ping succeeds again,
// without the process ever restarting.
func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	// Nothing to check is not the same as something failing: with no Pinger
	// configured, "the process is up" is the whole truth available.
	if s.pinger == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := s.pinger.Ping(r.Context()); err != nil {
		// Retry-After tells a caller this is expected to pass later, rather
		// than inviting a retry storm at whatever interval it picks itself.
		w.Header().Set("Retry-After", "2")
		writeError(w, r, domain.Unavailable("Not ready yet.").WithCause(err))
		return
	}

	w.WriteHeader(http.StatusOK)
}
