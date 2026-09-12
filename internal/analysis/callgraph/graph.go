// Package callgraph builds a directed call graph from []core.SymbolNode and
// provides pure, deterministic impact analyses on top of it.
//
// All public analysis functions return sorted, deterministic output so results
// are safe to compare in tests and stable across repeated calls.
//
// Tolerant matching: callee entries in SymbolNode.Calls may be stored as a
// full SymbolKey ("svc::pkg.Name"), a bare function name ("Name"), a
// qualified selector ("pkg.Name", "seg.Name", "<var>.Method"), or a
// constructor-scoped method reference ("<import-path>#.Method") — the forms
// documented on GoExtractor.Extract.  Build resolves each callee to the
// correct node when the match is unambiguous; genuine ambiguity resolves to
// nothing, never first-match-wins.
package callgraph

import (
	"path"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// Graph is a directed call graph.  Nodes are identified by their canonical
// SymbolKey; edges point from caller to callee.
//
// The zero value is not useful; always obtain a Graph via Build.
type Graph struct {
	// nodes maps canonical key -> SymbolNode (for membership tests).
	nodes map[string]core.SymbolNode

	// callees maps caller key -> sorted slice of callee keys.
	callees map[string][]string

	// callers maps callee key -> sorted slice of caller keys (reverse index).
	callers map[string][]string

	// byName maps bare function name -> canonical key, used for tolerant
	// matching.  Only populated when the bare name is unambiguous (exactly
	// one node with that Name).
	byName map[string]string

	// byPkgName maps "package.Name" -> canonical key. It resolves cross-package
	// selector calls ("pkg.Name", e.g. an external "pkg_test" package calling
	// "pkg.Name") precisely: the left side must be a real indexed package, so a
	// receiver-method call ("recv.Name") is not mistaken for a package selector.
	// Only populated when "package.Name" is unambiguous.
	byPkgName map[string]string

	// byDirName maps "dir-base.Name" -> canonical key, where dir-base is the
	// base name of the node's defining directory (path.Base(path.Dir(FilePath))).
	// It resolves callees the Go extractor canonicalized from an explicit import
	// alias to the import path's LAST SEGMENT: by Go import mechanics an import
	// path resolves to exactly one directory, so the last segment IS the
	// defining directory's base name — even when the declared package name
	// differs (kebab-case dirs like "evidence-signature" declare package
	// "evidencesignature", which byPkgName can never match). Only populated
	// when "dir-base.Name" is unambiguous across the whole node set.
	byDirName map[string]string

	// edgeKind records, for every (caller+"\x00"+callee) pair added by
	// AddTypedEdges, the typed Kind it was added with (core.EdgeDirect or
	// core.EdgeIfaceDispatch — core.EdgeUnresolved edges are never added,
	// see AddTypedEdges). Absent for a syntax-only edge (Build's own pass).
	// Consulted by TestImpactDetail (dispatch-modality detection) and by
	// CallersDetail/CalleesDetail (kind-labeled MCP surfacing, ADR-021 §3
	// CT-6). nil until AddTypedEdges is called at least once.
	edgeKind map[string]core.EdgeKind

	// dispatchOut counts, per caller key, how many EdgeIfaceDispatch edges
	// originate from it — the "N implementers" reported in verify's verdict
	// detail. If a caller has more than one distinct interface-dispatch call
	// site, N is their union: core.Edge carries no call-site identifier, so
	// this is a documented simplification (ADR-021 §3), not per-site precision.
	dispatchOut map[string]int
}

// NodeCount returns the number of nodes in the graph.
func (g *Graph) NodeCount() int { return len(g.nodes) }

// EdgeCount returns the total number of directed edges in the graph.
func (g *Graph) EdgeCount() int {
	n := 0
	for _, cs := range g.callees {
		n += len(cs)
	}
	return n
}

// ─── Build ───────────────────────────────────────────────────────────────────

// Build constructs a Graph from a slice of SymbolNodes.  The slice may be in
// any order.  Unknown callees (those not present in nodes) are silently dropped
// — they represent external dependencies.
func Build(nodes []core.SymbolNode) *Graph {
	g := buildIndexes(nodes)

	// Second pass: resolve call edges. Iterate the deduplicated node set so
	// duplicate observations of a caller cannot duplicate its edges; the
	// adjacency lists are sorted below, so map order does not leak out.
	for callerKey, n := range g.nodes {
		seen := make(map[string]struct{}) // deduplicate edges from this caller
		for _, raw := range n.Calls {
			calleeKey := g.resolve(n, raw)
			if calleeKey == "" {
				continue // unknown external symbol
			}
			if _, dup := seen[calleeKey]; dup {
				continue
			}
			seen[calleeKey] = struct{}{}
			g.callees[callerKey] = append(g.callees[callerKey], calleeKey)
			g.callers[calleeKey] = append(g.callers[calleeKey], callerKey)
		}
	}

	// Sort all adjacency lists for determinism.
	for k := range g.callees {
		sort.Strings(g.callees[k])
	}
	for k := range g.callers {
		sort.Strings(g.callers[k])
	}

	return g
}

// buildIndexes registers every node and computes the tolerant-matching
// indexes (byName / byPkgName / byDirName) WITHOUT adding a single edge.
// Build adds the syntax-derived edges on top; BuildResolved (resolved.go)
// deliberately does not, so the two graphs share one node universe and one
// canonicalization rule and differ only in which edges they carry.
func buildIndexes(nodes []core.SymbolNode) *Graph {
	g := &Graph{
		nodes:     make(map[string]core.SymbolNode, len(nodes)),
		callees:   make(map[string][]string, len(nodes)),
		callers:   make(map[string][]string, len(nodes)),
		byName:    make(map[string]string),
		byPkgName: make(map[string]string),
		byDirName: make(map[string]string),
	}

	// First pass: register all nodes (g.nodes dedupes identical SymbolKeys)
	// and build the byName / byPkgName indexes. The ambiguity counts MUST be
	// computed over the deduplicated key set, not the raw slice: duplicate
	// observations of the same symbol (e.g. stale worktree copies indexed
	// more than once) would otherwise make a genuinely unique name look
	// ambiguous and silently drop its cross-package call edges.
	for _, n := range nodes {
		g.nodes[n.Key()] = n
	}
	nameCounts := make(map[string]int, len(g.nodes))
	pkgNameCounts := make(map[string]int, len(g.nodes))
	dirNameCounts := make(map[string]int, len(g.nodes))
	for _, n := range g.nodes {
		nameCounts[n.Name]++
		if n.Package != "" {
			pkgNameCounts[n.Package+"."+n.Name]++
		}
		if dn := dirName(n); dn != "" {
			dirNameCounts[dn]++
		}
	}
	for _, n := range g.nodes {
		if nameCounts[n.Name] == 1 {
			g.byName[n.Name] = n.Key()
		}
		if n.Package != "" && pkgNameCounts[n.Package+"."+n.Name] == 1 {
			g.byPkgName[n.Package+"."+n.Name] = n.Key()
		}
		if dn := dirName(n); dn != "" && dirNameCounts[dn] == 1 {
			g.byDirName[dn] = n.Key()
		}
	}

	return g
}

// resolve converts a raw callee reference (full key or bare name) to a
// canonical key present in g.nodes.  Returns "" when unresolvable.  caller is
// the node whose Calls entry is being resolved; it scopes same-package
// bare-name binding.
func (g *Graph) resolve(caller core.SymbolNode, raw string) string {
	// Constructor-scoped method reference: "<import-path>#.Method" (see
	// GoExtractor.Extract). The Go extractor records this form when the
	// receiver variable's initializer is a constructor-shaped selector call
	// into an imported package, so the method lookup is scoped to THAT
	// package: by Go import mechanics the import path resolves to exactly one
	// directory, so a node belongs to the package iff the import path ends
	// with "/"+dir(FilePath). Exactly one method node there whose name ends
	// in ".Method" → edge; zero or more than one → NOTHING. The encoded form
	// never falls through to the global rules below (the constructor's return
	// type is unknown at the AST level; the legacy "<var>.Method" entry the
	// extractor records alongside remains the global-fallback secondary path).
	if i := strings.Index(raw, "#."); i > 0 {
		return g.resolveCtorScoped(raw[:i], raw[i+1:])
	}
	// Exact match (full SymbolKey already in nodes).
	if _, ok := g.nodes[raw]; ok {
		return raw
	}
	// Package-name-qualified full key ("svc::pkg.Name" — the form the Go
	// extractor records for a callee declared in the caller's own package,
	// and the form every stored Calls entry had before keys carried the
	// package DIRECTORY, PC-32). When the named package is the caller's
	// own, the call binds to the caller's directory-qualified package —
	// Go's same-package binding, now unambiguous between two `package main`
	// binaries. Any other service-prefixed form resolves only through the
	// unambiguous "pkg.Name" index: ambiguity resolves to nothing.
	if i := strings.Index(raw, "::"); i >= 0 {
		rest := raw[i+2:]
		if dot := strings.LastIndex(rest, "."); dot > 0 {
			if pkg, name := rest[:dot], rest[dot+1:]; pkg == caller.Package && caller.Package != "" {
				k := core.SymbolKey(caller.Service, caller.PackagePath(), name)
				if _, ok := g.nodes[k]; ok {
					return k
				}
			}
		}
		if key, ok := g.byPkgName[rest]; ok {
			return key
		}
		return ""
	}
	// Same-package bare-name binding: a bare (unqualified) identifier cannot
	// denote another package's symbol — Go's only cross-package reference form
	// is a qualified identifier, which extractors record with a dot. So when
	// the CALLER's package declares the bare name, the call binds there, by
	// language semantics — not first-match-wins. Restricted to plain function
	// callers: a method node's Package field is its receiver type name, not a
	// Go package, so binding through it would be a category error.
	if caller.Kind == core.KindGoFunc && caller.Package != "" && !strings.Contains(raw, ".") {
		k := core.SymbolKey(caller.Service, caller.PackagePath(), raw)
		if _, ok := g.nodes[k]; ok {
			return k
		}
	}
	// Unambiguous bare-name lookup.
	if key, ok := g.byName[raw]; ok {
		return key
	}
	// Precise package-qualified lookup: raw "pkg.Name" -> "svc::pkg.Name".
	if key, ok := g.byPkgName[raw]; ok {
		return key
	}
	// Directory-base lookup: raw "seg.Name" where seg is an import path's last
	// segment (the Go extractor's canonicalization of an explicit import
	// alias) matches the node defined in a directory of that base name.
	if key, ok := g.byDirName[raw]; ok {
		return key
	}
	// Suffix match: raw might be "pkg.Name" and the key is "svc::pkg.Name"
	// or "svc::dir/pkg.Name". Accept only if exactly one node matches.
	var match string
	for key := range g.nodes {
		if keyEndsWith(key, raw) {
			if match != "" {
				return "" // ambiguous
			}
			match = key
		}
	}
	if match != "" {
		return match
	}
	// Method-call fallback: extractors name a method node "Recv.Name" (key
	// "svc::Recv.Recv.Name") while a call site is recorded as "<var>.Name"
	// (e.g. "h.RunOnce"), which none of the lookups above can map. Accept
	// only when exactly one node key ends in "."+Method AND that node is a
	// method node (its Name has the "Recv.Method" shape) — mirroring the
	// "."+symbol suffix matching the query engine's callsSymbol does.
	// A method name defined on more than one receiver stays unresolved:
	// ambiguity resolves to nothing, never first-match-wins.
	if i := strings.LastIndex(raw, "."); i > 0 && i < len(raw)-1 {
		msuffix := raw[i:] // "."+Method
		var mmatch string
		for key := range g.nodes {
			if strings.HasSuffix(key, msuffix) {
				if mmatch != "" {
					return "" // ambiguous
				}
				mmatch = key
			}
		}
		if mmatch != "" && strings.HasSuffix(g.nodes[mmatch].Name, msuffix) {
			return mmatch
		}
	}
	return ""
}

// resolveCtorScoped resolves a constructor-scoped method reference: the
// unique node within the package at importPath whose Name ends in
// methodSuffix (".Method", so only "Recv.Method"-shaped method nodes can
// match — the same name-shape discipline as the global method fallback).
// Returns "" on zero or multiple matches: honesty preserved package-scoped.
func (g *Graph) resolveCtorScoped(importPath, methodSuffix string) string {
	if importPath == "" || len(methodSuffix) < 2 {
		return ""
	}
	var match string
	for key, n := range g.nodes {
		if !strings.HasSuffix(n.Name, methodSuffix) || n.FilePath == "" {
			continue
		}
		dir := path.Dir(n.FilePath)
		if dir == "." || dir == "/" {
			continue
		}
		if importPath != dir && !strings.HasSuffix(importPath, "/"+dir) {
			continue
		}
		if match != "" {
			return "" // >1 matching method in the package: ambiguous
		}
		match = key
	}
	return match
}

// dirName returns the "dir-base.Name" index key for a node, or "" when the
// node's FilePath has no usable directory.
func dirName(n core.SymbolNode) string {
	if n.FilePath == "" {
		return ""
	}
	dir := path.Dir(n.FilePath)
	if dir == "." || dir == "/" {
		return ""
	}
	return path.Base(dir) + "." + n.Name
}

// ─── RenameImpact ────────────────────────────────────────────────────────────

// RenameImpact returns the sorted slice of canonical keys for all direct
// callers of symbol.  These are the call sites that would need updating if
// symbol were renamed.
//
// symbol may be a full SymbolKey or a bare name (tolerant matching applies).
func RenameImpact(g *Graph, symbol string) []string {
	key := g.canonicalize(symbol)
	if key == "" {
		return []string{}
	}
	cs := g.callers[key]
	if len(cs) == 0 {
		return []string{}
	}
	out := make([]string, len(cs))
	copy(out, cs)
	sort.Strings(out)
	return out
}

// ─── Reachable ───────────────────────────────────────────────────────────────

// Reachable reports whether there is any directed path from the node identified
// by from to the node identified by to (following caller→callee edges).
//
// A node trivially reaches itself.  Both from and to may be full keys or bare
// names.
func Reachable(g *Graph, from, to string) bool {
	fromKey := g.canonicalize(from)
	toKey := g.canonicalize(to)
	if fromKey == "" || toKey == "" {
		return false
	}
	if fromKey == toKey {
		return true
	}
	visited := make(map[string]bool)
	return bfsReachable(g, fromKey, toKey, visited)
}

func bfsReachable(g *Graph, from, to string, visited map[string]bool) bool {
	queue := []string{from}
	visited[from] = true
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range g.callees[cur] {
			if next == to {
				return true
			}
			if !visited[next] {
				visited[next] = true
				queue = append(queue, next)
			}
		}
	}
	return false
}

