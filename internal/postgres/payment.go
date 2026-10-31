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

type PaymentRepo struct {
	pool     *pgxpool.Pool
	bookings *BookingRepo
}

func NewPaymentRepo(pool *pgxpool.Pool, bookings *BookingRepo) *PaymentRepo {
	return &PaymentRepo{pool: pool, bookings: bookings}
}

const paymentColumns = `
	id, booking_id, provider, amount_npr, status,
	transaction_uuid, coalesce(provider_ref, ''), raw_response,
	created_at, updated_at, verified_at`

func scanPayment(row pgx.Row) (domain.Payment, error) {
	var (
		p   domain.Payment
		raw pgtype.Text
	)
	err := row.Scan(&p.ID, &p.BookingID, &p.Provider, &p.AmountNPR, &p.Status,
		&p.TransactionUUID, &p.ProviderRef, &raw,
		&p.CreatedAt, &p.UpdatedAt, &p.VerifiedAt)
	if err != nil {
		return domain.Payment{}, err
	}
	if raw.Valid {
		p.RawResponse = []byte(raw.String)
	}
	return p, nil
}

// Create records a new payment intent.
//
// transaction_uuid is unique in the schema, which is what stops two intents
// ever sharing the identifier we hand a gateway -- if that could happen, one
// gateway callback could be applied to the wrong booking.
func (r *PaymentRepo) Create(ctx context.Context, p domain.Payment) (domain.Payment, error) {
	const q = `
		insert into payments (booking_id, provider, amount_npr, status, transaction_uuid)
		values ($1, $2, $3, $4, $5)
		returning ` + paymentColumns

	out, err := scanPayment(r.pool.QueryRow(ctx, q,
		p.BookingID, p.Provider, p.AmountNPR, p.Status, p.TransactionUUID))
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Payment{}, domain.Conflict("That payment has already been started.")
		}
		return domain.Payment{}, domain.Internal(err, "creating payment for booking %s", p.BookingID)
	}
	return out, nil
}

// ByTransactionUUID loads a payment by the identifier we gave the gateway.
//
// This is the lookup a callback arrives with: the gateway tells us which of
// our transactions it is talking about, and we go and find it. It is
// deliberately not a lookup by anything the gateway chose, so a callback
// naming a record we never created finds nothing.
func (r *PaymentRepo) ByTransactionUUID(ctx context.Context, transactionUUID string) (domain.Payment, error) {
	const q = `select ` + paymentColumns + ` from payments where transaction_uuid = $1`

	p, err := scanPayment(r.pool.QueryRow(ctx, q, transactionUUID))
	if noRows(err) {
		return domain.Payment{}, domain.NotFound("We have no record of that payment.")
	}
	if err != nil {
		return domain.Payment{}, domain.Internal(err, "loading payment %s", transactionUUID)
	}
	return p, nil
}

// LatestForBooking returns the most recent payment attempt on a booking, so a
// player who abandoned a checkout and came back is not charged twice for the
// same hour.
func (r *PaymentRepo) LatestForBooking(ctx context.Context, bookingID uuid.UUID) (domain.Payment, error) {
	const q = `
		select ` + paymentColumns + `
		from payments where booking_id = $1
		order by created_at desc limit 1`

	p, err := scanPayment(r.pool.QueryRow(ctx, q, bookingID))
	if noRows(err) {
		return domain.Payment{}, domain.NotFound("No payment has been started for that booking.")
	}
	if err != nil {
		return domain.Payment{}, domain.Internal(err, "loading payments for booking %s", bookingID)
	}
	return p, nil
}

// Settle writes the outcome of a verified payment and, when it succeeded,
// confirms the booking -- both in one transaction.
//
// The atomicity is the whole point. A payment recorded as verified while its
// booking stays pending is a player who has been charged and whose hour will
// be released by the janitor fifteen minutes later; a booking confirmed
// against a payment we failed to record is an hour given away for free. There
// is no ordering of two separate writes that avoids both, so they are one
// write.
//
// The row is locked FOR UPDATE and re-read inside the transaction rather than
// trusted from the caller's copy: gateways retry, and two callbacks for the
// same payment can arrive at once. The second finds the status already
// settled and does nothing.
func (r *PaymentRepo) Settle(ctx context.Context, p domain.Payment, confirmBooking bool) error {
	return InTx(ctx, r.pool, func(tx pgx.Tx) error {
		const lockSQL = `select status from payments where id = $1 for update`

		var current domain.PaymentStatus
		if err := tx.QueryRow(ctx, lockSQL, p.ID).Scan(&current); err != nil {
			if noRows(err) {
				return domain.NotFound("We have no record of that payment.")
			}
			return domain.Internal(err, "locking payment %s", p.ID)
		}
		if current.IsSettled() {
			// A concurrent callback got here first. Its outcome stands.
			return nil
		}

		const updateSQL = `
			update payments
			   set status = $2, provider_ref = $3, raw_response = $4, verified_at = $5
			 where id = $1`

		var raw any
		if len(p.RawResponse) > 0 {
			raw = string(p.RawResponse)
		}
		if _, err := tx.Exec(ctx, updateSQL, p.ID, p.Status, p.ProviderRef, raw, p.VerifiedAt); err != nil {
			return domain.Internal(err, "settling payment %s", p.ID)
		}

		if !confirmBooking {
			return nil
		}
		return r.bookings.Confirm(ctx, tx, p.BookingID)
	})
}

