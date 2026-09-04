// Package environment builds and prints the shell environment for installed Kompa packages.
package environment

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/kompa-tbm/kompa/internal/platform"
	"github.com/kompa-tbm/kompa/internal/store"
)

// Env holds the computed environment variable set for installed Kompa packages.
type Env struct {
	// Vars maps variable names to their values.
	Vars map[string]string
	// PathPrepend contains PATH entries to prepend (in order, first = highest priority).
	PathPrepend []string
}

// Build computes the Kompa environment from the installed packages and the Kompa bin directory.
func Build(dirs platform.Dirs, installedPkgs []*store.InstalledPackage) *Env {
	env := &Env{
		Vars:        make(map[string]string),
		PathPrepend: nil,
	}

	// Always prepend Kompa's own bin directory.
	env.PathPrepend = append(env.PathPrepend, dirs.Bin)

	// Accumulate per-package env vars.
	for _, pkg := range installedPkgs {
		if !pkg.Active {
			continue
		}
		for k, v := range pkg.RuntimeEnv {
			v = expandInstallDir(v, pkg.InstallPath)
			if k == "PATH" {
				// PATH entries get prepended.
				env.PathPrepend = append(env.PathPrepend, v)
			} else {
				// For other variables, later-installed packages win.
				env.Vars[k] = v
			}
		}
		// Add per-package lib and include paths.
		libDir := filepath.Join(pkg.InstallPath, "lib")
		includeDir := filepath.Join(pkg.InstallPath, "include")
		pkgconfigDir := filepath.Join(pkg.InstallPath, "lib", "pkgconfig")

		if dirExists(libDir) {
			appendEnvPath(env.Vars, "LIBRARY_PATH", libDir)
			if runtime.GOOS == "linux" {
				appendEnvPath(env.Vars, "LD_LIBRARY_PATH", libDir)
			} else if runtime.GOOS == "darwin" {
				appendEnvPath(env.Vars, "DYLD_LIBRARY_PATH", libDir)
			}
		}
		if dirExists(includeDir) {
			appendEnvPath(env.Vars, "CPATH", includeDir)
		}
		if dirExists(pkgconfigDir) {
			appendEnvPath(env.Vars, "PKG_CONFIG_PATH", pkgconfigDir)
		}
	}

	return env
}

// Diff returns env var assignments relative to the current process environment.
func Diff(env *Env) map[string]string {
	result := make(map[string]string)

	// Handle PATH separately.
	if len(env.PathPrepend) > 0 {
		currentPath := os.Getenv("PATH")
		sep := string(os.PathListSeparator)
		newPath := strings.Join(env.PathPrepend, sep)
		if currentPath != "" {
			newPath += sep + currentPath
		}
		result["PATH"] = newPath
	}

	for k, v := range env.Vars {
		if k == "PATH" {
			continue // handled above
		}
		current := os.Getenv(k)
		if strings.Contains(v, string(os.PathListSeparator)) {
			// It's a path-list variable.
			sep := string(os.PathListSeparator)
			if current != "" {
				result[k] = v + sep + current
			} else {
				result[k] = v
			}
		} else {
			result[k] = v
		}
	}

	return result
}

// PrintExport prints shell-specific export statements for the Kompa environment.
func PrintExport(dirs platform.Dirs, installedPkgs []*store.InstalledPackage, shell string) {
	env := Build(dirs, installedPkgs)
	diff := Diff(env)

	// Sort keys for deterministic output.
	keys := make([]string, 0, len(diff))
	for k := range diff {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	switch strings.ToLower(shell) {
	case "fish":
		for _, k := range keys {
			v := diff[k]
			if k == "PATH" {
				// Fish uses set -x PATH ...
				parts := strings.Split(v, string(os.PathListSeparator))
				fmt.Printf("set -x PATH %s\n", strings.Join(parts, " "))
			} else {
				fmt.Printf("set -x %s %s\n", k, shellQuote(v))
			}
		}
	case "cmd", "batch":
		for _, k := range keys {
			fmt.Printf("SET %s=%s\n", k, diff[k])
		}
	case "powershell", "pwsh":
		for _, k := range keys {
			fmt.Printf("$env:%s = %q\n", k, diff[k])
		}
	default:
		// sh/bash/zsh.
		for _, k := range keys {
			fmt.Printf("export %s=%s\n", k, shellQuote(diff[k]))
		}
	}
}

// PrintTable prints a human-readable table of env vars.
func PrintTable(dirs platform.Dirs, installedPkgs []*store.InstalledPackage) {
	env := Build(dirs, installedPkgs)
	diff := Diff(env)

	if len(diff) == 0 {
		fmt.Println("No environment changes (no packages installed).")
		return
	}

	fmt.Println("Kompa environment variables:")
	fmt.Println()

	keys := make([]string, 0, len(diff))
	for k := range diff {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		fmt.Printf("  %-24s %s\n", k, truncate(diff[k], 80))
	}
	fmt.Println()
	fmt.Println("To activate in your current shell:")
	fmt.Println()
	if runtime.GOOS == "windows" {
		fmt.Fprintln(os.Stdout, "  For PowerShell: kompa env --shell powershell | Invoke-Expression")
		// Use a variable to avoid go vet false positive on %T in %TEMP%
		cmdLine := "  For CMD:        kompa env --shell cmd > " + "%TEMP%" + "\\kenv.bat && " + "%TEMP%" + "\\kenv.bat"
		fmt.Fprintln(os.Stdout, cmdLine)
	} else {
		fmt.Fprintln(os.Stdout, `  eval "$(kompa env --shell bash)"`)
	}
}

// ShellScript returns a shell-specific script that activates the Kompa environment.
func ShellScript(dirs platform.Dirs, installedPkgs []*store.InstalledPackage) string {
	env := Build(dirs, installedPkgs)
	diff := Diff(env)

	var sb strings.Builder
	keys := make([]string, 0, len(diff))
	for k := range diff {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		fmt.Fprintf(&sb, "export %s=%s\n", k, shellQuote(diff[k]))
	}
	return sb.String()
}

func appendEnvPath(vars map[string]string, key, dir string) {
	sep := string(os.PathListSeparator)
	if existing, ok := vars[key]; ok {
		// Don't duplicate.
		for _, part := range strings.Split(existing, sep) {
			if part == dir {
				return
			}
		}
		vars[key] = existing + sep + dir
	} else {
		vars[key] = dir
	}
}

func expandInstallDir(val, installDir string) string {
	return strings.ReplaceAll(val, "{{install_dir}}", installDir)
}

func dirExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

func shellQuote(s string) string {
	if !strings.ContainsAny(s, " \t\n\"'\\$!`") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