// ─── CallPath ────────────────────────────────────────────────────────────────

// CallPath returns the shortest directed path (sequence of canonical keys)
// from from to to.  Returns nil when no path exists.  When from == to, returns
// a single-element slice.
//
// Among paths of equal length, the lexicographically smallest path is returned
// (deterministic BFS over sorted adjacency lists).
func CallPath(g *Graph, from, to string) []string {
	fromKey := g.canonicalize(from)
	toKey := g.canonicalize(to)
	if fromKey == "" || toKey == "" {
		return nil
	}
	if fromKey == toKey {
		return []string{fromKey}
	}

	// BFS with predecessor map to reconstruct path.
	prev := map[string]string{fromKey: ""}
	queue := []string{fromKey}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range g.callees[cur] {
			if _, seen := prev[next]; seen {
				continue
			}
			prev[next] = cur
			if next == toKey {
				return reconstructPath(prev, fromKey, toKey)
			}
			queue = append(queue, next)
		}
	}
	return nil
}

func reconstructPath(prev map[string]string, from, to string) []string {
	var path []string
	for cur := to; cur != ""; cur = prev[cur] {
		path = append(path, cur)
	}
	// Reverse.
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

// ─── TestImpact ──────────────────────────────────────────────────────────────

// TestImpact returns the sorted canonical keys of all test functions (nodes
// whose Name starts with "Test") that can transitively reach symbol via the
// call graph.
//
// symbol may be a full key or bare name.
func TestImpact(g *Graph, symbol string) []string {
	key := g.canonicalize(symbol)
	if key == "" {
		return []string{}
	}

	// Walk the reverse graph (callers) from symbol upward; collect test nodes.
	visited := make(map[string]bool)
	queue := []string{key}
	visited[key] = true
	var tests []string

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, caller := range g.callers[cur] {
			if visited[caller] {
				continue
			}
			visited[caller] = true
			node := g.nodes[caller]
			if strings.HasPrefix(node.Name, "Test") {
				tests = append(tests, caller)
			}
			queue = append(queue, caller)
		}
	}

	sort.Strings(tests)
	return tests
}

