package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/service"
	"github.com/google/uuid"
)

const (
	// defaultBookingLimit matches what the frontend asks for when it says
	// nothing (web/src/lib/api/client.js).
	defaultBookingLimit = 20
	// maxBookingLimit caps what one request may ask the database for. A
	// client naming a huge limit gets the cap, not an error: it is a request
	// for "as many as possible", and refusing it helps nobody.
	maxBookingLimit = 100
)

// handleAvailability — GET /v1/courts/{courtID}/availability?date=YYYY-MM-DD
//
// Both inputs are parsed before anything is called. A malformed court id or
// date is the client's mistake, and must read as a 400 with a field name, not
// as a 500 from a failed lookup further down.
func (s *Server) handleAvailability(w http.ResponseWriter, r *http.Request) {
	courtID, err := uuid.Parse(r.PathValue("courtID"))
	if err != nil {
		writeError(w, r, domain.Invalid("court_id", "That isn't a court."))
		return
	}

	raw := r.URL.Query().Get("date")
	if raw == "" {
		writeError(w, r, domain.Invalid("date", "Say which day, as ?date=YYYY-MM-DD."))
		return
	}
	// Parsed as a plain calendar date, with no zone attached. BookingService
	// reads only the year, month and day off it and resolves them against the
	// arena's own timezone, which is the only zone opening hours mean
	// anything in.
	date, err := time.Parse(dateLayout, raw)
	if err != nil {
		writeError(w, r, domain.Invalid("date", "Dates look like 2026-08-14."))
		return
	}

	slots, err := s.bookings.Availability(r.Context(), courtID, date)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, availabilityDTOFromDomain(courtID, date, slots))
}

// handleCreateBooking — POST /v1/bookings (authenticated)
//
// Takes a hold, 201.
//
// Two fields the request does not get to supply: the user id, which comes
// from the access token so that no client can book as somebody else, and the
// price, which BookingService resolves from pricing rules. A price on the
// wire is advisory — what the client was shown — and is never what gets
// written.
func (s *Server) handleCreateBooking(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	req, err := decode[createBookingRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	booking, err := s.bookings.Create(r.Context(), service.CreateBookingInput{
		UserID:  userID,
		CourtID: req.CourtID,
		Start:   req.StartsAt,
		End:     req.EndsAt,
		TeamID:  req.TeamID,
		Note:    req.Note,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusCreated, bookingDTOFromDomain(booking))
}

// handleListBookings — GET /v1/bookings?limit= (authenticated)
//
// Newest first, which is the order the service returns them in — no re-sort
// here, or there would be two answers to what "newest" means.
func (s *Server) handleListBookings(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	limit, err := bookingLimit(r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	bookings, err := s.bookings.ListMine(r.Context(), userID, limit)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, bookingDetailDTOsFromDomain(bookings))
}

// handleCancelBooking — DELETE /v1/bookings/{bookingID} (authenticated)
//
// 204. The booking is deliberately not loaded first to check who owns it:
// ownership is a condition of the repository's UPDATE, so there is no window
// between checking and writing in which the booking could change. A
// read-then-write here would put that window back.
func (s *Server) handleCancelBooking(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	bookingID, err := uuid.Parse(r.PathValue("bookingID"))
	if err != nil {
		writeError(w, r, domain.Invalid("booking_id", "That isn't a booking."))
		return
	}

	if err := s.bookings.Cancel(r.Context(), bookingID, userID); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// bookingLimit reads ?limit=, defaulting when absent and clamping when
// unreasonable. A limit that is not a number is rejected rather than ignored:
// silently treating ?limit=twenty as 20 hides a client bug.
func bookingLimit(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return defaultBookingLimit, nil
	}

	limit, err := strconv.Atoi(raw)
	if err != nil {
		return 0, domain.Invalid("limit", "The limit has to be a whole number.")
	}
	if limit < 1 {
		return 0, domain.Invalid("limit", "Ask for at least one booking.")
	}
	if limit > maxBookingLimit {
		limit = maxBookingLimit
	}
	return limit, nil
}
