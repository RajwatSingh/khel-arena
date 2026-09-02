package payment

import (
	"context"
	"net/url"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

// Cash is settlement at the arena, not a gateway.
//
// It exists because it is what most Kathmandu futsal actually runs on, and
// because `domain.ProviderCash` is already in the enum with
// `IsOnline() == false`. Modelling it as a Gateway keeps one payment path in
// the service rather than a special case threaded through it.
//
// Checkout succeeds and sends the player nowhere: an intent is recorded so the
// venue has something to reconcile against, and the empty Method says there is
// no gateway to visit.
//
// Verify always refuses. A cash payment becomes real when the arena says the
// notes changed hands -- `PaymentRepo.MarkCashReceivedOwnerScoped` -- and
// never on the strength of a link the player followed.
//
// That refusal is the point. The tempting implementation returns
// `Verified: true` and confirms the hour on the player's word, which is the
// same as making every booking free.
type Cash struct{}

func NewCash() *Cash { return &Cash{} }

func (Cash) Provider() domain.PaymentProvider { return domain.ProviderCash }

// Checkout records the intent and hands back nowhere to go. An empty Method
// is the signal: there is no gateway in this flow, only an arrangement with
// the venue.
func (Cash) Checkout(context.Context, domain.Payment, ReturnURLs) (Checkout, error) {
	return Checkout{}, nil
}

func (Cash) Verify(context.Context, domain.Payment, CallbackRef) (domain.GatewayResult, error) {
	return domain.GatewayResult{}, domain.Forbidden(
		"A cash payment is confirmed by the arena, not by this link.")
}

// RefFromCallback parses a provider's redirect into the reference used to ask
// that provider what happened.
//
// Everything it returns is untrusted by construction -- it came through the
// player's browser -- which is why the only field any adapter reads from it is
// an identifier.
func RefFromCallback(provider domain.PaymentProvider, values url.Values) (CallbackRef, error) {
	switch provider {
	case domain.ProviderEsewa:
		return esewaRefFromCallback(values)
	case domain.ProviderKhalti:
		return khaltiRefFromCallback(values)
	default:
		return CallbackRef{}, domain.Invalid("provider", "%s doesn't send payments back here.", provider)
	}
}
