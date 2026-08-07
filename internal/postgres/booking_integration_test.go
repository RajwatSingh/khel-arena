package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

func TestCreateHoldWritesABooking(t *testing.T) {
	f := newFixture(t, 1)
	ctx := context.Background()
	slot := f.slotAt(t, 18, time.Hour)

	got, err := f.repo.CreateHold(ctx, f.hold(t, f.players[0], slot))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.ID.String() == "" {
		t.Error("the booking should come back with an id")
	}
	if got.Status != domain.BookingPending {
		t.Errorf("status = %q, want %q", got.Status, domain.BookingPending)
	}
	if !got.Slot.Start.Equal(slot.Start) || !got.Slot.End.Equal(slot.End) {
		t.Errorf("slot round-tripped as %s, want %s", got.Slot, slot)
	}
	if got.PriceNPR != 1200 {
		t.Errorf("price = %d, want 1200", got.PriceNPR)
	}
	if got.HoldExpiresAt == nil {
		t.Error("a pending hold must record its expiry")
	}
}

// The guarantee, under real contention. Twenty players go for the same court
// and the same hour at the same instant; exactly one may end up holding it.
//
// This is the test the whole design exists to pass. It is also the one a
// mocked database cannot express: what is under test is Postgres's exclusion
// constraint and advisory lock doing their job when transactions collide.
func TestConcurrentBookingsCannotDoubleBook(t *testing.T) {
	const contenders = 20

	f := newFixture(t, contenders)
	ctx := context.Background()
	slot := f.slotAt(t, 19, time.Hour)

	var (
		start   sync.WaitGroup // holds every goroutine at the gate
		done    sync.WaitGroup
		mu      sync.Mutex
		wins    int
		rejects int
		other   []error
	)
	start.Add(1)
	done.Add(contenders)

	for i := range contenders {
		go func() {
			defer done.Done()
			start.Wait() // release everyone at once

			_, err := f.repo.CreateHold(ctx, f.hold(t, f.players[i], slot))

			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				wins++
			case errors.Is(err, domain.ErrConflict):
				rejects++
			default:
				other = append(other, err)
			}
		}()
	}

	start.Done()
	done.Wait()

	for _, err := range other {
		t.Errorf("a contender failed for an unexpected reason: %v", err)
	}
	if wins != 1 {
		t.Errorf("%d bookings succeeded, want exactly 1", wins)
	}
	if rejects != contenders-1 {
		t.Errorf("%d contenders were told the slot was taken, want %d", rejects, contenders-1)
	}

	// The decisive check: whatever the application did, the database holds
	// at most one live booking for this court and hour.
	if n := f.countLiveBookings(t, slot); n != 1 {
		t.Errorf("the database holds %d live bookings for this slot, want exactly 1", n)
	}
}

// Overlap, not just equality: a 90-minute booking must exclude an hour that
// starts inside it.
func TestOverlappingSlotsAreRejected(t *testing.T) {
	f := newFixture(t, 2)
	ctx := context.Background()

	first := f.slotAt(t, 18, 2*time.Hour) // 18:00-20:00
	if _, err := f.repo.CreateHold(ctx, f.hold(t, f.players[0], first)); err != nil {
		t.Fatalf("seeding the first booking: %v", err)
	}

	overlapping := []struct {
		name string
		slot domain.Slot
	}{
		{"identical", f.slotAt(t, 18, 2*time.Hour)},
		{"starts inside", f.slotAt(t, 19, time.Hour)},
		{"ends inside", f.slotAt(t, 17, 2*time.Hour)},
		{"contains it", f.slotAt(t, 17, 4*time.Hour)},
	}

	for _, tc := range overlapping {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.repo.CreateHold(ctx, f.hold(t, f.players[1], tc.slot))
			if err == nil {
				t.Fatal("an overlapping booking was accepted")
			}
			if !errors.Is(err, domain.ErrConflict) {
				t.Errorf("code = %q, want %q", domain.CodeOf(err), domain.CodeConflict)
			}
		})
	}
}

// Half-open ranges make back-to-back bookings legal. An arena that could not
// sell 18:00-19:00 and 19:00-20:00 to different teams would lose half its day.
func TestAdjacentSlotsAreAllowed(t *testing.T) {
	f := newFixture(t, 2)
	ctx := context.Background()

	if _, err := f.repo.CreateHold(ctx, f.hold(t, f.players[0], f.slotAt(t, 18, time.Hour))); err != nil {
		t.Fatalf("first booking: %v", err)
	}
	if _, err := f.repo.CreateHold(ctx, f.hold(t, f.players[1], f.slotAt(t, 19, time.Hour))); err != nil {
		t.Fatalf("the adjacent 19:00 slot should have been bookable: %v", err)
	}
}

