package console

// index.go — OC-36 (vision 35 §3 "Index: the dialect packs at work"; design
// §3): what the index holds for one project — each language with the
// engine that extracted it and the tier it reached, the typed share of the
// call edges, the edge kinds, and the inventory keys. The same facts
// `fathom report evidence --service S --json` reports; the page computes
// them from the same primitives so the two cannot disagree.
//
// Law 3 governs every field: a project with no typed pass has no typed
// share (absent, never 0%); a corpus the daemon cannot read has no
// inventory (absent, never "0 routes"); languages are listed in name
// order and never ranked (report_evidence.go's own "never ranked" rule).

import (
	"fmt"
	"sort"
)

// IndexLanguage is one language the store's symbols are written in.
type IndexLanguage struct {
	Language string `json:"language"`
	Engine   string `json:"engine"`
	Tier     string `json:"tier"`
	Files    int    `json:"files"`
	Symbols  int    `json:"symbols"`
}

// IndexEdgeKind is one typed-edge kind's count.
type IndexEdgeKind struct {
	Kind string `json:"kind"`
	// Resolved: an edge of this kind counts toward the typed share (it
	// names its callee); unresolved edges are never used for proof.
	Resolved bool `json:"resolved"`
	Count    int  `json:"count"`
}

// IndexFacts is what the adapter read for one project.
type IndexFacts struct {
	Service string
	// Languages from the store's own symbols (file paths), with the engine
	// and tier already joined; LanguagesErr is set when the symbols could
	// not be read.
	Languages    []IndexLanguage
	LanguagesErr string
	// EdgeKinds are the typed-edge rows by kind; EdgesErr is set when the
	// typed edges could not be read. No rows and no error means no typed
	// pass is recorded for the project.
	EdgeKinds []IndexEdgeKind
	EdgesErr  string
	// Inventory is the inventory items by kind, from the corpus's content;
	// InventoryNote is why it was not computed (a corpus not readable from
	// this daemon), InventoryErr why it failed.
	Inventory     map[string]int
	InventoryNote string
	InventoryErr  string
	// Source names what was read ("the daemon's store <path>").
	Source string
}

// IndexInventoryKey is one inventory kind's count.
type IndexInventoryKey struct {
	Kind  string `json:"kind"`
	Count int    `json:"count"`
}

// IndexProfileModel is the index tab's projection.
type IndexProfileModel struct {
	Service     string          `json:"service"`
	Languages   []IndexLanguage `json:"languages"`
	LanguageSet Value[int]      `json:"language_count"`
	// Edges is every typed edge; TypedEdges the resolved ones; TypedShare
	// their ratio rounded to four places (report evidence's typed_share).
	Edges      Value[int]          `json:"edges"`
	TypedEdges Value[int]          `json:"typed_edges"`
	TypedShare Value[float64]      `json:"typed_share"`
	EdgeKinds  []IndexEdgeKind     `json:"edge_kinds"`
	Inventory  Value[int]          `json:"inventory_total"`
	Keys       []IndexInventoryKey `json:"inventory_keys"`
}

// edgeKindOrder is the fixed display order of the typed pass's kinds
// (internal/core EdgeKind); an unknown kind sorts after them by name.
var edgeKindOrder = map[string]int{"direct": 0, "iface-dispatch": 1, "func-value": 2, "file-resolved": 3, "unresolved": 4}

// IndexProfile projects one project's index facts. Every count carries its
// truth state: an unread source is an error naming it, a source with
// nothing to say is absent with the reason, and only a real count is a
// number.
func IndexProfile(f IndexFacts) IndexProfileModel {
	m := IndexProfileModel{Service: f.Service, Languages: []IndexLanguage{}, EdgeKinds: []IndexEdgeKind{}, Keys: []IndexInventoryKey{}}
	src := f.Source

	switch {
	case f.LanguagesErr != "":
		m.LanguageSet = Errored[int]("the store's symbols could not be read: " + f.LanguagesErr).WithSource(src)
	case len(f.Languages) == 0:
		m.LanguageSet = Absent[int]("no indexed symbol is in a language fathom names — nothing was extracted, or only files outside docs/languages.md").WithSource(src)
	default:
		m.Languages = append(m.Languages, f.Languages...)
		sort.Slice(m.Languages, func(i, j int) bool { return m.Languages[i].Language < m.Languages[j].Language })
		m.LanguageSet = Measured(len(m.Languages), src).WithDenominator("languages among the indexed symbols' files, in name order (never ranked)")
	}

	total, resolved := 0, 0
	for _, k := range f.EdgeKinds {
		total += k.Count
		if k.Resolved {
			resolved += k.Count
		}
	}
	switch {
	case f.EdgesErr != "":
		reason := "the typed edges could not be read: " + f.EdgesErr
		m.Edges, m.TypedEdges, m.TypedShare = Errored[int](reason), Errored[int](reason), Errored[float64](reason)
	case total == 0:
		reason := "no typed-edge pass is recorded for " + f.Service + " — the call graph is syntax-only here, so there is no typed share to report (never 0%)"
		m.Edges, m.TypedEdges, m.TypedShare = Absent[int](reason), Absent[int](reason), Absent[float64](reason)
	default:
		m.EdgeKinds = append(m.EdgeKinds, f.EdgeKinds...)
		sort.Slice(m.EdgeKinds, func(i, j int) bool {
			a, b := m.EdgeKinds[i], m.EdgeKinds[j]
			oa, okA := edgeKindOrder[a.Kind]
			ob, okB := edgeKindOrder[b.Kind]
			switch {
			case okA && okB:
				return oa < ob
			case okA != okB:
				return okA
			}
			return a.Kind < b.Kind
		})
		m.Edges = Measured(total, src).WithDenominator("typed call edges the typed pass recorded")
		m.TypedEdges = Measured(resolved, src).WithDenominator(fmt.Sprintf("of %s typed edges, the ones that name their callee", thousands(total)))
		share := float64(int(float64(resolved)/float64(total)*10000+0.5)) / 10000
		m.TypedShare = Measured(share, src).WithDenominator(fmt.Sprintf("%s resolved of %s typed edges", thousands(resolved), thousands(total)))
	}

	invSrc := "the corpus's files, read through the daemon (inventory.Extract)"
	switch {
	case f.InventoryErr != "":
		m.Inventory = Errored[int]("the inventory could not be computed: " + f.InventoryErr).WithSource(invSrc)
	case f.InventoryNote != "":
		m.Inventory = Absent[int](f.InventoryNote).WithSource(invSrc)
	default:
		n := 0
		for k, c := range f.Inventory {
			m.Keys = append(m.Keys, IndexInventoryKey{Kind: k, Count: c})
			n += c
		}
		sort.Slice(m.Keys, func(i, j int) bool { return m.Keys[i].Kind < m.Keys[j].Kind })
		m.Inventory = Measured(n, invSrc).WithDenominator(fmt.Sprintf("inventory items across %d kinds (api, env, route, json and the repo's own surface patterns)", len(m.Keys)))
	}
	return m
}
