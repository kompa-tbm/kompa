// Package downloader handles HTTP downloads with progress reporting, resume support,
// retry logic, and checksum verification.
package downloader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"
)

// Options configures a download operation.
type Options struct {
	// MaxRetries is the number of times to retry on transient failure.
	MaxRetries int
	// Timeout is the per-attempt HTTP timeout.
	Timeout time.Duration
	// ExpectedSHA256 is the hex-encoded expected checksum; empty means skip verify.
	ExpectedSHA256 string
	// ShowProgress enables the terminal progress bar.
	ShowProgress bool
	// Label is shown in the progress bar.
	Label string
	// Resume attempts to resume a partial download if a .part file exists.
	Resume bool
}

// DefaultOptions returns sensible download defaults.
func DefaultOptions() Options {
	return Options{
		MaxRetries:   3,
		Timeout:      5 * time.Minute,
		ShowProgress: true,
		Resume:       true,
	}
}

// Result holds the outcome of a completed download.
type Result struct {
	Path   string
	SHA256 string
	Size   int64
}

// Download fetches url and saves it to destPath.
// It respects context cancellation and retries on transient network errors.
func Download(ctx context.Context, url, destPath string, opts Options) (*Result, error) {
	if !strings.HasPrefix(url, "https://") {
		return nil, fmt.Errorf("refusing to download from non-HTTPS URL: %s", url)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return nil, fmt.Errorf("creating download directory: %w", err)
	}

	partPath := destPath + ".part"
	var lastErr error

	for attempt := 0; attempt <= opts.MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential back-off: 1s, 2s, 4s.
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		result, err := attemptDownload(ctx, url, destPath, partPath, opts)
		if err == nil {
			return result, nil
		}

		if !isRetryable(err) {
			return nil, err
		}
		lastErr = err
	}

	return nil, fmt.Errorf("download failed after %d attempts: %w", opts.MaxRetries+1, lastErr)
}

func attemptDownload(ctx context.Context, url, destPath, partPath string, opts Options) (*Result, error) {
	client := &http.Client{Timeout: opts.Timeout}

	// Check for a partial download to resume.
	var startByte int64
	if opts.Resume {
		if fi, err := os.Stat(partPath); err == nil {
			startByte = fi.Size()
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "kompa/1.0")

	if startByte > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", startByte))
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, &transientError{err: fmt.Errorf("HTTP request: %w", err)}
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// Full response — ignore any partial file.
		startByte = 0
		if err := os.Remove(partPath); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("removing stale partial file: %w", err)
		}
	case http.StatusPartialContent:
		// Server supports resume.
	case http.StatusRequestedRangeNotSatisfiable:
		// The existing partial file is already complete; fall through.
		startByte = 0
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
		return nil, &transientError{err: fmt.Errorf("HTTP %d: server temporarily unavailable", resp.StatusCode)}
	default:
		return nil, fmt.Errorf("unexpected HTTP status %d downloading %s", resp.StatusCode, url)
	}

	// Open the part file (append if resuming, truncate otherwise).
	var flags int
	if startByte > 0 {
		flags = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	} else {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	f, err := os.OpenFile(partPath, flags, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening part file: %w", err)
	}
	defer f.Close()

	// Set up hashing (full file, including already-written bytes).
	hasher := sha256.New()
	if startByte > 0 && opts.ExpectedSHA256 != "" {
		// Re-hash already-downloaded bytes.
		existing, err := os.Open(partPath)
		if err != nil {
			return nil, fmt.Errorf("opening partial file for hashing: %w", err)
		}
		if _, err := io.Copy(hasher, existing); err != nil {
			existing.Close()
			return nil, fmt.Errorf("hashing partial file: %w", err)
		}
		existing.Close()
	}

	var totalSize int64
	if resp.ContentLength > 0 {
		totalSize = startByte + resp.ContentLength
	}

	// Set up progress bar.
	var bar *progressbar.ProgressBar
	if opts.ShowProgress {
		label := opts.Label
		if label == "" {
			label = filepath.Base(destPath)
		}
		bar = progressbar.NewOptions64(
			totalSize,
			progressbar.OptionSetDescription(label),
			progressbar.OptionSetWriter(os.Stderr),
			progressbar.OptionShowBytes(true),
			progressbar.OptionSetWidth(40),
			progressbar.OptionThrottle(65*time.Millisecond),
			progressbar.OptionShowCount(),
			progressbar.OptionOnCompletion(func() { fmt.Fprint(os.Stderr, "\n") }),
			progressbar.OptionSpinnerType(14),
			progressbar.OptionFullWidth(),
			progressbar.OptionSetRenderBlankState(true),
		)
		_ = bar.Add64(startByte)
	}

	// Tee: write to file, hash, and progress bar simultaneously.
	var writers []io.Writer
	writers = append(writers, f)
	writers = append(writers, hasher)
	if bar != nil {
		writers = append(writers, bar)
	}
	mw := io.MultiWriter(writers...)

	written, err := io.Copy(mw, resp.Body)
	if err != nil {
		return nil, &transientError{err: fmt.Errorf("reading response body: %w", err)}
	}

	totalWritten := startByte + written
	actualSHA256 := hex.EncodeToString(hasher.Sum(nil))

	// Verify checksum if requested.
	if opts.ExpectedSHA256 != "" {
		expected := strings.ToLower(strings.TrimSpace(opts.ExpectedSHA256))
		if actualSHA256 != expected {
			// Remove the corrupted part file so next attempt starts fresh.
			_ = os.Remove(partPath)
			return nil, fmt.Errorf(
				"checksum mismatch for %s:\n  expected: %s\n  actual:   %s\nThe file may be corrupted or tampered with.",
				filepath.Base(destPath), expected, actualSHA256,
			)
		}
	}

	// Atomic rename: part → dest.
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("closing part file: %w", err)
	}
	if err := os.Rename(partPath, destPath); err != nil {
		return nil, fmt.Errorf("finalising download (rename %s → %s): %w", partPath, destPath, err)
	}

	return &Result{
		Path:   destPath,
		SHA256: actualSHA256,
		Size:   totalWritten,
	}, nil
}

// transientError wraps errors that are safe to retry.
type transientError struct {
	err error
}

func (e *transientError) Error() string { return e.err.Error() }
func (e *transientError) Unwrap() error { return e.err }

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*transientError)
	return ok
}
