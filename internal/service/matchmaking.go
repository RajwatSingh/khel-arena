package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/postgres"
)

// MatchmakingStore is the storage this service needs.
type MatchmakingStore interface {
	Create(ctx context.Context, c domain.Call) (domain.Call, error)
	ByID(ctx context.Context, id uuid.UUID) (domain.Call, error)
	Feed(ctx context.Context, f postgres.CallFilter, now time.Time) ([]domain.Call, error)
	ListForUser(ctx context.Context, userID uuid.UUID) ([]domain.Call, error)
	Update(ctx context.Context, id uuid.UUID, c domain.Call) (domain.Call, error)
	SetStatus(ctx context.Context, id uuid.UUID, status domain.MatchmakingStatus) error
	Delete(ctx context.Context, id uuid.UUID) error

	Respond(ctx context.Context, callID, userID uuid.UUID, message string) error
	Responses(ctx context.Context, callID uuid.UUID) ([]domain.Response, error)
	Accept(ctx context.Context, callID, userID uuid.UUID) error
	Withdraw(ctx context.Context, callID, userID uuid.UUID) error
	HasResponded(ctx context.Context, callID, userID uuid.UUID) (bool, error)
}

// MatchmakingBookings is the slice of booking storage this service reads, for
// the case where a call opens a court the author has already paid for.
type MatchmakingBookings interface {
	ByID(ctx context.Context, id uuid.UUID) (domain.Booking, error)
}

// MatchmakingService is "we need two more".
//
// Every rule it enforces is a domain predicate: `CanBeJoinedBy`,
// `CanAcceptResponse`, `CanBeManagedBy`. This layer supplies the clock and the
// storage and does not get its own opinion about who may join what.
type MatchmakingService struct {
	calls    MatchmakingStore
	bookings MatchmakingBookings
	clock    Clock
}

func NewMatchmakingService(calls MatchmakingStore, bookings MatchmakingBookings, clock Clock) *MatchmakingService {
	if clock == nil {
		clock = SystemClock{}
	}
	return &MatchmakingService{calls: calls, bookings: bookings, clock: clock}
}

// CallWithResponses is a call and who has asked to join it.
type CallWithResponses struct {
	domain.Call
	Responses []domain.Response
	// YouResponded saves a client comparing ids to decide whether to show a
	// "join" button or a "waiting" one.
	YouResponded bool
}

// Create opens a call.
//
// A call attached to a booking has to be attached to the author's own booking:
// otherwise anyone could advertise somebody else's court, and the players who
// turned up would find it occupied by people who had never heard of them.
func (s *MatchmakingService) Create(ctx context.Context, authorID uuid.UUID, c domain.Call) (domain.Call, error) {
	if authorID == uuid.Nil {
		return domain.Call{}, domain.Unauthenticated("Sign in to post a game.")
	}
	c.AuthorID = authorID

	if c.BookingID != nil {
		booking, err := s.bookings.ByID(ctx, *c.BookingID)
		if err != nil {
			return domain.Call{}, err
		}
		if !booking.IsHeldBy(authorID) {
			return domain.Call{}, domain.NotFound("No booking with that reference.")
		}

		// The court and the kickoff come from the booking, not the request:
		// a call advertising a different time than the hour actually reserved
		// is how people end up at an empty pitch.
		c.StartsAt = booking.Slot.Start
	}

	if err := c.Validate(); err != nil {
		return domain.Call{}, err
	}
	return s.calls.Create(ctx, c)
}

// Feed is the public board of open games.
func (s *MatchmakingService) Feed(ctx context.Context, f postgres.CallFilter) ([]domain.Call, error) {
	if f.Skill != "" && !f.Skill.Valid() {
		return nil, domain.Invalid("skill", "We don't sort games by that.")
	}
	return s.calls.Feed(ctx, f, s.clock.Now())
}

func (s *MatchmakingService) MyCalls(ctx context.Context, userID uuid.UUID) ([]domain.Call, error) {
	if userID == uuid.Nil {
		return nil, domain.Unauthenticated("Sign in to see your games.")
	}
	return s.calls.ListForUser(ctx, userID)
}

