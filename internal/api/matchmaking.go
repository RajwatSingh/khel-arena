package api

import (
	"net/http"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/postgres"
	"github.com/google/uuid"
)

// "We need two more." A call either opens a court its author has already
// booked, or advertises a pickup game nobody has booked yet.

// handleCallFeed — GET /v1/calls?skill=&area=&limit=
//
// Public: the board of open games is the thing that makes somebody sign up.
func (s *Server) handleCallFeed(w http.ResponseWriter, r *http.Request) {
	limit, err := bookingLimit(r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	calls, err := s.calls.Feed(r.Context(), postgres.CallFilter{
		Skill: domain.SkillTier(filterValue(r, "skill")),
		Area:  filterValue(r, "area"),
		Limit: limit,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, callDTOsFromDomain(calls))
}

// handleMyCalls — GET /v1/calls/mine (authenticated)
func (s *Server) handleMyCalls(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	calls, err := s.calls.MyCalls(r.Context(), userID)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, callDTOsFromDomain(calls))
}

// handleCreateCall — POST /v1/calls (authenticated)
func (s *Server) handleCreateCall(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	req, err := decode[callWriteRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	call, err := s.calls.Create(r.Context(), userID, req.call())
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusCreated, callDTOFromDomain(call))
}

// handleGetCall — GET /v1/calls/{callID}
//
// Public, but the list of who has asked to join comes back only for the
// author: those are people's plans, not a public roster.
func (s *Server) handleGetCall(w http.ResponseWriter, r *http.Request) {
	callID, err := uuid.Parse(r.PathValue("callID"))
	if err != nil {
		writeError(w, r, domain.Invalid("call_id", "That isn't a game."))
		return
	}

	// Unauthenticated is fine here; it just means no viewer-specific fields.
	viewerID, _ := userIDFromContext(r.Context())

	call, err := s.calls.Get(r.Context(), callID, viewerID)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, callDetailDTOFromDomain(call))
}

// handleUpdateCall — PATCH /v1/calls/{callID} (authenticated)
func (s *Server) handleUpdateCall(w http.ResponseWriter, r *http.Request) {
	actorID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	callID, err := uuid.Parse(r.PathValue("callID"))
	if err != nil {
		writeError(w, r, domain.Invalid("call_id", "That isn't a game."))
		return
	}

	req, err := decode[callWriteRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	call, err := s.calls.Update(r.Context(), callID, actorID, req.call())
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, callDTOFromDomain(call))
}

// handleCancelCall — POST /v1/calls/{callID}/cancel (authenticated)
//
// Closed rather than deleted, so the people who signed up can still see what
// became of the game.
func (s *Server) handleCancelCall(w http.ResponseWriter, r *http.Request) {
	actorID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	callID, err := uuid.Parse(r.PathValue("callID"))
	if err != nil {
		writeError(w, r, domain.Invalid("call_id", "That isn't a game."))
		return
	}

	if err := s.calls.Cancel(r.Context(), callID, actorID); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleDeleteCall — DELETE /v1/calls/{callID} (authenticated)
func (s *Server) handleDeleteCall(w http.ResponseWriter, r *http.Request) {
	actorID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	callID, err := uuid.Parse(r.PathValue("callID"))
	if err != nil {
		writeError(w, r, domain.Invalid("call_id", "That isn't a game."))
		return
	}

	if err := s.calls.Delete(r.Context(), callID, actorID); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleRespondToCall — POST /v1/calls/{callID}/responses (authenticated)
//
// Asking to join. Not a place in the game: only the author turns a request
// into one.
func (s *Server) handleRespondToCall(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	callID, err := uuid.Parse(r.PathValue("callID"))
	if err != nil {
		writeError(w, r, domain.Invalid("call_id", "That isn't a game."))
		return
	}

	req, err := decode[respondRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	if err := s.calls.Respond(r.Context(), callID, userID, req.Message); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleWithdrawFromCall — DELETE /v1/calls/{callID}/responses (authenticated)
//
// Always allowed. Somebody who has changed their mind about a game should
// never be told they may not leave.
func (s *Server) handleWithdrawFromCall(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	callID, err := uuid.Parse(r.PathValue("callID"))
	if err != nil {
		writeError(w, r, domain.Invalid("call_id", "That isn't a game."))
		return
	}

	if err := s.calls.Withdraw(r.Context(), callID, userID); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleAcceptResponse — POST /v1/calls/{callID}/responses/{userID}/accept
func (s *Server) handleAcceptResponse(w http.ResponseWriter, r *http.Request) {
	actorID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	callID, err := uuid.Parse(r.PathValue("callID"))
	if err != nil {
		writeError(w, r, domain.Invalid("call_id", "That isn't a game."))
		return
	}
	userID, err := uuid.Parse(r.PathValue("userID"))
	if err != nil {
		writeError(w, r, domain.Invalid("user_id", "That isn't a player."))
		return
	}

	if err := s.calls.Accept(r.Context(), callID, actorID, userID); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
