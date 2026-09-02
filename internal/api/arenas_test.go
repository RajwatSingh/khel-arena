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

var _ ArenaAPI = (*service.ArenaService)(nil)

var (
	testArenaID = uuid.MustParse("44444444-4444-4444-8444-444444444444")
	testRuleID  = uuid.MustParse("66666666-6666-4666-8666-666666666666")
)

func testArena() domain.Arena {
	opens, _ := domain.ParseDayTime("06:00")
	closes, _ := domain.ParseDayTime("22:00")
	rating := 4.7

	return domain.Arena{
		ID:          testArenaID,
		Name:        "Dhuku Futsal",
		Slug:        "dhuku-futsal",
		Area:        "Jhamsikhel",
		City:        "Kathmandu",
		Description: "Two covered courts behind the bus stop.",
		Amenities:   []string{"Covered", "Floodlit"},
		Phone:       "9851000000",
		OpensAt:     opens,
		ClosesAt:    closes,
		Rating:      &rating,
		ReviewCount: 312,
		IsActive:    true,
	}
}

func testCourt() postgres.CourtWithRules {
	return postgres.CourtWithRules{
		Court: domain.Court{
			ID:           testCourtID,
			ArenaID:      testArenaID,
			Label:        "Court A",
			Sport:        domain.SportFutsal,
			Surface:      "40mm turf",
			SideCount:    5,
			BasePriceNPR: 1400,
			IsActive:     true,
		},
		PricingRules: []domain.PricingRule{{
			ID:        testRuleID,
			Label:     "Evening Peak",
			Days:      []time.Weekday{time.Monday, time.Sunday},
			StartHour: 17,
			EndHour:   21,
			PriceNPR:  2100,
			IsPeak:    true,
			Priority:  10,
		}},
	}
}

func TestHandleListArenas(t *testing.T) {
	arenas := &fakeArenas{list: func(context.Context) ([]postgres.ArenaListing, error) {
		return []postgres.ArenaListing{{
			Arena:        testArena(),
			CourtCount:   2,
			FromPriceNPR: 1400,
			Sports:       []domain.Sport{domain.SportFutsal},
		}}, nil
	}}

	w := do(newTestServer(t, nil, nil, nil, withArenas(arenas)), http.MethodGet, "/v1/arenas", "")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), want 200", w.Code, w.Body.String())
	}

	var got []arenaListingDTO
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding arenas: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("arenas = %d, want 1", len(got))
	}
	if got[0].Slug != "dhuku-futsal" || got[0].Name != "Dhuku Futsal" {
		t.Errorf("arena = %+v", got[0])
	}
	// Both computed in SQL. If they ever arrive as zero it means the listing
	// query lost its aggregate, which the index would render as "0 courts".
	if got[0].CourtCount != 2 || got[0].FromPriceNPR != 1400 {
		t.Errorf("court_count = %d, from_price_npr = %d", got[0].CourtCount, got[0].FromPriceNPR)
	}
	// The index badges each row with the sports a venue offers, and the
	// listing payload carries no courts to derive them from.
	if len(got[0].Sports) != 1 || got[0].Sports[0] != domain.SportFutsal {
		t.Errorf("sports = %v, want [futsal]", got[0].Sports)
	}
	// The owner view lists closed venues too, and needs to be able to say so.
	if !got[0].IsActive {
		t.Error("is_active did not reach the client")
	}
	// DayTime renders itself; the interface prints these straight.
	if got[0].OpensAt != "06:00" || got[0].ClosesAt != "22:00" {
		t.Errorf("hours = %q..%q, want 06:00..22:00", got[0].OpensAt, got[0].ClosesAt)
	}
}

func TestHandleListArenasEmptyIsAnArray(t *testing.T) {
	arenas := &fakeArenas{list: func(context.Context) ([]postgres.ArenaListing, error) { return nil, nil }}

	w := do(newTestServer(t, nil, nil, nil, withArenas(arenas)), http.MethodGet, "/v1/arenas", "")

	if body := w.Body.String(); body != "[]\n" {
		t.Errorf("body = %q, want an empty array the client can map over", body)
	}
}

