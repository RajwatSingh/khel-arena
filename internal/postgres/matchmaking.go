package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

type MatchmakingRepo struct {
	pool *pgxpool.Pool
}

func NewMatchmakingRepo(pool *pgxpool.Pool) *MatchmakingRepo { return &MatchmakingRepo{pool: pool} }

const callColumns = `
	p.id, p.author_id, p.booking_id, p.arena_id, p.title, coalesce(p.description, ''),
	p.needed_players, p.filled_players, p.skill, p.starts_at, p.status,
	p.created_at, p.updated_at,
	u.username, u.full_name, coalesce(u.avatar_url, ''), u.community_score,
	coalesce(a.name, ''), coalesce(a.area, '')`

func scanCall(row pgx.Row) (domain.Call, error) {
	var (
		c       domain.Call
		author  domain.UserSummary
		arena   string
		area    string
		desc    string
		bookID  *uuid.UUID
		arenaID *uuid.UUID
	)

	err := row.Scan(&c.ID, &c.AuthorID, &bookID, &arenaID, &c.Title, &desc,
		&c.NeededPlayers, &c.FilledPlayers, &c.Skill, &c.StartsAt, &c.Status,
		&c.CreatedAt, &c.UpdatedAt,
		&author.Username, &author.FullName, &author.AvatarURL, &author.CommunityScore,
		&arena, &area)
	if err != nil {
		return domain.Call{}, err
	}

	c.BookingID, c.ArenaID, c.Description = bookID, arenaID, desc
	author.ID = c.AuthorID
	c.Author = &author
	c.ArenaName, c.ArenaArea = arena, area
	return c, nil
}

const callJoins = `
	from matchmaking_posts p
	join users u on u.id = p.author_id
	left join arenas a on a.id = p.arena_id`

func (r *MatchmakingRepo) Create(ctx context.Context, c domain.Call) (domain.Call, error) {
	const insert = `
		insert into matchmaking_posts
			(author_id, booking_id, arena_id, title, description, needed_players, skill, starts_at)
		values ($1, $2, $3, $4, nullif($5, ''), $6, $7, $8)
		returning id`

	var id uuid.UUID
	err := r.pool.QueryRow(ctx, insert, c.AuthorID, c.BookingID, c.ArenaID,
		c.Title, c.Description, c.NeededPlayers, c.Skill, c.StartsAt).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			// One post per booking, which is what makes reopening a court
			// idempotent rather than stacking duplicate calls on it.
			return domain.Call{}, domain.Conflict("That booking is already open to join.")
		}
		return domain.Call{}, domain.Internal(err, "creating matchmaking call")
	}
	return r.ByID(ctx, id)
}

func (r *MatchmakingRepo) ByID(ctx context.Context, id uuid.UUID) (domain.Call, error) {
	const q = `select ` + callColumns + callJoins + ` where p.id = $1`

	c, err := scanCall(r.pool.QueryRow(ctx, q, id))
	if noRows(err) {
		return domain.Call{}, domain.NotFound("No call with that id.")
	}
	if err != nil {
		return domain.Call{}, domain.Internal(err, "loading call %s", id)
	}
	return c, nil
}

// CallFilter narrows the feed. Every field is optional; the zero value is
// "everything still open".
type CallFilter struct {
	Skill  domain.SkillTier
	Area   string
	Before time.Time
	Limit  int
}

