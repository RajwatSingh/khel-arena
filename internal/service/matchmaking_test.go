package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/postgres"
)

type stubCalls struct {
	call      domain.Call
	responses []domain.Response
	responded bool
	writes    int
	accepted  uuid.UUID
	message   string
}

func (s *stubCalls) Create(_ context.Context, c domain.Call) (domain.Call, error) {
	s.writes++
	c.ID = uuid.New()
	s.call = c
	return c, nil
}
func (s *stubCalls) ByID(context.Context, uuid.UUID) (domain.Call, error) { return s.call, nil }
func (s *stubCalls) Feed(context.Context, postgres.CallFilter, time.Time) ([]domain.Call, error) {
	return []domain.Call{s.call}, nil
}
func (s *stubCalls) ListForUser(context.Context, uuid.UUID) ([]domain.Call, error) {
	return []domain.Call{s.call}, nil
}
func (s *stubCalls) Update(_ context.Context, _ uuid.UUID, c domain.Call) (domain.Call, error) {
	s.writes++
	s.call = c
	return c, nil
}
func (s *stubCalls) SetStatus(context.Context, uuid.UUID, domain.MatchmakingStatus) error {
	s.writes++
	return nil
}
func (s *stubCalls) Delete(context.Context, uuid.UUID) error { s.writes++; return nil }
func (s *stubCalls) Respond(_ context.Context, _, _ uuid.UUID, message string) error {
	s.writes++
	s.message = message
	return nil
}
func (s *stubCalls) Responses(context.Context, uuid.UUID) ([]domain.Response, error) {
	return s.responses, nil
}
func (s *stubCalls) Accept(_ context.Context, _, userID uuid.UUID) error {
	s.writes++
	s.accepted = userID
	return nil
}
func (s *stubCalls) Withdraw(context.Context, uuid.UUID, uuid.UUID) error { s.writes++; return nil }
func (s *stubCalls) HasResponded(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return s.responded, nil
}

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

var now = time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)

func openCall(author uuid.UUID, needed, filled int) domain.Call {
	return domain.Call{
		ID:            uuid.New(),
		AuthorID:      author,
		Title:         "Need two more at Dhuku",
		NeededPlayers: needed,
		FilledPlayers: filled,
		Skill:         domain.SkillCasual,
		StartsAt:      now.Add(4 * time.Hour),
		Status:        domain.MatchmakingOpen,
	}
}

// A call attached to somebody else's booking would advertise a court its
// author has no claim on, and the players who turned up would find it taken.
func TestCreateCallRefusesSomebodyElsesBooking(t *testing.T) {
	bookingID := uuid.New()
	calls := &stubCalls{}
	svc := NewMatchmakingService(calls,
		stubBookings{booking: domain.Booking{ID: bookingID, UserID: captainID}}, fixedClock{now})

	_, err := svc.Create(context.Background(), outsider, domain.Call{
		BookingID: &bookingID, Title: "Come to my court", NeededPlayers: 2,
		StartsAt: now.Add(time.Hour),
	})

	if domain.CodeOf(err) != domain.CodeNotFound {
		t.Errorf("code = %q, want not_found", domain.CodeOf(err))
	}
	if calls.writes != 0 {
		t.Error("a call was posted against a booking the author does not hold")
	}
}

// A call advertising a different time than the hour actually reserved is how
// people end up at an empty pitch.
func TestCreateCallTakesKickoffFromTheBooking(t *testing.T) {
	bookingID := uuid.New()
	slotStart := now.Add(6 * time.Hour)
	calls := &stubCalls{}
	svc := NewMatchmakingService(calls, stubBookings{booking: domain.Booking{
		ID: bookingID, UserID: captainID,
		Slot: domain.Slot{Start: slotStart, End: slotStart.Add(time.Hour)},
	}}, fixedClock{now})

	_, err := svc.Create(context.Background(), captainID, domain.Call{
		BookingID: &bookingID, Title: "Need two more", NeededPlayers: 2,
		StartsAt: now.Add(99 * time.Hour), // a time the author made up
	})
	if err != nil {
		t.Fatalf("creating: %v", err)
	}

	if !calls.call.StartsAt.Equal(slotStart) {
		t.Errorf("starts_at = %v, want the booking's %v", calls.call.StartsAt, slotStart)
	}
}

