// Package crypto holds the one symmetric primitive this service needs:
// authenticated encryption for secrets that have to be stored and later read
// back in the clear to be used.
//
// A merchant's gateway secret key is the case this exists for. It cannot be
// hashed — we have to present it to eSewa or Khalti verbatim — but it must not
// sit in the database as plaintext, where a single dump leaks every venue's
// credentials at once.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

// KeySize is the required length of a Box key: 32 bytes for AES-256.
const KeySize = 32

// Box seals and opens small byte slices with AES-256-GCM.
//
// GCM gives confidentiality and integrity together: Open fails rather than
// returning altered bytes if the ciphertext was tampered with or was produced
// under a different key. Each Seal draws a fresh random nonce and prepends it
// to the output, so the same plaintext never encrypts to the same bytes twice
// and callers have nothing to track.
type Box struct {
	aead cipher.AEAD
}

// NewBox builds a Box from a 32-byte key.
//
// The key comes from PAYMENT_ENC_KEY (base64 of 32 random bytes). Rotating it
// makes every existing ciphertext unreadable, so it is a deployment secret on
// the same footing as the JWT signing key.
func NewBox(key []byte) (*Box, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("crypto: key is %d bytes, want %d", len(key), KeySize)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: %w", err)
	}
	return &Box{aead: aead}, nil
}

// Seal encrypts plaintext, returning nonce || ciphertext || tag.
func (b *Box) Seal(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("crypto: reading nonce: %w", err)
	}
	// Seal appends the ciphertext to its first argument, so passing `nonce`
	// there puts the nonce in front of the result with no extra copy.
	return b.aead.Seal(nonce, nonce, plaintext, nil), nil
}

// Open reverses Seal. It fails on a wrong key, a truncated value, or any
// tampering.
func (b *Box) Open(sealed []byte) ([]byte, error) {
	ns := b.aead.NonceSize()
	if len(sealed) < ns {
		return nil, errors.New("crypto: sealed value is too short")
	}
	nonce, ciphertext := sealed[:ns], sealed[ns:]
	plaintext, err := b.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("crypto: open: %w", err)
	}
	return plaintext, nil
}
