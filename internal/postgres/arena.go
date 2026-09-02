package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

// ArenaRepo reads venues and their courts.
//
// Everything here is a read. Owner-facing writes are a separate piece of work
// with an authorization story of their own, and nothing in this file should be
// mistaken for having one.
type ArenaRepo struct {
	pool *pgxpool.Pool
}

func NewArenaRepo(pool *pgxpool.Pool) *ArenaRepo { return &ArenaRepo{pool: pool} }

// ArenaListing is one row of the arena index: the venue plus the two figures
// the listing shows about it.
//
// Both are computed in SQL rather than by loading every court and reducing in
// Go, which is the N+1 this rewrite exists to avoid: one query returns every
// arena already carrying its court count and cheapest rate.
type ArenaListing struct {
	domain.Arena
	CourtCount   int
	FromPriceNPR int
	// Sports is the distinct set this venue offers, which is what the index
	// badges each row with. Aggregated in the same query rather than by
	// sending every court and reducing in the client -- the index needs the
	// set, not the courts.
	Sports []domain.Sport
}

// ArenaDetail is one venue with its courts and their pricing rules -- what the
// arena page renders in full.
type ArenaDetail struct {
	domain.Arena
	Courts []CourtWithRules
}

// CourtWithRules is a court and the rules that price it.
type CourtWithRules struct {
	domain.Court
	// Format is the venue's own name for the pitch ("5-a-side", "Full
	// court"). Empty when the owner has not set one, in which case the
	// transport layer derives a label from SideCount.
	Format       string
	PricingRules []domain.PricingRule
}

// LedgerCourt is one court with everything needed to project its day, for the
// city-wide grid on the home page.
type LedgerCourt struct {
	CourtWithRules
	ArenaID   uuid.UUID
	ArenaName string
	ArenaSlug string
	ArenaArea string
	OpensAt   domain.DayTime
	ClosesAt  domain.DayTime
}

const arenaColumns = `
	a.id, a.owner_id, a.name, a.slug, a.area, a.city, a.lat, a.lng,
	a.description, a.cover_url, a.amenities, a.phone,
	a.opens_at, a.closes_at, a.rating, a.review_count, a.is_active,
	a.created_at, a.updated_at`

func scanArena(row pgx.Row, extra ...any) (domain.Arena, error) {
	var (
		a                  domain.Arena
		opensAt, closesAt  pgtype.Time
		description, cover pgtype.Text
		phone              pgtype.Text
	)

	dest := []any{
		&a.ID, &a.OwnerID, &a.Name, &a.Slug, &a.Area, &a.City, &a.Lat, &a.Lng,
		&description, &cover, &a.Amenities, &phone,
		&opensAt, &closesAt, &a.Rating, &a.ReviewCount, &a.IsActive,
		&a.CreatedAt, &a.UpdatedAt,
	}
	dest = append(dest, extra...)

	if err := row.Scan(dest...); err != nil {
		return domain.Arena{}, err
	}

	a.Description = description.String
	a.CoverURL = cover.String
	a.Phone = phone.String
	a.OpensAt = dayTimeFromPg(opensAt)
	a.ClosesAt = dayTimeFromPg(closesAt)
	return a, nil
}

// List returns every active arena, with its court count and cheapest rate.
//
// Ordered by area then name so the index is stable between requests: an
// unordered query is free to return rows in whatever order the plan happens to
// produce, which makes a list appear to reshuffle itself on reload.
func (r *ArenaRepo) List(ctx context.Context) ([]ArenaListing, error) {
	const q = `
		select ` + arenaColumns + `,
			count(c.id) filter (where c.is_active) as court_count,
			coalesce(min(c.base_price) filter (where c.is_active), 0) as from_price,
			coalesce(
				array_agg(distinct c.sport::text) filter (where c.is_active),
				'{}'
			) as sports
		from arenas a
		left join courts c on c.arena_id = a.id
		where a.is_active
		group by a.id
		order by a.area, a.name`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, domain.Internal(err, "listing arenas")
	}
	defer rows.Close()

	var out []ArenaListing
	for rows.Next() {
		var (
			listing ArenaListing
			sports  []string
		)
		arena, err := scanArena(rows, &listing.CourtCount, &listing.FromPriceNPR, &sports)
		if err != nil {
			return nil, domain.Internal(err, "scanning arena listing")
		}
		listing.Arena = arena
		listing.Sports = make([]domain.Sport, 0, len(sports))
		for _, sport := range sports {
			listing.Sports = append(listing.Sports, domain.Sport(sport))
		}
		out = append(out, listing)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.Internal(err, "listing arenas")
	}
	return out, nil
}

