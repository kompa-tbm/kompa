package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

func newCacheCmd(s *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage the Kompa download cache",
		Long: `Inspect and manage Kompa's download cache.

Subcommands:
  list   Show cached files
  clean  Remove all cached files (alias for 'kompa clean')
  info   Show cache directory and total size`,
	}
	cmd.AddCommand(newCacheListCmd(s), newCacheInfoCmd(s))
	return cmd
}

func newCacheListCmd(s *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List files in the download cache",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			entries, err := os.ReadDir(s.dirs.Downloads)
			if err != nil && os.IsNotExist(err) {
				fmt.Println("Download cache is empty.")
				return nil
			}
			if err != nil {
				return err
			}

			if len(entries) == 0 {
				fmt.Println("Download cache is empty.")
				return nil
			}

			fmt.Printf("%-50s %-12s %s\n", "File", "Size", "Modified")
			fmt.Printf("%-50s %-12s %s\n", "────", "────", "────────")
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				fi, err := e.Info()
				if err != nil {
					continue
				}
				name := e.Name()
				if len(name) > 49 {
					name = name[:46] + "..."
				}
				fmt.Printf("%-50s %-12s %s\n",
					name,
					humanBytes(fi.Size()),
					fi.ModTime().Format(time.RFC3339),
				)
			}
			return nil
		},
	}
}

func newCacheInfoCmd(s *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show cache directory location and total size",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Cache directory: %s\n", s.dirs.Downloads)

			entries, err := os.ReadDir(s.dirs.Downloads)
			if err != nil && os.IsNotExist(err) {
				fmt.Println("Total size:      0 B (empty)")
				return nil
			}
			if err != nil {
				return err
			}

			var total int64
			var count int
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				if fi, err := e.Info(); err == nil {
					total += fi.Size()
					count++
				}
			}

			fmt.Printf("Total size:      %s (%d files)\n", humanBytes(total), count)

			// Also report metadata directory.
			fmt.Printf("Metadata dir:    %s\n", s.dirs.Metadata)

			var metaTotal int64
			metaEntries, _ := os.ReadDir(s.dirs.Metadata)
			for _, e := range metaEntries {
				if fi, err := e.Info(); err == nil {
					metaTotal += fi.Size()
				}
			}
			fmt.Printf("Metadata size:   %s\n", humanBytes(metaTotal))

			// Report packages directory total.
			pkgTotal, _ := dirSize(s.dirs.Packages)
			fmt.Printf("Installed size:  %s\n", humanBytes(pkgTotal))

			return nil
		},
	}
}

// dirSize recursively calculates the total byte size of a directory.
func dirSize(path string) (int64, error) {
	var total int64
	err := filepath.Walk(path, func(_ string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if !fi.IsDir() {
			total += fi.Size()
		}
		return nil
	})
	return total, err
}