// Who has asked to join is a list of people's plans, not a public roster.
func TestGetCallShowsResponsesOnlyToTheAuthor(t *testing.T) {
	calls := &stubCalls{
		call:      openCall(captainID, 4, 1),
		responses: []domain.Response{{UserID: playerID}},
		responded: true,
	}
	svc := NewMatchmakingService(calls, stubBookings{}, fixedClock{now})

	t.Run("the author sees them", func(t *testing.T) {
		got, err := svc.Get(context.Background(), calls.call.ID, captainID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if len(got.Responses) != 1 {
			t.Errorf("responses = %d, want 1", len(got.Responses))
		}
	})

	t.Run("a stranger does not", func(t *testing.T) {
		got, err := svc.Get(context.Background(), calls.call.ID, outsider)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if len(got.Responses) != 0 {
			t.Errorf("responses = %d, want none", len(got.Responses))
		}
		// They do learn whether *they* have asked, which is the one
		// viewer-specific thing a join button needs.
		if !got.YouResponded {
			t.Error("you_responded was not reported")
		}
	})
}

func TestRespondToCall(t *testing.T) {
	t.Run("joins an open call", func(t *testing.T) {
		calls := &stubCalls{call: openCall(captainID, 4, 1)}
		svc := NewMatchmakingService(calls, stubBookings{}, fixedClock{now})

		if err := svc.Respond(context.Background(), calls.call.ID, outsider, "I can play"); err != nil {
			t.Fatalf("responding: %v", err)
		}
		if calls.message != "I can play" {
			t.Errorf("message = %q", calls.message)
		}
	})

	t.Run("a full call refuses", func(t *testing.T) {
		calls := &stubCalls{call: openCall(captainID, 4, 4)}
		svc := NewMatchmakingService(calls, stubBookings{}, fixedClock{now})

		err := svc.Respond(context.Background(), calls.call.ID, outsider, "")

		if domain.CodeOf(err) != domain.CodeConflict {
			t.Errorf("code = %q, want conflict", domain.CodeOf(err))
		}
		if calls.writes != 0 {
			t.Error("a response was written to a full call")
		}
	})

	// Kickoff has passed: the game is over whether or not anything has got
	// round to writing that down.
	t.Run("a call that has started refuses", func(t *testing.T) {
		past := openCall(captainID, 4, 1)
		past.StartsAt = now.Add(-time.Hour)
		calls := &stubCalls{call: past}
		svc := NewMatchmakingService(calls, stubBookings{}, fixedClock{now})

		if err := svc.Respond(context.Background(), past.ID, outsider, ""); domain.CodeOf(err) != domain.CodeConflict {
			t.Errorf("code = %q, want conflict", domain.CodeOf(err))
		}
	})

	t.Run("the author cannot join their own call", func(t *testing.T) {
		calls := &stubCalls{call: openCall(captainID, 4, 1)}
		svc := NewMatchmakingService(calls, stubBookings{}, fixedClock{now})

		err := svc.Respond(context.Background(), calls.call.ID, captainID, "")

		if err == nil {
			t.Error("the author was allowed to join their own game")
		}
	})
}

func TestOnlyTheAuthorAccepts(t *testing.T) {
	calls := &stubCalls{call: openCall(captainID, 4, 1)}
	svc := NewMatchmakingService(calls, stubBookings{}, fixedClock{now})

	err := svc.Accept(context.Background(), calls.call.ID, outsider, playerID)

	if domain.CodeOf(err) != domain.CodeForbidden {
		t.Errorf("code = %q, want forbidden", domain.CodeOf(err))
	}
	if calls.writes != 0 {
		t.Error("a stranger accepted somebody into a game")
	}
}

func TestAuthorAcceptsAResponder(t *testing.T) {
	calls := &stubCalls{call: openCall(captainID, 4, 1)}
	svc := NewMatchmakingService(calls, stubBookings{}, fixedClock{now})

	if err := svc.Accept(context.Background(), calls.call.ID, captainID, playerID); err != nil {
		t.Fatalf("accepting: %v", err)
	}
	if calls.accepted != playerID {
		t.Errorf("accepted = %v, want %v", calls.accepted, playerID)
	}
}

// Somebody who has changed their mind about a game should never be told they
// may not leave -- not by the author, and not by the call being full.
func TestWithdrawIsAlwaysAllowed(t *testing.T) {
	full := openCall(captainID, 4, 4)
	full.Status = domain.MatchmakingFilled
	calls := &stubCalls{call: full}
	svc := NewMatchmakingService(calls, stubBookings{}, fixedClock{now})

	if err := svc.Withdraw(context.Background(), full.ID, playerID); err != nil {
		t.Errorf("withdrawing from a full call: %v", err)
	}
}

// Not-found rather than forbidden, which is `CanBeManagedBy`'s own choice: a
// distinct refusal would confirm that somebody else's call exists.
func TestOnlyTheAuthorManagesACall(t *testing.T) {
	calls := &stubCalls{call: openCall(captainID, 4, 1)}
	svc := NewMatchmakingService(calls, stubBookings{}, fixedClock{now})
	ctx := context.Background()

	for name, err := range map[string]error{
		"cancel": svc.Cancel(ctx, calls.call.ID, outsider),
		"delete": svc.Delete(ctx, calls.call.ID, outsider),
	} {
		t.Run(name, func(t *testing.T) {
			if domain.CodeOf(err) != domain.CodeNotFound {
				t.Errorf("code = %q, want not_found", domain.CodeOf(err))
			}
		})
	}
	if calls.writes != 0 {
		t.Error("a stranger changed somebody else's call")
	}
}

func TestFeedRejectsAnUnknownSkill(t *testing.T) {
	svc := NewMatchmakingService(&stubCalls{}, stubBookings{}, fixedClock{now})

	_, err := svc.Feed(context.Background(), postgres.CallFilter{Skill: "olympian"})

	if domain.CodeOf(err) != domain.CodeInvalid {
		t.Errorf("code = %q, want invalid", domain.CodeOf(err))
	}
}