// ─── Typed edges (ADR-021 §3) ────────────────────────────────────────────────

// AddTypedEdges augments g with typed call edges from the Go typed
// enrichment pass (internal/fusion.EnrichTypedEdges), additive alongside the
// syntax-derived edges Build already computed from SymbolNode.Calls.
//
// An edge whose endpoint isn't already a node in g (e.g. a stale typed-edge
// row from before a symbol was renamed or removed, or an EdgeUnresolved
// edge — its ToSymbol is a "<unresolved:N>" callsite marker, never a real
// key) is silently skipped: AddTypedEdges never invents a graph vertex the
// extraction pass didn't produce. A (caller, callee) pair already present
// from the syntax pass is not duplicated in the adjacency lists.
func AddTypedEdges(g *Graph, edges []core.Edge) {
	if len(edges) == 0 {
		return
	}
	if g.edgeKind == nil {
		g.edgeKind = make(map[string]core.EdgeKind)
	}
	if g.dispatchOut == nil {
		g.dispatchOut = make(map[string]int)
	}
	calleesTouched := make(map[string]bool)
	callersTouched := make(map[string]bool)

	for _, e := range edges {
		if e.FromSymbol == "" || e.ToSymbol == "" {
			continue
		}
		if _, ok := g.nodes[e.FromSymbol]; !ok {
			continue
		}
		if _, ok := g.nodes[e.ToSymbol]; !ok {
			continue
		}
		ek := e.FromSymbol + "\x00" + e.ToSymbol
		if _, marked := g.edgeKind[ek]; !marked {
			g.edgeKind[ek] = e.Kind
			if e.Kind == core.EdgeIfaceDispatch {
				g.dispatchOut[e.FromSymbol]++
			}
		}
		if !containsStr(g.callees[e.FromSymbol], e.ToSymbol) {
			g.callees[e.FromSymbol] = append(g.callees[e.FromSymbol], e.ToSymbol)
			calleesTouched[e.FromSymbol] = true
		}
		if !containsStr(g.callers[e.ToSymbol], e.FromSymbol) {
			g.callers[e.ToSymbol] = append(g.callers[e.ToSymbol], e.FromSymbol)
			callersTouched[e.ToSymbol] = true
		}
	}
	for k := range calleesTouched {
		sort.Strings(g.callees[k])
	}
	for k := range callersTouched {
		sort.Strings(g.callers[k])
	}
}

