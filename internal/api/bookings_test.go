package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/service"
	"github.com/google/uuid"
)

var (
	testCourtID   = uuid.MustParse("22222222-2222-4222-8222-222222222222")
	testBookingID = uuid.MustParse("33333333-3333-4333-8333-333333333333")
)

func testSlot() domain.Slot {
	start := time.Date(2026, 8, 14, 12, 15, 0, 0, time.UTC)
	return domain.Slot{Start: start, End: start.Add(time.Hour)}
}

func TestHandleAvailability(t *testing.T) {
	t.Run("projects the grid for a date", func(t *testing.T) {
		bookings := &fakeBookings{availability: func(context.Context, uuid.UUID, time.Time) ([]domain.GridSlot, error) {
			return []domain.GridSlot{{
				Slot:     testSlot(),
				PriceNPR: 1800,
				IsPeak:   true,
			}}, nil
		}}

		w := do(newTestServer(t, nil, bookings, nil), http.MethodGet,
			"/v1/courts/"+testCourtID.String()+"/availability?date=2026-08-14", "")

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d (%s), want 200", w.Code, w.Body.String())
		}

		var got availabilityDTO
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("decoding availability: %v", err)
		}
		if got.CourtID != testCourtID {
			t.Errorf("court_id = %v, want %v", got.CourtID, testCourtID)
		}
		if got.Date != "2026-08-14" {
			t.Errorf("date = %q, want the one asked for", got.Date)
		}
		if len(got.Slots) != 1 {
			t.Fatalf("slots = %d, want 1", len(got.Slots))
		}

		slot := got.Slots[0]
		if slot.PriceNPR != 1800 {
			t.Errorf("price_npr = %d, want 1800", slot.PriceNPR)
		}
		// Computed once here from the domain's own rule, rather than left for
		// the client to re-derive and possibly derive differently.
		if !slot.Available {
			t.Error("a slot that is neither booked nor past should be available")
		}
		if !slot.StartsAt.Equal(testSlot().Start) {
			t.Errorf("starts_at = %v, want %v", slot.StartsAt, testSlot().Start)
		}

		if !bookings.gotDate.Equal(time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("date passed to the service = %v", bookings.gotDate)
		}
	})

	t.Run("a booked or past slot is not available", func(t *testing.T) {
		bookings := &fakeBookings{availability: func(context.Context, uuid.UUID, time.Time) ([]domain.GridSlot, error) {
			return []domain.GridSlot{
				{Slot: testSlot(), PriceNPR: 1800, IsBooked: true},
				{Slot: testSlot(), PriceNPR: 1800, IsPast: true},
			}, nil
		}}

		w := do(newTestServer(t, nil, bookings, nil), http.MethodGet,
			"/v1/courts/"+testCourtID.String()+"/availability?date=2026-08-14", "")

		var got availabilityDTO
		_ = json.Unmarshal(w.Body.Bytes(), &got)
		for i, slot := range got.Slots {
			if slot.Available {
				t.Errorf("slot %d reported available", i)
			}
		}
	})

	// An empty grid has to serialize as [] rather than null, or the frontend
	// cannot map over it.
	t.Run("a closed day is an empty list, not null", func(t *testing.T) {
		bookings := &fakeBookings{availability: func(context.Context, uuid.UUID, time.Time) ([]domain.GridSlot, error) {
			return nil, nil
		}}

		w := do(newTestServer(t, nil, bookings, nil), http.MethodGet,
			"/v1/courts/"+testCourtID.String()+"/availability?date=2026-08-14", "")

		if body := w.Body.String(); !strings.Contains(body, `"slots":[]`) {
			t.Errorf("body = %s, want an empty slots array", body)
		}
	})

	// A bad date is the caller's mistake and must read as one, not as a 500
	// from something further down choking on a zero time.
	t.Run("date parsing", func(t *testing.T) {
		cases := map[string]struct {
			query      string
			wantStatus int
		}{
			"valid":      {"?date=2026-08-14", http.StatusOK},
			"missing":    {"", http.StatusBadRequest},
			"empty":      {"?date=", http.StatusBadRequest},
			"malformed":  {"?date=14-08-2026", http.StatusBadRequest},
			"not a date": {"?date=tomorrow", http.StatusBadRequest},
			"with time":  {"?date=2026-08-14T12:00:00Z", http.StatusBadRequest},
		}

		for name, tc := range cases {
			t.Run(name, func(t *testing.T) {
				bookings := &fakeBookings{availability: func(context.Context, uuid.UUID, time.Time) ([]domain.GridSlot, error) {
					return nil, nil
				}}

				w := do(newTestServer(t, nil, bookings, nil), http.MethodGet,
					"/v1/courts/"+testCourtID.String()+"/availability"+tc.query, "")

				if w.Code != tc.wantStatus {
					t.Errorf("status = %d (%s), want %d", w.Code, w.Body.String(), tc.wantStatus)
				}
			})
		}
	})

	t.Run("a court id that is not a uuid is a 400", func(t *testing.T) {
		w := do(newTestServer(t, nil, &fakeBookings{}, nil), http.MethodGet,
			"/v1/courts/not-a-uuid/availability?date=2026-08-14", "")

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})
}

