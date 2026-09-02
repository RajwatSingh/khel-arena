package media

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A one-pixel PNG, used because the store sniffs the type from the bytes
// rather than believing what it was told.
var pngBytes = []byte{
	0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89,
}

func newStore(t *testing.T) *DiskStore {
	t.Helper()

	store, err := NewDiskStore(t.TempDir(), "/media")
	if err != nil {
		t.Fatalf("creating store: %v", err)
	}
	return store
}

func TestSaveStoresAnImage(t *testing.T) {
	store := newStore(t)

	url, err := store.Save(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("saving: %v", err)
	}

	if !strings.HasPrefix(url, "/media/") || !strings.HasSuffix(url, ".png") {
		t.Errorf("url = %q, want /media/<name>.png", url)
	}
	// The extension comes from what was detected, not from anything a caller
	// said -- there is nowhere for a caller to say it.
	if _, err := os.Stat(filepath.Join(store.dir, filepath.Base(url))); err != nil {
		t.Errorf("file not on disk: %v", err)
	}
}

// Storing a script because it claimed to be an image is how an upload
// directory becomes an execution surface.
func TestSaveRefusesWhatIsNotAnImage(t *testing.T) {
	store := newStore(t)

	for name, body := range map[string][]byte{
		"html":   []byte("<!doctype html><script>alert(1)</script>"),
		"script": []byte("#!/bin/sh\nrm -rf /\n"),
		"empty":  {},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := store.Save(bytes.NewReader(body))
			if !errors.Is(err, ErrUnsupportedType) {
				t.Errorf("err = %v, want ErrUnsupportedType", err)
			}
		})
	}

	// And nothing was written.
	entries, _ := os.ReadDir(store.dir)
	if len(entries) != 0 {
		t.Errorf("%d files were written despite refusal", len(entries))
	}
}

func TestSaveRefusesSomethingTooLarge(t *testing.T) {
	store := newStore(t)

	// A valid PNG header followed by more bytes than the cap allows.
	big := append(append([]byte{}, pngBytes...), bytes.Repeat([]byte{0}, MaxUploadBytes+10)...)

	if _, err := store.Save(bytes.NewReader(big)); !errors.Is(err, ErrTooLarge) {
		t.Errorf("err = %v, want ErrTooLarge", err)
	}

	// The partial write is cleaned up rather than left on disk.
	entries, _ := os.ReadDir(store.dir)
	if len(entries) != 0 {
		t.Errorf("%d files left behind after an oversized upload", len(entries))
	}
}

// Two uploads of identical bytes must not collide: names are random, not
// derived from content, so one photograph never overwrites another.
func TestNamesAreUnique(t *testing.T) {
	store := newStore(t)

	first, err := store.Save(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("first save: %v", err)
	}
	second, err := store.Save(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("second save: %v", err)
	}

	if first == second {
		t.Errorf("both uploads landed on %q", first)
	}
}

// A stored URL is data. Data reaching os.Remove unexamined is a way to delete
// anything the process can write.
func TestDeleteCannotEscapeTheDirectory(t *testing.T) {
	store := newStore(t)

	outside := filepath.Join(filepath.Dir(store.dir), "precious.txt")
	if err := os.WriteFile(outside, []byte("do not delete"), 0o644); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	for _, path := range []string{
		"/media/../precious.txt",
		"/media/../../precious.txt",
		"/etc/passwd",
		"../precious.txt",
		"/media/",
		"/media/..",
	} {
		if err := store.Delete(path); err != nil {
			t.Errorf("Delete(%q) = %v", path, err)
		}
	}

	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("a file outside the media directory was deleted: %v", err)
	}
}

func TestDeleteRemovesItsOwn(t *testing.T) {
	store := newStore(t)

	url, err := store.Save(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("saving: %v", err)
	}
	if err := store.Delete(url); err != nil {
		t.Fatalf("deleting: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store.dir, filepath.Base(url))); !os.IsNotExist(err) {
		t.Error("the file is still there")
	}
	// Deleting twice is not an error: the outcome asked for is the outcome.
	if err := store.Delete(url); err != nil {
		t.Errorf("second delete: %v", err)
	}
}

func TestHandlerServesWithoutSniffing(t *testing.T) {
	store := newStore(t)

	url, err := store.Save(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("saving: %v", err)
	}

	w := httptest.NewRecorder()
	store.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, url, nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	// An upload must never become a page that runs in this origin.
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("nosniff is missing")
	}
	if !strings.Contains(w.Header().Get("Cache-Control"), "immutable") {
		t.Errorf("Cache-Control = %q", w.Header().Get("Cache-Control"))
	}
}

// The file server resolves beneath the directory and refuses to escape it.
func TestHandlerCannotEscapeTheDirectory(t *testing.T) {
	store := newStore(t)

	outside := filepath.Join(filepath.Dir(store.dir), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	w := httptest.NewRecorder()
	store.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/media/../secret.txt", nil))

	if w.Code == http.StatusOK && strings.Contains(w.Body.String(), "secret") {
		t.Fatal("the handler served a file outside its directory")
	}
}
