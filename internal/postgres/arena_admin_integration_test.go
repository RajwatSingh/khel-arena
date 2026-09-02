package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

// These need a real database, and that is the point.
//
// The owner-facing writes are the one place in this package where the SQL
// carries the authorization -- an UPDATE without its owner predicate is a
// privilege escalation, not a failing assertion somewhere -- and no fake can
// tell you whether the predicate is there. The first version of these
// statements also had a live syntax error that compiled cleanly and passed
// every unit test: `returning` a column list qualified with an alias the
// statement never declared.

func TestCreateAndUpdateArena(t *testing.T) {
	f := newFixture(t, 0)
	repo := NewArenaRepo(f.pool)
	ctx := context.Background()

	opens, _ := domain.ParseDayTime("06:00")
	closes, _ := domain.ParseDayTime("22:00")

	created, err := repo.CreateArena(ctx, domain.Arena{
		OwnerID:   f.owner,
		Name:      "Second Ground",
		Slug:      "second-ground-" + uuid.NewString()[:8],
		Area:      "Sanepa",
		City:      "Lalitpur",
		Amenities: []string{"Covered"},
		OpensAt:   opens,
		ClosesAt:  closes,
	})
	if err != nil {
		t.Fatalf("creating arena: %v", err)
	}
	t.Cleanup(func() {
		_, _ = f.pool.Exec(context.Background(), `delete from arenas where id = $1`, created.ID)
	})

	if created.Name != "Second Ground" || created.City != "Lalitpur" {
		t.Errorf("created = %+v", created)
	}
	if created.OpensAt.String() != "06:00" || created.ClosesAt.String() != "22:00" {
		t.Errorf("hours = %s..%s", created.OpensAt, created.ClosesAt)
	}

	// The update replaces every field it owns, which is why the route is PUT.
	newOpens, _ := domain.ParseDayTime("05:30")
	newCloses, _ := domain.ParseDayTime("23:00")
	lat, lng := 27.6795, 85.3095

	updated, err := repo.UpdateArenaOwnerScoped(ctx, created.ID, f.owner, domain.Arena{
		Name:        "Second Ground Renamed",
		Area:        "Sanepa",
		City:        "Lalitpur",
		Description: "Now with floodlights.",
		Amenities:   []string{"Covered", "Floodlit"},
		Phone:       "9851000000",
		OpensAt:     newOpens,
		ClosesAt:    newCloses,
		Lat:         &lat,
		Lng:         &lng,
	})
	if err != nil {
		t.Fatalf("updating arena: %v", err)
	}

	if updated.Name != "Second Ground Renamed" {
		t.Errorf("name = %q", updated.Name)
	}
	if updated.OpensAt.String() != "05:30" || updated.ClosesAt.String() != "23:00" {
		t.Errorf("hours = %s..%s, want 05:30..23:00", updated.OpensAt, updated.ClosesAt)
	}
	if len(updated.Amenities) != 2 || updated.Phone != "9851000000" {
		t.Errorf("amenities = %v, phone = %q", updated.Amenities, updated.Phone)
	}
	if updated.Lat == nil || *updated.Lat != lat {
		t.Errorf("lat = %v, want %v", updated.Lat, lat)
	}
	// The slug is deliberately not updatable: it is in every shared link.
	if updated.Slug != created.Slug {
		t.Errorf("slug changed from %q to %q", created.Slug, updated.Slug)
	}
}

