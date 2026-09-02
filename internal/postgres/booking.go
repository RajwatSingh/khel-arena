package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

// BookingRepo owns every query that touches bookings.
type BookingRepo struct {
	pool *pgxpool.Pool
}

func NewBookingRepo(pool *pgxpool.Pool) *BookingRepo { return &BookingRepo{pool: pool} }

// bookingColumns is named explicitly everywhere rather than using `select *`,
// so adding a column cannot silently change what a scan expects.
const bookingColumns = `
	id, court_id, user_id, team_id, slot, price_npr, is_peak,
	status, coalesce(note, ''), hold_expires_at, created_at, updated_at`

func scanBooking(row pgx.Row) (domain.Booking, error) {
	var b domain.Booking
	var slot pgtype.Range[pgtype.Timestamptz]
	var teamID *uuid.UUID
	var holdExpires *time.Time

	err := row.Scan(
		&b.ID, &b.CourtID, &b.UserID, &teamID, &slot, &b.PriceNPR, &b.IsPeak,
		&b.Status, &b.Note, &holdExpires, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		return domain.Booking{}, err
	}

	b.TeamID = teamID
	b.HoldExpiresAt = holdExpires
	b.Slot = domain.Slot{
		Start: slot.Lower.Time.UTC(),
		End:   slot.Upper.Time.UTC(),
	}
	return b, nil
}

// tstzrange builds the half-open [start, end) range Postgres stores, matching
// the convention domain.Slot uses.
func tstzrange(s domain.Slot) pgtype.Range[pgtype.Timestamptz] {
	return pgtype.Range[pgtype.Timestamptz]{
		Lower:     pgtype.Timestamptz{Time: s.Start, Valid: true},
		Upper:     pgtype.Timestamptz{Time: s.End, Valid: true},
		LowerType: pgtype.Inclusive,
		UpperType: pgtype.Exclusive,
		Valid:     true,
	}
}

// CreateHold writes a pending booking, or explains why the slot cannot be had.
//
// This is the replacement for the old `create_booking()` SECURITY DEFINER
// function, and it keeps that function's defence in depth:
//
//  1. An advisory lock keyed on court+start serialises simultaneous attempts,
//     so the second request waits for the first to finish rather than racing it
//     into a constraint violation.
//  2. Expired holds overlapping the slot are released first, so a lapsed hold
//     from an abandoned checkout cannot block a legitimate rebooking.
//  3. An explicit overlap check produces a message a player can act on.
//  4. The `no_double_booking` EXCLUDE constraint is the backstop. Steps 1-3 are
//     for the error message; this is the one that makes double-booking
//     impossible, and it holds even if this function is wrong.
//
// The price is passed in already resolved by the service from pricing rules.
// A price quoted by a client is never used.
func (r *BookingRepo) CreateHold(ctx context.Context, b domain.Booking) (domain.Booking, error) {
	var created domain.Booking

	err := InTx(ctx, r.pool, func(tx pgx.Tx) error {
		// 1. Serialise contenders for this exact court and start instant.
		// The lock is transaction-scoped, so it releases on commit or abort
		// with no unlock path to forget.
		const lockSQL = `select pg_advisory_xact_lock(hashtextextended($1, 42))`
		lockKey := b.CourtID.String() + "@" + b.Slot.Start.UTC().Format(time.RFC3339Nano)
		if _, err := tx.Exec(ctx, lockSQL, lockKey); err != nil {
			return fmt.Errorf("acquire slot lock: %w", err)
		}

		// 2. Release expired holds overlapping this slot. Only unpaid ones:
		// a hold with a verified payment against it is somebody's game.
		const releaseSQL = `
			update bookings b
			   set status = 'cancelled', hold_expires_at = null
			 where b.court_id = $1
			   and b.status = 'pending'
			   and b.slot && $2
			   and b.hold_expires_at <= now()
			   and not exists (
			     select 1 from payments p
			      where p.booking_id = b.id and p.status = 'verified'
			   )`
		if _, err := tx.Exec(ctx, releaseSQL, b.CourtID, tstzrange(b.Slot)); err != nil {
			return fmt.Errorf("release expired holds: %w", err)
		}

		// 3. Friendly pre-check, now that we hold the lock. `booking_blocks_slot`
		// is the same predicate the availability grid uses, so a slot shown as
		// free is a slot that will actually book.
		const conflictSQL = `
			select exists (
			  select 1 from bookings b
			   where b.court_id = $1
			     and b.slot && $2
			     and booking_blocks_slot(b.status, b.hold_expires_at)
			)`
		var taken bool
		if err := tx.QueryRow(ctx, conflictSQL, b.CourtID, tstzrange(b.Slot)).Scan(&taken); err != nil {
			return fmt.Errorf("check slot availability: %w", err)
		}
		if taken {
			return domain.Conflict("Someone took this slot moments before you. Please pick another time.")
		}

		// 4. The insert the EXCLUDE constraint guards unconditionally.
		const insertSQL = `
			insert into bookings
			  (court_id, user_id, team_id, slot, price_npr, is_peak, status, note, hold_expires_at)
			values ($1, $2, $3, $4, $5, $6, $7, nullif($8, ''), $9)
			returning ` + bookingColumns

		row := tx.QueryRow(ctx, insertSQL,
			b.CourtID, b.UserID, b.TeamID, tstzrange(b.Slot),
			b.PriceNPR, b.IsPeak, b.Status, b.Note, b.HoldExpiresAt)

		var err error
		created, err = scanBooking(row)
		if err != nil {
			switch {
			case isExclusionViolation(err):
				// Reached only if steps 1-3 are wrong. The constraint held
				// anyway, which is the entire point of it existing.
				return domain.Conflict("Someone took this slot moments before you. Please pick another time.")
			case isForeignKeyViolation(err):
				return domain.NotFound("That court is no longer available.")
			case isCheckViolation(err):
				return domain.Invalid("slot", "That booking isn't valid for this court.")
			}
			return fmt.Errorf("insert booking: %w", err)
		}
		return nil
	})

	if err != nil {
		if domain.CodeOf(err) != domain.CodeInternal {
			return domain.Booking{}, err
		}
		return domain.Booking{}, domain.Internal(err, "creating booking hold")
	}
	return created, nil
}