// Get returns a call, with its responses only for the author.
//
// Who has asked to join is the author's to see. Showing the list to everyone
// would publish a roster of people's plans to anyone who opened the page.
func (s *MatchmakingService) Get(ctx context.Context, callID, viewerID uuid.UUID) (CallWithResponses, error) {
	call, err := s.calls.ByID(ctx, callID)
	if err != nil {
		return CallWithResponses{}, err
	}

	out := CallWithResponses{Call: call}

	if call.AuthoredBy(viewerID) {
		responses, err := s.calls.Responses(ctx, callID)
		if err != nil {
			return CallWithResponses{}, err
		}
		out.Responses = responses
		return out, nil
	}

	if viewerID != uuid.Nil {
		responded, err := s.calls.HasResponded(ctx, callID, viewerID)
		if err != nil {
			return CallWithResponses{}, err
		}
		out.YouResponded = responded
	}
	return out, nil
}

func (s *MatchmakingService) Update(ctx context.Context, callID, actorID uuid.UUID, in domain.Call) (domain.Call, error) {
	call, err := s.load(ctx, callID, actorID)
	if err != nil {
		return domain.Call{}, err
	}
	if err := call.CanBeManagedBy(actorID); err != nil {
		return domain.Call{}, err
	}

	// A call attached to a booking keeps that booking's kickoff: the hour is
	// reserved, and the post does not get to disagree with it.
	in.AuthorID = call.AuthorID
	in.BookingID = call.BookingID
	if call.IsAttachedToBooking() {
		in.StartsAt = call.StartsAt
	}
	if err := in.Validate(); err != nil {
		return domain.Call{}, err
	}
	return s.calls.Update(ctx, callID, in)
}

// Cancel closes a call without deleting it, so the people who joined can still
// see what happened to the game they signed up for.
func (s *MatchmakingService) Cancel(ctx context.Context, callID, actorID uuid.UUID) error {
	call, err := s.load(ctx, callID, actorID)
	if err != nil {
		return err
	}
	if err := call.CanBeManagedBy(actorID); err != nil {
		return err
	}
	return s.calls.SetStatus(ctx, callID, domain.MatchmakingCancelled)
}

// Respond asks to join. It is a request, not a place: only the author can
// turn it into one.
func (s *MatchmakingService) Respond(ctx context.Context, callID, userID uuid.UUID, message string) error {
	call, err := s.load(ctx, callID, userID)
	if err != nil {
		return err
	}
	if err := call.CanBeJoinedBy(userID, s.clock.Now()); err != nil {
		return err
	}
	return s.calls.Respond(ctx, callID, userID, message)
}

// Accept gives a responder a place in the game.
func (s *MatchmakingService) Accept(ctx context.Context, callID, actorID, userID uuid.UUID) error {
	call, err := s.load(ctx, callID, actorID)
	if err != nil {
		return err
	}
	if err := call.CanAcceptResponse(actorID, s.clock.Now()); err != nil {
		return err
	}
	return s.calls.Accept(ctx, callID, userID)
}

// Withdraw takes back a request, freeing the spot if it had been given.
func (s *MatchmakingService) Withdraw(ctx context.Context, callID, userID uuid.UUID) error {
	if userID == uuid.Nil {
		return domain.Unauthenticated("Sign in to manage your games.")
	}
	// No domain check: withdrawing is always yours to do. A player who has
	// changed their mind about a game they signed up for should never be told
	// they may not leave.
	return s.calls.Withdraw(ctx, callID, userID)
}

func (s *MatchmakingService) Delete(ctx context.Context, callID, actorID uuid.UUID) error {
	call, err := s.load(ctx, callID, actorID)
	if err != nil {
		return err
	}
	if err := call.CanBeManagedBy(actorID); err != nil {
		return err
	}
	return s.calls.Delete(ctx, callID)
}

func (s *MatchmakingService) load(ctx context.Context, callID, actorID uuid.UUID) (domain.Call, error) {
	if actorID == uuid.Nil {
		return domain.Call{}, domain.Unauthenticated("Sign in to join a game.")
	}
	return s.calls.ByID(ctx, callID)
}