func TestHandleCreateBooking(t *testing.T) {
	slot := testSlot()

	t.Run("takes a hold", func(t *testing.T) {
		auth := &fakeAuth{authenticate: signedIn(testAccessToken)}
		bookings := &fakeBookings{create: func(_ context.Context, in service.CreateBookingInput) (domain.Booking, error) {
			return domain.Booking{
				ID:       testBookingID,
				CourtID:  in.CourtID,
				UserID:   in.UserID,
				Slot:     slot,
				PriceNPR: 1800,
				Status:   domain.BookingPending,
				Note:     in.Note,
			}, nil
		}}

		w := do(newTestServer(t, auth, bookings, nil), http.MethodPost, "/v1/bookings", `{
			"court_id": "`+testCourtID.String()+`",
			"starts_at": "2026-08-14T12:15:00Z",
			"ends_at": "2026-08-14T13:15:00Z",
			"note": "five a side"
		}`, bearer(testAccessToken))

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d (%s), want 201", w.Code, w.Body.String())
		}

		var got bookingDTO
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("decoding booking: %v", err)
		}
		if got.ID != testBookingID {
			t.Errorf("id = %v, want %v", got.ID, testBookingID)
		}
		if got.PriceNPR != 1800 {
			t.Errorf("price_npr = %d, want the server's figure", got.PriceNPR)
		}
		// domain.Slot is nested; the wire shape is flat.
		if !got.StartsAt.Equal(slot.Start) || !got.EndsAt.Equal(slot.End) {
			t.Errorf("slot = %v..%v, want %v..%v", got.StartsAt, got.EndsAt, slot.Start, slot.End)
		}
		if got.Reference == "" {
			t.Error("no reference on the booking")
		}
	})

	// The user id comes from the access token. If a body could name one,
	// anyone with a session could book as anyone else.
	t.Run("books as the caller, whatever the body says", func(t *testing.T) {
		auth := &fakeAuth{authenticate: signedIn(testAccessToken)}
		bookings := &fakeBookings{create: func(_ context.Context, in service.CreateBookingInput) (domain.Booking, error) {
			return domain.Booking{ID: testBookingID, UserID: in.UserID, Slot: slot}, nil
		}}

		w := do(newTestServer(t, auth, bookings, nil), http.MethodPost, "/v1/bookings", `{
			"court_id": "`+testCourtID.String()+`",
			"starts_at": "2026-08-14T12:15:00Z",
			"ends_at": "2026-08-14T13:15:00Z",
			"user_id": "44444444-4444-4444-8444-444444444444"
		}`, bearer(testAccessToken))

		// user_id is not a field of the request type, so it is refused
		// outright rather than quietly ignored.
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 for an unknown user_id field", w.Code)
		}
	})

	t.Run("carries the caller's id into the service", func(t *testing.T) {
		auth := &fakeAuth{authenticate: signedIn(testAccessToken)}
		bookings := &fakeBookings{create: func(_ context.Context, in service.CreateBookingInput) (domain.Booking, error) {
			return domain.Booking{ID: testBookingID, UserID: in.UserID, Slot: slot}, nil
		}}

		do(newTestServer(t, auth, bookings, nil), http.MethodPost, "/v1/bookings", `{
			"court_id": "`+testCourtID.String()+`",
			"starts_at": "2026-08-14T12:15:00Z",
			"ends_at": "2026-08-14T13:15:00Z"
		}`, bearer(testAccessToken))

		if bookings.gotInput.UserID != testUser.ID {
			t.Errorf("user id = %v, want the one from the token %v", bookings.gotInput.UserID, testUser.ID)
		}
	})

	t.Run("a race for the same hour is a 409", func(t *testing.T) {
		auth := &fakeAuth{authenticate: signedIn(testAccessToken)}
		bookings := &fakeBookings{create: func(context.Context, service.CreateBookingInput) (domain.Booking, error) {
			return domain.Booking{}, domain.Conflict("Someone took that hour first. Pick another.")
		}}

		w := do(newTestServer(t, auth, bookings, nil), http.MethodPost, "/v1/bookings", `{
			"court_id": "`+testCourtID.String()+`",
			"starts_at": "2026-08-14T12:15:00Z",
			"ends_at": "2026-08-14T13:15:00Z"
		}`, bearer(testAccessToken))

		if w.Code != http.StatusConflict {
			t.Errorf("status = %d, want 409", w.Code)
		}
	})

	t.Run("requires a session", func(t *testing.T) {
		auth := &fakeAuth{authenticate: signedIn(testAccessToken)}

		w := do(newTestServer(t, auth, &fakeBookings{}, nil), http.MethodPost, "/v1/bookings", `{
			"court_id": "`+testCourtID.String()+`",
			"starts_at": "2026-08-14T12:15:00Z",
			"ends_at": "2026-08-14T13:15:00Z"
		}`)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", w.Code)
		}
	})
}

