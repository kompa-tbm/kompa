// Package installer orchestrates the full install/uninstall lifecycle for Kompa packages.
package installer

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/kompa-tbm/kompa/internal/checksum"
	"github.com/kompa-tbm/kompa/internal/downloader"
	"github.com/kompa-tbm/kompa/internal/extractor"
	githubclient "github.com/kompa-tbm/kompa/internal/github"
	"github.com/kompa-tbm/kompa/internal/manifest"
	"github.com/kompa-tbm/kompa/internal/platform"
	"github.com/kompa-tbm/kompa/internal/store"
)

// Options configures an install operation.
type Options struct {
	Verbose      bool
	ShowProgress bool
	Force        bool // reinstall even if already present
	YesAll       bool // skip confirmation prompts
	NoCache      bool // delete cached downloads before starting
}

// Installer performs package installation.
type Installer struct {
	dirs       platform.Dirs
	db         *store.DB
	ghClient   *githubclient.Client
	platform   platform.Platform
	opts       Options
	logf       func(format string, args ...interface{})
	verbosef   func(format string, args ...interface{})
}

// New creates an Installer.
func New(
	dirs platform.Dirs,
	db *store.DB,
	ghClient *githubclient.Client,
	plat platform.Platform,
	opts Options,
) *Installer {
	return &Installer{
		dirs:     dirs,
		db:       db,
		ghClient: ghClient,
		platform: plat,
		opts:     opts,
		logf: func(format string, args ...interface{}) {
			fmt.Printf(format+"\n", args...)
		},
		verbosef: func(format string, args ...interface{}) {
			if opts.Verbose {
				fmt.Printf("[verbose] "+format+"\n", args...)
			}
		},
	}
}

// SetLogger replaces the default stdout logger.
func (inst *Installer) SetLogger(logf func(string, ...interface{})) {
	inst.logf = logf
}

// Install downloads and installs a single package (without dependency handling).
// The caller is responsible for resolving and installing dependencies first.
func (inst *Installer) Install(ctx context.Context, entry *manifest.PackageEntry) error {
	name := entry.Name
	version := entry.Version
	osName := entry.OS
	arch := entry.Architecture

	// Skip if already installed and not forced.
	if !inst.opts.Force && inst.db.IsInstalled(name, osName, arch) {
		existing := inst.db.GetLatest(name, osName, arch)
		if existing != nil && existing.Version == version {
			inst.logf("  ✓ %s %s is already installed", name, version)
			return nil
		}
	}

	inst.logf("  → Installing %s %s (%s/%s)", name, version, osName, arch)

	downloadPath := filepath.Join(inst.dirs.Downloads, entry.Archive)

	// Optionally clear cache.
	if inst.opts.NoCache {
		_ = os.Remove(downloadPath)
	}

	// Download the artifact.
	if err := inst.download(ctx, entry, downloadPath); err != nil {
		return fmt.Errorf("downloading %s: %w", name, err)
	}

	// Verify checksum.
	inst.verbosef("Verifying checksum for %s", entry.Archive)
	if err := checksum.VerifyFile(downloadPath, entry.SHA256); err != nil {
		// Remove the corrupted file.
		_ = os.Remove(downloadPath)
		return fmt.Errorf("checksum verification failed for %s: %w", name, err)
	}
	inst.logf("  ✓ SHA-256 verified")

	// Prepare installation directory.
	installDir := filepath.Join(inst.dirs.Packages, name, version)
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("creating install directory %s: %w", installDir, err)
	}

	// Extract archive.
	inst.logf("  → Extracting %s", entry.Archive)
	extractOpts := extractor.Options{
		StripComponents: 1, // strip top-level directory inside archive
		Verbose:         inst.opts.Verbose,
	}
	if err := extractor.Extract(downloadPath, installDir, extractOpts); err != nil {
		// Clean up partial extraction.
		_ = os.RemoveAll(installDir)
		return fmt.Errorf("extracting %s: %w", name, err)
	}

	// Walk the installed directory to record files.
	var installedFiles []string
	if err := filepath.WalkDir(installDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			rel, err := filepath.Rel(installDir, path)
			if err != nil {
				return err
			}
			installedFiles = append(installedFiles, rel)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("recording installed files: %w", err)
	}

	// Build runtime env map.
	runtimeEnv := make(map[string]string)
	for k, v := range entry.RuntimeEnv {
		runtimeEnv[k] = expandInstallDir(v, installDir)
	}

	// Create shims in the Kompa bin directory.
	if err := inst.createShims(entry, installDir); err != nil {
		return fmt.Errorf("creating shims for %s: %w", name, err)
	}

	// Record in database.
	pkg := &store.InstalledPackage{
		Name:         name,
		Version:      version,
		OS:           osName,
		Arch:         arch,
		InstallPath:  installDir,
		Files:        installedFiles,
		SHA256:       entry.SHA256,
		Size:         entry.Size,
		Dependencies: entry.Dependencies,
		InstalledAt:  time.Now().UTC(),
		ReleaseTag:   entry.ReleaseTag,
		Active:       true,
		RuntimeEnv:   runtimeEnv,
		Binaries:     entry.Binaries,
	}
	if err := inst.db.Install(pkg); err != nil {
		return fmt.Errorf("recording installation in database: %w", err)
	}

	inst.logf("  ✓ %s %s installed successfully", name, version)
	return nil
}

