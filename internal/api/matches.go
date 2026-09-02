package api

import (
	"net/http"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/google/uuid"
)

// Results and the table.
//
// One captain files a score, the other agrees, and only an agreed result
// counts toward the standings. That is the whole design: `verified` is what
// the standings view reads, and it means "both sides said the same thing".

// handleStandings — GET /v1/standings
//
// Public. The table is the point of recording results at all.
func (s *Server) handleStandings(w http.ResponseWriter, r *http.Request) {
	limit, err := bookingLimit(r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	standings, err := s.matches.Standings(r.Context(), limit)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, standingDTOsFromDomain(standings))
}

// handleTeamMatches — GET /v1/teams/{teamID}/matches
func (s *Server) handleTeamMatches(w http.ResponseWriter, r *http.Request) {
	teamID, err := uuid.Parse(r.PathValue("teamID"))
	if err != nil {
		writeError(w, r, domain.Invalid("team_id", "That isn't a team."))
		return
	}

	limit, err := bookingLimit(r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	matches, err := s.matches.ListForTeam(r.Context(), teamID, limit)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, matchDTOsFromDomain(matches))
}

// handleReportMatch — POST /v1/matches (authenticated)
//
// Files a result, unagreed. The reporter is the caller and is recorded, which
// is what lets the other captain — and only the other captain — confirm it.
func (s *Server) handleReportMatch(w http.ResponseWriter, r *http.Request) {
	actorID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	req, err := decode[matchWriteRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	match, err := s.matches.Report(r.Context(), actorID, req.match())
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusCreated, matchDTOFromDomain(match))
}

// handleConfirmMatch — POST /v1/matches/{matchID}/confirm (authenticated)
func (s *Server) handleConfirmMatch(w http.ResponseWriter, r *http.Request) {
	actorID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	matchID, err := uuid.Parse(r.PathValue("matchID"))
	if err != nil {
		writeError(w, r, domain.Invalid("match_id", "That isn't a result."))
		return
	}

	match, err := s.matches.Confirm(r.Context(), matchID, actorID)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, matchDTOFromDomain(match))
}

// handleDisputeMatch — POST /v1/matches/{matchID}/dispute (authenticated)
//
// Countering with a different score. The result goes back to the other
// captain, who can agree or counter again — two people who keep disagreeing
// keep passing it back, which is the honest model of an argument about a
// scoreline.
func (s *Server) handleDisputeMatch(w http.ResponseWriter, r *http.Request) {
	actorID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	matchID, err := uuid.Parse(r.PathValue("matchID"))
	if err != nil {
		writeError(w, r, domain.Invalid("match_id", "That isn't a result."))
		return
	}

	req, err := decode[disputeRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	match, err := s.matches.Dispute(r.Context(), matchID, actorID, req.HomeScore, req.AwayScore)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, matchDTOFromDomain(match))
}

// handleWithdrawMatch — DELETE /v1/matches/{matchID} (authenticated)
//
// Either captain, and only while the result is unagreed. Once both have said
// the same thing it is a record of what happened, not a draft.
func (s *Server) handleWithdrawMatch(w http.ResponseWriter, r *http.Request) {
	actorID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	matchID, err := uuid.Parse(r.PathValue("matchID"))
	if err != nil {
		writeError(w, r, domain.Invalid("match_id", "That isn't a result."))
		return
	}

	if err := s.matches.Withdraw(r.Context(), matchID, actorID); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
