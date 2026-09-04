// Package platform provides OS and architecture detection, and platform-specific path resolution.
package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// OS represents a normalized operating system name.
type OS string

const (
	Linux   OS = "linux"
	Windows OS = "windows"
	Darwin  OS = "darwin"
	Unknown OS = "unknown"
)

// Arch represents a normalized CPU architecture.
type Arch string

const (
	AMD64   Arch = "amd64"
	ARM64   Arch = "arm64"
	I386    Arch = "386"
	RISCV64 Arch = "riscv64"
	UnknownArch Arch = "unknown"
)

// Platform represents a specific OS + architecture combination.
type Platform struct {
	OS   OS
	Arch Arch
}

// String returns the canonical "os-arch" string for this platform.
func (p Platform) String() string {
	return fmt.Sprintf("%s-%s", p.OS, p.Arch)
}

// Current returns the Platform for the running process.
func Current() Platform {
	return Platform{
		OS:   normalizeOS(runtime.GOOS),
		Arch: normalizeArch(runtime.GOARCH),
	}
}

// Parse parses a platform string such as "linux-amd64".
func Parse(s string) (Platform, error) {
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return Platform{}, fmt.Errorf("invalid platform string %q: expected os-arch", s)
	}
	o := normalizeOS(parts[0])
	a := normalizeArch(parts[1])
	if o == Unknown {
		return Platform{}, fmt.Errorf("unknown OS %q", parts[0])
	}
	if a == UnknownArch {
		return Platform{}, fmt.Errorf("unknown architecture %q", parts[1])
	}
	return Platform{OS: o, Arch: a}, nil
}

func normalizeOS(goos string) OS {
	switch strings.ToLower(goos) {
	case "linux":
		return Linux
	case "windows":
		return Windows
	case "darwin":
		return Darwin
	default:
		return Unknown
	}
}

func normalizeArch(goarch string) Arch {
	switch strings.ToLower(goarch) {
	case "amd64", "x86_64":
		return AMD64
	case "arm64", "aarch64":
		return ARM64
	case "386", "i386", "i686", "x86":
		return I386
	case "riscv64":
		return RISCV64
	default:
		return UnknownArch
	}
}

// Dirs holds platform-specific directory paths for Kompa.
type Dirs struct {
	// Root is the top-level Kompa data directory.
	Root string
	// Bin is where shims/wrappers are placed.
	Bin string
	// Packages is where installed package trees live.
	Packages string
	// Cache is the HTTP/download cache.
	Cache string
	// Downloads is the staging area for downloaded archives.
	Downloads string
	// Metadata holds remote package metadata.
	Metadata string
	// Manifests holds locally generated manifests.
	Manifests string
	// Versions holds version-selection state.
	Versions string
	// DB is the path to the package database file.
	DB string
}

// GetDirs returns the standard Kompa directory layout for the current OS.
// It respects the KOMPA_HOME environment variable for overrides.
func GetDirs() (Dirs, error) {
	root := os.Getenv("KOMPA_HOME")
	if root == "" {
		var err error
		root, err = defaultRoot()
		if err != nil {
			return Dirs{}, fmt.Errorf("determining kompa root: %w", err)
		}
	}
	return dirsFromRoot(root), nil
}

// GetDirsFromRoot builds Dirs from an explicit root path.
func GetDirsFromRoot(root string) Dirs {
	return dirsFromRoot(root)
}

func dirsFromRoot(root string) Dirs {
	return Dirs{
		Root:      root,
		Bin:       filepath.Join(root, "bin"),
		Packages:  filepath.Join(root, "packages"),
		Cache:     filepath.Join(root, "cache"),
		Downloads: filepath.Join(root, "downloads"),
		Metadata:  filepath.Join(root, "metadata"),
		Manifests: filepath.Join(root, "manifests"),
		Versions:  filepath.Join(root, "versions"),
		DB:        filepath.Join(root, "db.json"),
	}
}

func defaultRoot() (string, error) {
	switch runtime.GOOS {
	case "windows":
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(base, "Kompa"), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "Kompa"), nil
	default: // linux and others
		base := os.Getenv("XDG_DATA_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".local", "share")
		}
		return filepath.Join(base, "kompa"), nil
	}
}

// EnsureDirs creates all required Kompa directories if they do not exist.
func EnsureDirs(d Dirs) error {
	dirs := []string{
		d.Root, d.Bin, d.Packages, d.Cache,
		d.Downloads, d.Metadata, d.Manifests, d.Versions,
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}
	return nil
}

// ExeSuffix returns the executable suffix for the current OS (".exe" on Windows, "" otherwise).
func ExeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// IsWindows returns true when running on Windows.
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// IsDarwin returns true when running on macOS.
func IsDarwin() bool {
	return runtime.GOOS == "darwin"
}

// IsLinux returns true when running on Linux.
func IsLinux() bool {
	return runtime.GOOS == "linux"
}
