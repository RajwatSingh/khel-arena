package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/postgres"
	"github.com/RajwatSingh/khel-arena/internal/service"
	"github.com/google/uuid"
)

var _ OwnerAPI = (*service.OwnerService)(nil)

var testPaymentID = uuid.MustParse("55555555-5555-4555-8555-555555555555")

func TestOwnerRoutesAllRequireASession(t *testing.T) {
	// Every method is set, so anything that answers without a token got past
	// the auth middleware rather than being refused by a nil fake.
	owner := &fakeOwner{
		myArenas:    func(context.Context, uuid.UUID) ([]postgres.ArenaListing, error) { return nil, nil },
		createArena: func(context.Context, uuid.UUID, domain.Arena) (domain.Arena, error) { return testArena(), nil },
		updateArena: func(context.Context, uuid.UUID, uuid.UUID, domain.Arena) (domain.Arena, error) {
			return testArena(), nil
		},
		setArenaActive: func(context.Context, uuid.UUID, uuid.UUID, bool) error { return nil },
		createCourt: func(context.Context, uuid.UUID, domain.Court, string) (postgres.CourtWithRules, error) {
			return testCourt(), nil
		},
		updateCourt: func(context.Context, uuid.UUID, uuid.UUID, domain.Court, string) (postgres.CourtWithRules, error) {
			return testCourt(), nil
		},
		setCourtActive: func(context.Context, uuid.UUID, uuid.UUID, bool) error { return nil },
		createPricingRule: func(context.Context, uuid.UUID, domain.PricingRule) (domain.PricingRule, error) {
			return domain.PricingRule{}, nil
		},
		copyPricingRules:  func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (int, error) { return 2, nil },
		deletePricingRule: func(context.Context, uuid.UUID, uuid.UUID) error { return nil },
		payments:          func(context.Context, uuid.UUID, uuid.UUID, int) ([]postgres.OwnerPayment, error) { return nil, nil },
		markCashReceived:  func(context.Context, uuid.UUID, uuid.UUID) (domain.Payment, error) { return domain.Payment{}, nil },
	}

	auth := &fakeAuth{authenticate: signedIn(testAccessToken)}
	h := newTestServer(t, auth, nil, nil, withOwner(owner))

	arena := "/v1/owner/arenas/" + testArenaID.String()
	court := "/v1/owner/courts/" + testCourtID.String()

	routes := []struct{ method, target, body string }{
		{http.MethodGet, "/v1/owner/arenas", ""},
		{http.MethodPost, "/v1/owner/arenas", `{"name":"A","area":"B","opens_at":"06:00","closes_at":"22:00"}`},
		{http.MethodPut, arena, `{"name":"A","area":"B","opens_at":"06:00","closes_at":"22:00"}`},
		{http.MethodPut, arena + "/active", `{"active":false}`},
		{http.MethodPost, arena + "/courts", `{"name":"Court A","side_count":5,"base_price_npr":1200}`},
		{http.MethodGet, arena + "/payments", ""},
		{http.MethodPut, court, `{"name":"Court A","side_count":5,"base_price_npr":1200}`},
		{http.MethodPut, court + "/active", `{"active":false}`},
		{http.MethodPost, court + "/pricing", `{"label":"Peak","days":[1],"start_hour":17,"end_hour":21,"price_npr":1800}`},
		{http.MethodPost, court + "/pricing/copy", `{"from_court_id":"` + uuid.New().String() + `"}`},
		{http.MethodDelete, "/v1/owner/pricing/" + uuid.New().String(), ""},
		{http.MethodPost, "/v1/owner/payments/" + testPaymentID.String() + "/received", ""},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.target, func(t *testing.T) {
			if w := do(h, route.method, route.target, route.body); w.Code != http.StatusUnauthorized {
				t.Errorf("without a token: status = %d, want 401 -- is this route behind withAuth?", w.Code)
			}
			if w := do(h, route.method, route.target, route.body, bearer(testAccessToken)); w.Code >= 400 {
				t.Errorf("with a token: status = %d (%s)", w.Code, w.Body.String())
			}
		})
	}
}

// The owner is the caller. A body naming one would let anybody list a venue in
// somebody else's name.
func TestCreateArenaTakesTheOwnerFromTheToken(t *testing.T) {
	owner := &fakeOwner{
		createArena: func(_ context.Context, ownerID uuid.UUID, a domain.Arena) (domain.Arena, error) {
			if ownerID != testUser.ID {
				t.Errorf("owner = %v, want the caller %v", ownerID, testUser.ID)
			}
			return testArena(), nil
		},
	}
	auth := &fakeAuth{authenticate: signedIn(testAccessToken)}

	w := do(newTestServer(t, auth, nil, nil, withOwner(owner)), http.MethodPost, "/v1/owner/arenas",
		`{"name":"Dhuku","area":"Jhamsikhel","opens_at":"06:00","closes_at":"22:00",
		  "owner_id":"99999999-9999-4999-8999-999999999999"}`, bearer(testAccessToken))

	// owner_id is not a field of the request type, so naming one is refused
	// rather than quietly ignored.
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for a body naming an owner", w.Code)
	}
}

