package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func ptr[T any](v T) *T { return &v }

// BlocksSlot is the Go statement of the same rule the SQL function
// `booking_blocks_slot` enforces. The availability grid reads the SQL
// version; if the two disagree, a slot renders free and then refuses to book.
func TestBookingBlocksSlot(t *testing.T) {
	now := time.Date(2030, time.March, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		status  BookingStatus
		expires *time.Time
		want    bool
	}{
		{"confirmed", BookingConfirmed, nil, true},
		{"completed", BookingCompleted, nil, true},
		{"cancelled", BookingCancelled, nil, false},
		{"no-show", BookingNoShow, nil, false},
		{"pending, hold still live", BookingPending, ptr(now.Add(10 * time.Minute)), true},
		{"pending, hold just expired", BookingPending, ptr(now.Add(-time.Second)), false},
		{"pending, hold expired long ago", BookingPending, ptr(now.Add(-time.Hour)), false},
		{"pending, hold expiring exactly now", BookingPending, ptr(now), false},
		{"pending with no expiry recorded", BookingPending, nil, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := Booking{Status: tc.status, HoldExpiresAt: tc.expires}
			if got := b.BlocksSlot(now); got != tc.want {
				t.Errorf("BlocksSlot = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBookingHoldExpired(t *testing.T) {
	now := time.Date(2030, time.March, 15, 12, 0, 0, 0, time.UTC)

	live := Booking{Status: BookingPending, HoldExpiresAt: ptr(now.Add(time.Minute))}
	if live.HoldExpired(now) {
		t.Error("a hold with time left has not expired")
	}

	lapsed := Booking{Status: BookingPending, HoldExpiresAt: ptr(now.Add(-time.Minute))}
	if !lapsed.HoldExpired(now) {
		t.Error("a hold past its expiry has expired")
	}

	// Only pending bookings hold. A confirmed one is paid for and stays.
	confirmed := Booking{Status: BookingConfirmed, HoldExpiresAt: ptr(now.Add(-time.Hour))}
	if confirmed.HoldExpired(now) {
		t.Error("a confirmed booking never expires, whatever its old hold said")
	}
}

func TestNewHoldPricesAndExpires(t *testing.T) {
	now := time.Date(2030, time.March, 15, 12, 0, 0, 0, time.UTC)
	courtID, userID := uuid.New(), uuid.New()

	slot, err := NewSlot(now.Add(6*time.Hour), now.Add(7*time.Hour))
	if err != nil {
		t.Fatalf("building slot: %v", err)
	}
	price := Price{PerHourNPR: 2000, TotalNPR: 2000, IsPeak: true}

	b, err := NewHold(courtID, userID, slot, price, nil, "bring bibs", now, 15*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if b.Status != BookingPending {
		t.Errorf("status = %q, want %q", b.Status, BookingPending)
	}
	if b.PriceNPR != 2000 {
		t.Errorf("price = %d, want the resolved 2000", b.PriceNPR)
	}
	if !b.IsPeak {
		t.Error("the resolved peak flag should carry onto the booking")
	}
	if b.HoldExpiresAt == nil {
		t.Fatal("a pending hold must record when it expires")
	}
	if want := now.Add(15 * time.Minute); !b.HoldExpiresAt.Equal(want) {
		t.Errorf("hold expires at %s, want %s", b.HoldExpiresAt, want)
	}
	if !b.BlocksSlot(now) {
		t.Error("a fresh hold should block its slot")
	}
	if b.BlocksSlot(now.Add(16 * time.Minute)) {
		t.Error("the hold should stop blocking once its window passes")
	}
}

func TestNewHoldRejectsPastSlots(t *testing.T) {
	now := time.Date(2030, time.March, 15, 12, 0, 0, 0, time.UTC)
	slot := Slot{Start: now.Add(-2 * time.Hour), End: now.Add(-time.Hour)}

	_, err := NewHold(uuid.New(), uuid.New(), slot, Price{TotalNPR: 1000}, nil, "", now, 15*time.Minute)
	if err == nil {
		t.Fatal("a slot in the past should not be bookable")
	}
	if !errors.Is(err, ErrInvalid) {
		t.Errorf("code = %q, want %q", CodeOf(err), CodeInvalid)
	}
}

func TestNewHoldRejectsAnOverlongNote(t *testing.T) {
	now := time.Date(2030, time.March, 15, 12, 0, 0, 0, time.UTC)
	slot, err := NewSlot(now.Add(time.Hour), now.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("building slot: %v", err)
	}

	note := make([]rune, 281)
	for i := range note {
		note[i] = 'a'
	}

	if _, err := NewHold(uuid.New(), uuid.New(), slot, Price{TotalNPR: 1000}, nil, string(note), now, 15*time.Minute); err == nil {
		t.Error("a note over 280 characters should be rejected")
	}
}

func TestBookingCancellationRules(t *testing.T) {
	now := time.Date(2030, time.March, 15, 12, 0, 0, 0, time.UTC)
	owner, stranger := uuid.New(), uuid.New()

	future := Slot{Start: now.Add(6 * time.Hour), End: now.Add(7 * time.Hour)}
	booking := func(status BookingStatus, slot Slot) Booking {
		return Booking{UserID: owner, Status: status, Slot: slot}
	}

	t.Run("the owner may cancel a live booking", func(t *testing.T) {
		for _, status := range []BookingStatus{BookingPending, BookingConfirmed} {
			if err := booking(status, future).CanBeCancelledBy(owner, now); err != nil {
				t.Errorf("a %s booking should be cancellable: %v", status, err)
			}
		}
	})

	// A stranger is told the booking does not exist rather than that they
	// may not touch it: the refusal itself would confirm it is real.
	t.Run("a stranger learns nothing", func(t *testing.T) {
		err := booking(BookingConfirmed, future).CanBeCancelledBy(stranger, now)
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("code = %q, want %q so the booking's existence stays private",
				CodeOf(err), CodeNotFound)
		}
	})

	t.Run("finished bookings cannot be cancelled", func(t *testing.T) {
		for _, status := range []BookingStatus{BookingCancelled, BookingCompleted, BookingNoShow} {
			err := booking(status, future).CanBeCancelledBy(owner, now)
			if !errors.Is(err, ErrConflict) {
				t.Errorf("a %s booking should refuse cancellation, got %v", status, err)
			}
		}
	})

	t.Run("a slot that has started cannot be cancelled", func(t *testing.T) {
		started := Slot{Start: now.Add(-30 * time.Minute), End: now.Add(30 * time.Minute)}
		if err := booking(BookingConfirmed, started).CanBeCancelledBy(owner, now); err == nil {
			t.Error("a game already under way should not be cancellable")
		}
	})
}

func TestBookingCanBeOpenedToCommunity(t *testing.T) {
	now := time.Date(2030, time.March, 15, 12, 0, 0, 0, time.UTC)
	owner, stranger := uuid.New(), uuid.New()
	future := Slot{Start: now.Add(6 * time.Hour), End: now.Add(7 * time.Hour)}

	live := Booking{UserID: owner, Status: BookingConfirmed, Slot: future}
	if err := live.CanBeOpenedToCommunity(owner, now); err != nil {
		t.Errorf("the owner should be able to open a live booking: %v", err)
	}

	if err := live.CanBeOpenedToCommunity(stranger, now); !errors.Is(err, ErrNotFound) {
		t.Errorf("a stranger should get %q, got %q", CodeNotFound, CodeOf(err))
	}

	cancelled := Booking{UserID: owner, Status: BookingCancelled, Slot: future}
	if err := cancelled.CanBeOpenedToCommunity(owner, now); err == nil {
		t.Error("a cancelled booking should not be advertised")
	}

	past := Booking{UserID: owner, Status: BookingConfirmed,
		Slot: Slot{Start: now.Add(-2 * time.Hour), End: now.Add(-time.Hour)}}
	if err := past.CanBeOpenedToCommunity(owner, now); err == nil {
		t.Error("a booking whose slot has passed should not be advertised")
	}
}

func TestGridSlotAvailability(t *testing.T) {
	tests := []struct {
		name             string
		booked, past, ok bool
	}{
		{"free and upcoming", false, false, true},
		{"already booked", true, false, false},
		{"in the past", false, true, false},
		{"booked and in the past", true, true, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := GridSlot{IsBooked: tc.booked, IsPast: tc.past}
			if got := g.Available(); got != tc.ok {
				t.Errorf("Available = %v, want %v", got, tc.ok)
			}
		})
	}
}
