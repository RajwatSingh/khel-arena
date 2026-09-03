package api

import (
	"net/http"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/google/uuid"
)

// The owner-facing surface, all behind /v1/owner and all authenticated.
//
// Authorization is per-arena, not per-role, past the first step: owning an
// account of type arena_owner lets you register a venue, and owning *that
// venue* is what lets you edit it. Every write below reaches a repository
// whose SQL carries the owner predicate, so a handler that forgot its check
// would still not be able to touch another owner's arena.

// handleMyArenas — GET /v1/owner/arenas
//
// Includes inactive venues, unlike the public index: this is the management
// view, and a venue you have closed is exactly the one you need to see to
// reopen it.
func (s *Server) handleMyArenas(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	arenas, err := s.owner.MyArenas(r.Context(), ownerID)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, arenaListingDTOsFromDomain(arenas))
}

// handleCreateArena — POST /v1/owner/arenas
func (s *Server) handleCreateArena(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	req, err := decode[arenaWriteRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	arena, err := s.owner.CreateArena(r.Context(), ownerID, req.arena())
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusCreated, arenaDTOFromDomain(arena))
}

// handleUpdateArena — PUT /v1/owner/arenas/{arenaID}
//
// PUT rather than PATCH because it replaces: the statement behind it writes
// every column it owns, so a field the body omits is a field that gets
// blanked. A PATCH that silently erases what you did not mention is a trap
// for any client that sends less than the whole resource.
func (s *Server) handleUpdateArena(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	arenaID, err := uuid.Parse(r.PathValue("arenaID"))
	if err != nil {
		writeError(w, r, domain.Invalid("arena_id", "That isn't an arena."))
		return
	}

	req, err := decode[arenaWriteRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	arena, err := s.owner.UpdateArena(r.Context(), arenaID, ownerID, req.arena())
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, arenaDTOFromDomain(arena))
}