// The slug is in every shared link. Changing it silently breaks all of them,
// so it is not on the edit form at all.
func TestArenaSlugIsNotEditable(t *testing.T) {
	auth := &fakeAuth{authenticate: signedIn(testAccessToken)}

	w := do(newTestServer(t, auth, nil, nil, withOwner(&fakeOwner{})), http.MethodPut,
		"/v1/owner/arenas/"+testArenaID.String(),
		`{"name":"Dhuku","area":"Jhamsikhel","opens_at":"06:00","closes_at":"22:00","slug":"new-slug"}`,
		bearer(testAccessToken))

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for a body naming a slug", w.Code)
	}
}

// A venue somebody else owns answers as not-found rather than forbidden: a
// distinct refusal would confirm the arena exists.
func TestEditingSomebodyElsesArena(t *testing.T) {
	owner := &fakeOwner{
		updateArena: func(context.Context, uuid.UUID, uuid.UUID, domain.Arena) (domain.Arena, error) {
			return domain.Arena{}, domain.NotFound("No arena of yours at that address.")
		},
	}
	auth := &fakeAuth{authenticate: signedIn(testAccessToken)}

	w := do(newTestServer(t, auth, nil, nil, withOwner(owner)), http.MethodPut,
		"/v1/owner/arenas/"+testArenaID.String(),
		`{"name":"Theirs","area":"Elsewhere","opens_at":"06:00","closes_at":"22:00"}`,
		bearer(testAccessToken))

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// A player account cannot list a venue. This is the first place the account
// type does real work rather than existing as an enum value.
func TestCreateArenaRequiresAnOwnerAccount(t *testing.T) {
	owner := &fakeOwner{
		createArena: func(context.Context, uuid.UUID, domain.Arena) (domain.Arena, error) {
			return domain.Arena{}, domain.Forbidden("Arena owner accounts can list a venue.")
		},
	}
	auth := &fakeAuth{authenticate: signedIn(testAccessToken)}

	w := do(newTestServer(t, auth, nil, nil, withOwner(owner)), http.MethodPost, "/v1/owner/arenas",
		`{"name":"Dhuku","area":"Jhamsikhel","opens_at":"06:00","closes_at":"22:00"}`,
		bearer(testAccessToken))

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

// The arena comes from the path. A body that could name one would let a court
// be added to a venue the URL says nothing about.
func TestCreateCourtTakesTheArenaFromThePath(t *testing.T) {
	var got domain.Court
	owner := &fakeOwner{
		createCourt: func(_ context.Context, _ uuid.UUID, c domain.Court, format string) (postgres.CourtWithRules, error) {
			got = c
			return testCourt(), nil
		},
	}
	auth := &fakeAuth{authenticate: signedIn(testAccessToken)}

	w := do(newTestServer(t, auth, nil, nil, withOwner(owner)), http.MethodPost,
		"/v1/owner/arenas/"+testArenaID.String()+"/courts",
		`{"name":"Court B","sport":"futsal","format":"7-a-side","side_count":7,"base_price_npr":2000}`,
		bearer(testAccessToken))

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d (%s), want 201", w.Code, w.Body.String())
	}
	if got.ArenaID != testArenaID {
		t.Errorf("arena = %v, want the one in the path %v", got.ArenaID, testArenaID)
	}
}

// ISO weekdays on the wire; Go's Sunday-is-zero counting stops at the boundary.
func TestPricingRuleDayConversion(t *testing.T) {
	var got domain.PricingRule
	owner := &fakeOwner{
		createPricingRule: func(_ context.Context, _ uuid.UUID, rule domain.PricingRule) (domain.PricingRule, error) {
			got = rule
			return rule, nil
		},
	}
	auth := &fakeAuth{authenticate: signedIn(testAccessToken)}

	w := do(newTestServer(t, auth, nil, nil, withOwner(owner)), http.MethodPost,
		"/v1/owner/courts/"+testCourtID.String()+"/pricing",
		`{"label":"Evening Peak","days":[1,7],"start_hour":17,"end_hour":21,"price_npr":1800,"is_peak":true,"priority":10}`,
		bearer(testAccessToken))

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d (%s), want 201", w.Code, w.Body.String())
	}
	if len(got.Days) != 2 || got.Days[0] != time.Monday || got.Days[1] != time.Sunday {
		t.Errorf("days = %v, want [Monday Sunday]", got.Days)
	}
	if got.CourtID != testCourtID {
		t.Errorf("court = %v, want the one in the path", got.CourtID)
	}
}