func TestHandleGetArena(t *testing.T) {
	t.Run("returns the venue with its courts and rules", func(t *testing.T) {
		arenas := &fakeArenas{bySlug: func(_ context.Context, slug string) (postgres.ArenaDetail, error) {
			return postgres.ArenaDetail{Arena: testArena(), Courts: []postgres.CourtWithRules{testCourt()}}, nil
		}}

		w := do(newTestServer(t, nil, nil, nil, withArenas(arenas)), http.MethodGet, "/v1/arenas/dhuku-futsal", "")

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d (%s), want 200", w.Code, w.Body.String())
		}
		if arenas.gotSlug != "dhuku-futsal" {
			t.Errorf("slug passed to the service = %q", arenas.gotSlug)
		}

		var got arenaDetailDTO
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("decoding arena: %v", err)
		}
		if len(got.Courts) != 1 {
			t.Fatalf("courts = %d, want 1", len(got.Courts))
		}

		court := got.Courts[0]
		if court.Name != "Court A" || court.BasePriceNPR != 1400 {
			t.Errorf("court = %+v", court)
		}
		// No stored format on this court, so it is derived from the side count
		// rather than left blank.
		if court.Format != "5-a-side" {
			t.Errorf("format = %q, want it derived from side_count", court.Format)
		}
		if len(court.Rules) != 1 || court.Rules[0].Label != "Evening Peak" {
			t.Fatalf("rules = %+v", court.Rules)
		}
		// Deleting a rule needs its id, and reading is the only way to learn
		// one -- without this the delete endpoint cannot be reached.
		if court.Rules[0].ID != testRuleID {
			t.Errorf("rule id = %v, want %v", court.Rules[0].ID, testRuleID)
		}
		// ISO weekdays on the wire: Monday is 1 and Sunday is 7. Go counts
		// Sunday as zero, and letting that out would shift every rule by a day
		// in any client that reads it.
		if want := []int{1, 7}; !equalInts(court.Rules[0].Days, want) {
			t.Errorf("days = %v, want %v", court.Rules[0].Days, want)
		}
	})

	t.Run("a stored format wins over the derived one", func(t *testing.T) {
		court := testCourt()
		court.Format = "7-a-side"
		arenas := &fakeArenas{bySlug: func(context.Context, string) (postgres.ArenaDetail, error) {
			return postgres.ArenaDetail{Arena: testArena(), Courts: []postgres.CourtWithRules{court}}, nil
		}}

		w := do(newTestServer(t, nil, nil, nil, withArenas(arenas)), http.MethodGet, "/v1/arenas/dhuku-futsal", "")

		var got arenaDetailDTO
		_ = json.Unmarshal(w.Body.Bytes(), &got)
		if got.Courts[0].Format != "7-a-side" {
			t.Errorf("format = %q, want the owner's own label", got.Courts[0].Format)
		}
	})

	t.Run("an unknown slug is a 404", func(t *testing.T) {
		arenas := &fakeArenas{bySlug: func(context.Context, string) (postgres.ArenaDetail, error) {
			return postgres.ArenaDetail{}, domain.NotFound("No arena at that address.")
		}}

		w := do(newTestServer(t, nil, nil, nil, withArenas(arenas)), http.MethodGet, "/v1/arenas/nowhere", "")

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", w.Code)
		}
	})
}

func TestHandleListAreas(t *testing.T) {
	arenas := &fakeArenas{listAreas: func(context.Context) ([]string, error) {
		return []string{"Jhamsikhel", "Baluwatar"}, nil
	}}

	w := do(newTestServer(t, nil, nil, nil, withArenas(arenas)), http.MethodGet, "/v1/areas", "")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var got []string
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if len(got) != 2 || got[0] != "Jhamsikhel" {
		t.Errorf("areas = %v", got)
	}
}

