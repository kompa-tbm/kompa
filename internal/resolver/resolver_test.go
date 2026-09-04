package resolver

import (
	"testing"

	"github.com/kompa-tbm/kompa/internal/packages"
)

func loadRegistry(t *testing.T) *packages.Registry {
	t.Helper()
	r, err := packages.LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry() error = %v", err)
	}
	return r
}

// noneInstalled is a helper that reports nothing as installed.
func noneInstalled(_ string) bool { return false }

// allInstalled reports every package as installed.
func allInstalled(_ string) bool { return true }

func TestResolve_SinglePackageNoDeps(t *testing.T) {
	r := loadRegistry(t)
	res := New(r, noneInstalled)

	plan, err := res.Resolve([]string{"zlib"})
	if err != nil {
		t.Fatalf("Resolve(zlib) error = %v", err)
	}
	if len(plan.Ordered) != 1 || plan.Ordered[0] != "zlib" {
		t.Errorf("Ordered = %v, want [zlib]", plan.Ordered)
	}
}

func TestResolve_WithDependencies(t *testing.T) {
	r := loadRegistry(t)
	res := New(r, noneInstalled)

	// gcc depends on binutils.
	plan, err := res.Resolve([]string{"gcc"})
	if err != nil {
		t.Fatalf("Resolve(gcc) error = %v", err)
	}

	// binutils must appear before gcc.
	order := plan.Ordered
	binutilsIdx := indexOf(order, "binutils")
	gccIdx := indexOf(order, "gcc")

	if binutilsIdx == -1 {
		t.Errorf("binutils not in ordered plan: %v", order)
	}
	if gccIdx == -1 {
		t.Errorf("gcc not in ordered plan: %v", order)
	}
	if binutilsIdx > gccIdx {
		t.Errorf("binutils (%d) should come before gcc (%d) in plan: %v", binutilsIdx, gccIdx, order)
	}
}

func TestResolve_AlreadyInstalled(t *testing.T) {
	r := loadRegistry(t)

	// Pretend binutils is already installed.
	res := New(r, func(name string) bool { return name == "binutils" })

	plan, err := res.Resolve([]string{"gcc"})
	if err != nil {
		t.Fatalf("Resolve(gcc) error = %v", err)
	}

	// gcc should be in Ordered; binutils in AlreadyInstalled.
	if !contains(plan.AlreadyInstalled, "binutils") {
		t.Errorf("binutils should be in AlreadyInstalled: %v", plan.AlreadyInstalled)
	}
	if !contains(plan.Ordered, "gcc") {
		t.Errorf("gcc should be in Ordered: %v", plan.Ordered)
	}
	if contains(plan.Ordered, "binutils") {
		t.Errorf("binutils should NOT be in Ordered: %v", plan.Ordered)
	}
}

func TestResolve_NothingToInstall(t *testing.T) {
	r := loadRegistry(t)
	res := New(r, allInstalled)

	plan, err := res.Resolve([]string{"zlib"})
	if err != nil {
		t.Fatalf("Resolve(zlib) with all installed: error = %v", err)
	}

	if len(plan.Ordered) != 0 {
		t.Errorf("Ordered should be empty when everything installed: %v", plan.Ordered)
	}
	if len(plan.AlreadyInstalled) == 0 {
		t.Error("AlreadyInstalled should not be empty")
	}
}

func TestResolve_UnknownPackage(t *testing.T) {
	r := loadRegistry(t)
	res := New(r, noneInstalled)

	_, err := res.Resolve([]string{"nonexistent-pkg-xyz"})
	if err == nil {
		t.Error("Resolve(unknown) expected error, got nil")
	}
}

func TestResolve_MultiplePackages(t *testing.T) {
	r := loadRegistry(t)
	res := New(r, noneInstalled)

	plan, err := res.Resolve([]string{"zlib", "sqlite", "lua"})
	if err != nil {
		t.Fatalf("Resolve(multiple) error = %v", err)
	}

	for _, name := range []string{"zlib", "sqlite", "lua"} {
		if !contains(plan.Ordered, name) {
			t.Errorf("%s not in plan: %v", name, plan.Ordered)
		}
	}
}

func TestResolve_DuplicatePackages(t *testing.T) {
	r := loadRegistry(t)
	res := New(r, noneInstalled)

	// Requesting zlib twice should produce it only once.
	plan, err := res.Resolve([]string{"zlib", "zlib"})
	if err != nil {
		t.Fatalf("Resolve(zlib, zlib) error = %v", err)
	}

	count := 0
	for _, n := range plan.Ordered {
		if n == "zlib" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("zlib appears %d times in plan, want 1: %v", count, plan.Ordered)
	}
}

func TestUpdateOrder(t *testing.T) {
	r := loadRegistry(t)
	res := New(r, noneInstalled)

	order, err := res.UpdateOrder([]string{"gcc"})
	if err != nil {
		t.Fatalf("UpdateOrder(gcc) error = %v", err)
	}
	if len(order) == 0 {
		t.Error("UpdateOrder returned empty order")
	}
	// gcc must be present.
	if !contains(order, "gcc") {
		t.Errorf("gcc not in update order: %v", order)
	}
}

func TestResolveRemove(t *testing.T) {
	r := loadRegistry(t)
	res := New(r, func(name string) bool {
		return name == "gcc" || name == "binutils"
	})

	toRemove, warnings, err := res.ResolveRemove([]string{"binutils"}, func(name string) bool {
		return name == "gcc" || name == "binutils"
	})
	if err != nil {
		t.Fatalf("ResolveRemove error = %v", err)
	}

	if !contains(toRemove, "binutils") {
		t.Errorf("binutils not in toRemove: %v", toRemove)
	}

	// gcc depends on binutils, so we should get a warning.
	if len(warnings) == 0 {
		t.Error("expected warning about gcc depending on binutils, got none")
	}
}

// indexOf returns the position of s in ss, or -1.
func indexOf(ss []string, s string) int {
	for i, v := range ss {
		if v == s {
			return i
		}
	}
	return -1
}
