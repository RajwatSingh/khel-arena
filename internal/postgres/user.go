package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo { return &UserRepo{pool: pool} }

// userColumns deliberately omits password_hash. The only query that reads it
// is CredentialsByEmail, so no other code path can leak a hash by accident.
const userColumns = `
	id, email, email_verified_at, username, full_name, account_type,
	coalesce(avatar_url, ''), coalesce(phone, ''), city,
	position, jersey_number, preferred_foot, skill, coalesce(bio, ''),
	matches_played, matches_won, community_score, created_at, updated_at`

func scanUser(row pgx.Row) (domain.User, error) {
	var u domain.User
	var verifiedAt *time.Time
	var position *domain.Position
	var jersey *int
	var foot *domain.Foot

	err := row.Scan(
		&u.ID, &u.Email, &verifiedAt, &u.Username, &u.FullName, &u.AccountType,
		&u.AvatarURL, &u.Phone, &u.City,
		&position, &jersey, &foot, &u.Skill, &u.Bio,
		&u.MatchesPlayed, &u.MatchesWon, &u.CommunityScore, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return domain.User{}, err
	}

	u.EmailVerifiedAt = verifiedAt
	u.Position = position
	u.JerseyNumber = jersey
	u.PreferredFoot = foot
	return u, nil
}

// Create registers a new account. The caller supplies an already-hashed
// password; this layer never sees a plaintext one.
func (r *UserRepo) Create(ctx context.Context, reg domain.Registration, passwordHash string) (domain.User, error) {
	const q = `
		insert into users (email, password_hash, username, full_name, account_type)
		values ($1, $2, $3, $4, $5)
		returning ` + userColumns

	u, err := scanUser(r.pool.QueryRow(ctx, q,
		reg.Email, passwordHash, reg.Username, reg.FullName, reg.AccountType))

	if err != nil {
		if isUniqueViolation(err) {
			// Name the field so the form can highlight it. Which of the two
			// collided is not a secret: signup would reveal it anyway, and
			// hiding it only produces an unusable error message.
			switch pgErrConstraint(err) {
			case "users_email_key":
				return domain.User{}, domain.Conflict("An account with that email already exists.").
					WithCause(err)
			case "users_username_key":
				return domain.User{}, domain.Conflict("That username is taken.").WithCause(err)
			}
			return domain.User{}, domain.Conflict("That account already exists.").WithCause(err)
		}
		return domain.User{}, domain.Internal(err, "creating user %s", reg.Email)
	}
	return u, nil
}

func (r *UserRepo) ByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	const q = `select ` + userColumns + ` from users where id = $1`

	u, err := scanUser(r.pool.QueryRow(ctx, q, id))
	if noRows(err) {
		return domain.User{}, domain.NotFound("That account doesn't exist.")
	}
	if err != nil {
		return domain.User{}, domain.Internal(err, "loading user %s", id)
	}
	return u, nil
}

func (r *UserRepo) ByUsername(ctx context.Context, username string) (domain.User, error) {
	const q = `select ` + userColumns + ` from users where username = $1`

	u, err := scanUser(r.pool.QueryRow(ctx, q, domain.NormalizeUsername(username)))
	if noRows(err) {
		return domain.User{}, domain.NotFound("That player doesn't exist.")
	}
	if err != nil {
		return domain.User{}, domain.Internal(err, "loading user @%s", username)
	}
	return u, nil
}

// CredentialsByEmail loads the password hash for a login attempt. It is the
// only query in the service that reads password_hash.
//
// A missing account returns ErrNotFound; the caller must still spend the cost
// of a hash comparison before answering, or the response time tells an
// attacker which email addresses are registered.
func (r *UserRepo) CredentialsByEmail(ctx context.Context, email string) (domain.Credentials, error) {
	const q = `select id, email, password_hash from users where email = $1`

	var c domain.Credentials
	err := r.pool.QueryRow(ctx, q, domain.NormalizeEmail(email)).
		Scan(&c.UserID, &c.Email, &c.PasswordHash)

	if noRows(err) {
		return domain.Credentials{}, domain.NotFound("No account with that email.")
	}
	if err != nil {
		return domain.Credentials{}, domain.Internal(err, "loading credentials")
	}
	return c, nil
}

