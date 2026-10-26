package catalogkit

import (
	"fmt"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/knowledge/catalog"
)

// Profile names an explicit set of root layers. Selection includes every
// transitive Requires dependency. It grants no authority: ResolveProfile uses
// the receiver's Policy unchanged, including grants for unselected layers.
type Profile struct {
	Name  string   `json:"name"`
	Roots []string `json:"roots"`
}

// ResolveProfile resolves a named selection from an already loaded Spec.
// The full loaded dependency graph must be valid, even outside the selection;
// structurally malformed unselected layers are not silently ignored. The profile name is
// descriptive and does not enter the effective bundle digest.
func ResolveProfile(spec Spec, policy Policy, profile Profile) (Bundle, error) {
	limits, err := effectiveLimits(policy.Limits)
	if err != nil {
		return Bundle{}, err
	}
	// Profile input is receiver-authored too. Bound collection allocation and
	// string scanning before validating names or constructing selection maps.
	if len(profile.Roots) > len(spec.Layers) || len(profile.Roots) > limits.MaxLayers {
		return Bundle{}, fmt.Errorf("catalogkit: profile root count exceeds loaded layers")
	}
	remaining := limits.MaxBytes
	consume := func(text string) bool {
		if len(text) >= remaining {
			return false
		}
		remaining -= len(text) + 1
		return true
	}
	if !consume(profile.Name) {
		return Bundle{}, fmt.Errorf("catalogkit: profile byte limit exceeded")
	}
	for _, root := range profile.Roots {
		if !consume(root) {
			return Bundle{}, fmt.Errorf("catalogkit: profile byte limit exceeded")
		}
	}
	if err := catalog.ValidateSlug(profile.Name); err != nil {
		return Bundle{}, fmt.Errorf("catalogkit: invalid profile name: %w", err)
	}
	if len(profile.Roots) == 0 {
		return Bundle{}, fmt.Errorf("catalogkit: profile %s has no roots", profile.Name)
	}
	if _, err := validateLoadedSpec(spec, policy, limits); err != nil {
		return Bundle{}, err
	}
	order, layers, ancestors, err := resolveGraph(spec.Layers)
	if err != nil {
		return Bundle{}, err
	}
	selected := make(map[string]bool, len(profile.Roots))
	seen := make(map[string]bool, len(profile.Roots))
	for _, root := range profile.Roots {
		if seen[root] {
			return Bundle{}, fmt.Errorf("catalogkit: duplicate profile root %s", root)
		}
		seen[root] = true
		if _, ok := layers[root]; !ok {
			return Bundle{}, fmt.Errorf("catalogkit: unknown profile root %s", root)
		}
		selected[root] = true
		for dependency := range ancestors[root] {
			selected[dependency] = true
		}
	}
	chosen := Spec{Version: spec.Version, Layers: make([]Layer, 0, len(selected))}
	for _, name := range order {
		if selected[name] {
			chosen.Layers = append(chosen.Layers, layers[name])
		}
	}
	return Resolve(chosen, policy)
}
