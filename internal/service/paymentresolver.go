package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/platform/payment"
)

// PaymentAccountReader is the credential storage the resolver reads.
type PaymentAccountReader interface {
	// EnabledForBooking returns the accounts a booking's arena is taking
	// payment through right now.
	EnabledForBooking(ctx context.Context, bookingID uuid.UUID) ([]domain.ArenaPaymentAccount, error)
	// ForPayment returns the account a started payment must be settled
	// through, enabled or not.
	ForPayment(ctx context.Context, paymentID uuid.UUID, provider domain.PaymentProvider) (domain.ArenaPaymentAccount, error)
}

// GatewayResolver picks the payment adapter for a booking or a started
// payment.
//
// Cash is one adapter for the whole deployment. An online provider is the
// venue's own merchant account, built per booking from the credentials its
// owner stored — there is no platform-wide eSewa or Khalti key. This is the
// seam where "whose money is it" is decided.
type GatewayResolver interface {
	// ForCheckout resolves the gateway a booking should be paid through.
	ForCheckout(ctx context.Context, bookingID uuid.UUID, provider domain.PaymentProvider) (payment.Gateway, error)
	// ForSettlement resolves the gateway a started payment must be verified
	// through.
	ForSettlement(ctx context.Context, p domain.Payment) (payment.Gateway, error)
}

type gatewayResolver struct {
	// accounts is nil when PAYMENT_ENC_KEY is unset: there is nothing to
	// decrypt stored secrets with, so online payments are simply off.
	accounts   PaymentAccountReader
	cash       payment.Gateway
	websiteURL string
}

// NewGatewayResolver builds the resolver. Pass a nil accounts reader to run
// cash-only.
func NewGatewayResolver(accounts PaymentAccountReader, websiteURL string) GatewayResolver {
	return &gatewayResolver{
		accounts:   accounts,
		cash:       payment.NewCash(),
		websiteURL: websiteURL,
	}
}

func (r *gatewayResolver) ForCheckout(ctx context.Context, bookingID uuid.UUID, provider domain.PaymentProvider) (payment.Gateway, error) {
	if provider == domain.ProviderCash {
		return r.cash, nil
	}
	if r.accounts == nil {
		return nil, domain.Invalid("provider", "This deployment only takes cash right now.")
	}

	accounts, err := r.accounts.EnabledForBooking(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	for _, acct := range accounts {
		if acct.Provider == provider {
			return payment.FromAccount(acct, r.websiteURL)
		}
	}
	return nil, domain.Invalid("provider", "This venue isn't set up to take %s.", provider)
}

func (r *gatewayResolver) ForSettlement(ctx context.Context, p domain.Payment) (payment.Gateway, error) {
	if p.Provider == domain.ProviderCash {
		return r.cash, nil
	}
	if r.accounts == nil {
		return nil, domain.Invalid("provider", "This deployment only takes cash right now.")
	}

	acct, err := r.accounts.ForPayment(ctx, p.ID, p.Provider)
	if err != nil {
		return nil, err
	}
	return payment.FromAccount(acct, r.websiteURL)
}
