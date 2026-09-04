package cli

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/kompa-tbm/kompa/internal/environment"
)

func newEnvCmd(s *appState) *cobra.Command {
	var shell string

	cmd := &cobra.Command{
		Use:   "env",
		Short: "Print the Kompa environment variables",
		Long: `Print the environment variables needed to use Kompa-installed packages.

By default, output is a human-readable table.
Use --shell to output shell-specific export statements.

Examples:
  kompa env
  kompa env --shell bash
  kompa env --shell zsh
  kompa env --shell fish
  kompa env --shell powershell

  # Activate in current shell:
  eval "$(kompa env --shell bash)"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return s.runEnv(cmd, args, shell)
		},
	}

	cmd.Flags().StringVar(&shell, "shell", "", "Output format: bash, zsh, fish, powershell, cmd")

	return cmd
}

func (s *appState) runEnv(cmd *cobra.Command, args []string, shell string) error {
	installedPkgs := s.db.List()

	// Filter to active packages on the current platform.
	osName := string(s.plat.OS)
	arch := string(s.plat.Arch)
	var active []*interface{}
	_ = active

	var activePkgs = installedPkgs[:0]
	for _, p := range installedPkgs {
		if p.OS == osName && p.Arch == arch && p.Active {
			activePkgs = append(activePkgs, p)
		}
	}
	installedPkgs = activePkgs

	if shell != "" {
		environment.PrintExport(s.dirs, installedPkgs, shell)
		return nil
	}

	// Human-readable table.
	environment.PrintTable(s.dirs, installedPkgs)
	return nil
}

func newShellCmd(s *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "shell",
		Short: "Launch a new shell with Kompa packages active",
		Long: `Launch a subshell with the Kompa environment activated.

The shell is determined by the SHELL environment variable, or the
system default. All installed packages will be available via PATH.

Example:
  kompa shell`,
		RunE: s.runShell,
	}
}

func (s *appState) runShell(cmd *cobra.Command, args []string) error {
	installedPkgs := s.db.List()
	osName := string(s.plat.OS)
	arch := string(s.plat.Arch)
	var active = installedPkgs[:0]
	for _, p := range installedPkgs {
		if p.OS == osName && p.Arch == arch && p.Active {
			active = append(active, p)
		}
	}

	env := environment.Build(s.dirs, active)
	diff := environment.Diff(env)

	// Build the env for the subprocess.
	procEnv := os.Environ()
	for k, v := range diff {
		procEnv = setEnv(procEnv, k, v)
	}

	var shellBin string
	if runtime.GOOS == "windows" {
		shellBin = os.Getenv("COMSPEC")
		if shellBin == "" {
			shellBin = "cmd.exe"
		}
	} else {
		shellBin = os.Getenv("SHELL")
		if shellBin == "" {
			shellBin = "/bin/sh"
		}
	}

	shellPath, err := exec.LookPath(shellBin)
	if err != nil {
		return fmt.Errorf("could not find shell %s: %w", shellBin, err)
	}

	shellCmd := exec.Command(shellPath)
	shellCmd.Env = procEnv
	shellCmd.Stdin = os.Stdin
	shellCmd.Stdout = os.Stdout
	shellCmd.Stderr = os.Stderr

	fmt.Printf("Launching Kompa shell (%s). Type 'exit' to return.\n", shellBin)
	return shellCmd.Run()
}

// setEnv replaces or appends a KEY=VALUE pair in an env slice.
func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, e := range env {
		if len(e) >= len(prefix) && e[:len(prefix)] == prefix {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}