// BySlug returns one arena with its courts and their pricing rules.
//
// Three statements in one batch, not three round trips: the arena, its courts,
// and every rule belonging to those courts. The rules are fetched for the
// whole arena in a single query and distributed in Go, rather than one query
// per court.
func (r *ArenaRepo) BySlug(ctx context.Context, slug string) (ArenaDetail, error) {
	const arenaSQL = `select ` + arenaColumns + ` from arenas a where a.slug = $1 and a.is_active`

	const courtsSQL = `
		select c.id, c.arena_id, c.label, c.sport, c.surface, c.side_count,
			c.base_price, c.is_active, c.format, c.created_at, c.updated_at
		from courts c
		join arenas a on a.id = c.arena_id
		where a.slug = $1 and c.is_active
		order by c.label`

	const rulesSQL = `
		select p.id, p.court_id, p.label, p.days, p.start_hour, p.end_hour,
			p.price_npr, p.is_peak, p.priority
		from pricing_rules p
		join courts c on c.id = p.court_id
		join arenas a on a.id = c.arena_id
		where a.slug = $1
		order by p.priority desc`

	batch := &pgx.Batch{}
	batch.Queue(arenaSQL, slug)
	batch.Queue(courtsSQL, slug)
	batch.Queue(rulesSQL, slug)

	results := r.pool.SendBatch(ctx, batch)
	defer results.Close()

	arena, err := scanArena(results.QueryRow())
	if noRows(err) {
		return ArenaDetail{}, domain.NotFound("No arena at that address.")
	}
	if err != nil {
		return ArenaDetail{}, domain.Internal(err, "loading arena %q", slug)
	}

	courtRows, err := results.Query()
	if err != nil {
		return ArenaDetail{}, domain.Internal(err, "loading courts for arena %q", slug)
	}

	// Indexed by court id so the rules below can be filed against their court
	// in one pass instead of a scan per rule.
	byID := map[uuid.UUID]int{}
	var courts []CourtWithRules
	for courtRows.Next() {
		var (
			c      CourtWithRules
			format pgtype.Text
		)
		err := courtRows.Scan(&c.ID, &c.ArenaID, &c.Label, &c.Sport, &c.Surface,
			&c.SideCount, &c.BasePriceNPR, &c.IsActive, &format, &c.CreatedAt, &c.UpdatedAt)
		if err != nil {
			courtRows.Close()
			return ArenaDetail{}, domain.Internal(err, "scanning court")
		}
		c.Format = format.String
		byID[c.ID] = len(courts)
		courts = append(courts, c)
	}
	if err := courtRows.Err(); err != nil {
		courtRows.Close()
		return ArenaDetail{}, domain.Internal(err, "loading courts for arena %q", slug)
	}
	courtRows.Close()

	ruleRows, err := results.Query()
	if err != nil {
		return ArenaDetail{}, domain.Internal(err, "loading pricing rules for arena %q", slug)
	}
	defer ruleRows.Close()

	for ruleRows.Next() {
		rule, err := scanPricingRule(ruleRows)
		if err != nil {
			return ArenaDetail{}, err
		}
		if i, ok := byID[rule.CourtID]; ok {
			courts[i].PricingRules = append(courts[i].PricingRules, rule)
		}
	}
	if err := ruleRows.Err(); err != nil {
		return ArenaDetail{}, domain.Internal(err, "loading pricing rules for arena %q", slug)
	}

	return ArenaDetail{Arena: arena, Courts: courts}, nil
}

// ListAreas returns the neighbourhoods that currently have an active court, so
// the filter never offers an area that would return nothing.
func (r *ArenaRepo) ListAreas(ctx context.Context) ([]string, error) {
	const q = `
		select distinct a.area
		from arenas a
		join courts c on c.arena_id = a.id and c.is_active
		where a.is_active
		order by a.area`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, domain.Internal(err, "listing areas")
	}
	defer rows.Close()

	var areas []string
	for rows.Next() {
		var area string
		if err := rows.Scan(&area); err != nil {
			return nil, domain.Internal(err, "scanning area")
		}
		areas = append(areas, area)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.Internal(err, "listing areas")
	}
	return areas, nil
}

