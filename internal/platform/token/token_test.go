package token

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestHashPasswordVerifies(t *testing.T) {
	const password = "correct horse battery staple"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("hashing: %v", err)
	}

	if err := VerifyPassword(password, hash); err != nil {
		t.Errorf("the correct password should verify: %v", err)
	}
	if err := VerifyPassword("wrong password entirely", hash); !errors.Is(err, ErrMismatch) {
		t.Errorf("a wrong password should report ErrMismatch, got %v", err)
	}
}

// The salt is what stops one cracked hash from cracking every account that
// chose the same password.
func TestHashPasswordSaltsEachHash(t *testing.T) {
	const password = "the same password twice"

	first, err := HashPassword(password)
	if err != nil {
		t.Fatalf("hashing: %v", err)
	}
	second, err := HashPassword(password)
	if err != nil {
		t.Fatalf("hashing: %v", err)
	}

	if first == second {
		t.Error("hashing the same password twice produced identical output; the salt is not random")
	}
	// Both must still verify.
	for i, h := range []string{first, second} {
		if err := VerifyPassword(password, h); err != nil {
			t.Errorf("hash %d failed to verify: %v", i, err)
		}
	}
}

func TestHashFormatIsSelfDescribing(t *testing.T) {
	hash, err := HashPassword("a password")
	if err != nil {
		t.Fatalf("hashing: %v", err)
	}

	// The PHC string records the parameters, so raising them later does not
	// invalidate hashes made under the old ones.
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=") {
		t.Errorf("hash is not in the expected PHC format: %q", hash)
	}
	if parts := strings.Split(hash, "$"); len(parts) != 6 {
		t.Errorf("hash has %d segments, want 6: %q", len(parts), hash)
	}
}

func TestVerifyRejectsMalformedHashes(t *testing.T) {
	malformed := []string{
		"",
		"not-a-hash",
		"$argon2id$",
		"$bcrypt$v=19$m=65536,t=3,p=4$c2FsdA$aGFzaA",       // wrong algorithm
		"$argon2id$v=99$m=65536,t=3,p=4$c2FsdA$aGFzaA",     // unknown version
		"$argon2id$v=19$m=0,t=0,p=0$c2FsdA$aGFzaA",         // zero parameters
		"$argon2id$v=19$m=65536,t=3,p=4$!!!not-base64$aGFzaA",
	}

	for _, h := range malformed {
		t.Run(h, func(t *testing.T) {
			err := VerifyPassword("anything", h)
			if err == nil {
				t.Fatal("a malformed hash must never verify")
			}
			if errors.Is(err, ErrMismatch) {
				t.Error("a malformed hash should report ErrInvalidHash, not a plain mismatch")
			}
		})
	}
}

func TestNeedsRehash(t *testing.T) {
	current, err := HashPassword("a password")
	if err != nil {
		t.Fatalf("hashing: %v", err)
	}
	if NeedsRehash(current) {
		t.Error("a hash made with the current parameters should not need rehashing")
	}

	// A hash from weaker settings must be flagged for upgrade at next login.
	weak := "$argon2id$v=19$m=4096,t=1,p=1$c2FsdHNhbHRzYWx0c2E$aGFzaGhhc2hoYXNoaGFzaGhhc2hoYXNoaGE"
	if !NeedsRehash(weak) {
		t.Error("a hash made with weaker parameters should need rehashing")
	}

	if !NeedsRehash("garbage") {
		t.Error("an unparseable hash should be treated as needing replacement")
	}
}

// Login compares against this when no account matches, so an unknown address
// costs the same as a real one.
func TestDummyHashIsUsable(t *testing.T) {
	if DummyHash == "" {
		t.Fatal("the dummy hash should be derived at startup")
	}
	if err := VerifyPassword("whatever someone guesses", DummyHash); !errors.Is(err, ErrMismatch) {
		t.Errorf("the dummy hash should compare cleanly and never match, got %v", err)
	}
}

// ---------------------------------------------------------------------------

func testIssuer(t *testing.T) *Issuer {
	t.Helper()
	iss, err := NewIssuer([]byte("a-test-secret-that-is-long-enough-to-pass"), "khel-arena-test", 15*time.Minute)
	if err != nil {
		t.Fatalf("building issuer: %v", err)
	}
	return iss
}

