// Package payment holds the gateway adapters.
//
// # The one rule this package exists to enforce
//
// A redirect back from a gateway is not evidence of payment. It arrives in the
// player's browser, over a URL the player can edit, and any signature on it is
// verified against parameters the player also controls. Every adapter here
// therefore treats the redirect as nothing more than a hint about *which*
// transaction to ask about, and then asks the gateway directly, server to
// server, over a connection the player is not part of. `Verified` is set from
// that answer and from nothing else.
//
// This is why `domain.GatewayResult` carries an amount as well as a flag: the
// service compares what the gateway says was paid against what the booking
// owed (`domain.Payment.Verify`). A correctly signed confirmation for the
// wrong amount is refused. Between "never trust the redirect" and "always
// check the amount", an adapter would have to be extraordinarily wrong before
// a free booking came out of it.
//
// # Money
//
// Prices are integer NPR everywhere in this system. Khalti speaks paisa --
// one hundredth of a rupee -- so its adapter converts at the boundary and
// nowhere else. Getting that wrong in the forgiving direction charges a player
// a hundred times too much; in the other, it accepts one percent of the price.
package payment

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

// Gateway is what a provider adapter has to be able to do.
//
// Two methods, because a payment has two moments: sending the player to the
// provider, and finding out what happened. Nothing in the interface lets a
// caller assert that a payment succeeded -- only ask.
type Gateway interface {
	// Provider names which gateway this is.
	Provider() domain.PaymentProvider

	// Checkout describes where to send the player and what to send with them.
	Checkout(ctx context.Context, p domain.Payment, ret ReturnURLs) (Checkout, error)

	// Verify asks the gateway, server to server, what became of a payment.
	// The reference comes from the redirect and is untrusted; everything in
	// the result comes from the gateway's own answer.
	Verify(ctx context.Context, p domain.Payment, ref CallbackRef) (domain.GatewayResult, error)
}

// ReturnURLs are where the gateway should send the player afterwards.
type ReturnURLs struct {
	Success string
	Failure string
}

// Checkout is how to hand the player over.
//
// Providers differ in shape: Khalti returns a URL to redirect to, eSewa wants
// a form POSTed to it with signed fields. Both are expressed here so the
// transport layer can hand either to a browser without knowing which provider
// it is talking about.
type Checkout struct {
	// Method is "GET" for a plain redirect or "POST" for a form submission.
	Method string
	// URL is where the player goes.
	URL string
	// Fields are form fields to submit, for POST checkouts. Empty for GET.
	Fields map[string]string
}

// CallbackRef is what the redirect told us, all of it untrusted. It exists to
// identify the transaction to ask about, not to describe its outcome.
type CallbackRef struct {
	// TransactionUUID is ours, echoed back by the gateway.
	TransactionUUID string
	// ProviderRef is the gateway's own identifier (Khalti's pidx). Some
	// providers need it to answer a status query.
	ProviderRef string
	// Raw is the whole callback, kept for the audit trail.
	Raw []byte
}

// httpClient is the client every adapter uses.
//
// The timeout is not optional. A gateway that accepts a connection and then
// never answers would otherwise hold a request goroutine -- and the player
// staring at a spinner -- until something else gave up first.
func httpClient() *http.Client {
	return &http.Client{Timeout: 15 * time.Second}
}

// badGateway wraps a transport-level failure talking to a provider.
//
// Deliberately CodeUnavailable rather than CodeInternal: the gateway being
// unreachable is not a defect in this service, and the player should be told
// to try again rather than shown an apology that suggests their booking is
// broken.
func badGateway(provider domain.PaymentProvider, err error, format string, args ...any) error {
	return domain.Unavailable("%s isn't responding. Your booking is still held -- try again in a moment.",
		provider).WithCause(fmt.Errorf(format+": %w", append(args, err)...))
}
