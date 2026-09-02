// Package service holds the use cases: the steps that turn a request into a
// change in the world, in the order and with the checks the business requires.
//
// Services depend on the domain for rules and on narrow repository interfaces
// for storage. They never import pgx or net/http, so a use case can be read
// as the sequence of decisions it actually makes.
package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/postgres"
)

// BookingStore is the storage this service needs. It is declared here, next
// to its consumer, rather than in the package that implements it: the use
// case decides what it requires, and a test can satisfy it without a database.
type BookingStore interface {
	LoadCourtContext(ctx context.Context, courtID uuid.UUID) (postgres.CourtContext, error)
	BookedRanges(ctx context.Context, courtID uuid.UUID, window domain.Slot) ([]domain.Slot, error)
	CreateHold(ctx context.Context, b domain.Booking) (domain.Booking, error)
	ByID(ctx context.Context, id uuid.UUID) (domain.Booking, error)
	Cancel(ctx context.Context, bookingID, userID uuid.UUID) error
	ListForUser(ctx context.Context, userID uuid.UUID, limit int) ([]domain.BookingDetail, error)
	ReleaseStaleHolds(ctx context.Context) (int, error)
	// MarkPlayed promotes confirmed bookings whose hour has passed. The
	// janitor's, like ReleaseStaleHolds, and reachable from no request.
	MarkPlayed(ctx context.Context) (int, error)
}

// Clock supplies the current time. Injecting it keeps every time-dependent
// rule testable without waiting for the wall clock to cooperate.
type Clock interface{ Now() time.Time }

// SystemClock reads the real time, in UTC.
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }

type BookingService struct {
	store      BookingStore
	clock      Clock
	loc        *time.Location
	holdWindow time.Duration
}

func NewBookingService(store BookingStore, clock Clock, loc *time.Location, holdWindow time.Duration) *BookingService {
	if clock == nil {
		clock = SystemClock{}
	}
	if loc == nil {
		loc = time.UTC
	}
	return &BookingService{store: store, clock: clock, loc: loc, holdWindow: holdWindow}
}

// CreateBookingInput is what a caller supplies. Note what is absent: a price.
// The client's displayed figure is advisory, and this service resolves the
// real one from pricing rules before writing anything.
type CreateBookingInput struct {
	UserID  uuid.UUID
	CourtID uuid.UUID
	Start   time.Time
	End     time.Time
	TeamID  *uuid.UUID
	Note    string
}

// Create takes a hold on a court.
//
// The order matters. Cheap, local checks run before anything touches the
// database, so a malformed request never reaches the locking path; the price
// is resolved server-side; and the write itself is left to the repository,
// where the advisory lock and the EXCLUDE constraint settle any race.
func (s *BookingService) Create(ctx context.Context, in CreateBookingInput) (domain.Booking, error) {
	now := s.clock.Now()

	slot, err := domain.NewSlot(in.Start, in.End)
	if err != nil {
		return domain.Booking{}, err
	}
	if slot.IsPast(now) {
		return domain.Booking{}, domain.Invalid("slot", "That slot has already started.")
	}
	if in.UserID == uuid.Nil {
		return domain.Booking{}, domain.Unauthenticated("Sign in to book a court.")
	}

	court, err := s.store.LoadCourtContext(ctx, in.CourtID)
	if err != nil {
		return domain.Booking{}, err
	}
	if !court.IsBookable() {
		return domain.Booking{}, domain.NotFound("That court is no longer available.")
	}
	if !slot.WithinOperatingHours(court.OpensAt, court.ClosesAt, s.loc) {
		return domain.Booking{}, domain.Invalid("slot",
			"%s is open from %s to %s.", court.ArenaName, court.OpensAt, court.ClosesAt)
	}

	// The authoritative price. Never the client's.
	price := domain.ResolvePrice(court.Court.BasePriceNPR, court.PricingRules, slot, s.loc)

	hold, err := domain.NewHold(in.CourtID, in.UserID, slot, price, in.TeamID, in.Note, now, s.holdWindow)
	if err != nil {
		return domain.Booking{}, err
	}

	return s.store.CreateHold(ctx, hold)
}

// Availability projects the bookable hours of one court on one date.
//
// Two queries, both indexed, then a pure projection in Go. The previous
// implementation ran a correlated price lookup per hour inside SQL; this
// reads the rules once and applies them in memory, which is both faster and
// testable without a database.
func (s *BookingService) Availability(ctx context.Context, courtID uuid.UUID, date time.Time) ([]domain.GridSlot, error) {
	court, err := s.store.LoadCourtContext(ctx, courtID)
	if err != nil {
		return nil, err
	}
	if !court.IsBookable() {
		return nil, domain.NotFound("That court is no longer available.")
	}

	window := domain.GridWindow(date, s.loc)
	booked, err := s.store.BookedRanges(ctx, courtID, window)
	if err != nil {
		return nil, err
	}

	return domain.BuildGrid(domain.GridRequest{
		Date:         date,
		OpensAt:      court.OpensAt,
		ClosesAt:     court.ClosesAt,
		Location:     s.loc,
		BasePriceNPR: court.Court.BasePriceNPR,
		Rules:        court.PricingRules,
		Booked:       booked,
		Now:          s.clock.Now(),
	}), nil
}

// Cancel releases a booking the caller owns.
//
// Ownership is enforced in the repository's UPDATE rather than by loading the
// booking and checking here: a read-then-write leaves a window in which the
// booking can change between the check and the write.
func (s *BookingService) Cancel(ctx context.Context, bookingID, userID uuid.UUID) error {
	if userID == uuid.Nil {
		return domain.Unauthenticated("Sign in to manage your bookings.")
	}
	return s.store.Cancel(ctx, bookingID, userID)
}

// ListMine returns the caller's bookings, newest first.
func (s *BookingService) ListMine(ctx context.Context, userID uuid.UUID, limit int) ([]domain.BookingDetail, error) {
	if userID == uuid.Nil {
		return nil, domain.Unauthenticated("Sign in to see your bookings.")
	}
	return s.store.ListForUser(ctx, userID, limit)
}

// ReleaseStaleHolds is the janitor's entry point, driven by the background
// sweeper rather than by a request.
func (s *BookingService) ReleaseStaleHolds(ctx context.Context) (int, error) {
	return s.store.ReleaseStaleHolds(ctx)
}