// UpdatePassword replaces a password hash and revokes every refresh token the
// account holds.
//
// Changing a password is how someone responds to a compromise, so it must end
// the sessions an attacker may be holding. Doing both in one transaction
// means there is no window where the password is new but the old sessions
// still work.
func (r *UserRepo) UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	return InTx(ctx, r.pool, func(tx pgx.Tx) error {
		const setSQL = `update users set password_hash = $2 where id = $1`
		tag, err := tx.Exec(ctx, setSQL, userID, passwordHash)
		if err != nil {
			return domain.Internal(err, "updating password for user %s", userID)
		}
		if tag.RowsAffected() == 0 {
			return domain.NotFound("That account doesn't exist.")
		}

		const revokeSQL = `
			update refresh_tokens set revoked_at = now()
			 where user_id = $1 and revoked_at is null`
		if _, err := tx.Exec(ctx, revokeSQL, userID); err != nil {
			return domain.Internal(err, "revoking sessions for user %s", userID)
		}
		return nil
	})
}

// UpdateProfile applies a partial update to the public player card.
//
// Every field is written as `coalesce($n, column)`, so a nil argument leaves
// the column alone. That keeps this one statement instead of assembling SQL
// per request, and makes a partial update incapable of blanking a field it
// never mentioned.
func (r *UserRepo) UpdateProfile(ctx context.Context, userID uuid.UUID, p domain.ProfileUpdate) (domain.User, error) {
	const q = `
		update users set
			full_name      = coalesce($2, full_name),
			avatar_url     = coalesce($3, avatar_url),
			phone          = coalesce($4, phone),
			city           = coalesce($5, city),
			skill          = coalesce($6, skill),
			bio            = coalesce($7, bio),
			position       = case when $8::boolean  then $9::futsal_position else position end,
			jersey_number  = case when $10::boolean then $11::int            else jersey_number end,
			preferred_foot = case when $12::boolean then $13::preferred_foot else preferred_foot end
		where id = $1
		returning ` + userColumns

	// The three nullable enum fields need a "was it mentioned" flag, because
	// coalesce cannot distinguish "leave alone" from "set to null" -- and
	// clearing a position is something a player must be able to do.
	var (
		setPosition = p.Position != nil
		setJersey   = p.JerseyNumber != nil
		setFoot     = p.PreferredFoot != nil

		position *domain.Position
		jersey   *int
		foot     *domain.Foot
	)
	if setPosition {
		position = *p.Position
	}
	if setJersey {
		jersey = *p.JerseyNumber
	}
	if setFoot {
		foot = *p.PreferredFoot
	}

	u, err := scanUser(r.pool.QueryRow(ctx, q, userID,
		p.FullName, p.AvatarURL, p.Phone, p.City, p.Skill, p.Bio,
		setPosition, position, setJersey, jersey, setFoot, foot))

	if noRows(err) {
		return domain.User{}, domain.NotFound("That account doesn't exist.")
	}
	if err != nil {
		if isCheckViolation(err) {
			return domain.User{}, domain.Invalid("", "Some of those details aren't valid.").WithCause(err)
		}
		return domain.User{}, domain.Internal(err, "updating profile for user %s", userID)
	}
	return u, nil
}

// MarkEmailVerified records that someone proved control of the address.
func (r *UserRepo) MarkEmailVerified(ctx context.Context, tx DB, userID uuid.UUID) error {
	const q = `update users set email_verified_at = now() where id = $1 and email_verified_at is null`
	if _, err := tx.Exec(ctx, q, userID); err != nil {
		return domain.Internal(err, "marking email verified for user %s", userID)
	}
	return nil
}
