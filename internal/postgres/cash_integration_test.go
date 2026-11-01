package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

// The pay-at-the-venue flow: the player books and the hour is confirmed at
// once, then the venue marks the cash received after the game. Approve and
// decline remain for requests made before cash bookings stopped needing
// approval.

func cashIntent(b domain.Booking) domain.Payment {
	return domain.Payment{
		BookingID:       b.ID,
		Provider:        domain.ProviderCash,
		AmountNPR:       b.PriceNPR,
		Status:          domain.PaymentInitiated,
		TransactionUUID: uuid.NewString(),
	}
}

func (f *fixture) bookCash(t *testing.T, player uuid.UUID, hour int) (domain.Booking, domain.Payment, *PaymentRepo) {
	t.Helper()
	ctx := context.Background()

	booking, err := f.repo.CreateHold(ctx, f.hold(t, player, f.slotAt(t, hour, time.Hour)))
	if err != nil {
		t.Fatalf("taking the hold: %v", err)
	}
	payments := NewPaymentRepo(f.pool, f.repo)
	p, err := payments.BookCash(ctx, cashIntent(booking))
	if err != nil {
		t.Fatalf("booking with cash: %v", err)
	}
	return booking, p, payments
}

// legacyCashRequest is a cash request from before cash bookings were
// confirmed on the spot: a pending hold with an open cash payment, waiting
// on the venue.
func (f *fixture) legacyCashRequest(t *testing.T, player uuid.UUID, hour int) (domain.Booking, domain.Payment, *PaymentRepo) {
	t.Helper()
	ctx := context.Background()

	booking, err := f.repo.CreateHold(ctx, f.hold(t, player, f.slotAt(t, hour, time.Hour)))
	if err != nil {
		t.Fatalf("taking the hold: %v", err)
	}
	payments := NewPaymentRepo(f.pool, f.repo)
	p, err := payments.Create(ctx, cashIntent(booking))
	if err != nil {
		t.Fatalf("recording the request: %v", err)
	}
	return booking, p, payments
}

func (f *fixture) bookingState(t *testing.T, id uuid.UUID) (domain.BookingStatus, *time.Time) {
	t.Helper()
	var status domain.BookingStatus
	var expires *time.Time
	if err := f.pool.QueryRow(context.Background(),
		`select status, hold_expires_at from bookings where id = $1`, id).Scan(&status, &expires); err != nil {
		t.Fatalf("reading booking: %v", err)
	}
	return status, expires
}

func (f *fixture) paymentStatus(t *testing.T, id uuid.UUID) domain.PaymentStatus {
	t.Helper()
	var status domain.PaymentStatus
	if err := f.pool.QueryRow(context.Background(),
		`select status from payments where id = $1`, id).Scan(&status); err != nil {
		t.Fatalf("reading payment: %v", err)
	}
	return status
}

func TestCashBookingConfirmsAtOnceAndIsIdempotent(t *testing.T) {
	f := newFixture(t, 1)
	booking, first, payments := f.bookCash(t, f.players[0], 18)

	status, expires := f.bookingState(t, booking.ID)
	if status != domain.BookingConfirmed || expires != nil {
		t.Errorf("status %q, expiry %v; want confirmed with no expiry", status, expires)
	}
	if got := f.paymentStatus(t, first.ID); got != domain.PaymentInitiated {
		t.Errorf("payment = %q, want initiated: the cash is still owed", got)
	}

	second, err := payments.BookCash(context.Background(), cashIntent(booking))
	if err != nil {
		t.Fatalf("asking again: %v", err)
	}
	if second.ID != first.ID {
		t.Error("a second tap recorded another cash payment for the same hour")
	}

	list, err := f.repo.ListForUser(context.Background(), f.players[0], 10)
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	if len(list) != 1 || list[0].PaymentProvider == nil || *list[0].PaymentProvider != domain.ProviderCash ||
		list[0].PaymentStatus == nil || *list[0].PaymentStatus != domain.PaymentInitiated {
		t.Errorf("the player's list does not show cash still owed: %+v", list)
	}
}

func TestCashBookingRefusesALapsedHold(t *testing.T) {
	f := newFixture(t, 1)
	ctx := context.Background()

	held := f.hold(t, f.players[0], f.slotAt(t, 18, time.Hour))
	lapsed := time.Now().UTC().Add(-time.Minute)
	held.HoldExpiresAt = &lapsed
	booking, err := f.repo.CreateHold(ctx, held)
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}

	_, err = NewPaymentRepo(f.pool, f.repo).BookCash(ctx, cashIntent(booking))
	if domain.CodeOf(err) != domain.CodeConflict {
		t.Errorf("code = %q, want conflict: a lapsed hold must not be revived", domain.CodeOf(err))
	}
	if status, _ := f.bookingState(t, booking.ID); status != domain.BookingPending {
		t.Errorf("booking = %q, want it left as it was", status)
	}
}

