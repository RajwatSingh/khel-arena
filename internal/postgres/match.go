package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

type MatchRepo struct {
	pool *pgxpool.Pool
}

func NewMatchRepo(pool *pgxpool.Pool) *MatchRepo { return &MatchRepo{pool: pool} }

const matchColumns = `
	m.id, m.booking_id, m.home_team, m.away_team, m.home_score, m.away_score,
	m.played_at, m.verified, m.reported_by, m.created_at,
	h.name, h.tag, a.name, a.tag`

const matchJoins = `
	from matches m
	join teams h on h.id = m.home_team
	join teams a on a.id = m.away_team`

func scanMatch(row pgx.Row) (domain.Match, error) {
	var (
		m          domain.Match
		reportedBy *uuid.UUID
	)
	err := row.Scan(&m.ID, &m.BookingID, &m.HomeTeam, &m.AwayTeam,
		&m.HomeScore, &m.AwayScore, &m.PlayedAt, &m.Verified, &reportedBy, &m.CreatedAt,
		&m.HomeName, &m.HomeTag, &m.AwayName, &m.AwayTag)
	if err != nil {
		return domain.Match{}, err
	}
	if reportedBy != nil {
		m.ReportedBy = *reportedBy
	}
	return m, nil
}

// Create files a result, unagreed.
func (r *MatchRepo) Create(ctx context.Context, m domain.Match) (domain.Match, error) {
	const q = `
		insert into matches
			(booking_id, home_team, away_team, home_score, away_score, played_at, reported_by)
		values ($1, $2, $3, $4, $5, $6, $7)
		returning id`

	var id uuid.UUID
	err := r.pool.QueryRow(ctx, q, m.BookingID, m.HomeTeam, m.AwayTeam,
		m.HomeScore, m.AwayScore, m.PlayedAt, m.ReportedBy).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			// One result per booking, which stops the same hour being
			// reported twice with different scores.
			return domain.Match{}, domain.Conflict("A result has already been filed for that booking.")
		}
		if isCheckViolation(err) {
			return domain.Match{}, domain.Invalid("away_team", "A team can't play itself.")
		}
		return domain.Match{}, domain.Internal(err, "creating match")
	}
	return r.ByID(ctx, id)
}

func (r *MatchRepo) ByID(ctx context.Context, id uuid.UUID) (domain.Match, error) {
	const q = `select ` + matchColumns + matchJoins + ` where m.id = $1`

	m, err := scanMatch(r.pool.QueryRow(ctx, q, id))
	if noRows(err) {
		return domain.Match{}, domain.NotFound("No result with that id.")
	}
	if err != nil {
		return domain.Match{}, domain.Internal(err, "loading match %s", id)
	}
	return m, nil
}

// Confirm marks a result agreed.
//
// The `and not verified` predicate makes a second confirmation a no-op rather
// than a second write, which matters because the standings view reads
// `verified` and nothing else: a double click must not be able to change what
// the table says.
func (r *MatchRepo) Confirm(ctx context.Context, id uuid.UUID) error {
	const q = `update matches set verified = true where id = $1 and not verified`

	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return domain.Internal(err, "confirming match %s", id)
	}
	if tag.RowsAffected() == 0 {
		// Either gone or already agreed. The service has already loaded it and
		// checked, so this is a race with another confirmation -- and the
		// other one's outcome is the same as this one's would have been.
		return nil
	}
	return nil
}

// Recount replaces a disputed score and hands the result back to the other
// captain.
//
// `reported_by` moves to whoever countered, which is what makes the next
// confirmation have to come from the original reporter. The `and not verified`
// predicate stops a result being rewritten after both sides agreed it.
func (r *MatchRepo) Recount(ctx context.Context, id uuid.UUID, homeScore, awayScore int, byUserID uuid.UUID) error {
	const q = `
		update matches
		   set home_score = $2, away_score = $3, reported_by = $4
		 where id = $1 and not verified`

	tag, err := r.pool.Exec(ctx, q, id, homeScore, awayScore, byUserID)
	if err != nil {
		if isCheckViolation(err) {
			return domain.Invalid("home_score", "Scores can't be negative.")
		}
		return domain.Internal(err, "disputing match %s", id)
	}
	if tag.RowsAffected() == 0 {
		return domain.Conflict("Both captains agreed that result. It stands.")
	}
	return nil
}

func (r *MatchRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `delete from matches where id = $1`, id)
	if err != nil {
		return domain.Internal(err, "deleting match %s", id)
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("No result with that id.")
	}
	return nil
}

// ListForTeam returns a team's results, newest first.
func (r *MatchRepo) ListForTeam(ctx context.Context, teamID uuid.UUID, limit int) ([]domain.Match, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	const q = `
		select ` + matchColumns + matchJoins + `
		where m.home_team = $1 or m.away_team = $1
		order by m.played_at desc
		limit $2`

	rows, err := r.pool.Query(ctx, q, teamID, limit)
	if err != nil {
		return nil, domain.Internal(err, "listing matches for team %s", teamID)
	}
	defer rows.Close()

	var out []domain.Match
	for rows.Next() {
		m, err := scanMatch(rows)
		if err != nil {
			return nil, domain.Internal(err, "scanning match")
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Standings reads the leaderboard.
//
// A view, not a query assembled here: `team_standings` already states three
// points for a win, one for a draw, goal difference as the tiebreaker, and
// counts only verified results. Restating any of that in Go would be a second
// definition of the table that could disagree with the first.
func (r *MatchRepo) Standings(ctx context.Context, limit int) ([]domain.Standing, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	// The view ranks as well as orders, so the position comes from it too --
	// numbering rows as they arrive here would be a second opinion about who
	// is above whom.
	const q = `
		select team_id, name, tag, coalesce(crest_url, ''),
			played, won, drawn, lost, goals_for, goals_against, goal_diff, points, rank
		from team_standings
		limit $1`

	rows, err := r.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, domain.Internal(err, "loading standings")
	}
	defer rows.Close()

	var out []domain.Standing
	for rows.Next() {
		var s domain.Standing
		err := rows.Scan(&s.TeamID, &s.Name, &s.Tag, &s.CrestURL,
			&s.Played, &s.Won, &s.Drawn, &s.Lost,
			&s.GoalsFor, &s.GoalsAgainst, &s.GoalDiff, &s.Points, &s.Rank)
		if err != nil {
			return nil, domain.Internal(err, "scanning standing")
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
