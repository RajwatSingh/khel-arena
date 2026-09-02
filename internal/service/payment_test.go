package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/platform/payment"
)

// These cover the path a gateway callback takes, which is the one
// attacker-reachable surface in this service. The properties worth pinning are
// negative: an underpayment is refused, a replay changes nothing, a callback
// naming the wrong provider is refused, and nothing a caller passes can mark a
// booking paid.

type stubGateway struct {
	provider domain.PaymentProvider
	result   domain.GatewayResult
	err      error
	calls    int
}

func (g *stubGateway) Provider() domain.PaymentProvider { return g.provider }

func (g *stubGateway) Checkout(context.Context, domain.Payment, payment.ReturnURLs) (payment.Checkout, error) {
	return payment.Checkout{Method: "GET", URL: "https://gateway.test/pay"}, nil
}

func (g *stubGateway) Verify(context.Context, domain.Payment, payment.CallbackRef) (domain.GatewayResult, error) {
	g.calls++
	return g.result, g.err
}

type stubPayments struct {
	payment   domain.Payment
	settled   domain.Payment
	confirm   bool
	settles   int
	loadErr   error
	createErr error
}

func (s *stubPayments) Create(_ context.Context, p domain.Payment) (domain.Payment, error) {
	if s.createErr != nil {
		return domain.Payment{}, s.createErr
	}
	p.ID = uuid.New()
	s.payment = p
	return p, nil
}

func (s *stubPayments) ByTransactionUUID(context.Context, string) (domain.Payment, error) {
	if s.loadErr != nil {
		return domain.Payment{}, s.loadErr
	}
	return s.payment, nil
}

func (s *stubPayments) LatestForBooking(context.Context, uuid.UUID) (domain.Payment, error) {
	return s.payment, s.loadErr
}

func (s *stubPayments) Settle(_ context.Context, p domain.Payment, confirmBooking bool) error {
	s.settles++
	s.settled = p
	s.confirm = confirmBooking
	return nil
}

type stubBookings struct {
	booking domain.Booking
	err     error
}

func (s stubBookings) ByID(context.Context, uuid.UUID) (domain.Booking, error) {
	return s.booking, s.err
}

var (
	testOwner    = uuid.MustParse("11111111-1111-4111-8111-111111111111")
	testStranger = uuid.MustParse("22222222-2222-4222-8222-222222222222")
)

func heldBooking() domain.Booking {
	expires := time.Now().Add(15 * time.Minute)
	return domain.Booking{
		ID:            uuid.New(),
		UserID:        testOwner,
		PriceNPR:      1800,
		Status:        domain.BookingPending,
		HoldExpiresAt: &expires,
	}
}

func newPaymentService(gw *stubGateway, payments *stubPayments, bookings stubBookings) *PaymentService {
	return NewPaymentService(payments, bookings,
		payment.Registry{gw.provider: gw},
		func(domain.PaymentProvider) payment.ReturnURLs { return payment.ReturnURLs{} },
		SystemClock{})
}

func startedPayment(t *testing.T, gw *stubGateway, booking domain.Booking) (*PaymentService, *stubPayments) {
	t.Helper()

	payments := &stubPayments{}
	svc := newPaymentService(gw, payments, stubBookings{booking: booking})

	if _, _, err := svc.Checkout(context.Background(), booking.ID, booking.UserID, gw.provider); err != nil {
		t.Fatalf("starting checkout: %v", err)
	}
	return svc, payments
}

func TestCheckoutPricesFromTheBooking(t *testing.T) {
	gw := &stubGateway{provider: domain.ProviderEsewa}
	booking := heldBooking()

	_, payments := startedPayment(t, gw, booking)

	// The client named no amount and could not have: the intent takes it from
	// the booking, which took it from the pricing rules server-side.
	if payments.payment.AmountNPR != booking.PriceNPR {
		t.Errorf("amount = %d, want the booking's %d", payments.payment.AmountNPR, booking.PriceNPR)
	}
	if payments.payment.Status != domain.PaymentInitiated {
		t.Errorf("status = %q, want initiated", payments.payment.Status)
	}
	if payments.payment.TransactionUUID == "" {
		t.Error("no transaction id was minted")
	}
}

// Paying for somebody else's hour must not be possible, and must not reveal
// that the hour exists.
func TestCheckoutRefusesSomebodyElsesBooking(t *testing.T) {
	gw := &stubGateway{provider: domain.ProviderEsewa}
	payments := &stubPayments{}
	svc := newPaymentService(gw, payments, stubBookings{booking: heldBooking()})

	_, _, err := svc.Checkout(context.Background(), uuid.New(), testStranger, domain.ProviderEsewa)

	if domain.CodeOf(err) != domain.CodeNotFound {
		t.Errorf("code = %q, want not_found (not forbidden, which would confirm it exists)", domain.CodeOf(err))
	}
	if payments.payment.ID != uuid.Nil {
		t.Error("a payment was created for a booking the caller does not own")
	}
}

