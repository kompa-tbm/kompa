package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newUseCmd(s *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "use <package>@<version>",
		Short: "Switch the active version of a package",
		Long: `Switch which installed version of a package is the active one.

The active version is the one used when you run the package's binaries
through the Kompa bin directory.

Examples:
  kompa use gcc@14
  kompa use gcc@15`,
		Args: cobra.ExactArgs(1),
		RunE: s.runUse,
	}
}

func (s *appState) runUse(cmd *cobra.Command, args []string) error {
	parts := strings.SplitN(args[0], "@", 2)
	if len(parts) != 2 || parts[1] == "" {
		return fmt.Errorf("specify version with @: e.g. kompa use gcc@14")
	}

	name := parts[0]
	version := parts[1]
	osName := string(s.plat.OS)
	arch := string(s.plat.Arch)

	if s.db.Get(name, version, osName, arch) == nil {
		installed := s.db.ListByName(name)
		if len(installed) == 0 {
			return fmt.Errorf("package %s is not installed", name)
		}
		var versions []string
		for _, p := range installed {
			versions = append(versions, p.Version)
		}
		return fmt.Errorf(
			"version %s of %s is not installed.\nInstalled versions: %s",
			version, name, strings.Join(versions, ", "),
		)
	}

	if err := s.db.SetActive(name, version, osName, arch); err != nil {
		return err
	}

	fmt.Printf("Now using %s %s\n", name, version)
	return nil
}

func newCurrentCmd(s *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "current [package]",
		Short: "Show the currently active version of a package",
		Long: `Show the active (currently selected) version of an installed package.

If no package name is given, shows the active version of every installed package.

Examples:
  kompa current
  kompa current gcc`,
		Args: cobra.MaximumNArgs(1),
		RunE: s.runCurrent,
	}
}

func (s *appState) runCurrent(cmd *cobra.Command, args []string) error {
	osName := string(s.plat.OS)
	arch := string(s.plat.Arch)

	if len(args) == 1 {
		name := args[0]
		pkg := s.db.GetActive(name, osName, arch)
		if pkg == nil {
			return fmt.Errorf("no active version of %s found", name)
		}
		fmt.Printf("%s %s\n", pkg.Name, pkg.Version)
		return nil
	}

	// All packages.
	all := s.db.List()
	var found bool
	for _, p := range all {
		if p.OS == osName && p.Arch == arch && p.Active {
			fmt.Printf("%-20s %s\n", p.Name, p.Version)
			found = true
		}
	}
	if !found {
		fmt.Println("No packages installed.")
	}
	return nil
}
