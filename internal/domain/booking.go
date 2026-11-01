package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Booking is a reservation of one court for one slot.
//
// A booking starts life as an unpaid hold: `Pending` with a `HoldExpiresAt`
// in the near future. Payment promotes it to `Confirmed` and clears the
// expiry, because a paid booking never lapses. If payment never arrives the
// hold expires, the slot frees itself, and the janitor eventually writes
// `Cancelled` so the player's own list tells the truth.
type Booking struct {
	ID       uuid.UUID
	CourtID  uuid.UUID
	UserID   uuid.UUID
	TeamID   *uuid.UUID
	Slot     Slot
	PriceNPR int
	IsPeak   bool
	Status   BookingStatus
	Note     string

	// HoldExpiresAt is set while Status is Pending and nil otherwise.
	HoldExpiresAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

// BlocksSlot reports whether this booking currently prevents someone else
// from taking the same court and time.
//
// This is the authoritative Go statement of the rule, and it matches the
// `booking_blocks_slot` SQL function exactly. Where the two are both applied
// -- the availability grid reads the SQL version, the create path relies on
// the EXCLUDE constraint -- they must agree, or a slot renders free and then
// refuses to book.
func (b Booking) BlocksSlot(now time.Time) bool {
	if !b.Status.IsLive() {
		return false
	}
	if b.Status == BookingPending {
		return b.HoldExpiresAt != nil && b.HoldExpiresAt.After(now)
	}
	return true
}

// IsHeldBy reports whether this booking belongs to the given user. Ownership
// is the basis of every authorization decision about a booking, so it is
// stated once here rather than re-derived at each call site.
func (b Booking) IsHeldBy(userID uuid.UUID) bool { return b.UserID == userID }

// HoldExpired reports whether an unpaid hold has lapsed.
func (b Booking) HoldExpired(now time.Time) bool {
	return b.Status == BookingPending &&
		b.HoldExpiresAt != nil &&
		!b.HoldExpiresAt.After(now)
}

// CanBeCancelledBy states who may cancel and when, and explains a refusal in
// terms the player will understand.
func (b Booking) CanBeCancelledBy(userID uuid.UUID, now time.Time) error {
	if !b.IsHeldBy(userID) {
		// Deliberately "not found": a stranger should not learn that a
		// booking exists by being told they are not allowed to touch it.
		return NotFound("That booking doesn't exist.")
	}
	if !b.Status.CanCancel() {
		switch b.Status {
		case BookingCancelled:
			return Conflict("This booking is already cancelled.")
		case BookingCompleted:
			return Conflict("This game has already been played.")
		default:
			return Conflict("This booking can no longer be cancelled.")
		}
	}
	if b.Slot.Start.Before(now) {
		return Conflict("This slot has already started.")
	}
	return nil
}

// CanBeOpenedToCommunity states whether the owner may advertise this booking
// on the find-a-player board.
func (b Booking) CanBeOpenedToCommunity(userID uuid.UUID, now time.Time) error {
	if !b.IsHeldBy(userID) {
		return NotFound("That booking doesn't exist.")
	}
	if !b.Status.IsLive() {
		return Conflict("This booking is no longer active.")
	}
	if b.Status == BookingCompleted {
		return Conflict("This game has already been played.")
	}
	if b.Slot.IsPast(now) {
		return Conflict("This slot has already started.")
	}
	return nil
}

// NewHold builds a pending booking for a slot whose price has already been
// resolved server-side. The caller supplies `now` and the hold window so that
// time is an input to the rule rather than a hidden dependency on the clock.
func NewHold(courtID, userID uuid.UUID, slot Slot, price Price, teamID *uuid.UUID, note string, now time.Time, holdWindow time.Duration) (Booking, error) {
	if err := slot.Validate(); err != nil {
		return Booking{}, err
	}
	if slot.IsPast(now) {
		return Booking{}, Invalid("slot", "That slot has already started.")
	}
	if len([]rune(note)) > 280 {
		return Booking{}, Invalid("note", "Keep the note under 280 characters.")
	}
	if holdWindow <= 0 {
		return Booking{}, Internal(nil, "hold window must be positive, got %s", holdWindow)
	}

	expires := now.Add(holdWindow)
	return Booking{
		CourtID:       courtID,
		UserID:        userID,
		TeamID:        teamID,
		Slot:          slot,
		PriceNPR:      price.TotalNPR,
		IsPeak:        price.IsPeak,
		Status:        BookingPending,
		Note:          note,
		HoldExpiresAt: &expires,
	}, nil
}

// BookingDetail is a booking with the court and arena context a player needs
// to recognise it: "Court A at Dhuku Futsal, Jhamsikhel", not a pair of UUIDs.
//
// It is a read model, assembled by one join rather than by loading a booking
// and then looking up its court and arena. That per-row lookup is what made
// the previous implementation slow, so the shape of this type is deliberate.
type BookingDetail struct {
	Booking

	CourtLabel string
	ArenaID    uuid.UUID
	ArenaName  string
	ArenaArea  string
	ArenaSlug  string

	// OpenToJoin reports whether a live community post is advertising this
	// booking. Derived from the posts table rather than stored on the
	// booking, so the two can never disagree.
	OpenToJoin bool

	// PaymentProvider and PaymentStatus describe the latest payment attempt,
	// nil when there has been none. They are what tell a player that a cash
	// booking is waiting on the venue rather than on them.
	PaymentProvider *PaymentProvider
	PaymentStatus   *PaymentStatus

	// PaidOnline is set when an eSewa or Khalti payment for this booking was
	// verified. The player can't cancel it themselves: the money is in the
	// venue's account, so the venue has to refund it.
	PaidOnline *PaymentProvider
	// ArenaPhone is who to call about a refund.
	ArenaPhone string
	// CancelReason is what the player said when they cancelled.
	CancelReason string
}

// ---------------------------------------------------------------------------
// Availability
// ---------------------------------------------------------------------------

// GridSlot is one cell of the availability matrix a player picks from: an
// hour on a court, priced, with whether it can be taken.
//
// These are projected on demand from operating hours, pricing rules and live
// bookings. Nothing materialises a row per hour per court -- that would be
// millions of rows describing time, which passes anyway.
type GridSlot struct {
	Slot     Slot
	PriceNPR int
	IsPeak   bool
	// RuleLabel names the rate that applied, for the line shown under the
	// price ("Evening Peak", or BaseRateLabel).
	RuleLabel string
	IsBooked  bool
	IsPast    bool
}

// Available reports whether a player can actually take this cell.
func (g GridSlot) Available() bool { return !g.IsBooked && !g.IsPast }

// ---------------------------------------------------------------------------
// Cancellation and the user score
// ---------------------------------------------------------------------------

// The user score rules. A player starts at UserScoreStart. Cancelling a
// confirmed booking less than LateCancelWindow before it starts costs
// LateCancelPenalty points. At BookingBanFloor or below, the player can't
// book for BookingBanLength; once that has passed the score reads as
// UserScoreAfterBan.
//
// Only confirmed bookings count. Letting go of an unpaid hold costs nothing,
// so nobody is pushed into leaving a hold to lapse instead of releasing it.
const (
	UserScoreStart    = 100
	LateCancelWindow  = 5 * time.Hour
	LateCancelPenalty = 20
	BookingBanFloor   = 10
	UserScoreAfterBan = 50
)

// BookingBanLength is how long a player at or below BookingBanFloor can't
// book. Kept as a Postgres interval: "a month" is a calendar month there.
const BookingBanLength = "1 month"

// MaxCancelReasonLength matches the column check in migration 0013.
const MaxCancelReasonLength = 300

// CancelReason checks and tidies the reason a player gives for cancelling.
func CancelReason(raw string) (string, error) {
	reason := strings.TrimSpace(raw)
	v := &Validation{}
	v.Check(len([]rune(reason)) >= 3, "reason", "Tell the venue why you're cancelling.")
	v.Check(len([]rune(reason)) <= MaxCancelReasonLength, "reason",
		"Keep the reason under %d characters.", MaxCancelReasonLength)
	if err := v.Err(); err != nil {
		return "", err
	}
	return reason, nil
}

// CancelOutcome is what cancelling did to the player's standing.
type CancelOutcome struct {
	// Late is true when the booking was confirmed and less than
	// LateCancelWindow from its start, so it cost points.
	Late bool
	// UserScore is the player's score after the cancellation.
	UserScore int
	// BookingBannedUntil is set when the player can't book until then.
	BookingBannedUntil *time.Time
}

// BookingStanding is whether a player may book right now.
type BookingStanding struct {
	UserScore          int
	BookingBannedUntil *time.Time
}

// Banned reports whether the player is barred from booking at `now`.
func (s BookingStanding) Banned(now time.Time) bool {
	return s.BookingBannedUntil != nil && s.BookingBannedUntil.After(now)
}

// Cancellation is a cancelled booking as its venue sees it: who, when, why,
// and what it cost the player.
type Cancellation struct {
	BookingID      uuid.UUID
	Slot           Slot
	CourtLabel     string
	PlayerName     string
	PlayerUsername string
	Reason         string
	CancelledAt    time.Time
	// Penalty is the points the cancellation took off the player's score;
	// zero when it wasn't late.
	Penalty int
	// PenaltyRestoredAt is set once the venue has given those points back.
	PenaltyRestoredAt *time.Time
}

// CanRestore reports whether the venue can still give the points back.
func (c Cancellation) CanRestore() bool {
	return c.Penalty > 0 && c.PenaltyRestoredAt == nil
}