func TestCheckoutRefusesAnUnconfiguredProvider(t *testing.T) {
	gw := &stubGateway{provider: domain.ProviderEsewa}
	svc := newPaymentService(gw, &stubPayments{}, stubBookings{booking: heldBooking()})

	_, _, err := svc.Checkout(context.Background(), uuid.New(), testOwner, domain.ProviderKhalti)

	if domain.CodeOf(err) != domain.CodeInvalid {
		t.Errorf("code = %q, want invalid", domain.CodeOf(err))
	}
}

func TestSettleConfirmsAVerifiedPayment(t *testing.T) {
	booking := heldBooking()
	gw := &stubGateway{provider: domain.ProviderEsewa}
	svc, payments := startedPayment(t, gw, booking)

	gw.result = domain.GatewayResult{
		Verified:        true,
		TransactionUUID: payments.payment.TransactionUUID,
		ProviderRef:     "esewa-ref-1",
		AmountNPR:       booking.PriceNPR,
	}

	settled, err := svc.Settle(context.Background(), domain.ProviderEsewa,
		payment.CallbackRef{TransactionUUID: payments.payment.TransactionUUID})
	if err != nil {
		t.Fatalf("settling: %v", err)
	}

	if settled.Status != domain.PaymentVerified {
		t.Errorf("status = %q, want verified", settled.Status)
	}
	if !payments.confirm {
		t.Error("the booking was not confirmed alongside a verified payment")
	}
	if settled.VerifiedAt == nil {
		t.Error("no verified_at was stamped")
	}
}

// The attack this exists for: pay NPR 1 for an NPR 1,800 court and present a
// genuine, correctly signed confirmation.
func TestSettleRefusesAnUnderpayment(t *testing.T) {
	booking := heldBooking()
	gw := &stubGateway{provider: domain.ProviderEsewa}
	svc, payments := startedPayment(t, gw, booking)

	gw.result = domain.GatewayResult{
		Verified:        true,
		TransactionUUID: payments.payment.TransactionUUID,
		ProviderRef:     "esewa-ref-1",
		AmountNPR:       1, // genuine confirmation, wrong amount
	}

	_, err := svc.Settle(context.Background(), domain.ProviderEsewa,
		payment.CallbackRef{TransactionUUID: payments.payment.TransactionUUID})

	if domain.CodeOf(err) != domain.CodeConflict {
		t.Fatalf("code = %q, want conflict", domain.CodeOf(err))
	}
	if payments.confirm {
		t.Fatal("the booking was confirmed against an underpayment")
	}
	// The evidence has to survive: this is either a misconfigured price or an
	// attempt to underpay, and both need a human to look.
	if payments.settled.Status != domain.PaymentFailed {
		t.Errorf("status = %q, want failed and recorded", payments.settled.Status)
	}
	if payments.settles != 1 {
		t.Errorf("settles = %d, want the outcome written exactly once", payments.settles)
	}
}

func TestSettleRecordsAFailedPayment(t *testing.T) {
	booking := heldBooking()
	gw := &stubGateway{provider: domain.ProviderEsewa}
	svc, payments := startedPayment(t, gw, booking)

	gw.result = domain.GatewayResult{Verified: false, ProviderRef: "esewa-ref-1"}

	settled, err := svc.Settle(context.Background(), domain.ProviderEsewa,
		payment.CallbackRef{TransactionUUID: payments.payment.TransactionUUID})
	if err != nil {
		t.Fatalf("settling: %v", err)
	}

	if settled.Status != domain.PaymentFailed {
		t.Errorf("status = %q, want failed", settled.Status)
	}
	if payments.confirm {
		t.Error("the booking was confirmed on an unverified payment")
	}
}

// Gateways retry and players refresh the redirect. Neither may reach the
// gateway a second time or change anything.
func TestSettleIsIdempotent(t *testing.T) {
	booking := heldBooking()
	gw := &stubGateway{provider: domain.ProviderEsewa}
	svc, payments := startedPayment(t, gw, booking)

	gw.result = domain.GatewayResult{
		Verified:        true,
		TransactionUUID: payments.payment.TransactionUUID,
		AmountNPR:       booking.PriceNPR,
	}

	ref := payment.CallbackRef{TransactionUUID: payments.payment.TransactionUUID}
	if _, err := svc.Settle(context.Background(), domain.ProviderEsewa, ref); err != nil {
		t.Fatalf("first settle: %v", err)
	}

	// The stored payment is now settled; a replay must stop at that check.
	payments.payment = payments.settled

	if _, err := svc.Settle(context.Background(), domain.ProviderEsewa, ref); err != nil {
		t.Fatalf("replayed settle: %v", err)
	}

	if gw.calls != 1 {
		t.Errorf("gateway asked %d times, want 1 -- a replay re-queried the gateway", gw.calls)
	}
	if payments.settles != 1 {
		t.Errorf("settles = %d, want 1 -- a replay wrote again", payments.settles)
	}
}

