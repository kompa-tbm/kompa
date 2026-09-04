package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kompa-tbm/kompa/internal/config"
)

func newConfigCmd(s *appState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View and modify Kompa configuration",
		Long: `View and modify Kompa's persistent configuration.

Subcommands:
  get <key>         Print the value of a configuration key
  set <key> <value> Set a configuration key
  list              List all configuration keys and values
  reset             Reset configuration to defaults

Configuration keys:
  github_repo            GitHub repository hosting releases (owner/repo)
  github_token           GitHub personal access token
  verbose                Enable verbose output by default
  no_color               Disable ANSI colors
  cache_ttl              Metadata cache TTL in seconds
  max_download_retries   Number of download retries
  download_timeout_secs  Per-attempt download timeout
  parallel_downloads     Max simultaneous downloads
  kompa_home             Override Kompa data directory

Examples:
  kompa config list
  kompa config get github_repo
  kompa config set github_token ghp_xxxx
  kompa config set verbose true`,
	}

	cmd.AddCommand(
		newConfigGetCmd(s),
		newConfigSetCmd(s),
		newConfigListCmd(s),
		newConfigResetCmd(s),
	)
	return cmd
}

func newConfigGetCmd(s *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a configuration value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			val, err := config.Get(s.cfg, args[0])
			if err != nil {
				return err
			}
			fmt.Println(val)
			return nil
		},
	}
}

func newConfigSetCmd(s *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.Set(&s.cfg, args[0], args[1]); err != nil {
				return err
			}
			if err := config.Save(s.dirs, s.cfg); err != nil {
				return fmt.Errorf("saving configuration: %w", err)
			}
			fmt.Printf("Set %s = %s\n", args[0], args[1])
			return nil
		},
	}
}

func newConfigListCmd(s *appState) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all configuration values",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			keys := config.AllKeys()
			for _, key := range keys {
				val, err := config.Get(s.cfg, key)
				if err != nil {
					continue
				}
				// Redact token.
				if strings.Contains(key, "token") && val != "" {
					val = val[:min(4, len(val))] + strings.Repeat("*", len(val)-min(4, len(val)))
				}
				fmt.Printf("%-30s %s\n", key, val)
			}
			return nil
		},
	}
}

func newConfigResetCmd(s *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Reset configuration to defaults",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !s.yes {
				if !confirm("Reset all configuration to defaults?") {
					fmt.Println("Aborted.")
					return nil
				}
			}
			def := config.Defaults()
			if err := config.Save(s.dirs, def); err != nil {
				return err
			}
			s.cfg = def
			fmt.Println("Configuration reset to defaults.")
			return nil
		},
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