func TestAccessTokenRoundTrip(t *testing.T) {
	iss := testIssuer(t)
	userID := uuid.New()
	now := time.Now().UTC()

	signed, expiresAt, err := iss.AccessToken(userID, "player", now)
	if err != nil {
		t.Fatalf("minting: %v", err)
	}
	if !expiresAt.After(now) {
		t.Error("the token should expire in the future")
	}

	claims, err := iss.Verify(signed)
	if err != nil {
		t.Fatalf("verifying: %v", err)
	}

	gotID, err := claims.UserID()
	if err != nil {
		t.Fatalf("reading the subject: %v", err)
	}
	if gotID != userID {
		t.Errorf("subject = %s, want %s", gotID, userID)
	}
	if claims.AccountType != "player" {
		t.Errorf("account type = %q, want %q", claims.AccountType, "player")
	}
}

func TestVerifyRejectsExpiredTokens(t *testing.T) {
	iss := testIssuer(t)
	// Minted far enough in the past that it is expired now.
	signed, _, err := iss.AccessToken(uuid.New(), "player", time.Now().UTC().Add(-time.Hour))
	if err != nil {
		t.Fatalf("minting: %v", err)
	}

	if _, err := iss.Verify(signed); !errors.Is(err, ErrTokenExpired) {
		t.Errorf("expected ErrTokenExpired, got %v", err)
	}
}

func TestVerifyRejectsAForeignSignature(t *testing.T) {
	mine := testIssuer(t)
	theirs, err := NewIssuer([]byte("a-completely-different-secret-key-here!!"), "khel-arena-test", 15*time.Minute)
	if err != nil {
		t.Fatalf("building the other issuer: %v", err)
	}

	signed, _, err := theirs.AccessToken(uuid.New(), "player", time.Now().UTC())
	if err != nil {
		t.Fatalf("minting: %v", err)
	}

	if _, err := mine.Verify(signed); !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("a token signed with another key must be rejected, got %v", err)
	}
}

// The classic JWT forgery: a token declaring "alg: none" verifies against any
// key if the parser trusts the token's own header.
func TestVerifyRejectsTheNoneAlgorithm(t *testing.T) {
	iss := testIssuer(t)

	forged := jwt.NewWithClaims(jwt.SigningMethodNone, Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			Issuer:    "khel-arena-test",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		AccountType: "arena_owner",
	})
	signed, err := forged.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("building the forged token: %v", err)
	}

	if _, err := iss.Verify(signed); err == nil {
		t.Fatal("an unsigned token was accepted; the signing method is not pinned")
	}
}

func TestVerifyRejectsAnotherIssuer(t *testing.T) {
	iss := testIssuer(t)

	// Same secret, different issuer name: a token minted by a sibling
	// service must not authenticate here.
	other, err := NewIssuer([]byte("a-test-secret-that-is-long-enough-to-pass"), "some-other-service", 15*time.Minute)
	if err != nil {
		t.Fatalf("building the other issuer: %v", err)
	}
	signed, _, err := other.AccessToken(uuid.New(), "player", time.Now().UTC())
	if err != nil {
		t.Fatalf("minting: %v", err)
	}

	if _, err := iss.Verify(signed); err == nil {
		t.Error("a token from another issuer should be rejected")
	}
}

func TestVerifyRejectsGarbage(t *testing.T) {
	iss := testIssuer(t)
	for _, s := range []string{"", "not.a.token", "a.b.c", strings.Repeat("x", 500)} {
		if _, err := iss.Verify(s); err == nil {
			t.Errorf("expected %q to be rejected", s)
		}
	}
}

func TestNewIssuerRejectsAWeakSecret(t *testing.T) {
	if _, err := NewIssuer([]byte("too-short"), "khel-arena", time.Minute); err == nil {
		t.Error("a secret under 32 bytes should be refused")
	}
	if _, err := NewIssuer(make([]byte, 32), "khel-arena", 0); err == nil {
		t.Error("a non-positive TTL should be refused")
	}
}

func TestRefreshTokensAreUniqueAndHashStably(t *testing.T) {
	first, firstDigest, err := NewRefreshToken()
	if err != nil {
		t.Fatalf("generating: %v", err)
	}
	second, _, err := NewRefreshToken()
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	if first == second {
		t.Error("two refresh tokens came out identical")
	}
	if len(firstDigest) != 32 {
		t.Errorf("digest is %d bytes, want a 32-byte SHA-256", len(firstDigest))
	}

	// The digest must be reproducible, since lookup is by digest.
	if got := HashRefreshToken(first); string(got) != string(firstDigest) {
		t.Error("hashing the same token twice produced different digests")
	}
	// The plaintext must not be recoverable from what is stored.
	if strings.Contains(string(firstDigest), first) {
		t.Error("the digest contains the plaintext token")
	}
}
