// Package apisurface computes the exported public API surface of a service
// from a []core.SymbolNode, and produces a deterministic diff between two
// API snapshots for break detection.
//
// All exported functions are pure, deterministic, and safe for concurrent use.
package apisurface

import (
	"sort"
	"strings"
	"unicode"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// goAPIKinds is the set of SourceKind values that can appear in a Go public API.
var goAPIKinds = map[core.SourceKind]bool{
	core.KindGoFunc:      true,
	core.KindGoMethod:    true,
	core.KindGoType:      true,
	core.KindGoInterface: true,
	core.KindGoConst:     true,
	core.KindGoVar:       true,
}

// isExported reports whether name is an exported Go identifier: non-empty and
// starts with an uppercase Unicode letter.
func isExported(name string) bool {
	if name == "" {
		return false
	}
	r := []rune(name)
	return unicode.IsUpper(r[0])
}

// symbolKey returns the stable lookup key for a SymbolNode: "<Package>.<Name>".
// When Package is empty the key is just Name.
func symbolKey(n core.SymbolNode) string {
	if n.Package == "" {
		return n.Name
	}
	return n.Package + "." + n.Name
}

// Surface returns the exported public API surface derived from nodes.
//
// A node is included when:
//   - its Kind is one of the Go API kinds (func, method, type, interface, const, var), and
//   - its Name starts with an uppercase Unicode letter (Go export convention).
//
// The returned slice is sorted by (Package, Name) for determinism. The input
// slice is never modified.
func Surface(nodes []core.SymbolNode) []core.SymbolNode {
	result := make([]core.SymbolNode, 0, len(nodes))
	for _, n := range nodes {
		if !goAPIKinds[n.Kind] {
			continue
		}
		// For methods the receiver is part of the name; we check the part after
		// the last dot (if any) — but in practice core.SymbolNode.Name holds just
		// the method/function identifier without the receiver prefix.
		baseName := n.Name
		if idx := strings.LastIndex(n.Name, "."); idx >= 0 {
			baseName = n.Name[idx+1:]
		}
		if !isExported(baseName) {
			continue
		}
		result = append(result, n)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Package != result[j].Package {
			return result[i].Package < result[j].Package
		}
		return result[i].Name < result[j].Name
	})
	return result
}

// Changes holds the categorised delta between two API snapshots.
// Each entry is a symbol key of the form "<Package>.<Name>" (or just "<Name>"
// when Package is empty). All slices are sorted for determinism.
type Changes struct {
	// Added contains keys present in the new snapshot but not the old — a
	// backward-compatible (minor) change.
	Added []string
	// Removed contains keys present in the old snapshot but not the new — a
	// breaking change.
	Removed []string
	// Changed contains keys present in both snapshots but with a different
	// Signature — also a breaking change.
	Changed []string
}

// Diff computes the Changes between an old and a new API surface (both
// produced by Surface or equivalent). It operates only on the exported entries
// supplied; callers should filter with Surface first if needed.
//
// The key for each symbol is "<Package>.<Name>" (Package may be empty).
// "Changed" means the key exists in both snapshots but the Signature field
// differs.  All output slices are sorted ascending for determinism.
func Diff(old, new []core.SymbolNode) Changes {
	oldMap := make(map[string]string, len(old)) // key → Signature
	for _, n := range old {
		oldMap[symbolKey(n)] = n.Signature
	}
	newMap := make(map[string]string, len(new)) // key → Signature
	for _, n := range new {
		newMap[symbolKey(n)] = n.Signature
	}

	var added, removed, changed []string

	for key, newSig := range newMap {
		if oldSig, exists := oldMap[key]; !exists {
			added = append(added, key)
		} else if oldSig != newSig {
			changed = append(changed, key)
		}
	}
	for key := range oldMap {
		if _, exists := newMap[key]; !exists {
			removed = append(removed, key)
		}
	}

	sort.Strings(added)
	sort.Strings(removed)
	sort.Strings(changed)

	return Changes{
		Added:   added,
		Removed: removed,
		Changed: changed,
	}
}