func TestHandleLedger(t *testing.T) {
	ledger := func() service.Ledger {
		cheapest := 1400
		return service.Ledger{
			Date: time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC),
			Rows: []service.LedgerRow{{
				LedgerCourt: postgres.LedgerCourt{
					CourtWithRules: testCourt(),
					ArenaID:        testArenaID,
					ArenaName:      "Dhuku Futsal",
					ArenaSlug:      "dhuku-futsal",
					ArenaArea:      "Jhamsikhel",
				},
				Slots: []domain.GridSlot{{Slot: testSlot(), PriceNPR: 1400, RuleLabel: "Base rate"}},
			}},
			OpenHours:   1,
			CheapestNPR: &cheapest,
		}
	}

	t.Run("projects the city for a date", func(t *testing.T) {
		arenas := &fakeArenas{cityLedger: func(context.Context, time.Time, domain.Sport, string) (service.Ledger, error) {
			return ledger(), nil
		}}

		w := do(newTestServer(t, nil, nil, nil, withArenas(arenas)), http.MethodGet, "/v1/ledger?date=2026-08-14", "")

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d (%s), want 200", w.Code, w.Body.String())
		}

		var got ledgerDTO
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("decoding ledger: %v", err)
		}
		if got.Date != "2026-08-14" || got.OpenHours != 1 {
			t.Errorf("ledger = %+v", got)
		}
		if got.CheapestNPR == nil || *got.CheapestNPR != 1400 {
			t.Errorf("cheapest_npr = %v, want 1400", got.CheapestNPR)
		}
		if len(got.Rows) != 1 {
			t.Fatalf("rows = %d, want 1", len(got.Rows))
		}

		// Every row has to carry its venue, or the grid links each cell to
		// /arenas/undefined and labels it with nothing -- which is exactly the
		// bug this shape exists to prevent.
		row := got.Rows[0]
		if row.ArenaSlug != "dhuku-futsal" || row.ArenaName != "Dhuku Futsal" || row.ArenaArea != "Jhamsikhel" {
			t.Errorf("row is missing its arena: %+v", row)
		}
		if row.CourtName != "Court A" || row.Format != "5-a-side" {
			t.Errorf("row court = %q / %q", row.CourtName, row.Format)
		}
		if len(row.Slots) != 1 || row.Slots[0].Rule != "Base rate" {
			t.Errorf("slots = %+v", row.Slots)
		}
	})

	t.Run("filters pass through, and 'all' means no filter", func(t *testing.T) {
		cases := map[string]struct{ sport, area string }{
			"?date=2026-08-14":                              {"", ""},
			"?date=2026-08-14&sport=all&area=all":           {"", ""},
			"?date=2026-08-14&sport=futsal":                 {"futsal", ""},
			"?date=2026-08-14&sport=futsal&area=Jhamsikhel": {"futsal", "Jhamsikhel"},
			"?date=2026-08-14&area=Baluwatar":               {"", "Baluwatar"},
		}

		for query, want := range cases {
			t.Run(query, func(t *testing.T) {
				arenas := &fakeArenas{cityLedger: func(context.Context, time.Time, domain.Sport, string) (service.Ledger, error) {
					return service.Ledger{}, nil
				}}

				do(newTestServer(t, nil, nil, nil, withArenas(arenas)), http.MethodGet, "/v1/ledger"+query, "")

				if string(arenas.gotSport) != want.sport || arenas.gotArea != want.area {
					t.Errorf("service got sport=%q area=%q, want %q / %q",
						arenas.gotSport, arenas.gotArea, want.sport, want.area)
				}
			})
		}
	})

	t.Run("date parsing", func(t *testing.T) {
		cases := map[string]int{
			"?date=2026-08-14": http.StatusOK,
			"":                 http.StatusBadRequest,
			"?date=":           http.StatusBadRequest,
			"?date=tomorrow":   http.StatusBadRequest,
			"?date=14-08-2026": http.StatusBadRequest,
		}

		for query, want := range cases {
			t.Run(query, func(t *testing.T) {
				arenas := &fakeArenas{cityLedger: func(context.Context, time.Time, domain.Sport, string) (service.Ledger, error) {
					return service.Ledger{}, nil
				}}

				w := do(newTestServer(t, nil, nil, nil, withArenas(arenas)), http.MethodGet, "/v1/ledger"+query, "")

				if w.Code != want {
					t.Errorf("status = %d (%s), want %d", w.Code, w.Body.String(), want)
				}
			})
		}
	})

	t.Run("an unknown sport is rejected by the service", func(t *testing.T) {
		arenas := &fakeArenas{cityLedger: func(context.Context, time.Time, domain.Sport, string) (service.Ledger, error) {
			return service.Ledger{}, domain.Invalid("sport", "We don't list that sport.")
		}}

		w := do(newTestServer(t, nil, nil, nil, withArenas(arenas)), http.MethodGet, "/v1/ledger?date=2026-08-14&sport=quidditch", "")

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})

	t.Run("an empty city is an array, not null", func(t *testing.T) {
		arenas := &fakeArenas{cityLedger: func(context.Context, time.Time, domain.Sport, string) (service.Ledger, error) {
			return service.Ledger{Date: time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)}, nil
		}}

		w := do(newTestServer(t, nil, nil, nil, withArenas(arenas)), http.MethodGet, "/v1/ledger?date=2026-08-14", "")

		if !strings.Contains(w.Body.String(), `"rows":[]`) {
			t.Errorf("body = %s, want an empty rows array", w.Body.String())
		}
	})
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
