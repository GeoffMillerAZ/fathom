// Package concern implements concern-scoped orchestration helpers.
package concern

import (
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// PropagateConcern walks the call graph backward from seed, collecting symbol
// names whose Package+Name contains concern (case-insensitive substring match)
// or which are reachable callers of such symbols.
//
// The traversal is breadth-first and deterministic: each tier of callers is
// sorted before being enqueued.  Only symbols reachable from seed (inclusive)
// are considered.  The returned slice is sorted and deduplicated.
func PropagateConcern(symbols []core.SymbolNode, concern, seed string) []string {
	if len(symbols) == 0 || seed == "" {
		return nil
	}

	// Build a reverse call graph: callee key → set of caller names.
	// A symbol's "key" is Package.Name.
	key := func(s core.SymbolNode) string { return s.Package + "." + s.Name }

	// Index symbols by key.
	byKey := make(map[string]core.SymbolNode, len(symbols))
	for _, sym := range symbols {
		byKey[key(sym)] = sym
	}

	// Build caller index: callee key → caller keys.
	callerOf := make(map[string][]string)
	for _, sym := range symbols {
		k := key(sym)
		for _, callee := range sym.Calls {
			callerOf[callee] = append(callerOf[callee], k)
		}
	}

	lower := strings.ToLower(concern)

	// relevant returns true if the symbol's identity contains the concern token.
	relevant := func(s core.SymbolNode) bool {
		combined := strings.ToLower(s.Package + "." + s.Name)
		return strings.Contains(combined, lower)
	}

	collected := make(map[string]struct{})

	// Start BFS from seed; include seed if it is relevant.
	visited := make(map[string]struct{})
	queue := []string{seed}
	visited[seed] = struct{}{}

	for len(queue) > 0 {
		// Sort current queue tier for determinism.
		sort.Strings(queue)
		next := queue
		queue = nil
		for _, cur := range next {
			sym, ok := byKey[cur]
			if ok && relevant(sym) {
				collected[cur] = struct{}{}
			}
			// Enqueue callers of cur.
			callers := callerOf[cur]
			sort.Strings(callers)
			for _, caller := range callers {
				if _, seen := visited[caller]; !seen {
					visited[caller] = struct{}{}
					queue = append(queue, caller)
				}
			}
		}
	}

	if len(collected) == 0 {
		return nil
	}

	out := make([]string, 0, len(collected))
	for k := range collected {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// EvidenceSummary counts observations by EvidenceTier and returns a map of
// tier → count.  The map is always non-nil (but may be empty).
func EvidenceSummary(obs []core.Observation) map[core.EvidenceTier]int {
	result := make(map[core.EvidenceTier]int)
	for _, o := range obs {
		result[o.EvidenceTier]++
	}
	return result
}
