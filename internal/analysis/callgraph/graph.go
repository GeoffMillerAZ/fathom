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

	// byNameAll maps bare Name -> EVERY node key with that name, sorted.
	// byName above holds only the UNAMBIGUOUS ones, because resolving a
	// call edge must never guess; this one holds all of them, because
	// "who calls X" has a good answer when X names three functions (the
	// union) and canonicalize's refusal would throw it away. Read only by
	// CandidateKeys (resolved.go).
	byNameAll map[string][]string

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

	// bySuffix maps a component-boundary suffix (the raw keyEndsWith once
	// scanned every key for) to how many node keys end with "."+suffix or
	// "/"+suffix, and — when exactly one does — that key. Built once per
	// Build (buildSuffixIndex, O(nodes)) so resolveAmbiguousCallees's suffix
	// match costs one map lookup per unresolved callee instead of the
	// O(nodes) scan the fallback used to run for every one of them
	// (bench-pyramid SB-50). nil on a graph buildIndexes alone produced
	// (BuildResolved never calls resolve, so it never needs this).
	bySuffix map[string]suffixCandidate

	// byDotSuffix is the "."-boundary-only subset of bySuffix: it is what
	// the method-call fallback in resolveAmbiguousCallees needs, since a
	// receiver-variable callee ("<var>.Method") can only match a key
	// immediately preceded by a literal ".", never a directory separator.
	byDotSuffix map[string]suffixCandidate

	// edgeKind records, for every (caller+"\x00"+callee) pair added by
	// AddTypedEdges, the typed Kind it was added with (core.EdgeDirect or
	// core.EdgeIfaceDispatch — core.EdgeUnresolved edges are never added,
	// see AddTypedEdges). Absent for a syntax-only edge (Build's own pass).
	// Consulted by TestImpactDetail (dispatch-modality detection) and by
	// CallersDetail/CalleesDetail (kind-labeled MCP surfacing, ADR-021 §3
	// CT-6). nil until AddTypedEdges is called at least once.
	edgeKind map[string]core.EdgeKind

	// instantiates holds, per caller key, the type keys of the typed pass's
	// core.EdgeInstantiates facts (EW-15): the first-party types with
	// methods whose values the caller's body holds. Never call edges — the
	// product walk reads them to follow an interface dispatch edge only
	// into a type some reached caller instantiates. nil until
	// AddTypedEdges sees one.
	instantiates map[string][]string

	// typedFiles is every file the typed pass resolved a Go call site,
	// value reference or instantiation in (any Go typed edge's FromFile):
	// there a receiver-variable method call the syntax pass matched by
	// name alone is superseded by the typed pass's own answer (EW-15), and
	// the graph drops it (dropTypedFileGuesses, wave lg).
	typedFiles map[string]bool
	// typedDirs is the directory of every typedFiles entry: a package the
	// typed pass resolved at all. One it did not (it failed to load)
	// carries no instantiation facts, so the product walk never prunes an
	// interface dispatch into its types.
	typedDirs map[string]bool

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
	g.buildSuffixIndex()

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
		byNameAll: make(map[string][]string),
	}

	// Count deduplicated keys so stale worktrees do not create false ambiguity.
	// The final observation wins.
	for _, n := range nodes {
		g.nodes[n.Key()] = n
	}
	for key, n := range g.nodes {
		g.byNameAll[n.Name] = append(g.byNameAll[n.Name], key)
		if n.Package != "" {
			name := n.Package + "." + n.Name
			if _, seen := g.byPkgName[name]; seen {
				g.byPkgName[name] = ""
			} else {
				g.byPkgName[name] = key
			}
		}
		if name := dirName(n); name != "" {
			if _, seen := g.byDirName[name]; seen {
				g.byDirName[name] = ""
			} else {
				g.byDirName[name] = key
			}
		}
	}
	for name, keys := range g.byNameAll {
		sort.Strings(keys)
		if len(keys) == 1 {
			g.byName[name] = keys[0]
		}
	}
	// Every SymbolKey contains "::", so empty values mark collisions only
	// during construction. Published indexes contain unique matches only.
	for name, key := range g.byPkgName {
		if key == "" {
			delete(g.byPkgName, name)
		}
	}
	for name, key := range g.byDirName {
		if key == "" {
			delete(g.byDirName, name)
		}
	}

	return g
}

