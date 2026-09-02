package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/platform/payment"
	"github.com/RajwatSingh/khel-arena/internal/postgres"
	"github.com/RajwatSingh/khel-arena/internal/service"
	"github.com/google/uuid"
)

// The interfaces exist so the handlers can be tested without a database. That
// only holds if the real services still satisfy them, so assert it here --
// this fails at compile time the moment a service signature drifts.
var (
	_ AuthAPI    = (*service.AuthService)(nil)
	_ BookingAPI = (*service.BookingService)(nil)
	_ ProfileAPI = (*service.ProfileService)(nil)
)

// TestRoutes walks the endpoint table.
//
// Every row is one call web/src/lib/api/client.js makes. A route that is
// missing, registered under a different verb, or accidentally left off the
// auth middleware shows up here rather than in the browser.
func TestRoutes(t *testing.T) {
	// Every method these fakes expose succeeds, so any non-2xx below comes
	// from routing or middleware rather than from a handler's own logic.
	auth := &fakeAuth{
		authenticate: signedIn(testAccessToken),
		register: func(context.Context, domain.Registration, postgres.SessionContext) (service.Session, error) {
			return testSession(), nil
		},
		login: func(context.Context, string, string, postgres.SessionContext) (service.Session, error) {
			return testSession(), nil
		},
		refresh: func(context.Context, string, postgres.SessionContext) (service.Session, error) {
			return testSession(), nil
		},
		logout: func(context.Context, string) error { return nil },
		beginPasswordReset: func(context.Context, string) (string, domain.User, error) {
			return "", domain.User{}, nil
		},
		completePasswordReset: func(context.Context, string, string) error { return nil },
		changePassword:        func(context.Context, uuid.UUID, string, string) error { return nil },
	}
	bookings := &fakeBookings{
		create: func(_ context.Context, in service.CreateBookingInput) (domain.Booking, error) {
			return domain.Booking{ID: testBookingID, UserID: in.UserID, Slot: testSlot()}, nil
		},
		availability: func(context.Context, uuid.UUID, time.Time) ([]domain.GridSlot, error) { return nil, nil },
		listMine:     func(context.Context, uuid.UUID, int) ([]domain.BookingDetail, error) { return nil, nil },
		cancel:       func(context.Context, uuid.UUID, uuid.UUID) error { return nil },
	}
	profiles := &fakeProfiles{
		me: func(context.Context, uuid.UUID) (domain.User, error) { return testUser, nil },
		update: func(context.Context, uuid.UUID, domain.ProfileUpdate) (domain.User, error) {
			return testUser, nil
		},
	}

	arenas := &fakeArenas{
		list:      func(context.Context) ([]postgres.ArenaListing, error) { return nil, nil },
		bySlug:    func(context.Context, string) (postgres.ArenaDetail, error) { return postgres.ArenaDetail{}, nil },
		listAreas: func(context.Context) ([]string, error) { return nil, nil },
		cityLedger: func(context.Context, time.Time, domain.Sport, string) (service.Ledger, error) {
			return service.Ledger{}, nil
		},
	}

	payments := &fakePayments{
		checkout: func(context.Context, uuid.UUID, uuid.UUID, domain.PaymentProvider) (payment.Checkout, domain.Payment, error) {
			return payment.Checkout{Method: "GET", URL: "https://gateway.test/pay"}, domain.Payment{}, nil
		},
		status: func(context.Context, uuid.UUID, uuid.UUID) (domain.Payment, error) {
			return domain.Payment{}, nil
		},
	}

	matches := &fakeMatches{
		standings: func(context.Context, int) ([]domain.Standing, error) { return nil, nil },
	}

	h := newTestServer(t, auth, bookings, profiles, withArenas(arenas), withPayments(payments),
		withMatches(matches))

	cases := []struct {
		method    string
		target    string
		body      string
		protected bool
		want      int
	}{
		{http.MethodGet, "/healthz", "", false, http.StatusOK},
		{http.MethodGet, "/readyz", "", false, http.StatusOK},

		{http.MethodPost, "/v1/auth/register", `{"email":"r@k.np","username":"rajwat","full_name":"R","password":"kathmandu2026"}`, false, http.StatusCreated},
		{http.MethodPost, "/v1/auth/login", `{"email":"r@k.np","password":"kathmandu2026"}`, false, http.StatusOK},
		{http.MethodPost, "/v1/auth/refresh", "", false, http.StatusOK},
		{http.MethodPost, "/v1/auth/logout", "", false, http.StatusNoContent},
		{http.MethodPost, "/v1/auth/password/forgot", `{"email":"r@k.np"}`, false, http.StatusAccepted},
		{http.MethodPost, "/v1/auth/password/reset", `{"token":"t","new_password":"kathmandu2026"}`, false, http.StatusNoContent},
		{http.MethodGet, "/v1/courts/" + testCourtID.String() + "/availability?date=2026-08-14", "", false, http.StatusOK},
		{http.MethodGet, "/v1/arenas", "", false, http.StatusOK},
		{http.MethodGet, "/v1/arenas/dhuku-futsal", "", false, http.StatusOK},
		{http.MethodGet, "/v1/areas", "", false, http.StatusOK},
		{http.MethodGet, "/v1/ledger?date=2026-08-14", "", false, http.StatusOK},
		{http.MethodGet, "/v1/payments/providers", "", false, http.StatusOK},
		{http.MethodGet, "/v1/standings", "", false, http.StatusOK},

		{http.MethodPost, "/v1/auth/password/change", `{"current_password":"a","new_password":"kathmandu2026"}`, true, http.StatusNoContent},
		{http.MethodGet, "/v1/me", "", true, http.StatusOK},
		{http.MethodPost, "/v1/bookings", `{"court_id":"` + testCourtID.String() + `","starts_at":"2026-08-14T12:15:00Z","ends_at":"2026-08-14T13:15:00Z"}`, true, http.StatusCreated},
		{http.MethodGet, "/v1/bookings", "", true, http.StatusOK},
		{http.MethodDelete, "/v1/bookings/" + testBookingID.String(), "", true, http.StatusNoContent},
		{http.MethodPost, "/v1/bookings/" + testBookingID.String() + "/checkout", `{"provider":"esewa"}`, true, http.StatusCreated},
		{http.MethodGet, "/v1/bookings/" + testBookingID.String() + "/payment", "", true, http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.target, func(t *testing.T) {
			var opts []func(*http.Request)
			if tc.protected {
				opts = append(opts, bearer(testAccessToken))
			}

			w := do(h, tc.method, tc.target, tc.body, opts...)
			if w.Code != tc.want {
				t.Fatalf("status = %d (%s), want %d", w.Code, w.Body.String(), tc.want)
			}

			// The same route without a token must be refused. A protected
			// route registered outside the auth middleware still answers 200
			// here, which is the mistake this half of the test exists to
			// catch.
			if tc.protected {
				if w := do(h, tc.method, tc.target, tc.body); w.Code != http.StatusUnauthorized {
					t.Errorf("without a token: status = %d, want 401 -- is this route behind withAuth?", w.Code)
				}
			}
		})
	}
}

// A route that exists under one verb must not answer under another; Go 1.22's
// method patterns give this for free, and this pins it.
func TestRoutesRejectWrongMethod(t *testing.T) {
	auth := &fakeAuth{authenticate: signedIn(testAccessToken)}
	h := newTestServer(t, auth, &fakeBookings{}, &fakeProfiles{})

	cases := []struct{ method, target string }{
		{http.MethodGet, "/v1/auth/login"},
		{http.MethodPost, "/v1/me"},
		{http.MethodDelete, "/v1/bookings"},
		{http.MethodPost, "/v1/courts/" + testCourtID.String() + "/availability"},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.target, func(t *testing.T) {
			w := do(h, tc.method, tc.target, "", bearer(testAccessToken))
			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("status = %d, want 405", w.Code)
			}
		})
	}
}

func TestUnknownRouteIs404(t *testing.T) {
	h := newTestServer(t, &fakeAuth{}, &fakeBookings{}, &fakeProfiles{})

	// Deliberately something the API will never serve. This test has now been
	// broken twice by paths becoming real, which is the hazard of asserting
	// against a route that merely does not exist yet.
	if w := do(h, http.MethodGet, "/v1/definitely-not-a-route", ""); w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}
