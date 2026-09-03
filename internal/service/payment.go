package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/platform/payment"
)

// PaymentStore is the payment storage this service needs.
type PaymentStore interface {
	Create(ctx context.Context, p domain.Payment) (domain.Payment, error)
	ByTransactionUUID(ctx context.Context, transactionUUID string) (domain.Payment, error)
	LatestForBooking(ctx context.Context, bookingID uuid.UUID) (domain.Payment, error)
	// Settle writes the outcome and confirms the booking in one transaction.
	Settle(ctx context.Context, p domain.Payment, confirmBooking bool) error
}

// PaymentBookings is the slice of booking storage this service reads.
type PaymentBookings interface {
	ByID(ctx context.Context, id uuid.UUID) (domain.Booking, error)
}

// PaymentService turns a held booking into a paid one.
//
// The order of operations here is the security story. A payment is created
// against a booking the caller owns, for the amount that booking already
// records; the player is sent to a gateway; and what comes back through their
// browser is used only to decide which payment to ask the gateway about. The
// gateway's own answer is the only thing that confirms anything.
//
// The gateway itself is resolved per booking: an online payment goes through
// the venue's own merchant account, so there is no one place a platform key
// lives. See GatewayResolver.
type PaymentService struct {
	payments   PaymentStore
	bookings   PaymentBookings
	gateways   GatewayResolver
	returnURLs func(domain.PaymentProvider) payment.ReturnURLs
	clock      Clock
}

func NewPaymentService(
	payments PaymentStore,
	bookings PaymentBookings,
	gateways GatewayResolver,
	returnURLs func(domain.PaymentProvider) payment.ReturnURLs,
	clock Clock,
) *PaymentService {
	if clock == nil {
		clock = SystemClock{}
	}
	return &PaymentService{
		payments:   payments,
		bookings:   bookings,
		gateways:   gateways,
		returnURLs: returnURLs,
		clock:      clock,
	}
}

// Checkout starts a payment and describes where to send the player.
//
// Ownership is checked here rather than in the repository, unlike
// cancellation: cancelling is a single UPDATE whose WHERE clause can carry the
// check, but this reads a booking and then writes a different table, so there
// is no one statement to attach it to. The read-then-check is safe because the
// only thing racing it is the janitor releasing an expired hold, and
// `NewPaymentIntent` refuses a booking that is no longer payable.
func (s *PaymentService) Checkout(ctx context.Context, bookingID, userID uuid.UUID, provider domain.PaymentProvider) (payment.Checkout, domain.Payment, error) {
	if userID == uuid.Nil {
		return payment.Checkout{}, domain.Payment{}, domain.Unauthenticated("Sign in to pay for a booking.")
	}

	booking, err := s.bookings.ByID(ctx, bookingID)
	if err != nil {
		return payment.Checkout{}, domain.Payment{}, err
	}
	if !booking.IsHeldBy(userID) {
		// Not found rather than forbidden: whether a booking exists is not
		// something a stranger gets to learn by asking to pay for it.
		return payment.Checkout{}, domain.Payment{}, domain.NotFound("No booking with that reference.")
	}

	// The gateway is the venue's, resolved from the booking. An unconfigured
	// provider fails here, before a payment row exists.
	gateway, err := s.gateways.ForCheckout(ctx, bookingID, provider)
	if err != nil {
		return payment.Checkout{}, domain.Payment{}, err
	}

	// The amount comes from the booking, which got it from the pricing rules
	// when the hold was taken. No client figure reaches this.
	intent, err := domain.NewPaymentIntent(booking, provider)
	if err != nil {
		return payment.Checkout{}, domain.Payment{}, err
	}

	created, err := s.payments.Create(ctx, intent)
	if err != nil {
		return payment.Checkout{}, domain.Payment{}, err
	}

	checkout, err := gateway.Checkout(ctx, created, s.returnURLs(provider))
	if err != nil {
		return payment.Checkout{}, domain.Payment{}, err
	}
	return checkout, created, nil
}

// Settle applies a gateway callback.
//
// Nothing the caller passes decides the outcome. `ref` names a transaction;
// the payment is loaded from our own records by that name; the gateway is
// asked what happened; and `domain.Payment.Verify` decides -- checking the
// amount against what was owed and ignoring a callback for a payment that has
// already settled.
//
// The booking is confirmed in the same transaction as the payment write. See
// PaymentRepo.Settle for why that is not two writes.
func (s *PaymentService) Settle(ctx context.Context, provider domain.PaymentProvider, ref payment.CallbackRef) (domain.Payment, error) {
	if ref.TransactionUUID == "" {
		return domain.Payment{}, domain.Invalid("transaction", "That payment link names no transaction.")
	}

	p, err := s.payments.ByTransactionUUID(ctx, ref.TransactionUUID)
	if err != nil {
		return domain.Payment{}, err
	}
	if p.Provider != provider {
		// A callback claiming to be from a provider this payment was never
		// started with.
		return domain.Payment{}, domain.Invalid("provider", "That payment link doesn't match this payment.")
	}
	if p.Status.IsSettled() {
		// Already done. Gateways retry and players refresh; neither should
		// reach the gateway again or change anything.
		return p, nil
	}

	// The gateway is resolved from the payment's own arena, so verification
	// goes back to the same merchant account the money went to.
	gateway, err := s.gateways.ForSettlement(ctx, p)
	if err != nil {
		return domain.Payment{}, err
	}

	result, err := gateway.Verify(ctx, p, ref)
	if err != nil {
		return domain.Payment{}, err
	}

	confirmBooking, verifyErr := p.Verify(result, s.clock.Now())

	// The outcome is written even when Verify returned an error: a genuine
	// confirmation for the wrong amount is exactly the case where the
	// evidence must survive, and leaving the payment `initiated` would let the
	// same callback be replayed.
	if err := s.payments.Settle(ctx, p, confirmBooking); err != nil {
		return domain.Payment{}, err
	}
	if verifyErr != nil {
		return p, verifyErr
	}
	return p, nil
}

// Status reports the latest payment attempt on a booking the caller owns, for
// a client polling after a redirect.
func (s *PaymentService) Status(ctx context.Context, bookingID, userID uuid.UUID) (domain.Payment, error) {
	if userID == uuid.Nil {
		return domain.Payment{}, domain.Unauthenticated("Sign in to see a payment.")
	}

	booking, err := s.bookings.ByID(ctx, bookingID)
	if err != nil {
		return domain.Payment{}, err
	}
	if !booking.IsHeldBy(userID) {
		return domain.Payment{}, domain.NotFound("No booking with that reference.")
	}
	return s.payments.LatestForBooking(ctx, bookingID)
}
