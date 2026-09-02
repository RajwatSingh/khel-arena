package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

// Khalti is the Khalti ePayment v2 adapter.
//
// Both halves are server-to-server: initiating returns a URL to send the
// player to, and lookup reports what became of it. The player's browser never
// carries anything this adapter believes.
type Khalti struct {
	// SecretKey authenticates us to Khalti. Sent as `Key <secret>`.
	SecretKey string
	// BaseURL is the API root; sandbox and production differ by host.
	BaseURL string
	// WebsiteURL is the merchant site Khalti shows the payer.
	WebsiteURL string

	client *http.Client
}

const (
	KhaltiBaseURLSandbox = "https://dev.khalti.com/api/v2"
	KhaltiBaseURLLive    = "https://khalti.com/api/v2"
)

func NewKhalti(secretKey, baseURL, websiteURL string) *Khalti {
	return &Khalti{
		SecretKey:  secretKey,
		BaseURL:    strings.TrimRight(baseURL, "/"),
		WebsiteURL: websiteURL,
		client:     httpClient(),
	}
}

func (k *Khalti) Provider() domain.PaymentProvider { return domain.ProviderKhalti }

// toPaisa converts integer rupees to the paisa Khalti speaks.
//
// One hundredth of a rupee. Every amount crossing this boundary goes through
// here and its inverse, because a missed conversion is not a rounding error --
// it is a factor of a hundred, in whichever direction hurts.
func toPaisa(npr int) int { return npr * 100 }

// fromPaisa converts back, rounding to the nearest rupee. Khalti returns whole
// paisa, and every amount we send is a whole number of rupees, so this is
// exact for anything we originated.
func fromPaisa(paisa int) int { return (paisa + 50) / 100 }

type khaltiInitiateRequest struct {
	ReturnURL         string         `json:"return_url"`
	WebsiteURL        string         `json:"website_url"`
	Amount            int            `json:"amount"`
	PurchaseOrderID   string         `json:"purchase_order_id"`
	PurchaseOrderName string         `json:"purchase_order_name"`
	AmountBreakdown   []khaltiAmount `json:"amount_breakdown,omitempty"`
}

type khaltiAmount struct {
	Label  string `json:"label"`
	Amount int    `json:"amount"`
}

type khaltiInitiateResponse struct {
	PIDX       string `json:"pidx"`
	PaymentURL string `json:"payment_url"`
	ExpiresAt  string `json:"expires_at"`
}

func (k *Khalti) Checkout(ctx context.Context, p domain.Payment, ret ReturnURLs) (Checkout, error) {
	body, err := json.Marshal(khaltiInitiateRequest{
		ReturnURL:  ret.Success,
		WebsiteURL: k.WebsiteURL,
		Amount:     toPaisa(p.AmountNPR),
		// Our transaction id is the order id, so the lookup below can be
		// matched back to the payment we created without trusting anything
		// the browser carried.
		PurchaseOrderID:   p.TransactionUUID,
		PurchaseOrderName: "Court booking",
	})
	if err != nil {
		return Checkout{}, badGateway(k.Provider(), err, "encoding Khalti initiate")
	}

	var out khaltiInitiateResponse
	if err := k.post(ctx, "/epayment/initiate/", body, &out); err != nil {
		return Checkout{}, err
	}
	if out.PaymentURL == "" {
		return Checkout{}, badGateway(k.Provider(),
			fmt.Errorf("no payment_url in reply"), "initiating Khalti payment")
	}

	return Checkout{Method: http.MethodGet, URL: out.PaymentURL}, nil
}

type khaltiLookupResponse struct {
	PIDX          string `json:"pidx"`
	TotalAmount   int    `json:"total_amount"`
	Status        string `json:"status"`
	TransactionID string `json:"transaction_id"`
}

// Verify asks Khalti's lookup endpoint what happened.
//
// Khalti's own documentation is explicit that lookup is the authority and the
// redirect is not, which matches what this package does for every provider.
// The pidx comes from the redirect and is untrusted -- but it is only used to
// address the question, and the answer is Khalti's.
func (k *Khalti) Verify(ctx context.Context, p domain.Payment, ref CallbackRef) (domain.GatewayResult, error) {
	pidx := ref.ProviderRef
	if pidx == "" {
		pidx = p.ProviderRef
	}
	if pidx == "" {
		return domain.GatewayResult{}, domain.Invalid("pidx", "That payment link is incomplete.")
	}

	body, err := json.Marshal(map[string]string{"pidx": pidx})
	if err != nil {
		return domain.GatewayResult{}, badGateway(k.Provider(), err, "encoding Khalti lookup")
	}

	var out khaltiLookupResponse
	raw, err := k.postRaw(ctx, "/epayment/lookup/", body, &out)
	if err != nil {
		return domain.GatewayResult{}, err
	}

	return domain.GatewayResult{
		// "Completed" alone. Pending, Initiated, Refunded, Expired and
		// User canceled all mean no money is ours to keep.
		Verified: strings.EqualFold(out.Status, "Completed"),
		// Left empty rather than echoing the pidx: this field means *our*
		// transaction id, and Khalti's lookup does not return it. The service
		// already knows which payment it asked about, and domain.Verify skips
		// the comparison when this is empty rather than comparing our id
		// against a foreign one and always failing.
		TransactionUUID: "",
		ProviderRef:     out.PIDX,
		AmountNPR:       fromPaisa(out.TotalAmount),
		Raw:             raw,
	}, nil
}

func (k *Khalti) post(ctx context.Context, path string, body []byte, out any) error {
	_, err := k.postRaw(ctx, path, body, out)
	return err
}

func (k *Khalti) postRaw(ctx context.Context, path string, body []byte, out any) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, k.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, badGateway(k.Provider(), err, "building Khalti request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Key "+k.SecretKey)

	resp, err := k.client.Do(req)
	if err != nil {
		return nil, badGateway(k.Provider(), err, "calling Khalti %s", path)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return nil, badGateway(k.Provider(), err, "reading Khalti %s", path)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// The body is kept in the cause, not the message: Khalti's errors are
		// for us to read in a log, not for a player to be shown.
		return nil, badGateway(k.Provider(),
			fmt.Errorf("status %d: %s", resp.StatusCode, raw), "Khalti %s", path)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return nil, badGateway(k.Provider(), err, "decoding Khalti %s reply %q", path, raw)
	}
	return raw, nil
}

// khaltiRefFromCallback reads the pidx out of a Khalti redirect.
//
// The redirect also carries status, amount and a transaction id. All of them
// are ignored: they arrived through the player's browser, and the lookup call
// answers the same questions from a source the player cannot touch.
func khaltiRefFromCallback(values url.Values) (CallbackRef, error) {
	pidx := values.Get("pidx")
	if pidx == "" {
		return CallbackRef{}, domain.Invalid("pidx", "That payment link is incomplete.")
	}

	return CallbackRef{
		TransactionUUID: values.Get("purchase_order_id"),
		ProviderRef:     pidx,
		Raw:             []byte(values.Encode()),
	}, nil
}
