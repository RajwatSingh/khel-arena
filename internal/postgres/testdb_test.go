package postgres

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/platform/config"
)

// These tests run against a real PostgreSQL server. There is no fake: the
// behaviour under test *is* the database's -- an exclusion constraint, an
// advisory lock, and what happens when two transactions collide. A stub would
// only prove the stub agrees with itself.
//
// Point TEST_DATABASE_URL at a disposable database and run:
//
//	TEST_DATABASE_URL=postgres://khel:khel@localhost:55432/khel_arena_test go test ./internal/postgres/
//
// Without it, these tests skip rather than fail, so `go test ./...` stays
// useful on a machine with no database.

var schemaOnce atomic.Bool

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping integration tests")
	}

	ctx := context.Background()
	pool, err := Connect(ctx, config.Database{
		URL:      url,
		MaxConns: 20,
		MinConns: 2,
	})
	if err != nil {
		t.Fatalf("connecting to the test database: %v", err)
	}
	t.Cleanup(pool.Close)

	// Migrate once per run, not once per test.
	if schemaOnce.CompareAndSwap(false, true) {
		if _, err := Migrate(ctx, pool); err != nil {
			t.Fatalf("migrating the test database: %v", err)
		}
	}

	return pool
}

// fixture is a freshly seeded arena with one court, plus a pool of players.
// Each test gets its own rows, so tests do not collide and none of them has
// to clean up after another.
type fixture struct {
	pool    *pgxpool.Pool
	repo    *BookingRepo
	courtID uuid.UUID
	arenaID uuid.UUID
	owner   uuid.UUID
	players []uuid.UUID
}

func newFixture(t *testing.T, playerCount int) *fixture {
	t.Helper()

	pool := testPool(t)
	ctx := context.Background()

	// A unique suffix keeps parallel tests from colliding on unique columns.
	suffix := strings.ReplaceAll(uuid.NewString()[:8], "-", "")

	f := &fixture{pool: pool, repo: NewBookingRepo(pool)}

	err := pool.QueryRow(ctx, `
		insert into users (email, password_hash, username, full_name, account_type)
		values ($1, 'x', $2, 'Arena Owner', 'arena_owner')
		returning id`,
		"owner-"+suffix+"@test.np", "owner_"+suffix,
	).Scan(&f.owner)
	if err != nil {
		t.Fatalf("seeding arena owner: %v", err)
	}

	err = pool.QueryRow(ctx, `
		insert into arenas (owner_id, name, slug, area, opens_at, closes_at)
		values ($1, $2, $3, 'Jhamsikhel', '06:00', '22:00')
		returning id`,
		f.owner, "Test Arena "+suffix, "test-arena-"+suffix,
	).Scan(&f.arenaID)
	if err != nil {
		t.Fatalf("seeding arena: %v", err)
	}

	err = pool.QueryRow(ctx, `
		insert into courts (arena_id, label, base_price)
		values ($1, 'Court A', 1200)
		returning id`, f.arenaID,
	).Scan(&f.courtID)
	if err != nil {
		t.Fatalf("seeding court: %v", err)
	}

	for i := range playerCount {
		var id uuid.UUID
		err := pool.QueryRow(ctx, `
			insert into users (email, password_hash, username, full_name)
			values ($1, 'x', $2, $3)
			returning id`,
			fmt.Sprintf("p%d-%s@test.np", i, suffix),
			fmt.Sprintf("player_%d_%s", i, suffix),
			fmt.Sprintf("Player %d", i),
		).Scan(&id)
		if err != nil {
			t.Fatalf("seeding player %d: %v", i, err)
		}
		f.players = append(f.players, id)
	}

	t.Cleanup(func() {
		// Ordered by dependency; the schema's cascades handle the rest.
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `delete from bookings where court_id = $1`, f.courtID)
		_, _ = pool.Exec(cleanup, `delete from arenas where id = $1`, f.arenaID)
		ids := append([]uuid.UUID{f.owner}, f.players...)
		_, _ = pool.Exec(cleanup, `delete from users where id = any($1)`, ids)
	})

	return f
}

// slotAt builds a slot on a fixed future date, at a Kathmandu wall-clock hour
// inside the seeded arena's 06:00-22:00 opening hours.
func (f *fixture) slotAt(t *testing.T, hour int, duration time.Duration) domain.Slot {
	t.Helper()

	ktm, err := time.LoadLocation("Asia/Kathmandu")
	if err != nil {
		t.Fatalf("loading Asia/Kathmandu: %v", err)
	}
	// Far enough ahead that the slot never falls into the past mid-run.
	start := time.Date(2031, time.March, 15, hour, 0, 0, 0, ktm)

	slot, err := domain.NewSlot(start, start.Add(duration))
	if err != nil {
		t.Fatalf("building slot: %v", err)
	}
	return slot
}

// hold builds an unsaved pending booking for a player.
func (f *fixture) hold(t *testing.T, player uuid.UUID, slot domain.Slot) domain.Booking {
	t.Helper()

	b, err := domain.NewHold(
		f.courtID, player, slot,
		domain.Price{PerHourNPR: 1200, TotalNPR: 1200 * slot.Hours()},
		nil, "", time.Now().UTC(), 15*time.Minute,
	)
	if err != nil {
		t.Fatalf("building hold: %v", err)
	}
	return b
}

// countBookings reports how many live bookings cover a slot on the fixture's
// court -- the number that must never exceed one.
func (f *fixture) countLiveBookings(t *testing.T, slot domain.Slot) int {
	t.Helper()

	var n int
	err := f.pool.QueryRow(context.Background(), `
		select count(*) from bookings
		 where court_id = $1 and slot && $2
		   and status not in ('cancelled', 'no_show')`,
		f.courtID, tstzrange(slot),
	).Scan(&n)
	if err != nil {
		t.Fatalf("counting bookings: %v", err)
	}
	return n
}
