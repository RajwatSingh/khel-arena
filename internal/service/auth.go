package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/RajwatSingh/khel-arena/internal/domain"
	"github.com/RajwatSingh/khel-arena/internal/platform/token"
	"github.com/RajwatSingh/khel-arena/internal/postgres"
)

// UserStore is the account storage the auth service needs.
type UserStore interface {
	Create(ctx context.Context, reg domain.Registration, passwordHash string) (domain.User, error)
	ByID(ctx context.Context, id uuid.UUID) (domain.User, error)
	CredentialsByEmail(ctx context.Context, email string) (domain.Credentials, error)
	UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error
}

// SessionStore is the session storage the auth service needs.
type SessionStore interface {
	StoreRefreshToken(ctx context.Context, userID uuid.UUID, digest []byte, expiresAt time.Time, sc postgres.SessionContext) (uuid.UUID, error)
	RotateRefreshToken(ctx context.Context, oldDigest, newDigest []byte, expiresAt time.Time, sc postgres.SessionContext) (uuid.UUID, error)
	RevokeRefreshToken(ctx context.Context, digest []byte) error
	StoreVerificationToken(ctx context.Context, userID uuid.UUID, purpose string, digest []byte, expiresAt time.Time) error
	ConsumeVerificationToken(ctx context.Context, purpose string, digest []byte) (uuid.UUID, error)
}

// Session is what a successful sign-in hands back.
type Session struct {
	User                  domain.User
	AccessToken           string
	AccessTokenExpiresAt  time.Time
	RefreshToken          string
	RefreshTokenExpiresAt time.Time
}

type AuthService struct {
	users      UserStore
	sessions   SessionStore
	issuer     *token.Issuer
	clock      Clock
	refreshTTL time.Duration
}

func NewAuthService(users UserStore, sessions SessionStore, issuer *token.Issuer, clock Clock, refreshTTL time.Duration) *AuthService {
	if clock == nil {
		clock = SystemClock{}
	}
	return &AuthService{
		users:      users,
		sessions:   sessions,
		issuer:     issuer,
		clock:      clock,
		refreshTTL: refreshTTL,
	}
}

// Register creates an account and signs the new user straight in.
func (s *AuthService) Register(ctx context.Context, reg domain.Registration, sc postgres.SessionContext) (Session, error) {
	if err := reg.Validate(); err != nil {
		return Session{}, err
	}

	hash, err := token.HashPassword(reg.Password)
	if err != nil {
		return Session{}, domain.Internal(err, "hashing password")
	}

	user, err := s.users.Create(ctx, reg, hash)
	if err != nil {
		return Session{}, err
	}

	return s.issueSession(ctx, user, sc)
}

// Login exchanges an email and password for a session.
//
// Every failure returns the same message and takes the same work. A different
// message for "no such account" tells an attacker which addresses are
// registered; returning early without hashing tells them the same thing
// through response time. Hence the comparison against a dummy hash below.
func (s *AuthService) Login(ctx context.Context, email, password string, sc postgres.SessionContext) (Session, error) {
	const rejection = "That email and password don't match."

	creds, err := s.users.CredentialsByEmail(ctx, email)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		// Spend the same time we would have spent on a real account.
		_ = token.VerifyPassword(password, token.DummyHash)
		return Session{}, domain.Unauthenticated(rejection)
	case err != nil:
		return Session{}, err
	}

	if err := token.VerifyPassword(password, creds.PasswordHash); err != nil {
		if errors.Is(err, token.ErrMismatch) {
			return Session{}, domain.Unauthenticated(rejection)
		}
		return Session{}, domain.Internal(err, "verifying password for user %s", creds.UserID)
	}

	user, err := s.users.ByID(ctx, creds.UserID)
	if err != nil {
		return Session{}, err
	}

	// A successful login is the only moment the plaintext is available, so
	// it is the only chance to upgrade a hash made under weaker parameters.
	if token.NeedsRehash(creds.PasswordHash) {
		if rehashed, err := token.HashPassword(password); err == nil {
			// Best effort: failing to upgrade must not fail the login.
			_ = s.users.UpdatePassword(ctx, user.ID, rehashed)
		}
	}

	return s.issueSession(ctx, user, sc)
}

// Refresh exchanges a refresh token for a new pair.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string, sc postgres.SessionContext) (Session, error) {
	if refreshToken == "" {
		return Session{}, domain.Unauthenticated("Your session has expired. Please sign in again.")
	}

	plaintext, newDigest, err := token.NewRefreshToken()
	if err != nil {
		return Session{}, domain.Internal(err, "generating refresh token")
	}

	now := s.clock.Now()
	expiresAt := now.Add(s.refreshTTL)

	userID, err := s.sessions.RotateRefreshToken(
		ctx, token.HashRefreshToken(refreshToken), newDigest, expiresAt, sc)
	if err != nil {
		return Session{}, err
	}

	user, err := s.users.ByID(ctx, userID)
	if err != nil {
		return Session{}, err
	}

	access, accessExpiry, err := s.issuer.AccessToken(user.ID, string(user.AccountType), now)
	if err != nil {
		return Session{}, domain.Internal(err, "minting access token")
	}

	return Session{
		User:                  user,
		AccessToken:           access,
		AccessTokenExpiresAt:  accessExpiry,
		RefreshToken:          plaintext,
		RefreshTokenExpiresAt: expiresAt,
	}, nil
}

