package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

// Esewa is the eSewa ePay v2 adapter.
//
// Checkout is a signed form POST; settlement is confirmed by asking eSewa's
// status endpoint. The signature on the outgoing form proves to eSewa that
// the amount came from us -- it is not what proves to us that payment
// happened. That is the status call, and only the status call.
type Esewa struct {
	// SecretKey signs the checkout form. eSewa publishes a shared test key;
	// production issues a real one per merchant.
	SecretKey []byte
	// ProductCode is the merchant identifier ("EPAYTEST" in sandbox).
	ProductCode string
	// FormURL is where the checkout form is POSTed.
	FormURL string
	// StatusURL is the server-to-server status endpoint.
	StatusURL string

	client *http.Client
}

// eSewa's published endpoints. Sandbox and production differ only in host,
// but they are named rather than switched on a boolean so that a
// misconfiguration is visible in the config rather than implied by a flag.
const (
	EsewaFormURLSandbox   = "https://rc-epay.esewa.com.np/api/epay/main/v2/form"
	EsewaStatusURLSandbox = "https://rc.esewa.com.np/api/epay/transaction/status/"
	EsewaFormURLLive      = "https://epay.esewa.com.np/api/epay/main/v2/form"
	EsewaStatusURLLive    = "https://esewa.com.np/api/epay/transaction/status/"
)

func NewEsewa(secretKey []byte, productCode, formURL, statusURL string) *Esewa {
	return &Esewa{
		SecretKey:   secretKey,
		ProductCode: productCode,
		FormURL:     formURL,
		StatusURL:   statusURL,
		client:      httpClient(),
	}
}

func (e *Esewa) Provider() domain.PaymentProvider { return domain.ProviderEsewa }

// signedFieldNames is the exact list, in the exact order, that the signature
// covers. eSewa recomputes the HMAC over these names in this order, so the
// order is part of the protocol and not a formatting choice.
const esewaSignedFieldNames = "total_amount,transaction_uuid,product_code"