// BookCash books the hour for a player paying cash at the venue.
//
// The booking is confirmed straight away -- the venue does not approve it --
// and the payment is recorded as `initiated`: the hour is theirs, the money
// is still owed. The venue settles it later through
// MarkCashReceivedOwnerScoped. Both writes happen in one transaction, so there
// is never a confirmed cash booking with no payment on file for the venue to
// collect, nor a cash payment on a hold that lapsed.
//
// Idempotent: asking twice returns the payment already on file rather than
// recording a second debt for the same hour.
func (r *PaymentRepo) BookCash(ctx context.Context, p domain.Payment) (domain.Payment, error) {
	var out domain.Payment

	err := InTx(ctx, r.pool, func(tx pgx.Tx) error {
		// Lock the booking first: the janitor releasing this hold is the one
		// thing that can race this, and it updates the same row.
		const lockSQL = `
			select status, hold_expires_at > now()
			  from bookings where id = $1 for update`
		var status domain.BookingStatus
		var live *bool
		if err := tx.QueryRow(ctx, lockSQL, p.BookingID).Scan(&status, &live); err != nil {
			if noRows(err) {
				return domain.NotFound("That booking doesn't exist.")
			}
			return domain.Internal(err, "locking booking %s", p.BookingID)
		}

		const existingSQL = `
			select ` + paymentColumns + `
			  from payments
			 where booking_id = $1 and provider = 'cash' and status = 'initiated'
			 order by created_at desc limit 1`
		existing, err := scanPayment(tx.QueryRow(ctx, existingSQL, p.BookingID))
		switch {
		case err == nil && status == domain.BookingConfirmed:
			out = existing // already booked; a double tap
			return nil
		case err != nil && !noRows(err):
			return domain.Internal(err, "looking up cash payment for booking %s", p.BookingID)
		}

		if status != domain.BookingPending || live == nil || !*live {
			return domain.Conflict("This hold has lapsed. Pick the hour again.")
		}

		if err == nil {
			// A request left over from when venues approved cash bookings:
			// confirm it now rather than recording a second payment.
			out = existing
		} else {
			const insertSQL = `
				insert into payments (booking_id, provider, amount_npr, status, transaction_uuid)
				values ($1, $2, $3, $4, $5)
				returning ` + paymentColumns
			out, err = scanPayment(tx.QueryRow(ctx, insertSQL,
				p.BookingID, p.Provider, p.AmountNPR, p.Status, p.TransactionUUID))
			if err != nil {
				return domain.Internal(err, "recording cash payment for booking %s", p.BookingID)
			}
		}

		return confirmCashBooking(ctx, tx, p.BookingID)
	})

	return out, err
}

// lockOwnedCashPayment locks a cash payment and proves the caller owns the
// arena it was taken at, in one statement, so nothing can change between the
// check and the write that follows it.
//
// The ownership predicate is in the SQL rather than a prior check: these are
// the writes where a missing predicate means any owner can approve, decline
// or settle any booking at any venue.
func lockOwnedCashPayment(ctx context.Context, tx pgx.Tx, paymentID, ownerID uuid.UUID) (domain.Payment, error) {
	const lockSQL = `
		select p.id, p.booking_id, p.provider, p.amount_npr, p.status,
			p.transaction_uuid, coalesce(p.provider_ref, ''), p.raw_response,
			p.created_at, p.updated_at, p.verified_at
		from payments p
		join bookings b on b.id = p.booking_id
		join courts  c on c.id = b.court_id
		join arenas  a on a.id = c.arena_id
		where p.id = $1 and a.owner_id = $2
		for update of p, b`

	p, err := scanPayment(tx.QueryRow(ctx, lockSQL, paymentID, ownerID))
	if noRows(err) {
		return domain.Payment{}, domain.NotFound("No payment of yours with that id.")
	}
	if err != nil {
		return domain.Payment{}, domain.Internal(err, "locking payment %s", paymentID)
	}
	if p.Provider != domain.ProviderCash {
		// A gateway payment is settled by the gateway. Letting an owner
		// touch one by hand would be a way to confirm a booking nobody paid
		// for.
		return domain.Payment{}, domain.Conflict("That payment is settled through %s, not at the arena.", p.Provider)
	}
	return p, nil
}