func TestHandleListBookings(t *testing.T) {
	detail := domain.BookingDetail{
		Booking: domain.Booking{
			ID:       testBookingID,
			CourtID:  testCourtID,
			UserID:   testUser.ID,
			Slot:     testSlot(),
			PriceNPR: 1800,
			Status:   domain.BookingPending,
		},
		CourtLabel: "Court A",
		ArenaName:  "Dhuku Futsal",
		ArenaSlug:  "dhuku-futsal",
		ArenaArea:  "Jhamsikhel",
	}

	t.Run("returns the caller's bookings with arena context", func(t *testing.T) {
		auth := &fakeAuth{authenticate: signedIn(testAccessToken)}
		bookings := &fakeBookings{listMine: func(context.Context, uuid.UUID, int) ([]domain.BookingDetail, error) {
			return []domain.BookingDetail{detail}, nil
		}}

		w := do(newTestServer(t, auth, bookings, nil), http.MethodGet, "/v1/bookings", "", bearer(testAccessToken))

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d (%s), want 200", w.Code, w.Body.String())
		}

		var got []bookingDetailDTO
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("decoding bookings: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("bookings = %d, want 1", len(got))
		}
		// The embedded bookingDTO must flatten, not nest.
		if got[0].ID != testBookingID {
			t.Errorf("id = %v, want %v", got[0].ID, testBookingID)
		}
		if got[0].ArenaName != "Dhuku Futsal" || got[0].CourtName != "Court A" {
			t.Errorf("arena context = %+v", got[0])
		}
		if bookings.gotUserID != testUser.ID {
			t.Errorf("listed for %v, want the caller %v", bookings.gotUserID, testUser.ID)
		}
	})

	t.Run("limit", func(t *testing.T) {
		cases := map[string]struct {
			query      string
			wantStatus int
			wantLimit  int
		}{
			"absent defaults":    {"", http.StatusOK, defaultBookingLimit},
			"honoured":           {"?limit=5", http.StatusOK, 5},
			"clamped at the top": {"?limit=100000", http.StatusOK, maxBookingLimit},
			"not a number":       {"?limit=twenty", http.StatusBadRequest, 0},
			"zero":               {"?limit=0", http.StatusBadRequest, 0},
			"negative":           {"?limit=-3", http.StatusBadRequest, 0},
		}

		for name, tc := range cases {
			t.Run(name, func(t *testing.T) {
				auth := &fakeAuth{authenticate: signedIn(testAccessToken)}
				bookings := &fakeBookings{listMine: func(context.Context, uuid.UUID, int) ([]domain.BookingDetail, error) {
					return nil, nil
				}}

				w := do(newTestServer(t, auth, bookings, nil), http.MethodGet, "/v1/bookings"+tc.query, "", bearer(testAccessToken))

				if w.Code != tc.wantStatus {
					t.Fatalf("status = %d (%s), want %d", w.Code, w.Body.String(), tc.wantStatus)
				}
				if tc.wantStatus == http.StatusOK && bookings.gotLimit != tc.wantLimit {
					t.Errorf("limit = %d, want %d", bookings.gotLimit, tc.wantLimit)
				}
			})
		}
	})

	t.Run("no bookings is an empty list, not null", func(t *testing.T) {
		auth := &fakeAuth{authenticate: signedIn(testAccessToken)}
		bookings := &fakeBookings{listMine: func(context.Context, uuid.UUID, int) ([]domain.BookingDetail, error) {
			return nil, nil
		}}

		w := do(newTestServer(t, auth, bookings, nil), http.MethodGet, "/v1/bookings", "", bearer(testAccessToken))

		if body := w.Body.String(); body != "[]\n" {
			t.Errorf("body = %q, want an empty array", body)
		}
	})
}

