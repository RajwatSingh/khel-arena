package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

func testPayment() domain.Payment {
	return domain.Payment{
		AmountNPR:       1800,
		TransactionUUID: "tx-abc-123",
		Provider:        domain.ProviderEsewa,
	}
}

// ------------------------------------------------------------------ eSewa --

// The signature is over a literal string in a fixed field order. Both are
// protocol, not formatting, so this pins them: a change to either produces a
// signature eSewa rejects, and the failure looks like a bad credential.
func TestEsewaSignature(t *testing.T) {
	e := NewEsewa([]byte("8gBm/:&EnhH.1/q"), "EPAYTEST", "", "")

	got := e.sign("100", "241028")

	// Recomputed independently: base64(HMAC-SHA256(key,
	// "total_amount=100,transaction_uuid=241028,product_code=EPAYTEST")).
	mac := hmacSHA256Base64([]byte("8gBm/:&EnhH.1/q"),
		"total_amount=100,transaction_uuid=241028,product_code=EPAYTEST")
	if got != mac {
		t.Errorf("signature = %q, want %q", got, mac)
	}
}

func TestEsewaCheckoutFields(t *testing.T) {
	e := NewEsewa([]byte("secret"), "EPAYTEST", "https://esewa.test/form", "")

	c, err := e.Checkout(context.Background(), testPayment(),
		ReturnURLs{Success: "https://khel.test/ok", Failure: "https://khel.test/no"})
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}

	if c.Method != http.MethodPost || c.URL != "https://esewa.test/form" {
		t.Errorf("checkout = %s %s, want a POST to the form URL", c.Method, c.URL)
	}

	// The signature covers total_amount as a literal. If the field and the
	// signed string ever disagree about how 1800 is written, eSewa rejects it.
	if c.Fields["total_amount"] != "1800" || c.Fields["amount"] != "1800" {
		t.Errorf("amounts = %q / %q, want 1800", c.Fields["total_amount"], c.Fields["amount"])
	}
	if c.Fields["signature"] != e.sign("1800", "tx-abc-123") {
		t.Error("the form's signature does not match its own total_amount")
	}
	if c.Fields["signed_field_names"] != esewaSignedFieldNames {
		t.Errorf("signed_field_names = %q", c.Fields["signed_field_names"])
	}
}

func TestEsewaVerifyAsksTheGatewayNotTheRedirect(t *testing.T) {
	var asked url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r.URL.Query()
		_, _ = w.Write([]byte(`{"product_code":"EPAYTEST","transaction_uuid":"tx-abc-123",
			"total_amount":1800,"status":"COMPLETE","ref_id":"0KL2SD"}`))
	}))
	defer server.Close()

	e := NewEsewa([]byte("secret"), "EPAYTEST", "", server.URL)

	// The redirect claims a different, larger amount and a different status.
	// None of it is read.
	result, err := e.Verify(context.Background(), testPayment(), CallbackRef{
		TransactionUUID: "tx-abc-123",
		Raw:             []byte(`{"status":"COMPLETE","total_amount":"999999"}`),
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	if !result.Verified {
		t.Error("a COMPLETE status did not verify")
	}
	if result.AmountNPR != 1800 {
		t.Errorf("amount = %d, want the gateway's 1800 and not the redirect's", result.AmountNPR)
	}
	if result.ProviderRef != "0KL2SD" {
		t.Errorf("provider ref = %q", result.ProviderRef)
	}
	// Asked about our own transaction and our own amount.
	if asked.Get("transaction_uuid") != "tx-abc-123" || asked.Get("total_amount") != "1800" {
		t.Errorf("asked eSewa %v", asked)
	}
}

func TestEsewaVerifyOnlyCompleteCounts(t *testing.T) {
	for _, status := range []string{"PENDING", "AMBIGUOUS", "NOT_FOUND", "CANCELED", "FULL_REFUND", ""} {
		t.Run(status, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"transaction_uuid":"tx-abc-123","total_amount":1800,"status":"` + status + `"}`))
			}))
			defer server.Close()

			e := NewEsewa([]byte("secret"), "EPAYTEST", "", server.URL)
			result, err := e.Verify(context.Background(), testPayment(), CallbackRef{})
			if err != nil {
				t.Fatalf("verify: %v", err)
			}
			if result.Verified {
				t.Errorf("status %q was treated as payment", status)
			}
		})
	}
}

// A reply about a different transaction is not evidence about this one.
func TestEsewaVerifyRefusesAMismatchedTransaction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"transaction_uuid":"somebody-elses","total_amount":1800,"status":"COMPLETE"}`))
	}))
	defer server.Close()

	e := NewEsewa([]byte("secret"), "EPAYTEST", "", server.URL)
	_, err := e.Verify(context.Background(), testPayment(), CallbackRef{})

	if domain.CodeOf(err) != domain.CodeInternal {
		t.Errorf("code = %q, want internal -- this is a defect or an attack, not a routine no", domain.CodeOf(err))
	}
}

