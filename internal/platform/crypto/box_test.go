package crypto

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func newKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, KeySize)
	if _, err := rand.Read(k); err != nil {
		t.Fatalf("generating key: %v", err)
	}
	return k
}

func TestSealOpenRoundTrips(t *testing.T) {
	box, err := NewBox(newKey(t))
	if err != nil {
		t.Fatalf("NewBox: %v", err)
	}

	secret := []byte("8gBm/:&EnhH.1/q") // an eSewa-shaped key
	sealed, err := box.Seal(secret)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if bytes.Contains(sealed, secret) {
		t.Fatal("the plaintext is visible in the ciphertext")
	}

	opened, err := box.Open(sealed)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !bytes.Equal(opened, secret) {
		t.Fatalf("round trip = %q, want %q", opened, secret)
	}
}

func TestSealIsNonDeterministic(t *testing.T) {
	box, _ := NewBox(newKey(t))
	a, _ := box.Seal([]byte("same"))
	b, _ := box.Seal([]byte("same"))
	if bytes.Equal(a, b) {
		t.Fatal("two seals of the same plaintext produced identical bytes")
	}
}

func TestOpenRejectsWrongKeyAndTampering(t *testing.T) {
	box, _ := NewBox(newKey(t))
	sealed, _ := box.Seal([]byte("secret"))

	other, _ := NewBox(newKey(t))
	if _, err := other.Open(sealed); err == nil {
		t.Error("a different key opened the ciphertext")
	}

	sealed[len(sealed)-1] ^= 0xff
	if _, err := box.Open(sealed); err == nil {
		t.Error("a tampered ciphertext was accepted")
	}
}

func TestNewBoxRejectsWrongKeySize(t *testing.T) {
	if _, err := NewBox(make([]byte, 16)); err == nil {
		t.Error("a 16-byte key was accepted")
	}
}
