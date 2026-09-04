package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newCleanCmd(s *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "clean",
		Short: "Remove downloaded archive files from the cache",
		Long: `Remove downloaded archive files that have been cached in the Kompa downloads directory.

Installed packages are not affected. To remove an installed package, use 'kompa remove'.

Examples:
  kompa clean
  kompa clean --yes`,
		Args: cobra.NoArgs,
		RunE: s.runClean,
	}
}

func (s *appState) runClean(cmd *cobra.Command, args []string) error {
	s.printHeader()

	entries, err := os.ReadDir(s.dirs.Downloads)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading downloads directory: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("Download cache is already empty.")
		return nil
	}

	// Calculate total size.
	var totalSize int64
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(s.dirs.Downloads, e.Name())
		fi, err := e.Info()
		if err != nil {
			continue
		}
		totalSize += fi.Size()
		files = append(files, path)
	}

	fmt.Printf("Downloads directory: %s\n", s.dirs.Downloads)
	fmt.Printf("Files to remove: %d (%s)\n", len(files), humanBytes(totalSize))
	fmt.Println()

	if !s.yes && !s.quiet {
		if !confirm("Remove all cached downloads?") {
			fmt.Println("Aborted.")
			return nil
		}
	}

	var removed int
	for _, f := range files {
		if err := os.Remove(f); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: could not remove %s: %v\n", filepath.Base(f), err)
		} else {
			removed++
			s.verbosef("removed %s", filepath.Base(f))
		}
	}

	fmt.Printf("Removed %d file(s) (%s freed).\n", removed, humanBytes(totalSize))
	return nil
}

func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
