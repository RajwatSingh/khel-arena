package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

type TournamentRepo struct {
	pool *pgxpool.Pool
}

func NewTournamentRepo(pool *pgxpool.Pool) *TournamentRepo { return &TournamentRepo{pool: pool} }

const tournamentColumns = `
	t.id, t.organizer_id, t.arena_id, t.name, t.slug, t.format, t.side_count,
	t.squad_cap, t.max_teams, t.team_count, t.entry_fee_npr, t.prize_pool_npr,
	t.prize_split, t.skill, coalesce(t.description, ''), coalesce(t.rules, ''),
	t.starts_on, t.register_by, t.status, t.created_at, t.updated_at,
	coalesce(a.name, ''), coalesce(a.area, ''), u.username`

const tournamentJoins = `
	from tournaments t
	join users u on u.id = t.organizer_id
	left join arenas a on a.id = t.arena_id`

func scanTournament(row pgx.Row) (domain.Tournament, error) {
	var (
		t       domain.Tournament
		split   []int32
		arenaID *uuid.UUID
	)

	err := row.Scan(&t.ID, &t.OrganizerID, &arenaID, &t.Name, &t.Slug, &t.Format,
		&t.SideCount, &t.SquadCap, &t.MaxTeams, &t.TeamCount, &t.EntryFeeNPR,
		&t.PrizePoolNPR, &split, &t.Skill, &t.Description, &t.Rules,
		&t.StartsOn, &t.RegisterBy, &t.Status, &t.CreatedAt, &t.UpdatedAt,
		&t.ArenaName, &t.ArenaArea, &t.OrganizerUsername)
	if err != nil {
		return domain.Tournament{}, err
	}

	t.ArenaID = arenaID
	t.PrizeSplit = make([]int, 0, len(split))
	for _, pct := range split {
		t.PrizeSplit = append(t.PrizeSplit, int(pct))
	}
	return t, nil
}

func int32Slice(in []int) []int32 {
	out := make([]int32, 0, len(in))
	for _, v := range in {
		out = append(out, int32(v))
	}
	return out
}

func (r *TournamentRepo) Create(ctx context.Context, t domain.Tournament) (domain.Tournament, error) {
	const q = `
		insert into tournaments (
			organizer_id, arena_id, name, slug, format, side_count, squad_cap,
			max_teams, entry_fee_npr, prize_pool_npr, prize_split, skill,
			description, rules, starts_on, register_by
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
			nullif($13, ''), nullif($14, ''), $15, $16)
		returning id`

	var id uuid.UUID
	err := r.pool.QueryRow(ctx, q, t.OrganizerID, t.ArenaID, t.Name, t.Slug, t.Format,
		t.SideCount, t.SquadCap, t.MaxTeams, t.EntryFeeNPR, t.PrizePoolNPR,
		int32Slice(t.PrizeSplit), t.Skill, t.Description, t.Rules,
		t.StartsOn, t.RegisterBy).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Tournament{}, domain.Conflict("A tournament already uses that web address.")
		}
		return domain.Tournament{}, domain.Internal(err, "creating tournament %q", t.Name)
	}
	return r.ByID(ctx, id)
}

func (r *TournamentRepo) ByID(ctx context.Context, id uuid.UUID) (domain.Tournament, error) {
	const q = `select ` + tournamentColumns + tournamentJoins + ` where t.id = $1`

	t, err := scanTournament(r.pool.QueryRow(ctx, q, id))
	if noRows(err) {
		return domain.Tournament{}, domain.NotFound("No tournament with that id.")
	}
	if err != nil {
		return domain.Tournament{}, domain.Internal(err, "loading tournament %s", id)
	}
	return t, nil
}

func (r *TournamentRepo) BySlug(ctx context.Context, slug string) (domain.Tournament, error) {
	const q = `select ` + tournamentColumns + tournamentJoins + ` where t.slug = $1`

	t, err := scanTournament(r.pool.QueryRow(ctx, q, slug))
	if noRows(err) {
		return domain.Tournament{}, domain.NotFound("No tournament at that address.")
	}
	if err != nil {
		return domain.Tournament{}, domain.Internal(err, "loading tournament %q", slug)
	}
	return t, nil
}