// ByID loads one booking.
func (r *BookingRepo) ByID(ctx context.Context, id uuid.UUID) (domain.Booking, error) {
	const q = `select ` + bookingColumns + ` from bookings where id = $1`

	b, err := scanBooking(r.pool.QueryRow(ctx, q, id))
	if noRows(err) {
		return domain.Booking{}, domain.NotFound("That booking doesn't exist.")
	}
	if err != nil {
		return domain.Booking{}, domain.Internal(err, "loading booking %s", id)
	}
	return b, nil
}

// Cancel releases a booking and takes down any community post riding on it.
//
// Ownership and the status guard are both in the UPDATE's WHERE clause, so
// the check and the write cannot drift apart the way a read-then-write can.
// The caller is told which of the two failed by re-reading only when the
// update matches nothing.
func (r *BookingRepo) Cancel(ctx context.Context, bookingID, userID uuid.UUID) error {
	return InTx(ctx, r.pool, func(tx pgx.Tx) error {
		const cancelSQL = `
			update bookings
			   set status = 'cancelled', hold_expires_at = null
			 where id = $1
			   and user_id = $2
			   and status in ('pending', 'confirmed')`

		tag, err := tx.Exec(ctx, cancelSQL, bookingID, userID)
		if err != nil {
			return domain.Internal(err, "cancelling booking %s", bookingID)
		}

		if tag.RowsAffected() == 0 {
			// Nothing changed. Work out why, without revealing the existence
			// of a booking that belongs to someone else.
			const lookupSQL = `select user_id, status from bookings where id = $1`
			var ownerID uuid.UUID
			var status domain.BookingStatus

			err := tx.QueryRow(ctx, lookupSQL, bookingID).Scan(&ownerID, &status)
			if noRows(err) || (err == nil && ownerID != userID) {
				return domain.NotFound("That booking doesn't exist.")
			}
			if err != nil {
				return domain.Internal(err, "inspecting booking %s", bookingID)
			}
			switch status {
			case domain.BookingCancelled:
				return domain.Conflict("This booking is already cancelled.")
			case domain.BookingCompleted:
				return domain.Conflict("This game has already been played.")
			default:
				return domain.Conflict("This booking can no longer be cancelled.")
			}
		}

		// Pull any community post that was advertising this booking.
		const closePostSQL = `
			update matchmaking_posts
			   set status = 'cancelled'
			 where booking_id = $1 and status <> 'cancelled'`
		if _, err := tx.Exec(ctx, closePostSQL, bookingID); err != nil {
			return domain.Internal(err, "closing community post for booking %s", bookingID)
		}
		return nil
	})
}

// Confirm promotes a paid hold to a confirmed booking. Clearing the expiry is
// what makes it permanent: a paid booking never lapses.
func (r *BookingRepo) Confirm(ctx context.Context, tx DB, bookingID uuid.UUID) error {
	const q = `
		update bookings
		   set status = 'confirmed', hold_expires_at = null
		 where id = $1 and status = 'pending'`

	tag, err := tx.Exec(ctx, q, bookingID)
	if err != nil {
		return domain.Internal(err, "confirming booking %s", bookingID)
	}
	if tag.RowsAffected() == 0 {
		// Either already confirmed (a replayed callback, which is fine) or
		// cancelled while the player was paying, which is not.
		const statusSQL = `select status from bookings where id = $1`
		var status domain.BookingStatus
		if err := tx.QueryRow(ctx, statusSQL, bookingID).Scan(&status); err != nil {
			if noRows(err) {
				return domain.NotFound("That booking doesn't exist.")
			}
			return domain.Internal(err, "inspecting booking %s", bookingID)
		}
		if status == domain.BookingConfirmed {
			return nil // idempotent
		}
		return domain.Conflict("This booking was %s before the payment completed.", status)
	}
	return nil
}

