package api

import (
	"net/http"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/postgres"
	"github.com/google/uuid"
)

// handleListReviews — GET /v1/arenas/{arenaID}/reviews
func (s *Server) handleListReviews(w http.ResponseWriter, r *http.Request) {
	arenaID, err := uuid.Parse(r.PathValue("arenaID"))
	if err != nil {
		writeError(w, r, domain.Invalid("arena_id", "That isn't an arena."))
		return
	}

	limit, err := bookingLimit(r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	reviews, err := s.reviews.ListReviews(r.Context(), arenaID, limit)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, reviewDTOsFromDomain(reviews))
}

// handleMyReview — GET /v1/arenas/{arenaID}/reviews/mine (authenticated)
//
// What the caller has said, and whether they may say anything: a review has
// to be earned by having played there, and a page that offered the form to
// somebody who cannot use it would be a worse experience than one that
// explains.
func (s *Server) handleMyReview(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	arenaID, err := uuid.Parse(r.PathValue("arenaID"))
	if err != nil {
		writeError(w, r, domain.Invalid("arena_id", "That isn't an arena."))
		return
	}

	review, canReview, err := s.reviews.MyReview(r.Context(), arenaID, userID)
	if err != nil {
		writeError(w, r, err)
		return
	}

	out := myReviewDTO{CanReview: canReview}
	if review.ID != uuid.Nil {
		dto := reviewDTOFromDomain(review)
		out.Review = &dto
	}
	encode(w, http.StatusOK, out)
}

// handleReviewArena — PUT /v1/arenas/{arenaID}/reviews/mine (authenticated)
//
// PUT because there is at most one review per player per arena: posting again
// replaces what you said rather than adding a second opinion. The address says
// so — "mine" is a place, and writing to it twice is the same act twice.
func (s *Server) handleReviewArena(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	arenaID, err := uuid.Parse(r.PathValue("arenaID"))
	if err != nil {
		writeError(w, r, domain.Invalid("arena_id", "That isn't an arena."))
		return
	}

	req, err := decode[reviewWriteRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	review, err := s.reviews.Review(r.Context(), arenaID, userID, req.Rating, req.Comment)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, reviewDTOFromDomain(review))
}

// handleDeleteReview — DELETE /v1/arenas/{arenaID}/reviews/mine (authenticated)
func (s *Server) handleDeleteReview(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	arenaID, err := uuid.Parse(r.PathValue("arenaID"))
	if err != nil {
		writeError(w, r, domain.Invalid("arena_id", "That isn't an arena."))
		return
	}

	if err := s.reviews.DeleteReview(r.Context(), arenaID, userID); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleListPhotos — GET /v1/arenas/{arenaID}/photos
func (s *Server) handleListPhotos(w http.ResponseWriter, r *http.Request) {
	arenaID, err := uuid.Parse(r.PathValue("arenaID"))
	if err != nil {
		writeError(w, r, domain.Invalid("arena_id", "That isn't an arena."))
		return
	}

	photos, err := s.reviews.ListPhotos(r.Context(), arenaID)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, photoDTOsFromDomain(photos))
}

// handleAddPhoto — POST /v1/owner/arenas/{arenaID}/photos (authenticated)
func (s *Server) handleAddPhoto(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	arenaID, err := uuid.Parse(r.PathValue("arenaID"))
	if err != nil {
		writeError(w, r, domain.Invalid("arena_id", "That isn't an arena."))
		return
	}

	req, err := decode[photoWriteRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	photo, err := s.reviews.AddPhoto(r.Context(), ownerID, postgres.Photo{
		ArenaID:   arenaID,
		URL:       req.URL,
		Caption:   req.Caption,
		SortOrder: req.SortOrder,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusCreated, photoDTOFromDomain(photo))
}

// handleDeletePhoto — DELETE /v1/owner/photos/{photoID} (authenticated)
func (s *Server) handleDeletePhoto(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	photoID, err := uuid.Parse(r.PathValue("photoID"))
	if err != nil {
		writeError(w, r, domain.Invalid("photo_id", "That isn't a photo."))
		return
	}

	if err := s.reviews.DeletePhoto(r.Context(), photoID, ownerID); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleListHighlights — GET /v1/players/{userID}/highlights
//
// Public: a highlight reel is the part of a player card meant to be seen.
func (s *Server) handleListHighlights(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(r.PathValue("userID"))
	if err != nil {
		writeError(w, r, domain.Invalid("user_id", "That isn't a player."))
		return
	}

	highlights, err := s.reviews.ListHighlights(r.Context(), userID)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, highlightDTOsFromDomain(highlights))
}

// handleAddHighlight — POST /v1/me/highlights (authenticated)
//
// Addressed as "me" rather than by user id: this only ever writes to the
// caller's own card, and a path that named a user would invite the question
// of whose card it is.
func (s *Server) handleAddHighlight(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	req, err := decode[highlightWriteRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	highlight, err := s.reviews.AddHighlight(r.Context(), userID, req.Title, req.URL)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusCreated, highlightDTOFromDomain(highlight))
}

// handleDeleteHighlight — DELETE /v1/me/highlights/{highlightID}
func (s *Server) handleDeleteHighlight(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	highlightID, err := uuid.Parse(r.PathValue("highlightID"))
	if err != nil {
		writeError(w, r, domain.Invalid("highlight_id", "That isn't a clip."))
		return
	}

	if err := s.reviews.DeleteHighlight(r.Context(), highlightID, userID); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