// confirmCashBooking confirms the booking behind a cash payment.
//
// Unlike a gateway confirmation it accepts a booking that is already
// confirmed or played: cash is approved before the game and collected after
// it, by which time the janitor may have marked the hour completed.
func confirmCashBooking(ctx context.Context, tx pgx.Tx, bookingID uuid.UUID) error {
	const confirmSQL = `
		update bookings
		   set status = 'confirmed', hold_expires_at = null
		 where id = $1 and status = 'pending' and hold_expires_at > now()`

	tag, err := tx.Exec(ctx, confirmSQL, bookingID)
	if err != nil {
		if isExclusionViolation(err) {
			return domain.Conflict("Someone else has booked that hour since this request was made.").WithCause(err)
		}
		return domain.Internal(err, "confirming booking %s", bookingID)
	}
	if tag.RowsAffected() == 1 {
		return nil
	}

	var status domain.BookingStatus
	if err := tx.QueryRow(ctx, `select status from bookings where id = $1`, bookingID).Scan(&status); err != nil {
		return domain.Internal(err, "inspecting booking %s", bookingID)
	}
	switch status {
	case domain.BookingConfirmed, domain.BookingCompleted:
		return nil // already approved
	case domain.BookingPending:
		return domain.Conflict("This request lapsed when the game started.")
	default:
		return domain.Conflict("This booking was %s, so there is nothing to approve.", status)
	}
}

// ApproveCashOwnerScoped is the venue accepting a pay-at-the-venue booking.
//
// Cash bookings are now confirmed the moment the player books (see BookCash),
// so this only matters for requests made before that change that are still
// waiting.
//
// The booking is confirmed -- the hour is now theirs and stops lapsing -- but
// the payment stays `initiated`: no money has changed hands yet. That happens
// after the game, through MarkCashReceivedOwnerScoped.
func (r *PaymentRepo) ApproveCashOwnerScoped(ctx context.Context, paymentID, ownerID uuid.UUID) (domain.Payment, error) {
	var out domain.Payment

	err := InTx(ctx, r.pool, func(tx pgx.Tx) error {
		p, err := lockOwnedCashPayment(ctx, tx, paymentID, ownerID)
		if err != nil {
			return err
		}
		if p.Status != domain.PaymentInitiated && p.Status != domain.PaymentVerified {
			return domain.Conflict("This request has already been closed.")
		}
		out = p
		return confirmCashBooking(ctx, tx, p.BookingID)
	})

	return out, err
}

// DeclineCashOwnerScoped is the venue turning a cash request down. The hold
// is released at once, so the hour goes straight back on the board, and the
// payment intent is closed so it stops asking for attention.
//
// Only a request that is still waiting can be declined. An approved booking is
// a promise to the player; withdrawing it is a conversation, not a button.
func (r *PaymentRepo) DeclineCashOwnerScoped(ctx context.Context, paymentID, ownerID uuid.UUID) (domain.Payment, error) {
	var out domain.Payment

	err := InTx(ctx, r.pool, func(tx pgx.Tx) error {
		p, err := lockOwnedCashPayment(ctx, tx, paymentID, ownerID)
		if err != nil {
			return err
		}
		if p.Status == domain.PaymentFailed {
			out = p
			return nil // already declined
		}
		if p.Status != domain.PaymentInitiated {
			return domain.Conflict("This booking has already been paid for.")
		}

		const cancelSQL = `
			update bookings
			   set status = 'cancelled', hold_expires_at = null
			 where id = $1 and status = 'pending'`
		tag, err := tx.Exec(ctx, cancelSQL, p.BookingID)
		if err != nil {
			return domain.Internal(err, "declining booking %s", p.BookingID)
		}
		if tag.RowsAffected() == 0 {
			return domain.Conflict("Only a booking still waiting for approval can be declined.")
		}

		const failSQL = `update payments set status = 'failed' where id = $1`
		if _, err := tx.Exec(ctx, failSQL, p.ID); err != nil {
			return domain.Internal(err, "closing cash request %s", p.ID)
		}

		const closePostSQL = `
			update matchmaking_posts set status = 'cancelled'
			 where booking_id = $1 and status <> 'cancelled'`
		if _, err := tx.Exec(ctx, closePostSQL, p.BookingID); err != nil {
			return domain.Internal(err, "closing community post for booking %s", p.BookingID)
		}

		p.Status = domain.PaymentFailed
		out = p
		return nil
	})

	return out, err
}