// Logout ends one session.
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil // already signed out
	}
	return s.sessions.RevokeRefreshToken(ctx, token.HashRefreshToken(refreshToken))
}

// Authenticate verifies an access token and returns who it belongs to.
//
// This reads only the token: no database round-trip on the hot path, which is
// the point of a short-lived signed token. Anything that can change under a
// live session is checked where it matters, not here.
func (s *AuthService) Authenticate(accessToken string) (uuid.UUID, domain.AccountType, error) {
	claims, err := s.issuer.Verify(accessToken)
	if err != nil {
		if errors.Is(err, token.ErrTokenExpired) {
			return uuid.Nil, "", domain.Unauthenticated("Your session has expired.")
		}
		return uuid.Nil, "", domain.Unauthenticated("Please sign in.")
	}

	userID, err := claims.UserID()
	if err != nil {
		return uuid.Nil, "", domain.Unauthenticated("Please sign in.")
	}
	return userID, domain.AccountType(claims.AccountType), nil
}

// PasswordResetTTL is how long a reset link stays usable. Short, because the
// link is a bearer credential sitting in an inbox.
const PasswordResetTTL = time.Hour

// BeginPasswordReset issues a reset token for an email address.
//
// It returns the token only when the address is registered, and reports
// success either way: telling a caller that an address is unknown turns this
// endpoint into a way to enumerate accounts. The caller mails the token if
// there is one and says the same thing to the user regardless.
func (s *AuthService) BeginPasswordReset(ctx context.Context, email string) (resetToken string, user domain.User, err error) {
	creds, err := s.users.CredentialsByEmail(ctx, email)
	if errors.Is(err, domain.ErrNotFound) {
		return "", domain.User{}, nil
	}
	if err != nil {
		return "", domain.User{}, err
	}

	plaintext, digest, err := token.NewRefreshToken()
	if err != nil {
		return "", domain.User{}, domain.Internal(err, "generating reset token")
	}

	expiresAt := s.clock.Now().Add(PasswordResetTTL)
	if err := s.sessions.StoreVerificationToken(ctx, creds.UserID, "password_reset", digest, expiresAt); err != nil {
		return "", domain.User{}, err
	}

	u, err := s.users.ByID(ctx, creds.UserID)
	if err != nil {
		return "", domain.User{}, err
	}
	return plaintext, u, nil
}

// CompletePasswordReset burns the token and sets the new password.
//
// UpdatePassword also revokes every session the account holds: someone
// resetting a password may be locking an intruder out, and leaving that
// intruder's session alive would defeat the point.
func (s *AuthService) CompletePasswordReset(ctx context.Context, resetToken, newPassword string) error {
	if err := domain.ValidatePassword(newPassword); err != nil {
		return err
	}

	userID, err := s.sessions.ConsumeVerificationToken(ctx, "password_reset", token.HashRefreshToken(resetToken))
	if err != nil {
		return err
	}

	hash, err := token.HashPassword(newPassword)
	if err != nil {
		return domain.Internal(err, "hashing password")
	}
	return s.users.UpdatePassword(ctx, userID, hash)
}

// ChangePassword updates the password for a signed-in user, after proving
// they know the current one.
func (s *AuthService) ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error {
	if err := domain.ValidatePassword(newPassword); err != nil {
		return err
	}

	user, err := s.users.ByID(ctx, userID)
	if err != nil {
		return err
	}
	creds, err := s.users.CredentialsByEmail(ctx, user.Email)
	if err != nil {
		return err
	}

	if err := token.VerifyPassword(currentPassword, creds.PasswordHash); err != nil {
		if errors.Is(err, token.ErrMismatch) {
			return domain.Invalid("current_password", "That's not your current password.")
		}
		return domain.Internal(err, "verifying current password")
	}

	hash, err := token.HashPassword(newPassword)
	if err != nil {
		return domain.Internal(err, "hashing password")
	}
	return s.users.UpdatePassword(ctx, userID, hash)
}

// issueSession mints the token pair for a user who has just proved who they are.
func (s *AuthService) issueSession(ctx context.Context, user domain.User, sc postgres.SessionContext) (Session, error) {
	now := s.clock.Now()

	access, accessExpiry, err := s.issuer.AccessToken(user.ID, string(user.AccountType), now)
	if err != nil {
		return Session{}, domain.Internal(err, "minting access token")
	}

	plaintext, digest, err := token.NewRefreshToken()
	if err != nil {
		return Session{}, domain.Internal(err, "generating refresh token")
	}

	refreshExpiry := now.Add(s.refreshTTL)
	if _, err := s.sessions.StoreRefreshToken(ctx, user.ID, digest, refreshExpiry, sc); err != nil {
		return Session{}, err
	}

	return Session{
		User:                  user,
		AccessToken:           access,
		AccessTokenExpiresAt:  accessExpiry,
		RefreshToken:          plaintext,
		RefreshTokenExpiresAt: refreshExpiry,
	}, nil
}
