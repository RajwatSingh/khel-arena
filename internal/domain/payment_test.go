package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func initiatedPayment(amount int) Payment {
	return Payment{
		ID:              uuid.New(),
		BookingID:       uuid.New(),
		Provider:        ProviderEsewa,
		AmountNPR:       amount,
		Status:          PaymentInitiated,
		TransactionUUID: "txn-abc-123",
	}
}

func TestPaymentVerifyConfirmsAMatchingPayment(t *testing.T) {
	now := time.Now().UTC()
	p := initiatedPayment(2000)

	confirm, err := p.Verify(GatewayResult{
		Verified:        true,
		TransactionUUID: "txn-abc-123",
		ProviderRef:     "esewa-ref-9",
		AmountNPR:       2000,
	}, now)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !confirm {
		t.Error("a verified, correctly-priced payment should confirm the booking")
	}
	if p.Status != PaymentVerified {
		t.Errorf("status = %q, want %q", p.Status, PaymentVerified)
	}
	if p.VerifiedAt == nil || !p.VerifiedAt.Equal(now) {
		t.Error("a verified payment should be stamped with the time it settled")
	}
	if p.ProviderRef != "esewa-ref-9" {
		t.Errorf("provider ref = %q, want the gateway's reference", p.ProviderRef)
	}
}

// The whole reason the amount check is a domain rule. A gateway callback is
// reachable by anyone: without this, a player pays NPR 1 for an NPR 2,000
// court and hands us a genuine, correctly-signed confirmation of it.
func TestPaymentVerifyRejectsAnUnderpayment(t *testing.T) {
	p := initiatedPayment(2000)

	confirm, err := p.Verify(GatewayResult{
		Verified:        true,
		TransactionUUID: "txn-abc-123",
		ProviderRef:     "esewa-ref-9",
		AmountNPR:       1,
	}, time.Now().UTC())

	if confirm {
		t.Fatal("an underpayment must never confirm the booking")
	}
	if err == nil {
		t.Fatal("an underpayment should report an error")
	}
	if !errors.Is(err, ErrConflict) {
		t.Errorf("code = %q, want %q", CodeOf(err), CodeConflict)
	}
	if p.Status != PaymentFailed {
		t.Errorf("status = %q, want %q", p.Status, PaymentFailed)
	}
	if p.VerifiedAt != nil {
		t.Error("a rejected payment must not be stamped as verified")
	}
	// The evidence has to survive: this needs a human to look at it.
	if p.ProviderRef == "" {
		t.Error("the gateway reference should be kept for investigation")
	}
}

func TestPaymentVerifyRejectsAnOverpayment(t *testing.T) {
	p := initiatedPayment(2000)

	confirm, err := p.Verify(GatewayResult{
		Verified: true, TransactionUUID: "txn-abc-123", AmountNPR: 5000,
	}, time.Now().UTC())

	if confirm || err == nil {
		t.Fatal("a mismatched amount must not confirm, even when it is too much")
	}
	if p.Status != PaymentFailed {
		t.Errorf("status = %q, want %q", p.Status, PaymentFailed)
	}
}

func TestPaymentVerifyRecordsAFailedAttempt(t *testing.T) {
	p := initiatedPayment(2000)

	confirm, err := p.Verify(GatewayResult{
		Verified: false, TransactionUUID: "txn-abc-123", AmountNPR: 2000,
	}, time.Now().UTC())

	if err != nil {
		t.Fatalf("a declined payment is not an error condition: %v", err)
	}
	if confirm {
		t.Error("an unverified payment must not confirm the booking")
	}
	if p.Status != PaymentFailed {
		t.Errorf("status = %q, want %q", p.Status, PaymentFailed)
	}
}

