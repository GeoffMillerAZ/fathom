// Package symbolnav provides pure, deterministic symbol-navigation analyses
// over []core.SymbolNode and a callgraph.Graph.
//
// All exported functions are stateless and safe for concurrent use; they never
// mutate their arguments.  Outputs are always sorted for determinism.
//
// # Signature note
//
// The callgraph.Graph type stores forward (callee) edges internally but does
// not expose them directly.  Functions that require forward-edge traversal
// (Callees, CallHierarchy, Community) therefore accept the original
// []core.SymbolNode slice alongside the Graph so that they can build the
// forward index locally.  The Graph is still required for the reverse (caller)
// index that it maintains.
package symbolnav

import (
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// ─── public types ────────────────────────────────────────────────────────────

// Tree is a node in a call-hierarchy tree.  Children are always sorted by
// their Key for determinism.
type Tree struct {
	Key      string // canonical SymbolKey
	Children []Tree // subtrees (callees when forward, callers when reverse)
}

// Context bundles the neighbourhood information for a single symbol.
type Context struct {
	Def          core.SymbolNode // the definition node (zero value when not found)
	Callers      []string        // canonical keys of direct callers, sorted
	Callees      []string        // canonical keys of direct callees, sorted
	RelatedTests []string        // canonical keys of Test* callers, sorted
}

// Diff summarises the difference between two symbol snapshots.
type Diff struct {
	Added    []string // keys present in new but not in old
	Removed  []string // keys present in old but not in new
	Moved    []string // keys whose FilePath changed between old and new
	Modified []string // keys whose Signature changed (and not already in Moved)
}

// ─── internal index ──────────────────────────────────────────────────────────

// forwardIndex maps canonical key → sorted slice of callee canonical keys,
// built from the raw Calls lists of SymbolNode.
type forwardIndex map[string][]string

// buildForwardIndex creates a forwardIndex that matches how callgraph.Build
// resolves callee references (exact key, then bare-name, then suffix match).
func buildForwardIndex(nodes []core.SymbolNode) forwardIndex {
	// Build node registry and bare-name index (same logic as callgraph.Build).
	nodeKeys := make(map[string]struct{}, len(nodes))
	nameCounts := make(map[string]int, len(nodes))
	for _, n := range nodes {
		nodeKeys[n.Key()] = struct{}{}
		nameCounts[n.Name]++
	}
	byName := make(map[string]string, len(nodes))
	for _, n := range nodes {
		if nameCounts[n.Name] == 1 {
			byName[n.Name] = n.Key()
		}
	}

	resolve := func(raw string) string {
		if _, ok := nodeKeys[raw]; ok {
			return raw
		}
		if k, ok := byName[raw]; ok {
			return k
		}
		suffix := "." + raw
		var match string
		for k := range nodeKeys {
			if strings.HasSuffix(k, suffix) {
				if match != "" {
					return ""
				}
				match = k
			}
		}
		return match
	}

	idx := make(forwardIndex, len(nodes))
	for _, n := range nodes {
		callerKey := n.Key()
		seen := make(map[string]struct{})
		for _, raw := range n.Calls {
			k := resolve(raw)
			if k == "" {
				continue
			}
			if _, dup := seen[k]; dup {
				continue
			}
			seen[k] = struct{}{}
			idx[callerKey] = append(idx[callerKey], k)
		}
		sort.Strings(idx[callerKey])
	}
	return idx
}

// ─── Callees ─────────────────────────────────────────────────────────────────

// Callees returns the sorted canonical keys of the direct callees of symbol.
// symbol may be a full SymbolKey or a bare function name.
//
// nodes is the same slice that was passed to callgraph.Build when constructing g.
func Callees(g *callgraph.Graph, nodes []core.SymbolNode, symbol string) []string {
	idx := buildForwardIndex(nodes)
	key := resolveSymbol(nodes, symbol)
	if key == "" {
		return []string{}
	}
	cs := idx[key]
	if len(cs) == 0 {
		return []string{}
	}
	out := make([]string, len(cs))
	copy(out, cs)
	return out // already sorted by buildForwardIndex
}

// ─── CallHierarchy ───────────────────────────────────────────────────────────

// CallHierarchy returns a tree rooted at symbol that expands both the forward
// (callee) direction and the reverse (caller) direction up to depth hops.
// depth 0 returns a leaf node; depth < 0 is treated as 0.
//
// The tree is deterministic: children at every level are sorted by Key.
func CallHierarchy(g *callgraph.Graph, nodes []core.SymbolNode, symbol string, depth int) Tree {
	if depth < 0 {
		depth = 0
	}
	idx := buildForwardIndex(nodes)
	key := resolveSymbol(nodes, symbol)
	if key == "" {
		return Tree{Key: symbol}
	}
	visited := make(map[string]struct{})
	return buildHierarchy(g, idx, key, depth, visited)
}

func buildHierarchy(g *callgraph.Graph, idx forwardIndex, key string, depth int, visited map[string]struct{}) Tree {
	t := Tree{Key: key}
	if depth == 0 {
		return t
	}
	if _, seen := visited[key]; seen {
		return t
	}
	visited[key] = struct{}{}

	// Collect callee keys (forward).
	callees := idx[key]

	// Collect caller keys (reverse) via RenameImpact.
	callers := callgraph.RenameImpact(g, key)

	// Merge into one sorted unique set of neighbour keys.
	allNeighbours := sortedUnion(callees, callers)

	for _, nk := range allNeighbours {
		child := buildHierarchy(g, idx, nk, depth-1, visited)
		t.Children = append(t.Children, child)
	}
	// Children are already in sorted order because allNeighbours is sorted.
	return t
}

// ─── Community ───────────────────────────────────────────────────────────────

// Community returns the sorted unique canonical keys of symbols that are
// closely related to symbol: its direct callers, direct callees, and any
// symbol sharing the same Package or FilePath.
func Community(g *callgraph.Graph, nodes []core.SymbolNode, symbol string) []string {
	idx := buildForwardIndex(nodes)
	key := resolveSymbol(nodes, symbol)
	if key == "" {
		return []string{}
	}

	// Find the target node for package/file comparison.
	var targetNode core.SymbolNode
	var found bool
	for _, n := range nodes {
		if n.Key() == key {
			targetNode = n
			found = true
			break
		}
	}

	set := make(map[string]struct{})

	// Direct callees.
	for _, k := range idx[key] {
		if k != key {
			set[k] = struct{}{}
		}
	}

	// Direct callers.
	for _, k := range callgraph.RenameImpact(g, key) {
		if k != key {
			set[k] = struct{}{}
		}
	}

	// Same package or same file.
	if found {
		for _, n := range nodes {
			nk := n.Key()
			if nk == key {
				continue
			}
			if (targetNode.Package != "" && n.Package == targetNode.Package) ||
				(targetNode.FilePath != "" && n.FilePath == targetNode.FilePath) {
				set[nk] = struct{}{}
			}
		}
	}

	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ─── SymbolContext ────────────────────────────────────────────────────────────

// SymbolContext returns the Context for symbol: its definition node, direct
// callers, direct callees, and any Test* callers.
func SymbolContext(nodes []core.SymbolNode, g *callgraph.Graph, symbol string) Context {
	idx := buildForwardIndex(nodes)
	key := resolveSymbol(nodes, symbol)

	var ctx Context

	if key == "" {
		return ctx
	}

	// Definition node.
	for _, n := range nodes {
		if n.Key() == key {
			ctx.Def = n
			break
		}
	}

	// Callees (forward).
	cs := idx[key]
	if len(cs) > 0 {
		ctx.Callees = make([]string, len(cs))
		copy(ctx.Callees, cs)
	} else {
		ctx.Callees = []string{}
	}

	// Callers (reverse).
	callers := callgraph.RenameImpact(g, key)
	ctx.Callers = callers

	// Related tests: Test* callers.
	for _, n := range nodes {
		nk := n.Key()
		if strings.HasPrefix(n.Name, "Test") {
			for _, callee := range idx[nk] {
				if callee == key {
					ctx.RelatedTests = append(ctx.RelatedTests, nk)
					break
				}
			}
		}
	}
	sort.Strings(ctx.RelatedTests)
	if ctx.RelatedTests == nil {
		ctx.RelatedTests = []string{}
	}

	return ctx
}

// ─── Entrypoints ─────────────────────────────────────────────────────────────

// Entrypoints returns the sorted SymbolNodes that are likely program
// entrypoints: functions named "main", exported functions whose signature
// contains "http.ResponseWriter" (HTTP handler pattern), or CLI-root patterns
// (cobra/urfave Command.RunE/Run signatures).
func Entrypoints(nodes []core.SymbolNode) []core.SymbolNode {
	var out []core.SymbolNode
	for _, n := range nodes {
		if isEntrypoint(n) {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		ki, kj := out[i].Key(), out[j].Key()
		return ki < kj
	})
	return out
}

func isEntrypoint(n core.SymbolNode) bool {
	// Exact name "main".
	if n.Name == "main" {
		return true
	}
	// Exported HTTP handler: signature contains "http.ResponseWriter".
	if isExported(n.Name) && strings.Contains(n.Signature, "http.ResponseWriter") {
		return true
	}
	// CLI roots: exported funcs whose signature matches cobra/urfave patterns.
	if isExported(n.Name) && isCLIRoot(n.Signature) {
		return true
	}
	return false
}

func isExported(name string) bool {
	if name == "" {
		return false
	}
	first := rune(name[0])
	return first >= 'A' && first <= 'Z'
}

func isCLIRoot(sig string) bool {
	// Cobra: func(*cobra.Command, []string) error
	// urfave/cli: func(*cli.Context) error
	return strings.Contains(sig, "*cobra.Command") ||
		strings.Contains(sig, "*cli.Context") ||
		strings.Contains(sig, "cli.Context")
}

// ─── Outline ─────────────────────────────────────────────────────────────────

// Outline returns the SymbolNodes defined in the given file, ordered by their
// Line number (ascending).  Ties are broken by Name for determinism.
func Outline(nodes []core.SymbolNode, file string) []core.SymbolNode {
	file = strings.TrimPrefix(file, "./")
	var out []core.SymbolNode
	for _, n := range nodes {
		if fileMatches(n.FilePath, file) {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// fileMatches reports whether a stored FilePath matches the queried file. The
// store keys symbols by absolute path, but the documented `outline` contract
// accepts a repo-relative path; this reconciles the two. A relative query is
// matched on a path-segment suffix (the leading "/" prevents "participant.go"
// from matching "myparticipant.go"); an absolute query is matched exactly.
func fileMatches(stored, query string) bool {
	if query == "" {
		return false
	}
	if stored == query {
		return true
	}
	return strings.HasSuffix(stored, "/"+query)
}

// ─── Implementations ─────────────────────────────────────────────────────────

// Implementations returns the sorted names (not full keys) of types whose
// exported method sets appear to cover the method set of the named interface.
//
// The heuristic: collect all method names declared on iface (nodes with
// Kind==KindGoInterface and Package+Name matching iface, or nodes that are
// methods whose receiver matches iface).  Then find all types whose method
// nodes include every required method name.
//
// iface may be a bare name or a "pkg.Name" qualified name.
func Implementations(nodes []core.SymbolNode, iface string) []string {
	ifaceMethods := collectInterfaceMethods(nodes, iface)
	if len(ifaceMethods) == 0 {
		return []string{}
	}

	// For each type, collect its method names.
	typeMethods := make(map[string]map[string]struct{}) // typeName → set of method names
	for _, n := range nodes {
		if n.Kind != core.KindGoMethod {
			continue
		}
		// Receiver type is encoded in the Name as "Type.Method" or just "Method"
		// with Package context.  We use the convention "ReceiverType.MethodName".
		recv, meth := splitReceiverMethod(n.Name)
		if recv == "" || meth == "" {
			continue
		}
		if typeMethods[recv] == nil {
			typeMethods[recv] = make(map[string]struct{})
		}
		typeMethods[recv][meth] = struct{}{}
	}

	var out []string
	for typeName, meths := range typeMethods {
		if typeName == iface {
			continue // skip self
		}
		covers := true
		for m := range ifaceMethods {
			if _, ok := meths[m]; !ok {
				covers = false
				break
			}
		}
		if covers {
			out = append(out, typeName)
		}
	}
	sort.Strings(out)
	return out
}

// collectInterfaceMethods returns the set of method names that iface declares.
// It looks for KindGoInterface nodes matching iface and KindGoMethod nodes
// whose receiver is iface.
func collectInterfaceMethods(nodes []core.SymbolNode, iface string) map[string]struct{} {
	methods := make(map[string]struct{})
	// Bare or qualified name match for interface definition.
	for _, n := range nodes {
		if n.Kind != core.KindGoInterface {
			continue
		}
		if n.Name == iface || qualifiedName(n) == iface {
			// Interface node found; its method names are encoded in its Calls
			// (pointing to its method nodes) or we look for method nodes with
			// receiver == iface.
			break
		}
	}
	// Collect method names for receiver == iface.
	for _, n := range nodes {
		if n.Kind != core.KindGoMethod {
			continue
		}
		recv, meth := splitReceiverMethod(n.Name)
		if recv == iface || qualifiedName(n) == iface {
			if meth != "" {
				methods[meth] = struct{}{}
			} else if recv == iface {
				methods[n.Name] = struct{}{}
			}
		}
	}
	// Also parse method names from the interface node's Signature or Calls.
	for _, n := range nodes {
		if n.Kind != core.KindGoInterface {
			continue
		}
		if n.Name != iface && qualifiedName(n) != iface {
			continue
		}
		for _, c := range n.Calls {
			_, meth := splitReceiverMethod(c)
			if meth != "" {
				methods[meth] = struct{}{}
			} else {
				// bare name
				methods[c] = struct{}{}
			}
		}
	}
	return methods
}

func qualifiedName(n core.SymbolNode) string {
	if n.Package == "" {
		return n.Name
	}
	// last segment of package path
	pkg := n.Package
	if idx := strings.LastIndex(pkg, "/"); idx >= 0 {
		pkg = pkg[idx+1:]
	}
	return pkg + "." + n.Name
}

// splitReceiverMethod splits a name like "Receiver.Method" into its parts.
// Returns ("", "") if the name has no dot.
func splitReceiverMethod(name string) (recv, meth string) {
	idx := strings.Index(name, ".")
	if idx < 0 {
		return "", ""
	}
	return name[:idx], name[idx+1:]
}

// ─── TypeHierarchy ───────────────────────────────────────────────────────────

// TypeHierarchy returns a map from interface name (bare) to the sorted slice
// of implementor names (bare), using the same heuristic as Implementations.
//
// Only interfaces that have at least one discoverable implementor are included.
// The map is deterministic when iterated in key order.
func TypeHierarchy(nodes []core.SymbolNode) map[string][]string {
	// Collect all interface names.
	ifaceNames := make(map[string]struct{})
	for _, n := range nodes {
		if n.Kind == core.KindGoInterface {
			ifaceNames[n.Name] = struct{}{}
		}
	}

	result := make(map[string][]string)
	for iface := range ifaceNames {
		impls := Implementations(nodes, iface)
		if len(impls) > 0 {
			result[iface] = impls
		}
	}
	return result
}

// ─── SymbolDiff ──────────────────────────────────────────────────────────────

// SymbolDiff computes the structural difference between two snapshots of a
// symbol set.  Symbols are matched by their canonical Key().
//
// A symbol is:
//   - Added   if its key appears in new but not in old.
//   - Removed if its key appears in old but not in new.
//   - Moved   if it appears in both and its FilePath changed.
//   - Modified if it appears in both, FilePath is the same, but Signature differs.
//
// A symbol may only appear in one category (Moved takes precedence over
// Modified).  All slices in the result are sorted.
func SymbolDiff(oldNodes, newNodes []core.SymbolNode) Diff {
	oldMap := make(map[string]core.SymbolNode, len(oldNodes))
	for _, n := range oldNodes {
		oldMap[n.Key()] = n
	}
	newMap := make(map[string]core.SymbolNode, len(newNodes))
	for _, n := range newNodes {
		newMap[n.Key()] = n
	}

	var added, removed, moved, modified []string

	for k, nn := range newMap {
		on, exists := oldMap[k]
		if !exists {
			added = append(added, k)
			continue
		}
		if nn.FilePath != on.FilePath {
			moved = append(moved, k)
		} else if nn.Signature != on.Signature {
			modified = append(modified, k)
		}
	}

	for k := range oldMap {
		if _, exists := newMap[k]; !exists {
			removed = append(removed, k)
		}
	}

	sort.Strings(added)
	sort.Strings(removed)
	sort.Strings(moved)
	sort.Strings(modified)

	return Diff{
		Added:    nilToEmpty(added),
		Removed:  nilToEmpty(removed),
		Moved:    nilToEmpty(moved),
		Modified: nilToEmpty(modified),
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// resolveSymbol finds the canonical key for symbol among nodes.
// It accepts an exact key, a bare Name, or a "pkg.Name" suffix.
func resolveSymbol(nodes []core.SymbolNode, symbol string) string {
	// Exact key match.
	for _, n := range nodes {
		if n.Key() == symbol {
			return symbol
		}
	}
	// Bare name (unambiguous).
	var match string
	for _, n := range nodes {
		if n.Name == symbol {
			if match != "" {
				return "" // ambiguous
			}
			match = n.Key()
		}
	}
	if match != "" {
		return match
	}
	// Suffix match ".<symbol>".
	suffix := "." + symbol
	match = ""
	for _, n := range nodes {
		k := n.Key()
		if strings.HasSuffix(k, suffix) {
			if match != "" {
				return "" // ambiguous
			}
			match = k
		}
	}
	return match
}

// sortedUnion returns a sorted, deduplicated union of two already-sorted slices.
func sortedUnion(a, b []string) []string {
	set := make(map[string]struct{}, len(a)+len(b))
	for _, s := range a {
		set[s] = struct{}{}
	}
	for _, s := range b {
		set[s] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// nilToEmpty returns an empty non-nil slice when s is nil.
func nilToEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