// suffixCandidate tracks, for one component-boundary suffix, how many
// distinct node keys end with it and — only when that count is exactly
// one — which key. count > 1 means genuinely ambiguous: resolveAmbiguousCallees
// must return "" rather than guess, never first-match-wins.
type suffixCandidate struct {
	key   string
	count int
}

// buildSuffixIndex populates bySuffix and byDotSuffix from every node key's
// component-boundary suffixes — the suffixes keyEndsWith's "."+raw / "/"+raw
// checks compare against — once per Build. It is what turns
// resolveAmbiguousCallees's suffix match and method-call fallback from an
// O(nodes) scan per unresolved callee into an O(1) map lookup against an
// index this function builds in O(nodes) (more precisely, O(total key
// bytes), since it walks each key once).
//
// For a key, a "boundary" is any position immediately after a '.' or '/'
// character; the suffix starting there is a candidate for whatever raw a
// later query names. A key can add the SAME suffix at most once (the
// per-key seen sets below): keyEndsWith's own semantics is a single
// fixed-position tail check per (key, raw) pair, so a key must never
// contribute more than one hit for a given suffix, even in the
// pathological case where the same substring recurs at two boundaries.
func (g *Graph) buildSuffixIndex() {
	g.bySuffix = make(map[string]suffixCandidate)
	g.byDotSuffix = make(map[string]suffixCandidate)
	for key := range g.nodes {
		seenAll := make(map[string]struct{})
		seenDot := make(map[string]struct{})
		for i := 1; i < len(key); i++ {
			c := key[i-1]
			if c != '.' && c != '/' {
				continue
			}
			suf := key[i:]
			if _, dup := seenAll[suf]; !dup {
				seenAll[suf] = struct{}{}
				addSuffixCandidate(g.bySuffix, suf, key)
			}
			if c == '.' {
				if _, dup := seenDot[suf]; !dup {
					seenDot[suf] = struct{}{}
					addSuffixCandidate(g.byDotSuffix, suf, key)
				}
			}
		}
	}
}

// addSuffixCandidate records one (suffix, key) occurrence: the first key
// seen for a suffix is remembered as the answer; a second occurrence clears
// it, because a query landing on this suffix has no single honest answer —
// count alone says "ambiguous", the key is only meaningful when count is 1.
func addSuffixCandidate(m map[string]suffixCandidate, suf, key string) {
	c := m[suf]
	c.count++
	if c.count == 1 {
		c.key = key
	} else {
		c.key = ""
	}
	m[suf] = c
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
	// Ambiguous-name fallback: raw might be a dotted partial ("pkg.Name",
	// "seg.Name") that keyEndsWith would match, or a receiver-variable
	// method call ("<var>.Method") the lookups above can't map. Resolved
	// through the suffix index Build computed once (bench-pyramid SB-50) —
	// see resolveAmbiguousCallees.
	return g.resolveAmbiguousCallees(raw)
}

