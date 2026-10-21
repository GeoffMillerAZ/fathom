package callgraph

import (
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// Frozen original buildIndexes. This reference must allocate each map once,
// including on the baseline arm, so the allocation RED cannot be circular.
func h3ReservedIndexes(nodes []core.SymbolNode) *Graph {
	g := &Graph{
		nodes:     make(map[string]core.SymbolNode, len(nodes)),
		callees:   make(map[string][]string, len(nodes)),
		callers:   make(map[string][]string, len(nodes)),
		byName:    make(map[string]string),
		byPkgName: make(map[string]string),
		byDirName: make(map[string]string),
		byNameAll: make(map[string][]string),
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
		key := n.Key()
		g.byNameAll[n.Name] = append(g.byNameAll[n.Name], key)
		if nameCounts[n.Name] == 1 {
			g.byName[n.Name] = key
		}
		if n.Package != "" && pkgNameCounts[n.Package+"."+n.Name] == 1 {
			g.byPkgName[n.Package+"."+n.Name] = key
		}
		if dn := dirName(n); dn != "" && dirNameCounts[dn] == 1 {
			g.byDirName[dn] = key
		}
	}
	for k := range g.byNameAll {
		sort.Strings(g.byNameAll[k])
	}

	return g
}

func h3Build(mode string, nodes []core.SymbolNode, typed []core.Edge) *Graph {
	switch mode {
	case "both":
		return BuildBoth(nodes, typed)
	case "resolved":
		return BuildResolved(nodes, typed)
	default:
		return Build(nodes)
	}
}

// Baseline Build's edge pass, with reserved adjacency restored before any
// insertion. Duplicates and sorting follow the original production algorithm.
func h3ReservedBuild(mode string, nodes []core.SymbolNode, typed []core.Edge) *Graph {
	g := h3ReservedIndexes(nodes)
	if mode != "resolved" {
		g.buildSuffixIndex()
		for callerKey, n := range g.nodes {
			seen := make(map[string]struct{})
			for _, raw := range n.Calls {
				calleeKey := g.resolve(n, raw)
				if calleeKey == "" {
					continue
				}
				if _, duplicate := seen[calleeKey]; duplicate {
					continue
				}
				seen[calleeKey] = struct{}{}
				g.callees[callerKey] = append(g.callees[callerKey], calleeKey)
				g.callers[calleeKey] = append(g.callers[calleeKey], callerKey)
			}
		}
		for k := range g.callees {
			sort.Strings(g.callees[k])
		}
		for k := range g.callers {
			sort.Strings(g.callers[k])
		}
	}
	if mode == "resolved" {
		resolved := make([]core.Edge, 0, len(typed))
		for _, e := range typed {
			if core.EdgeConfidenceOf(e) == core.EdgeConfidenceResolved {
				resolved = append(resolved, e)
			}
		}
		AddTypedEdges(g, resolved)
	} else if mode == "both" && len(typed) > 0 {
		AddTypedEdges(g, typed)
	}
	return g
}

func h3AssertEqual(t *testing.T, got, want *Graph) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("graph contents changed: nodes %d/%d edges %d/%d", got.NodeCount(), want.NodeCount(), got.EdgeCount(), want.EdgeCount())
	}
	queries := []string{"missing", "Beta", "Run", "dep.Service.Run", "Entry"}
	for key := range want.nodes {
		queries = append(queries, key)
	}
	sort.Strings(queries)
	for _, query := range queries {
		if !reflect.DeepEqual(CandidateKeys(got, query), CandidateKeys(want, query)) ||
			Canonical(got, query) != Canonical(want, query) ||
			!reflect.DeepEqual(CallersDetail(got, query), CallersDetail(want, query)) ||
			!reflect.DeepEqual(CalleesDetail(got, query), CalleesDetail(want, query)) {
			t.Fatalf("public query changed for %q", query)
		}
		for _, target := range queries {
			if Reachable(got, query, target) != Reachable(want, query, target) ||
				!reflect.DeepEqual(CallPath(got, query, target), CallPath(want, query, target)) {
				t.Fatalf("reachability/order changed for %q -> %q", query, target)
			}
		}
	}
}

