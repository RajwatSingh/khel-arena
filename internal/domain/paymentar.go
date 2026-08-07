package domain

import (
	"time"

	"github.com/google/uuid"
)

// Payment is one attempt to settle a booking.
//
// TransactionUUID is ours and is what we hand the gateway; ProviderRef is
// whatever the gateway calls its own record (eSewa's ref_id, Khalti's pidx).
// Both are unique in the database, so a replayed callback cannot credit the
// same booking twice.
type Payment struct {
	ID              uuid.UUID
	BookingID       uuid.UUID
	Provider        PaymentProvider
	AmountNPR       int
	Status          PaymentStatus
	TransactionUUID string
	ProviderRef     string
	RawResponse     []byte

	CreatedAt  time.Time
	UpdatedAt  time.Time
	VerifiedAt *time.Time
}

// GatewayResult is what a provider adapter reports back after verifying a
// callback. Adapters live outside this package; this is the shape they must
// reduce their provider's vocabulary to.
type GatewayResult struct {
	// Verified is true only when the gateway confirmed the payment
	// server-to-server. A signed redirect alone is never enough.
	Verified        bool
	TransactionUUID string
	ProviderRef     string
	// AmountNPR is what the gateway says was actually paid, which must be
	// checked against what we asked for.
	AmountNPR int
	Raw       []byte
}

// Verify applies a gateway result to this payment and reports whether the
// booking should now be confirmed.
//
// The amount check is the reason this is a domain rule rather than a line in
// a handler. A gateway callback is attacker-reachable: without comparing what
// was paid against what was owed, someone can pay NPR 1 for an NPR 2,000
// court and present a genuine, correctly signed confirmation.
func (p *Payment) Verify(result GatewayResult, now time.Time) (confirmBooking bool, err error) {
	// A settled payment ignores late or duplicate callbacks. Gateways retry,
	// and a player can refresh the redirect; neither should change anything.
	if p.Status.IsSettled() {
		return false, nil
	}

	if result.TransactionUUID != "" && result.TransactionUUID != p.TransactionUUID {
		return false, Internal(nil,
			"gateway result for transaction %q applied to payment %q",
			result.TransactionUUID, p.TransactionUUID)
	}

	if !result.Verified {
		p.Status = PaymentFailed
		p.RawResponse = result.Raw
		p.ProviderRef = result.ProviderRef
		return false, nil
	}

	if result.AmountNPR != p.AmountNPR {
		// Genuine confirmation, wrong amount. Refuse it and keep the
		// evidence: this is either a misconfigured price or an attempt to
		// underpay, and both need a human.
		p.Status = PaymentFailed
		p.RawResponse = result.Raw
		p.ProviderRef = result.ProviderRef
		return false, Conflict(
			"The amount paid (NPR %d) doesn't match this booking (NPR %d). Nothing has been confirmed.",
			result.AmountNPR, p.AmountNPR)
	}

	p.Status = PaymentVerified
	p.ProviderRef = result.ProviderRef
	p.RawResponse = result.Raw
	p.VerifiedAt = &now
	return true, nil
}

// NewPaymentIntent starts a payment for a booking.
//
// The amount comes from the booking, never from the client: the price was
// resolved server-side when the hold was taken, and that is the only figure
// this service will accept.
func NewPaymentIntent(b Booking, provider PaymentProvider) (Payment, error) {
	if err := provider.Validate(); err != nil {
		return Payment{}, err
	}
	if !b.Status.CanCancel() {
		// Same predicate as cancellation: a booking that can no longer be
		// cancelled is finished, and paying for it makes no sense.
		return Payment{}, Conflict("This booking can no longer be paid for.")
	}
	if b.PriceNPR <= 0 {
		return Payment{}, Conflict("This booking has nothing to pay.")
	}

	return Payment{
		BookingID:       b.ID,
		Provider:        provider,
		AmountNPR:       b.PriceNPR,
		Status:          PaymentInitiated,
		TransactionUUID: uuid.NewString(),
	}, nil
}

// Refund marks a verified payment as refunded.
func (p *Payment) Refund() error {
	switch p.Status {
	case PaymentVerified:
		p.Status = PaymentRefunded
		return nil
	case PaymentRefunded:
		return Conflict("This payment has already been refunded.")
	default:
		return Conflict("Only a completed payment can be refunded.")
	}
}
