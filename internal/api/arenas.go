package api

import (
	"net/http"
	"time"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

// The browsing endpoints. All public: a player picking a court has not signed
// in yet, and requiring it to see what a venue charges would be the wrong way
// round.

// handleListArenas — GET /v1/arenas
func (s *Server) handleListArenas(w http.ResponseWriter, r *http.Request) {
	arenas, err := s.arenas.List(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, arenaListingDTOsFromDomain(arenas))
}

// handleGetArena — GET /v1/arenas/{slug}
//
// Addressed by slug rather than id because the slug is what appears in a link
// somebody shares, and a URL a person can read is worth more here than one
// fewer index lookup.
func (s *Server) handleGetArena(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		writeError(w, r, domain.Invalid("slug", "Which arena?"))
		return
	}

	arena, err := s.arenas.BySlug(r.Context(), slug)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, arenaDetailDTOFromDomain(arena))
}

// handleListAreas — GET /v1/areas
//
// The neighbourhoods with something bookable in them, for the filter. A bare
// array rather than an object: it is a list of strings and will not grow
// fields, so an envelope would only add a key to step through.
func (s *Server) handleListAreas(w http.ResponseWriter, r *http.Request) {
	areas, err := s.arenas.ListAreas(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}

	if areas == nil {
		areas = []string{}
	}
	encode(w, http.StatusOK, areas)
}

// handleLedger — GET /v1/ledger?date=&sport=&area=
//
// Every court in the city, one day, one request. This is the home page's
// whole payload, and the reason it is one endpoint rather than the client
// looping over /availability per court: that loop is the N+1 this design
// exists to avoid, and moving it into the browser would not make it cheaper.
//
// `sport` and `area` both default to everything, which is what the filter's
// default position means. Only `date` is required, because there is no
// sensible default for "which day" that the server should be picking on the
// client's behalf -- the client knows what the person is looking at.
func (s *Server) handleLedger(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("date")
	if raw == "" {
		writeError(w, r, domain.Invalid("date", "Say which day, as ?date=YYYY-MM-DD."))
		return
	}
	date, err := time.Parse(dateLayout, raw)
	if err != nil {
		writeError(w, r, domain.Invalid("date", "Dates look like 2026-08-14."))
		return
	}

	// "all" is what the interface's filter calls its default position; it
	// means the same as saying nothing, and both arrive here as "".
	sport := filterValue(r, "sport")
	area := filterValue(r, "area")

	ledger, err := s.arenas.CityLedger(r.Context(), date, domain.Sport(sport), area)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, ledgerDTOFromDomain(ledger))
}

// filterValue reads an optional filter, treating the interface's "all" and an
// absent parameter as the same thing.
func filterValue(r *http.Request, key string) string {
	v := r.URL.Query().Get(key)
	if v == "all" {
		return ""
	}
	return v
}