func TestHandleCancelBooking(t *testing.T) {
	t.Run("cancels", func(t *testing.T) {
		auth := &fakeAuth{authenticate: signedIn(testAccessToken)}
		var gotBookingID uuid.UUID
		bookings := &fakeBookings{cancel: func(_ context.Context, bookingID, userID uuid.UUID) error {
			gotBookingID = bookingID
			return nil
		}}

		w := do(newTestServer(t, auth, bookings, nil), http.MethodDelete,
			"/v1/bookings/"+testBookingID.String(), "", bearer(testAccessToken))

		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d (%s), want 204", w.Code, w.Body.String())
		}
		if gotBookingID != testBookingID {
			t.Errorf("booking id = %v, want %v", gotBookingID, testBookingID)
		}
		// Ownership is a condition of the repository's UPDATE, so the handler
		// must reach it with the caller's id rather than loading the booking
		// and checking here.
		if bookings.gotUserID != testUser.ID {
			t.Errorf("user id = %v, want the caller %v", bookings.gotUserID, testUser.ID)
		}
	})

	t.Run("someone else's booking is a 404, decided by the repository", func(t *testing.T) {
		auth := &fakeAuth{authenticate: signedIn(testAccessToken)}
		bookings := &fakeBookings{cancel: func(context.Context, uuid.UUID, uuid.UUID) error {
			return domain.NotFound("No booking with that reference.")
		}}

		w := do(newTestServer(t, auth, bookings, nil), http.MethodDelete,
			"/v1/bookings/"+testBookingID.String(), "", bearer(testAccessToken))

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", w.Code)
		}
	})

	t.Run("a booking id that is not a uuid is a 400", func(t *testing.T) {
		auth := &fakeAuth{authenticate: signedIn(testAccessToken)}

		w := do(newTestServer(t, auth, &fakeBookings{}, nil), http.MethodDelete,
			"/v1/bookings/not-a-uuid", "", bearer(testAccessToken))

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})

	t.Run("requires a session", func(t *testing.T) {
		auth := &fakeAuth{authenticate: signedIn(testAccessToken)}

		w := do(newTestServer(t, auth, &fakeBookings{}, nil), http.MethodDelete,
			"/v1/bookings/"+testBookingID.String(), "")

		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", w.Code)
		}
	})
}

func TestHealthProbes(t *testing.T) {
	t.Run("healthz answers while the process is up", func(t *testing.T) {
		w := do(newTestServer(t, nil, nil, nil), http.MethodGet, "/healthz", "")

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", w.Code)
		}
	})

	t.Run("readyz reflects the database", func(t *testing.T) {
		srv := NewServer(Options{Pinger: fakePinger{err: context.DeadlineExceeded}})

		w := do(srv.Handler(), http.MethodGet, "/readyz", "")

		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503 when the database is unreachable", w.Code)
		}
		if w.Header().Get("Retry-After") == "" {
			t.Error("no Retry-After on a 503")
		}
	})
}
