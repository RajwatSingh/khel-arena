package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// ArenaPaymentAccount is one venue's own merchant credentials for one gateway.
//
// The gateway adapter for a booking is built from the row belonging to that
// booking's arena, so the money settles into the venue's account and never
// touches a platform one. SecretKey is plaintext here — it is decrypted on
// read and only ever lives in memory; the stored column is ciphertext and the
// owner-facing API returns nothing but a hint.
type ArenaPaymentAccount struct {
	ArenaID      uuid.UUID
	Provider     PaymentProvider
	SecretKey    string
	MerchantCode string // eSewa product code; unused by Khalti
	Live         bool
	Enabled      bool
}

// onlinePaymentProviders is the subset an arena can hold an account for. Cash
// is settled with the venue and needs no credentials.
var onlinePaymentProviders = []PaymentProvider{ProviderEsewa, ProviderKhalti}

// Validate checks a submitted account before it is stored.
func (a *ArenaPaymentAccount) Validate() error {
	a.SecretKey = strings.TrimSpace(a.SecretKey)
	a.MerchantCode = strings.TrimSpace(a.MerchantCode)

	v := &Validation{}
	v.Check(a.Provider.CanBePerArena(), "provider",
		"Only eSewa and Khalti are set up per venue.")
	v.Check(a.SecretKey != "", "secret_key", "The gateway secret key is required.")
	if a.Provider == ProviderEsewa {
		v.Check(a.MerchantCode != "", "merchant_code",
			"eSewa needs the merchant (product) code it issued you.")
	}
	return v.Err()
}

// CanBePerArena reports whether a provider is one a venue holds its own
// account for.
func (p PaymentProvider) CanBePerArena() bool {
	for _, allowed := range onlinePaymentProviders {
		if p == allowed {
			return true
		}
	}
	return false
}

// ArenaPaymentAccountInfo is the safe-to-return view: what an owner needs to
// see the state of an account without the secret leaving the server.
type ArenaPaymentAccountInfo struct {
	Provider     PaymentProvider `json:"provider"`
	MerchantCode string          `json:"merchant_code"`
	Live         bool            `json:"live"`
	Enabled      bool            `json:"enabled"`
	// SecretHint is the last four characters of the stored key, so an owner
	// can tell which key is in place without it being shown.
	SecretHint string    `json:"secret_hint"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// HintFor returns the last four characters of a secret, for SecretHint.
func HintFor(secret string) string {
	r := []rune(secret)
	if len(r) <= 4 {
		return strings.Repeat("•", len(r))
	}
	return "…" + string(r[len(r)-4:])
}
