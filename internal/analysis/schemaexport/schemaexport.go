// Package schemaexport serialises the sextant symbol graph to common interchange
// formats.
//
//   - ExportJSON encodes []core.SymbolNode as a deterministic, indented JSON
//     array.  Nodes are sorted by their canonical SymbolKey before encoding so
//     the output is identical across repeated calls regardless of input order.
//
//   - ExportDOT encodes []core.Edge as a Graphviz digraph.  Edges are sorted
//     (from, to) for determinism.
//
// Both functions are pure and safe for concurrent use.
package schemaexport

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// ExportJSON serialises nodes to a deterministic, indented JSON array.
//
// Nodes are sorted by their canonical SymbolKey ("service::pkg.Name") before
// encoding so the output is stable regardless of the input slice order.
// Returns an error only when json.Marshal fails (which should not happen for
// well-formed SymbolNode values).
func ExportJSON(nodes []core.SymbolNode) (string, error) {
	// Work on a shallow copy so we do not mutate the caller's slice.
	sorted := make([]core.SymbolNode, len(nodes))
	copy(sorted, nodes)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Key() < sorted[j].Key()
	})

	// Sort each node's Calls slice for determinism.
	for i := range sorted {
		if len(sorted[i].Calls) > 1 {
			calls := make([]string, len(sorted[i].Calls))
			copy(calls, sorted[i].Calls)
			sort.Strings(calls)
			sorted[i].Calls = calls
		}
	}

	data, err := json.MarshalIndent(sorted, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ExportDOT serialises edges to a Graphviz digraph in DOT language.
//
// Edge labels include the FromSymbol → ToSymbol names.  Edges are sorted by
// (FromSymbol, ToSymbol) for determinism.  The digraph is named "sextant".
func ExportDOT(edges []core.Edge) string {
	// Work on a copy and sort.
	sorted := make([]core.Edge, len(edges))
	copy(sorted, edges)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].FromSymbol != sorted[j].FromSymbol {
			return sorted[i].FromSymbol < sorted[j].FromSymbol
		}
		return sorted[i].ToSymbol < sorted[j].ToSymbol
	})

	var sb strings.Builder
	sb.WriteString("digraph sextant {\n")
	for _, e := range sorted {
		// DOT identifiers must be quoted when they contain special characters.
		sb.WriteString("  ")
		sb.WriteString(dotQuote(e.FromSymbol))
		sb.WriteString(" -> ")
		sb.WriteString(dotQuote(e.ToSymbol))
		sb.WriteString(";\n")
	}
	sb.WriteString("}\n")
	return sb.String()
}

// dotQuote wraps s in double quotes, escaping any embedded double quotes and
// backslashes, so it is a valid DOT quoted identifier.
func dotQuote(s string) string {
	// Fast path: simple identifiers (letters, digits, underscore, colon, dot).
	needsQuote := false
	for _, r := range s {
		if !isDotSafe(r) {
			needsQuote = true
			break
		}
	}
	if !needsQuote && s != "" {
		return s
	}
	// Escape backslash and double-quote, then wrap.
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

func isDotSafe(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') ||
		r == '_' || r == ':' || r == '.'
}
