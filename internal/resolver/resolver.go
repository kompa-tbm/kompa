// Package resolver implements dependency graph resolution for Kompa packages.
package resolver

import (
	"fmt"
	"strings"

	"github.com/kompa-tbm/kompa/internal/packages"
)

// InstallPlan is the ordered list of packages to install to satisfy a request.
type InstallPlan struct {
	// Ordered is the topologically sorted install order (dependencies first).
	Ordered []string
	// AlreadyInstalled lists packages that are already present and will be skipped.
	AlreadyInstalled []string
}

// IsInstalled is a function type that checks whether a package is currently installed.
type IsInstalled func(name string) bool

// Resolver resolves package dependency graphs.
type Resolver struct {
	registry    *packages.Registry
	isInstalled IsInstalled
}

// New returns a new Resolver backed by the given registry.
// isInstalled is called to determine whether a package is already installed.
func New(registry *packages.Registry, isInstalled IsInstalled) *Resolver {
	return &Resolver{
		registry:    registry,
		isInstalled: isInstalled,
	}
}

// Resolve returns the full ordered install plan for the requested package names.
// It performs a depth-first topological sort and detects circular dependencies.
func (r *Resolver) Resolve(names []string) (*InstallPlan, error) {
	// Validate all requested packages exist.
	for _, name := range names {
		if _, err := r.registry.Get(name); err != nil {
			return nil, err
		}
	}

	state := &resolveState{
		resolver:  r,
		visited:   make(map[string]visitState),
		order:     nil,
		installed: nil,
	}

	for _, name := range names {
		if err := state.visit(name, nil); err != nil {
			return nil, err
		}
	}

	return &InstallPlan{
		Ordered:          state.order,
		AlreadyInstalled: state.installed,
	}, nil
}

// ResolveRemove returns all packages that must be removed when the named packages
// are uninstalled. It checks whether anything else depends on each package and
// warns about broken dependents.
func (r *Resolver) ResolveRemove(names []string, isInstalled IsInstalled) ([]string, []string, error) {
	// Build reverse dependency map.
	reverseDeps := make(map[string][]string)
	for _, def := range r.registry.All() {
		for _, dep := range def.Dependencies {
			reverseDeps[dep] = append(reverseDeps[dep], def.Name)
		}
	}

	var toRemove []string
	var warnings []string

	for _, name := range names {
		// Check if anything installed depends on this package.
		var dependents []string
		for _, rdep := range reverseDeps[name] {
			if isInstalled(rdep) && !contains(names, rdep) {
				dependents = append(dependents, rdep)
			}
		}
		if len(dependents) > 0 {
			warnings = append(warnings, fmt.Sprintf(
				"%s is required by: %s (those packages may break)",
				name, strings.Join(dependents, ", "),
			))
		}
		toRemove = append(toRemove, name)
	}

	return toRemove, warnings, nil
}

// visitState tracks DFS state for cycle detection.
type visitState int

const (
	unvisited visitState = iota
	inProgress
	done
)

type resolveState struct {
	resolver  *Resolver
	visited   map[string]visitState
	order     []string
	installed []string
}

func (s *resolveState) visit(name string, stack []string) error {
	switch s.visited[name] {
	case done:
		return nil
	case inProgress:
		// Cycle detected — build a readable path.
		cycle := append(stack, name)
		return fmt.Errorf("circular dependency detected: %s", strings.Join(cycle, " -> "))
	}

	s.visited[name] = inProgress
	newStack := append(stack, name) //nolint:gocritic

	def, err := s.resolver.registry.Get(name)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", name, err)
	}

	// Visit all runtime dependencies first.
	for _, dep := range def.Dependencies {
		if _, err := s.resolver.registry.Get(dep); err != nil {
			return fmt.Errorf("package %s has unknown dependency %q: %w", name, dep, err)
		}
		if err := s.visit(dep, newStack); err != nil {
			return err
		}
	}

	s.visited[name] = done

	if s.resolver.isInstalled(name) {
		s.installed = append(s.installed, name)
	} else {
		s.order = append(s.order, name)
	}

	return nil
}

// UpdateOrder returns the topologically sorted list for updating the given packages.
// Unlike Resolve, it includes already-installed packages.
func (r *Resolver) UpdateOrder(names []string) ([]string, error) {
	for _, name := range names {
		if _, err := r.registry.Get(name); err != nil {
			return nil, err
		}
	}

	state := &updateState{
		resolver: r,
		visited:  make(map[string]visitState),
		order:    nil,
	}
	for _, name := range names {
		if err := state.visit(name, nil); err != nil {
			return nil, err
		}
	}
	return state.order, nil
}

type updateState struct {
	resolver *Resolver
	visited  map[string]visitState
	order    []string
}

func (s *updateState) visit(name string, stack []string) error {
	switch s.visited[name] {
	case done:
		return nil
	case inProgress:
		cycle := append(stack, name)
		return fmt.Errorf("circular dependency: %s", strings.Join(cycle, " -> "))
	}
	s.visited[name] = inProgress
	newStack := append(stack, name) //nolint:gocritic

	def, err := s.resolver.registry.Get(name)
	if err != nil {
		return err
	}
	for _, dep := range def.Dependencies {
		if err := s.visit(dep, newStack); err != nil {
			return err
		}
	}
	s.visited[name] = done
	s.order = append(s.order, name)
	return nil
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
