// Package media stores the files people upload.
//
// # The storage decision, written down
//
// Files go to a directory on disk, served back by the API. Not an object
// store, and that is a choice with a shelf life: local disk means one machine
// or a shared volume, and no CDN in front. It is the right call at this size
// -- a Kathmandu futsal site with a few photographs per venue -- and the wrong
// one the day this runs on more than one box or wants images resized on the
// way out.
//
// `Store` is an interface for exactly that reason. Swapping in S3 or R2 later
// is one implementation, not a rewrite of the handlers.
//
// # What the caller does not get to decide
//
// The filename. A name that came from a browser can contain slashes, dots, or
// the name of a file already on disk, and each of those is a way out of the
// directory or over somebody else's photograph. Names here are random, and
// the extension comes from the content type we detected -- never the one the
// client claimed.
package media

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// MaxUploadBytes caps one file. Generous for a photograph of a pitch, small
// enough that a handful of uploads cannot fill a disk.
const MaxUploadBytes = 5 << 20 // 5 MiB

// allowedTypes is what may be stored, mapped to the extension it is saved
// with. An allow list, not a deny list: the question is "is this an image we
// serve", and anything absent from here is not.
var allowedTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// Sentinel errors the transport layer maps to statuses.
var (
	ErrUnsupportedType = errors.New("unsupported file type")
	ErrTooLarge        = errors.New("file too large")
)

// Store keeps uploaded files.
type Store interface {
	// Save reads a file and returns the URL path it can be fetched from.
	Save(r io.Reader) (string, error)
	// Delete removes one, addressed by the path Save returned.
	Delete(urlPath string) error
}

// DiskStore writes into a directory and serves from a URL prefix.
type DiskStore struct {
	dir    string
	prefix string
}

func NewDiskStore(dir, prefix string) (*DiskStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating media directory %s: %w", dir, err)
	}
	return &DiskStore{dir: dir, prefix: "/" + strings.Trim(prefix, "/")}, nil
}

// Prefix is the URL path files are served under.
func (s *DiskStore) Prefix() string { return s.prefix }

// Save stores one file.
//
// The content type is sniffed from the first bytes rather than read from the
// request. A client can claim any type it likes, and storing a script because
// it said "image/png" is how an upload directory becomes an execution surface.
func (s *DiskStore) Save(r io.Reader) (string, error) {
	// Bounded before anything is written: one byte past the cap is enough to
	// know it is over, and a reader that never ends cannot fill the disk.
	limited := io.LimitReader(r, MaxUploadBytes+1)

	head := make([]byte, 512)
	n, err := io.ReadFull(limited, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("reading upload: %w", err)
	}
	head = head[:n]

	ext, ok := allowedTypes[detectType(head)]
	if !ok {
		return "", ErrUnsupportedType
	}

	name, err := randomName()
	if err != nil {
		return "", err
	}
	name += ext

	// O_EXCL: if the name somehow exists, fail rather than overwrite. Sixteen
	// random bytes make that impossible in practice, and "impossible in
	// practice" is not a reason to let the one case be a silent overwrite.
	path := filepath.Join(s.dir, name)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("creating %s: %w", path, err)
	}
	defer f.Close()

	written, err := io.Copy(f, io.MultiReader(bytes.NewReader(head), limited))
	if err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("writing %s: %w", path, err)
	}
	if written > MaxUploadBytes {
		_ = os.Remove(path)
		return "", ErrTooLarge
	}

	return s.prefix + "/" + name, nil
}

// Delete removes a file this store saved.
//
// The path is checked against the prefix and reduced to its base name before
// it touches the filesystem. A stored URL is data, and data reaching os.Remove
// unexamined is a way to delete anything the process can write.
func (s *DiskStore) Delete(urlPath string) error {
	if !strings.HasPrefix(urlPath, s.prefix+"/") {
		return nil // not ours; nothing to do
	}

	name := filepath.Base(strings.TrimPrefix(urlPath, s.prefix+"/"))
	if name == "." || name == ".." || name == string(filepath.Separator) || name == "" {
		return nil
	}

	err := os.Remove(filepath.Join(s.dir, name))
	if os.IsNotExist(err) {
		return nil // already gone
	}
	return err
}

// Handler serves the stored files.
//
// http.FileServer resolves paths beneath the directory and refuses to escape
// it, which is the property that matters. Files have random names and never
// change, so they are cacheable indefinitely.
func (s *DiskStore) Handler() http.Handler {
	fs := http.FileServer(http.Dir(s.dir))

	return http.StripPrefix(s.prefix, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		// Never sniffed by the browser: an upload must not be able to become a
		// page that runs in this origin.
		w.Header().Set("X-Content-Type-Options", "nosniff")
		fs.ServeHTTP(w, r)
	}))
}

func detectType(head []byte) string {
	// DetectContentType appends a charset for some types; the media type is
	// the part before the semicolon.
	return strings.TrimSpace(strings.Split(http.DetectContentType(head), ";")[0])
}

func randomName() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating a file name: %w", err)
	}
	return hex.EncodeToString(b), nil
}
