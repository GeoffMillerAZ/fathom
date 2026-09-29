package archconformance

import (
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// typePackages maps a Go TYPE's or INTERFACE's own Name to its declaring
// package — used to resolve a METHOD node's real package, since the Go
// extractor overrides SymbolNode.Package to the receiver type name for
// methods (internal/extractor goextract.go funcNode: "pkg = recv"), losing
// the declaring package on that node. A type/interface node never gets this
// override (genDeclNodes keeps Package == the file's declared package
// name), so it is always the correct source of truth.
func typePackages(nodes []core.SymbolNode) map[string]string {
	m := make(map[string]string, len(nodes))
	for _, n := range nodes {
		if (n.Kind == core.KindGoType || n.Kind == core.KindGoInterface) && n.Name != "" && n.Package != "" {
			m[n.Name] = n.Package
		}
	}
	return m
}

// declPkg resolves n's declaring Go package: for a method, the receiver
// type's OWN package via typePkg (falling back to n.Package, the receiver
// type name, when the receiver type itself wasn't indexed — e.g. an
// embedded/external receiver); for anything else, n.Package is already the
// real declaring package, unchanged.
func declPkg(n core.SymbolNode, typePkg map[string]string) string {
	if n.Kind == core.KindGoMethod {
		if pkg, ok := typePkg[n.Package]; ok {
			return pkg
		}
	}
	return n.Package
}

// bareName strips a method's "Receiver." prefix from n.Name (the Go
// extractor names a method node "Receiver.MethodName") so the rebuilt edge
// key is "declPkg.MethodName" — one dot, what Check's pkgOf "everything
// before the LAST dot" rule expects — never "declPkg.Receiver.MethodName".
func bareName(n core.SymbolNode) string {
	if n.Kind == core.KindGoMethod {
		if rest, ok := strings.CutPrefix(n.Name, n.Package+"."); ok {
			return rest
		}
	}
	return n.Name
}

// symbolKey renders n's normalized (declaring-package, bare-name) edge
// endpoint.
func symbolKey(n core.SymbolNode, typePkg map[string]string) string {
	return declPkg(n, typePkg) + "." + bareName(n)
}

// edgeResolver resolves a raw Calls() entry to the node it targets, so
// EdgesFromNodes can normalize the TARGET side too (a method call recorded
// as "h.RunOnce" — a receiver VARIABLE, unresolvable to a package by syntax
// alone; see internal/extractor goextract.go collectCalls — still resolves
// when "RunOnce" uniquely identifies one node in the service). Mirrors
// callgraph.Graph.resolve's matching order (internal/analysis/callgraph/
// graph.go): exact canonical key, then a bare name that is UNIQUE across the
// node set, then a dotted suffix that is UNIQUE — never an ambiguous
// first-match guess.
type edgeResolver struct {
	byKey  map[string]core.SymbolNode
	byName map[string]core.SymbolNode
	nodes  []core.SymbolNode
	// bySuffix indexes every dot-led suffix of every node key (".Name",
	// ".Recv.Name", …) to the node carrying it, or to nothing when two
	// nodes share it. Built on the first suffix lookup (CX-20: the drift
	// radar runs EdgesFromNodes at every ingest of a repository with a
	// rule pack, where the linear per-callee scan it replaces cost
	// O(edges×nodes)).
	bySuffix map[string]suffixEntry
}

// suffixEntry is one dot-led key suffix's owner and how many nodes carry it.
type suffixEntry struct {
	node  core.SymbolNode
	count int
}

func newEdgeResolver(nodes []core.SymbolNode) *edgeResolver {
	r := &edgeResolver{
		byKey:  make(map[string]core.SymbolNode, len(nodes)),
		byName: make(map[string]core.SymbolNode, len(nodes)),
		nodes:  nodes,
	}
	nameCounts := make(map[string]int, len(nodes))
	for _, n := range nodes {
		r.byKey[n.Key()] = n
		nameCounts[n.Name]++
	}
	for _, n := range nodes {
		if nameCounts[n.Name] == 1 {
			r.byName[n.Name] = n
		}
	}
	return r
}

// resolve returns the node raw targets, and whether it was found. Ambiguous
// suffix matches (more than one node key ends with the same suffix) resolve
// to nothing — genuine ambiguity is never first-match-wins.
func (r *edgeResolver) resolve(raw string) (core.SymbolNode, bool) {
	if n, ok := r.byKey[raw]; ok {
		return n, true
	}
	if n, ok := r.byName[raw]; ok {
		return n, true
	}
	if n, ok := r.uniqueSuffix("." + raw); ok {
		return n, true
	}
	if i := strings.LastIndex(raw, "."); i > 0 && i < len(raw)-1 {
		if n, ok := r.uniqueSuffix(raw[i:]); ok {
			return n, true
		}
	}
	return core.SymbolNode{}, false
}

// uniqueSuffix answers "which one node's key ends with this dot-led
// suffix" from a suffix index built once per resolver. It used to scan every
// node per unresolved callee — O(edges×nodes), measured ~33s for a
// two-service fleet store (critic pass, Wave 3a) — which was fine per
// arch_conformance call but not at every ingest (CX-20).
func (r *edgeResolver) uniqueSuffix(suffix string) (core.SymbolNode, bool) {
	if !strings.HasPrefix(suffix, ".") {
		// Every caller passes a dot-led suffix; anything else keeps the
		// original linear scan's exact semantics.
		var match core.SymbolNode
		found := false
		for _, n := range r.nodes {
			if strings.HasSuffix(n.Key(), suffix) {
				if found {
					return core.SymbolNode{}, false // ambiguous
				}
				match = n
				found = true
			}
		}
		return match, found
	}
	if r.bySuffix == nil {
		r.bySuffix = make(map[string]suffixEntry, 2*len(r.nodes))
		for _, n := range r.nodes {
			key := n.Key()
			for i := 0; i < len(key); i++ {
				if key[i] != '.' {
					continue
				}
				e := r.bySuffix[key[i:]]
				if e.count == 0 {
					e.node = n
				}
				e.count++
				r.bySuffix[key[i:]] = e
			}
		}
	}
	// A key ending in the dot-led suffix carries it at exactly one dot, so
	// the index answers the scan's question: one owner, or ambiguous.
	e := r.bySuffix[suffix]
	if e.count != 1 {
		return core.SymbolNode{}, false
	}
	return e.node, true
}

// KeyIndex maps each node's NORMALIZED symbol key — the same
// declaring-package "declPkg.BareName" form EdgesFromNodes renders edge
// endpoints (and therefore Violation.From/.To) in — back to the node itself,
// so a Violation consumer can recover the violating symbol's real file and
// line (review_diff cites them; a bare-path citation must never be persisted
// on a claim — docs/plans/review-verb.md C2-1). When two nodes normalize to
// the same key (rare: same package + same bare name), the first in nodes
// order wins deterministically.
func KeyIndex(nodes []core.SymbolNode) map[string]core.SymbolNode {
	typePkg := typePackages(nodes)
	m := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		key := symbolKey(n, typePkg)
		if _, ok := m[key]; !ok {
			m[key] = n
		}
	}
	return m
}

