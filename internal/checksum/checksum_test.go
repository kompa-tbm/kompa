package checksum

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// knownContent and its precomputed SHA-256.
const knownContent = "Hello, Kompa!\n"
const knownSHA256 = "d4dfd25175e0694c7465919f19fe9d9af4f52e0e202933c6ff392b609d1bc306"

func TestSumBytes(t *testing.T) {
	got := SumBytes([]byte(knownContent))
	if got != knownSHA256 {
		t.Errorf("SumBytes() = %q, want %q", got, knownSHA256)
	}
}

func TestSumReader(t *testing.T) {
	r := bytes.NewBufferString(knownContent)
	got, err := SumReader(r)
	if err != nil {
		t.Fatalf("SumReader() error = %v", err)
	}
	if got != knownSHA256 {
		t.Errorf("SumReader() = %q, want %q", got, knownSHA256)
	}
}

func TestSumFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")
	if err := os.WriteFile(path, []byte(knownContent), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := SumFile(path)
	if err != nil {
		t.Fatalf("SumFile() error = %v", err)
	}
	if got != knownSHA256 {
		t.Errorf("SumFile() = %q, want %q", got, knownSHA256)
	}
}

func TestSumFileMissing(t *testing.T) {
	_, err := SumFile("/nonexistent/path/file.txt")
	if err == nil {
		t.Error("SumFile() on missing file: expected error, got nil")
	}
}

func TestVerifyFile_OK(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")
	if err := os.WriteFile(path, []byte(knownContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := VerifyFile(path, knownSHA256); err != nil {
		t.Errorf("VerifyFile() with correct checksum: unexpected error: %v", err)
	}
}

func TestVerifyFile_Mismatch(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")
	if err := os.WriteFile(path, []byte(knownContent), 0644); err != nil {
		t.Fatal(err)
	}

	wrongSHA := "0000000000000000000000000000000000000000000000000000000000000000"
	err := VerifyFile(path, wrongSHA)
	if err == nil {
		t.Error("VerifyFile() with wrong checksum: expected error, got nil")
	}
	if !IsMismatch(err) {
		t.Errorf("VerifyFile() error is not *MismatchError: %T", err)
	}
}

func TestVerifyFile_BadLength(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")
	if err := os.WriteFile(path, []byte(knownContent), 0644); err != nil {
		t.Fatal(err)
	}

	err := VerifyFile(path, "tooshort")
	if err == nil {
		t.Error("VerifyFile() with bad-length checksum: expected error, got nil")
	}
}

func TestVerifyReader_OK(t *testing.T) {
	r := bytes.NewBufferString(knownContent)
	if err := VerifyReader(r, knownSHA256); err != nil {
		t.Errorf("VerifyReader() with correct checksum: unexpected error: %v", err)
	}
}

func TestVerifyReader_Mismatch(t *testing.T) {
	r := bytes.NewBufferString(knownContent)
	wrongSHA := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	err := VerifyReader(r, wrongSHA)
	if err == nil {
		t.Error("VerifyReader() with wrong checksum: expected error, got nil")
	}
}

func TestMismatchError(t *testing.T) {
	err := &MismatchError{
		Path:     "/tmp/test.tar.zst",
		Expected: "aaa",
		Actual:   "bbb",
	}
	msg := err.Error()
	if msg == "" {
		t.Error("MismatchError.Error() returned empty string")
	}
}

func TestWritingHasher(t *testing.T) {
	var buf bytes.Buffer
	wh := NewWritingHasher(&buf)

	data := []byte(knownContent)
	n, err := wh.Write(data)
	if err != nil {
		t.Fatalf("WritingHasher.Write() error = %v", err)
	}
	if n != len(data) {
		t.Errorf("WritingHasher.Write() wrote %d bytes, want %d", n, len(data))
	}
	if buf.String() != knownContent {
		t.Errorf("WritingHasher did not forward data to underlying writer")
	}
	got := wh.Sum()
	if got != knownSHA256 {
		t.Errorf("WritingHasher.Sum() = %q, want %q", got, knownSHA256)
	}
}