// Feed returns open calls that have not started, soonest first.
//
// Expired calls are excluded by the time predicate rather than by their
// status: a call whose kickoff has passed is over whether or not anything has
// got round to writing that down. The janitor closes them eventually; the feed
// does not wait for it.
func (r *MatchmakingRepo) Feed(ctx context.Context, f CallFilter, now time.Time) ([]domain.Call, error) {
	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	const q = `
		select ` + callColumns + callJoins + `
		where p.status = 'open'
		  and p.starts_at > $1
		  and p.filled_players < p.needed_players
		  and ($2 = '' or p.skill::text = $2)
		  and ($3 = '' or a.area = $3)
		  and ($4::timestamptz is null or p.starts_at < $4)
		order by p.starts_at
		limit $5`

	var before any
	if !f.Before.IsZero() {
		before = f.Before
	}

	rows, err := r.pool.Query(ctx, q, now, string(f.Skill), f.Area, before, limit)
	if err != nil {
		return nil, domain.Internal(err, "loading matchmaking feed")
	}
	defer rows.Close()

	var out []domain.Call
	for rows.Next() {
		c, err := scanCall(rows)
		if err != nil {
			return nil, domain.Internal(err, "scanning call")
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.Internal(err, "loading matchmaking feed")
	}
	return out, nil
}

// ListForUser returns calls the player wrote or answered.
func (r *MatchmakingRepo) ListForUser(ctx context.Context, userID uuid.UUID) ([]domain.Call, error) {
	const q = `
		select ` + callColumns + callJoins + `
		where p.author_id = $1
		   or exists (select 1 from matchmaking_responses x where x.post_id = p.id and x.user_id = $1)
		order by p.starts_at desc
		limit 100`

	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, domain.Internal(err, "listing calls for user %s", userID)
	}
	defer rows.Close()

	var out []domain.Call
	for rows.Next() {
		c, err := scanCall(rows)
		if err != nil {
			return nil, domain.Internal(err, "scanning call")
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *MatchmakingRepo) Update(ctx context.Context, id uuid.UUID, c domain.Call) (domain.Call, error) {
	const q = `
		update matchmaking_posts
		   set title = $2, description = nullif($3, ''), needed_players = $4,
		       skill = $5, starts_at = $6
		 where id = $1`

	tag, err := r.pool.Exec(ctx, q, id, c.Title, c.Description, c.NeededPlayers, c.Skill, c.StartsAt)
	if err != nil {
		if isCheckViolation(err) {
			// needed_players cannot drop below what is already filled.
			return domain.Call{}, domain.Conflict("More players have joined than that.")
		}
		return domain.Call{}, domain.Internal(err, "updating call %s", id)
	}
	if tag.RowsAffected() == 0 {
		return domain.Call{}, domain.NotFound("No call with that id.")
	}
	return r.ByID(ctx, id)
}

func (r *MatchmakingRepo) SetStatus(ctx context.Context, id uuid.UUID, status domain.MatchmakingStatus) error {
	const q = `update matchmaking_posts set status = $2 where id = $1`

	tag, err := r.pool.Exec(ctx, q, id, status)
	if err != nil {
		return domain.Internal(err, "setting status of call %s", id)
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("No call with that id.")
	}
	return nil
}

func (r *MatchmakingRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `delete from matchmaking_posts where id = $1`, id)
	if err != nil {
		return domain.Internal(err, "deleting call %s", id)
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("No call with that id.")
	}
	return nil
}

// Respond records a player asking to join.
//
// A response is not a place in the game: `accepted` defaults to false and only
// the author can flip it. That is why this does not touch filled_players --
// asking is free, and only acceptance costs a spot.
func (r *MatchmakingRepo) Respond(ctx context.Context, callID, userID uuid.UUID, message string) error {
	const q = `
		insert into matchmaking_responses (post_id, user_id, message)
		values ($1, $2, nullif($3, ''))`

	if _, err := r.pool.Exec(ctx, q, callID, userID, message); err != nil {
		if isUniqueViolation(err) {
			return domain.Conflict("You've already asked to join this one.")
		}
		return domain.Internal(err, "responding to call %s", callID)
	}
	return nil
}

// Responses returns who has asked to join.
func (r *MatchmakingRepo) Responses(ctx context.Context, callID uuid.UUID) ([]domain.Response, error) {
	const q = `
		select x.post_id, x.user_id, coalesce(x.message, ''), x.accepted, x.created_at,
			u.username, u.full_name, coalesce(u.avatar_url, ''), u.community_score
		from matchmaking_responses x
		join users u on u.id = x.user_id
		where x.post_id = $1
		order by x.accepted desc, x.created_at`

	rows, err := r.pool.Query(ctx, q, callID)
	if err != nil {
		return nil, domain.Internal(err, "loading responses for call %s", callID)
	}
	defer rows.Close()

	var out []domain.Response
	for rows.Next() {
		var (
			resp      domain.Response
			responder domain.UserSummary
		)
		err := rows.Scan(&resp.CallID, &resp.UserID, &resp.Message, &resp.Accepted, &resp.CreatedAt,
			&responder.Username, &responder.FullName, &responder.AvatarURL, &responder.CommunityScore)
		if err != nil {
			return nil, domain.Internal(err, "scanning response")
		}
		responder.ID = resp.UserID
		resp.Responder = &responder
		out = append(out, resp)
	}
	return out, rows.Err()
}

// Accept gives a responder a place in the game.
//
// The spot count and the acceptance move together, in one transaction, under
// a lock on the post. Two captains accepting the last spot at the same moment
// would otherwise both read "one left" and both write "filled" -- and the
// `not_overfilled` CHECK would catch the second only by rolling it back with
// a constraint error nobody can act on. Locking first turns that race into an
// orderly "this one is full".
func (r *MatchmakingRepo) Accept(ctx context.Context, callID, userID uuid.UUID) error {
	return InTx(ctx, r.pool, func(tx pgx.Tx) error {
		const lockSQL = `
			select needed_players, filled_players, status
			from matchmaking_posts where id = $1 for update`

		var (
			needed, filled int
			status         domain.MatchmakingStatus
		)
		if err := tx.QueryRow(ctx, lockSQL, callID).Scan(&needed, &filled, &status); err != nil {
			if noRows(err) {
				return domain.NotFound("No call with that id.")
			}
			return domain.Internal(err, "locking call %s", callID)
		}
		if status != domain.MatchmakingOpen {
			return domain.Conflict("This game is no longer taking players.")
		}
		if filled >= needed {
			return domain.Conflict("This game is already full.")
		}

		const acceptSQL = `
			update matchmaking_responses set accepted = true
			 where post_id = $1 and user_id = $2 and not accepted`

		tag, err := tx.Exec(ctx, acceptSQL, callID, userID)
		if err != nil {
			return domain.Internal(err, "accepting response on call %s", callID)
		}
		if tag.RowsAffected() == 0 {
			// Either they never asked, or they are already in. Both are
			// "nothing to do", and the second is a double click.
			return nil
		}

		const fillSQL = `update matchmaking_posts set filled_players = filled_players + 1 where id = $1`
		if _, err := tx.Exec(ctx, fillSQL, callID); err != nil {
			return domain.Internal(err, "filling a spot on call %s", callID)
		}

		// A call that just filled up stops appearing in the feed. Written
		// here rather than left for the janitor so the state is true the
		// moment it becomes true.
		if filled+1 >= needed {
			const closeSQL = `update matchmaking_posts set status = 'filled' where id = $1`
			if _, err := tx.Exec(ctx, closeSQL, callID); err != nil {
				return domain.Internal(err, "closing filled call %s", callID)
			}
		}
		return nil
	})
}

// Withdraw removes a response, freeing the spot if it had been accepted.
func (r *MatchmakingRepo) Withdraw(ctx context.Context, callID, userID uuid.UUID) error {
	return InTx(ctx, r.pool, func(tx pgx.Tx) error {
		const q = `
			delete from matchmaking_responses
			 where post_id = $1 and user_id = $2
			returning accepted`

		var accepted bool
		if err := tx.QueryRow(ctx, q, callID, userID).Scan(&accepted); err != nil {
			if noRows(err) {
				return domain.NotFound("You haven't asked to join this one.")
			}
			return domain.Internal(err, "withdrawing from call %s", callID)
		}
		if !accepted {
			return nil // never held a spot
		}

		// Giving the spot back reopens the call: it was full, and now it is
		// not. `greatest(...,0)` because the count must never go negative even
		// if something upstream has gone wrong.
		const freeSQL = `
			update matchmaking_posts
			   set filled_players = greatest(filled_players - 1, 0),
			       status = case when status = 'filled' then 'open' else status end
			 where id = $1`
		if _, err := tx.Exec(ctx, freeSQL, callID); err != nil {
			return domain.Internal(err, "freeing a spot on call %s", callID)
		}
		return nil
	})
}

// HasResponded reports whether a player has already asked to join.
func (r *MatchmakingRepo) HasResponded(ctx context.Context, callID, userID uuid.UUID) (bool, error) {
	const q = `select exists (select 1 from matchmaking_responses where post_id = $1 and user_id = $2)`

	var responded bool
	if err := r.pool.QueryRow(ctx, q, callID, userID).Scan(&responded); err != nil {
		return false, domain.Internal(err, "checking response on call %s", callID)
	}
	return responded, nil
}

// ExpireStale closes calls whose kickoff has passed.
//
// The janitor's job, like releasing stale holds: the feed already excludes
// them by time, so this is the stored state catching up with what is already
// true rather than something correctness depends on.
func (r *MatchmakingRepo) ExpireStale(ctx context.Context) (int, error) {
	const q = `
		update matchmaking_posts set status = 'expired'
		 where status = 'open' and starts_at <= now()`

	tag, err := r.pool.Exec(ctx, q)
	if err != nil {
		return 0, domain.Internal(err, "expiring stale matchmaking calls")
	}
	return int(tag.RowsAffected()), nil
}
