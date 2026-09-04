package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kompa-tbm/kompa/internal/installer"
	"github.com/kompa-tbm/kompa/internal/manifest"
	"github.com/kompa-tbm/kompa/internal/resolver"
)

func newInstallCmd(s *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "install <package>[@version] [package...]",
		Short: "Install one or more packages",
		Long: `Install one or more packages from the Kompa package registry.

Dependencies are resolved and installed automatically.
Artifacts are downloaded from GitHub Releases, their checksums verified,
and the packages extracted into the Kompa installation directory.

Examples:
  kompa install gcc
  kompa install clang llvm
  kompa install gcc@14
  kompa install sqlite zlib`,
		Args:    cobra.MinimumNArgs(1),
		RunE:    s.runInstall,
	}
}

func (s *appState) runInstall(cmd *cobra.Command, args []string) error {
	s.printHeader()

	// Parse package names and optional versions.
	type pkgRequest struct {
		name    string
		version string // empty means "latest"
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

	// Validate all names exist in the registry.
	for _, req := range requests {
		if _, err := s.registry.Get(req.name); err != nil {
			return err
		}
	}

	osName := string(s.plat.OS)
	arch := string(s.plat.Arch)

	// Fetch the release manifest.
	s.logf("Resolving dependencies...")
	releaseManifest, err := s.fetchManifest()
	if err != nil {
		return fmt.Errorf("could not fetch package manifest: %w\n\nCheck your internet connection or GitHub API availability.", err)
	}

	// Verify platform support via registry.
	for _, req := range requests {
		def, _ := s.registry.Get(req.name)
		ok, reason := def.IsSupportedOn(osName, arch)
		if !ok {
			return fmt.Errorf("package %s is not supported on %s/%s: %s", req.name, osName, arch, reason)
		}
	}

	// Build the name list for dependency resolution.
	names := make([]string, len(requests))
	for i, r := range requests {
		names[i] = r.name
	}

	// Resolve dependencies.
	res := resolver.New(s.registry, func(name string) bool {
		return s.db.IsInstalled(name, osName, arch)
	})
	plan, err := res.Resolve(names)
	if err != nil {
		return fmt.Errorf("dependency resolution failed: %w", err)
	}

	// Report what will be installed.
	if len(plan.AlreadyInstalled) > 0 {
		for _, name := range plan.AlreadyInstalled {
			s.logf("  ✓ %s (already installed)", name)
		}
	}
	if len(plan.Ordered) == 0 {
		s.logf("Nothing to install.")
		return nil
	}
	for _, name := range plan.Ordered {
		s.logf("  • %s", name)
	}
	fmt.Println()

	// Confirm with user if not --yes.
	if !s.yes && !s.quiet {
		if !confirm(fmt.Sprintf("Install %d package(s)?", len(plan.Ordered))) {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// Install each package in dependency order.
	inst := installer.New(s.dirs, s.db, s.ghClient, s.plat, installer.Options{
		Verbose:      s.cfg.Verbose,
		ShowProgress: !s.quiet,
		Force:        s.force,
		YesAll:       s.yes,
		NoCache:      s.noCache,
	})

	ctx := context.Background()
	for _, name := range plan.Ordered {
		entry := findEntry(releaseManifest, name, osName, arch)
		if entry == nil {
			return fmt.Errorf(
				"package %s (%s/%s) not found in the latest release manifest.\n"+
					"It may not have been built yet. Check: https://github.com/%s/releases",
				name, osName, arch, s.cfg.GithubRepo,
			)
		}

		if err := inst.Install(ctx, entry); err != nil {
			return fmt.Errorf("installing %s: %w", name, err)
		}
	}

	fmt.Println()
	fmt.Println("Run:")
	if s.plat.OS == "windows" {
		fmt.Printf(`  $env:PATH = "%s;$env:PATH"`, s.dirs.Bin)
	} else {
		fmt.Printf(`  eval "$(kompa env --shell bash)"`)
	}
	fmt.Println()

	return nil
}

// findEntry looks up a package entry in the manifest, with version override if specified.
func findEntry(m *manifest.ReleaseManifest, name, osName, arch string) *manifest.PackageEntry {
	return m.Find(name, osName, arch)
}

// confirm prompts the user for a yes/no answer. Returns true for yes.
func confirm(prompt string) bool {
	fmt.Printf("%s [y/N] ", prompt)
	var response string
	fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}
