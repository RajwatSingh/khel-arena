package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/postgres"
)

// OwnerStore is the owner-facing venue storage.
//
// Every write here is owner-scoped in its own SQL. The checks this service
// makes on top are for the sake of a clear error, not for safety: a
// read-then-write leaves a window, and these are the writes where that window
// means editing somebody else's arena.
type OwnerStore interface {
	CreateArena(ctx context.Context, a domain.Arena) (domain.Arena, error)
	UpdateArenaOwnerScoped(ctx context.Context, arenaID, ownerID uuid.UUID, a domain.Arena) (domain.Arena, error)
	SetArenaActiveOwnerScoped(ctx context.Context, arenaID, ownerID uuid.UUID, active bool) error
	ListArenasForOwner(ctx context.Context, ownerID uuid.UUID) ([]postgres.ArenaListing, error)
	OwnsArena(ctx context.Context, arenaID, ownerID uuid.UUID) (bool, error)

	CreateCourtOwnerScoped(ctx context.Context, ownerID uuid.UUID, c domain.Court, format string) (postgres.CourtWithRules, error)
	UpdateCourtOwnerScoped(ctx context.Context, courtID, ownerID uuid.UUID, c domain.Court, format string) (postgres.CourtWithRules, error)
	SetCourtActiveOwnerScoped(ctx context.Context, courtID, ownerID uuid.UUID, active bool) error

	CreatePricingRuleOwnerScoped(ctx context.Context, ownerID uuid.UUID, rule domain.PricingRule) (domain.PricingRule, error)
	DeletePricingRuleOwnerScoped(ctx context.Context, ruleID, ownerID uuid.UUID) error
}

// OwnerPayments is the reconciliation half.
type OwnerPayments interface {
	MarkCashReceivedOwnerScoped(ctx context.Context, paymentID, ownerID uuid.UUID, now time.Time) (domain.Payment, error)
	ListForArenaOwner(ctx context.Context, arenaID, ownerID uuid.UUID, limit int) ([]postgres.OwnerPayment, error)
}

// OwnerService is everything an arena owner can do.
//
// This is the first place `domain.AccountArenaOwner` does real work. Until
// now it was an enum value nothing consulted; here it gates who may register a
// venue at all, and per-arena ownership gates everything after that.
type OwnerService struct {
	arenas   OwnerStore
	payments OwnerPayments
	users    ProfileReader
	clock    Clock
}

// ProfileReader is the account lookup this service needs, to find out what
// kind of account is asking.
type ProfileReader interface {
	ByID(ctx context.Context, id uuid.UUID) (domain.User, error)
}

func NewOwnerService(arenas OwnerStore, payments OwnerPayments, users ProfileReader, clock Clock) *OwnerService {
	if clock == nil {
		clock = SystemClock{}
	}
	return &OwnerService{arenas: arenas, payments: payments, users: users, clock: clock}
}

// requireOwnerAccount checks the caller is registered as an arena owner.
//
// Only enforced on creating a venue. Once someone owns an arena, everything
// else is gated by owning *that* arena, which is the stronger check -- and
// re-reading the account on every court edit would be a database round trip to
// re-learn something the arena row already implies.
func (s *OwnerService) requireOwnerAccount(ctx context.Context, userID uuid.UUID) error {
	if userID == uuid.Nil {
		return domain.Unauthenticated("Sign in to manage an arena.")
	}

	user, err := s.users.ByID(ctx, userID)
	if err != nil {
		return err
	}
	if !user.IsArenaOwner() {
		return domain.Forbidden("Arena owner accounts can list a venue. Yours is a player account.")
	}
	return nil
}

func (s *OwnerService) MyArenas(ctx context.Context, ownerID uuid.UUID) ([]postgres.ArenaListing, error) {
	if ownerID == uuid.Nil {
		return nil, domain.Unauthenticated("Sign in to manage an arena.")
	}
	return s.arenas.ListArenasForOwner(ctx, ownerID)
}

// CreateArena registers a venue.
func (s *OwnerService) CreateArena(ctx context.Context, ownerID uuid.UUID, a domain.Arena) (domain.Arena, error) {
	if err := s.requireOwnerAccount(ctx, ownerID); err != nil {
		return domain.Arena{}, err
	}

	// The owner is the caller, never a field in the request: otherwise anyone
	// could list a venue in somebody else's name.
	a.OwnerID = ownerID
	a.IsActive = true

	if err := a.Validate(); err != nil {
		return domain.Arena{}, err
	}
	return s.arenas.CreateArena(ctx, a)
}

