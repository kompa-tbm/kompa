package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kompa-tbm/kompa/internal/installer"
	"github.com/kompa-tbm/kompa/internal/resolver"
)

func newRemoveCmd(s *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove <package> [package...]",
		Aliases: []string{"uninstall", "rm"},
		Short:   "Remove installed packages",
		Long: `Remove one or more installed packages from the Kompa installation.

Only files that were installed by Kompa for that package are removed.
Kompa will warn if other installed packages depend on the one being removed.

Examples:
  kompa remove gcc
  kompa remove sqlite zlib
  kompa uninstall lua`,
		Args:  cobra.MinimumNArgs(1),
		RunE:  s.runRemove,
	}
	return cmd
}

func (s *appState) runRemove(cmd *cobra.Command, args []string) error {
	s.printHeader()

	osName := string(s.plat.OS)
	arch := string(s.plat.Arch)

	// Parse package names and optional version specifiers.
	type pkgRequest struct {
		name    string
		version string
	}
	requests := make([]pkgRequest, 0, len(args))
	for _, arg := range args {
		parts := strings.SplitN(arg, "@", 2)
		req := pkgRequest{name: parts[0]}
		if len(parts) == 2 {
			req.version = parts[1]
		}
		requests = append(requests, req)
	}

	// Verify each package is actually installed.
	for _, req := range requests {
		if req.version != "" {
			if s.db.Get(req.name, req.version, osName, arch) == nil {
				return fmt.Errorf("package %s@%s is not installed", req.name, req.version)
			}
		} else {
			if !s.db.IsInstalled(req.name, osName, arch) {
				return fmt.Errorf("package %s is not installed", req.name)
			}
		}
	}

	// Check for reverse dependencies.
	names := make([]string, len(requests))
	for i, r := range requests {
		names[i] = r.name
	}

	res := resolver.New(s.registry, func(name string) bool {
		return s.db.IsInstalled(name, osName, arch)
	})
	_, warnings, err := res.ResolveRemove(names, func(name string) bool {
		return s.db.IsInstalled(name, osName, arch)
	})
	if err != nil {
		return err
	}
	for _, w := range warnings {
		fmt.Printf("  ⚠ %s\n", w)
	}
	if len(warnings) > 0 {
		fmt.Println()
	}

	// List what will be removed.
	for _, req := range requests {
		s.logf("  • %s", req.name)
	}
	fmt.Println()

	if !s.yes && !s.quiet {
		if !confirm(fmt.Sprintf("Remove %d package(s)?", len(requests))) {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// Remove each package.
	inst := installer.New(s.dirs, s.db, s.ghClient, s.plat, installer.Options{
		Verbose: s.cfg.Verbose,
	})

	for _, req := range requests {
		var version string
		if req.version != "" {
			version = req.version
		} else {
			pkg := s.db.GetLatest(req.name, osName, arch)
			if pkg == nil {
				return fmt.Errorf("package %s not found in database", req.name)
			}
			version = pkg.Version
		}

		if err := inst.Uninstall(req.name, version, osName, arch); err != nil {
			return fmt.Errorf("removing %s: %w", req.name, err)
		}
	}

	return nil
}
