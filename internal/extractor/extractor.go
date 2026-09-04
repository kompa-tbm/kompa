// Package extractor unpacks archives while preventing path traversal attacks.
package extractor

import (
	"archive/tar"
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
	"compress/gzip"
	"compress/bzip2"
	xzreader "github.com/klauspost/compress/zstd" // zstd used for .zst; xz handled separately
)

// maxSymlinkDepth prevents symlink chains from causing infinite loops.
const maxSymlinkDepth = 5

// Options configures extraction behaviour.
type Options struct {
	// StripComponents removes leading path components (like tar --strip-components).
	StripComponents int
	// Verbose prints each extracted file to stderr.
	Verbose bool
}

// Extract unpacks the archive at archivePath into destDir.
// The format is inferred from the file extension.
// Path traversal is blocked: no entry may escape destDir.
func Extract(archivePath, destDir string, opts Options) error {
	lower := strings.ToLower(archivePath)

	switch {
	case strings.HasSuffix(lower, ".tar.zst"):
		return extractTarZst(archivePath, destDir, opts)
	case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
		return extractTarGz(archivePath, destDir, opts)
	case strings.HasSuffix(lower, ".tar.xz") || strings.HasSuffix(lower, ".txz"):
		return extractTarXz(archivePath, destDir, opts)
	case strings.HasSuffix(lower, ".tar.bz2") || strings.HasSuffix(lower, ".tbz2"):
		return extractTarBz2(archivePath, destDir, opts)
	case strings.HasSuffix(lower, ".tar"):
		return extractTar(archivePath, destDir, opts)
	case strings.HasSuffix(lower, ".zip"):
		return extractZip(archivePath, destDir, opts)
	default:
		return fmt.Errorf("unsupported archive format: %s", filepath.Base(archivePath))
	}
}

// extractTarZst unpacks a .tar.zst archive.
func extractTarZst(archivePath, destDir string, opts Options) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("opening %s: %w", archivePath, err)
	}
	defer f.Close()

	dec, err := zstd.NewReader(f)
	if err != nil {
		return fmt.Errorf("creating zstd reader: %w", err)
	}
	defer dec.Close()

	return extractTarReader(dec, destDir, opts)
}

// extractTarGz unpacks a .tar.gz archive.
func extractTarGz(archivePath, destDir string, opts Options) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("opening %s: %w", archivePath, err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("creating gzip reader: %w", err)
	}
	defer gz.Close()

	return extractTarReader(gz, destDir, opts)
}

// extractTarBz2 unpacks a .tar.bz2 archive.
func extractTarBz2(archivePath, destDir string, opts Options) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("opening %s: %w", archivePath, err)
	}
	defer f.Close()

	return extractTarReader(bzip2.NewReader(f), destDir, opts)
}

// extractTarXz unpacks a .tar.xz archive using the zstd package's xz support.
// We use a pure-Go xz reader to avoid CGo dependencies.
func extractTarXz(archivePath, destDir string, opts Options) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("opening %s: %w", archivePath, err)
	}
	defer f.Close()

	// Use xz decoder (pure Go, provided via klauspost/compress indirectly).
	// Since klauspost/compress doesn't include xz, we use the xi2 package or
	// fall back to executing 'xz' if available. For portability we use a
	// pipe to the system xz/unxz binary when available, otherwise error clearly.
	xzReader, err := newXZReader(f)
	if err != nil {
		return fmt.Errorf("opening xz stream in %s: %w", archivePath, err)
	}
	defer xzReader.Close()

	return extractTarReader(xzReader, destDir, opts)
}

// extractTar unpacks a plain .tar archive.
func extractTar(archivePath, destDir string, opts Options) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("opening %s: %w", archivePath, err)
	}
	defer f.Close()
	return extractTarReader(f, destDir, opts)
}