// List returns tournaments still worth entering or watching, soonest first.
//
// Cancelled and completed ones are excluded: a listing is a thing you act on,
// and a finished bracket belongs on its own page rather than in the index.
func (r *TournamentRepo) List(ctx context.Context, limit int) ([]domain.Tournament, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	const q = `
		select ` + tournamentColumns + tournamentJoins + `
		where t.status in ('open', 'full', 'ongoing')
		order by t.starts_on
		limit $1`

	rows, err := r.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, domain.Internal(err, "listing tournaments")
	}
	defer rows.Close()

	var out []domain.Tournament
	for rows.Next() {
		t, err := scanTournament(rows)
		if err != nil {
			return nil, domain.Internal(err, "scanning tournament")
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *TournamentRepo) SetStatus(ctx context.Context, id uuid.UUID, status domain.TournamentStatus) error {
	const q = `update tournaments set status = $2 where id = $1`

	tag, err := r.pool.Exec(ctx, q, id, status)
	if err != nil {
		return domain.Internal(err, "setting status of tournament %s", id)
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("No tournament with that id.")
	}
	return nil
}

// Entries returns the teams in a bracket.
func (r *TournamentRepo) Entries(ctx context.Context, tournamentID uuid.UUID) ([]domain.TournamentEntry, error) {
	const q = `
		select e.tournament_id, e.team_id, e.registered_by, e.paid, e.registered_at,
			t.name, t.tag, coalesce(t.crest_url, '')
		from tournament_teams e
		join teams t on t.id = e.team_id
		where e.tournament_id = $1
		order by e.registered_at`

	rows, err := r.pool.Query(ctx, q, tournamentID)
	if err != nil {
		return nil, domain.Internal(err, "loading entries for tournament %s", tournamentID)
	}
	defer rows.Close()

	var out []domain.TournamentEntry
	for rows.Next() {
		var e domain.TournamentEntry
		err := rows.Scan(&e.TournamentID, &e.TeamID, &e.RegisteredBy, &e.Paid, &e.RegisteredAt,
			&e.TeamName, &e.TeamTag, &e.CrestURL)
		if err != nil {
			return nil, domain.Internal(err, "scanning tournament entry")
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Register enters a team.
//
// The count and the open/full flip are maintained by a trigger that locks the
// tournament row, and `within_capacity` rejects the registration that would
// overflow the bracket. So the race between two captains claiming the last
// slot is settled in the database, and this method's job is only to translate
// what it says into something a person can read.
func (r *TournamentRepo) Register(ctx context.Context, tournamentID, teamID, byUserID uuid.UUID) error {
	const q = `
		insert into tournament_teams (tournament_id, team_id, registered_by)
		values ($1, $2, $3)`

	if _, err := r.pool.Exec(ctx, q, tournamentID, teamID, byUserID); err != nil {
		if isUniqueViolation(err) {
			return domain.Conflict("That team is already entered.")
		}
		if isCheckViolation(err) {
			// `within_capacity`: somebody took the last slot first.
			return domain.Conflict("This tournament filled up while you were registering.")
		}
		return domain.Internal(err, "registering team %s in tournament %s", teamID, tournamentID)
	}
	return nil
}

func (r *TournamentRepo) Withdraw(ctx context.Context, tournamentID, teamID uuid.UUID) error {
	const q = `delete from tournament_teams where tournament_id = $1 and team_id = $2`

	tag, err := r.pool.Exec(ctx, q, tournamentID, teamID)
	if err != nil {
		return domain.Internal(err, "withdrawing team %s", teamID)
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("That team isn't entered.")
	}
	return nil
}

// SetEntryPaid records that a team settled its entry fee.
func (r *TournamentRepo) SetEntryPaid(ctx context.Context, tournamentID, teamID uuid.UUID, paid bool) error {
	const q = `update tournament_teams set paid = $3 where tournament_id = $1 and team_id = $2`

	tag, err := r.pool.Exec(ctx, q, tournamentID, teamID, paid)
	if err != nil {
		return domain.Internal(err, "marking entry paid")
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("That team isn't entered.")
	}
	return nil
}