func TestCancellingFreesTheSlot(t *testing.T) {
	f := newFixture(t, 2)
	ctx := context.Background()
	slot := f.slotAt(t, 18, time.Hour)

	first, err := f.repo.CreateHold(ctx, f.hold(t, f.players[0], slot))
	if err != nil {
		t.Fatalf("first booking: %v", err)
	}

	// While it stands, nobody else can have the slot.
	if _, err := f.repo.CreateHold(ctx, f.hold(t, f.players[1], slot)); err == nil {
		t.Fatal("the slot should have been unavailable while held")
	}

	if err := f.repo.Cancel(ctx, first.ID, f.players[0]); err != nil {
		t.Fatalf("cancelling: %v", err)
	}

	if _, err := f.repo.CreateHold(ctx, f.hold(t, f.players[1], slot)); err != nil {
		t.Errorf("a cancelled slot should be immediately rebookable: %v", err)
	}
}

// A stranger must not be able to cancel someone else's booking, and must not
// learn that it exists.
func TestCancelRequiresOwnership(t *testing.T) {
	f := newFixture(t, 2)
	ctx := context.Background()

	booking, err := f.repo.CreateHold(ctx, f.hold(t, f.players[0], f.slotAt(t, 18, time.Hour)))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}

	err = f.repo.Cancel(ctx, booking.ID, f.players[1])
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("code = %q, want %q so the booking's existence stays private",
			domain.CodeOf(err), domain.CodeNotFound)
	}

	// It must still be live.
	if n := f.countLiveBookings(t, booking.Slot); n != 1 {
		t.Errorf("the booking was affected by a stranger's cancellation: %d live", n)
	}

	// Cancelling twice is a conflict, not a silent success.
	if err := f.repo.Cancel(ctx, booking.ID, f.players[0]); err != nil {
		t.Fatalf("the owner's cancellation failed: %v", err)
	}
	if err := f.repo.Cancel(ctx, booking.ID, f.players[0]); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("a second cancellation should conflict, got %v", err)
	}
}

// An abandoned checkout must not hold a court forever. Once the hold window
// lapses the slot is bookable again, without waiting for the janitor.
func TestExpiredHoldsReleaseTheirSlot(t *testing.T) {
	f := newFixture(t, 2)
	ctx := context.Background()
	slot := f.slotAt(t, 18, time.Hour)

	// A hold that expired a minute ago.
	stale := f.hold(t, f.players[0], slot)
	expired := time.Now().UTC().Add(-time.Minute)
	stale.HoldExpiresAt = &expired

	if _, err := f.repo.CreateHold(ctx, stale); err != nil {
		t.Fatalf("seeding the stale hold: %v", err)
	}

	// Someone else may take the slot straight away: CreateHold releases the
	// lapsed hold inside the same transaction.
	if _, err := f.repo.CreateHold(ctx, f.hold(t, f.players[1], slot)); err != nil {
		t.Fatalf("an expired hold should not block a new booking: %v", err)
	}

	if n := f.countLiveBookings(t, slot); n != 1 {
		t.Errorf("%d live bookings after the takeover, want 1", n)
	}
}

// A hold with a verified payment behind it is somebody's game, however old
// the hold window says it is.
func TestPaidHoldsAreNeverReleased(t *testing.T) {
	f := newFixture(t, 2)
	ctx := context.Background()
	slot := f.slotAt(t, 18, time.Hour)

	paid := f.hold(t, f.players[0], slot)
	expired := time.Now().UTC().Add(-time.Hour)
	paid.HoldExpiresAt = &expired

	booking, err := f.repo.CreateHold(ctx, paid)
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}

	_, err = f.pool.Exec(ctx, `
		insert into payments (booking_id, provider, amount_npr, status, transaction_uuid, verified_at)
		values ($1, 'esewa', $2, 'verified', $3, now())`,
		booking.ID, booking.PriceNPR, "txn-"+booking.ID.String())
	if err != nil {
		t.Fatalf("recording the payment: %v", err)
	}

	// The slot must stay taken despite the lapsed hold.
	if _, err := f.repo.CreateHold(ctx, f.hold(t, f.players[1], slot)); err == nil {
		t.Error("a paid booking was released as if it were an abandoned hold")
	}

	released, err := f.repo.ReleaseStaleHolds(ctx)
	if err != nil {
		t.Fatalf("running the janitor: %v", err)
	}
	if released != 0 {
		t.Errorf("the janitor released %d paid holds, want 0", released)
	}
}