// containsStr reports whether want is present in ss.
func containsStr(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

// EdgeInfo is one caller or callee edge, labeled with its typed Kind when
// known (ADR-021 §3 CT-6: find_callers / symbol_context surfacing). Kind is
// "" for a syntax-only edge — one Build derived from SymbolNode.Calls that
// AddTypedEdges never separately confirmed (either the typed pass hasn't
// run, or the call site resolved to EdgeUnresolved and so was never added
// as a graph edge at all).
type EdgeInfo struct {
	Key  string
	Kind core.EdgeKind
}

// CallersDetail returns every caller of symbol (full key or bare name),
// each labeled with its typed Kind when AddTypedEdges confirmed that
// specific (caller, symbol) pair — sorted by Key. Returns nil when symbol
// doesn't resolve.
func CallersDetail(g *Graph, symbol string) []EdgeInfo {
	key := g.canonicalize(symbol)
	if key == "" {
		return nil
	}
	callers := g.callers[key]
	out := make([]EdgeInfo, len(callers))
	for i, c := range callers {
		out[i] = EdgeInfo{Key: c, Kind: g.edgeKind[c+"\x00"+key]}
	}
	return out
}

// CalleesDetail returns every callee of symbol (full key or bare name),
// each labeled with its typed Kind when AddTypedEdges confirmed that
// specific (symbol, callee) pair — sorted by Key. Returns nil when symbol
// doesn't resolve.
func CalleesDetail(g *Graph, symbol string) []EdgeInfo {
	key := g.canonicalize(symbol)
	if key == "" {
		return nil
	}
	callees := g.callees[key]
	out := make([]EdgeInfo, len(callees))
	for i, c := range callees {
		out[i] = EdgeInfo{Key: c, Kind: g.edgeKind[key+"\x00"+c]}
	}
	return out
}

// ReachDetail describes one Test* symbol that statically reaches a target,
// and the MODALITY of that reach: whether the shortest reverse-BFS path
// crossed a typed EdgeIfaceDispatch edge (ADR-021 §3), and if so how many
// implementers were in scope at that crossing.
type ReachDetail struct {
	Key          string
	ViaDispatch  bool
	Implementers int // count of dispatch edges from the crossed caller; 0 when !ViaDispatch
}

// TestImpactDetail is TestImpact with typed-edge dispatch modality: for each
// Test* function that statically reaches symbol (via any edge, syntax or
// typed), reports whether the path crossed an EdgeIfaceDispatch edge.
// Crossing such an edge now counts as reached — strictly better evidence
// than the old blanket refusal for an ambiguous method name — but is
// labeled distinctly so a reader sees the modality (see internal/verify's
// test_reaches detail rendering).
//
// symbol may be a full key or bare name.
func TestImpactDetail(g *Graph, symbol string) []ReachDetail {
	key := g.canonicalize(symbol)
	if key == "" {
		return nil
	}

	visited := map[string]bool{key: true}
	viaDispatch := map[string]bool{}
	implementers := map[string]int{}
	queue := []string{key}
	var results []ReachDetail

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, caller := range g.callers[cur] {
			if visited[caller] {
				continue
			}
			visited[caller] = true

			crosses := viaDispatch[cur]
			n := implementers[cur]
			if !crosses && g.edgeKind[caller+"\x00"+cur] == core.EdgeIfaceDispatch {
				crosses = true
				n = g.dispatchOut[caller]
			}
			viaDispatch[caller] = crosses
			implementers[caller] = n

			if node := g.nodes[caller]; strings.HasPrefix(node.Name, "Test") {
				results = append(results, ReachDetail{Key: caller, ViaDispatch: crosses, Implementers: n})
			}
			queue = append(queue, caller)
		}
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Key < results[j].Key })
	return results
}