// ListForUser returns a player's bookings, newest first, with the court and
// arena context the list needs.
//
// The join is done here rather than by fetching bookings and then looking up
// each court: that shape is what made the previous version slow.
// MarkPlayed promotes confirmed bookings whose hour has passed.
//
// `completed` existed in the enum and nothing ever set it, which made it a
// status the system could describe but never reach. The janitor calls this
// alongside its other sweeps: a booking whose slot ended is played whether or
// not anything has got round to writing that down.
//
// Only `confirmed` moves. A pending hold that lapsed is the janitor's other
// job and becomes cancelled, not played -- nobody turned up to an hour nobody
// paid for.
func (r *BookingRepo) MarkPlayed(ctx context.Context) (int, error) {
	const q = `
		update bookings
		   set status = 'completed'
		 where status = 'confirmed' and upper(slot) <= now()`

	tag, err := r.pool.Exec(ctx, q)
	if err != nil {
		return 0, domain.Internal(err, "marking played bookings")
	}
	return int(tag.RowsAffected()), nil
}

func (r *BookingRepo) ListForUser(ctx context.Context, userID uuid.UUID, limit int) ([]domain.BookingDetail, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	const q = `
		select
			b.id, b.court_id, b.user_id, b.team_id, b.slot, b.price_npr, b.is_peak,
			b.status, coalesce(b.note, ''), b.hold_expires_at, b.created_at, b.updated_at,
			c.label, a.id, a.name, a.area, a.slug,
			(mp.id is not null and mp.status = 'open') as open_to_join
		from bookings b
		join courts c on c.id = b.court_id
		join arenas a on a.id = c.arena_id
		left join matchmaking_posts mp on mp.booking_id = b.id
		where b.user_id = $1
		order by b.created_at desc
		limit $2`

	rows, err := r.pool.Query(ctx, q, userID, limit)
	if err != nil {
		return nil, domain.Internal(err, "listing bookings for user %s", userID)
	}
	defer rows.Close()

	var out []domain.BookingDetail
	for rows.Next() {
		var d domain.BookingDetail
		var slot pgtype.Range[pgtype.Timestamptz]
		var teamID *uuid.UUID
		var holdExpires *time.Time

		err := rows.Scan(
			&d.ID, &d.CourtID, &d.UserID, &teamID, &slot, &d.PriceNPR, &d.IsPeak,
			&d.Status, &d.Note, &holdExpires, &d.CreatedAt, &d.UpdatedAt,
			&d.CourtLabel, &d.ArenaID, &d.ArenaName, &d.ArenaArea, &d.ArenaSlug,
			&d.OpenToJoin,
		)
		if err != nil {
			return nil, domain.Internal(err, "scanning booking row")
		}

		d.TeamID = teamID
		d.HoldExpiresAt = holdExpires
		d.Slot = domain.Slot{Start: slot.Lower.Time.UTC(), End: slot.Upper.Time.UTC()}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.Internal(err, "listing bookings for user %s", userID)
	}
	return out, nil
}

// ReleaseStaleHolds cancels every expired unpaid hold, fails its dangling
// payment intent, and takes down any community post tied to it.
//
// Availability is already correct without this -- an expired hold stops
// blocking the moment its window lapses, because `booking_blocks_slot` says
// so. This exists so a player's own list stops showing a booking they never
// completed, and so abandoned payment intents do not sit "initiated" forever.
func (r *BookingRepo) ReleaseStaleHolds(ctx context.Context) (int, error) {
	var released int

	err := InTx(ctx, r.pool, func(tx pgx.Tx) error {
		const releaseSQL = `
			update bookings b
			   set status = 'cancelled', hold_expires_at = null
			 where b.status = 'pending'
			   and b.hold_expires_at <= now()
			   and not exists (
			     select 1 from payments p
			      where p.booking_id = b.id and p.status = 'verified'
			   )
			returning b.id`

		rows, err := tx.Query(ctx, releaseSQL)
		if err != nil {
			return fmt.Errorf("release stale holds: %w", err)
		}

		var ids []uuid.UUID
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return fmt.Errorf("scan released hold: %w", err)
			}
			ids = append(ids, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("release stale holds: %w", err)
		}

		if len(ids) == 0 {
			return nil
		}
		released = len(ids)

		const failPaymentsSQL = `
			update payments set status = 'failed'
			 where booking_id = any($1) and status = 'initiated'`
		if _, err := tx.Exec(ctx, failPaymentsSQL, ids); err != nil {
			return fmt.Errorf("fail abandoned payment intents: %w", err)
		}

		const closePostsSQL = `
			update matchmaking_posts set status = 'cancelled'
			 where booking_id = any($1) and status <> 'cancelled'`
		if _, err := tx.Exec(ctx, closePostsSQL, ids); err != nil {
			return fmt.Errorf("close posts for released holds: %w", err)
		}
		return nil
	})

	if err != nil {
		return 0, domain.Internal(err, "releasing stale holds")
	}
	return released, nil
}
