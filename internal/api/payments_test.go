package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/platform/payment"
	"github.com/RajwatSingh/khel-arena/internal/service"
	"github.com/google/uuid"
)

var _ PaymentAPI = (*service.PaymentService)(nil)

func esewaCallbackQuery(transactionUUID string) string {
	payload := base64.StdEncoding.EncodeToString([]byte(
		`{"transaction_uuid":"` + transactionUUID + `","transaction_code":"0KL2SD","status":"COMPLETE"}`))
	return "?data=" + url.QueryEscape(payload)
}

func TestHandleArenaPaymentProviders(t *testing.T) {
	arenaID := uuid.New()
	accounts := &fakePaymentAccounts{
		providersForArena: func(_ context.Context, got uuid.UUID) ([]domain.PaymentProvider, error) {
			if got != arenaID {
				t.Errorf("arena = %v, want %v", got, arenaID)
			}
			return []domain.PaymentProvider{domain.ProviderEsewa, domain.ProviderKhalti}, nil
		},
	}

	w := do(newTestServer(t, nil, nil, nil, withPaymentAccounts(accounts)),
		http.MethodGet, "/v1/arenas/"+arenaID.String()+"/payment-providers", "")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var got []string
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if len(got) != 2 || got[0] != "esewa" {
		t.Errorf("providers = %v", got)
	}
}

// With online payments off (no PaymentAccountAPI wired), the endpoint still
// answers — with an empty list, not a 500.
func TestHandleArenaPaymentProvidersEmptyWhenDisabled(t *testing.T) {
	w := do(newTestServer(t, nil, nil, nil),
		http.MethodGet, "/v1/arenas/"+uuid.New().String()+"/payment-providers", "")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if strings.TrimSpace(w.Body.String()) != "[]" {
		t.Errorf("body = %q, want []", w.Body.String())
	}
}

func TestHandleCreateCheckout(t *testing.T) {
	t.Run("starts a payment and says where to send the player", func(t *testing.T) {
		auth := &fakeAuth{authenticate: signedIn(testAccessToken)}
		payments := &fakePayments{
			checkout: func(_ context.Context, bookingID, userID uuid.UUID, provider domain.PaymentProvider) (payment.Checkout, domain.Payment, error) {
				if bookingID != testBookingID {
					t.Errorf("booking = %v", bookingID)
				}
				return payment.Checkout{
						Method: "POST",
						URL:    "https://esewa.test/form",
						Fields: map[string]string{"signature": "abc"},
					}, domain.Payment{
						ID: uuid.New(), Provider: provider, AmountNPR: 1800,
					}, nil
			},
		}

		w := do(newTestServer(t, auth, nil, nil, withPayments(payments)), http.MethodPost,
			"/v1/bookings/"+testBookingID.String()+"/checkout", `{"provider":"esewa"}`, bearer(testAccessToken))

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d (%s), want 201", w.Code, w.Body.String())
		}

		var got checkoutDTO
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("decoding checkout: %v", err)
		}
		if got.Method != "POST" || got.URL != "https://esewa.test/form" {
			t.Errorf("checkout = %+v", got)
		}
		if got.Fields["signature"] != "abc" {
			t.Error("the signed fields did not survive to the client")
		}
		// The caller is the signed-in user, not anything in the body.
		if payments.gotUserID != testUser.ID {
			t.Errorf("user = %v, want the one from the token", payments.gotUserID)
		}
	})

	// The amount is the server's. A body naming one is refused outright rather
	// than ignored, so a client that thinks it can set a price finds out.
	t.Run("a request naming an amount is refused", func(t *testing.T) {
		auth := &fakeAuth{authenticate: signedIn(testAccessToken)}

		w := do(newTestServer(t, auth, nil, nil, withPayments(&fakePayments{})), http.MethodPost,
			"/v1/bookings/"+testBookingID.String()+"/checkout",
			`{"provider":"esewa","amount_npr":1}`, bearer(testAccessToken))

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})

	t.Run("requires a session", func(t *testing.T) {
		auth := &fakeAuth{authenticate: signedIn(testAccessToken)}

		w := do(newTestServer(t, auth, nil, nil, withPayments(&fakePayments{})), http.MethodPost,
			"/v1/bookings/"+testBookingID.String()+"/checkout", `{"provider":"esewa"}`)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", w.Code)
		}
	})

	t.Run("a malformed booking id is refused", func(t *testing.T) {
		auth := &fakeAuth{authenticate: signedIn(testAccessToken)}

		w := do(newTestServer(t, auth, nil, nil, withPayments(&fakePayments{})), http.MethodPost,
			"/v1/bookings/not-a-uuid/checkout", `{"provider":"esewa"}`, bearer(testAccessToken))

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})
}

