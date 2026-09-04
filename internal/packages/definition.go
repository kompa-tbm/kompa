// Package packages defines the package definition schema and registry.
package packages

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

//go:embed registry
var registryFS embed.FS

// BuildSystem identifies the build system a package uses.
type BuildSystem string

const (
	BuildAutoconf BuildSystem = "autoconf"
	BuildCMake    BuildSystem = "cmake"
	BuildMake     BuildSystem = "make"
	BuildMeson    BuildSystem = "meson"
	BuildCustom   BuildSystem = "custom"
	BuildPrebuilt BuildSystem = "prebuilt"
)

// ArtifactFormat defines the archive format for distributed packages.
type ArtifactFormat string

const (
	FormatTarZst ArtifactFormat = "tar.zst"
	FormatTarGz  ArtifactFormat = "tar.gz"
	FormatTarXz  ArtifactFormat = "tar.xz"
	FormatZip    ArtifactFormat = "zip"
)

// SupportedPlatform defines one supported OS+Arch combination for a package.
type SupportedPlatform struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
	// UnsupportedReason contains a human-readable explanation if this combination
	// is explicitly listed as unsupported.
	UnsupportedReason string `json:"unsupported_reason,omitempty"`
}

// EnvVar defines an environment variable that should be set when a package is active.
type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	// Prepend indicates the value should be prepended to an existing PATH-like variable.
	Prepend bool `json:"prepend,omitempty"`
}

// Definition is the full metadata record for a Kompa package.
type Definition struct {
	// Schema version for forward/backward compatibility.
	SchemaVersion int `json:"schema_version"`

	// Identity
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Homepage    string `json:"homepage"`
	License     string `json:"license"`

	// Upstream source
	SourceRepo    string `json:"source_repo"`
	SourceURL     string `json:"source_url"`
	SourceTag     string `json:"source_tag"`
	SourceSHA256  string `json:"source_sha256,omitempty"`

	// Build
	BuildSystem   BuildSystem `json:"build_system"`
	BuildFlags    []string    `json:"build_flags,omitempty"`
	BuildEnv      []EnvVar    `json:"build_env,omitempty"`
	ConfigureArgs []string    `json:"configure_args,omitempty"`
	MakeTargets   []string    `json:"make_targets,omitempty"`

	// Distribution
	ArtifactFormat ArtifactFormat `json:"artifact_format"`

	// Dependencies (other Kompa package names).
	Dependencies []string `json:"dependencies,omitempty"`

	// Build-only dependencies (not needed at runtime).
	BuildDependencies []string `json:"build_dependencies,omitempty"`

	// Supported platforms. If empty, all platforms are assumed supported.
	SupportedPlatforms []SupportedPlatform `json:"supported_platforms,omitempty"`

	// Layout describes what gets installed.
	Binaries  []string `json:"binaries,omitempty"`
	Libraries []string `json:"libraries,omitempty"`
	Headers   []string `json:"headers,omitempty"`

	// RuntimeEnv lists environment variables to set when this package is active.
	RuntimeEnv []EnvVar `json:"runtime_env,omitempty"`

	// Tags for search/categorization.
	Tags []string `json:"tags,omitempty"`
}

// IsSupportedOn returns true if the package supports the given OS and architecture.
// It also returns the reason string if the platform is explicitly marked unsupported.
func (d *Definition) IsSupportedOn(os, arch string) (bool, string) {
	if len(d.SupportedPlatforms) == 0 {
		// No explicit list — assume universally supported.
		return true, ""
	}
	for _, sp := range d.SupportedPlatforms {
		if sp.OS == os && sp.Arch == arch {
			if sp.UnsupportedReason != "" {
				return false, sp.UnsupportedReason
			}
			return true, ""
		}
	}
	return false, fmt.Sprintf("platform %s/%s is not in the supported platform list for %s", os, arch, d.Name)
}

// ArtifactName returns the expected artifact filename for a given platform.
func (d *Definition) ArtifactName(os, arch string) string {
	ext := string(d.ArtifactFormat)
	if ext == "" {
		if os == "windows" {
			ext = "zip"
		} else {
			ext = "tar.zst"
		}
	}
	return fmt.Sprintf("%s-%s-%s.%s", d.Name, os, arch, ext)
}

