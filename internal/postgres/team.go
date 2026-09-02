package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

type TeamRepo struct {
	pool *pgxpool.Pool
}

func NewTeamRepo(pool *pgxpool.Pool) *TeamRepo { return &TeamRepo{pool: pool} }

// teamColumns carries MemberCount as a subquery rather than a stored counter.
//
// A squad is at most twenty people and the roster is read on the same screens
// as the count, so counting is cheap; a stored column would be one more thing
// that can disagree with the rows it describes.
const teamColumns = `
	t.id, t.name, t.tag, coalesce(t.crest_url, ''), t.captain_id, t.home_arena,
	t.join_code, (select count(*) from team_members m where m.team_id = t.id),
	t.created_at, t.updated_at`

func scanTeam(row pgx.Row) (domain.Team, error) {
	var t domain.Team
	err := row.Scan(&t.ID, &t.Name, &t.Tag, &t.CrestURL, &t.CaptainID, &t.HomeArena,
		&t.JoinCode, &t.MemberCount, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return domain.Team{}, err
	}
	return t, nil
}

// Create registers a team and seats its captain on the roster.
//
// One transaction, because a team whose captain is not a member is a roster
// the domain rules cannot reason about: `CanRemoveMember` refuses to remove
// the captain, and a captain absent from `team_members` would be a team nobody
// can manage and nobody can leave.
func (r *TeamRepo) Create(ctx context.Context, t domain.Team) (domain.Team, error) {
	var created domain.Team

	err := InTx(ctx, r.pool, func(tx pgx.Tx) error {
		const insertTeam = `
			insert into teams (name, tag, crest_url, captain_id, home_arena)
			values ($1, $2, nullif($3, ''), $4, $5)
			returning id`

		var teamID uuid.UUID
		err := tx.QueryRow(ctx, insertTeam, t.Name, t.Tag, t.CrestURL, t.CaptainID, t.HomeArena).Scan(&teamID)
		if err != nil {
			if isUniqueViolation(err) {
				// Name and tag are both unique. Which one collided is worth
				// saying, so the person can change the right field.
				return domain.Conflict("A team already uses that name or tag.")
			}
			return domain.Internal(err, "creating team %q", t.Name)
		}

		const insertCaptain = `
			insert into team_members (team_id, user_id, role) values ($1, $2, 'captain')`
		if _, err := tx.Exec(ctx, insertCaptain, teamID, t.CaptainID); err != nil {
			return domain.Internal(err, "seating captain on team %s", teamID)
		}

		const selectTeam = `select ` + teamColumns + ` from teams t where t.id = $1`
		created, err = scanTeam(tx.QueryRow(ctx, selectTeam, teamID))
		if err != nil {
			return domain.Internal(err, "reading back team %s", teamID)
		}
		return nil
	})

	return created, err
}

func (r *TeamRepo) ByID(ctx context.Context, id uuid.UUID) (domain.Team, error) {
	const q = `select ` + teamColumns + ` from teams t where t.id = $1`

	t, err := scanTeam(r.pool.QueryRow(ctx, q, id))
	if noRows(err) {
		return domain.Team{}, domain.NotFound("No team with that id.")
	}
	if err != nil {
		return domain.Team{}, domain.Internal(err, "loading team %s", id)
	}
	return t, nil
}

// ByJoinCode finds a team from an invite code.
func (r *TeamRepo) ByJoinCode(ctx context.Context, code string) (domain.Team, error) {
	const q = `select ` + teamColumns + ` from teams t where t.join_code = $1`

	t, err := scanTeam(r.pool.QueryRow(ctx, q, code))
	if noRows(err) {
		return domain.Team{}, domain.NotFound("That invite code doesn't match a team.")
	}
	if err != nil {
		return domain.Team{}, domain.Internal(err, "loading team by join code")
	}
	return t, nil
}

// ListForUser returns the teams a player is on.
func (r *TeamRepo) ListForUser(ctx context.Context, userID uuid.UUID) ([]domain.Team, error) {
	const q = `
		select ` + teamColumns + `
		from teams t
		join team_members m on m.team_id = t.id
		where m.user_id = $1
		order by t.name`

	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, domain.Internal(err, "listing teams for user %s", userID)
	}
	defer rows.Close()

	var out []domain.Team
	for rows.Next() {
		t, err := scanTeam(rows)
		if err != nil {
			return nil, domain.Internal(err, "scanning team")
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.Internal(err, "listing teams for user %s", userID)
	}
	return out, nil
}

// Roster returns a team's members with enough of each player to render them.
func (r *TeamRepo) Roster(ctx context.Context, teamID uuid.UUID) ([]domain.Member, error) {
	const q = `
		select m.team_id, m.user_id, m.role, m.joined_at,
			u.username, u.full_name, coalesce(u.avatar_url, ''), u.community_score
		from team_members m
		join users u on u.id = m.user_id
		where m.team_id = $1
		order by (m.role = 'captain') desc, m.joined_at`

	rows, err := r.pool.Query(ctx, q, teamID)
	if err != nil {
		return nil, domain.Internal(err, "loading roster for team %s", teamID)
	}
	defer rows.Close()

	var out []domain.Member
	for rows.Next() {
		var (
			m       domain.Member
			summary domain.UserSummary
		)
		err := rows.Scan(&m.TeamID, &m.UserID, &m.Role, &m.JoinedAt,
			&summary.Username, &summary.FullName, &summary.AvatarURL, &summary.CommunityScore)
		if err != nil {
			return nil, domain.Internal(err, "scanning roster row")
		}
		summary.ID = m.UserID
		m.User = &summary
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.Internal(err, "loading roster for team %s", teamID)
	}
	return out, nil
}