func TestEsewaVerifyOnAnUnreachableGateway(t *testing.T) {
	e := NewEsewa([]byte("secret"), "EPAYTEST", "", "http://127.0.0.1:1/status")

	_, err := e.Verify(context.Background(), testPayment(), CallbackRef{})

	// Unavailable, not internal: the gateway being down is not our defect, and
	// the player should be told to try again rather than that we are broken.
	if domain.CodeOf(err) != domain.CodeUnavailable {
		t.Errorf("code = %q, want unavailable", domain.CodeOf(err))
	}
}

func TestEsewaCallbackParsing(t *testing.T) {
	payload := base64.StdEncoding.EncodeToString([]byte(
		`{"transaction_uuid":"tx-abc-123","transaction_code":"0KL2SD","status":"COMPLETE"}`))

	ref, err := esewaRefFromCallback(url.Values{"data": {payload}})
	if err != nil {
		t.Fatalf("parsing callback: %v", err)
	}
	if ref.TransactionUUID != "tx-abc-123" {
		t.Errorf("transaction = %q", ref.TransactionUUID)
	}

	for name, values := range map[string]url.Values{
		"absent":     {},
		"not base64": {"data": {"!!!not base64!!!"}},
		"not json":   {"data": {base64.StdEncoding.EncodeToString([]byte("nope"))}},
		"no uuid":    {"data": {base64.StdEncoding.EncodeToString([]byte(`{"status":"COMPLETE"}`))}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := esewaRefFromCallback(values); domain.CodeOf(err) != domain.CodeInvalid {
				t.Errorf("code = %q, want invalid", domain.CodeOf(err))
			}
		})
	}
}

// ----------------------------------------------------------------- Khalti --

// One hundredth of a rupee. Getting this wrong is not a rounding error, it is
// a factor of a hundred in whichever direction hurts.
func TestKhaltiPaisaConversion(t *testing.T) {
	cases := map[int]int{0: 0, 1: 100, 1800: 180000, 12345: 1234500}
	for npr, paisa := range cases {
		if got := toPaisa(npr); got != paisa {
			t.Errorf("toPaisa(%d) = %d, want %d", npr, got, paisa)
		}
		if got := fromPaisa(paisa); got != npr {
			t.Errorf("fromPaisa(%d) = %d, want %d", paisa, got, npr)
		}
	}
}

func TestKhaltiCheckoutSendsPaisa(t *testing.T) {
	var body map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		if got := r.Header.Get("Authorization"); got != "Key test-secret" {
			t.Errorf("Authorization = %q", got)
		}
		_, _ = w.Write([]byte(`{"pidx":"pidx-1","payment_url":"https://khalti.test/pay/pidx-1"}`))
	}))
	defer server.Close()

	k := NewKhalti("test-secret", server.URL, "https://khel.test")
	p := testPayment()
	p.Provider = domain.ProviderKhalti

	c, err := k.Checkout(context.Background(), p, ReturnURLs{Success: "https://khel.test/back"})
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}

	if c.Method != http.MethodGet || c.URL != "https://khalti.test/pay/pidx-1" {
		t.Errorf("checkout = %s %s, want a redirect to the payment URL", c.Method, c.URL)
	}
	// 1800 rupees is 180,000 paisa. Sending 1800 would charge one percent.
	if body["amount"] != float64(180000) {
		t.Errorf("amount = %v, want 180000 paisa", body["amount"])
	}
	if body["purchase_order_id"] != "tx-abc-123" {
		t.Errorf("purchase_order_id = %v, want our transaction id", body["purchase_order_id"])
	}
}

