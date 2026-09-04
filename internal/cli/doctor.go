package cli

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/cobra"
)

func newDoctorCmd(s *appState) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Run diagnostics on the Kompa installation",
		Long: `Check the Kompa installation and environment for common problems.

Checks:
  • Kompa directories exist and are writable
  • Package database is valid
  • GitHub API is reachable
  • Required system tools are available
  • PATH includes the Kompa bin directory
  • No corrupted package installations

Examples:
  kompa doctor`,
		Args: cobra.NoArgs,
		RunE: s.runDoctor,
	}
}

type checkResult struct {
	name    string
	ok      bool
	message string
}

func pass(name, msg string) checkResult { return checkResult{name: name, ok: true, message: msg} }
func fail(name, msg string) checkResult { return checkResult{name: name, ok: false, message: msg} }

func (s *appState) runDoctor(cmd *cobra.Command, args []string) error {
	fmt.Println("Kompa Doctor")
	fmt.Println("────────────")
	fmt.Println()

	var results []checkResult

	// 1. Check directories.
	dirs := []struct {
		label string
		path  string
	}{
		{"Root directory", s.dirs.Root},
		{"Bin directory", s.dirs.Bin},
		{"Packages directory", s.dirs.Packages},
		{"Cache directory", s.dirs.Cache},
		{"Downloads directory", s.dirs.Downloads},
	}
	for _, d := range dirs {
		fi, err := os.Stat(d.path)
		if err != nil {
			results = append(results, fail(d.label, fmt.Sprintf("missing: %s", d.path)))
			continue
		}
		if !fi.IsDir() {
			results = append(results, fail(d.label, fmt.Sprintf("not a directory: %s", d.path)))
			continue
		}
		// Write test.
		probe := filepath.Join(d.path, ".kompa_probe")
		if err := os.WriteFile(probe, []byte("ok"), 0644); err != nil {
			results = append(results, fail(d.label, fmt.Sprintf("not writable: %s: %v", d.path, err)))
		} else {
			_ = os.Remove(probe)
			results = append(results, pass(d.label, d.path))
		}
	}

	// 2. Check package database.
	pkgs := s.db.List()
	results = append(results, pass("Package database", fmt.Sprintf("%d packages installed", len(pkgs))))

	// 3. Registry validation.
	errs := s.registry.ValidateAll()
	if len(errs) > 0 {
		results = append(results, fail("Package registry", fmt.Sprintf("%d invalid definitions", len(errs))))
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  registry error: %v\n", e)
		}
	} else {
		results = append(results, pass("Package registry", fmt.Sprintf("%d packages", len(s.registry.All()))))
	}

	// 4. GitHub API connectivity.
	apiResult := checkGitHubAPI(s.cfg.GithubRepo)
	results = append(results, apiResult)

	// 5. PATH includes Kompa bin.
	pathEnv := os.Getenv("PATH")
	if containsPath(pathEnv, s.dirs.Bin) {
		results = append(results, pass("PATH includes kompa/bin", s.dirs.Bin))
	} else {
		results = append(results, fail("PATH includes kompa/bin",
			fmt.Sprintf("%s is not in PATH — run: eval \"$(kompa env --shell bash)\"", s.dirs.Bin)))
	}

	// 6. System tool checks.
	tools := []string{"tar", "xz"}
	if runtime.GOOS == "windows" {
		tools = []string{"tar"}
	}
	for _, tool := range tools {
		if path, err := exec.LookPath(tool); err == nil {
			results = append(results, pass("System tool: "+tool, path))
		} else {
			results = append(results, fail("System tool: "+tool, "not found in PATH"))
		}
	}

	// 7. Check installed package directories exist.
	brokenCount := 0
	for _, pkg := range pkgs {
		if _, err := os.Stat(pkg.InstallPath); err != nil {
			brokenCount++
			if s.cfg.Verbose {
				fmt.Printf("  ⚠ broken install: %s@%s at %s\n", pkg.Name, pkg.Version, pkg.InstallPath)
			}
		}
	}
	if brokenCount > 0 {
		results = append(results, fail("Installed package integrity",
			fmt.Sprintf("%d package(s) have missing install directories; run 'kompa doctor --verbose'", brokenCount)))
	} else if len(pkgs) > 0 {
		results = append(results, pass("Installed package integrity", "all install directories present"))
	}

	// Print results.
	allOK := true
	for _, r := range results {
		if r.ok {
			fmt.Printf("  ✓ %-40s %s\n", r.name, r.message)
		} else {
			allOK = false
			fmt.Printf("  ✗ %-40s %s\n", r.name, r.message)
		}
	}

	fmt.Println()
	if allOK {
		fmt.Println("All checks passed.")
	} else {
		fmt.Println("Some checks failed. See above for details.")
		return fmt.Errorf("doctor found problems")
	}
	return nil
}

func checkGitHubAPI(repo string) checkResult {
	url := fmt.Sprintf("https://api.github.com/repos/%s", repo)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fail("GitHub API", fmt.Sprintf("unreachable: %v", err))
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
		// 404 for a nonexistent repo is still proof the API is reachable.
		return pass("GitHub API", "reachable")
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return fail("GitHub API", "rate limited; set GITHUB_TOKEN for higher limits")
	}
	return fail("GitHub API", fmt.Sprintf("HTTP %d", resp.StatusCode))
}

func containsPath(pathEnv, target string) bool {
	sep := string(os.PathListSeparator)
	for _, p := range splitPath(pathEnv, sep) {
		if filepath.Clean(p) == filepath.Clean(target) {
			return true
		}
	}
	return false
}

func splitPath(s, sep string) []string {
	if s == "" {
		return nil
	}
	var parts []string
	for _, p := range filepath.SplitList(s) {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}
