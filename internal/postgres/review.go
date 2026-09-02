package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

// ReviewRepo covers the three things people attach to a venue or to
// themselves: reviews of an arena, its photo gallery, and a player's own
// highlight links.
//
// The arena's `rating` and `review_count` are maintained by a trigger, not
// here. Recomputing an average in Go on every write would be a second answer
// to what a venue is rated, and the listing reads the stored one.
type ReviewRepo struct {
	pool *pgxpool.Pool
}

func NewReviewRepo(pool *pgxpool.Pool) *ReviewRepo { return &ReviewRepo{pool: pool} }

// HasPlayedAt reports whether a player has actually been to a venue.
//
// "Played" means a booking they held, at a court of that arena, that was paid
// for and whose hour has passed. A pending hold does not count -- it may never
// have been settled -- and neither does a cancelled one.
//
// `confirmed` is accepted alongside `completed` because the janitor promotes
// one to the other on a timer: between the final whistle and the next sweep a
// booking is confirmed-and-past, and somebody who just played should not be
// told to come back in a minute.
func (r *ReviewRepo) HasPlayedAt(ctx context.Context, arenaID, userID uuid.UUID) (bool, error) {
	const q = `
		select exists (
			select 1
			from bookings b
			join courts c on c.id = b.court_id
			where c.arena_id = $1
			  and b.user_id = $2
			  and b.status in ('confirmed', 'completed')
			  and upper(b.slot) <= now()
		)`

	var played bool
	if err := r.pool.QueryRow(ctx, q, arenaID, userID).Scan(&played); err != nil {
		return false, domain.Internal(err, "checking play history at arena %s", arenaID)
	}
	return played, nil
}

// Upsert writes a player's review of an arena.
//
// One per player per arena, enforced by a unique constraint: posting again
// edits what you said rather than stacking a second opinion. That is what
// makes `on conflict` the right shape here -- the client does not have to know
// whether it is creating or editing, because from the player's side it is the
// same act.
func (r *ReviewRepo) Upsert(ctx context.Context, review domain.Review) (domain.Review, error) {
	const q = `
		insert into arena_reviews (arena_id, user_id, rating, comment)
		values ($1, $2, $3, nullif($4, ''))
		on conflict (arena_id, user_id) do update
			set rating = excluded.rating, comment = excluded.comment
		returning id, arena_id, user_id, rating, coalesce(comment, ''), created_at, updated_at`

	var out domain.Review
	err := r.pool.QueryRow(ctx, q, review.ArenaID, review.UserID, review.Rating, review.Comment).
		Scan(&out.ID, &out.ArenaID, &out.UserID, &out.Rating, &out.Comment, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		if isForeignKeyViolation(err) {
			return domain.Review{}, domain.NotFound("No arena at that address.")
		}
		return domain.Review{}, domain.Internal(err, "saving review of arena %s", review.ArenaID)
	}
	return out, nil
}

// DeleteReview removes a player's own review.
func (r *ReviewRepo) DeleteReview(ctx context.Context, arenaID, userID uuid.UUID) error {
	const q = `delete from arena_reviews where arena_id = $1 and user_id = $2`

	tag, err := r.pool.Exec(ctx, q, arenaID, userID)
	if err != nil {
		return domain.Internal(err, "deleting review of arena %s", arenaID)
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("You haven't reviewed that arena.")
	}
	return nil
}

// ReviewByUser returns one player's review of an arena, if they left one.
func (r *ReviewRepo) ReviewByUser(ctx context.Context, arenaID, userID uuid.UUID) (domain.Review, error) {
	const q = `
		select id, arena_id, user_id, rating, coalesce(comment, ''), created_at, updated_at
		from arena_reviews where arena_id = $1 and user_id = $2`

	var v domain.Review
	err := r.pool.QueryRow(ctx, q, arenaID, userID).
		Scan(&v.ID, &v.ArenaID, &v.UserID, &v.Rating, &v.Comment, &v.CreatedAt, &v.UpdatedAt)
	if noRows(err) {
		return domain.Review{}, domain.NotFound("You haven't reviewed that arena.")
	}
	if err != nil {
		return domain.Review{}, domain.Internal(err, "loading review of arena %s", arenaID)
	}
	return v, nil
}

