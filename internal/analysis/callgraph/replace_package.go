package callgraph

// replace_package.go — SU-4 (docs/plans/substrate.md §A, ADR-041 decision
// 3): a package-granular alternative to Build for the two callers that
// only ever change ONE package's worth of data — internal/fusion/
// typedcalls.go (typed edges replaced per Go package, ADR-021 §3) and
// internal/fusion.ReingestFile (SU-5: one file's observations/syntax edges
// replaced, typed edges left alone). Both used to force a full Build of
// the whole service's graph (the ~9.7s/10.6k-symbol cost BP-28 exists to
// amortize across QUERIES, never across an INGEST-time edit) just to
// reflect one package's worth of change.
//
// SCOPE (a deliberate, documented approximation): ReplacePackage correctly
// updates pkg's own nodes, pkg's own outgoing edges (re-resolved against
// the patched graph), and strips every dangling edge that pointed at a
// removed pkg symbol. What it does NOT do is re-resolve any OTHER
// package's Calls against the new picture — a cross-package bare-name
// call that only becomes resolvable (or ambiguous) because of THIS edit
// stays exactly as it was, until the next full ingest (Build) makes it
// exact again. This is the SAME bounded-staleness contract ADR-041
// decision 5 already accepts for typed edges ("stay package-granular
// until the next ingest") — never a WRONG edge, only a possibly-missed one
// for a narrow class of calls, which the resolve() call order below
// mostly avoids anyway: an exact-key match, a same-package qualified
// match, and Go's own same-package bare-name binding (the dominant case
// for ordinary intra-package calls) all resolve directly off g.nodes with
// no ambiguity index involved at all.
import (
	"sort"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// ReplacePackage returns a NEW *Graph with every node currently in package
// pkg replaced by newNodes, pkg's own outgoing edges re-resolved, and —
// when typedEdges is non-nil — pkg's typed edges replaced too (nil means
// "leave typed edges alone", SU-5's ReingestFile case). g itself is never
// mutated: every other holder of g (the cache may still be serving it to a
// concurrent reader) keeps seeing the pre-patch graph, exactly the
// "*Graph handed out by Graph() is READ-ONLY for the caller" contract the
// cache's own doc comment already promises.
func (g *Graph) ReplacePackage(pkg string, newNodes []core.SymbolNode, typedEdges []core.Edge) *Graph {
	ng := &Graph{
		nodes:        cloneNodes(g.nodes),
		callees:      cloneAdjacency(g.callees),
		callers:      cloneAdjacency(g.callers),
		byName:       cloneStrMap(g.byName),
		byPkgName:    cloneStrMap(g.byPkgName),
		byDirName:    cloneStrMap(g.byDirName),
		byNameAll:    cloneAdjacency(g.byNameAll),
		bySuffix:     cloneSuffixMap(g.bySuffix),
		byDotSuffix:  cloneSuffixMap(g.byDotSuffix),
		edgeKind:     cloneEdgeKindMap(g.edgeKind),
		dispatchOut:  cloneIntMap(g.dispatchOut),
		instantiates: cloneAdjacency(g.instantiates),
		typedFiles:   cloneBoolMap(g.typedFiles),
		typedDirs:    cloneBoolMap(g.typedDirs),
	}

	// newNodes == nil means "this package's symbols are unchanged" (SU-4's
	// typedcalls.go wiring: only typed edges are being replaced) — default
	// it to pkg's CURRENT node set so the edge-recompute below reproduces
	// the exact same syntax edges it already has (a no-op for them) while
	// still giving typedEdges somewhere real to land. This is also what
	// makes a "surviving key" (same identity before and after) the common
	// case rather than the exception: see the newKeySet check below.
	if newNodes == nil {
		for _, n := range ng.nodes {
			if n.Package == pkg {
				newNodes = append(newNodes, n)
			}
		}
	}
	newKeySet := make(map[string]bool, len(newNodes))
	for _, n := range newNodes {
		newKeySet[n.Key()] = true
	}

	var oldKeys []string
	for key, n := range ng.nodes {
		if n.Package == pkg {
			oldKeys = append(oldKeys, key)
		}
	}
	sort.Strings(oldKeys) // deterministic iteration below

	touchedNames := map[string]bool{}

	// keptTyped is pkg's current typed out-edges, read before they are
	// stripped below: with typedEdges nil ("leave typed edges alone",
	// SU-5's ReingestFile) they are added back after the syntax edges are
	// re-derived, so the patched graph answers as a fresh BuildBoth over
	// the same typed edges would (wave lg), rather than losing the
	// package's typed edges until the next ingest.
	var keptTyped []core.Edge
	if typedEdges == nil {
		for _, k := range oldKeys {
			n := ng.nodes[k]
			for _, callee := range ng.callees[k] {
				if kind, ok := ng.edgeKind[k+"\x00"+callee]; ok {
					keptTyped = append(keptTyped, core.Edge{FromService: n.Service, FromFile: n.FilePath, FromSymbol: k, ToSymbol: callee, Kind: kind})
				}
			}
		}
	}

	for _, k := range oldKeys {
		n := ng.nodes[k]
		surviving := newKeySet[k] // the same key reappears in newNodes

		// Strip this old node's own outgoing edges (both syntax and typed
		// — they share the same adjacency maps) so they can be re-derived
		// below from the (possibly changed) Calls list.
		for _, callee := range ng.callees[k] {
			ng.callers[callee] = removeOne(ng.callers[callee], k)
			delete(ng.edgeKind, k+"\x00"+callee)
		}
		delete(ng.callees, k)
		delete(ng.dispatchOut, k)
		if typedEdges != nil {
			// Its instantiation facts are typed edges too: replaced with
			// the package's new ones below.
			delete(ng.instantiates, k)
		}

		if surviving {
			// This key is coming right back (its own node data may have
			// changed, but its IDENTITY has not) — every OTHER package's
			// edges INTO it are still valid and must be left alone. Only
			// the node map entry gets refreshed below (from newNodes).
			continue
		}

		// Genuinely removed: strip edges FROM other (untouched) nodes
		// INTO this key — otherwise they would dangle, pointing at a
		// symbol no longer in ng.nodes.
		for _, caller := range ng.callers[k] {
			ng.callees[caller] = removeOne(ng.callees[caller], k)
			delete(ng.edgeKind, caller+"\x00"+k)
		}
		delete(ng.callers, k)

		delete(ng.nodes, k)
		touchedNames[n.Name] = true
		ng.byNameAll[n.Name] = removeOne(ng.byNameAll[n.Name], k)
		if len(ng.byNameAll[n.Name]) == 0 {
			delete(ng.byNameAll, n.Name)
		}
		if n.Package != "" {
			// Only nodes with Package == pkg can ever contribute to this
			// composite key (the key string embeds the package), so
			// deleting it here is always correct, never a false negative
			// for some other package's node.
			delete(ng.byPkgName, n.Package+"."+n.Name)
		}
		if dn := dirName(n); dn != "" && ng.byDirName[dn] == k {
			delete(ng.byDirName, dn)
		}
		removeSuffixKey(ng.bySuffix, k)
		removeSuffixKey(ng.byDotSuffix, k)
	}

	for _, n := range newNodes {
		ng.nodes[n.Key()] = n
	}

	// byPkgName for pkg's own new nodes: safe to compute purely from
	// newNodes (no outside package can share a "pkg.Name" composite key —
	// see the package doc comment).
	pkgNameCounts := map[string]int{}
	for _, n := range newNodes {
		if n.Package == pkg {
			pkgNameCounts[n.Package+"."+n.Name]++
		}
	}
	for _, n := range newNodes {
		touchedNames[n.Name] = true
		ng.byNameAll[n.Name] = appendSortedUnique(ng.byNameAll[n.Name], n.Key())
		if n.Package == pkg && pkgNameCounts[n.Package+"."+n.Name] == 1 {
			ng.byPkgName[n.Package+"."+n.Name] = n.Key()
		}
		// byDirName is deliberately NOT populated for new nodes: unlike
		// byPkgName, a directory base name is not provably scoped to pkg
		// alone (see the package doc comment) — leaving it absent is
		// always safe, claiming it without that guarantee would not be.
	}

	// byName is global ambiguity — recompute it, but ONLY for the names
	// this call actually touched, using byNameAll (which already holds
	// every node sharing that name, in or out of pkg) as the true count.
	for name := range touchedNames {
		keys := ng.byNameAll[name]
		if len(keys) == 1 {
			ng.byName[name] = keys[0]
		} else {
			delete(ng.byName, name)
		}
	}

	// Resolve pkg's own new outgoing edges against the patched graph.
	for _, n := range newNodes {
		callerKey := n.Key()
		seen := make(map[string]struct{})
		for _, raw := range n.Calls {
			calleeKey := ng.resolve(n, raw)
			if calleeKey == "" {
				continue
			}
			if _, dup := seen[calleeKey]; dup {
				continue
			}
			seen[calleeKey] = struct{}{}
			ng.callees[callerKey] = append(ng.callees[callerKey], calleeKey)
			ng.callers[calleeKey] = append(ng.callers[calleeKey], callerKey)
		}
		sort.Strings(ng.callees[callerKey])
	}
	// Re-sort every callers[] list a new outgoing edge above landed in.
	for _, n := range newNodes {
		for _, callee := range ng.callees[n.Key()] {
			sort.Strings(ng.callers[callee])
		}
	}

	if typedEdges == nil {
		typedEdges = keptTyped
	}
	AddTypedEdges(ng, typedEdges)
	// AddTypedEdges drops a typed file's syntax-only method guesses (E);
	// with no typed edge to add it returns first, so pkg's re-derived edges
	// are checked here too.
	newKeys := make([]string, 0, len(newNodes))
	for _, n := range newNodes {
		newKeys = append(newKeys, n.Key())
	}
	ng.dropTypedFileGuesses(newKeys)

	return ng
}

func cloneNodes(m map[string]core.SymbolNode) map[string]core.SymbolNode {
	out := make(map[string]core.SymbolNode, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func cloneStrMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func cloneBoolMap(m map[string]bool) map[string]bool {
	if m == nil {
		return nil
	}
	out := make(map[string]bool, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func cloneIntMap(m map[string]int) map[string]int {
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func cloneEdgeKindMap(m map[string]core.EdgeKind) map[string]core.EdgeKind {
	if m == nil {
		return nil
	}
	out := make(map[string]core.EdgeKind, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// cloneAdjacency deep-clones a map[string][]string — the slice values, not
// just the map, since a later append into a shared slice's backing array
// would corrupt the ORIGINAL graph's data (classic aliasing bug) even
// though the map header itself was copied.
func cloneAdjacency(m map[string][]string) map[string][]string {
	out := make(map[string][]string, len(m))
	for k, v := range m {
		if v == nil {
			out[k] = nil
			continue
		}
		cp := make([]string, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

func cloneSuffixMap(m map[string]suffixCandidate) map[string]suffixCandidate {
	if m == nil {
		return nil
	}
	out := make(map[string]suffixCandidate, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// removeOne returns ss with the first occurrence of want removed (order of
// remaining elements preserved; the caller re-sorts when it cares).
func removeOne(ss []string, want string) []string {
	for i, s := range ss {
		if s == want {
			return append(ss[:i:i], ss[i+1:]...)
		}
	}
	return ss
}

// appendSortedUnique appends want to ss (skipping a duplicate) and
// re-sorts — byNameAll's own invariant (Build sorts it once at the end;
// this keeps that invariant after an incremental insert).
func appendSortedUnique(ss []string, want string) []string {
	for _, s := range ss {
		if s == want {
			return ss
		}
	}
	ss = append(ss, want)
	sort.Strings(ss)
	return ss
}

// removeSuffixKey deletes every entry in m whose sole recorded candidate
// (count == 1) is key — see buildSuffixIndex's own doc comment for why a
// count > 1 entry needs no change: its key is already "" (ambiguous,
// answers nothing), so it can never dangle regardless of which of the 2+
// original contributors this call removed.
func removeSuffixKey(m map[string]suffixCandidate, key string) {
	for i := 1; i < len(key); i++ {
		c := key[i-1]
		if c != '.' && c != '/' {
			continue
		}
		suf := key[i:]
		if cand, ok := m[suf]; ok && cand.count == 1 && cand.key == key {
			delete(m, suf)
		}
	}
}
