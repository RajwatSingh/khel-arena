package api

import (
	"net/http"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/google/uuid"
)

// handleListTournaments — GET /v1/tournaments
//
// Public. Cancelled and completed brackets are left out by the service: a
// listing is something you act on.
func (s *Server) handleListTournaments(w http.ResponseWriter, r *http.Request) {
	limit, err := bookingLimit(r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	tournaments, err := s.tournaments.List(r.Context(), limit)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, tournamentDTOsFromDomain(tournaments))
}

// handleGetTournament — GET /v1/tournaments/{slug}
func (s *Server) handleGetTournament(w http.ResponseWriter, r *http.Request) {
	t, err := s.tournaments.Get(r.Context(), r.PathValue("slug"))
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, tournamentDetailDTOFromDomain(t))
}

// handleCreateTournament — POST /v1/tournaments (authenticated)
func (s *Server) handleCreateTournament(w http.ResponseWriter, r *http.Request) {
	organizerID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	req, err := decode[tournamentWriteRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	t, err := req.tournament()
	if err != nil {
		writeError(w, r, err)
		return
	}

	created, err := s.tournaments.Create(r.Context(), organizerID, t)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusCreated, tournamentDTOFromDomain(created))
}

// handleRegisterTeam — POST /v1/tournaments/{tournamentID}/teams (authenticated)
func (s *Server) handleRegisterTeam(w http.ResponseWriter, r *http.Request) {
	actorID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	tournamentID, err := uuid.Parse(r.PathValue("tournamentID"))
	if err != nil {
		writeError(w, r, domain.Invalid("tournament_id", "That isn't a tournament."))
		return
	}

	req, err := decode[registerTeamRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	if err := s.tournaments.Register(r.Context(), tournamentID, req.TeamID, actorID); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleWithdrawTeam — DELETE /v1/tournaments/{tournamentID}/teams/{teamID}
func (s *Server) handleWithdrawTeam(w http.ResponseWriter, r *http.Request) {
	actorID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	tournamentID, err := uuid.Parse(r.PathValue("tournamentID"))
	if err != nil {
		writeError(w, r, domain.Invalid("tournament_id", "That isn't a tournament."))
		return
	}
	teamID, err := uuid.Parse(r.PathValue("teamID"))
	if err != nil {
		writeError(w, r, domain.Invalid("team_id", "That isn't a team."))
		return
	}

	if err := s.tournaments.Withdraw(r.Context(), tournamentID, teamID, actorID); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleSetEntryPaid — PUT /v1/tournaments/{tournamentID}/teams/{teamID}/paid
//
// The organiser recording an entry fee. The same shape as an arena confirming
// cash: whoever took the money is the one who says so.
func (s *Server) handleSetEntryPaid(w http.ResponseWriter, r *http.Request) {
	actorID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	tournamentID, err := uuid.Parse(r.PathValue("tournamentID"))
	if err != nil {
		writeError(w, r, domain.Invalid("tournament_id", "That isn't a tournament."))
		return
	}
	teamID, err := uuid.Parse(r.PathValue("teamID"))
	if err != nil {
		writeError(w, r, domain.Invalid("team_id", "That isn't a team."))
		return
	}

	req, err := decode[paidRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	if err := s.tournaments.SetEntryPaid(r.Context(), tournamentID, teamID, actorID, req.Paid); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleSetTournamentStatus — PUT /v1/tournaments/{tournamentID}/status
//
// Open and full are maintained by the database as teams come and go. This is
// for the transitions only a person can make: starting it, finishing it,
// calling it off.
func (s *Server) handleSetTournamentStatus(w http.ResponseWriter, r *http.Request) {
	actorID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	tournamentID, err := uuid.Parse(r.PathValue("tournamentID"))
	if err != nil {
		writeError(w, r, domain.Invalid("tournament_id", "That isn't a tournament."))
		return
	}

	req, err := decode[tournamentStatusRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	if err := s.tournaments.SetStatus(r.Context(), tournamentID, actorID, req.Status); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
