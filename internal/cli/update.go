package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kompa-tbm/kompa/internal/installer"
	"github.com/kompa-tbm/kompa/internal/resolver"
)

func newUpdateCmd(s *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "update <package|all> [package...]",
		Short: "Update installed packages to the latest available version",
		Long: `Update one or more installed packages to the latest Kompa-built version.

Use 'all' as the package name to update everything installed.

Examples:
  kompa update gcc
  kompa update all
  kompa update sqlite lua`,
		Args:    cobra.MinimumNArgs(1),
		RunE:    s.runUpdate,
	}
}

func (s *appState) runUpdate(cmd *cobra.Command, args []string) error {
	s.printHeader()

	osName := string(s.plat.OS)
	arch := string(s.plat.Arch)

	// Determine which packages to update.
	var names []string
	if len(args) == 1 && strings.ToLower(args[0]) == "all" {
		installed := s.db.List()
		seen := make(map[string]struct{})
		for _, p := range installed {
			if p.OS == osName && p.Arch == arch {
				if _, ok := seen[p.Name]; !ok {
					names = append(names, p.Name)
					seen[p.Name] = struct{}{}
				}
			}
		}
		if len(names) == 0 {
			fmt.Println("No packages installed.")
			return nil
		}
	} else {
		for _, arg := range args {
			name := strings.SplitN(arg, "@", 2)[0]
			if !s.db.IsInstalled(name, osName, arch) {
				return fmt.Errorf("package %s is not installed", name)
			}
			names = append(names, name)
		}
	}

	// Fetch latest manifest.
	s.logf("Fetching latest release...")
	releaseManifest, err := s.fetchManifest()
	if err != nil {
		return fmt.Errorf("fetching package manifest: %w", err)
	}

	// Build ordered update plan.
	res := resolver.New(s.registry, func(name string) bool {
		return s.db.IsInstalled(name, osName, arch)
	})
	ordered, err := res.UpdateOrder(names)
	if err != nil {
		return fmt.Errorf("computing update order: %w", err)
	}

	// Determine which packages actually have new versions.
	type updateItem struct {
		name       string
		oldVersion string
		newVersion string
	}
	var toUpdate []updateItem

	for _, name := range ordered {
		entry := releaseManifest.Find(name, osName, arch)
		if entry == nil {
			s.verbosef("skipping %s: not in current manifest for %s/%s", name, osName, arch)
			continue
		}
		current := s.db.GetLatest(name, osName, arch)
		if current == nil {
			// Not installed — treat as fresh install.
			toUpdate = append(toUpdate, updateItem{name: name, newVersion: entry.Version})
			continue
		}
		if current.Version == entry.Version && !s.force {
			s.logf("  ✓ %s %s (up to date)", name, current.Version)
			continue
		}
		toUpdate = append(toUpdate, updateItem{
			name:       name,
			oldVersion: current.Version,
			newVersion: entry.Version,
		})
	}

	if len(toUpdate) == 0 {
		fmt.Println("All packages are up to date.")
		return nil
	}

	fmt.Printf("Packages to update (%d):\n", len(toUpdate))
	for _, u := range toUpdate {
		if u.oldVersion == "" {
			s.logf("  • %s → %s (new)", u.name, u.newVersion)
		} else {
			s.logf("  • %s  %s → %s", u.name, u.oldVersion, u.newVersion)
		}
	}
	fmt.Println()

	if !s.yes && !s.quiet {
		if !confirm(fmt.Sprintf("Update %d package(s)?", len(toUpdate))) {
			fmt.Println("Aborted.")
			return nil
		}
	}

	inst := installer.New(s.dirs, s.db, s.ghClient, s.plat, installer.Options{
		Verbose:      s.cfg.Verbose,
		ShowProgress: !s.quiet,
		Force:        true, // always re-install on update
		YesAll:       s.yes,
		NoCache:      s.noCache,
	})

	ctx := context.Background()
	for _, u := range toUpdate {
		entry := releaseManifest.Find(u.name, osName, arch)
		if err := inst.Install(ctx, entry); err != nil {
			return fmt.Errorf("updating %s: %w", u.name, err)
		}
	}

	return nil
}
