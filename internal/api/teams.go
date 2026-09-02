package api

import (
	"net/http"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/google/uuid"
)

// Squad management. Everything here is authenticated: a team is a group of
// people, and there is nothing useful to show somebody who is not one of them.

// handleMyTeams — GET /v1/teams
func (s *Server) handleMyTeams(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	teams, err := s.teams.MyTeams(r.Context(), userID)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, teamDTOsFromDomain(teams))
}

// handleCreateTeam — POST /v1/teams
func (s *Server) handleCreateTeam(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	req, err := decode[teamWriteRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	team, err := s.teams.Create(r.Context(), userID, req.team())
	if err != nil {
		writeError(w, r, err)
		return
	}

	// The creator is a member, so they get the invite code back -- it is the
	// first thing they need in order to fill the squad.
	encode(w, http.StatusCreated, teamDetailDTO{
		teamDTO:  teamDTOFromDomain(team),
		Members:  []memberDTO{},
		JoinCode: team.JoinCode,
	})
}

// handleGetTeam — GET /v1/teams/{teamID}
func (s *Server) handleGetTeam(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	teamID, err := uuid.Parse(r.PathValue("teamID"))
	if err != nil {
		writeError(w, r, domain.Invalid("team_id", "That isn't a team."))
		return
	}

	team, err := s.teams.Get(r.Context(), teamID, userID)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, teamDetailDTOFromDomain(team))
}

// handleUpdateTeam — PATCH /v1/teams/{teamID}
func (s *Server) handleUpdateTeam(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	teamID, err := uuid.Parse(r.PathValue("teamID"))
	if err != nil {
		writeError(w, r, domain.Invalid("team_id", "That isn't a team."))
		return
	}

	req, err := decode[teamWriteRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	team, err := s.teams.Update(r.Context(), teamID, userID, req.team())
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, teamDTOFromDomain(team))
}

// handleJoinTeam — POST /v1/teams/join
//
// The code identifies the team, so there is no team id in this request: a code
// cannot be redeemed against a squad it does not belong to.
func (s *Server) handleJoinTeam(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	req, err := decode[joinTeamRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	team, err := s.teams.Join(r.Context(), userID, req.Code)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, teamDTOFromDomain(team))
}

// handleAddTeamMember — POST /v1/teams/{teamID}/members
func (s *Server) handleAddTeamMember(w http.ResponseWriter, r *http.Request) {
	actorID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	teamID, err := uuid.Parse(r.PathValue("teamID"))
	if err != nil {
		writeError(w, r, domain.Invalid("team_id", "That isn't a team."))
		return
	}

	req, err := decode[memberRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	if err := s.teams.AddMember(r.Context(), teamID, actorID, req.UserID); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleRemoveTeamMember — DELETE /v1/teams/{teamID}/members/{userID}
//
// One route for two actions, because the domain treats them as one: a captain
// removing somebody and a player leaving differ only in who the target is, and
// `CanRemoveMember` decides which is allowed.
func (s *Server) handleRemoveTeamMember(w http.ResponseWriter, r *http.Request) {
	actorID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	teamID, err := uuid.Parse(r.PathValue("teamID"))
	if err != nil {
		writeError(w, r, domain.Invalid("team_id", "That isn't a team."))
		return
	}
	targetID, err := uuid.Parse(r.PathValue("userID"))
	if err != nil {
		writeError(w, r, domain.Invalid("user_id", "That isn't a player."))
		return
	}

	if err := s.teams.RemoveMember(r.Context(), teamID, actorID, targetID); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleTransferCaptaincy — PUT /v1/teams/{teamID}/captain
func (s *Server) handleTransferCaptaincy(w http.ResponseWriter, r *http.Request) {
	actorID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	teamID, err := uuid.Parse(r.PathValue("teamID"))
	if err != nil {
		writeError(w, r, domain.Invalid("team_id", "That isn't a team."))
		return
	}

	req, err := decode[memberRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	if err := s.teams.TransferCaptaincy(r.Context(), teamID, actorID, req.UserID); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleRotateJoinCode — POST /v1/teams/{teamID}/join-code
func (s *Server) handleRotateJoinCode(w http.ResponseWriter, r *http.Request) {
	actorID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	teamID, err := uuid.Parse(r.PathValue("teamID"))
	if err != nil {
		writeError(w, r, domain.Invalid("team_id", "That isn't a team."))
		return
	}

	code, err := s.teams.RotateJoinCode(r.Context(), teamID, actorID)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, joinCodeDTO{JoinCode: code})
}

// handleDisbandTeam — DELETE /v1/teams/{teamID}
//
// Bookings the team made survive: `on delete set null` detaches them rather
// than removing them. A squad breaking up does not un-book the hours they
// reserved, and somebody still has to turn up.
func (s *Server) handleDisbandTeam(w http.ResponseWriter, r *http.Request) {
	actorID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	teamID, err := uuid.Parse(r.PathValue("teamID"))
	if err != nil {
		writeError(w, r, domain.Invalid("team_id", "That isn't a team."))
		return
	}

	if err := s.teams.Disband(r.Context(), teamID, actorID); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
