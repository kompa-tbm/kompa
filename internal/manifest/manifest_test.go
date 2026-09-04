package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func validEntry() *PackageEntry {
	return &PackageEntry{
		SchemaVersion: 1,
		Name:          "gcc",
		Version:       "14.2.0",
		Platform:      "linux-amd64",
		OS:            "linux",
		Architecture:  "amd64",
		Archive:       "gcc-linux-amd64.tar.zst",
		SHA256:        "aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd",
		Size:          1024 * 1024 * 50,
		DownloadURL:   "https://github.com/kompa-tbm/kompa/releases/download/v1/gcc-linux-amd64.tar.zst",
		Dependencies:  []string{"binutils"},
		BuildTime:     time.Now(),
		ReleaseTag:    "v1",
	}
}

func TestPackageEntry_Validate_OK(t *testing.T) {
	e := validEntry()
	if err := e.Validate(); err != nil {
		t.Errorf("Validate() on valid entry: unexpected error: %v", err)
	}
}

func TestPackageEntry_Validate_MissingFields(t *testing.T) {
	tests := []struct {
		name  string
		mutate func(*PackageEntry)
	}{
		{"missing name", func(e *PackageEntry) { e.Name = "" }},
		{"missing version", func(e *PackageEntry) { e.Version = "" }},
		{"missing os", func(e *PackageEntry) { e.OS = "" }},
		{"missing arch", func(e *PackageEntry) { e.Architecture = "" }},
		{"missing archive", func(e *PackageEntry) { e.Archive = "" }},
		{"missing sha256", func(e *PackageEntry) { e.SHA256 = "" }},
		{"missing download_url", func(e *PackageEntry) { e.DownloadURL = "" }},
		{"short sha256", func(e *PackageEntry) { e.SHA256 = "abc123" }},
		{"http url", func(e *PackageEntry) { e.DownloadURL = "http://example.com/file.tar.zst" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := validEntry()
			tt.mutate(e)
			if err := e.Validate(); err == nil {
				t.Errorf("Validate() expected error for %q but got nil", tt.name)
			}
		})
	}
}

func TestParseManifest(t *testing.T) {
	m := &ReleaseManifest{
		SchemaVersion: 1,
		ReleaseTag:    "v1",
		BuildTime:     time.Now(),
		KompaVersion:  "0.1.0",
		Packages:      []*PackageEntry{validEntry()},
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	parsed, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}

	if parsed.ReleaseTag != "v1" {
		t.Errorf("ReleaseTag = %q, want %q", parsed.ReleaseTag, "v1")
	}
	if len(parsed.Packages) != 1 {
		t.Fatalf("len(Packages) = %d, want 1", len(parsed.Packages))
	}
	if parsed.Packages[0].Name != "gcc" {
		t.Errorf("Packages[0].Name = %q, want %q", parsed.Packages[0].Name, "gcc")
	}
}

func TestParseManifest_Invalid(t *testing.T) {
	_, err := ParseManifest([]byte("not json"))
	if err == nil {
		t.Error("ParseManifest(invalid JSON) expected error, got nil")
	}
}

func TestReleaseManifest_Find(t *testing.T) {
	m := &ReleaseManifest{
		SchemaVersion: 1,
		ReleaseTag:    "v1",
		Packages: []*PackageEntry{
			validEntry(),
		},
	}

	e := m.Find("gcc", "linux", "amd64")
	if e == nil {
		t.Fatal("Find(gcc, linux, amd64) returned nil, want entry")
	}
	if e.Name != "gcc" {
		t.Errorf("entry.Name = %q, want gcc", e.Name)
	}

	missing := m.Find("clang", "linux", "amd64")
	if missing != nil {
		t.Error("Find(clang, ...) should return nil")
	}
}

func TestReleaseManifest_PackageNames(t *testing.T) {
	e1 := validEntry()
	e2 := validEntry()
	e2.Name = "clang"
	e2.Archive = "clang-linux-amd64.tar.zst"
	e2.DownloadURL = "https://github.com/kompa-tbm/kompa/releases/download/v1/clang-linux-amd64.tar.zst"

	m := &ReleaseManifest{
		SchemaVersion: 1,
		ReleaseTag:    "v1",
		Packages:      []*PackageEntry{e1, e2},
	}

	names := m.PackageNames()
	if len(names) != 2 {
		t.Errorf("PackageNames() len = %d, want 2", len(names))
	}
	// Should be sorted.
	if names[0] != "clang" || names[1] != "gcc" {
		t.Errorf("PackageNames() = %v, want [clang gcc]", names)
	}
}

func TestReleaseManifest_Validate(t *testing.T) {
	m := &ReleaseManifest{
		SchemaVersion: 1,
		ReleaseTag:    "v1",
		Packages:      []*PackageEntry{validEntry()},
	}
	if err := m.Validate(); err != nil {
		t.Errorf("Validate() on valid manifest: %v", err)
	}
}

func TestReleaseManifest_Validate_Errors(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ReleaseManifest)
	}{
		{"zero schema version", func(m *ReleaseManifest) { m.SchemaVersion = 0 }},
		{"missing release tag", func(m *ReleaseManifest) { m.ReleaseTag = "" }},
		{"invalid package", func(m *ReleaseManifest) { m.Packages[0].Name = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &ReleaseManifest{
				SchemaVersion: 1,
				ReleaseTag:    "v1",
				Packages:      []*PackageEntry{validEntry()},
			}
			tt.mutate(m)
			if err := m.Validate(); err == nil {
				t.Errorf("Validate() expected error for %q but got nil", tt.name)
			}
		})
	}
}

func TestSaveLoadManifestFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "manifest.json")

	m := &ReleaseManifest{
		SchemaVersion: 1,
		ReleaseTag:    "v42",
		BuildTime:     time.Now().UTC().Truncate(time.Second),
		KompaVersion:  "0.1.0",
		Packages:      []*PackageEntry{validEntry()},
	}

	if err := SaveManifestFile(path, m); err != nil {
		t.Fatalf("SaveManifestFile() error = %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("manifest file not created: %v", err)
	}

	loaded, err := LoadManifestFile(path)
	if err != nil {
		t.Fatalf("LoadManifestFile() error = %v", err)
	}

	if loaded.ReleaseTag != "v42" {
		t.Errorf("loaded ReleaseTag = %q, want v42", loaded.ReleaseTag)
	}
	if len(loaded.Packages) != 1 {
		t.Errorf("loaded Packages len = %d, want 1", len(loaded.Packages))
	}
}

func TestLoadManifestFile_Missing(t *testing.T) {
	_, err := LoadManifestFile("/nonexistent/manifest.json")
	if err == nil {
		t.Error("LoadManifestFile() on missing file: expected error, got nil")
	}
}
