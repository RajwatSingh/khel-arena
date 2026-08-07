package postgres

import (
	"time"

	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

// CourtContext is everything needed to price and validate a booking on one
// court: the court itself, its arena's operating hours, and its pricing rules.
//
// Loaded in a single round trip so the booking path does not make three
// sequential queries to learn three facts about the same court.
type CourtContext struct {
	Court        domain.Court
	ArenaID      uuid.UUID
	ArenaName    string
	ArenaSlug    string
	OpensAt      domain.DayTime
	ClosesAt     domain.DayTime
	ArenaActive  bool
	PricingRules []domain.PricingRule
}

// IsBookable reports whether the court and its arena are both open for business.
func (c CourtContext) IsBookable() bool { return c.Court.IsActive && c.ArenaActive }

// LoadCourtContext fetches a court, its arena's hours and its pricing rules in
// one batch -- two statements, one network round trip.
func (r *BookingRepo) LoadCourtContext(ctx context.Context, courtID uuid.UUID) (CourtContext, error) {
	const courtSQL = `
		select
			c.id, c.arena_id, c.label, c.sport, c.surface, c.side_count,
			c.base_price, c.is_active,
			a.name, a.slug, a.opens_at, a.closes_at, a.is_active
		from courts c
		join arenas a on a.id = c.arena_id
		where c.id = $1`

	const rulesSQL = `
		select id, court_id, label, days, start_hour, end_hour, price_npr, is_peak, priority
		from pricing_rules
		where court_id = $1
		order by priority desc`

	batch := &pgx.Batch{}
	batch.Queue(courtSQL, courtID)
	batch.Queue(rulesSQL, courtID)

	results := r.pool.SendBatch(ctx, batch)
	defer results.Close()

	var cc CourtContext
	var opensAt, closesAt pgtype.Time

	err := results.QueryRow().Scan(
		&cc.Court.ID, &cc.ArenaID, &cc.Court.Label, &cc.Court.Sport, &cc.Court.Surface,
		&cc.Court.SideCount, &cc.Court.BasePriceNPR, &cc.Court.IsActive,
		&cc.ArenaName, &cc.ArenaSlug, &opensAt, &closesAt, &cc.ArenaActive,
	)
	if noRows(err) {
		return CourtContext{}, domain.NotFound("That court doesn't exist.")
	}
	if err != nil {
		return CourtContext{}, domain.Internal(err, "loading court %s", courtID)
	}
	cc.Court.ArenaID = cc.ArenaID
	cc.OpensAt = dayTimeFromPg(opensAt)
	cc.ClosesAt = dayTimeFromPg(closesAt)

	rows, err := results.Query()
	if err != nil {
		return CourtContext{}, domain.Internal(err, "loading pricing rules for court %s", courtID)
	}
	defer rows.Close()

	for rows.Next() {
		var rule domain.PricingRule
		var isoDays []int32

		err := rows.Scan(&rule.ID, &rule.CourtID, &rule.Label, &isoDays,
			&rule.StartHour, &rule.EndHour, &rule.PriceNPR, &rule.IsPeak, &rule.Priority)
		if err != nil {
			return CourtContext{}, domain.Internal(err, "scanning pricing rule")
		}

		rule.Days = make([]time.Weekday, 0, len(isoDays))
		for _, iso := range isoDays {
			day, err := domain.WeekdayFromISO(int(iso))
			if err != nil {
				// A day outside 1-7 means corrupt data; skipping it silently
				// would misprice the slot, so refuse to price at all.
				return CourtContext{}, domain.Internal(err,
					"pricing rule %s has an invalid weekday %d", rule.ID, iso)
			}
			rule.Days = append(rule.Days, day)
		}
		cc.PricingRules = append(cc.PricingRules, rule)
	}
	if err := rows.Err(); err != nil {
		return CourtContext{}, domain.Internal(err, "loading pricing rules for court %s", courtID)
	}

	return cc, nil
}

// BookedRanges returns the slots on a court that are taken during a window.
//
// "Taken" is decided by `booking_blocks_slot`, the same predicate the create
// path checks, so a slot the grid shows as free is one that will actually
// book. Cancelled bookings and lapsed holds are absent by construction.
func (r *BookingRepo) BookedRanges(ctx context.Context, courtID uuid.UUID, window domain.Slot) ([]domain.Slot, error) {
	const q = `
		select slot
		from bookings
		where court_id = $1
		  and slot && $2
		  and booking_blocks_slot(status, hold_expires_at)
		order by lower(slot)`

	rows, err := r.pool.Query(ctx, q, courtID, tstzrange(window))
	if err != nil {
		return nil, domain.Internal(err, "loading booked ranges for court %s", courtID)
	}
	defer rows.Close()

	var out []domain.Slot
	for rows.Next() {
		var slot pgtype.Range[pgtype.Timestamptz]
		if err := rows.Scan(&slot); err != nil {
			return nil, domain.Internal(err, "scanning booked range")
		}
		out = append(out, domain.Slot{
			Start: slot.Lower.Time.UTC(),
			End:   slot.Upper.Time.UTC(),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, domain.Internal(err, "loading booked ranges for court %s", courtID)
	}
	return out, nil
}

func dayTimeFromPg(t pgtype.Time) domain.DayTime {
	if !t.Valid {
		return domain.DayTime{}
	}
	totalMinutes := int(t.Microseconds / 1_000_000 / 60)
	return domain.DayTime{Hour: totalMinutes / 60, Minute: totalMinutes % 60}
}