// ListReviews returns an arena's reviews, newest first.
func (r *ReviewRepo) ListReviews(ctx context.Context, arenaID uuid.UUID, limit int) ([]domain.Review, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	const q = `
		select v.id, v.arena_id, v.user_id, v.rating, coalesce(v.comment, ''),
			v.created_at, v.updated_at,
			u.username, u.full_name, coalesce(u.avatar_url, ''), u.community_score
		from arena_reviews v
		join users u on u.id = v.user_id
		where v.arena_id = $1
		order by v.created_at desc
		limit $2`

	rows, err := r.pool.Query(ctx, q, arenaID, limit)
	if err != nil {
		return nil, domain.Internal(err, "listing reviews of arena %s", arenaID)
	}
	defer rows.Close()

	var out []domain.Review
	for rows.Next() {
		var (
			v      domain.Review
			author domain.UserSummary
		)
		err := rows.Scan(&v.ID, &v.ArenaID, &v.UserID, &v.Rating, &v.Comment,
			&v.CreatedAt, &v.UpdatedAt,
			&author.Username, &author.FullName, &author.AvatarURL, &author.CommunityScore)
		if err != nil {
			return nil, domain.Internal(err, "scanning review")
		}
		author.ID = v.UserID
		v.Author = &author
		out = append(out, v)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Photos
// ---------------------------------------------------------------------------

// Photo is one image in a venue's gallery.
type Photo struct {
	ID        uuid.UUID
	ArenaID   uuid.UUID
	URL       string
	Caption   string
	SortOrder int
}

// AddPhotoOwnerScoped adds an image, and only to an arena the caller owns.
//
// The ownership predicate is in the INSERT, like every other owner-facing
// write: the arena_id comes from a SELECT already filtered by owner.
func (r *ReviewRepo) AddPhotoOwnerScoped(ctx context.Context, ownerID uuid.UUID, p Photo) (Photo, error) {
	const q = `
		insert into arena_photos (arena_id, url, caption, sort_order)
		select a.id, $3, nullif($4, ''), $5
		from arenas a
		where a.id = $1 and a.owner_id = $2
		returning id, arena_id, url, coalesce(caption, ''), sort_order`

	var out Photo
	var caption pgtype.Text
	err := r.pool.QueryRow(ctx, q, p.ArenaID, ownerID, p.URL, p.Caption, p.SortOrder).
		Scan(&out.ID, &out.ArenaID, &out.URL, &caption, &out.SortOrder)
	if noRows(err) {
		return Photo{}, domain.NotFound("No arena of yours at that address.")
	}
	if err != nil {
		return Photo{}, domain.Internal(err, "adding photo to arena %s", p.ArenaID)
	}
	out.Caption = caption.String
	return out, nil
}

func (r *ReviewRepo) DeletePhotoOwnerScoped(ctx context.Context, photoID, ownerID uuid.UUID) error {
	const q = `
		delete from arena_photos p
		using arenas a
		where p.id = $1 and a.id = p.arena_id and a.owner_id = $2`

	tag, err := r.pool.Exec(ctx, q, photoID, ownerID)
	if err != nil {
		return domain.Internal(err, "deleting photo %s", photoID)
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("No photo of yours with that id.")
	}
	return nil
}

func (r *ReviewRepo) ListPhotos(ctx context.Context, arenaID uuid.UUID) ([]Photo, error) {
	const q = `
		select id, arena_id, url, coalesce(caption, ''), sort_order
		from arena_photos
		where arena_id = $1
		order by sort_order, created_at desc`

	rows, err := r.pool.Query(ctx, q, arenaID)
	if err != nil {
		return nil, domain.Internal(err, "listing photos of arena %s", arenaID)
	}
	defer rows.Close()

	var out []Photo
	for rows.Next() {
		var p Photo
		if err := rows.Scan(&p.ID, &p.ArenaID, &p.URL, &p.Caption, &p.SortOrder); err != nil {
			return nil, domain.Internal(err, "scanning photo")
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Highlights
// ---------------------------------------------------------------------------

// Highlight is a link on a player's card.
type Highlight struct {
	ID     uuid.UUID
	UserID uuid.UUID
	Title  string
	URL    string
}

// AddHighlight puts a link on the caller's own card. The user id is the
// caller's, never a parameter a client chooses.
func (r *ReviewRepo) AddHighlight(ctx context.Context, h Highlight) (Highlight, error) {
	const q = `
		insert into profile_highlights (user_id, title, url)
		values ($1, $2, $3)
		returning id, user_id, title, url`

	var out Highlight
	err := r.pool.QueryRow(ctx, q, h.UserID, h.Title, h.URL).
		Scan(&out.ID, &out.UserID, &out.Title, &out.URL)
	if err != nil {
		if isCheckViolation(err) {
			// The schema insists on an http(s) URL and a title of a sensible
			// length; both are worth saying precisely.
			return Highlight{}, domain.Invalid("url", "Give a full web address, starting http:// or https://.")
		}
		return Highlight{}, domain.Internal(err, "adding highlight for user %s", h.UserID)
	}
	return out, nil
}

func (r *ReviewRepo) DeleteHighlight(ctx context.Context, highlightID, userID uuid.UUID) error {
	const q = `delete from profile_highlights where id = $1 and user_id = $2`

	tag, err := r.pool.Exec(ctx, q, highlightID, userID)
	if err != nil {
		return domain.Internal(err, "deleting highlight %s", highlightID)
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("No highlight of yours with that id.")
	}
	return nil
}

func (r *ReviewRepo) ListHighlights(ctx context.Context, userID uuid.UUID) ([]Highlight, error) {
	const q = `
		select id, user_id, title, url
		from profile_highlights
		where user_id = $1
		order by created_at desc`

	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, domain.Internal(err, "listing highlights for user %s", userID)
	}
	defer rows.Close()

	var out []Highlight
	for rows.Next() {
		var h Highlight
		if err := rows.Scan(&h.ID, &h.UserID, &h.Title, &h.URL); err != nil {
			return nil, domain.Internal(err, "scanning highlight")
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
