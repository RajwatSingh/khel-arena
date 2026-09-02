package postgres

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

// Owner-facing writes.
//
// Every statement here is scoped by owner in its WHERE clause rather than by a
// check the caller makes first. The service still loads and checks ownership
// for the sake of a good error message, but that check is not what enforces
// this: a read-then-write leaves a window, and these are the writes where the
// window means one arena's owner editing another's courts.
//
// `OwnerScoped` in the names is a reminder, not decoration -- an UPDATE here
// without an owner predicate is a privilege escalation.

// unaliasedArenaColumns is arenaColumns without the `a.` qualifier, for the
// INSERT, which has no table alias to qualify against.
//
// Two spellings of one list is a wart, and the alternative is worse: aliasing
// every read query's table to nothing, or writing the columns out a third time
// inside each statement.
var unaliasedArenaColumns = strings.ReplaceAll(arenaColumns, "a.", "")

// CreateArena registers a venue owned by the caller.
func (r *ArenaRepo) CreateArena(ctx context.Context, a domain.Arena) (domain.Arena, error) {
	q := `
		insert into arenas (
			owner_id, name, slug, area, city, lat, lng, description,
			cover_url, amenities, phone, opens_at, closes_at
		)
		values ($1, $2, $3, $4, $5, $6, $7, nullif($8, ''), nullif($9, ''), $10,
			nullif($11, ''), $12, $13)
		returning ` + unaliasedArenaColumns

	arena, err := scanArena(r.pool.QueryRow(ctx, q,
		a.OwnerID, a.Name, a.Slug, a.Area, a.City, a.Lat, a.Lng, a.Description,
		a.CoverURL, a.Amenities, a.Phone, dayTimeToPg(a.OpensAt), dayTimeToPg(a.ClosesAt)))
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Arena{}, domain.Conflict("An arena already uses that web address.")
		}
		return domain.Arena{}, domain.Internal(err, "creating arena %q", a.Name)
	}
	return arena, nil
}

// UpdateArenaOwnerScoped edits a venue, and only if the caller owns it.
//
// The slug is deliberately not updatable. It is in every link anyone has
// shared, and changing it silently breaks all of them -- that wants a redirect
// table and a deliberate decision, not a field on an edit form.
func (r *ArenaRepo) UpdateArenaOwnerScoped(ctx context.Context, arenaID, ownerID uuid.UUID, a domain.Arena) (domain.Arena, error) {
	// Aliased so the shared column list, which is written against `a.`, has
	// something to qualify against.
	const q = `
		update arenas a set
			name = $3, area = $4, city = $5, lat = $6, lng = $7,
			description = nullif($8, ''), cover_url = nullif($9, ''),
			amenities = $10, phone = nullif($11, ''),
			opens_at = $12, closes_at = $13
		where a.id = $1 and a.owner_id = $2
		returning ` + arenaColumns

	arena, err := scanArena(r.pool.QueryRow(ctx, q,
		arenaID, ownerID, a.Name, a.Area, a.City, a.Lat, a.Lng, a.Description,
		a.CoverURL, a.Amenities, a.Phone, dayTimeToPg(a.OpensAt), dayTimeToPg(a.ClosesAt)))
	if noRows(err) {
		// No row matched: either it does not exist or it belongs to somebody
		// else. Both answer the same way, so this cannot be used to discover
		// which arenas exist.
		return domain.Arena{}, domain.NotFound("No arena of yours at that address.")
	}
	if err != nil {
		return domain.Arena{}, domain.Internal(err, "updating arena %s", arenaID)
	}
	return arena, nil
}

// SetArenaActiveOwnerScoped opens or closes a venue.
//
// Deactivating hides it from every listing and stops new bookings, and leaves
// bookings already taken alone: people have plans, and a venue going quiet on
// the site is not the same as cancelling everyone's Saturday.
func (r *ArenaRepo) SetArenaActiveOwnerScoped(ctx context.Context, arenaID, ownerID uuid.UUID, active bool) error {
	const q = `update arenas set is_active = $3 where id = $1 and owner_id = $2`

	tag, err := r.pool.Exec(ctx, q, arenaID, ownerID, active)
	if err != nil {
		return domain.Internal(err, "setting arena %s active=%v", arenaID, active)
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("No arena of yours at that address.")
	}
	return nil
}

