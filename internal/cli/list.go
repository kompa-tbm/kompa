package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newListCmd(s *appState) *cobra.Command {
	return &cobra.Command{
		Use:     "list [package]",
		Aliases: []string{"ls"},
		Short:   "List installed packages",
		Long: `List all installed packages, or all versions of a specific package.

Examples:
  kompa list
  kompa list gcc
  kompa list --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: s.runList,
	}
}

func (s *appState) runList(cmd *cobra.Command, args []string) error {
	osName := string(s.plat.OS)
	arch := string(s.plat.Arch)

	var pkgs interface{}

	if len(args) == 1 {
		pkgs = s.db.ListByName(args[0])
	} else {
		all := s.db.List()
		// Filter to current platform.
		var filtered = all[:0]
		for _, p := range all {
			if p.OS == osName && p.Arch == arch {
				filtered = append(filtered, p)
			}
		}
		pkgs = filtered
	}

	if s.jsonOut {
		out, err := json.MarshalIndent(pkgs, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	}

	type rowItem struct {
		name    string
		version string
		active  bool
		os      string
		arch    string
	}

	var rows []rowItem
	switch v := pkgs.(type) {
	case []*interface{}:
		_ = v
	}

	// Re-extract as concrete type.
	switch v := pkgs.(type) {
	case interface{ Len() int }:
		_ = v
	}

	// Use the db directly.
	installedList := s.db.List()
	var filtered []*rowItem
	filterName := ""
	if len(args) == 1 {
		filterName = args[0]
	}
	for _, p := range installedList {
		if filterName != "" && p.Name != filterName {
			continue
		}
		if filterName == "" && (p.OS != osName || p.Arch != arch) {
			continue
		}
		item := &rowItem{
			name:    p.Name,
			version: p.Version,
			active:  p.Active,
			os:      p.OS,
			arch:    p.Arch,
		}
		filtered = append(filtered, item)
		rows = append(rows, *item)
	}

	if len(rows) == 0 {
		if filterName != "" {
			fmt.Printf("No versions of %s are installed.\n", filterName)
		} else {
			fmt.Println("No packages installed.")
			fmt.Printf("\nRun 'kompa search <name>' to find packages.\n")
		}
		return nil
	}

	// Print table.
	fmt.Printf("%-20s %-16s %-8s %s\n", "Package", "Version", "Status", "Platform")
	fmt.Printf("%-20s %-16s %-8s %s\n", "───────", "───────", "──────", "────────")
	for _, r := range rows {
		status := "installed"
		if r.active {
			status = "active"
		}
		plat := r.os + "/" + r.arch
		if filterName == "" {
			fmt.Printf("%-20s %-16s %-8s %s\n", r.name, r.version, status, plat)
		} else {
			marker := "  "
			if r.active {
				marker = "* "
			}
			fmt.Printf("%s%-20s %-16s %s\n", marker, r.name, r.version, plat)
		}
	}

	_ = filtered
	return nil
}