// ─── DeadCode ────────────────────────────────────────────────────────────────

// DeadCode returns the sorted canonical keys of all nodes that are both:
//   - not reachable from any of the provided entrypoints, AND
//   - have no callers in the graph (i.e., nothing in the graph calls them).
//
// Entrypoints may be full keys or bare names.  The result is sorted for
// determinism.
func DeadCode(g *Graph, entrypoints []string) []string {
	// Resolve entrypoints.
	epKeys := make([]string, 0, len(entrypoints))
	for _, ep := range entrypoints {
		if k := g.canonicalize(ep); k != "" {
			epKeys = append(epKeys, k)
		}
	}

	// BFS from all entrypoints to find reachable set.
	reachable := make(map[string]bool, len(g.nodes))
	queue := make([]string, 0, len(epKeys))
	for _, k := range epKeys {
		if !reachable[k] {
			reachable[k] = true
			queue = append(queue, k)
		}
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range g.callees[cur] {
			if !reachable[next] {
				reachable[next] = true
				queue = append(queue, next)
			}
		}
	}

	// Collect nodes that are unreachable AND have no callers.
	var dead []string
	for key := range g.nodes {
		if reachable[key] {
			continue
		}
		if len(g.callers[key]) == 0 {
			dead = append(dead, key)
		}
	}
	sort.Strings(dead)
	return dead
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// Canonical resolves symbol — a canonical key, a package-name-qualified
// key ("svc::pkg.Name"), a "pkg.Name" partial, or a bare name — to the
// canonical key present in g, or "" when it names nothing or more than one
// node. It is the tolerant-matching rule every public analysis in this
// package applies to its symbol argument, exposed so a caller that keys its
// own tables by canonical key (findings' file neighborhood) resolves a
// user-supplied subject the same way.
func Canonical(g *Graph, symbol string) string { return g.canonicalize(symbol) }

// canonicalize resolves symbol to the canonical key present in g.nodes.
// Accepts full keys or bare names.  Returns "" when unresolvable.
func (g *Graph) canonicalize(symbol string) string {
	if _, ok := g.nodes[symbol]; ok {
		return symbol
	}
	if key, ok := g.byName[symbol]; ok {
		return key
	}
	// Precise package-qualified lookup: "pkg.Name" -> canonical key. A
	// package-NAME-qualified full key ("svc::pkg.Name", the pre-PC-32 form a
	// caller may still hold) resolves the same way, and only when
	// unambiguous — two `package main` binaries make "svc::main.run" name
	// nothing rather than whichever was ingested last.
	if key, ok := g.byPkgName[symbol]; ok {
		return key
	}
	if i := strings.Index(symbol, "::"); i >= 0 {
		if key, ok := g.byPkgName[symbol[i+2:]]; ok {
			return key
		}
		return ""
	}
	// Try suffix match for dotted partial keys like "pkg.Name".
	var match string
	for key := range g.nodes {
		if keyEndsWith(key, symbol) {
			if match != "" {
				return "" // ambiguous
			}
			match = key
		}
	}
	return match
}

// keyEndsWith reports whether the canonical key ends with the partial
// reference raw at a component boundary: "."+raw (the historical
// "svc::pkg.Name" shape, and "Recv.Method" against "svc::dir/Recv.Recv.Method")
// or "/"+raw ("pkg.Name" against a directory-qualified "svc::dir/pkg.Name",
// PC-32). Without the "/" boundary a partial reference would match only
// root-level symbols once keys carried the package directory.
func keyEndsWith(key, raw string) bool {
	return strings.HasSuffix(key, "."+raw) || strings.HasSuffix(key, "/"+raw)
}
