package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/kompa-tbm/kompa/internal/checksum"
	githubclient "github.com/kompa-tbm/kompa/internal/github"
	"github.com/kompa-tbm/kompa/internal/downloader"
)

func newSelfUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "self-update",
		Short: "Update Kompa itself to the latest version",
		Long: `Download and install the latest version of the Kompa CLI.

Kompa fetches the latest release of itself from GitHub, verifies the checksum,
and replaces the running binary.

Examples:
  kompa self-update`,
		Args: cobra.NoArgs,
		RunE: runSelfUpdate,
	}
}

func runSelfUpdate(cmd *cobra.Command, args []string) error {
	fmt.Println("Checking for Kompa updates...")

	// Use the default repo.
	client := githubclient.NewClient("kompa-tbm/kompa", os.Getenv("GITHUB_TOKEN"))
	client.SetTimeout(30 * time.Second)

	release, err := client.LatestRelease()
	if err != nil {
		return fmt.Errorf("checking for updates: %w", err)
	}

	if release.TagName == Version {
		fmt.Printf("Already on the latest version: %s\n", Version)
		return nil
	}

	fmt.Printf("New version available: %s → %s\n", Version, release.TagName)

	// Determine the asset name for the current platform.
	assetName := selfUpdateAssetName()
	asset := release.FindAsset(assetName)
	if asset == nil {
		return fmt.Errorf(
			"no binary found for %s/%s in release %s (looking for %s)",
			runtime.GOOS, runtime.GOARCH, release.TagName, assetName,
		)
	}

	// Determine the checksum asset name.
	checksumAssetName := assetName + ".sha256"
	checksumAsset := release.FindAsset(checksumAssetName)

	// Download the new binary to a temp file.
	tmpDir := os.TempDir()
	tmpPath := filepath.Join(tmpDir, "kompa-new"+platform_ExeSuffix())

	fmt.Printf("Downloading %s...\n", assetName)
	_, err = downloader.Download(context.Background(), asset.BrowserDownloadURL, tmpPath, downloader.Options{
		MaxRetries:   3,
		Timeout:      5 * time.Minute,
		ShowProgress: true,
		Label:        assetName,
	})
	if err != nil {
		return fmt.Errorf("downloading new version: %w", err)
	}

	// Verify checksum if available.
	if checksumAsset != nil {
		csumPath := filepath.Join(tmpDir, checksumAssetName)
		_, err = downloader.Download(context.Background(), checksumAsset.BrowserDownloadURL, csumPath, downloader.Options{
			ShowProgress: false,
		})
		if err == nil {
			csumBytes, err := os.ReadFile(csumPath)
			if err == nil {
				expectedSHA := parseChecksumFile(string(csumBytes))
				if expectedSHA != "" {
					if err := checksum.VerifyFile(tmpPath, expectedSHA); err != nil {
						_ = os.Remove(tmpPath)
						return fmt.Errorf("checksum verification failed for new binary: %w", err)
					}
					fmt.Println("✓ SHA-256 verified")
				}
			}
		}
	}

	// Make the new binary executable.
	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("setting executable permission: %w", err)
	}

	// Replace the current binary.
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding current executable: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolving executable path: %w", err)
	}

	// On Windows, we can't replace the running binary directly.
	// Rename old, rename new, then clean up.
	oldPath := execPath + ".old"
	if err := os.Rename(execPath, oldPath); err != nil {
		return fmt.Errorf("backing up current binary: %w", err)
	}
	if err := os.Rename(tmpPath, execPath); err != nil {
		// Try to restore the old binary.
		_ = os.Rename(oldPath, execPath)
		return fmt.Errorf("replacing binary: %w", err)
	}
	_ = os.Remove(oldPath)

	fmt.Printf("✓ Kompa updated to %s\n", release.TagName)
	return nil
}

func selfUpdateAssetName() string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	ext := ""
	if goos == "windows" {
		ext = ".exe"
	}
	return fmt.Sprintf("kompa-%s-%s%s", goos, goarch, ext)
}

func platform_ExeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

func parseChecksumFile(content string) string {
	// Format: "<sha256>  <filename>" or just "<sha256>".
	for _, line := range splitLines(content) {
		line = trimSpace(line)
		if len(line) >= 64 {
			return line[:64]
		}
	}
	return ""
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