// Uninstall removes an installed package.
func (inst *Installer) Uninstall(name, version, osName, arch string) error {
	pkg := inst.db.Get(name, version, osName, arch)
	if pkg == nil {
		return fmt.Errorf("package %s@%s (%s/%s) is not installed", name, version, osName, arch)
	}

	inst.logf("  → Removing %s %s", name, version)

	// Remove only the files owned by this package.
	for _, rel := range pkg.Files {
		abs := filepath.Join(pkg.InstallPath, rel)
		if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
			inst.verbosef("warning: could not remove %s: %v", abs, err)
		}
	}

	// Remove shims.
	inst.removeShims(pkg)

	// Remove installation directory if empty.
	_ = removeEmptyDirs(pkg.InstallPath)

	// Remove from database.
	if err := inst.db.Remove(name, version, osName, arch); err != nil {
		return fmt.Errorf("removing database record: %w", err)
	}

	inst.logf("  ✓ %s %s removed", name, version)
	return nil
}

// download fetches the artifact, using a cached copy if available.
func (inst *Installer) download(ctx context.Context, entry *manifest.PackageEntry, destPath string) error {
	// Check for a valid cached copy.
	if !inst.opts.NoCache {
		if fi, err := os.Stat(destPath); err == nil && fi.Size() > 0 {
			// Verify the cached file's checksum.
			if err := checksum.VerifyFile(destPath, entry.SHA256); err == nil {
				inst.verbosef("Using cached download for %s", entry.Archive)
				return nil
			}
			// Checksum mismatch — delete and re-download.
			inst.verbosef("Cached file for %s failed checksum; re-downloading", entry.Archive)
			_ = os.Remove(destPath)
		}
	}

	inst.logf("  ↓ Downloading %s %s", entry.Name, entry.Version)

	dlOpts := downloader.Options{
		MaxRetries:     3,
		Timeout:        10 * time.Minute,
		ExpectedSHA256: entry.SHA256,
		ShowProgress:   inst.opts.ShowProgress,
		Label:          entry.Archive,
		Resume:         true,
	}

	_, err := downloader.Download(ctx, entry.DownloadURL, destPath, dlOpts)
	return err
}

// createShims creates wrapper scripts/symlinks in the Kompa bin directory.
func (inst *Installer) createShims(entry *manifest.PackageEntry, installDir string) error {
	if err := os.MkdirAll(inst.dirs.Bin, 0755); err != nil {
		return fmt.Errorf("creating bin directory: %w", err)
	}

	binDir := filepath.Join(installDir, "bin")
	for _, bin := range entry.Binaries {
		srcName := bin
		if runtime.GOOS == "windows" && !strings.HasSuffix(bin, ".exe") {
			srcName = bin + ".exe"
		}
		src := filepath.Join(binDir, srcName)

		dstName := bin
		if runtime.GOOS == "windows" && !strings.HasSuffix(bin, ".exe") {
			dstName = bin + ".exe"
		}
		dst := filepath.Join(inst.dirs.Bin, dstName)

		// Remove any existing shim.
		_ = os.Remove(dst)

		// Prefer symlinks; fall back to a wrapper script on Windows.
		if runtime.GOOS == "windows" {
			if err := inst.writeWindowsShim(src, dst, bin); err != nil {
				inst.verbosef("warning: could not create shim for %s: %v", bin, err)
			}
		} else {
			if err := os.Symlink(src, dst); err != nil {
				inst.verbosef("warning: could not symlink %s: %v", bin, err)
			}
		}
	}
	return nil
}

// writeWindowsShim writes a .cmd wrapper that calls the real binary.
func (inst *Installer) writeWindowsShim(src, dst, binName string) error {
	// For .exe files we just create a symlink if supported, otherwise copy.
	if err := os.Symlink(src, dst); err != nil {
		// Symlinks may need elevated privileges on Windows; fall back to copy.
		return copyFile(src, dst)
	}
	return nil
}

// removeShims deletes shim files for the given package.
func (inst *Installer) removeShims(pkg *store.InstalledPackage) {
	for _, bin := range pkg.Binaries {
		name := bin
		if runtime.GOOS == "windows" && !strings.HasSuffix(bin, ".exe") {
			name = bin + ".exe"
		}
		shimPath := filepath.Join(inst.dirs.Bin, name)
		// Only remove if the shim points into this package's install dir.
		if target, err := os.Readlink(shimPath); err == nil {
			if strings.HasPrefix(target, pkg.InstallPath) {
				_ = os.Remove(shimPath)
			}
		} else {
			// Not a symlink — remove if it's a plain file in our bin dir.
			if fi, err2 := os.Stat(shimPath); err2 == nil && !fi.IsDir() {
				_ = os.Remove(shimPath)
			}
		}
	}
}

// expandInstallDir replaces {{install_dir}} placeholders in env var values.
func expandInstallDir(val, installDir string) string {
	return strings.ReplaceAll(val, "{{install_dir}}", installDir)
}

// removeEmptyDirs removes dir and its parents up the tree if they are empty.
func removeEmptyDirs(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // already gone
	}
	if len(entries) > 0 {
		return nil
	}
	if err := os.Remove(dir); err != nil {
		return err
	}
	return removeEmptyDirs(filepath.Dir(dir))
}

func copyFile(src, dst string) error {
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

	buf := make([]byte, 32*1024)
	for {
		n, err := in.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return werr
			}
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return err
		}
	}
	return nil
}