func TestHandlePaymentCallback(t *testing.T) {
	const appURL = "https://khel.test"

	newCallbackServer := func(t *testing.T, payments *fakePayments) http.Handler {
		t.Helper()
		payments.t = t
		return NewServer(Options{
			Payments:       payments,
			AppURL:         appURL,
			LoginRateLimit: RateLimit{Disabled: true},
		}).Handler()
	}

	t.Run("a verified payment redirects as paid", func(t *testing.T) {
		bookingID := uuid.New()
		payments := &fakePayments{
			settle: func(context.Context, domain.PaymentProvider, payment.CallbackRef) (domain.Payment, error) {
				return domain.Payment{BookingID: bookingID, Status: domain.PaymentVerified}, nil
			},
		}

		w := do(newCallbackServer(t, payments), http.MethodGet,
			"/v1/payments/esewa/callback"+esewaCallbackQuery("tx-1"), "")

		if w.Code != http.StatusSeeOther {
			t.Fatalf("status = %d (%s), want 303", w.Code, w.Body.String())
		}

		location := w.Header().Get("Location")
		if !strings.HasPrefix(location, appURL+"/bookings?") {
			t.Fatalf("Location = %q, want a redirect into the interface", location)
		}
		if !strings.Contains(location, "payment=paid") {
			t.Errorf("Location = %q, want the outcome in the query", location)
		}
	})

	t.Run("an unverified payment redirects as failed", func(t *testing.T) {
		payments := &fakePayments{
			settle: func(context.Context, domain.PaymentProvider, payment.CallbackRef) (domain.Payment, error) {
				return domain.Payment{BookingID: uuid.New(), Status: domain.PaymentFailed}, nil
			},
		}

		w := do(newCallbackServer(t, payments), http.MethodGet,
			"/v1/payments/esewa/callback"+esewaCallbackQuery("tx-1"), "")

		if !strings.Contains(w.Header().Get("Location"), "payment=failed") {
			t.Errorf("Location = %q", w.Header().Get("Location"))
		}
	})

	// The gateway sends a browser here, and that URL is one anybody can be
	// pointed at. A redirect target taken from the request would be an open
	// redirect on an endpoint built for strangers to hit.
	t.Run("the redirect target ignores the request", func(t *testing.T) {
		payments := &fakePayments{
			settle: func(context.Context, domain.PaymentProvider, payment.CallbackRef) (domain.Payment, error) {
				return domain.Payment{BookingID: uuid.New(), Status: domain.PaymentVerified}, nil
			},
		}

		w := do(newCallbackServer(t, payments), http.MethodGet,
			"/v1/payments/esewa/callback"+esewaCallbackQuery("tx-1")+
				"&return_url=https://evil.test&next=https://evil.test", "")

		if location := w.Header().Get("Location"); !strings.HasPrefix(location, appURL) {
			t.Errorf("Location = %q, want it built from configuration", location)
		}
	})

	// A settle failure is still a person mid-redirect; they get sent home with
	// the outcome, not a JSON envelope in the address bar.
	t.Run("a settlement error still redirects", func(t *testing.T) {
		payments := &fakePayments{
			settle: func(context.Context, domain.PaymentProvider, payment.CallbackRef) (domain.Payment, error) {
				return domain.Payment{}, domain.Conflict("The amount paid doesn't match this booking.")
			},
		}

		w := do(newCallbackServer(t, payments), http.MethodGet,
			"/v1/payments/esewa/callback"+esewaCallbackQuery("tx-1"), "")

		if w.Code != http.StatusSeeOther {
			t.Fatalf("status = %d, want 303", w.Code)
		}
		if !strings.Contains(w.Header().Get("Location"), "payment=failed") {
			t.Errorf("Location = %q", w.Header().Get("Location"))
		}
		// The gateway's quarrel with us is not shown to the player in a URL.
		if strings.Contains(w.Header().Get("Location"), "amount") {
			t.Error("the failure message leaked into the redirect")
		}
	})

	t.Run("an unknown provider is refused", func(t *testing.T) {
		w := do(newCallbackServer(t, &fakePayments{}), http.MethodGet,
			"/v1/payments/paypal/callback?data=x", "")

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})

	t.Run("a callback naming no transaction never reaches the service", func(t *testing.T) {
		// settle is nil, so the fake fails the test if the handler calls it.
		w := do(newCallbackServer(t, &fakePayments{}), http.MethodGet,
			"/v1/payments/esewa/callback", "")

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})

	// It has to be reachable without a token: the gateway redirects a browser
	// that carries no Authorization header. Safe because nothing in the
	// request decides the outcome.
	t.Run("does not require a session", func(t *testing.T) {
		payments := &fakePayments{
			settle: func(context.Context, domain.PaymentProvider, payment.CallbackRef) (domain.Payment, error) {
				return domain.Payment{BookingID: uuid.New(), Status: domain.PaymentVerified}, nil
			},
		}

		w := do(newCallbackServer(t, payments), http.MethodGet,
			"/v1/payments/esewa/callback"+esewaCallbackQuery("tx-1"), "")

		if w.Code == http.StatusUnauthorized {
			t.Error("the gateway callback demanded a token the gateway cannot send")
		}
	})
}

func TestHandlePaymentStatus(t *testing.T) {
	auth := &fakeAuth{authenticate: signedIn(testAccessToken)}
	paymentID := uuid.New()
	payments := &fakePayments{
		status: func(context.Context, uuid.UUID, uuid.UUID) (domain.Payment, error) {
			return domain.Payment{
				ID: paymentID, BookingID: testBookingID, Provider: domain.ProviderEsewa,
				AmountNPR: 1800, Status: domain.PaymentVerified,
				TransactionUUID: "tx-secret", RawResponse: []byte(`{"gateway":"internals"}`),
			}, nil
		},
	}

	w := do(newTestServer(t, auth, nil, nil, withPayments(payments)), http.MethodGet,
		"/v1/bookings/"+testBookingID.String()+"/payment", "", bearer(testAccessToken))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), want 200", w.Code, w.Body.String())
	}

	var got paymentDTO
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding payment: %v", err)
	}
	if got.ID != paymentID || got.Status != domain.PaymentVerified {
		t.Errorf("payment = %+v", got)
	}

	// The transaction id addresses a gateway callback, and the raw response is
	// the gateway's own reply. Neither belongs in a browser.
	body := w.Body.String()
	if strings.Contains(body, "tx-secret") {
		t.Error("the transaction id leaked to the client")
	}
	if strings.Contains(body, "internals") {
		t.Error("the raw gateway response leaked to the client")
	}
}
