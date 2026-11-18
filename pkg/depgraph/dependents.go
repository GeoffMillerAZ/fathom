package depgraph

import "sort"

// Dependents answers the question a rebuild asks: if these units changed,
// what else must be rebuilt?
//
// It returns the TRANSITIVE dependents of changed — every unit that reaches
// any of them through Deps, at any depth — as a sorted slice of unit names.
// The seeds themselves are excluded. The caller decides whether to
// rebuild them.
//
// Sorted, so two runs over one graph agree and a CI job can diff its blast
// radius against the last one. An unsorted answer here would make every
// rebuild plan look different from the last.
//
// CYCLE-SAFE by construction — the visited set is checked before descent,
// not after. A dependency cycle is a real thing to find in a declared
// graph, and a blast-radius query that hangs on one is worse than useless
// because it hangs in CI, where nobody is watching it.

func Dependents(units []Unit, changed []string) []string {
	if len(units) == 0 || len(changed) == 0 {
		return nil
	}

	// reverse[dep] = the units that declare dep. Built once: a query is a
	// walk, not a repeated scan, because a blast radius over a few hundred
	// images should not be quadratic.
	reverse := make(map[string][]string, len(units))
	known := make(map[string]struct{}, len(units))
	for _, u := range units {
		known[u.Name] = struct{}{}
		for _, d := range u.Deps {
			reverse[d] = append(reverse[d], u.Name)
		}
	}

	seeds := make(map[string]struct{}, len(changed))
	for _, c := range changed {
		seeds[c] = struct{}{}
	}

	visited := make(map[string]struct{})
	queue := append([]string(nil), changed...)
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, dependent := range reverse[cur] {
			if _, seen := visited[dependent]; seen {
				continue
			}
			visited[dependent] = struct{}{}
			queue = append(queue, dependent)
		}
	}

	var out []string
	for name := range visited {
		// A seed that is also a dependent of another seed stays out: the
		// caller asked what ELSE must be rebuilt.
		if _, isSeed := seeds[name]; isSeed {
			continue
		}
		out = append(out, name)
	}
	sort.Strings(out)
	// nil, not an empty slice, and consistently so — including the early
	// returns above. An API that answers "nothing" as nil sometimes and as
	// []string{} other times makes `== nil` and `len() == 0` disagree, which
	// is a trap for exactly one caller and always the wrong one.
	return out
}

// UnknownDeps reports every dependency named by a unit that no unit in the
// set provides, sorted and deduplicated.
//
// It exists because Dependents CANNOT tell you this: a dep pointing at
// nothing simply contributes no dependents, so an entire subtree can be
// silently missing from a blast radius and the answer still looks like an
// answer. That is under-keying by another name (the same failure Edge's own
// doc names), and the only safe posture is to surface it and let the caller
// refuse.
func UnknownDeps(units []Unit) []string {
	known := make(map[string]struct{}, len(units))
	for _, u := range units {
		known[u.Name] = struct{}{}
	}
	missing := make(map[string]struct{})
	for _, u := range units {
		for _, d := range u.Deps {
			if d == "" {
				continue
			}
			if _, ok := known[d]; !ok {
				missing[d] = struct{}{}
			}
		}
	}
	var out []string
	for d := range missing {
		out = append(out, d)
	}
	sort.Strings(out)
	// nil when nothing is missing — see Dependents on why this is not an
	// empty slice.
	return out
}