// handleSetArenaActive — PUT /v1/owner/arenas/{arenaID}/active
//
// Closing a venue hides it and stops new bookings; it deliberately leaves
// bookings already taken alone. People have plans, and going quiet on the site
// is not the same as cancelling everyone's Saturday.
func (s *Server) handleSetArenaActive(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	arenaID, err := uuid.Parse(r.PathValue("arenaID"))
	if err != nil {
		writeError(w, r, domain.Invalid("arena_id", "That isn't an arena."))
		return
	}

	req, err := decode[activeRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	if err := s.owner.SetArenaActive(r.Context(), arenaID, ownerID, req.Active); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleCreateCourt — POST /v1/owner/arenas/{arenaID}/courts
func (s *Server) handleCreateCourt(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	arenaID, err := uuid.Parse(r.PathValue("arenaID"))
	if err != nil {
		writeError(w, r, domain.Invalid("arena_id", "That isn't an arena."))
		return
	}

	req, err := decode[courtWriteRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	// The arena comes from the path, not the body: a body that could name one
	// would let a court be added to a venue the URL says nothing about.
	court := req.court()
	court.ArenaID = arenaID

	created, err := s.owner.CreateCourt(r.Context(), ownerID, court, req.Format)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusCreated, courtDTOFromDomain(created))
}

// handleUpdateCourt — PUT /v1/owner/courts/{courtID}. Replaces, like the arena above.
func (s *Server) handleUpdateCourt(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	courtID, err := uuid.Parse(r.PathValue("courtID"))
	if err != nil {
		writeError(w, r, domain.Invalid("court_id", "That isn't a court."))
		return
	}

	req, err := decode[courtWriteRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	court, err := s.owner.UpdateCourt(r.Context(), courtID, ownerID, req.court(), req.Format)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, courtDTOFromDomain(court))
}

// handleSetCourtActive — PUT /v1/owner/courts/{courtID}/active
//
// Retired rather than deleted. A court with bookings against it cannot be
// removed -- the foreign key says so, and rightly: who played where is not
// something an edit screen should erase.
func (s *Server) handleSetCourtActive(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	courtID, err := uuid.Parse(r.PathValue("courtID"))
	if err != nil {
		writeError(w, r, domain.Invalid("court_id", "That isn't a court."))
		return
	}

	req, err := decode[activeRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	if err := s.owner.SetCourtActive(r.Context(), courtID, ownerID, req.Active); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleCreatePricingRule — POST /v1/owner/courts/{courtID}/pricing
func (s *Server) handleCreatePricingRule(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	courtID, err := uuid.Parse(r.PathValue("courtID"))
	if err != nil {
		writeError(w, r, domain.Invalid("court_id", "That isn't a court."))
		return
	}

	req, err := decode[pricingRuleWriteRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	rule, err := req.rule()
	if err != nil {
		writeError(w, r, err)
		return
	}
	rule.CourtID = courtID

	created, err := s.owner.CreatePricingRule(r.Context(), ownerID, rule)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusCreated, pricingRuleDTOFromDomain(created))
}

// handleCopyPricingRules — POST /v1/owner/courts/{courtID}/pricing/copy
//
// Copies another of the caller's courts' rate windows onto this one. Appends
// rather than replaces: overlapping windows are already resolved by priority,
// and quietly deleting what an owner set up would be the worse surprise.
func (s *Server) handleCopyPricingRules(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	toCourtID, err := uuid.Parse(r.PathValue("courtID"))
	if err != nil {
		writeError(w, r, domain.Invalid("court_id", "That isn't a court."))
		return
	}

	req, err := decode[copyPricingRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	copied, err := s.owner.CopyPricingRules(r.Context(), req.FromCourtID, toCourtID, ownerID)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, copyPricingDTO{Copied: copied})
}

// handleDeletePricingRule — DELETE /v1/owner/pricing/{ruleID}
//
// Deleted rather than deactivated, unlike a court: a rule holds no history.
// Removing it changes what future hours cost and nothing about hours already
// booked, whose price was resolved and written when the booking was made.
func (s *Server) handleDeletePricingRule(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	ruleID, err := uuid.Parse(r.PathValue("ruleID"))
	if err != nil {
		writeError(w, r, domain.Invalid("rule_id", "That isn't a pricing rule."))
		return
	}

	if err := s.owner.DeletePricingRule(r.Context(), ruleID, ownerID); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleListPaymentAccounts — GET /v1/owner/arenas/{arenaID}/payment-accounts
//
// The state of each gateway account on one of the caller's venues. Secrets are
// never in the reply — only a four-character hint, so an owner can tell which
// key is in place.
func (s *Server) handleListPaymentAccounts(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	arenaID, err := uuid.Parse(r.PathValue("arenaID"))
	if err != nil {
		writeError(w, r, domain.Invalid("arena_id", "That isn't an arena."))
		return
	}

	accounts, err := s.owner.PaymentAccounts(r.Context(), arenaID, ownerID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if accounts == nil {
		accounts = []domain.ArenaPaymentAccountInfo{}
	}
	encode(w, http.StatusOK, accounts)
}

// handleSetPaymentAccount — PUT /v1/owner/arenas/{arenaID}/payment-accounts/{provider}
//
// Stores or replaces the venue's credentials for one gateway. The money for a
// booking settles into the account named here, which is the whole point of
// this being per venue.
func (s *Server) handleSetPaymentAccount(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	arenaID, err := uuid.Parse(r.PathValue("arenaID"))
	if err != nil {
		writeError(w, r, domain.Invalid("arena_id", "That isn't an arena."))
		return
	}

	provider := domain.PaymentProvider(r.PathValue("provider"))
	if !provider.CanBePerArena() {
		writeError(w, r, domain.Invalid("provider", "%q isn't a per-venue payment method.", provider))
		return
	}

	req, err := decode[paymentAccountWriteRequest](w, r)
	if err != nil {
		writeError(w, r, err)
		return
	}

	if err := s.owner.SetPaymentAccount(r.Context(), arenaID, ownerID, req.account(provider)); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleDeletePaymentAccount — DELETE /v1/owner/arenas/{arenaID}/payment-accounts/{provider}
func (s *Server) handleDeletePaymentAccount(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	arenaID, err := uuid.Parse(r.PathValue("arenaID"))
	if err != nil {
		writeError(w, r, domain.Invalid("arena_id", "That isn't an arena."))
		return
	}

	provider := domain.PaymentProvider(r.PathValue("provider"))

	if err := s.owner.RemovePaymentAccount(r.Context(), arenaID, ownerID, provider); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleOwnerPayments — GET /v1/owner/arenas/{arenaID}/payments
func (s *Server) handleOwnerPayments(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

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

	payments, err := s.owner.Payments(r.Context(), arenaID, ownerID, limit)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, ownerPaymentDTOsFromDomain(payments))
}

// handleMarkCashReceived — POST /v1/owner/payments/{paymentID}/received
//
// The only way a cash booking becomes confirmed, and deliberately the arena's
// call rather than the player's: the venue is the only party that knows
// whether the notes changed hands. The repository refuses to apply it to a
// gateway payment, so this cannot become a way to confirm an unpaid online
// booking by hand.
func (s *Server) handleMarkCashReceived(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	paymentID, err := uuid.Parse(r.PathValue("paymentID"))
	if err != nil {
		writeError(w, r, domain.Invalid("payment_id", "That isn't a payment."))
		return
	}

	settled, err := s.owner.MarkCashReceived(r.Context(), paymentID, ownerID)
	if err != nil {
		writeError(w, r, err)
		return
	}

	encode(w, http.StatusOK, paymentDTOFromDomain(settled))
}
