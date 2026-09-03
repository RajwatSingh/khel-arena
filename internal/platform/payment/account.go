package payment

import (
	"github.com/RajwatSingh/khel-arena/internal/domain"
)

// FromAccount builds the gateway adapter for one venue's stored credentials.
//
// This is what replaced the global registry for online providers: instead of
// one eSewa adapter per deployment, there is one per arena, constructed here
// from the row that arena's owner filled in. The sandbox/production hosts are
// chosen from acct.Live using the constants each adapter already publishes, so
// a test account talks to the test host without a second config field.
//
// websiteURL is the merchant site shown to a Khalti payer; it is the
// interface's own origin, the same value for every arena.
func FromAccount(acct domain.ArenaPaymentAccount, websiteURL string) (Gateway, error) {
	switch acct.Provider {
	case domain.ProviderEsewa:
		formURL, statusURL := EsewaFormURLSandbox, EsewaStatusURLSandbox
		if acct.Live {
			formURL, statusURL = EsewaFormURLLive, EsewaStatusURLLive
		}
		return NewEsewa([]byte(acct.SecretKey), acct.MerchantCode, formURL, statusURL), nil

	case domain.ProviderKhalti:
		base := KhaltiBaseURLSandbox
		if acct.Live {
			base = KhaltiBaseURLLive
		}
		return NewKhalti(acct.SecretKey, base, websiteURL), nil

	default:
		return nil, domain.Invalid("provider",
			"%s can't be configured per venue.", acct.Provider)
	}
}
