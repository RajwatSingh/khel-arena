package postgres

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

// The user score rules live in SQL: what counts as late, what a cancellation
// costs, when a ban starts and when it runs out.

// bookFor seeds a booking that starts `in` from now, in the given status.
func (f *fixture) bookFor(t *testing.T, player uuid.UUID, in time.Duration, status string) uuid.UUID {
	t.Helper()

	start := time.Now().UTC().Truncate(time.Minute).Add(in)
	slot, err := domain.NewSlot(start, start.Add(time.Hour))
	if err != nil {
		t.Fatalf("building slot: %v", err)
	}

	var id uuid.UUID
	err = f.pool.QueryRow(context.Background(), `
		insert into bookings (court_id, user_id, slot, price_npr, status, hold_expires_at)
		values ($1, $2, $3, 1200, $4::booking_status,
		        case when $4 = 'pending' then now() + interval '15 minutes' end)
		returning id`,
		f.courtID, player, tstzrange(slot), status).Scan(&id)
	if err != nil {
		t.Fatalf("seeding %s booking: %v", status, err)
	}
	return id
}

func (f *fixture) standing(t *testing.T, player uuid.UUID) domain.BookingStanding {
	t.Helper()
	s, err := f.repo.BookingStanding(context.Background(), player)
	if err != nil {
		t.Fatalf("reading standing: %v", err)
	}
	return s
}

func TestLateCancelCostsPointsAndRecordsTheReason(t *testing.T) {
	f := newFixture(t, 1)
	ctx := context.Background()
	player := f.players[0]

	if s := f.standing(t, player); s.UserScore != domain.UserScoreStart || s.BookingBannedUntil != nil {
		t.Fatalf("a new player starts at %+v, want 100 and no ban", s)
	}

	// Confirmed, two hours away: late.
	late := f.bookFor(t, player, 2*time.Hour, "confirmed")
	outcome, err := f.repo.Cancel(ctx, late, player, "Not enough players")
	if err != nil {
		t.Fatalf("cancelling: %v", err)
	}
	if !outcome.Late || outcome.UserScore != 80 || outcome.BookingBannedUntil != nil {
		t.Errorf("outcome = %+v, want late, 80, no ban", outcome)
	}

	var reason string
	var cancelledAt *time.Time
	err = f.pool.QueryRow(ctx, `select cancel_reason, cancelled_at from bookings where id = $1`, late).
		Scan(&reason, &cancelledAt)
	if err != nil || reason != "Not enough players" || cancelledAt == nil {
		t.Errorf("reason = %q, cancelled_at = %v, err = %v", reason, cancelledAt, err)
	}

	// Confirmed, two days away: not late.
	early := f.bookFor(t, player, 48*time.Hour, "confirmed")
	outcome, err = f.repo.Cancel(ctx, early, player, "Plans changed")
	if err != nil || outcome.Late || outcome.UserScore != 80 {
		t.Errorf("early cancel: outcome = %+v, err = %v, want no penalty", outcome, err)
	}

	// An unpaid hold an hour away: holds never cost points.
	hold := f.bookFor(t, player, time.Hour, "pending")
	outcome, err = f.repo.Cancel(ctx, hold, player, "Booked the wrong time")
	if err != nil || outcome.Late || outcome.UserScore != 80 {
		t.Errorf("hold cancel: outcome = %+v, err = %v, want no penalty", outcome, err)
	}
}

func TestFallingToTheFloorBansBookingForAMonth(t *testing.T) {
	f := newFixture(t, 1)
	ctx := context.Background()
	player := f.players[0]

	// 100 -> 80 -> 60 -> 40 -> 20 -> 0. The fifth crosses the floor of 10.
	var outcome domain.CancelOutcome
	for i := range 5 {
		// Same hour each time: a cancelled booking frees its slot.
		id := f.bookFor(t, player, 2*time.Hour, "confirmed")
		var err error
		outcome, err = f.repo.Cancel(ctx, id, player, "Plans changed")
		if err != nil {
			t.Fatalf("cancel %d: %v", i+1, err)
		}
		if i < 4 && outcome.BookingBannedUntil != nil {
			t.Fatalf("banned after only %d late cancels (score %d)", i+1, outcome.UserScore)
		}
	}

	if outcome.UserScore != 0 || outcome.BookingBannedUntil == nil {
		t.Fatalf("after five late cancels: %+v, want 0 and a ban", outcome)
	}
	until := *outcome.BookingBannedUntil
	if days := time.Until(until).Hours() / 24; days < 27 || days > 32 {
		t.Errorf("ban lasts %.1f days, want about a month", days)
	}
	if !f.standing(t, player).Banned(time.Now()) {
		t.Error("BookingStanding does not report the ban")
	}

	// A month later: the ban has run out, and the score reads as 50.
	if _, err := f.pool.Exec(ctx,
		`update users set booking_banned_until = now() - interval '1 minute' where id = $1`, player); err != nil {
		t.Fatalf("ageing the ban: %v", err)
	}
	if s := f.standing(t, player); s.Banned(time.Now()) || s.UserScore != domain.UserScoreAfterBan {
		t.Errorf("after the ban: %+v, want 50 and no ban", s)
	}

	// The next late cancel is charged from 50, not from 0.
	id := f.bookFor(t, player, 3*time.Hour, "confirmed")
	outcome, err := f.repo.Cancel(ctx, id, player, "Plans changed")
	if err != nil {
		t.Fatalf("cancel after ban: %v", err)
	}
	if outcome.UserScore != 30 || outcome.BookingBannedUntil != nil {
		t.Errorf("after the ban, a late cancel left %+v, want 30 and no ban", outcome)
	}
}

