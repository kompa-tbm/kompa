package packages

import (
	"testing"
)

func TestLoadRegistry(t *testing.T) {
	r, err := LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry() error = %v", err)
	}

	names := r.Names()
	if len(names) == 0 {
		t.Fatal("LoadRegistry() returned empty registry")
	}

	// Verify every expected package is present.
	required := []string{
		"gcc", "gfortran", "clang", "llvm", "binutils",
		"gdb", "lldb", "zig", "go", "nim", "ocaml", "ghc",
		"ffmpeg", "lua", "zlib", "sqlite", "busybox",
	}
	for _, name := range required {
		if _, err := r.Get(name); err != nil {
			t.Errorf("registry missing required package %q", name)
		}
	}
}

func TestRegistry_Get_NotFound(t *testing.T) {
	r, err := LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry() error = %v", err)
	}

	_, err = r.Get("nonexistent-package-xyz")
	if err == nil {
		t.Error("Get(nonexistent) expected error, got nil")
	}
	if !IsNotFound(err) {
		t.Errorf("Get(nonexistent) error is not NotFoundError: %T", err)
	}
}

func TestRegistry_Search(t *testing.T) {
	r, err := LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry() error = %v", err)
	}

	results := r.Search("compiler")
	if len(results) == 0 {
		t.Error("Search(compiler) returned no results")
	}

	results = r.Search("sqlite")
	if len(results) == 0 {
		t.Error("Search(sqlite) returned no results")
	}

	results = r.Search("zzz_no_match_xyz")
	if len(results) != 0 {
		t.Errorf("Search(no-match) returned %d results, want 0", len(results))
	}
}

func TestRegistry_ValidateAll(t *testing.T) {
	r, err := LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry() error = %v", err)
	}

	errs := r.ValidateAll()
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("validation error: %v", e)
		}
	}
}

func TestDefinition_IsSupportedOn(t *testing.T) {
	r, err := LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry() error = %v", err)
	}

	gcc, err := r.Get("gcc")
	if err != nil {
		t.Fatalf("Get(gcc) error = %v", err)
	}

	// GCC is supported on linux/amd64.
	ok, reason := gcc.IsSupportedOn("linux", "amd64")
	if !ok {
		t.Errorf("gcc.IsSupportedOn(linux, amd64) = false, reason: %s", reason)
	}

	// GCC on Windows is explicitly unsupported.
	ok, reason = gcc.IsSupportedOn("windows", "amd64")
	if ok {
		t.Error("gcc.IsSupportedOn(windows, amd64) = true, want false")
	}
	if reason == "" {
		t.Error("gcc.IsSupportedOn(windows, amd64) reason is empty, want explanation")
	}
}

func TestDefinition_ArtifactName(t *testing.T) {
	def := &Definition{
		Name:           "gcc",
		ArtifactFormat: "tar.zst",
	}

	tests := []struct {
		os   string
		arch string
		want string
	}{
		{"linux", "amd64", "gcc-linux-amd64.tar.zst"},
		{"darwin", "arm64", "gcc-darwin-arm64.tar.zst"},
		{"windows", "amd64", "gcc-windows-amd64.tar.zst"},
	}
	for _, tt := range tests {
		got := def.ArtifactName(tt.os, tt.arch)
		if got != tt.want {
			t.Errorf("ArtifactName(%s, %s) = %q, want %q", tt.os, tt.arch, got, tt.want)
		}
	}
}

func TestDefinition_ArtifactName_DefaultFormat(t *testing.T) {
	def := &Definition{Name: "lua"}

	// Windows defaults to zip.
	if got := def.ArtifactName("windows", "amd64"); got != "lua-windows-amd64.zip" {
		t.Errorf("ArtifactName(windows, amd64) = %q, want zip extension", got)
	}

	// Linux defaults to tar.zst.
	if got := def.ArtifactName("linux", "amd64"); got != "lua-linux-amd64.tar.zst" {
		t.Errorf("ArtifactName(linux, amd64) = %q, want tar.zst extension", got)
	}
}

func TestValidate_Errors(t *testing.T) {
	tests := []struct {
		name string
		def  *Definition
	}{
		{"empty name", &Definition{Version: "1.0", Description: "test", SourceURL: "https://example.com", BuildSystem: BuildMake}},
		{"empty version", &Definition{Name: "foo", Description: "test", SourceURL: "https://example.com", BuildSystem: BuildMake}},
		{"empty description", &Definition{Name: "foo", Version: "1.0", SourceURL: "https://example.com", BuildSystem: BuildMake}},
		{"no source", &Definition{Name: "foo", Version: "1.0", Description: "test", BuildSystem: BuildMake}},
		{"bad build system", &Definition{Name: "foo", Version: "1.0", Description: "test", SourceURL: "https://example.com", BuildSystem: "unknown"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Validate(tt.def); err == nil {
				t.Errorf("Validate() expected error for %q, got nil", tt.name)
			}
		})
	}
}

func TestValidate_OK(t *testing.T) {
	def := &Definition{
		Name:        "foo",
		Version:     "1.0",
		Description: "A test package",
		SourceURL:   "https://example.com/foo-1.0.tar.gz",
		BuildSystem: BuildAutoconf,
	}
	if err := Validate(def); err != nil {
		t.Errorf("Validate() unexpected error: %v", err)
	}
}

func TestRegistry_All_Sorted(t *testing.T) {
	r, err := LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry() error = %v", err)
	}
	all := r.All()
	for i := 1; i < len(all); i++ {
		if all[i].Name < all[i-1].Name {
			t.Errorf("All() not sorted: %q before %q", all[i-1].Name, all[i].Name)
		}
	}
}