func TestPricingRuleRejectsAnImpossibleWeekday(t *testing.T) {
	auth := &fakeAuth{authenticate: signedIn(testAccessToken)}

	w := do(newTestServer(t, auth, nil, nil, withOwner(&fakeOwner{})), http.MethodPost,
		"/v1/owner/courts/"+testCourtID.String()+"/pricing",
		`{"label":"Peak","days":[0,9],"start_hour":17,"end_hour":21,"price_npr":1800}`,
		bearer(testAccessToken))

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestOwnerPaymentsCarryReconciliationContext(t *testing.T) {
	start := time.Date(2026, 9, 4, 11, 15, 0, 0, time.UTC)
	owner := &fakeOwner{
		payments: func(context.Context, uuid.UUID, uuid.UUID, int) ([]postgres.OwnerPayment, error) {
			return []postgres.OwnerPayment{{
				Payment: domain.Payment{
					ID: testPaymentID, BookingID: testBookingID, Provider: domain.ProviderCash,
					AmountNPR: 1800, Status: domain.PaymentInitiated,
					TransactionUUID: "tx-secret", RawResponse: []byte(`{"gateway":"internals"}`),
				},
				Slot:           domain.Slot{Start: start, End: start.Add(time.Hour)},
				CourtLabel:     "Court A",
				PlayerName:     "Rajwat Singh",
				PlayerUsername: "rajwat",
			}}, nil
		},
	}
	auth := &fakeAuth{authenticate: signedIn(testAccessToken)}

	w := do(newTestServer(t, auth, nil, nil, withOwner(owner)), http.MethodGet,
		"/v1/owner/arenas/"+testArenaID.String()+"/payments", "", bearer(testAccessToken))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), want 200", w.Code, w.Body.String())
	}

	var got []ownerPaymentDTO
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding payments: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("payments = %d, want 1", len(got))
	}
	// An owner reconciling the till has to know which hour and who to ask.
	if got[0].PlayerName != "Rajwat Singh" || got[0].CourtName != "Court A" {
		t.Errorf("payment = %+v", got[0])
	}
	if !got[0].StartsAt.Equal(start) {
		t.Errorf("starts_at = %v, want %v", got[0].StartsAt, start)
	}

	// The same two fields stay out as on the player-facing view: one addresses
	// a gateway callback, the other is the gateway's own reply.
	body := w.Body.String()
	if strings.Contains(body, "tx-secret") || strings.Contains(body, "internals") {
		t.Errorf("gateway internals leaked to the owner view: %s", body)
	}
}

func TestMarkCashReceived(t *testing.T) {
	t.Run("settles a cash payment", func(t *testing.T) {
		verified := time.Now()
		owner := &fakeOwner{
			markCashReceived: func(_ context.Context, paymentID, ownerID uuid.UUID) (domain.Payment, error) {
				if paymentID != testPaymentID {
					t.Errorf("payment = %v", paymentID)
				}
				if ownerID != testUser.ID {
					t.Errorf("owner = %v, want the caller", ownerID)
				}
				return domain.Payment{
					ID: testPaymentID, Provider: domain.ProviderCash,
					Status: domain.PaymentVerified, VerifiedAt: &verified,
				}, nil
			},
		}
		auth := &fakeAuth{authenticate: signedIn(testAccessToken)}

		w := do(newTestServer(t, auth, nil, nil, withOwner(owner)), http.MethodPost,
			"/v1/owner/payments/"+testPaymentID.String()+"/received", "", bearer(testAccessToken))

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d (%s), want 200", w.Code, w.Body.String())
		}

		var got paymentDTO
		_ = json.Unmarshal(w.Body.Bytes(), &got)
		if got.Status != domain.PaymentVerified {
			t.Errorf("status = %q, want verified", got.Status)
		}
	})

	// Letting an owner mark a gateway payment received by hand would be a way
	// to confirm a booking nobody paid for.
	t.Run("refuses a gateway payment", func(t *testing.T) {
		owner := &fakeOwner{
			markCashReceived: func(context.Context, uuid.UUID, uuid.UUID) (domain.Payment, error) {
				return domain.Payment{}, domain.Conflict("That payment is settled through esewa, not at the arena.")
			},
		}
		auth := &fakeAuth{authenticate: signedIn(testAccessToken)}

		w := do(newTestServer(t, auth, nil, nil, withOwner(owner)), http.MethodPost,
			"/v1/owner/payments/"+testPaymentID.String()+"/received", "", bearer(testAccessToken))

		if w.Code != http.StatusConflict {
			t.Errorf("status = %d, want 409", w.Code)
		}
	})
}