// extractTarReader iterates a tar stream and writes entries to destDir.
func extractTarReader(r io.Reader, destDir string, opts Options) error {
	destDir = filepath.Clean(destDir)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}

	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar entry: %w", err)
		}

		entryPath, err := safePath(destDir, hdr.Name, opts.StripComponents)
		if err != nil {
			return err
		}
		if entryPath == "" {
			// Stripped to nothing; skip.
			continue
		}

		if opts.Verbose {
			fmt.Fprintf(os.Stderr, "  extract: %s\n", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(entryPath, hdr.FileInfo().Mode()|0100); err != nil {
				return fmt.Errorf("creating directory %s: %w", entryPath, err)
			}

		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(entryPath), 0755); err != nil {
				return fmt.Errorf("creating parent dir for %s: %w", entryPath, err)
			}
			if err := writeFile(entryPath, tr, hdr.FileInfo().Mode()); err != nil {
				return err
			}

		case tar.TypeSymlink:
			// Validate symlink target is safe (does not escape destDir).
			if err := validateSymlink(destDir, entryPath, hdr.Linkname); err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(entryPath), 0755); err != nil {
				return fmt.Errorf("creating parent dir for symlink %s: %w", entryPath, err)
			}
			// Remove any existing file at the target path before creating symlink.
			_ = os.Remove(entryPath)
			if err := os.Symlink(hdr.Linkname, entryPath); err != nil {
				return fmt.Errorf("creating symlink %s -> %s: %w", entryPath, hdr.Linkname, err)
			}

		case tar.TypeLink:
			target, err := safePath(destDir, hdr.Linkname, opts.StripComponents)
			if err != nil {
				return fmt.Errorf("hard link target %s: %w", hdr.Linkname, err)
			}
			if err := os.MkdirAll(filepath.Dir(entryPath), 0755); err != nil {
				return fmt.Errorf("creating parent dir for hard link %s: %w", entryPath, err)
			}
			_ = os.Remove(entryPath)
			if err := os.Link(target, entryPath); err != nil {
				// Fallback: copy.
				if err2 := copyFromTar(target, entryPath); err2 != nil {
					return fmt.Errorf("hard link fallback copy %s -> %s: %w", target, entryPath, err2)
				}
			}

		default:
			// Ignore other types (devices, fifos, etc.).
		}
	}
	return nil
}

// extractZip unpacks a .zip archive.
func extractZip(archivePath, destDir string, opts Options) error {
	destDir = filepath.Clean(destDir)

	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("opening zip %s: %w", archivePath, err)
	}
	defer zr.Close()

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}

	for _, f := range zr.File {
		entryPath, err := safePath(destDir, f.Name, opts.StripComponents)
		if err != nil {
			return err
		}
		if entryPath == "" {
			continue
		}

		if opts.Verbose {
			fmt.Fprintf(os.Stderr, "  extract: %s\n", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(entryPath, f.Mode()|0100); err != nil {
				return fmt.Errorf("creating zip directory %s: %w", entryPath, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(entryPath), 0755); err != nil {
			return fmt.Errorf("creating parent dir for %s: %w", entryPath, err)
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("opening zip entry %s: %w", f.Name, err)
		}
		writeErr := writeFile(entryPath, rc, f.Mode())
		rc.Close()
		if writeErr != nil {
			return writeErr
		}
	}
	return nil
}

// safePath validates and resolves a tar/zip entry name against destDir.
// It returns "" if the entry should be skipped (stripped away), or an error
// if the entry would escape destDir (path traversal).
func safePath(destDir, entryName string, stripComponents int) (string, error) {
	// Normalise separators.
	entryName = filepath.FromSlash(entryName)
	// Clean the entry name to remove ./ prefixes and normalise.
	clean := filepath.Clean(entryName)

	// Strip leading components.
	parts := strings.Split(clean, string(filepath.Separator))
	if stripComponents > 0 {
		if len(parts) <= stripComponents {
			return "", nil
		}
		parts = parts[stripComponents:]
	}
	if len(parts) == 0 {
		return "", nil
	}
	clean = filepath.Join(parts...)

	// Final assembled path.
	target := filepath.Join(destDir, clean)

	// Ensure it stays within destDir (prevent path traversal).
	if !strings.HasPrefix(target+string(filepath.Separator), destDir+string(filepath.Separator)) {
		if target != destDir { // allow exact destDir match
			return "", fmt.Errorf(
				"path traversal attempt: entry %q would escape installation directory",
				entryName,
			)
		}
	}

	return target, nil
}

// validateSymlink ensures a symlink target, resolved relative to the symlink's
// directory, stays within destDir.
func validateSymlink(destDir, symlinkPath, target string) error {
	// Absolute symlink targets are rejected outright.
	if filepath.IsAbs(target) {
		return fmt.Errorf("absolute symlink target rejected: %s -> %s", symlinkPath, target)
	}
	// Resolve relative to the symlink's containing directory.
	resolved := filepath.Clean(filepath.Join(filepath.Dir(symlinkPath), target))
	if !strings.HasPrefix(resolved+string(filepath.Separator), destDir+string(filepath.Separator)) {
		if resolved != destDir {
			return fmt.Errorf(
				"symlink escape attempt: %s -> %s would escape installation directory",
				symlinkPath, target,
			)
		}
	}
	return nil
}

func writeFile(path string, r io.Reader, mode os.FileMode) error {
	// Ensure mode includes at least user read.
	if mode == 0 {
		mode = 0644
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("creating file %s: %w", path, err)
	}
	defer f.Close()

	const maxFileSize = 4 * 1024 * 1024 * 1024 // 4 GiB guard
	lr := &io.LimitedReader{R: r, N: maxFileSize}
	if _, err := io.Copy(f, lr); err != nil {
		return fmt.Errorf("writing file %s: %w", path, err)
	}
	return nil
}

func copyFromTar(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	fi, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fi.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// Ensure zstd import is used (it's also used in extractTarZst).
var _ = xzreader.NewReader