// The predicate that stops one owner editing another's venue lives in the
// WHERE clause. This is the test that would fail if somebody removed it.
func TestArenaWritesAreOwnerScoped(t *testing.T) {
	f := newFixture(t, 1)
	repo := NewArenaRepo(f.pool)
	ctx := context.Background()

	stranger := f.players[0]
	opens, _ := domain.ParseDayTime("06:00")
	closes, _ := domain.ParseDayTime("22:00")

	_, err := repo.UpdateArenaOwnerScoped(ctx, f.arenaID, stranger, domain.Arena{
		Name: "Stolen", Area: "Nowhere", City: "Kathmandu", OpensAt: opens, ClosesAt: closes,
	})
	if domain.CodeOf(err) != domain.CodeNotFound {
		t.Errorf("code = %q, want not_found", domain.CodeOf(err))
	}

	if err := repo.SetArenaActiveOwnerScoped(ctx, f.arenaID, stranger, false); domain.CodeOf(err) != domain.CodeNotFound {
		t.Errorf("deactivate: code = %q, want not_found", domain.CodeOf(err))
	}

	// And the venue is untouched.
	arena, err := repo.BySlug(ctx, arenaSlug(t, f))
	if err != nil {
		t.Fatalf("reading arena back: %v", err)
	}
	if arena.Name == "Stolen" {
		t.Fatal("a stranger renamed somebody else's venue")
	}
}

func TestCourtWritesAreOwnerScoped(t *testing.T) {
	f := newFixture(t, 1)
	repo := NewArenaRepo(f.pool)
	ctx := context.Background()

	stranger := f.players[0]

	_, err := repo.CreateCourtOwnerScoped(ctx, stranger,
		domain.Court{ArenaID: f.arenaID, Label: "Theirs", Sport: domain.SportFutsal,
			Surface: "turf", SideCount: 5, BasePriceNPR: 1000}, "")
	if domain.CodeOf(err) != domain.CodeNotFound {
		t.Errorf("create: code = %q, want not_found", domain.CodeOf(err))
	}

	_, err = repo.UpdateCourtOwnerScoped(ctx, f.courtID, stranger,
		domain.Court{Label: "Renamed", Sport: domain.SportFutsal, Surface: "turf",
			SideCount: 5, BasePriceNPR: 1}, "")
	if domain.CodeOf(err) != domain.CodeNotFound {
		t.Errorf("update: code = %q, want not_found", domain.CodeOf(err))
	}

	if err := repo.SetCourtActiveOwnerScoped(ctx, f.courtID, stranger, false); domain.CodeOf(err) != domain.CodeNotFound {
		t.Errorf("retire: code = %q, want not_found", domain.CodeOf(err))
	}
}

func TestCourtCreateAndUpdate(t *testing.T) {
	f := newFixture(t, 0)
	repo := NewArenaRepo(f.pool)
	ctx := context.Background()

	created, err := repo.CreateCourtOwnerScoped(ctx, f.owner, domain.Court{
		ArenaID: f.arenaID, Label: "Court B", Sport: domain.SportFutsal,
		Surface: "40mm turf", SideCount: 7, BasePriceNPR: 2000,
	}, "7-a-side")
	if err != nil {
		t.Fatalf("creating court: %v", err)
	}
	if created.Label != "Court B" || created.Format != "7-a-side" || created.BasePriceNPR != 2000 {
		t.Errorf("created = %+v", created)
	}

	updated, err := repo.UpdateCourtOwnerScoped(ctx, created.ID, f.owner, domain.Court{
		Label: "Court B (covered)", Sport: domain.SportFutsal,
		Surface: "40mm turf, covered", SideCount: 7, BasePriceNPR: 2200,
	}, "7-a-side")
	if err != nil {
		t.Fatalf("updating court: %v", err)
	}
	if updated.Label != "Court B (covered)" || updated.BasePriceNPR != 2200 {
		t.Errorf("updated = %+v", updated)
	}

	// A second court with the same label collides, which is what stops an
	// arena having two "Court A"s.
	_, err = repo.CreateCourtOwnerScoped(ctx, f.owner, domain.Court{
		ArenaID: f.arenaID, Label: "Court A", Sport: domain.SportFutsal,
		Surface: "turf", SideCount: 5, BasePriceNPR: 1200,
	}, "")
	if domain.CodeOf(err) != domain.CodeConflict {
		t.Errorf("duplicate label: code = %q, want conflict", domain.CodeOf(err))
	}
}