// LedgerCourts returns every active court matching a sport and area, with the
// arena hours and pricing rules needed to project a day for each.
//
// This is the read behind the city-wide grid. Two queries regardless of how
// many courts match -- one for the courts, one for every rule belonging to
// them -- because the alternative is a rules query per court, and the home
// page renders every court in the city.
//
// An empty sport or area means "all", which is the filter's default rather
// than a special case the caller has to encode.
func (r *ArenaRepo) LedgerCourts(ctx context.Context, sport domain.Sport, area string) ([]LedgerCourt, error) {
	const courtsSQL = `
		select c.id, c.arena_id, c.label, c.sport, c.surface, c.side_count,
			c.base_price, c.is_active, c.format, c.created_at, c.updated_at,
			a.id, a.name, a.slug, a.area, a.opens_at, a.closes_at
		from courts c
		join arenas a on a.id = c.arena_id
		where c.is_active and a.is_active
			and ($1 = '' or c.sport::text = $1)
			and ($2 = '' or a.area = $2)
		order by a.area, a.name, c.label`

	const rulesSQL = `
		select p.id, p.court_id, p.label, p.days, p.start_hour, p.end_hour,
			p.price_npr, p.is_peak, p.priority
		from pricing_rules p
		join courts c on c.id = p.court_id
		join arenas a on a.id = c.arena_id
		where c.is_active and a.is_active
			and ($1 = '' or c.sport::text = $1)
			and ($2 = '' or a.area = $2)
		order by p.priority desc`

	batch := &pgx.Batch{}
	batch.Queue(courtsSQL, string(sport), area)
	batch.Queue(rulesSQL, string(sport), area)

	results := r.pool.SendBatch(ctx, batch)
	defer results.Close()

	courtRows, err := results.Query()
	if err != nil {
		return nil, domain.Internal(err, "loading ledger courts")
	}

	byID := map[uuid.UUID]int{}
	var courts []LedgerCourt
	for courtRows.Next() {
		var (
			lc                LedgerCourt
			format            pgtype.Text
			opensAt, closesAt pgtype.Time
		)
		err := courtRows.Scan(&lc.ID, &lc.ArenaID, &lc.Label, &lc.Sport, &lc.Surface,
			&lc.SideCount, &lc.BasePriceNPR, &lc.IsActive, &format, &lc.CreatedAt, &lc.UpdatedAt,
			&lc.ArenaID, &lc.ArenaName, &lc.ArenaSlug, &lc.ArenaArea, &opensAt, &closesAt)
		if err != nil {
			courtRows.Close()
			return nil, domain.Internal(err, "scanning ledger court")
		}
		lc.Format = format.String
		lc.OpensAt = dayTimeFromPg(opensAt)
		lc.ClosesAt = dayTimeFromPg(closesAt)
		byID[lc.ID] = len(courts)
		courts = append(courts, lc)
	}
	if err := courtRows.Err(); err != nil {
		courtRows.Close()
		return nil, domain.Internal(err, "loading ledger courts")
	}
	courtRows.Close()

	ruleRows, err := results.Query()
	if err != nil {
		return nil, domain.Internal(err, "loading ledger pricing rules")
	}
	defer ruleRows.Close()

	for ruleRows.Next() {
		rule, err := scanPricingRule(ruleRows)
		if err != nil {
			return nil, err
		}
		if i, ok := byID[rule.CourtID]; ok {
			courts[i].PricingRules = append(courts[i].PricingRules, rule)
		}
	}
	if err := ruleRows.Err(); err != nil {
		return nil, domain.Internal(err, "loading ledger pricing rules")
	}

	return courts, nil
}

// scanPricingRule reads one rule, converting Postgres's ISO weekday numbers to
// Go's.
//
// A day outside 1-7 means corrupt data. Skipping it silently would misprice
// the slot -- quietly, and in the customer's favour or ours depending on the
// rule -- so it refuses to price at all.
func scanPricingRule(row pgx.Row) (domain.PricingRule, error) {
	var (
		rule    domain.PricingRule
		isoDays []int32
	)

	err := row.Scan(&rule.ID, &rule.CourtID, &rule.Label, &isoDays,
		&rule.StartHour, &rule.EndHour, &rule.PriceNPR, &rule.IsPeak, &rule.Priority)
	if err != nil {
		return domain.PricingRule{}, domain.Internal(err, "scanning pricing rule")
	}

	rule.Days = make([]time.Weekday, 0, len(isoDays))
	for _, iso := range isoDays {
		day, err := domain.WeekdayFromISO(int(iso))
		if err != nil {
			return domain.PricingRule{}, domain.Internal(err,
				"pricing rule %s has an invalid weekday %d", rule.ID, iso)
		}
		rule.Days = append(rule.Days, day)
	}
	return rule, nil
}
