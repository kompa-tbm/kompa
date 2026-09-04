package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newSearchCmd(s *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search available packages",
		Long: `Search the Kompa package registry for packages matching a query.

The search covers package names, descriptions, and tags.

Examples:
  kompa search gcc
  kompa search sqlite
  kompa search compiler`,
		Args:    cobra.ExactArgs(1),
		RunE:    s.runSearch,
	}
}

func (s *appState) runSearch(cmd *cobra.Command, args []string) error {
	query := args[0]
	results := s.registry.Search(query)

	if len(results) == 0 {
		fmt.Printf("No packages found matching %q.\n", query)
		return nil
	}

	if s.jsonOut {
		out, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	}

	osName := string(s.plat.OS)
	arch := string(s.plat.Arch)

	fmt.Printf("%-16s %-12s %-14s %s\n", "Name", "Version", "Installed", "Description")
	fmt.Printf("%-16s %-12s %-14s %s\n", "────", "───────", "─────────", "───────────")
	for _, def := range results {
		installed := ""
		if s.db.IsInstalled(def.Name, osName, arch) {
			pkg := s.db.GetLatest(def.Name, osName, arch)
			if pkg != nil {
				installed = "✓ " + pkg.Version
			}
		}
		desc := def.Description
		if len(desc) > 55 {
			desc = desc[:52] + "..."
		}
		fmt.Printf("%-16s %-12s %-14s %s\n", def.Name, def.Version, installed, desc)
	}
	return nil
}