// EdgesFromNodes builds a service's checked call-edge set from nodes' raw
// Calls lists, with BOTH endpoints normalized to declaring-package symbol
// keys (P1-4). Before this normalization, a method-origin edge's FromSymbol
// carried the RECEIVER TYPE as its "package" (e.g. "Handler.Handler.Serve",
// whose pkgOf-derived prefix is "Handler.Handler" — never a real package
// name any rule names), and a method-target edge's raw callee (often a
// receiver-VARIABLE-qualified form like "h.Forbidden", never
// package-qualified at all) fared no better — so package-named rules
// silently never matched either side of a method call, guarding only
// function-to-function edges. Every callee that DOESN'T resolve to a node in
// this service (external/stdlib calls, or genuine ambiguity) keeps its raw
// form exactly as before — no regression for the edges that already
// couldn't be resolved.
//
// Shared by the arch_conformance MCP tool and the review_diff composed verb
// (docs/plans/review-verb.md C1-1) — one edge-normalization implementation,
// never two that could drift.
func EdgesFromNodes(nodes []core.SymbolNode) []core.Edge {
	typePkg := typePackages(nodes)
	resolver := newEdgeResolver(nodes)
	var edges []core.Edge
	for _, n := range nodes {
		from := symbolKey(n, typePkg)
		for _, callee := range n.Calls {
			to := callee
			if target, ok := resolver.resolve(callee); ok {
				to = symbolKey(target, typePkg)
			}
			edges = append(edges, core.Edge{
				FromService: n.Service,
				FromFile:    n.FilePath,
				FromSymbol:  from,
				ToSymbol:    to,
			})
		}
	}
	return edges
}