// sign computes the base64 HMAC-SHA256 eSewa expects.
//
// The signed string is `name=value,name=value,...` over signedFieldNames in
// order. Amounts must be rendered here exactly as they appear in the form
// field -- a signature over "1200" against a field reading "1200.0" is
// rejected, and the failure looks like a credential problem rather than a
// formatting one.
func (e *Esewa) sign(totalAmount, transactionUUID string) string {
	message := fmt.Sprintf("total_amount=%s,transaction_uuid=%s,product_code=%s",
		totalAmount, transactionUUID, e.ProductCode)

	mac := hmac.New(sha256.New, e.SecretKey)
	mac.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (e *Esewa) Checkout(_ context.Context, p domain.Payment, ret ReturnURLs) (Checkout, error) {
	// Integer rupees throughout. eSewa accepts a decimal but the signature is
	// over the literal string, so one representation is chosen here and used
	// for both the field and the signature.
	total := strconv.Itoa(p.AmountNPR)

	return Checkout{
		Method: http.MethodPost,
		URL:    e.FormURL,
		Fields: map[string]string{
			"amount":                  total,
			"tax_amount":              "0",
			"total_amount":            total,
			"transaction_uuid":        p.TransactionUUID,
			"product_code":            e.ProductCode,
			"product_service_charge":  "0",
			"product_delivery_charge": "0",
			"success_url":             ret.Success,
			"failure_url":             ret.Failure,
			"signed_field_names":      esewaSignedFieldNames,
			"signature":               e.sign(total, p.TransactionUUID),
		},
	}, nil
}

// esewaStatus is the shape of the status endpoint's reply.
type esewaStatus struct {
	ProductCode     string  `json:"product_code"`
	TransactionUUID string  `json:"transaction_uuid"`
	TotalAmount     float64 `json:"total_amount"`
	Status          string  `json:"status"`
	RefID           string  `json:"ref_id"`
}

// Verify asks eSewa what happened, ignoring whatever the redirect claimed.
//
// The redirect carries a base64 JSON payload with its own signature, and
// checking it would be defence in depth at best: it reaches us through the
// player's browser. This asks eSewa directly instead, keyed on our own
// transaction id, and reports only what eSewa says.
func (e *Esewa) Verify(ctx context.Context, p domain.Payment, ref CallbackRef) (domain.GatewayResult, error) {
	q := url.Values{}
	q.Set("product_code", e.ProductCode)
	q.Set("total_amount", strconv.Itoa(p.AmountNPR))
	q.Set("transaction_uuid", p.TransactionUUID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.StatusURL+"?"+q.Encode(), nil)
	if err != nil {
		return domain.GatewayResult{}, badGateway(e.Provider(), err, "building eSewa status request")
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return domain.GatewayResult{}, badGateway(e.Provider(), err, "calling eSewa status")
	}
	defer resp.Body.Close()

	// Bounded: a provider is not permitted to make us allocate without limit.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return domain.GatewayResult{}, badGateway(e.Provider(), err, "reading eSewa status")
	}
	if resp.StatusCode != http.StatusOK {
		return domain.GatewayResult{}, badGateway(e.Provider(),
			fmt.Errorf("status %d: %s", resp.StatusCode, body), "eSewa status")
	}

	var status esewaStatus
	if err := json.Unmarshal(body, &status); err != nil {
		return domain.GatewayResult{}, badGateway(e.Provider(), err, "decoding eSewa status %q", body)
	}

	// A reply about a different transaction is not evidence about this one.
	// The service checks this again (domain.Payment.Verify), and it is checked
	// here too because the two guards fail differently: this one catches a
	// mismatch before any state is touched.
	if status.TransactionUUID != "" && status.TransactionUUID != p.TransactionUUID {
		return domain.GatewayResult{}, domain.Internal(nil,
			"eSewa answered for transaction %q when asked about %q",
			status.TransactionUUID, p.TransactionUUID)
	}

	return domain.GatewayResult{
		// COMPLETE is the only status that means money moved. PENDING,
		// AMBIGUOUS, NOT_FOUND, CANCELED and FULL_REFUND all mean it did not,
		// and are deliberately not enumerated: anything that is not success
		// is not success.
		Verified:        strings.EqualFold(status.Status, "COMPLETE"),
		TransactionUUID: p.TransactionUUID,
		ProviderRef:     status.RefID,
		// eSewa reports a decimal; the booking is integer rupees. Rounding
		// rather than truncating so that 1199.999 back from a float is 1200
		// and not 1199, which would fail the amount check on a correct payment.
		AmountNPR: int(status.TotalAmount + 0.5),
		Raw:       body,
	}, nil
}

// esewaRefFromCallback pulls our transaction id out of an eSewa redirect.
//
// eSewa v2 returns a single `data` parameter: base64 of a JSON object. This
// reads only the transaction id from it and discards the rest, including the
// status and the signature -- believing either would be trusting the browser.
func esewaRefFromCallback(values url.Values) (CallbackRef, error) {
	raw := values.Get("data")
	if raw == "" {
		return CallbackRef{}, domain.Invalid("data", "That payment link is incomplete.")
	}

	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return CallbackRef{}, domain.Invalid("data", "That payment link is malformed.").WithCause(err)
	}

	var payload struct {
		TransactionUUID string `json:"transaction_uuid"`
		TransactionCode string `json:"transaction_code"`
	}
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return CallbackRef{}, domain.Invalid("data", "That payment link is malformed.").WithCause(err)
	}
	if payload.TransactionUUID == "" {
		return CallbackRef{}, domain.Invalid("data", "That payment link names no transaction.")
	}

	return CallbackRef{
		TransactionUUID: payload.TransactionUUID,
		ProviderRef:     payload.TransactionCode,
		Raw:             decoded,
	}, nil
}
