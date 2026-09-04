// Package checksum provides SHA-256 verification for downloaded artifacts.
package checksum

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// VerifyFile reads the file at path and compares its SHA-256 hash against expected.
// expected must be a lowercase hex-encoded string (64 characters).
func VerifyFile(path, expected string) error {
	expected = strings.ToLower(strings.TrimSpace(expected))
	if len(expected) != 64 {
		return fmt.Errorf("invalid expected checksum length %d (must be 64 hex chars)", len(expected))
	}

	actual, err := SumFile(path)
	if err != nil {
		return fmt.Errorf("computing checksum of %s: %w", path, err)
	}

	if actual != expected {
		return &MismatchError{
			Path:     path,
			Expected: expected,
			Actual:   actual,
		}
	}
	return nil
}

// VerifyReader computes the SHA-256 of all bytes read from r and compares against expected.
func VerifyReader(r io.Reader, expected string) error {
	expected = strings.ToLower(strings.TrimSpace(expected))
	if len(expected) != 64 {
		return fmt.Errorf("invalid expected checksum length %d", len(expected))
	}

	actual, err := SumReader(r)
	if err != nil {
		return fmt.Errorf("computing checksum: %w", err)
	}

	if actual != expected {
		return &MismatchError{
			Expected: expected,
			Actual:   actual,
		}
	}
	return nil
}

// SumFile computes and returns the hex-encoded SHA-256 of the file at path.
func SumFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()
	return SumReader(f)
}

// SumReader computes and returns the hex-encoded SHA-256 of all bytes read from r.
func SumReader(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", fmt.Errorf("reading data for checksum: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// SumBytes computes and returns the hex-encoded SHA-256 of b.
func SumBytes(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// WritingHasher wraps an io.Writer and computes a running SHA-256 as data passes through.
type WritingHasher struct {
	w      io.Writer
	hasher io.Writer
	sum    []byte
}

// NewWritingHasher returns a WritingHasher that writes to w and accumulates a SHA-256.
func NewWritingHasher(w io.Writer) *WritingHasher {
	h := sha256.New()
	return &WritingHasher{w: w, hasher: h, sum: nil}
}

// Write implements io.Writer, forwarding data to the underlying writer and the hash.
func (wh *WritingHasher) Write(p []byte) (int, error) {
	n, err := wh.w.Write(p)
	if n > 0 {
		_, _ = wh.hasher.Write(p[:n])
	}
	return n, err
}

// Sum returns the current hex-encoded SHA-256 digest.
func (wh *WritingHasher) Sum() string {
	if h, ok := wh.hasher.(interface{ Sum([]byte) []byte }); ok {
		return hex.EncodeToString(h.Sum(nil))
	}
	return ""
}

// MismatchError is returned when a checksum does not match.
type MismatchError struct {
	Path     string
	Expected string
	Actual   string
}

func (e *MismatchError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf(
			"checksum mismatch for %s:\n  expected: %s\n  actual:   %s\n"+
				"The downloaded file may be corrupted or tampered with.",
			e.Path, e.Expected, e.Actual,
		)
	}
	return fmt.Sprintf(
		"checksum mismatch:\n  expected: %s\n  actual:   %s",
		e.Expected, e.Actual,
	)
}

// IsMismatch returns true when err is a *MismatchError.
func IsMismatch(err error) bool {
	_, ok := err.(*MismatchError)
	return ok
}
