// Package manifest defines the release manifest format and parsing logic.
// A manifest is the machine-readable record produced by the CI build system
// and consumed by the Kompa CLI to locate and verify artifacts.
package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// SchemaVersion is the current manifest schema version.
	SchemaVersion = 1
	// ManifestFileName is the standard name for the release manifest file.
	ManifestFileName = "manifest.json"
)

// PackageEntry describes a single built artifact within a release.
type PackageEntry struct {
	// SchemaVersion allows forward-compatible parsing.
	SchemaVersion int `json:"schema_version"`

	// Identity
	Name    string `json:"name"`
	Version string `json:"version"`

	// Platform
	Platform     string `json:"platform"`      // e.g. "linux-amd64"
	OS           string `json:"os"`            // e.g. "linux"
	Architecture string `json:"architecture"`  // e.g. "amd64"

	// Artifact
	Archive  string `json:"archive"`   // filename, e.g. "gcc-linux-amd64.tar.zst"
	SHA256   string `json:"sha256"`    // hex-encoded SHA-256 of the archive
	Size     int64  `json:"size"`      // bytes
	DownloadURL string `json:"download_url"` // full HTTPS URL

	// Dependency names (other Kompa package names required at runtime).
	Dependencies []string `json:"dependencies,omitempty"`

	// Metadata
	BuildTime time.Time `json:"build_time"`
	KompaVersion string `json:"kompa_version"`
	ReleaseTag   string `json:"release_tag"`

	// InstallLayout describes where files land relative to the package root.
	Binaries  []string `json:"binaries,omitempty"`
	Libraries []string `json:"libraries,omitempty"`
	Headers   []string `json:"headers,omitempty"`

	// RuntimeEnv lists environment variables for this package (key=value strings).
	RuntimeEnv map[string]string `json:"runtime_env,omitempty"`
}

// Validate checks required fields and format consistency.
func (e *PackageEntry) Validate() error {
	if e.Name == "" {
		return fmt.Errorf("name is required")
	}
	if e.Version == "" {
		return fmt.Errorf("version is required")
	}
	if e.OS == "" {
		return fmt.Errorf("os is required")
	}
	if e.Architecture == "" {
		return fmt.Errorf("architecture is required")
	}
	if e.Archive == "" {
		return fmt.Errorf("archive is required")
	}
	if e.SHA256 == "" {
		return fmt.Errorf("sha256 is required")
	}
	if len(e.SHA256) != 64 {
		return fmt.Errorf("sha256 must be 64 hex characters, got %d", len(e.SHA256))
	}
	if !isHex(e.SHA256) {
		return fmt.Errorf("sha256 contains non-hex characters")
	}
	if e.DownloadURL == "" {
		return fmt.Errorf("download_url is required")
	}
	if !strings.HasPrefix(e.DownloadURL, "https://") {
		return fmt.Errorf("download_url must use HTTPS, got: %s", e.DownloadURL)
	}
	return nil
}

// ReleaseManifest is the top-level release manifest containing all built packages.
type ReleaseManifest struct {
	SchemaVersion int             `json:"schema_version"`
	ReleaseTag    string          `json:"release_tag"`
	BuildTime     time.Time       `json:"build_time"`
	KompaVersion  string          `json:"kompa_version"`
	Packages      []*PackageEntry `json:"packages"`
}

// Find returns the PackageEntry for the given name, os, and arch, or nil.
func (m *ReleaseManifest) Find(name, os, arch string) *PackageEntry {
	for _, p := range m.Packages {
		if p.Name == name && p.OS == os && p.Architecture == arch {
			return p
		}
	}
	return nil
}

// FindByPlatform returns the PackageEntry for the given name and platform string ("os-arch").
func (m *ReleaseManifest) FindByPlatform(name, platform string) *PackageEntry {
	for _, p := range m.Packages {
		if p.Name == name && p.Platform == platform {
			return p
		}
	}
	return nil
}

// AllForPackage returns every entry for a given package name (all platforms).
func (m *ReleaseManifest) AllForPackage(name string) []*PackageEntry {
	var out []*PackageEntry
	for _, p := range m.Packages {
		if p.Name == name {
			out = append(out, p)
		}
	}
	return out
}

// PackageNames returns the deduplicated, sorted list of package names in this manifest.
func (m *ReleaseManifest) PackageNames() []string {
	seen := make(map[string]struct{})
	for _, p := range m.Packages {
		seen[p.Name] = struct{}{}
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sortStrings(names)
	return names
}

// Validate checks the manifest and all its entries.
func (m *ReleaseManifest) Validate() error {
	if m.SchemaVersion < 1 {
		return fmt.Errorf("schema_version must be >= 1")
	}
	if m.ReleaseTag == "" {
		return fmt.Errorf("release_tag is required")
	}
	for i, e := range m.Packages {
		if err := e.Validate(); err != nil {
			return fmt.Errorf("package[%d] %s: %w", i, e.Name, err)
		}
	}
	return nil
}

// ParseManifest decodes a ReleaseManifest from JSON bytes.
func ParseManifest(data []byte) (*ReleaseManifest, error) {
	var m ReleaseManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest JSON: %w", err)
	}
	// Back-fill per-entry fields that the top-level record carries.
	for _, e := range m.Packages {
		if e.ReleaseTag == "" {
			e.ReleaseTag = m.ReleaseTag
		}
		if e.KompaVersion == "" {
			e.KompaVersion = m.KompaVersion
		}
		if e.Platform == "" && e.OS != "" && e.Architecture != "" {
			e.Platform = e.OS + "-" + e.Architecture
		}
	}
	return &m, nil
}

// LoadManifestFile loads and parses a manifest from a local JSON file.
func LoadManifestFile(path string) (*ReleaseManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest file %s: %w", path, err)
	}
	m, err := ParseManifest(data)
	if err != nil {
		return nil, fmt.Errorf("parsing manifest file %s: %w", path, err)
	}
	return m, nil
}

// SaveManifestFile writes a ReleaseManifest to a JSON file.
func SaveManifestFile(path string, m *ReleaseManifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling manifest: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating manifest directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing manifest file %s: %w", path, err)
	}
	return nil
}

// MarshalEntry encodes a single PackageEntry to JSON.
func MarshalEntry(e *PackageEntry) ([]byte, error) {
	return json.MarshalIndent(e, "", "  ")
}

// ParseEntry decodes a single PackageEntry from JSON bytes.
func ParseEntry(data []byte) (*PackageEntry, error) {
	var e PackageEntry
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, fmt.Errorf("parsing package entry JSON: %w", err)
	}
	if e.Platform == "" && e.OS != "" && e.Architecture != "" {
		e.Platform = e.OS + "-" + e.Architecture
	}
	return &e, nil
}

func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func sortStrings(ss []string) {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j] < ss[j-1]; j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
}