func h3Freeze(g *Graph) *Graph {
	s := *g
	s.nodes = cloneNodes(g.nodes)
	for k, n := range s.nodes {
		if n.Calls != nil {
			n.Calls = append(make([]string, 0, len(n.Calls)), n.Calls...)
			s.nodes[k] = n
		}
	}
	s.callees, s.callers = cloneAdjacency(g.callees), cloneAdjacency(g.callers)
	s.byName, s.byPkgName, s.byDirName = cloneStrMap(g.byName), cloneStrMap(g.byPkgName), cloneStrMap(g.byDirName)
	s.byNameAll = cloneAdjacency(g.byNameAll)
	if g.bySuffix != nil {
		s.bySuffix = make(map[string]suffixCandidate)
		for k, v := range g.bySuffix {
			s.bySuffix[k] = v
		}
	}
	if g.byDotSuffix != nil {
		s.byDotSuffix = make(map[string]suffixCandidate)
		for k, v := range g.byDotSuffix {
			s.byDotSuffix[k] = v
		}
	}
	s.edgeKind = cloneEdgeKindMap(g.edgeKind)
	if g.dispatchOut != nil {
		s.dispatchOut = cloneIntMap(g.dispatchOut)
	}
	if g.instantiates != nil {
		s.instantiates = cloneAdjacency(g.instantiates)
	}
	s.typedFiles, s.typedDirs = cloneBoolMap(g.typedFiles), cloneBoolMap(g.typedDirs)
	return &s
}

func TestAdjacencyCapacitySemantics(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "fixture", Package: "app", Name: "Entry", FilePath: "app/entry.go", Kind: core.KindGoFunc},
		{Service: "fixture", Package: "app", Name: "Alpha", FilePath: "app/alpha.go", Kind: core.KindGoFunc},
		{Service: "fixture", Package: "one", Name: "Beta", FilePath: "one/beta.go", Kind: core.KindGoFunc},
		{Service: "fixture", Package: "two", Name: "Beta", FilePath: "two/beta.go", Kind: core.KindGoFunc},
		{Service: "fixture", Package: "dep", Name: "Service.Run", FilePath: "dep/service.go", Kind: core.KindGoMethod},
		{Service: "fixture", Package: "data", Name: "Sink", FilePath: "data/config.json", Kind: core.KindJSONKey},
	}
	entry, alpha, sink := nodes[0].Key(), nodes[1].Key(), nodes[5].Key()
	nodes[0].Calls = []string{alpha, alpha, "Beta", "recv.Run", "unknown"}
	nodes[1].Calls = []string{entry}
	typed := []core.Edge{
		{FromService: "fixture", FromFile: nodes[0].FilePath, FromSymbol: entry, ToSymbol: sink, Kind: core.EdgeDirect},
		{FromService: "fixture", FromFile: nodes[0].FilePath, FromSymbol: entry, ToSymbol: sink, Kind: core.EdgeDirect},
	}
	for _, mode := range []string{"syntax", "both", "resolved"} {
		t.Run(mode, func(t *testing.T) {
			g, old := h3Build(mode, nodes, typed), h3ReservedBuild(mode, nodes, typed)
			h3AssertEqual(t, g, old)
			if g.NodeCount() != 6 || Canonical(g, "Beta") != "" || len(CandidateKeys(g, "Beta")) != 2 {
				t.Fatal("node universe or ambiguity changed")
			}
			if mode != "resolved" && (!reflect.DeepEqual(g.callers[alpha], []string{entry}) || !reflect.DeepEqual(g.callees[alpha], []string{entry})) {
				t.Fatal("caller/callee orientation or duplicate edge changed")
			}
			if mode != "syntax" && !reflect.DeepEqual(g.callers[sink], []string{entry}) {
				t.Fatal("duplicate typed edge was not represented once")
			}
			if mode == "syntax" && !containsStr(g.callees[entry], nodes[4].Key()) || mode == "both" && containsStr(g.callees[entry], nodes[4].Key()) {
				t.Fatal("typed-file method-guess suppression changed")
			}
			t.Logf("initial mode=%s nodes=%d edges=%d", mode, g.NodeCount(), g.EdgeCount())
			reversed := append([]core.SymbolNode(nil), nodes...)
			for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
				reversed[i], reversed[j] = reversed[j], reversed[i]
			}
			h3AssertEqual(t, h3Build(mode, reversed, typed), old)

			// Build is last-wins for duplicate keys, including differing Calls.
			duplicate := nodes[0]
			duplicate.Calls = []string{sink, sink}
			dups := append(append([]core.SymbolNode(nil), nodes...), duplicate)
			duplicateGraph := h3Build(mode, dups, typed)
			h3AssertEqual(t, duplicateGraph, h3ReservedBuild(mode, dups, typed))
			if !reflect.DeepEqual(duplicateGraph.callees[entry], []string{sink}) {
				t.Fatal("duplicate caller must retain last-wins Calls and one sink edge")
			}
			beforeTypedOnly := h3Freeze(g)
			h3AssertEqual(t, g.ReplacePackage("app", nil, nil), old.ReplacePackage("app", nil, nil))
			h3AssertEqual(t, g, beforeTypedOnly)

			// Replacement retains its separate raw-entry semantics. It may
			// append to initially empty maps and must never mutate old holders.
			fresh := core.SymbolNode{Service: "fixture", Package: "app", Name: "Fresh", FilePath: "app/fresh.go", Kind: core.KindGoFunc}
			first := nodes[0]
			first.Calls = []string{sink}
			last := nodes[0]
			last.Calls = []string{fresh.Key()}
			replacements := [][]core.SymbolNode{{first, last, fresh}, {}, {first, fresh}}
			for step, replacement := range replacements {
				before := h3Freeze(g)
				gNext := g.ReplacePackage("app", replacement, []core.Edge{})
				oldNext := old.ReplacePackage("app", replacement, []core.Edge{})
				h3AssertEqual(t, gNext, oldNext)
				h3AssertEqual(t, g, before)
				if step == 0 {
					want := []string{sink, fresh.Key()}
					sort.Strings(want)
					if !reflect.DeepEqual(gNext.callees[entry], want) {
						t.Fatal("replacement raw duplicate callers lost an edge")
					}
				}
				if step == 1 && (gNext.EdgeCount() != 0 || len(gNext.callers[alpha]) != 0 || len(gNext.callees[entry]) != 0) {
					t.Fatal("package removal left outgoing or reverse edges")
				}
				t.Logf("replacement mode=%s step=%d nodes=%d edges=%d", mode, step, gNext.NodeCount(), gNext.EdgeCount())
				g, old = gNext, oldNext
			}
		})
	}
}