// ListArenasForOwner returns the venues a user owns, including inactive ones:
// this is the management view, not the public index.
func (r *ArenaRepo) ListArenasForOwner(ctx context.Context, ownerID uuid.UUID) ([]ArenaListing, error) {
	const q = `
		select ` + arenaColumns + `,
			count(c.id) filter (where c.is_active) as court_count,
			coalesce(min(c.base_price) filter (where c.is_active), 0) as from_price,
			coalesce(array_agg(distinct c.sport::text) filter (where c.is_active), '{}') as sports
		from arenas a
		left join courts c on c.arena_id = a.id
		where a.owner_id = $1
		group by a.id
		order by a.name`

	rows, err := r.pool.Query(ctx, q, ownerID)
	if err != nil {
		return nil, domain.Internal(err, "listing arenas for owner %s", ownerID)
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
			return nil, domain.Internal(err, "scanning owner arena")
		}
		listing.Arena = arena
		listing.Sports = make([]domain.Sport, 0, len(sports))
		for _, s := range sports {
			listing.Sports = append(listing.Sports, domain.Sport(s))
		}
		out = append(out, listing)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.Internal(err, "listing arenas for owner %s", ownerID)
	}
	return out, nil
}

// OwnsArena reports whether a user owns a venue, for the checks a service
// makes before doing something that is not a single owner-scoped statement.
func (r *ArenaRepo) OwnsArena(ctx context.Context, arenaID, ownerID uuid.UUID) (bool, error) {
	const q = `select exists (select 1 from arenas where id = $1 and owner_id = $2)`

	var owns bool
	if err := r.pool.QueryRow(ctx, q, arenaID, ownerID).Scan(&owns); err != nil {
		return false, domain.Internal(err, "checking ownership of arena %s", arenaID)
	}
	return owns, nil
}

// ---------------------------------------------------------------------------
// Courts
// ---------------------------------------------------------------------------

const courtColumns = `
	c.id, c.arena_id, c.label, c.sport, c.surface, c.side_count,
	c.base_price, c.is_active, c.format, c.created_at, c.updated_at`

func scanCourt(row pgx.Row) (CourtWithRules, error) {
	var (
		c      CourtWithRules
		format pgtype.Text
	)
	err := row.Scan(&c.ID, &c.ArenaID, &c.Label, &c.Sport, &c.Surface,
		&c.SideCount, &c.BasePriceNPR, &c.IsActive, &format, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return CourtWithRules{}, err
	}
	c.Format = format.String
	return c, nil
}

// CreateCourtOwnerScoped adds a court, and only to an arena the caller owns.
//
// The ownership predicate is in the INSERT itself: the arena_id is taken from
// a SELECT that already filters by owner, so an arena somebody else owns
// yields no row to insert against rather than a court on their pitch.
func (r *ArenaRepo) CreateCourtOwnerScoped(ctx context.Context, ownerID uuid.UUID, c domain.Court, format string) (CourtWithRules, error) {
	// Columns are listed out rather than reusing courtColumns: that constant is
	// qualified with the `c.` alias the read queries use, and an INSERT has no
	// such alias to qualify against.
	const q = `
		insert into courts (arena_id, label, sport, surface, side_count, base_price, format)
		select a.id, $3, $4, $5, $6, $7, nullif($8, '')
		from arenas a
		where a.id = $1 and a.owner_id = $2
		returning id, arena_id, label, sport, surface, side_count,
			base_price, is_active, format, created_at, updated_at`

	court, err := scanCourt(r.pool.QueryRow(ctx, q,
		c.ArenaID, ownerID, c.Label, c.Sport, c.Surface, c.SideCount, c.BasePriceNPR, format))
	if noRows(err) {
		return CourtWithRules{}, domain.NotFound("No arena of yours at that address.")
	}
	if err != nil {
		if isUniqueViolation(err) {
			return CourtWithRules{}, domain.Conflict("That arena already has a court called %q.", c.Label)
		}
		return CourtWithRules{}, domain.Internal(err, "creating court on arena %s", c.ArenaID)
	}
	return court, nil
}

// UpdateCourtOwnerScoped edits a court belonging to an arena the caller owns.
func (r *ArenaRepo) UpdateCourtOwnerScoped(ctx context.Context, courtID, ownerID uuid.UUID, c domain.Court, format string) (CourtWithRules, error) {
	const q = `
		update courts set
			label = $3, sport = $4, surface = $5, side_count = $6,
			base_price = $7, format = nullif($8, '')
		from arenas a
		where courts.id = $1 and courts.arena_id = a.id and a.owner_id = $2
		returning courts.id, courts.arena_id, courts.label, courts.sport,
			courts.surface, courts.side_count, courts.base_price, courts.is_active,
			courts.format, courts.created_at, courts.updated_at`

	court, err := scanCourt(r.pool.QueryRow(ctx, q,
		courtID, ownerID, c.Label, c.Sport, c.Surface, c.SideCount, c.BasePriceNPR, format))
	if noRows(err) {
		return CourtWithRules{}, domain.NotFound("No court of yours with that id.")
	}
	if err != nil {
		if isUniqueViolation(err) {
			return CourtWithRules{}, domain.Conflict("That arena already has a court called %q.", c.Label)
		}
		return CourtWithRules{}, domain.Internal(err, "updating court %s", courtID)
	}
	return court, nil
}