func TestReleaseStaleHoldsCancelsAbandonedCheckouts(t *testing.T) {
	f := newFixture(t, 1)
	ctx := context.Background()
	slot := f.slotAt(t, 20, time.Hour)

	stale := f.hold(t, f.players[0], slot)
	expired := time.Now().UTC().Add(-time.Minute)
	stale.HoldExpiresAt = &expired

	booking, err := f.repo.CreateHold(ctx, stale)
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}

	// An abandoned payment intent, which should be failed alongside the hold.
	_, err = f.pool.Exec(ctx, `
		insert into payments (booking_id, provider, amount_npr, status, transaction_uuid)
		values ($1, 'khalti', $2, 'initiated', $3)`,
		booking.ID, booking.PriceNPR, "txn-"+booking.ID.String())
	if err != nil {
		t.Fatalf("recording the intent: %v", err)
	}

	released, err := f.repo.ReleaseStaleHolds(ctx)
	if err != nil {
		t.Fatalf("running the janitor: %v", err)
	}
	if released < 1 {
		t.Fatalf("the janitor released %d holds, want at least 1", released)
	}

	after, err := f.repo.ByID(ctx, booking.ID)
	if err != nil {
		t.Fatalf("reloading the booking: %v", err)
	}
	if after.Status != domain.BookingCancelled {
		t.Errorf("status = %q, want %q so the player's list tells the truth",
			after.Status, domain.BookingCancelled)
	}

	var paymentStatus string
	err = f.pool.QueryRow(ctx, `select status from payments where booking_id = $1`, booking.ID).
		Scan(&paymentStatus)
	if err != nil {
		t.Fatalf("reloading the payment: %v", err)
	}
	if paymentStatus != "failed" {
		t.Errorf("payment status = %q, want \"failed\": an abandoned intent must not sit initiated forever",
			paymentStatus)
	}
}

func TestBookedRangesReflectsWhatBlocksASlot(t *testing.T) {
	f := newFixture(t, 3)
	ctx := context.Background()

	live := f.slotAt(t, 18, time.Hour)
	if _, err := f.repo.CreateHold(ctx, f.hold(t, f.players[0], live)); err != nil {
		t.Fatalf("seeding the live hold: %v", err)
	}

	// An expired hold, which must not appear.
	staleSlot := f.slotAt(t, 20, time.Hour)
	stale := f.hold(t, f.players[1], staleSlot)
	expired := time.Now().UTC().Add(-time.Minute)
	stale.HoldExpiresAt = &expired
	if _, err := f.repo.CreateHold(ctx, stale); err != nil {
		t.Fatalf("seeding the stale hold: %v", err)
	}

	// A cancelled booking, which must not appear either.
	cancelledSlot := f.slotAt(t, 21, time.Hour)
	cancelled, err := f.repo.CreateHold(ctx, f.hold(t, f.players[2], cancelledSlot))
	if err != nil {
		t.Fatalf("seeding the cancelled booking: %v", err)
	}
	if err := f.repo.Cancel(ctx, cancelled.ID, f.players[2]); err != nil {
		t.Fatalf("cancelling: %v", err)
	}

	window := domain.GridWindow(live.Start, time.UTC)
	ranges, err := f.repo.BookedRanges(ctx, f.courtID, window)
	if err != nil {
		t.Fatalf("loading booked ranges: %v", err)
	}

	if len(ranges) != 1 {
		t.Fatalf("got %d booked ranges, want 1 (the live hold only): %v", len(ranges), ranges)
	}
	if !ranges[0].Start.Equal(live.Start) {
		t.Errorf("booked range = %s, want the live hold at %s", ranges[0], live)
	}
}

func TestListForUserJoinsArenaContext(t *testing.T) {
	f := newFixture(t, 2)
	ctx := context.Background()

	booking, err := f.repo.CreateHold(ctx, f.hold(t, f.players[0], f.slotAt(t, 18, time.Hour)))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	// A second player's booking, which must not leak into the first's list.
	if _, err := f.repo.CreateHold(ctx, f.hold(t, f.players[1], f.slotAt(t, 19, time.Hour))); err != nil {
		t.Fatalf("seeding the other player: %v", err)
	}

	list, err := f.repo.ListForUser(ctx, f.players[0], 50)
	if err != nil {
		t.Fatalf("listing: %v", err)
	}

	if len(list) != 1 {
		t.Fatalf("got %d bookings, want only this player's 1", len(list))
	}
	got := list[0]
	if got.ID != booking.ID {
		t.Errorf("returned the wrong booking")
	}
	if got.CourtLabel != "Court A" {
		t.Errorf("court label = %q, want %q", got.CourtLabel, "Court A")
	}
	if got.ArenaArea != "Jhamsikhel" {
		t.Errorf("arena area = %q, want %q", got.ArenaArea, "Jhamsikhel")
	}
	if got.ArenaName == "" {
		t.Error("the arena name should be joined in, not left to a second query")
	}
	if got.OpenToJoin {
		t.Error("a booking with no community post should not read as open to join")
	}
}
