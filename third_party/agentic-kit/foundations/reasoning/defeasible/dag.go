package defeasible

import "fmt"

// detectSupersessionCycle walks the global Supersedes DAG and returns an error
// if any cycle is found. Uses recursive DFS with grey/black colouring.
func detectSupersessionCycle(modifiers []Modifier) error {
	// Build adjacency: id → ids it supersedes.
	adj := make(map[string][]string, len(modifiers))
	for _, m := range modifiers {
		if len(m.Supersedes) > 0 {
			sup := make([]string, len(m.Supersedes))
			copy(sup, m.Supersedes)
			adj[m.ID] = sup
		}
	}

	const (
		white = 0 // unvisited
		grey  = 1 // in progress (ancestor on current DFS path)
		black = 2 // fully explored
	)
	colour := make(map[string]int, len(modifiers))

	var dfs func(id string) error
	dfs = func(id string) error {
		if colour[id] == grey {
			return fmt.Errorf("defeasible: cycle detected in Supersedes DAG involving modifier %q", id)
		}
		if colour[id] == black {
			return nil
		}
		colour[id] = grey
		for _, next := range adj[id] {
			if err := dfs(next); err != nil {
				return err
			}
		}
		colour[id] = black
		return nil
	}

	for _, m := range modifiers {
		if colour[m.ID] == white {
			if err := dfs(m.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// buildSupersededSet returns the set of modifier IDs that are transitively
// superseded by any ACTIVE modifier in the slice. Inactive modifiers
// (StatusInvalidated, StatusWaived, or not in activeStatuses) have no
// supersession power — their Supersedes lists are ignored. Any ID in the
// returned set should never win.
func buildSupersededSet(modifiers []Modifier, activeStatuses map[Status]bool) map[string]bool {
	// Only active modifiers may supersede others.
	var active []Modifier
	for _, m := range modifiers {
		if isActive(m, activeStatuses) {
			active = append(active, m)
		}
	}
	adj := buildSupersessionAdj(active)
	result := make(map[string]bool)
	for _, m := range active {
		markTransitive(m.ID, adj, result)
	}
	return result
}

// computeSupersededPerTarget returns, for each TargetKey, the set of active
// modifier IDs that were removed from contention by supersession. These are
// included in ShadowedModifierIDs on the resulting EffectiveProvision.
func computeSupersededPerTarget(modifiers []Modifier, activeStatuses map[Status]bool) map[string]map[string]bool {
	result := make(map[string]map[string]bool)

	// Collect active modifiers per target key.
	activeMods := make(map[string][]Modifier)
	for _, m := range modifiers {
		if isActive(m, activeStatuses) {
			activeMods[m.TargetKey] = append(activeMods[m.TargetKey], m)
		}
	}

	for key, mods := range activeMods {
		adj := buildSupersessionAdj(mods)

		activeIDs := make(map[string]bool, len(mods))
		for _, m := range mods {
			activeIDs[m.ID] = true
		}

		// Compute which IDs are transitively superseded within this target group.
		shadowed := make(map[string]bool)
		for _, m := range mods {
			markTransitive(m.ID, adj, shadowed)
		}

		// Intersect with activeIDs so we only report active modifiers as shadowed.
		perTarget := make(map[string]bool)
		for id := range shadowed {
			if activeIDs[id] {
				perTarget[id] = true
			}
		}
		if len(perTarget) > 0 {
			result[key] = perTarget
		}
	}
	return result
}

// buildSupersessionAdj builds an adjacency map (id → ids it supersedes) from
// a slice of modifiers, making defensive copies of each Supersedes slice.
func buildSupersessionAdj(modifiers []Modifier) map[string][]string {
	adj := make(map[string][]string, len(modifiers))
	for _, m := range modifiers {
		if len(m.Supersedes) > 0 {
			sup := make([]string, len(m.Supersedes))
			copy(sup, m.Supersedes)
			adj[m.ID] = sup
		}
	}
	return adj
}

// markTransitive recursively marks all IDs transitively superseded by the
// modifier identified by id. It does NOT mark id itself — only the IDs
// reachable through the adjacency graph.
func markTransitive(id string, adj map[string][]string, result map[string]bool) {
	for _, target := range adj[id] {
		if !result[target] {
			result[target] = true
			markTransitive(target, adj, result)
		}
	}
}
