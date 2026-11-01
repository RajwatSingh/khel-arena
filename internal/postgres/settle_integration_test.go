package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

// When eSewa or Khalti says COMPLETE, the money has moved. These check that
// the verified row and the gateway's raw response are always kept, whatever
// has happened to the booking, and that a booking which can't take the money
// is flagged for the venue to refund.

// onlineIntent records an initiated gateway payment for a booking.
func (f *fixture) onlineIntent(t *testing.T, bookingID uuid.UUID, provider domain.PaymentProvider) domain.Payment {
	t.Helper()
	p, err := NewPaymentRepo(f.pool, f.repo).Create(context.Background(), domain.Payment{
		BookingID: bookingID, Provider: provider, AmountNPR: 1200,
		Status: domain.PaymentInitiated, TransactionUUID: "txn-" + uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("recording intent: %v", err)
	}
	return p
}

// complete is the gateway saying the money moved, as the service applies it.
func complete(t *testing.T, p domain.Payment) domain.Payment {
	t.Helper()
	_, err := p.Verify(domain.GatewayResult{
		Verified: true, TransactionUUID: p.TransactionUUID, AmountNPR: p.AmountNPR,
		ProviderRef: "ref-" + p.TransactionUUID, Raw: []byte(`{"status":"COMPLETE"}`),
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("verifying: %v", err)
	}
	return p
}

func (f *fixture) bookingStatus(t *testing.T, id uuid.UUID) domain.BookingStatus {
	t.Helper()
	b, err := f.repo.ByID(context.Background(), id)
	if err != nil {
		t.Fatalf("reloading booking: %v", err)
	}
	return b.Status
}

func assertKeptForRefund(t *testing.T, got domain.Payment) {
	t.Helper()
	if got.Status != domain.PaymentVerified || got.VerifiedAt == nil {
		t.Errorf("status = %q, verified_at = %v, want the verified payment kept", got.Status, got.VerifiedAt)
	}
	if string(got.RawResponse) == "" || got.ProviderRef == "" {
		t.Errorf("raw = %q, ref = %q, want the gateway's evidence kept", got.RawResponse, got.ProviderRef)
	}
	if !got.RefundOwed() || got.RefundReason == "" {
		t.Errorf("refund_due_at = %v, reason = %q, want it flagged for a refund", got.RefundDueAt, got.RefundReason)
	}
}

func TestCompleteOnAPendingHoldConfirmsIt(t *testing.T) {
	f := newFixture(t, 1)
	id := f.bookFor(t, f.players[0], 48*time.Hour, "pending")
	p := f.onlineIntent(t, id, domain.ProviderKhalti)

	got, err := NewPaymentRepo(f.pool, f.repo).Settle(context.Background(), complete(t, p))
	if err != nil {
		t.Fatalf("settling: %v", err)
	}
	if got.Status != domain.PaymentVerified || got.RefundDueAt != nil {
		t.Errorf("payment = %+v, want verified with no refund", got)
	}
	if s := f.bookingStatus(t, id); s != domain.BookingConfirmed {
		t.Errorf("booking = %q, want confirmed", s)
	}
}

// The player cancelled while on the gateway page. Cancelling fails the
// intent; the gateway then says COMPLETE. Before, that answer was dropped.
func TestCompleteAfterThePlayerCancelledIsKeptAndFlagged(t *testing.T) {
	f := newFixture(t, 1)
	ctx := context.Background()
	player := f.players[0]
	id := f.bookFor(t, player, 48*time.Hour, "pending")
	p := f.onlineIntent(t, id, domain.ProviderEsewa)

	if _, err := f.repo.Cancel(ctx, id, player, "Plans changed"); err != nil {
		t.Fatalf("cancelling: %v", err)
	}
	p.Status = domain.PaymentFailed // as cancelling left it

	got, err := NewPaymentRepo(f.pool, f.repo).Settle(ctx, complete(t, p))
	if err != nil {
		t.Fatalf("settling must not fail: %v", err)
	}
	assertKeptForRefund(t, got)
	if s := f.bookingStatus(t, id); s != domain.BookingCancelled {
		t.Errorf("booking = %q, want it left cancelled", s)
	}

	// The player's list says a refund is owed.
	list, err := f.repo.ListForUser(ctx, player, 10)
	if err != nil || len(list) != 1 || list[0].RefundStatus != "owed" {
		t.Errorf("list = %+v, err = %v, want refund_status owed", list, err)
	}
}

// The hold lapsed while the player was on the gateway, and the janitor
// released it before the callback arrived.
func TestCompleteAfterTheHoldLapsedIsKeptAndFlagged(t *testing.T) {
	f := newFixture(t, 1)
	ctx := context.Background()
	id := f.bookFor(t, f.players[0], 48*time.Hour, "pending")
	p := f.onlineIntent(t, id, domain.ProviderKhalti)

	if _, err := f.pool.Exec(ctx,
		`update bookings set hold_expires_at = now() - interval '1 minute' where id = $1`, id); err != nil {
		t.Fatalf("expiring hold: %v", err)
	}
	if _, err := f.repo.ReleaseStaleHolds(ctx); err != nil {
		t.Fatalf("janitor: %v", err)
	}
	p.Status = domain.PaymentFailed // as the janitor left it

	got, err := NewPaymentRepo(f.pool, f.repo).Settle(ctx, complete(t, p))
	if err != nil {
		t.Fatalf("settling must not fail: %v", err)
	}
	assertKeptForRefund(t, got)
}

// Paid twice: Khalti, then eSewa as well. The second is kept, and flagged.
func TestASecondCompletePaymentIsKeptAndFlagged(t *testing.T) {
	f := newFixture(t, 1)
	ctx := context.Background()
	repo := NewPaymentRepo(f.pool, f.repo)
	id := f.bookFor(t, f.players[0], 48*time.Hour, "pending")

	first := f.onlineIntent(t, id, domain.ProviderKhalti)
	second := f.onlineIntent(t, id, domain.ProviderEsewa)

	if _, err := repo.Settle(ctx, complete(t, first)); err != nil {
		t.Fatalf("first: %v", err)
	}
	got, err := repo.Settle(ctx, complete(t, second))
	if err != nil {
		t.Fatalf("second must not fail: %v", err)
	}
	assertKeptForRefund(t, got)
	if s := f.bookingStatus(t, id); s != domain.BookingConfirmed {
		t.Errorf("booking = %q, want confirmed by the first", s)
	}
}

// Booked to pay cash, then paid online after all. The online payment stands;
// the venue must not also collect the cash.
func TestCompleteOnACashBookingClosesTheCashRequest(t *testing.T) {
	f := newFixture(t, 1)
	ctx := context.Background()
	repo := NewPaymentRepo(f.pool, f.repo)

	hold, err := f.repo.CreateHold(ctx, f.hold(t, f.players[0], f.slotAt(t, 19, time.Hour)))
	if err != nil {
		t.Fatalf("hold: %v", err)
	}
	if _, err := repo.BookCash(ctx, cashIntent(hold)); err != nil {
		t.Fatalf("booking cash: %v", err)
	}
	p := f.onlineIntent(t, hold.ID, domain.ProviderKhalti)

	got, err := repo.Settle(ctx, complete(t, p))
	if err != nil || got.RefundDueAt != nil {
		t.Fatalf("settle = %+v, err = %v, want verified with no refund", got, err)
	}

	var cashStatus string
	_ = f.pool.QueryRow(ctx,
		`select status from payments where booking_id = $1 and provider = 'cash'`, hold.ID).Scan(&cashStatus)
	if cashStatus != "failed" {
		t.Errorf("cash request = %q, want it closed", cashStatus)
	}
}

// A replay of a COMPLETE changes nothing.
func TestSettleIsIdempotentForAKeptRefund(t *testing.T) {
	f := newFixture(t, 1)
	ctx := context.Background()
	repo := NewPaymentRepo(f.pool, f.repo)
	id := f.bookFor(t, f.players[0], 48*time.Hour, "pending")
	p := f.onlineIntent(t, id, domain.ProviderKhalti)
	if _, err := f.repo.Cancel(ctx, id, f.players[0], "Plans changed"); err != nil {
		t.Fatalf("cancelling: %v", err)
	}

	first, err := repo.Settle(ctx, complete(t, p))
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	again, err := repo.Settle(ctx, complete(t, p))
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if !again.RefundDueAt.Equal(*first.RefundDueAt) {
		t.Errorf("a replay moved refund_due_at from %v to %v", first.RefundDueAt, again.RefundDueAt)
	}
}

func TestOnlyTheVenueCanMarkARefundSent(t *testing.T) {
	f := newFixture(t, 2)
	ctx := context.Background()
	repo := NewPaymentRepo(f.pool, f.repo)
	id := f.bookFor(t, f.players[0], 48*time.Hour, "pending")
	p := f.onlineIntent(t, id, domain.ProviderKhalti)
	if _, err := f.repo.Cancel(ctx, id, f.players[0], "Plans changed"); err != nil {
		t.Fatalf("cancelling: %v", err)
	}
	kept, err := repo.Settle(ctx, complete(t, p))
	if err != nil {
		t.Fatalf("settling: %v", err)
	}

	// It shows on the venue's payments list.
	list, err := repo.ListForArenaOwner(ctx, f.arenaID, f.owner, 10)
	if err != nil || len(list) == 0 || !list[0].RefundOwed() {
		t.Fatalf("owner list = %+v, err = %v, want the refund owed", list, err)
	}

	if _, err := repo.MarkRefundedOwnerScoped(ctx, kept.ID, f.players[1]); domain.CodeOf(err) != domain.CodeNotFound {
		t.Errorf("stranger: code = %q, want not_found", domain.CodeOf(err))
	}
	done, err := repo.MarkRefundedOwnerScoped(ctx, kept.ID, f.owner)
	if err != nil || done.RefundedAt == nil || done.RefundOwed() {
		t.Fatalf("mark refunded = %+v, err = %v", done, err)
	}
	if _, err := repo.MarkRefundedOwnerScoped(ctx, kept.ID, f.owner); domain.CodeOf(err) != domain.CodeNotFound {
		t.Errorf("second time: code = %q, want not_found", domain.CodeOf(err))
	}

	players, _ := f.repo.ListForUser(ctx, f.players[0], 10)
	if len(players) != 1 || players[0].RefundStatus != "refunded" {
		t.Errorf("player's list = %+v, want refund_status refunded", players)
	}
}