func TestPricingRuleLifecycle(t *testing.T) {
	f := newFixture(t, 1)
	repo := NewArenaRepo(f.pool)
	ctx := context.Background()

	rule, err := repo.CreatePricingRuleOwnerScoped(ctx, f.owner, domain.PricingRule{
		CourtID:   f.courtID,
		Label:     "Weekend mornings",
		Days:      []time.Weekday{time.Saturday, time.Sunday},
		StartHour: 6,
		EndHour:   11,
		PriceNPR:  900,
		Priority:  5,
	})
	if err != nil {
		t.Fatalf("creating rule: %v", err)
	}
	if rule.ID == uuid.Nil {
		t.Fatal("no id came back -- the delete endpoint is addressed by it")
	}

	// ISO weekdays round-trip: the schema stores 6 and 7, Go counts Sunday as
	// zero, and a mismatch shifts every rule by a day.
	detail, err := repo.BySlug(ctx, arenaSlug(t, f))
	if err != nil {
		t.Fatalf("reading arena: %v", err)
	}
	var found *domain.PricingRule
	for i := range detail.Courts {
		for j := range detail.Courts[i].PricingRules {
			if detail.Courts[i].PricingRules[j].ID == rule.ID {
				found = &detail.Courts[i].PricingRules[j]
			}
		}
	}
	if found == nil {
		t.Fatal("the rule did not come back with the arena")
	}
	if len(found.Days) != 2 || found.Days[0] != time.Saturday || found.Days[1] != time.Sunday {
		t.Errorf("days = %v, want [Saturday Sunday]", found.Days)
	}

	// A stranger cannot delete it.
	if err := repo.DeletePricingRuleOwnerScoped(ctx, rule.ID, f.players[0]); domain.CodeOf(err) != domain.CodeNotFound {
		t.Errorf("stranger delete: code = %q, want not_found", domain.CodeOf(err))
	}

	if err := repo.DeletePricingRuleOwnerScoped(ctx, rule.ID, f.owner); err != nil {
		t.Fatalf("deleting rule: %v", err)
	}
	// Deleting twice is a not-found, not a silent success.
	if err := repo.DeletePricingRuleOwnerScoped(ctx, rule.ID, f.owner); domain.CodeOf(err) != domain.CodeNotFound {
		t.Errorf("second delete: code = %q, want not_found", domain.CodeOf(err))
	}
}

func TestListArenasForOwnerIncludesClosedVenues(t *testing.T) {
	f := newFixture(t, 0)
	repo := NewArenaRepo(f.pool)
	ctx := context.Background()

	if err := repo.SetArenaActiveOwnerScoped(ctx, f.arenaID, f.owner, false); err != nil {
		t.Fatalf("closing venue: %v", err)
	}

	// The public index drops it...
	public, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("listing arenas: %v", err)
	}
	for _, a := range public {
		if a.ID == f.arenaID {
			t.Fatal("a closed venue is still on the public index")
		}
	}

	// ...and the owner's own view keeps it, marked closed. This is the view
	// you come to in order to reopen it.
	mine, err := repo.ListArenasForOwner(ctx, f.owner)
	if err != nil {
		t.Fatalf("listing owner arenas: %v", err)
	}
	var listing *ArenaListing
	for i := range mine {
		if mine[i].ID == f.arenaID {
			listing = &mine[i]
		}
	}
	if listing == nil {
		t.Fatal("the owner cannot see their own closed venue")
	}
	if listing.IsActive {
		t.Error("the closed venue reports itself as active")
	}
}

// arenaSlug reads the fixture arena's slug, which newFixture generates.
func arenaSlug(t *testing.T, f *fixture) string {
	t.Helper()

	var slug string
	err := f.pool.QueryRow(context.Background(),
		`select slug from arenas where id = $1`, f.arenaID).Scan(&slug)
	if err != nil {
		t.Fatalf("reading arena slug: %v", err)
	}
	return slug
}
