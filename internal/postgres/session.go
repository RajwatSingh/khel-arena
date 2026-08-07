package postgres

import (
	"context"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

// SessionRepo stores refresh tokens and the single-use tokens behind password
// reset and email verification.
type SessionRepo struct {
	pool *pgxpool.Pool
}

func NewSessionRepo(pool *pgxpool.Pool) *SessionRepo { return &SessionRepo{pool: pool} }

// SessionContext is the optional provenance recorded against a session, so a
// user can recognise their own devices in a session list.
type SessionContext struct {
	UserAgent string
	IP        string
}

func (s SessionContext) addr() *netip.Addr {
	if s.IP == "" {
		return nil
	}
	addr, err := netip.ParseAddr(s.IP)
	if err != nil {
		return nil
	}
	return &addr
}

// StoreRefreshToken records a new session. Only the digest is written; the
// plaintext exists solely in the response to the client.
func (r *SessionRepo) StoreRefreshToken(ctx context.Context, userID uuid.UUID, digest []byte, expiresAt time.Time, sc SessionContext) (uuid.UUID, error) {
	const q = `
		insert into refresh_tokens (user_id, token_hash, expires_at, user_agent, ip)
		values ($1, $2, $3, nullif($4, ''), $5)
		returning id`

	var id uuid.UUID
	err := r.pool.QueryRow(ctx, q, userID, digest, expiresAt, sc.UserAgent, sc.addr()).Scan(&id)
	if err != nil {
		return uuid.Nil, domain.Internal(err, "storing refresh token for user %s", userID)
	}
	return id, nil
}

// RotateRefreshToken exchanges a valid refresh token for a new one.
//
// Rotation is single-use: presenting a token that was already exchanged means
// two parties hold it, which means one of them stole it. The whole chain of
// sessions for that user is then revoked, because there is no way to tell the
// thief from the victim -- and leaving the thief's session alive is worse than
// making the victim log in again.
//
// The old token is marked revoked and pointed at its replacement in the same
// transaction as the new one is written, so there is no instant where both are
// usable or neither is.
func (r *SessionRepo) RotateRefreshToken(ctx context.Context, oldDigest, newDigest []byte, expiresAt time.Time, sc SessionContext) (userID uuid.UUID, err error) {
	err = InTx(ctx, r.pool, func(tx pgx.Tx) error {
		// Lock the row so two refreshes with the same token cannot both win.
		const lookupSQL = `
			select id, user_id, expires_at, revoked_at
			from refresh_tokens
			where token_hash = $1
			for update`

		var (
			tokenID   uuid.UUID
			expires   time.Time
			revokedAt *time.Time
		)
		err := tx.QueryRow(ctx, lookupSQL, oldDigest).Scan(&tokenID, &userID, &expires, &revokedAt)
		if noRows(err) {
			return domain.Unauthenticated("Your session has expired. Please sign in again.")
		}
		if err != nil {
			return domain.Internal(err, "looking up refresh token")
		}

		if revokedAt != nil {
			// Reuse of a rotated token. Burn every session this user holds.
			const revokeAllSQL = `
				update refresh_tokens set revoked_at = now()
				 where user_id = $1 and revoked_at is null`
			if _, err := tx.Exec(ctx, revokeAllSQL, userID); err != nil {
				return domain.Internal(err, "revoking sessions after token reuse")
			}
			return domain.Unauthenticated("Your session has expired. Please sign in again.")
		}

		if !expires.After(time.Now()) {
			return domain.Unauthenticated("Your session has expired. Please sign in again.")
		}

		const insertSQL = `
			insert into refresh_tokens (user_id, token_hash, expires_at, user_agent, ip)
			values ($1, $2, $3, nullif($4, ''), $5)
			returning id`
		var newID uuid.UUID
		if err := tx.QueryRow(ctx, insertSQL, userID, newDigest, expiresAt, sc.UserAgent, sc.addr()).Scan(&newID); err != nil {
			return domain.Internal(err, "storing rotated refresh token")
		}

		const retireSQL = `
			update refresh_tokens
			   set revoked_at = now(), replaced_by = $2
			 where id = $1`
		if _, err := tx.Exec(ctx, retireSQL, tokenID, newID); err != nil {
			return domain.Internal(err, "retiring rotated refresh token")
		}
		return nil
	})

	if err != nil {
		return uuid.Nil, err
	}
	return userID, nil
}

// RevokeRefreshToken ends one session. Revoking an unknown or already-revoked
// token reports success: logging out is idempotent, and refusing tells a
// caller which tokens are real.
func (r *SessionRepo) RevokeRefreshToken(ctx context.Context, digest []byte) error {
	const q = `update refresh_tokens set revoked_at = now() where token_hash = $1 and revoked_at is null`
	if _, err := r.pool.Exec(ctx, q, digest); err != nil {
		return domain.Internal(err, "revoking refresh token")
	}
	return nil
}

// RevokeAllForUser ends every session a user holds.
func (r *SessionRepo) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	const q = `update refresh_tokens set revoked_at = now() where user_id = $1 and revoked_at is null`
	if _, err := r.pool.Exec(ctx, q, userID); err != nil {
		return domain.Internal(err, "revoking sessions for user %s", userID)
	}
	return nil
}

