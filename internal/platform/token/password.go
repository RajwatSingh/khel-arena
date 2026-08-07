// Package token handles the two secrets this service issues: password hashes
// and the JWTs that carry a session.
package token

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters.
//
// Argon2id, rather than bcrypt, because it is memory-hard: an attacker with
// GPUs cannot trade silicon for memory bandwidth as cheaply. The figures below
// are the RFC 9106 second recommended profile -- 64 MiB with three passes --
// which costs roughly 50ms per hash on commodity hardware. That is
// deliberately slow for one login and ruinous for an offline crack.
//
// Every hash records the parameters it was made with, so these can be raised
// later without invalidating existing passwords: NeedsRehash reports which
// stored hashes are behind, and the next successful login upgrades them.
const (
	argonMemoryKiB  uint32 = 64 * 1024
	argonIterations uint32 = 3
	argonKeyLength  uint32 = 32
	argonSaltLength        = 16
)

// argonParallelism is capped so a machine with many cores does not produce
// hashes that a smaller one cannot verify in reasonable time.
var argonParallelism = uint8(min(runtime.NumCPU(), 4))

// ErrMismatch means the password did not match the hash. It carries no detail
// on purpose: everything a caller may safely learn is that it was wrong.
var ErrMismatch = errors.New("password does not match")

// ErrInvalidHash means the stored hash could not be parsed -- corrupted data
// or a hash from a scheme this build does not know.
var ErrInvalidHash = errors.New("invalid password hash")

// HashPassword derives an Argon2id hash with a fresh random salt.
//
// The result is a PHC string:
//
//	$argon2id$v=19$m=65536,t=3,p=4$<salt>$<hash>
//
// Self-describing, so a hash made under one set of parameters stays verifiable
// after they change.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating salt: %w", err)
	}

	key := argon2.IDKey([]byte(password), salt,
		argonIterations, argonMemoryKiB, argonParallelism, argonKeyLength)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemoryKiB, argonIterations, argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword reports whether password matches encodedHash.
//
// The comparison is constant-time: a byte-by-byte check that returns early
// leaks, through timing, how much of a guess was right.
func VerifyPassword(password, encodedHash string) error {
	params, salt, want, err := decodeHash(encodedHash)
	if err != nil {
		return err
	}

	got := argon2.IDKey([]byte(password), salt,
		params.iterations, params.memoryKiB, params.parallelism, uint32(len(want)))

	if subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrMismatch
	}
	return nil
}

// NeedsRehash reports whether a stored hash was made with weaker parameters
// than the current ones. Call it after a successful login: that is the only
// moment the plaintext is available to re-hash with.
func NeedsRehash(encodedHash string) bool {
	params, _, _, err := decodeHash(encodedHash)
	if err != nil {
		// Unparseable hashes cannot be verified against anyway; treat them
		// as needing replacement rather than silently keeping them.
		return true
	}
	return params.memoryKiB < argonMemoryKiB ||
		params.iterations < argonIterations ||
		params.parallelism < argonParallelism
}

// DummyHash is a valid hash of a value nobody knows.
//
// Login compares against this when no account matches the email, so a request
// for an unknown address costs the same as one for a real account. Without
// it, response time answers "is this email registered?" for free.
var DummyHash = func() string {
	h, err := HashPassword("khel-arena-timing-equaliser-not-a-real-password")
	if err != nil {
		panic("token: cannot derive dummy hash: " + err.Error())
	}
	return h
}()

type argonParams struct {
	memoryKiB   uint32
	iterations  uint32
	parallelism uint8
}

func decodeHash(encoded string) (argonParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" {
		return argonParams{}, nil, nil, ErrInvalidHash
	}
	if parts[1] != "argon2id" {
		return argonParams{}, nil, nil, fmt.Errorf("%w: unsupported algorithm %q", ErrInvalidHash, parts[1])
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return argonParams{}, nil, nil, ErrInvalidHash
	}
	if version != argon2.Version {
		return argonParams{}, nil, nil, fmt.Errorf("%w: unsupported version %d", ErrInvalidHash, version)
	}

	var p argonParams
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memoryKiB, &p.iterations, &p.parallelism); err != nil {
		return argonParams{}, nil, nil, ErrInvalidHash
	}
	if p.memoryKiB == 0 || p.iterations == 0 || p.parallelism == 0 {
		return argonParams{}, nil, nil, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return argonParams{}, nil, nil, ErrInvalidHash
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return argonParams{}, nil, nil, ErrInvalidHash
	}

	return p, salt, key, nil
}