func TestKhaltiVerifyReadsTheLookupNotTheRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/lookup/") {
			t.Errorf("asked %s, want the lookup endpoint", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"pidx":"pidx-1","total_amount":180000,"status":"Completed"}`))
	}))
	defer server.Close()

	k := NewKhalti("test-secret", server.URL, "https://khel.test")
	p := testPayment()
	p.Provider = domain.ProviderKhalti

	result, err := k.Verify(context.Background(), p, CallbackRef{ProviderRef: "pidx-1"})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	if !result.Verified {
		t.Error("a Completed lookup did not verify")
	}
	if result.AmountNPR != 1800 {
		t.Errorf("amount = %d, want 1800 rupees from 180000 paisa", result.AmountNPR)
	}
	// Khalti's lookup does not return our transaction id, and echoing the pidx
	// into this field would make domain.Verify compare our id against a
	// foreign one and refuse every genuine payment.
	if result.TransactionUUID != "" {
		t.Errorf("transaction_uuid = %q, want it left empty", result.TransactionUUID)
	}
}

func TestKhaltiVerifyOnlyCompletedCounts(t *testing.T) {
	for _, status := range []string{"Pending", "Initiated", "Refunded", "Expired", "User canceled", ""} {
		t.Run(status, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"pidx":"pidx-1","total_amount":180000,"status":"` + status + `"}`))
			}))
			defer server.Close()

			k := NewKhalti("s", server.URL, "https://khel.test")
			result, err := k.Verify(context.Background(), testPayment(), CallbackRef{ProviderRef: "pidx-1"})
			if err != nil {
				t.Fatalf("verify: %v", err)
			}
			if result.Verified {
				t.Errorf("status %q was treated as payment", status)
			}
		})
	}
}

func TestKhaltiCallbackParsing(t *testing.T) {
	ref, err := khaltiRefFromCallback(url.Values{
		"pidx":              {"pidx-1"},
		"purchase_order_id": {"tx-abc-123"},
		"status":            {"Completed"},
		"amount":            {"1"},
	})
	if err != nil {
		t.Fatalf("parsing callback: %v", err)
	}
	if ref.ProviderRef != "pidx-1" || ref.TransactionUUID != "tx-abc-123" {
		t.Errorf("ref = %+v", ref)
	}

	if _, err := khaltiRefFromCallback(url.Values{}); domain.CodeOf(err) != domain.CodeInvalid {
		t.Errorf("code = %q, want invalid for a callback with no pidx", domain.CodeOf(err))
	}
}

// ------------------------------------------------------------------- cash --

// The tempting implementation returns Verified: true and confirms the hour on
// the player's word, which makes every booking free.
func TestCashNeverVerifies(t *testing.T) {
	c := NewCash()

	// Checkout succeeds with nowhere to go: the intent exists for the venue to
	// reconcile, and the empty Method says there is no gateway.
	checkout, err := c.Checkout(context.Background(), testPayment(), ReturnURLs{})
	if err != nil {
		t.Fatalf("cash checkout: %v", err)
	}
	if checkout.Method != "" || checkout.URL != "" {
		t.Errorf("checkout = %+v, want nowhere to send the player", checkout)
	}

	result, verifyErr := c.Verify(context.Background(), testPayment(), CallbackRef{})
	if verifyErr == nil {
		t.Error("cash verified a payment")
	}
	if result.Verified {
		t.Fatal("cash reported a payment as verified")
	}
}

// ---------------------------------------------------------------- factory --

func TestFromAccountPicksTheHostFromLive(t *testing.T) {
	sandbox, err := FromAccount(domain.ArenaPaymentAccount{
		Provider: domain.ProviderEsewa, SecretKey: "s", MerchantCode: "EPAYTEST", Live: false,
	}, "https://khelarena.test")
	if err != nil {
		t.Fatalf("building sandbox eSewa: %v", err)
	}
	if got := sandbox.(*Esewa).FormURL; got != EsewaFormURLSandbox {
		t.Errorf("form URL = %q, want the sandbox host", got)
	}

	live, err := FromAccount(domain.ArenaPaymentAccount{
		Provider: domain.ProviderKhalti, SecretKey: "s", Live: true,
	}, "https://khelarena.test")
	if err != nil {
		t.Fatalf("building live Khalti: %v", err)
	}
	if got := live.(*Khalti).BaseURL; got != KhaltiBaseURLLive {
		t.Errorf("base URL = %q, want the live host", got)
	}

	if _, err := FromAccount(domain.ArenaPaymentAccount{Provider: domain.ProviderCash}, ""); domain.CodeOf(err) != domain.CodeInvalid {
		t.Errorf("code = %q, want invalid for a provider with no per-venue account", domain.CodeOf(err))
	}
}

func TestRefFromCallbackRejectsUnknownProviders(t *testing.T) {
	if _, err := RefFromCallback(domain.ProviderCash, url.Values{}); domain.CodeOf(err) != domain.CodeInvalid {
		t.Errorf("code = %q, want invalid", domain.CodeOf(err))
	}
}

// hmacSHA256Base64 recomputes the signature independently of the code under
// test, so the assertion is against the algorithm rather than against
// whatever sign() happens to do.
func hmacSHA256Base64(key []byte, message string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