func TestABookingPaidOnlineCannotBeCancelledByThePlayer(t *testing.T) {
	f := newFixture(t, 1)
	ctx := context.Background()
	player := f.players[0]

	id := f.bookFor(t, player, 72*time.Hour, "confirmed")
	_, err := f.pool.Exec(ctx, `
		insert into payments (booking_id, provider, amount_npr, status, transaction_uuid, verified_at)
		values ($1, 'khalti', 1200, 'verified', $2, now())`, id, "txn-"+id.String())
	if err != nil {
		t.Fatalf("recording the payment: %v", err)
	}

	_, err = f.repo.Cancel(ctx, id, player, "Plans changed")
	if domain.CodeOf(err) != domain.CodeConflict {
		t.Fatalf("code = %q, want conflict", domain.CodeOf(err))
	}
	if !strings.Contains(err.Error(), "refund") {
		t.Errorf("message = %q, want it to point at a refund from the venue", err)
	}

	var status string
	_ = f.pool.QueryRow(ctx, `select status from bookings where id = $1`, id).Scan(&status)
	if status != "confirmed" {
		t.Errorf("status = %q, want it untouched", status)
	}

	// The bookings list tells the client so, and who to call.
	list, err := f.repo.ListForUser(ctx, player, 10)
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %v, err = %v", list, err)
	}
	if list[0].PaidOnline == nil || *list[0].PaidOnline != domain.ProviderKhalti {
		t.Errorf("paid_online = %v, want khalti", list[0].PaidOnline)
	}
}

func TestTheVenueSeesTheReasonAndCanGiveThePointsBack(t *testing.T) {
	f := newFixture(t, 2)
	ctx := context.Background()
	player, stranger := f.players[0], f.players[1]

	// Five late cancels: score 0 and banned.
	var last uuid.UUID
	for range 5 {
		last = f.bookFor(t, player, 2*time.Hour, "confirmed")
		if _, err := f.repo.Cancel(ctx, last, player, "Family emergency"); err != nil {
			t.Fatalf("cancelling: %v", err)
		}
	}
	early := f.bookFor(t, player, 72*time.Hour, "confirmed")
	if _, err := f.repo.Cancel(ctx, early, player, "Plans changed"); err != nil {
		t.Fatalf("cancelling early: %v", err)
	}
	if !f.standing(t, player).Banned(time.Now()) {
		t.Fatal("setup: the player should be banned")
	}

	// The owner sees every cancellation, newest first, with its reason.
	list, err := f.repo.CancellationsForOwner(ctx, f.arenaID, f.owner, 10)
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	if len(list) != 6 {
		t.Fatalf("cancellations = %d, want 6", len(list))
	}
	if list[0].BookingID != early || list[0].Reason != "Plans changed" || list[0].CanRestore() {
		t.Errorf("newest = %+v, want the early one, with nothing to restore", list[0])
	}
	if list[1].Penalty != domain.LateCancelPenalty || !list[1].CanRestore() {
		t.Errorf("a late one = %+v, want 20 points to restore", list[1])
	}

	// Another arena owner, or a player, sees nothing and can't restore.
	if got, _ := f.repo.CancellationsForOwner(ctx, f.arenaID, stranger, 10); len(got) != 0 {
		t.Errorf("a stranger listed %d cancellations", len(got))
	}
	if _, err := f.repo.RestorePenaltyOwnerScoped(ctx, last, stranger); domain.CodeOf(err) != domain.CodeNotFound {
		t.Errorf("stranger restore: code = %q, want not_found", domain.CodeOf(err))
	}

	// The owner gives one back: 0 -> 20, which lifts the ban.
	row, err := f.repo.RestorePenaltyOwnerScoped(ctx, last, f.owner)
	if err != nil {
		t.Fatalf("restoring: %v", err)
	}
	if row.PenaltyRestoredAt == nil || row.CanRestore() {
		t.Errorf("row after restore = %+v", row)
	}
	if s := f.standing(t, player); s.UserScore != 20 || s.Banned(time.Now()) {
		t.Errorf("after restoring: %+v, want 20 and no ban", s)
	}

	// Once only.
	if _, err := f.repo.RestorePenaltyOwnerScoped(ctx, last, f.owner); domain.CodeOf(err) != domain.CodeConflict {
		t.Errorf("second restore: code = %q, want conflict", domain.CodeOf(err))
	}
	// Nothing to give back on a cancellation that cost nothing.
	if _, err := f.repo.RestorePenaltyOwnerScoped(ctx, early, f.owner); domain.CodeOf(err) != domain.CodeConflict {
		t.Errorf("restoring a free cancel: code = %q, want conflict", domain.CodeOf(err))
	}
}

func TestRestoringNeverGoesAbove100(t *testing.T) {
	f := newFixture(t, 1)
	ctx := context.Background()
	player := f.players[0]

	id := f.bookFor(t, player, time.Hour, "confirmed")
	if _, err := f.repo.Cancel(ctx, id, player, "Plans changed"); err != nil {
		t.Fatalf("cancelling: %v", err)
	}
	// Meanwhile the score was topped back up some other way.
	if _, err := f.pool.Exec(ctx, `update users set user_score = 95 where id = $1`, player); err != nil {
		t.Fatalf("setting score: %v", err)
	}
	if _, err := f.repo.RestorePenaltyOwnerScoped(ctx, id, f.owner); err != nil {
		t.Fatalf("restoring: %v", err)
	}
	if s := f.standing(t, player); s.UserScore != 100 {
		t.Errorf("score = %d, want capped at 100", s.UserScore)
	}
}