// resolveAmbiguousCallees resolves raw through the component-boundary
// suffix indexes Build computes once per call (buildSuffixIndex), replacing
// what used to be two full O(nodes) scans of g.nodes run for every one of
// resolve's unresolved callees — O(calls * nodes) overall. The index makes
// each of the two checks below an O(1) map lookup, so resolving every call
// in the corpus costs O(calls) amortized against the one O(nodes) build.
// Semantics are byte-for-byte the ones the scans implemented: a lone
// candidate wins, two or more resolve to nothing — ambiguity, never
// first-match-wins.
func (g *Graph) resolveAmbiguousCallees(raw string) string {
	// Suffix match: raw might be "pkg.Name" and the key is "svc::pkg.Name"
	// or "svc::dir/pkg.Name". Accept only if exactly one node matches.
	if c, ok := g.bySuffix[raw]; ok {
		return c.key // "" when c.count > 1 (ambiguous); the sole key otherwise
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
		msuffix := raw[i:]  // ".Method"
		method := raw[i+1:] // "Method" — the byDotSuffix key (boundary-less)
		if c, ok := g.byDotSuffix[method]; ok && c.count == 1 {
			if strings.HasSuffix(g.nodes[c.key].Name, msuffix) {
				return c.key
			}
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

// ReachableAny reports whether any directed path exists from the node
// identified by from to AT LEAST ONE of the nodes named in to (following
// caller→callee edges) — a single BFS from from regardless of how many
// candidates to names, so a caller checking "does X reach ANY of this
// large set" (TC-22's orphan detection: does a test reach any symbol in a
// large changed-file set) pays one graph walk, not one per candidate the
// way calling Reachable in a loop would.
//
// from and to entries may be full keys or bare names. A from that
// canonicalizes to "" reports false; a to entry that does not canonicalize
// is simply never matched (never an error — the caller's own candidate set
// may legitimately include a name callgraph cannot resolve, e.g. a symbol
// outside this service).
func ReachableAny(g *Graph, from string, to []string) bool {
	fromKey := g.canonicalize(from)
	if fromKey == "" {
		return false
	}
	want := make(map[string]bool, len(to))
	for _, t := range to {
		if k := g.canonicalize(t); k != "" {
			want[k] = true
		}
	}
	if len(want) == 0 {
		return false
	}
	if want[fromKey] {
		return true
	}
	visited := map[string]bool{fromKey: true}
	queue := []string{fromKey}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range g.callees[cur] {
			if want[next] {
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
			if IsTestNode(node) {
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
// extraction pass didn't produce — with one exception, package
// initialization. The typed pass attributes a package's variable
// initializers to "<pkg>.init" (TypedEdgeEpoch 3); a package with such
// initializers but no init() function has no indexed node under that key,
// so its first edge adds one (packageInitNode). A (caller, callee) pair
// already present from the syntax pass is not duplicated in the adjacency
// lists.
//
// Once the typed edges are in, a syntax-only edge into a Go method from a
// caller in a file the typed pass resolved is dropped (wave lg's E,
// dropTypedFileGuesses): the typed pass answered that file's call sites.
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
	instTouched := make(map[string]bool)

	for _, e := range edges {
		if e.FromSymbol == "" || e.ToSymbol == "" {
			continue
		}
		switch e.Kind {
		case core.EdgeDirect, core.EdgeIfaceDispatch, core.EdgeFuncValue, core.EdgeUnresolved, core.EdgeInstantiates:
			if e.FromFile != "" {
				if g.typedFiles == nil {
					g.typedFiles, g.typedDirs = make(map[string]bool), make(map[string]bool)
				}
				g.typedFiles[e.FromFile] = true
				g.typedDirs[path.Dir(e.FromFile)] = true
			}
		}
		if _, ok := g.nodes[e.ToSymbol]; !ok {
			continue
		}
		if _, ok := g.nodes[e.FromSymbol]; !ok {
			n, isInit := packageInitNode(e)
			if !isInit {
				continue
			}
			g.nodes[e.FromSymbol] = n
		}
		if e.Kind == core.EdgeInstantiates {
			// A fact about the caller, never a call edge.
			if g.instantiates == nil {
				g.instantiates = make(map[string][]string)
			}
			if !containsStr(g.instantiates[e.FromSymbol], e.ToSymbol) {
				g.instantiates[e.FromSymbol] = append(g.instantiates[e.FromSymbol], e.ToSymbol)
				instTouched[e.FromSymbol] = true
			}
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
	for k := range instTouched {
		sort.Strings(g.instantiates[k])
	}
	g.dropTypedFileGuesses(nil)
}

// dropTypedFileGuesses is wave lg's E (LB-F199, design language-breadth
// §1.6): it removes every syntax-only edge into a Go method whose caller
// sits in a file the typed pass resolved (typedFiles), from callers (every
// caller when nil). Such an edge is the syntax pass's guess at a receiver
// — callgraph.resolve's unambiguous bare-name rule binds a dependency's
// call (`assert.Equal`, a `sync.Mutex`'s `Lock`) to the one in-service
// method of that name — and the typed pass, which compiled every call site
// in that file, answered that site itself: with a typed edge to the same
// method (then the edge is typed and stays), an edge elsewhere, an
// unresolved marker, or nothing at all for a callee outside the service.
// core.Edge carries no call site, so the file stands for the site: a file
// the typed pass never compiled (a build-tag-excluded file, a package that
// failed to load) is not in typedFiles and keeps its syntax edges.
// ProductReach.walk applied the same rule to product reach alone since
// EW-15; applying it here serves it to every reader of the graph.
func (g *Graph) dropTypedFileGuesses(callers []string) {
	if len(g.typedFiles) == 0 {
		return
	}
	if callers == nil {
		callers = make([]string, 0, len(g.callees))
		for k := range g.callees {
			callers = append(callers, k)
		}
	}
	for _, c := range callers {
		if !g.typedFiles[g.nodes[c].FilePath] {
			continue
		}
		cs := g.callees[c]
		kept := cs[:0:0]
		for _, m := range cs {
			if _, typed := g.edgeKind[c+"\x00"+m]; !typed && g.nodes[m].Kind == core.KindGoMethod {
				g.callers[m] = removeOne(g.callers[m], c)
				continue
			}
			kept = append(kept, m)
		}
		if len(kept) != len(cs) {
			g.callees[c] = kept
		}
	}
}

// isGoTypeKind reports whether kind is a Go type declaration (a named
// type or an interface): the callee kinds whose syntax-only edges are
// references, not calls (EdgeTypeRef).
func isGoTypeKind(kind core.SourceKind) bool {
	return kind == core.KindGoType || kind == core.KindGoInterface
}

// edgeLabel is the (caller, callee) edge's EdgeInfo kind across every edge
// the graph holds: the typed pass's kind, core.EdgeTypeRef for a reference
// to a Go type, "" for any other syntax-only edge.
//
// The reference is wave lg's D (LB-F200): a syntax-only edge (the typed
// pass never confirmed the pair) whose callee node is a Go type or
// interface is a reference, not a call. The kind is the callee node's,
// fixed when the graph was built, so the classification costs no map
// entry and survives ReplacePackage as is.
func (g *Graph) edgeLabel(caller, callee string) core.EdgeKind {
	if k, ok := g.edgeKind[caller+"\x00"+callee]; ok {
		return k
	}
	if isGoTypeKind(g.nodes[callee].Kind) {
		return core.EdgeTypeRef
	}
	return ""
}

// packageInitNode returns the vertex for a typed edge FROM a package's
// initialization — "<svc>::<dir>/<pkg>.init", whose FromFile sits in that
// package's directory — when no init() function put one in the index.
// Anything else (a method key, a file outside the key's directory) is not
// package initialization and gets no vertex.
func packageInitNode(e core.Edge) (core.SymbolNode, bool) {
	svc, rest, ok := strings.Cut(e.FromSymbol, "::")
	if !ok || !strings.HasSuffix(rest, ".init") || e.FromFile == "" {
		return core.SymbolNode{}, false
	}
	qual := strings.TrimSuffix(rest, ".init")
	pkg := qual[strings.LastIndex(qual, "/")+1:]
	if pkg == "" || strings.Contains(pkg, ".") {
		return core.SymbolNode{}, false
	}
	n := core.SymbolNode{Service: svc, Package: pkg, Name: "init", Kind: core.KindGoFunc, FilePath: e.FromFile, Tier: core.TierVerified}
	if n.Key() != e.FromSymbol {
		return core.SymbolNode{}, false
	}
	return n, true
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
// as a graph edge at all) — and core.EdgeTypeRef for a syntax-only edge to
// a Go type, which only the reference views (ReferrersDetail,
// ReferencesDetail, DependentsDetail, DependenciesDetail) return.
type EdgeInfo struct {
	Key  string
	Kind core.EdgeKind
}

// CallersDetail returns every caller of symbol (full key or bare name),
// each labeled with its typed Kind when AddTypedEdges confirmed that
// specific (caller, symbol) pair — sorted by Key. Returns nil when symbol
// doesn't resolve.
//
// A reference to a Go type is not a call (wave lg, LB-F200): an edge
// edgeLabel labels core.EdgeTypeRef is left out here and answered by
// ReferrersDetail (DependentsDetail returns both). The reach walkers (TestImpact,
// TestImpactDetail, Reachable, ReachableAny, CallPath, RenameImpact,
// DeadCode, ForwardReach, ProductReach) still follow it.
func CallersDetail(g *Graph, symbol string) []EdgeInfo {
	return g.detail(symbol, true, detailCalls)
}

// CalleesDetail returns every callee of symbol (full key or bare name),
// each labeled with its typed Kind when AddTypedEdges confirmed that
// specific (symbol, callee) pair — sorted by Key. Returns nil when symbol
// doesn't resolve. A reference to a Go type is left out (ReferencesDetail).
func CalleesDetail(g *Graph, symbol string) []EdgeInfo {
	return g.detail(symbol, false, detailCalls)
}

// ReferrersDetail returns the symbols that reference the Go type symbol
// without calling it (a declaration, a composite literal, an assertion),
// each labeled core.EdgeTypeRef, sorted by Key: a type's consumers, which
// CallersDetail leaves out. Returns nil when symbol doesn't resolve.
func ReferrersDetail(g *Graph, symbol string) []EdgeInfo {
	return g.detail(symbol, true, detailRefs)
}

// ReferencesDetail returns the Go types symbol references without calling
// them, each labeled core.EdgeTypeRef, sorted by Key. Returns nil when
// symbol doesn't resolve.
func ReferencesDetail(g *Graph, symbol string) []EdgeInfo {
	return g.detail(symbol, false, detailRefs)
}

// DependentsDetail is CallersDetail and ReferrersDetail together, sorted by
// Key, every row labeled (a reference core.EdgeTypeRef): what a change to
// symbol can break, for the blast-radius readers that must not lose a
// type's consumers when they stopped being callers.
func DependentsDetail(g *Graph, symbol string) []EdgeInfo {
	return g.detail(symbol, true, detailAll)
}

// DependenciesDetail is CalleesDetail and ReferencesDetail together,
// sorted by Key, every row labeled.
func DependenciesDetail(g *Graph, symbol string) []EdgeInfo {
	return g.detail(symbol, false, detailAll)
}

// detailWhich selects the edges a detail view returns.
type detailWhich int

const (
	detailCalls detailWhich = iota // calls only: no reference to a Go type
	detailRefs                     // references to a Go type only
	detailAll                      // both
)

// detail renders symbol's callers (in) or callees (!in) of the selected
// edges, each labeled by edgeLabel. The adjacency lists are sorted, so the
// rows are too.
func (g *Graph) detail(symbol string, in bool, which detailWhich) []EdgeInfo {
	key := g.canonicalize(symbol)
	if key == "" {
		return nil
	}
	adj := g.callees[key]
	if in {
		adj = g.callers[key]
	}
	out := make([]EdgeInfo, 0, len(adj))
	for _, other := range adj {
		from, to := key, other
		if in {
			from, to = other, key
		}
		kind := g.edgeLabel(from, to)
		switch {
		case which == detailCalls && kind == core.EdgeTypeRef:
			continue
		case which == detailRefs && kind != core.EdgeTypeRef:
			continue
		}
		out = append(out, EdgeInfo{Key: other, Kind: kind})
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

			if node := g.nodes[caller]; IsTestNode(node) {
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