func (s *OwnerService) UpdateArena(ctx context.Context, arenaID, ownerID uuid.UUID, a domain.Arena) (domain.Arena, error) {
	if ownerID == uuid.Nil {
		return domain.Arena{}, domain.Unauthenticated("Sign in to manage an arena.")
	}

	// Validate needs a slug to check, and the slug is not updatable -- so it
	// is filled from the name the same way creation would, purely to satisfy
	// the rule. The repository does not write it.
	a.OwnerID = ownerID
	if a.Slug == "" {
		a.Slug = domain.Slugify(a.Name)
	}
	if err := a.Validate(); err != nil {
		return domain.Arena{}, err
	}
	return s.arenas.UpdateArenaOwnerScoped(ctx, arenaID, ownerID, a)
}

func (s *OwnerService) SetArenaActive(ctx context.Context, arenaID, ownerID uuid.UUID, active bool) error {
	if ownerID == uuid.Nil {
		return domain.Unauthenticated("Sign in to manage an arena.")
	}
	return s.arenas.SetArenaActiveOwnerScoped(ctx, arenaID, ownerID, active)
}

func (s *OwnerService) CreateCourt(ctx context.Context, ownerID uuid.UUID, c domain.Court, format string) (postgres.CourtWithRules, error) {
	if ownerID == uuid.Nil {
		return postgres.CourtWithRules{}, domain.Unauthenticated("Sign in to manage an arena.")
	}
	if err := c.Validate(); err != nil {
		return postgres.CourtWithRules{}, err
	}
	return s.arenas.CreateCourtOwnerScoped(ctx, ownerID, c, format)
}

func (s *OwnerService) UpdateCourt(ctx context.Context, courtID, ownerID uuid.UUID, c domain.Court, format string) (postgres.CourtWithRules, error) {
	if ownerID == uuid.Nil {
		return postgres.CourtWithRules{}, domain.Unauthenticated("Sign in to manage an arena.")
	}
	if err := c.Validate(); err != nil {
		return postgres.CourtWithRules{}, err
	}
	return s.arenas.UpdateCourtOwnerScoped(ctx, courtID, ownerID, c, format)
}

func (s *OwnerService) SetCourtActive(ctx context.Context, courtID, ownerID uuid.UUID, active bool) error {
	if ownerID == uuid.Nil {
		return domain.Unauthenticated("Sign in to manage an arena.")
	}
	return s.arenas.SetCourtActiveOwnerScoped(ctx, courtID, ownerID, active)
}

func (s *OwnerService) CreatePricingRule(ctx context.Context, ownerID uuid.UUID, rule domain.PricingRule) (domain.PricingRule, error) {
	if ownerID == uuid.Nil {
		return domain.PricingRule{}, domain.Unauthenticated("Sign in to manage an arena.")
	}
	if err := rule.Validate(); err != nil {
		return domain.PricingRule{}, err
	}
	return s.arenas.CreatePricingRuleOwnerScoped(ctx, ownerID, rule)
}

func (s *OwnerService) DeletePricingRule(ctx context.Context, ruleID, ownerID uuid.UUID) error {
	if ownerID == uuid.Nil {
		return domain.Unauthenticated("Sign in to manage an arena.")
	}
	return s.arenas.DeletePricingRuleOwnerScoped(ctx, ruleID, ownerID)
}

// ---------------------------------------------------------------------------
// Reconciliation
// ---------------------------------------------------------------------------

// Payments lists what has been taken at one of the caller's venues.
func (s *OwnerService) Payments(ctx context.Context, arenaID, ownerID uuid.UUID, limit int) ([]postgres.OwnerPayment, error) {
	if ownerID == uuid.Nil {
		return nil, domain.Unauthenticated("Sign in to manage an arena.")
	}
	return s.payments.ListForArenaOwner(ctx, arenaID, ownerID, limit)
}

// MarkCashReceived confirms that a player settled at the venue.
//
// This is the only way a cash booking becomes confirmed, and it is deliberately
// the arena's call rather than the player's: the venue is the only party that
// knows whether the notes changed hands. The repository refuses to apply it to
// a gateway payment, so this cannot become a way to confirm an unpaid online
// booking by hand.
func (s *OwnerService) MarkCashReceived(ctx context.Context, paymentID, ownerID uuid.UUID) (domain.Payment, error) {
	if ownerID == uuid.Nil {
		return domain.Payment{}, domain.Unauthenticated("Sign in to manage an arena.")
	}
	return s.payments.MarkCashReceivedOwnerScoped(ctx, paymentID, ownerID, s.clock.Now())
}