// A callback claiming to be from a provider this payment was never started
// with.
func TestSettleRefusesAProviderMismatch(t *testing.T) {
	booking := heldBooking()
	esewa := &stubGateway{provider: domain.ProviderEsewa}
	khalti := &stubGateway{provider: domain.ProviderKhalti}

	payments := &stubPayments{}
	svc := NewPaymentService(payments, stubBookings{booking: booking},
		payment.Registry{domain.ProviderEsewa: esewa, domain.ProviderKhalti: khalti},
		func(domain.PaymentProvider) payment.ReturnURLs { return payment.ReturnURLs{} },
		SystemClock{})

	if _, _, err := svc.Checkout(context.Background(), booking.ID, booking.UserID, domain.ProviderEsewa); err != nil {
		t.Fatalf("starting checkout: %v", err)
	}

	_, err := svc.Settle(context.Background(), domain.ProviderKhalti,
		payment.CallbackRef{TransactionUUID: payments.payment.TransactionUUID})

	if domain.CodeOf(err) != domain.CodeInvalid {
		t.Errorf("code = %q, want invalid", domain.CodeOf(err))
	}
	if payments.settles != 0 {
		t.Error("a provider mismatch still wrote an outcome")
	}
}

func TestSettleRefusesACallbackNamingNoTransaction(t *testing.T) {
	gw := &stubGateway{provider: domain.ProviderEsewa}
	svc := newPaymentService(gw, &stubPayments{}, stubBookings{booking: heldBooking()})

	_, err := svc.Settle(context.Background(), domain.ProviderEsewa, payment.CallbackRef{})

	if domain.CodeOf(err) != domain.CodeInvalid {
		t.Errorf("code = %q, want invalid", domain.CodeOf(err))
	}
	if gw.calls != 0 {
		t.Error("the gateway was asked about a transaction that was never named")
	}
}

// A gateway that is down must not fail the payment: nothing is known yet, and
// writing "failed" would lose a payment that may well have succeeded.
func TestSettleLeavesThePaymentAloneWhenTheGatewayIsUnreachable(t *testing.T) {
	booking := heldBooking()
	gw := &stubGateway{provider: domain.ProviderEsewa}
	svc, payments := startedPayment(t, gw, booking)

	gw.err = domain.Unavailable("eSewa isn't responding.")

	_, err := svc.Settle(context.Background(), domain.ProviderEsewa,
		payment.CallbackRef{TransactionUUID: payments.payment.TransactionUUID})

	if domain.CodeOf(err) != domain.CodeUnavailable {
		t.Errorf("code = %q, want unavailable", domain.CodeOf(err))
	}
	if payments.settles != 0 {
		t.Error("an unreachable gateway settled the payment anyway")
	}
}

func TestStatusRefusesSomebodyElsesBooking(t *testing.T) {
	gw := &stubGateway{provider: domain.ProviderEsewa}
	svc := newPaymentService(gw, &stubPayments{}, stubBookings{booking: heldBooking()})

	_, err := svc.Status(context.Background(), uuid.New(), testStranger)

	if domain.CodeOf(err) != domain.CodeNotFound {
		t.Errorf("code = %q, want not_found", domain.CodeOf(err))
	}
}

func TestCheckoutRequiresASession(t *testing.T) {
	gw := &stubGateway{provider: domain.ProviderEsewa}
	svc := newPaymentService(gw, &stubPayments{}, stubBookings{booking: heldBooking()})

	_, _, err := svc.Checkout(context.Background(), uuid.New(), uuid.Nil, domain.ProviderEsewa)

	if domain.CodeOf(err) != domain.CodeUnauthenticated {
		t.Errorf("code = %q, want unauthenticated", domain.CodeOf(err))
	}
}

func TestCheckoutRefusesABookingThatCannotBePaid(t *testing.T) {
	cancelled := heldBooking()
	cancelled.Status = domain.BookingCancelled

	gw := &stubGateway{provider: domain.ProviderEsewa}
	payments := &stubPayments{}
	svc := newPaymentService(gw, payments, stubBookings{booking: cancelled})

	_, _, err := svc.Checkout(context.Background(), cancelled.ID, testOwner, domain.ProviderEsewa)

	if domain.CodeOf(err) != domain.CodeConflict {
		t.Errorf("code = %q, want conflict", domain.CodeOf(err))
	}
	if !errors.Is(err, domain.ErrConflict) && domain.CodeOf(err) != domain.CodeConflict {
		t.Error("expected a conflict")
	}
}