// SetCourtActiveOwnerScoped retires or restores a court.
//
// Retiring rather than deleting. A court with bookings against it cannot be
// deleted -- the foreign key says so, and rightly: the history of who played
// where is not something an edit screen should be able to erase.
func (r *ArenaRepo) SetCourtActiveOwnerScoped(ctx context.Context, courtID, ownerID uuid.UUID, active bool) error {
	const q = `
		update courts set is_active = $3
		from arenas a
		where courts.id = $1 and courts.arena_id = a.id and a.owner_id = $2`

	tag, err := r.pool.Exec(ctx, q, courtID, ownerID, active)
	if err != nil {
		return domain.Internal(err, "setting court %s active=%v", courtID, active)
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("No court of yours with that id.")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Pricing rules
// ---------------------------------------------------------------------------

// CreatePricingRuleOwnerScoped adds a rate window to a court the caller owns.
func (r *ArenaRepo) CreatePricingRuleOwnerScoped(ctx context.Context, ownerID uuid.UUID, rule domain.PricingRule) (domain.PricingRule, error) {
	const q = `
		insert into pricing_rules (court_id, label, days, start_hour, end_hour, price_npr, is_peak, priority)
		select c.id, $3, $4, $5, $6, $7, $8, $9
		from courts c
		join arenas a on a.id = c.arena_id
		where c.id = $1 and a.owner_id = $2
		returning id, court_id, label, days, start_hour, end_hour, price_npr, is_peak, priority`

	created, err := scanPricingRule(r.pool.QueryRow(ctx, q,
		rule.CourtID, ownerID, rule.Label, isoDays(rule.Days),
		rule.StartHour, rule.EndHour, rule.PriceNPR, rule.IsPeak, rule.Priority))
	if noRows(err) {
		return domain.PricingRule{}, domain.NotFound("No court of yours with that id.")
	}
	if err != nil {
		return domain.PricingRule{}, err
	}
	return created, nil
}

// CopyPricingRulesOwnerScoped copies one court's rate card onto another.
//
// The tedious part of running a venue with five identical courts is setting
// the same four rate windows five times. This does it in one statement, and
// the statement is what carries the authorization: both courts are reached
// through a join to `arenas` filtered by owner, so a card cannot be copied
// from or onto somebody else's pitch.
//
// It appends rather than replaces. Two overlapping windows are already a
// situation the pricing rules handle -- highest priority wins, ties break to
// the narrower -- and silently deleting what an owner had set up would be a
// worse surprise than a duplicate they can remove.
func (r *ArenaRepo) CopyPricingRulesOwnerScoped(ctx context.Context, fromCourtID, toCourtID, ownerID uuid.UUID) (int, error) {
	if fromCourtID == toCourtID {
		return 0, domain.Invalid("from_court_id", "That's the same court.")
	}

	const q = `
		insert into pricing_rules (court_id, label, days, start_hour, end_hour, price_npr, is_peak, priority)
		select $2, p.label, p.days, p.start_hour, p.end_hour, p.price_npr, p.is_peak, p.priority
		from pricing_rules p
		join courts src on src.id = p.court_id
		join arenas sa  on sa.id = src.arena_id
		-- The destination is joined in as well, so a copy onto a court the
		-- caller does not own inserts nothing rather than inserting freely.
		join courts dst on dst.id = $2
		join arenas da  on da.id = dst.arena_id
		where p.court_id = $1 and sa.owner_id = $3 and da.owner_id = $3`

	tag, err := r.pool.Exec(ctx, q, fromCourtID, toCourtID, ownerID)
	if err != nil {
		return 0, domain.Internal(err, "copying pricing rules from court %s", fromCourtID)
	}
	return int(tag.RowsAffected()), nil
}

// DeletePricingRuleOwnerScoped removes a rate window.
//
// Deleted rather than deactivated, unlike a court: a rule holds no history.
// Removing it changes what future hours cost and nothing about hours already
// booked, whose price was resolved and written when the booking was made.
func (r *ArenaRepo) DeletePricingRuleOwnerScoped(ctx context.Context, ruleID, ownerID uuid.UUID) error {
	const q = `
		delete from pricing_rules p
		using courts c, arenas a
		where p.id = $1 and c.id = p.court_id and a.id = c.arena_id and a.owner_id = $2`

	tag, err := r.pool.Exec(ctx, q, ruleID, ownerID)
	if err != nil {
		return domain.Internal(err, "deleting pricing rule %s", ruleID)
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("No pricing rule of yours with that id.")
	}
	return nil
}

// isoDays converts Go weekdays to the ISO numbers the schema stores. The
// conversion lives in the domain; this just applies it per element.
func isoDays(days []time.Weekday) []int32 {
	out := make([]int32, 0, len(days))
	for _, d := range days {
		out = append(out, int32(domain.ISOWeekday(d)))
	}
	return out
}