// Gateways retry, and a player can refresh the success page. Neither may
// change a payment that has already settled.
func TestPaymentVerifyIsIdempotentOnceSettled(t *testing.T) {
	now := time.Now().UTC()
	good := GatewayResult{Verified: true, TransactionUUID: "txn-abc-123", AmountNPR: 2000}

	p := initiatedPayment(2000)
	if _, err := p.Verify(good, now); err != nil {
		t.Fatalf("first verification: %v", err)
	}
	firstSettled := *p.VerifiedAt

	// A replayed callback, an hour later.
	confirm, err := p.Verify(good, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("replayed callback: %v", err)
	}
	if confirm {
		t.Error("a replay must not re-confirm the booking")
	}
	if !p.VerifiedAt.Equal(firstSettled) {
		t.Error("a replay must not move the settlement time")
	}

	// A settled failure must not be revivable by a later "success" either.
	failed := initiatedPayment(2000)
	failed.Status = PaymentFailed
	if confirm, _ := failed.Verify(good, now); confirm {
		t.Error("a failed payment must not be confirmed by a later callback")
	}
	if failed.Status != PaymentFailed {
		t.Errorf("status = %q, want it to stay %q", failed.Status, PaymentFailed)
	}
}

// A callback carrying someone else's transaction id has either been
// misrouted or forged. Applying it would settle the wrong booking.
func TestPaymentVerifyRejectsAMismatchedTransaction(t *testing.T) {
	p := initiatedPayment(2000)

	confirm, err := p.Verify(GatewayResult{
		Verified: true, TransactionUUID: "txn-someone-else", AmountNPR: 2000,
	}, time.Now().UTC())

	if confirm {
		t.Fatal("a callback for a different transaction must not confirm this booking")
	}
	if err == nil {
		t.Fatal("expected a mismatched transaction id to be reported")
	}
	if p.Status != PaymentInitiated {
		t.Errorf("status = %q, want it untouched at %q", p.Status, PaymentInitiated)
	}
}

func TestNewPaymentIntentTakesItsAmountFromTheBooking(t *testing.T) {
	b := Booking{
		ID:       uuid.New(),
		PriceNPR: 2400,
		Status:   BookingPending,
	}

	p, err := NewPaymentIntent(b, ProviderKhalti)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.AmountNPR != 2400 {
		t.Errorf("amount = %d, want the booking's own price of 2400", p.AmountNPR)
	}
	if p.BookingID != b.ID {
		t.Error("the intent should point at the booking it settles")
	}
	if p.Status != PaymentInitiated {
		t.Errorf("status = %q, want %q", p.Status, PaymentInitiated)
	}
	if p.TransactionUUID == "" {
		t.Error("an intent needs a transaction id to hand the gateway")
	}
}

func TestNewPaymentIntentRefusesFinishedBookings(t *testing.T) {
	for _, status := range []BookingStatus{BookingCancelled, BookingCompleted, BookingNoShow} {
		t.Run(string(status), func(t *testing.T) {
			b := Booking{ID: uuid.New(), PriceNPR: 2000, Status: status}
			if _, err := NewPaymentIntent(b, ProviderEsewa); err == nil {
				t.Errorf("a %s booking should not be payable", status)
			}
		})
	}
}

func TestNewPaymentIntentRejectsAnUnknownProvider(t *testing.T) {
	b := Booking{ID: uuid.New(), PriceNPR: 2000, Status: BookingPending}
	if _, err := NewPaymentIntent(b, PaymentProvider("bitcoin")); err == nil {
		t.Error("an unknown provider should be rejected")
	}
}

func TestPaymentRefundTransitions(t *testing.T) {
	verified := initiatedPayment(2000)
	verified.Status = PaymentVerified
	if err := verified.Refund(); err != nil {
		t.Fatalf("a verified payment should be refundable: %v", err)
	}
	if verified.Status != PaymentRefunded {
		t.Errorf("status = %q, want %q", verified.Status, PaymentRefunded)
	}

	// Refunding twice would pay the player out twice.
	if err := verified.Refund(); err == nil {
		t.Error("a second refund should be refused")
	}

	for _, status := range []PaymentStatus{PaymentInitiated, PaymentFailed} {
		p := initiatedPayment(2000)
		p.Status = status
		if err := p.Refund(); err == nil {
			t.Errorf("a %s payment should not be refundable", status)
		}
	}
}
