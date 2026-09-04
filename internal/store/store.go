// Package store manages the local package database for Kompa.
// The database is a single JSON file that tracks every installed package.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SchemaVersion is the current DB schema version.
const SchemaVersion = 1

// InstalledPackage records a package that is currently installed.
type InstalledPackage struct {
	// Identity
	Name    string `json:"name"`
	Version string `json:"version"`

	// Platform
	OS   string `json:"os"`
	Arch string `json:"arch"`

	// Location
	InstallPath string `json:"install_path"`

	// Content
	Files  []string `json:"files"`
	SHA256 string   `json:"sha256"`
	Size   int64    `json:"size"`

	// Relations
	Dependencies []string `json:"dependencies,omitempty"`

	// State
	InstalledAt time.Time `json:"installed_at"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
	ReleaseTag  string    `json:"release_tag"`
	Active      bool      `json:"active"`

	// RuntimeEnv is the set of environment variables this package exports.
	RuntimeEnv map[string]string `json:"runtime_env,omitempty"`

	// Binaries is the list of executable basenames provided by this package.
	Binaries []string `json:"binaries,omitempty"`
}

// Platform returns the "os-arch" platform string.
func (p *InstalledPackage) Platform() string {
	return p.OS + "-" + p.Arch
}

// DB is the Kompa package database.
type DB struct {
	mu       sync.RWMutex
	path     string
	data     *dbData
}

type dbData struct {
	SchemaVersion int                          `json:"schema_version"`
	Packages      map[string]*InstalledPackage `json:"packages"` // key: name@version@os@arch
}

// Open loads the database from path.
// If the file does not exist, an empty database is returned.
func Open(path string) (*DB, error) {
	db := &DB{
		path: path,
		data: &dbData{
			SchemaVersion: SchemaVersion,
			Packages:      make(map[string]*InstalledPackage),
		},
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return db, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading package database %s: %w", path, err)
	}

	if err := json.Unmarshal(data, db.data); err != nil {
		return nil, fmt.Errorf("parsing package database %s: %w", path, err)
	}
	return db, nil
}

// Save writes the database to disk atomically.
func (db *DB) Save() error {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.saveLocked()
}

func (db *DB) saveLocked() error {
	data, err := json.MarshalIndent(db.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling package database: %w", err)
	}

	dir := filepath.Dir(db.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating database directory: %w", err)
	}

	// Write atomically via a temp file.
	tmp := db.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("writing temporary database file: %w", err)
	}
	if err := os.Rename(tmp, db.path); err != nil {
		return fmt.Errorf("renaming database file: %w", err)
	}
	return nil
}

// key builds the unique key for a package record.
func key(name, version, os, arch string) string {
	return fmt.Sprintf("%s@%s@%s@%s", name, version, os, arch)
}

// Install records a newly installed package.
func (db *DB) Install(pkg *InstalledPackage) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if pkg.InstalledAt.IsZero() {
		pkg.InstalledAt = time.Now().UTC()
	}
	pkg.Active = true

	k := key(pkg.Name, pkg.Version, pkg.OS, pkg.Arch)
	db.data.Packages[k] = pkg
	return db.saveLocked()
}

// Remove deletes the record for the given package.
func (db *DB) Remove(name, version, os, arch string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	k := key(name, version, os, arch)
	if _, ok := db.data.Packages[k]; !ok {
		return fmt.Errorf("package %s@%s (%s/%s) is not recorded in the database", name, version, os, arch)
	}
	delete(db.data.Packages, k)
	return db.saveLocked()
}

// Get returns the installed package record, or nil if not found.
func (db *DB) Get(name, version, os, arch string) *InstalledPackage {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.data.Packages[key(name, version, os, arch)]
}

// GetLatest returns the most recently installed version of a package for the given os/arch.
func (db *DB) GetLatest(name, os, arch string) *InstalledPackage {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var latest *InstalledPackage
	for _, p := range db.data.Packages {
		if p.Name != name || p.OS != os || p.Arch != arch {
			continue
		}
		if latest == nil || p.InstalledAt.After(latest.InstalledAt) {
			latest = p
		}
	}
	return latest
}

// GetActive returns the active version of a package for the given os/arch.
func (db *DB) GetActive(name, os, arch string) *InstalledPackage {
	db.mu.RLock()
	defer db.mu.RUnlock()
	for _, p := range db.data.Packages {
		if p.Name == name && p.OS == os && p.Arch == arch && p.Active {
			return p
		}
	}
	return nil
}

// IsInstalled returns true if any version of name is installed for the given os/arch.
func (db *DB) IsInstalled(name, os, arch string) bool {
	return db.GetLatest(name, os, arch) != nil
}

// List returns all installed packages sorted by name then version.
func (db *DB) List() []*InstalledPackage {
	db.mu.RLock()
	defer db.mu.RUnlock()

	out := make([]*InstalledPackage, 0, len(db.data.Packages))
	for _, p := range db.data.Packages {
		out = append(out, p)
	}
	sortPackages(out)
	return out
}

// ListByName returns all installed versions of a specific package.
func (db *DB) ListByName(name string) []*InstalledPackage {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var out []*InstalledPackage
	for _, p := range db.data.Packages {
		if p.Name == name {
			out = append(out, p)
		}
	}
	sortPackages(out)
	return out
}

// SetActive marks a specific version as the active one for its os/arch,
// and deactivates all other versions of the same package.
func (db *DB) SetActive(name, version, os, arch string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	found := false
	for _, p := range db.data.Packages {
		if p.Name == name && p.OS == os && p.Arch == arch {
			p.Active = (p.Version == version)
			if p.Version == version {
				found = true
			}
		}
	}
	if !found {
		return fmt.Errorf("package %s@%s (%s/%s) is not installed", name, version, os, arch)
	}
	return db.saveLocked()
}

// UpdateFiles replaces the file list for an installed package.
func (db *DB) UpdateFiles(name, version, os, arch string, files []string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	k := key(name, version, os, arch)
	p, ok := db.data.Packages[k]
	if !ok {
		return fmt.Errorf("package %s@%s not found in database", name, version)
	}
	p.Files = files
	p.UpdatedAt = time.Now().UTC()
	return db.saveLocked()
}

func sortPackages(pkgs []*InstalledPackage) {
	for i := 1; i < len(pkgs); i++ {
		for j := i; j > 0; j-- {
			a, b := pkgs[j-1], pkgs[j]
			if a.Name > b.Name || (a.Name == b.Name && a.Version > b.Version) {
				pkgs[j-1], pkgs[j] = pkgs[j], pkgs[j-1]
			} else {
				break
			}
		}
	}
}
