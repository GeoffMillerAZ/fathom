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

// buildForwardIndex creates a forwardIndex from g's own resolved callee
// edges — the ONE resolver (callgraph.Build) every reachability verdict
// uses, so a callee that verify or find_callers credits is the callee this
// package lists, and vice versa. It used to re-implement a subset of that
// resolver (exact key, bare name, suffix), which drifted the moment keys
// carried the package directory (PC-32) and had never applied same-package
// binding or the package-qualified index in the first place.
func buildForwardIndex(g *callgraph.Graph, nodes []core.SymbolNode) forwardIndex {
	idx := make(forwardIndex, len(nodes))
	for _, n := range nodes {
		callerKey := n.Key()
		if _, done := idx[callerKey]; done {
			continue
		}
		cs := callgraph.CalleesDetail(g, callerKey)
		out := make([]string, 0, len(cs))
		for _, c := range cs {
			out = append(out, c.Key)
		}
		sort.Strings(out)
		idx[callerKey] = out
	}
	return idx
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
	idx := buildForwardIndex(g, nodes)
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
	idx := buildForwardIndex(g, nodes)
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
	idx := buildForwardIndex(g, nodes)
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
// contains "http.ResponseWriter" (HTTP handler pattern), CLI-root patterns
// (cobra/urfave Command.RunE/Run signatures), or a Unity message of a C#
// MonoBehaviour, which the engine calls by name (unityMessageMethods).
func Entrypoints(nodes []core.SymbolNode) []core.SymbolNode {
	var out []core.SymbolNode
	unity := unityMessageMethods(nodes)
	for i, n := range nodes {
		if isEntrypoint(n) || unity[i] {
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

// unityMessages are the messages Unity's engine sends a MonoBehaviour by
// name (the Unity Scripting API's MonoBehaviour "Messages"): none is called
// from the script's own code, so each would read unreached.
var unityMessages = map[string]bool{
	"Awake": true, "Start": true, "Update": true, "FixedUpdate": true, "LateUpdate": true,
	"OnEnable": true, "OnDisable": true, "OnDestroy": true, "OnValidate": true, "Reset": true, "OnGUI": true,
	"OnApplicationFocus": true, "OnApplicationPause": true, "OnApplicationQuit": true,
	"OnTriggerEnter": true, "OnTriggerExit": true, "OnTriggerStay": true,
	"OnTriggerEnter2D": true, "OnTriggerExit2D": true, "OnTriggerStay2D": true,
	"OnCollisionEnter": true, "OnCollisionExit": true, "OnCollisionStay": true,
	"OnCollisionEnter2D": true, "OnCollisionExit2D": true, "OnCollisionStay2D": true,
	"OnControllerColliderHit": true, "OnJointBreak": true, "OnJointBreak2D": true,
	"OnParticleCollision": true, "OnParticleTrigger": true, "OnParticleSystemStopped": true, "OnParticleUpdateJobScheduled": true,
	"OnMouseDown": true, "OnMouseDrag": true, "OnMouseEnter": true, "OnMouseExit": true,
	"OnMouseOver": true, "OnMouseUp": true, "OnMouseUpAsButton": true,
	"OnBecameVisible": true, "OnBecameInvisible": true, "OnWillRenderObject": true,
	"OnPreCull": true, "OnPreRender": true, "OnPostRender": true, "OnRenderImage": true, "OnRenderObject": true,
	"OnDrawGizmos": true, "OnDrawGizmosSelected": true,
	"OnAnimatorIK": true, "OnAnimatorMove": true, "OnAudioFilterRead": true,
	"OnTransformChildrenChanged": true, "OnTransformParentChanged": true,
}

// unityMessageMethods marks (by index) each C# method that is a Unity message
// of a MonoBehaviour (LB-F290): a CSharpMethod named Owner.Message whose
// owner, a class of the same namespace (any of a partial class's files), has
// a base list naming MonoBehaviour (`: MonoBehaviour`,
// `: UnityEngine.MonoBehaviour`), or naming a class of the repository that
// does, transitively, matched by its name. The base list is the class's
// signature (the C# dialect signs a class with its header). A Unity message
// written in any other class is no entrypoint.
func unityMessageMethods(nodes []core.SymbolNode) map[int]bool {
	bases := map[[2]string][]string{} // (namespace, class name) -> base names
	for _, n := range nodes {
		if n.Kind == "CSharpClass" {
			k := [2]string{n.Package, n.Name}
			bases[k] = append(bases[k], csharpBaseNames(n.Signature)...)
		}
	}
	behaviour := map[string]bool{"MonoBehaviour": true} // class names (last segment) known to derive from it
	mono := map[[2]string]bool{}
	for changed := true; changed; {
		changed = false
		for k, bs := range bases {
			if mono[k] {
				continue
			}
			for _, b := range bs {
				if behaviour[b] {
					mono[k] = true
					behaviour[lastSegment(k[1])] = true
					changed = true
					break
				}
			}
		}
	}
	out := map[int]bool{}
	for i, n := range nodes {
		dot := strings.LastIndexByte(n.Name, '.')
		if n.Kind != "CSharpMethod" || dot < 0 || !unityMessages[n.Name[dot+1:]] {
			continue
		}
		if mono[[2]string{n.Package, n.Name[:dot]}] {
			out[i] = true
		}
	}
	return out
}

// csharpBaseNames reads a C# type header's base list (`[Attr] public class
// X<T> : A.B, I<T> where T : C`) as its bases' last name segments, without
// type arguments or a global:: prefix: [B I].
func csharpBaseNames(sig string) []string {
	for strings.HasPrefix(sig, "[") { // attribute lists
		depth, end := 0, -1
		for i, c := range sig {
			if c == '[' {
				depth++
			} else if c == ']' {
				if depth--; depth == 0 {
					end = i
					break
				}
			}
		}
		if end < 0 {
			return nil
		}
		sig = strings.TrimSpace(sig[end+1:])
	}
	colon := -1
	for i := 0; i < len(sig); i++ {
		if sig[i] == ':' {
			if i+1 < len(sig) && sig[i+1] == ':' {
				i++ // global::
				continue
			}
			colon = i
			break
		}
	}
	if colon < 0 {
		return nil
	}
	list := sig[colon+1:]
	if i := strings.Index(list, " where "); i >= 0 {
		list = list[:i]
	}
	var out []string
	depth, start := 0, 0
	for i := 0; i <= len(list); i++ {
		if i < len(list) {
			switch list[i] {
			case '<':
				depth++
				continue
			case '>':
				depth--
				continue
			case ',':
				if depth > 0 {
					continue
				}
			default:
				continue
			}
		}
		b := strings.TrimSpace(list[start:i])
		if j := strings.IndexByte(b, '<'); j >= 0 {
			b = b[:j]
		}
		b = strings.TrimPrefix(b, "global::")
		if b != "" {
			out = append(out, lastSegment(b))
		}
		start = i + 1
	}
	return out
}

// lastSegment is a dotted name's last segment.
func lastSegment(name string) string {
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		return name[i+1:]
	}
	return name
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
// It accepts an exact key, a bare Name, a "pkg.Name" partial, or the
// package-name-qualified full key ("svc::pkg.Name") a caller may still hold
// from before keys carried the package directory (PC-32) — each only when
// it names exactly one node; ambiguity resolves to nothing.
func resolveSymbol(nodes []core.SymbolNode, symbol string) string {
	key, _, _ := ResolveTiered(nodes, symbol)
	return key
}

// ResolveTiered is resolveSymbol's full-fidelity sibling (J3(a), app-kit
// O12): the exact same tier cascade (exact key, bare name, "svc::pkg.Name",
// suffix — the first tier that matches ANYTHING decides the answer, exactly
// as resolveSymbol always has), but instead of collapsing an ambiguous tier
// to a bare "", it returns every node that tier matched.
//
// ok is true only when exactly one node resolved (candidates then holds
// that one node) — the unchanged, unambiguous case. ok is false in two
// DIFFERENT situations a caller must distinguish: candidates == nil is a
// genuine miss (no tier matched anything at all — resolveSymbol's own "no
// match" case); len(candidates) > 1 is real ambiguity (a bare name, or a
// "svc::pkg.Name"/suffix query, matched more than one distinct symbol) —
// symbol_context (internal/mcp/nav_tools.go) uses this to render a bounded
// candidate list instead of a silently empty neighbourhood.
func ResolveTiered(nodes []core.SymbolNode, symbol string) (key string, candidates []core.SymbolNode, ok bool) {
	// Exact key match.
	for _, n := range nodes {
		if n.Key() == symbol {
			return symbol, []core.SymbolNode{n}, true
		}
	}
	// Bare name.
	var byName []core.SymbolNode
	for _, n := range nodes {
		if n.Name == symbol {
			byName = append(byName, n)
		}
	}
	if len(byName) == 1 {
		return byName[0].Key(), byName, true
	}
	if len(byName) > 1 {
		return "", byName, false
	}
	// "svc::pkg.Name" → the node(s) whose Package.Name matches, deduped by
	// key (two nodes sharing one key are not ambiguity).
	if i := strings.Index(symbol, "::"); i >= 0 {
		rest := symbol[i+2:]
		seen := map[string]bool{}
		var byQualified []core.SymbolNode
		for _, n := range nodes {
			if n.Package+"."+n.Name == rest && !seen[n.Key()] {
				seen[n.Key()] = true
				byQualified = append(byQualified, n)
			}
		}
		if len(byQualified) == 1 {
			return byQualified[0].Key(), byQualified, true
		}
		return "", byQualified, false
	}
	// Suffix match at a component boundary: ".<symbol>" or "/<symbol>",
	// deduped by key.
	seen := map[string]bool{}
	var bySuffix []core.SymbolNode
	for _, n := range nodes {
		k := n.Key()
		if (strings.HasSuffix(k, "."+symbol) || strings.HasSuffix(k, "/"+symbol)) && !seen[k] {
			seen[k] = true
			bySuffix = append(bySuffix, n)
		}
	}
	if len(bySuffix) == 1 {
		return bySuffix[0].Key(), bySuffix, true
	}
	return "", bySuffix, false
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
