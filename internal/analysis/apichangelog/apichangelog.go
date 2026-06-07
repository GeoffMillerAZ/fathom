// Package apichangelog produces a human-readable API change log by comparing
// two snapshots of exported symbols via apisurface.Diff.
//
// All public functions are pure, deterministic, and safe for concurrent use.
package apichangelog

import (
	"fmt"
	"sort"
	"strings"

	"github.com/geoffmilleraz/sextant/internal/analysis/apisurface"
	"github.com/geoffmilleraz/sextant/internal/core"
)

// Entry represents one API change.
type Entry struct {
	// Kind is one of "added", "removed", or "changed".
	Kind string
	// Symbol is the stable symbol key ("<Package>.<Name>" or just "<Name>").
	Symbol string
	// Detail provides additional human-readable context.
	// For "changed" entries this is the new Signature; otherwise it is empty.
	Detail string
}

// kindOrder assigns a stable sort priority to each Kind value.
var kindOrder = map[string]int{
	"added":   0,
	"changed": 1,
	"removed": 2,
}

// Changelog computes the list of API changes between the old and new symbol
// sets.  It calls apisurface.Surface on each input (so only exported Go API
// symbols are considered) and then delegates to apisurface.Diff.
//
// The returned slice is sorted by (Kind priority, Symbol) for determinism:
//   - added   (0) before changed (1) before removed (2)
//   - within the same Kind, ascending alphabetical Symbol order
func Changelog(old, new []core.SymbolNode) []Entry {
	oldSurface := apisurface.Surface(old)
	newSurface := apisurface.Surface(new)

	// Build a map of new symbol key → Signature for "changed" Detail.
	newSigs := make(map[string]string, len(newSurface))
	for _, n := range newSurface {
		newSigs[symbolKey(n)] = n.Signature
	}

	changes := apisurface.Diff(oldSurface, newSurface)

	entries := make([]Entry, 0, len(changes.Added)+len(changes.Removed)+len(changes.Changed))

	for _, sym := range changes.Added {
		entries = append(entries, Entry{Kind: "added", Symbol: sym})
	}
	for _, sym := range changes.Removed {
		entries = append(entries, Entry{Kind: "removed", Symbol: sym})
	}
	for _, sym := range changes.Changed {
		entries = append(entries, Entry{Kind: "changed", Symbol: sym, Detail: newSigs[sym]})
	}

	sort.Slice(entries, func(i, j int) bool {
		pi, pj := kindOrder[entries[i].Kind], kindOrder[entries[j].Kind]
		if pi != pj {
			return pi < pj
		}
		return entries[i].Symbol < entries[j].Symbol
	})
	return entries
}

// symbolKey mirrors apisurface's internal key derivation so that Detail can be
// looked up from the new surface by key.
func symbolKey(n core.SymbolNode) string {
	if n.Package == "" {
		return n.Name
	}
	return n.Package + "." + n.Name
}

// Format renders a slice of Entry values as a Markdown change-log string.
//
// The output is a level-2 section heading ("## API Changes") followed by a
// bullet list.  Each bullet looks like:
//
//   - **added** `pkg.Symbol`
//   - **removed** `pkg.Symbol`
//   - **changed** `pkg.Symbol` — new signature: `func Foo(x int) error`
//
// An empty entries slice produces only the heading (no bullets).
func Format(entries []Entry) string {
	var b strings.Builder
	b.WriteString("## API Changes\n")
	for _, e := range entries {
		switch e.Kind {
		case "changed":
			fmt.Fprintf(&b, "- **%s** `%s` — new signature: `%s`\n", e.Kind, e.Symbol, e.Detail)
		default:
			fmt.Fprintf(&b, "- **%s** `%s`\n", e.Kind, e.Symbol)
		}
	}
	return b.String()
}