func TestCashBookingConfirmsALegacyRequest(t *testing.T) {
	f := newFixture(t, 1)
	booking, legacy, payments := f.legacyCashRequest(t, f.players[0], 18)

	got, err := payments.BookCash(context.Background(), cashIntent(booking))
	if err != nil {
		t.Fatalf("booking: %v", err)
	}
	if got.ID != legacy.ID {
		t.Error("a second cash payment was recorded instead of reusing the waiting request")
	}
	if status, _ := f.bookingState(t, booking.ID); status != domain.BookingConfirmed {
		t.Errorf("booking = %q, want confirmed", status)
	}
}

func TestCollectCashAfterTheGame(t *testing.T) {
	f := newFixture(t, 1)
	ctx := context.Background()
	booking, p, payments := f.bookCash(t, f.players[0], 18)

	// The game is played and the janitor marks it so before the venue gets
	// round to the till. Collecting the cash must still work.
	if _, err := f.pool.Exec(ctx, `update bookings set status = 'completed' where id = $1`, booking.ID); err != nil {
		t.Fatalf("marking played: %v", err)
	}

	settled, err := payments.MarkCashReceivedOwnerScoped(ctx, p.ID, f.owner, time.Now())
	if err != nil {
		t.Fatalf("collecting after the game: %v", err)
	}
	if settled.Status != domain.PaymentVerified {
		t.Errorf("payment = %q, want verified", settled.Status)
	}
	if status, _ := f.bookingState(t, booking.ID); status != domain.BookingCompleted {
		t.Errorf("booking = %q, want it left completed", status)
	}
}

func TestOnlyTheVenueOwnerCanTouchACashPayment(t *testing.T) {
	f := newFixture(t, 1)
	ctx := context.Background()
	booking, p, payments := f.legacyCashRequest(t, f.players[0], 18)

	stranger := seedUser(t, f, "otherowner")
	for name, act := range map[string]func(context.Context, uuid.UUID, uuid.UUID) (domain.Payment, error){
		"approve": payments.ApproveCashOwnerScoped,
		"decline": payments.DeclineCashOwnerScoped,
		"receive": func(ctx context.Context, id, owner uuid.UUID) (domain.Payment, error) {
			return payments.MarkCashReceivedOwnerScoped(ctx, id, owner, time.Now())
		},
	} {
		if _, err := act(ctx, p.ID, stranger); domain.CodeOf(err) != domain.CodeNotFound {
			t.Errorf("%s by a stranger: code = %q, want not_found", name, domain.CodeOf(err))
		}
	}
	if status, _ := f.bookingState(t, booking.ID); status != domain.BookingPending {
		t.Errorf("booking = %q, want it untouched", status)
	}
}

func TestDeclineFreesTheHour(t *testing.T) {
	f := newFixture(t, 2)
	ctx := context.Background()
	booking, p, payments := f.legacyCashRequest(t, f.players[0], 18)

	if _, err := payments.DeclineCashOwnerScoped(ctx, p.ID, f.owner); err != nil {
		t.Fatalf("declining: %v", err)
	}
	if status, _ := f.bookingState(t, booking.ID); status != domain.BookingCancelled {
		t.Errorf("booking = %q, want cancelled", status)
	}
	if got := f.paymentStatus(t, p.ID); got != domain.PaymentFailed {
		t.Errorf("payment = %q, want failed", got)
	}
	if _, err := f.repo.CreateHold(ctx, f.hold(t, f.players[1], booking.Slot)); err != nil {
		t.Errorf("the declined hour is still blocked: %v", err)
	}

	// A booked cash hour is a promise; the venue cannot decline it afterwards.
	other, q, _ := f.bookCash(t, f.players[0], 20)
	if _, err := payments.DeclineCashOwnerScoped(ctx, q.ID, f.owner); domain.CodeOf(err) != domain.CodeConflict {
		t.Errorf("declining a booked hour: code = %q, want conflict", domain.CodeOf(err))
	}
	if status, _ := f.bookingState(t, other.ID); status != domain.BookingConfirmed {
		t.Errorf("booked hour = %q, want still confirmed", status)
	}
}

func TestPlayerCancellingClosesTheCashPayment(t *testing.T) {
	f := newFixture(t, 1)
	booking, p, _ := f.bookCash(t, f.players[0], 18)

	if err := f.cancel(context.Background(), booking.ID, f.players[0]); err != nil {
		t.Fatalf("cancelling: %v", err)
	}
	if got := f.paymentStatus(t, p.ID); got != domain.PaymentFailed {
		t.Errorf("payment = %q, want failed so the venue stops expecting cash", got)
	}
}
