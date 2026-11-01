package api

import (
	"net/http"
	"net/url"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/platform/payment"
	"github.com/google/uuid"
)

// handleArenaPaymentProviders — GET /v1/arenas/{arenaID}/payment-providers
//
// The online providers one venue is currently taking. Public: which methods a
// shop accepts is not a secret, and the arena page needs it to decide which
// pay buttons to show. Cash is never in this list — it is settled with the
// venue, not started here.
//
// An empty list is the honest answer for a venue that has configured nothing,
// or for a deployment with online payments switched off entirely.
func (s *Server) handleArenaPaymentProviders(w http.ResponseWriter, r *http.Request) {
	arenaID, err := uuid.Parse(r.PathValue("arenaID"))
	if err != nil {
		writeError(w, r, domain.Invalid("arena_id", "That isn't an arena."))
		return
	}

	providers := []domain.PaymentProvider{}
	if s.paymentAccounts != nil {
		providers, err = s.paymentAccounts.ProvidersForArena(r.Context(), arenaID)
		if err != nil {
			writeError(w, r, err)
			return
		}
	}
	encode(w, http.StatusOK, providers)
}

// handleCreateCheckout — POST /v1/bookings/{bookingID}/checkout (authenticated)
//
// Starts a payment and says where to send the player. The amount is never in
// the request: it comes from the booking, which got it from the pricing rules
// when the hold was taken.
func (s *Server) handleCreateCheckout(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	bookingID, err := uuid.Parse(r.PathValue("bookingID"))
	if err != nil {
		writeError(w, r, domain.Invalid("booking_id", "That isn't a booking."))
		return
	}

	req, err := decode[checkoutRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	checkout, created, err := s.payments.Checkout(r.Context(), bookingID, userID, req.Provider)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusCreated, checkoutDTOFromDomain(checkout, created))
}

// handlePaymentCallback — GET /v1/payments/{provider}/callback
//
// Where a gateway sends the player back.
//
// Everything in this request is untrusted. It arrives in the player's browser
// over a URL they can edit, so the query string is used only to work out
// which transaction to ask the gateway about -- the answer comes from a
// server-to-server call the player is not part of. See the package comment on
// internal/platform/payment.
//
// It redirects rather than returning JSON because a person is looking at it:
// the gateway sent a browser here, not a client library.
func (s *Server) handlePaymentCallback(w http.ResponseWriter, r *http.Request) {
	// Every exit redirects. The gateway sent a person here, and a JSON error
	// would leave them on a bare API page with no way back into the site.
	provider := domain.PaymentProvider(r.PathValue("provider"))
	if !provider.Valid() {
		s.redirectAfterPayment(w, r, uuid.Nil, "failed")
		return
	}

	// No transaction to look up. eSewa sends a cancelled or failed payment
	// back with no `data` at all, so this is the ordinary "didn't pay" path,
	// not just a tampered link. The payment stays initiated and the player can
	// try again while the hold lasts.
	ref, err := payment.RefFromCallback(provider, r.URL.Query())
	if err != nil {
		s.redirectAfterPayment(w, r, uuid.Nil, "failed")
		return
	}

	settled, err := s.payments.Settle(r.Context(), provider, ref)
	if err != nil {
		// The player is mid-redirect; a JSON envelope would be a wall of text
		// in their address bar. Send them back to their bookings with the
		// outcome in the URL, and let the interface say what went wrong.
		//
		// The message is not passed through: an unverified payment's failure
		// is between us and the gateway, and the booking page reads the
		// authoritative status from /v1/bookings anyway.
		s.redirectAfterPayment(w, r, settled.BookingID, "failed")
		return
	}

	outcome := "failed"
	switch {
	case settled.Status == domain.PaymentVerified && settled.RefundDueAt != nil:
		// The money moved, but the booking couldn't take it: the player
		// needs to hear that the venue owes them a refund.
		outcome = "refund"
	case settled.Status == domain.PaymentVerified:
		outcome = "paid"
	}
	s.redirectAfterPayment(w, r, settled.BookingID, outcome)
}

// redirectAfterPayment sends the player back into the interface.
//
// The destination is built from configuration, never from anything in the
// request: a redirect target taken from a query parameter is an open redirect,
// and this endpoint's URL is one a gateway will happily send anybody to.
func (s *Server) redirectAfterPayment(w http.ResponseWriter, r *http.Request, bookingID uuid.UUID, outcome string) {
	q := url.Values{}
	q.Set("payment", outcome)
	if bookingID != uuid.Nil {
		q.Set("booking", bookingID.String())
	}

	http.Redirect(w, r, s.appURL+"/bookings?"+q.Encode(), http.StatusSeeOther)
}

// handlePaymentStatus — GET /v1/bookings/{bookingID}/payment (authenticated)
//
// For a client polling after a redirect. Ownership is checked in the service.
func (s *Server) handlePaymentStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	bookingID, err := uuid.Parse(r.PathValue("bookingID"))
	if err != nil {
		writeError(w, r, domain.Invalid("booking_id", "That isn't a booking."))
		return
	}

	p, err := s.payments.Status(r.Context(), bookingID, userID)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, paymentDTOFromDomain(p))
}