func TestAdjacencyCapacityEmptyAndTypedInsertion(t *testing.T) {
	for _, mode := range []string{"syntax", "both", "resolved"} {
		h3AssertEqual(t, h3Build(mode, nil, nil), h3ReservedBuild(mode, nil, nil))
	}
	nodes := []core.SymbolNode{
		{Service: "fixture", Name: "A", Kind: core.KindGoFunc},
		{Service: "fixture", Name: "B", Kind: core.KindGoFunc},
	}
	g := BuildResolved(nodes, nil)
	if g.callees == nil || g.callers == nil || g.HasEdges() {
		t.Fatal("empty adjacency must remain writable")
	}
	edge := core.Edge{FromSymbol: nodes[0].Key(), ToSymbol: nodes[1].Key(), Kind: core.EdgeDirect}
	AddTypedEdges(g, []core.Edge{edge, edge})
	if g.EdgeCount() != 1 || !reflect.DeepEqual(g.callees[edge.FromSymbol], []string{edge.ToSymbol}) || !reflect.DeepEqual(g.callers[edge.ToSymbol], []string{edge.FromSymbol}) {
		t.Fatal("typed insertion into empty adjacency changed")
	}
	nodes[0].Calls = []string{nodes[1].Key()}
	empty := Build(nil)
	added := empty.ReplacePackage("", nodes, []core.Edge{})
	if empty.NodeCount() != 0 || empty.EdgeCount() != 0 || added.NodeCount() != 2 || added.EdgeCount() != 1 {
		t.Fatal("package addition to an empty graph changed")
	}
}

