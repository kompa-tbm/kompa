// Package cli implements the Kompa command-line interface.
package cli

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/kompa-tbm/kompa/internal/config"
	githubclient "github.com/kompa-tbm/kompa/internal/github"
	"github.com/kompa-tbm/kompa/internal/manifest"
	"github.com/kompa-tbm/kompa/internal/packages"
	"github.com/kompa-tbm/kompa/internal/platform"
	"github.com/kompa-tbm/kompa/internal/store"
)

// Version is injected at build time via -ldflags.
var Version = "dev"

// appState holds shared runtime state threaded through all commands.
type appState struct {
	dirs     platform.Dirs
	cfg      config.Config
	db       *store.DB
	registry *packages.Registry
	ghClient *githubclient.Client
	plat     platform.Platform

	// flags
	verbose  bool
	quiet    bool
	yes      bool
	force    bool
	jsonOut  bool
	noCache  bool
}

// newRoot constructs and returns the root Cobra command with all subcommands attached.
func newRoot() *cobra.Command {
	state := &appState{}

	root := &cobra.Command{
		Use:   "kompa",
		Short: "Kompa — cross-platform developer toolchain manager",
		Long: `Kompa
─────
A cross-platform developer toolchain manager.

Install, update, and manage compilers, debuggers, and development libraries
from a single CLI. Packages are built from source by Kompa's CI and distributed
as verified artifacts through GitHub Releases.

Examples:
  kompa install gcc
  kompa install clang llvm
  kompa list
  kompa info sqlite
  kompa env
  kompa doctor`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip initialisation for top-level help / version / self-update.
			skip := map[string]bool{
				"version":     true,
				"help":        true,
				"self-update": true,
			}
			if skip[cmd.Name()] {
				return nil
			}
			return state.init(cmd)
		},
	}

	// Global flags.
	root.PersistentFlags().BoolVarP(&state.verbose, "verbose", "v", false, "Enable verbose output")
	root.PersistentFlags().BoolVarP(&state.quiet, "quiet", "q", false, "Suppress non-essential output")
	root.PersistentFlags().BoolVarP(&state.yes, "yes", "y", false, "Answer yes to all confirmation prompts")
	root.PersistentFlags().BoolVar(&state.force, "force", false, "Force the operation even if already done")
	root.PersistentFlags().BoolVar(&state.jsonOut, "json", false, "Output results as JSON")
	root.PersistentFlags().BoolVar(&state.noCache, "no-cache", false, "Bypass the local download cache")

	// Subcommands.
	root.AddCommand(
		newInstallCmd(state),
		newRemoveCmd(state),
		newUpdateCmd(state),
		newListCmd(state),
		newSearchCmd(state),
		newInfoCmd(state),
		newEnvCmd(state),
		newDoctorCmd(state),
		newVersionCmd(),
		newCleanCmd(state),
		newCacheCmd(state),
		newConfigCmd(state),
		newSelfUpdateCmd(),
		newUseCmd(state),
		newCurrentCmd(state),
	)

	return root
}

// Execute is the entry point called from main.
func Execute() {
	root := newRoot()
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// init loads shared state once before any command runs.
func (s *appState) init(cmd *cobra.Command) error {
	s.plat = platform.Current()

	dirs, err := platform.GetDirs()
	if err != nil {
		return fmt.Errorf("determining Kompa directories: %w", err)
	}
	s.dirs = dirs

	if err := platform.EnsureDirs(dirs); err != nil {
		return fmt.Errorf("creating Kompa directories: %w", err)
	}

	cfg, err := config.Load(dirs)
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	s.cfg = cfg

	// Verbose flag overrides config.
	if s.verbose {
		s.cfg.Verbose = true
	}

	db, err := store.Open(dirs.DB)
	if err != nil {
		return fmt.Errorf("opening package database: %w", err)
	}
	s.db = db

	registry, err := packages.LoadRegistry()
	if err != nil {
		return fmt.Errorf("loading package registry: %w", err)
	}
	s.registry = registry

	token := cfg.GithubToken
	if envToken := os.Getenv("GITHUB_TOKEN"); envToken != "" {
		token = envToken
	}
	s.ghClient = githubclient.NewClient(cfg.GithubRepo, token)

	return nil
}

// fetchManifest retrieves and parses the latest release manifest.
func (s *appState) fetchManifest() (*manifest.ReleaseManifest, error) {
	release, err := s.ghClient.LatestReleaseWithManifest()
	if err != nil {
		return nil, fmt.Errorf("fetching latest release: %w", err)
	}

	data, err := s.ghClient.FetchManifestContent(release)
	if err != nil {
		return nil, fmt.Errorf("fetching manifest: %w", err)
	}

	m, err := manifest.ParseManifest(data)
	if err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}
	return m, nil
}

// printHeader prints the Kompa header unless quiet mode is on.
func (s *appState) printHeader() {
	if !s.quiet {
		fmt.Println()
		fmt.Println("Kompa")
		fmt.Println("─────")
		fmt.Println()
	}
}

// logf prints a formatted message unless quiet.
func (s *appState) logf(format string, args ...interface{}) {
	if !s.quiet {
		fmt.Printf(format+"\n", args...)
	}
}

// verbosef prints only in verbose mode.
func (s *appState) verbosef(format string, args ...interface{}) {
	if s.cfg.Verbose {
		fmt.Printf("[verbose] "+format+"\n", args...)
	}
}

// errorf always prints an error.
func errorf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
}

// currentOS returns the OS string for the current platform.
func currentOS() string {
	return runtime.GOOS
}

// currentArch returns the arch string for the current platform.
func currentArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "amd64"
	case "arm64":
		return "arm64"
	case "386":
		return "386"
	default:
		return runtime.GOARCH
	}
}
