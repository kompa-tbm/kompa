package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newInfoCmd(s *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "info <package>",
		Short: "Show detailed information about a package",
		Long: `Show detailed metadata for a package in the Kompa registry.

Includes version, description, source, dependencies, supported platforms,
and installation status.

Examples:
  kompa info gcc
  kompa info sqlite
  kompa info --json gcc`,
		Args:    cobra.ExactArgs(1),
		RunE:    s.runInfo,
	}
}

func (s *appState) runInfo(cmd *cobra.Command, args []string) error {
	name := args[0]

	def, err := s.registry.Get(name)
	if err != nil {
		return err
	}

	osName := string(s.plat.OS)
	arch := string(s.plat.Arch)

	if s.jsonOut {
		type infoOutput struct {
			Definition  interface{}       `json:"definition"`
			Installed   interface{}       `json:"installed"`
		}
		out := infoOutput{Definition: def}
		if pkg := s.db.GetLatest(name, osName, arch); pkg != nil {
			out.Installed = pkg
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Name:        %s\n", def.Name)
	fmt.Printf("Version:     %s\n", def.Version)
	fmt.Printf("Description: %s\n", def.Description)
	fmt.Printf("Homepage:    %s\n", def.Homepage)
	fmt.Printf("License:     %s\n", def.License)
	fmt.Println()

	fmt.Printf("Source:      %s\n", def.SourceURL)
	if def.SourceTag != "" {
		fmt.Printf("Tag:         %s\n", def.SourceTag)
	}
	fmt.Printf("Build:       %s\n", def.BuildSystem)
	fmt.Println()

	if len(def.Dependencies) > 0 {
		fmt.Printf("Dependencies: %s\n", strings.Join(def.Dependencies, ", "))
	} else {
		fmt.Printf("Dependencies: none\n")
	}

	if len(def.Binaries) > 0 {
		fmt.Printf("Binaries:    %s\n", strings.Join(def.Binaries, ", "))
	}
	fmt.Println()

	// Supported platforms.
	fmt.Println("Supported platforms:")
	if len(def.SupportedPlatforms) == 0 {
		fmt.Println("  all")
	} else {
		for _, sp := range def.SupportedPlatforms {
			if sp.UnsupportedReason != "" {
				fmt.Printf("  ✗ %s/%s — %s\n", sp.OS, sp.Arch, sp.UnsupportedReason)
			} else {
				marker := "  ✓"
				if sp.OS == osName && sp.Arch == arch {
					marker = "  ★"
				}
				fmt.Printf("%s %s/%s\n", marker, sp.OS, sp.Arch)
			}
		}
	}
	fmt.Println()

	// Installation status.
	if pkg := s.db.GetLatest(name, osName, arch); pkg != nil {
		fmt.Printf("Installed:   %s (installed %s)\n",
			pkg.Version, pkg.InstalledAt.Format("2006-01-02"))
		fmt.Printf("Location:    %s\n", pkg.InstallPath)
	} else {
		fmt.Printf("Installed:   no\n")
	}

	if len(def.Tags) > 0 {
		fmt.Printf("Tags:        %s\n", strings.Join(def.Tags, ", "))
	}

	return nil
}