// DeleteExpiredTokens clears out rows nobody can use any more. Revoked rows
// are kept for a grace period so that reuse detection still has something to
// find when a stolen token is presented shortly after rotation.
func (r *SessionRepo) DeleteExpiredTokens(ctx context.Context, retainRevokedFor time.Duration) (int64, error) {
	const q = `
		delete from refresh_tokens
		 where expires_at < now()
		    or (revoked_at is not null and revoked_at < now() - $1::interval)`

	tag, err := r.pool.Exec(ctx, q, retainRevokedFor)
	if err != nil {
		return 0, domain.Internal(err, "deleting expired refresh tokens")
	}
	return tag.RowsAffected(), nil
}

// ---------------------------------------------------------------------------
// Single-use tokens: password reset and email verification
// ---------------------------------------------------------------------------

// StoreVerificationToken issues a single-use token, replacing any live token
// the user already holds for that purpose.
//
// Replacing rather than accumulating means requesting a second reset link
// invalidates the first, so an old link found in a mailbox cannot be used.
func (r *SessionRepo) StoreVerificationToken(ctx context.Context, userID uuid.UUID, purpose string, digest []byte, expiresAt time.Time) error {
	return InTx(ctx, r.pool, func(tx pgx.Tx) error {
		const supersedeSQL = `
			update verification_tokens set consumed_at = now()
			 where user_id = $1 and purpose = $2::verification_purpose and consumed_at is null`
		if _, err := tx.Exec(ctx, supersedeSQL, userID, purpose); err != nil {
			return domain.Internal(err, "superseding previous %s token", purpose)
		}

		const insertSQL = `
			insert into verification_tokens (user_id, purpose, token_hash, expires_at)
			values ($1, $2::verification_purpose, $3, $4)`
		if _, err := tx.Exec(ctx, insertSQL, userID, purpose, digest, expiresAt); err != nil {
			return domain.Internal(err, "storing %s token", purpose)
		}
		return nil
	})
}

// ConsumeVerificationToken burns a single-use token and returns whose it was.
//
// The UPDATE both checks and consumes in one statement, so the same link
// cannot be redeemed twice by two concurrent requests.
func (r *SessionRepo) ConsumeVerificationToken(ctx context.Context, purpose string, digest []byte) (uuid.UUID, error) {
	const q = `
		update verification_tokens
		   set consumed_at = now()
		 where token_hash = $1
		   and purpose = $2::verification_purpose
		   and consumed_at is null
		   and expires_at > now()
		returning user_id`

	var userID uuid.UUID
	err := r.pool.QueryRow(ctx, q, digest, purpose).Scan(&userID)
	if noRows(err) {
		return uuid.Nil, domain.Invalid("token", "That link has expired or has already been used.")
	}
	if err != nil {
		return uuid.Nil, domain.Internal(err, "consuming %s token", purpose)
	}
	return userID, nil
}
