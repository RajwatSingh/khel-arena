package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Session token errors. Callers distinguish these to decide whether to ask
// for a refresh or a fresh login.
var (
	ErrTokenExpired = errors.New("token expired")
	ErrTokenInvalid = errors.New("token invalid")
)

// Claims is what an access token carries.
//
// Only the identity and the account type: enough to authorize a request
// without a database round-trip, and little enough that a token going stale
// cannot grant something it should not. Anything that changes -- a username,
// a ban -- is read from the database at the point it matters.
type Claims struct {
	jwt.RegisteredClaims
	AccountType string `json:"act"`
}

func (c Claims) UserID() (uuid.UUID, error) {
	id, err := uuid.Parse(c.Subject)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: subject %q is not a user id", ErrTokenInvalid, c.Subject)
	}
	return id, nil
}

// Issuer mints and verifies access tokens.
type Issuer struct {
	secret    []byte
	issuer    string
	accessTTL time.Duration
}

func NewIssuer(secret []byte, issuerName string, accessTTL time.Duration) (*Issuer, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("jwt secret must be at least 32 bytes, got %d", len(secret))
	}
	if accessTTL <= 0 {
		return nil, fmt.Errorf("access token TTL must be positive, got %s", accessTTL)
	}
	return &Issuer{secret: secret, issuer: issuerName, accessTTL: accessTTL}, nil
}

// AccessToken mints a signed token for a user.
func (i *Issuer) AccessToken(userID uuid.UUID, accountType string, now time.Time) (string, time.Time, error) {
	expiresAt := now.Add(i.accessTTL)

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    i.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        uuid.NewString(),
		},
		AccountType: accountType,
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(i.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("signing access token: %w", err)
	}
	return signed, expiresAt, nil
}

// Verify checks a token's signature and validity window and returns its claims.
//
// The signing method is pinned to HS256. Accepting whatever the token's own
// header names is the classic JWT forgery: a token declaring "alg: none", or
// one signed with the public key of an asymmetric pair, would otherwise verify.
func (i *Issuer) Verify(tokenString string) (Claims, error) {
	var claims Claims

	_, err := jwt.ParseWithClaims(tokenString, &claims,
		func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("%w: unexpected signing method %v", ErrTokenInvalid, t.Header["alg"])
			}
			return i.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(i.issuer),
		jwt.WithExpirationRequired(),
	)

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return Claims{}, ErrTokenExpired
		}
		return Claims{}, fmt.Errorf("%w: %s", ErrTokenInvalid, err)
	}
	return claims, nil
}

// ---------------------------------------------------------------------------
// Refresh tokens
//
// Refresh tokens are opaque random strings, not JWTs. A JWT is valid until it
// expires and cannot be withdrawn; a refresh token lives for weeks, so it must
// be revocable. Only a SHA-256 digest is stored, so a database leak does not
// hand out live sessions.
// ---------------------------------------------------------------------------

// refreshTokenBytes is the entropy in a refresh token. 32 bytes is beyond
// brute-forcing and keeps the encoded form a manageable 43 characters.
const refreshTokenBytes = 32

// NewRefreshToken returns a fresh token and the digest to store against it.
// The plaintext is returned once, to the client; it is never persisted.
func NewRefreshToken() (plaintext string, digest []byte, err error) {
	buf := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("generating refresh token: %w", err)
	}
	plaintext = base64.RawURLEncoding.EncodeToString(buf)
	return plaintext, HashRefreshToken(plaintext), nil
}

// HashRefreshToken digests a token for storage and lookup.
//
// A plain SHA-256 is right here, unlike for passwords: the token already has
// 256 bits of entropy, so there is nothing to guess and no need to make
// verification slow. Slow hashing would only cost the server on every refresh.
func HashRefreshToken(plaintext string) []byte {
	sum := sha256.Sum256([]byte(plaintext))
	return sum[:]
}