func h3Workload(n int, dense bool) ([]core.SymbolNode, []core.Edge) {
	nodes := make([]core.SymbolNode, n)
	for i := range nodes {
		nodes[i] = core.SymbolNode{Service: "fixture", Package: "data", Name: fmt.Sprintf("Key%05d", i), FilePath: "data/config.json", Kind: core.KindJSONKey}
	}
	var typed []core.Edge
	for i := range nodes {
		if !dense && i > 0 {
			continue
		}
		for offset := 1; offset <= 4; offset++ {
			target := nodes[(i+offset)%n].Key()
			nodes[i].Calls = append(nodes[i].Calls, target)
			typed = append(typed, core.Edge{FromService: "fixture", FromSymbol: nodes[i].Key(), ToSymbol: target, Kind: core.EdgeDirect})
		}
	}
	return nodes, typed
}

func h3Allocated(build func() *Graph) (uint64, int, int) {
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	g := build()
	runtime.ReadMemStats(&after)
	nodes, edges := g.NodeCount(), g.EdgeCount()
	runtime.KeepAlive(g)
	return after.TotalAlloc - before.TotalAlloc, nodes, edges
}

func TestAdjacencyCapacitySparseAllocationBound(t *testing.T) {
	// Fixed hypothesis: removing only empty adjacency reservations saves
	// at least512KiB for16384 indexed nodes. Baseline's identical builders
	// must expose this RED; compile errors and skips cannot qualify.
	nodes, _ := h3Workload(16384, false)
	var saved [3]int64
	for i := range saved {
		actual, n, e := h3Allocated(func() *Graph { return buildIndexes(nodes) })
		reserved, rn, re := h3Allocated(func() *Graph { return h3ReservedIndexes(nodes) })
		if n != 16384 || rn != n || e != 0 || re != 0 {
			t.Fatal("index allocation workload changed")
		}
		saved[i] = int64(reserved) - int64(actual)
		t.Logf("sample=%d nodes=%d raw_calls=4 indexed_edges=%d production_B=%d reserved_B=%d saved_B=%d", i, n, e, actual, reserved, saved[i])
	}
	runtime.KeepAlive(nodes)
	for _, saving := range saved {
		if saving < 512<<10 {
			t.Fatalf("adjacency reservation saving below512KiB: samples=%v", saved)
		}
	}
}

var h3BenchmarkSink *Graph

func BenchmarkAdjacencyCapacity(b *testing.B) {
	for _, dense := range []bool{false, true} {
		shape := "sparse"
		if dense {
			shape = "dense"
		}
		nodes, typed := h3Workload(16384, dense)
		for _, mode := range []string{"syntax", "both", "resolved"} {
			b.Run(shape+"/"+mode, func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					h3BenchmarkSink = h3Build(mode, nodes, typed)
				}
				b.StopTimer()
				b.ReportMetric(float64(h3BenchmarkSink.NodeCount()), "nodes/op")
				b.ReportMetric(float64(h3BenchmarkSink.EdgeCount()), "edges/op")
				b.ReportMetric(float64(len(typed)), "raw-calls/op")
			})
		}
	}
}

func h3Retained(build func() *Graph) (int64, int, int) {
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	g := build()
	runtime.GC()
	runtime.ReadMemStats(&after)
	nodes, edges := g.NodeCount(), g.EdgeCount()
	runtime.KeepAlive(g)
	return int64(after.HeapAlloc) - int64(before.HeapAlloc), nodes, edges
}

func TestAdjacencyCapacityRetainedWorkload(t *testing.T) {
	for _, dense := range []bool{false, true} {
		nodes, typed := h3Workload(16384, dense)
		wantEdges := 4
		if dense {
			wantEdges = 16384 * 4
		}
		for _, mode := range []string{"syntax", "both", "resolved"} {
			production, n, e := h3Retained(func() *Graph { return h3Build(mode, nodes, typed) })
			reserved, rn, re := h3Retained(func() *Graph { return h3ReservedBuild(mode, nodes, typed) })
			if n != 16384 || rn != n || e != wantEdges || re != e {
				t.Fatal("retained workload changed node/edge corpus")
			}
			t.Logf("dense=%t mode=%s nodes=%d edges=%d raw_calls=%d production_retained_HeapAlloc_B=%d reserved_retained_HeapAlloc_B=%d", dense, mode, n, e, len(typed), production, reserved)
		}
		runtime.KeepAlive(nodes)
	}
}
