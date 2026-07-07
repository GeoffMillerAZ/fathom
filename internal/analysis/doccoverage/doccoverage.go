// Package doccoverage computes documentation coverage over a set of symbols.
//
// The caller supplies a slice of core.SymbolNode (the exported symbols to
// check) and a map[string]bool indicating which symbol keys are documented.
// The key used for lookup is SymbolNode.Name (exact match).  The caller is
// responsible for building the documented map from whatever doc-extraction
// mechanism is appropriate for their codebase.
//
// Only exported symbols (first rune uppercase) are included in the report.
// Unexported symbols are silently skipped.
//
// All exported functions are pure, deterministic, and safe for concurrent use.
package doccoverage

import (
	"sort"
	"unicode"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// Report is the documentation coverage result.
type Report struct {
	// Covered holds the names of exported symbols that have documentation.
	Covered []string
	// Uncovered holds the names of exported symbols lacking documentation.
	Uncovered []string
	// Pct is the coverage percentage (0–100). NaN-safe: returns 100 if there
	// are no exported symbols.
	Pct float64
}

// Coverage returns the documentation coverage report for nodes, using the
// documented map to determine which symbols are documented.  A symbol key is
// its Name field.  Both Covered and Uncovered are sorted for determinism.
func Coverage(nodes []core.SymbolNode, documented map[string]bool) Report {
	var covered, uncovered []string

	seen := make(map[string]bool) // de-duplicate by name
	for _, n := range nodes {
		if !isExported(n.Name) {
			continue
		}
		if seen[n.Name] {
			continue
		}
		seen[n.Name] = true

		if documented[n.Name] {
			covered = append(covered, n.Name)
		} else {
			uncovered = append(uncovered, n.Name)
		}
	}

	sort.Strings(covered)
	sort.Strings(uncovered)

	total := len(covered) + len(uncovered)
	var pct float64
	if total == 0 {
		pct = 100.0
	} else {
		pct = float64(len(covered)) / float64(total) * 100.0
	}

	return Report{
		Covered:   covered,
		Uncovered: uncovered,
		Pct:       pct,
	}
}

// isExported reports whether name starts with an uppercase letter.
func isExported(name string) bool {
	for _, r := range name {
		return unicode.IsUpper(r)
	}
	return false
}