// Update edits a team's details.
func (r *TeamRepo) Update(ctx context.Context, teamID uuid.UUID, t domain.Team) (domain.Team, error) {
	const q = `
		update teams set name = $2, tag = $3, crest_url = nullif($4, ''), home_arena = $5
		where id = $1
		returning id`

	var id uuid.UUID
	err := r.pool.QueryRow(ctx, q, teamID, t.Name, t.Tag, t.CrestURL, t.HomeArena).Scan(&id)
	if noRows(err) {
		return domain.Team{}, domain.NotFound("No team with that id.")
	}
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Team{}, domain.Conflict("A team already uses that name or tag.")
		}
		return domain.Team{}, domain.Internal(err, "updating team %s", teamID)
	}
	return r.ByID(ctx, teamID)
}

// AddMember seats a player.
//
// The uniqueness of (team_id, user_id) is what makes joining twice harmless:
// a second attempt collides rather than adding a duplicate row that would
// inflate the squad count and let one person occupy two places.
func (r *TeamRepo) AddMember(ctx context.Context, teamID, userID uuid.UUID) error {
	const q = `insert into team_members (team_id, user_id, role) values ($1, $2, 'player')`

	if _, err := r.pool.Exec(ctx, q, teamID, userID); err != nil {
		if isUniqueViolation(err) {
			return domain.Conflict("They're already on this team.")
		}
		return domain.Internal(err, "adding member to team %s", teamID)
	}
	return nil
}

func (r *TeamRepo) RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error {
	const q = `delete from team_members where team_id = $1 and user_id = $2`

	tag, err := r.pool.Exec(ctx, q, teamID, userID)
	if err != nil {
		return domain.Internal(err, "removing member from team %s", teamID)
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("They aren't on this team.")
	}
	return nil
}

// IsMember reports whether a player is on a roster.
func (r *TeamRepo) IsMember(ctx context.Context, teamID, userID uuid.UUID) (bool, error) {
	const q = `select exists (select 1 from team_members where team_id = $1 and user_id = $2)`

	var member bool
	if err := r.pool.QueryRow(ctx, q, teamID, userID).Scan(&member); err != nil {
		return false, domain.Internal(err, "checking membership of team %s", teamID)
	}
	return member, nil
}

// TransferCaptaincy hands the armband over.
//
// Three writes in one transaction, and the order matters. A unique index
// allows exactly one captain per team, so the outgoing captain is demoted
// before the incoming one is promoted -- the other order violates the index
// halfway through and rolls the whole thing back.
func (r *TeamRepo) TransferCaptaincy(ctx context.Context, teamID, fromID, toID uuid.UUID) error {
	return InTx(ctx, r.pool, func(tx pgx.Tx) error {
		const demote = `update team_members set role = 'player' where team_id = $1 and user_id = $2`
		if _, err := tx.Exec(ctx, demote, teamID, fromID); err != nil {
			return domain.Internal(err, "demoting captain of team %s", teamID)
		}

		const promote = `update team_members set role = 'captain' where team_id = $1 and user_id = $2`
		tag, err := tx.Exec(ctx, promote, teamID, toID)
		if err != nil {
			return domain.Internal(err, "promoting captain of team %s", teamID)
		}
		if tag.RowsAffected() == 0 {
			return domain.NotFound("They aren't on this team.")
		}

		// The captain is denormalised onto the team as well as the roster,
		// because every read of a team wants it and joining the roster to find
		// it would be a join on every listing.
		const setCaptain = `update teams set captain_id = $2 where id = $1`
		if _, err := tx.Exec(ctx, setCaptain, teamID, toID); err != nil {
			return domain.Internal(err, "setting captain of team %s", teamID)
		}
		return nil
	})
}

// RotateJoinCode retires a leaked invite code without disbanding the team.
func (r *TeamRepo) RotateJoinCode(ctx context.Context, teamID uuid.UUID) (string, error) {
	// Generated in SQL by the same expression the column defaults to, so a
	// rotated code is indistinguishable from an original one.
	const q = `
		update teams
		   set join_code = upper(substr(md5(gen_random_uuid()::text), 1, 8))
		 where id = $1
		returning join_code`

	var code string
	err := r.pool.QueryRow(ctx, q, teamID).Scan(&code)
	if noRows(err) {
		return "", domain.NotFound("No team with that id.")
	}
	if err != nil {
		return "", domain.Internal(err, "rotating join code for team %s", teamID)
	}
	return code, nil
}

// Disband deletes a team.
//
// team_members cascades; bookings do not -- `on delete set null` leaves the
// booking standing with its team detached. A squad breaking up does not
// un-book the hours they reserved, and somebody still has to turn up.
func (r *TeamRepo) Disband(ctx context.Context, teamID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `delete from teams where id = $1`, teamID)
	if err != nil {
		return domain.Internal(err, "disbanding team %s", teamID)
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("No team with that id.")
	}
	return nil
}