// MarkCashReceivedOwnerScoped settles a cash payment on the say-so of the
// arena that took the money.
//
// This is the counterpart to `payment.Cash` refusing to verify anything by
// itself. A gateway confirms a payment because we asked it; cash is confirmed
// because the venue says the player handed over the notes, and the venue is
// the only party in a position to know.
//
// Usually called after the game, on a booking the venue approved earlier. It
// also works on a request nobody approved yet -- a player who walks in and
// pays up front -- in which case it approves and settles at once.
//
// Idempotent, like the gateway path: a second confirmation of the same
// payment is a double click, not a second payment.
func (r *PaymentRepo) MarkCashReceivedOwnerScoped(ctx context.Context, paymentID, ownerID uuid.UUID, now time.Time) (domain.Payment, error) {
	var settled domain.Payment

	err := InTx(ctx, r.pool, func(tx pgx.Tx) error {
		p, err := lockOwnedCashPayment(ctx, tx, paymentID, ownerID)
		if err != nil {
			return err
		}
		if p.Status.IsSettled() {
			settled = p
			return nil // already done
		}

		if err := confirmCashBooking(ctx, tx, p.BookingID); err != nil {
			return err
		}

		p.Status = domain.PaymentVerified
		p.VerifiedAt = &now

		const updateSQL = `update payments set status = $2, verified_at = $3 where id = $1`
		if _, err := tx.Exec(ctx, updateSQL, p.ID, p.Status, p.VerifiedAt); err != nil {
			return domain.Internal(err, "settling cash payment %s", p.ID)
		}

		settled = p
		return nil
	})

	return settled, err
}

// ListForArenaOwner returns payments taken at a venue the caller owns, newest
// first -- the reconciliation view.
func (r *PaymentRepo) ListForArenaOwner(ctx context.Context, arenaID, ownerID uuid.UUID, limit int) ([]OwnerPayment, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	const q = `
		select p.id, p.booking_id, p.provider, p.amount_npr, p.status,
			p.transaction_uuid, coalesce(p.provider_ref, ''), p.raw_response,
			p.created_at, p.updated_at, p.verified_at,
			b.slot, b.status, c.label, u.full_name, u.username
		from payments p
		join bookings b on b.id = p.booking_id
		join courts  c on c.id = b.court_id
		join arenas  a on a.id = c.arena_id
		join users   u on u.id = b.user_id
		where a.id = $1 and a.owner_id = $2
		order by p.created_at desc
		limit $3`

	rows, err := r.pool.Query(ctx, q, arenaID, ownerID, limit)
	if err != nil {
		return nil, domain.Internal(err, "listing payments for arena %s", arenaID)
	}
	defer rows.Close()

	var out []OwnerPayment
	for rows.Next() {
		var (
			op   OwnerPayment
			raw  pgtype.Text
			slot pgtype.Range[pgtype.Timestamptz]
		)
		err := rows.Scan(&op.ID, &op.BookingID, &op.Provider, &op.AmountNPR, &op.Status,
			&op.TransactionUUID, &op.ProviderRef, &raw,
			&op.CreatedAt, &op.UpdatedAt, &op.VerifiedAt,
			&slot, &op.BookingStatus, &op.CourtLabel, &op.PlayerName, &op.PlayerUsername)
		if err != nil {
			return nil, domain.Internal(err, "scanning owner payment")
		}
		if raw.Valid {
			op.RawResponse = []byte(raw.String)
		}
		op.Slot = domain.Slot{Start: slot.Lower.Time.UTC(), End: slot.Upper.Time.UTC()}
		out = append(out, op)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.Internal(err, "listing payments for arena %s", arenaID)
	}
	return out, nil
}

// OwnerPayment is a payment with the context an arena needs to reconcile it:
// which hour, which court, and who booked it.
type OwnerPayment struct {
	domain.Payment
	Slot           domain.Slot
	BookingStatus  domain.BookingStatus
	CourtLabel     string
	PlayerName     string
	PlayerUsername string
}
