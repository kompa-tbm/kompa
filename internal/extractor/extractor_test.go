package extractor

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
)

// createTarGz builds an in-memory .tar.gz and writes it to path.
func createTarGz(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(tw, content); err != nil {
			t.Fatal(err)
		}
	}
}

// createTarZst builds a .tar.zst and writes it to path.
func createTarZst(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	enc, err := zstd.NewWriter(f)
	if err != nil {
		t.Fatal(err)
	}
	defer enc.Close()

	tw := tar.NewWriter(enc)
	defer tw.Close()

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(tw, content); err != nil {
			t.Fatal(err)
		}
	}
}

// createZip builds a .zip and writes it to path.
func createZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, content); err != nil {
			t.Fatal(err)
		}
	}
}

func TestExtractTarGz(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "test.tar.gz")
	destDir := filepath.Join(tmp, "out")

	files := map[string]string{
		"top/file1.txt": "hello",
		"top/file2.txt": "world",
		"top/sub/a.txt": "nested",
	}
	createTarGz(t, archivePath, files)

	if err := Extract(archivePath, destDir, Options{StripComponents: 1}); err != nil {
		t.Fatalf("Extract(.tar.gz) error = %v", err)
	}

	assertFile(t, filepath.Join(destDir, "file1.txt"), "hello")
	assertFile(t, filepath.Join(destDir, "file2.txt"), "world")
	assertFile(t, filepath.Join(destDir, "sub", "a.txt"), "nested")
}

func TestExtractTarZst(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "test.tar.zst")
	destDir := filepath.Join(tmp, "out")

	files := map[string]string{
		"pkg/bin/mytool": "#!/bin/sh\necho hi",
		"pkg/README":     "readme content",
	}
	createTarZst(t, archivePath, files)

	if err := Extract(archivePath, destDir, Options{StripComponents: 1}); err != nil {
		t.Fatalf("Extract(.tar.zst) error = %v", err)
	}

	assertFile(t, filepath.Join(destDir, "bin", "mytool"), "#!/bin/sh\necho hi")
	assertFile(t, filepath.Join(destDir, "README"), "readme content")
}

func TestExtractZip(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "test.zip")
	destDir := filepath.Join(tmp, "out")

	files := map[string]string{
		"top/config.yaml": "key: value",
		"top/lib/util.so": "ELF binary",
	}
	createZip(t, archivePath, files)

	if err := Extract(archivePath, destDir, Options{StripComponents: 1}); err != nil {
		t.Fatalf("Extract(.zip) error = %v", err)
	}

	assertFile(t, filepath.Join(destDir, "config.yaml"), "key: value")
	assertFile(t, filepath.Join(destDir, "lib", "util.so"), "ELF binary")
}

func TestExtractUnsupportedFormat(t *testing.T) {
	tmp := t.TempDir()
	err := Extract(filepath.Join(tmp, "file.rar"), tmp, Options{})
	if err == nil {
		t.Error("Extract(.rar) expected error for unsupported format, got nil")
	}
}

func TestSafePath_TraversalBlocked(t *testing.T) {
	// Use a real temp directory so paths are valid on all OS.
	destDir := t.TempDir()
	dangerous := []string{
		"../../etc/passwd",
		"../../../root/.ssh/authorized_keys",
		"subdir/../../etc/shadow",
	}
	for _, entry := range dangerous {
		t.Run(entry, func(t *testing.T) {
			result, err := safePath(destDir, entry, 0)
			if err == nil && result != "" {
				// If no error, result must still stay within destDir.
				if !strings.HasPrefix(result+string(filepath.Separator), destDir+string(filepath.Separator)) &&
					result != destDir {
					t.Errorf("safePath(%q) = %q, escapes destDir %q", entry, result, destDir)
				}
			}
		})
	}
}

func TestSafePath_StripComponents(t *testing.T) {
	destDir := t.TempDir()

	// With strip=1, "top/file.txt" becomes "file.txt" inside destDir.
	result, err := safePath(destDir, "top/file.txt", 1)
	if err != nil {
		t.Fatalf("safePath error = %v", err)
	}
	expected := filepath.Join(destDir, "file.txt")
	if result != expected {
		t.Errorf("safePath(strip=1) = %q, want %q", result, expected)
	}

	// With strip=1 on a top-level-only path, result should be empty (skip).
	result2, err2 := safePath(destDir, "top", 1)
	if err2 != nil {
		t.Fatalf("safePath(top, strip=1) error = %v", err2)
	}
	if result2 != "" {
		t.Errorf("safePath(top, strip=1) = %q, want empty (skip)", result2)
	}
}

func TestValidateSymlink_Absolute(t *testing.T) {
	destDir := t.TempDir()
	// An absolute target pointing outside destDir must be rejected.
	absTarget := filepath.Join(t.TempDir(), "outside")
	err := validateSymlink(destDir, filepath.Join(destDir, "link"), absTarget)
	if err == nil {
		t.Error("validateSymlink with absolute external target: expected error, got nil")
	}
}

func TestValidateSymlink_Escape(t *testing.T) {
	destDir := t.TempDir()
	symlinkPath := filepath.Join(destDir, "link")
	// "../../etc/passwd" would escape destDir on any real system.
	err := validateSymlink(destDir, symlinkPath, "../../etc/passwd")
	if err == nil {
		t.Error("validateSymlink that escapes destDir: expected error, got nil")
	}
}

func TestValidateSymlink_OK(t *testing.T) {
	destDir := t.TempDir()
	// ../lib/libfoo.so relative to destDir/bin/link resolves to destDir/lib/libfoo.so — safe.
	symlinkPath := filepath.Join(destDir, "bin", "link")
	err := validateSymlink(destDir, symlinkPath, "../lib/libfoo.so")
	if err != nil {
		t.Errorf("validateSymlink with safe relative target: unexpected error: %v", err)
	}
}

// assertFile checks that path exists and contains want.
func assertFile(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("expected file %s: %v", path, err)
		return
	}
	if string(data) != want {
		t.Errorf("file %s = %q, want %q", path, string(data), want)
	}
}