// Registry is an in-memory index of all known package definitions.
type Registry struct {
	packages map[string]*Definition
}

// LoadRegistry loads all package definitions from the embedded filesystem.
func LoadRegistry() (*Registry, error) {
	r := &Registry{packages: make(map[string]*Definition)}

	err := fs.WalkDir(registryFS, "registry", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		data, err := registryFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}
		var def Definition
		if err := json.Unmarshal(data, &def); err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}
		if def.Name == "" {
			return fmt.Errorf("package definition %s has no name", path)
		}
		r.packages[def.Name] = &def
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("loading package registry: %w", err)
	}
	return r, nil
}

// Get returns the Definition for the named package, or an error if not found.
func (r *Registry) Get(name string) (*Definition, error) {
	def, ok := r.packages[name]
	if !ok {
		return nil, &NotFoundError{Name: name}
	}
	return def, nil
}

// All returns all package definitions sorted by name.
func (r *Registry) All() []*Definition {
	out := make([]*Definition, 0, len(r.packages))
	for _, d := range r.packages {
		out = append(out, d)
	}
	// Sort by name for determinism.
	sortDefinitions(out)
	return out
}

// Search returns definitions whose name, description, or tags match the query.
func (r *Registry) Search(query string) []*Definition {
	query = strings.ToLower(query)
	var results []*Definition
	for _, d := range r.packages {
		if strings.Contains(strings.ToLower(d.Name), query) ||
			strings.Contains(strings.ToLower(d.Description), query) ||
			containsTag(d.Tags, query) {
			results = append(results, d)
		}
	}
	sortDefinitions(results)
	return results
}

// Names returns the sorted list of all package names in the registry.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.packages))
	for k := range r.packages {
		names = append(names, k)
	}
	sortStrings(names)
	return names
}

// ValidateAll validates every definition in the registry and returns any errors.
func (r *Registry) ValidateAll() []error {
	var errs []error
	for _, d := range r.packages {
		if err := Validate(d); err != nil {
			errs = append(errs, fmt.Errorf("package %s: %w", d.Name, err))
		}
	}
	return errs
}

// Validate checks a Definition for required fields and consistency.
func Validate(d *Definition) error {
	if d.Name == "" {
		return fmt.Errorf("name is required")
	}
	if d.Version == "" {
		return fmt.Errorf("version is required")
	}
	if d.Description == "" {
		return fmt.Errorf("description is required")
	}
	if d.SourceURL == "" && d.SourceRepo == "" {
		return fmt.Errorf("source_url or source_repo is required")
	}
	validBuildSystems := map[BuildSystem]bool{
		BuildAutoconf: true, BuildCMake: true, BuildMake: true,
		BuildMeson: true, BuildCustom: true, BuildPrebuilt: true,
	}
	if !validBuildSystems[d.BuildSystem] {
		return fmt.Errorf("invalid build_system %q", d.BuildSystem)
	}
	return nil
}

// NotFoundError is returned when a package is not in the registry.
type NotFoundError struct {
	Name string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("package %q not found in registry", e.Name)
}

// IsNotFound returns true if err is a *NotFoundError.
func IsNotFound(err error) bool {
	_, ok := err.(*NotFoundError)
	return ok
}

// ArtifactBaseName returns the base name without the package directory prefix.
func ArtifactBaseName(path string) string {
	return filepath.Base(path)
}

func containsTag(tags []string, query string) bool {
	for _, t := range tags {
		if strings.Contains(strings.ToLower(t), query) {
			return true
		}
	}
	return false
}

func sortDefinitions(defs []*Definition) {
	for i := 1; i < len(defs); i++ {
		for j := i; j > 0 && defs[j].Name < defs[j-1].Name; j-- {
			defs[j], defs[j-1] = defs[j-1], defs[j]
		}
	}
}

func sortStrings(ss []string) {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j] < ss[j-1]; j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
}
