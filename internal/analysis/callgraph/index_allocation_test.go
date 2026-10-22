package callgraph

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// Frozen pre-change index construction. Keep this independent of buildIndexes
// so the allocation comparison has a real baseline in the same binary.
func countMapIndexes(nodes []core.SymbolNode) *Graph {
	g := &Graph{
		nodes:   make(map[string]core.SymbolNode, len(nodes)),
		callees: make(map[string][]string, len(nodes)), callers: make(map[string][]string, len(nodes)),
		byName: make(map[string]string), byPkgName: make(map[string]string),
		byDirName: make(map[string]string), byNameAll: make(map[string][]string),
	}
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
	for name := range g.byNameAll {
		sort.Strings(g.byNameAll[name])
	}
	return g
}

func indexAllocationFixture(noise int) []core.SymbolNode {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "api", Name: "Entry", Kind: core.KindGoFunc, FilePath: "api/entry.go", Calls: []string{"OldOnly"}},
		{Service: "svc", Package: "alpha", Name: "Unique", Kind: core.KindGoFunc, FilePath: "alpha/unique.go"},
		{Service: "svc", Package: "api", Name: "OldOnly", Kind: core.KindGoFunc, FilePath: "api/old.go"},
		{Service: "svc", Package: "alpha", Name: "Shared", Kind: core.KindGoFunc, FilePath: "alpha/shared.go"},
		{Service: "svc", Package: "alpha", Name: "Shared", Kind: core.KindGoFunc, FilePath: "other/shared.go"},
		{Service: "svc", Package: "beta", Name: "Shared", Kind: core.KindGoFunc, FilePath: "third/shared.go"},
		{Service: "svc", Package: "alpha", Name: "Third", Kind: core.KindGoFunc, FilePath: "alpha/third.go"},
		{Service: "svc", Package: "alpha", Name: "Third", Kind: core.KindGoFunc, FilePath: "other/third.go"},
		{Service: "svc", Package: "alpha", Name: "Third", Kind: core.KindGoFunc, FilePath: "third/third.go"},
		{Service: "svc", Package: "docs", Name: "Chunk", Kind: core.KindDocChunk, FilePath: "docs/readme.md"},
		{Service: "svc", Package: "p1", Name: "DirCollision", Kind: core.KindGoFunc, FilePath: "a/shared/one.go"},
		{Service: "svc", Package: "p2", Name: "DirCollision", Kind: core.KindGoFunc, FilePath: "b/shared/two.go"},
		{Service: "svc", Package: "p3", Name: "DirCollision", Kind: core.KindGoFunc, FilePath: "c/shared/three.go"},
		{Service: "", Package: "", Name: "", Kind: core.KindJSONKey},
		{Service: "svc", Package: "zero", Name: "", Kind: core.KindJSONKey, FilePath: "zero/empty.json"},
		{Service: "", Package: "meta", Name: "Vacant", Kind: core.KindDocChunk, FilePath: "meta/empty.md"},
	}
	// Last observation wins for one key, including its Calls and payload.
	duplicate := nodes[1]
	duplicate.Signature = "last"
	nodes = append(nodes, duplicate)
	entry := nodes[0]
	entry.Calls = []string{"Unique", "alpha.Shared", "other.Shared", "alpha.Third", "shared.DirCollision", "docs.Chunk", "missing.None"}
	nodes = append(nodes, entry)
	for i := 0; i < noise; i++ {
		nodes = append(nodes, core.SymbolNode{Service: "svc", Package: "json", Name: fmt.Sprintf("field%d.deep.value", i), Kind: core.KindJSONKey, FilePath: fmt.Sprintf("data/file%04d.json", i)})
	}
	return nodes
}

func countMapGraph(nodes []core.SymbolNode, typed []core.Edge, mode string) *Graph {
	g := countMapIndexes(nodes)
	if mode != "resolved" {
		g.buildSuffixIndex()
		for caller, node := range g.nodes {
			seen := make(map[string]bool)
			for _, raw := range node.Calls {
				callee := g.resolve(node, raw)
				if callee == "" || seen[callee] {
					continue
				}
				seen[callee] = true
				g.callees[caller] = append(g.callees[caller], callee)
				g.callers[callee] = append(g.callers[callee], caller)
			}
		}
		for key := range g.callees {
			sort.Strings(g.callees[key])
		}
		for key := range g.callers {
			sort.Strings(g.callers[key])
		}
	}
	if mode == "resolved" {
		resolved := make([]core.Edge, 0, len(typed))
		for _, edge := range typed {
			if core.EdgeConfidenceOf(edge) == core.EdgeConfidenceResolved {
				resolved = append(resolved, edge)
			}
		}
		AddTypedEdges(g, resolved)
	} else if mode == "both" {
		AddTypedEdges(g, typed)
	}
	return g
}

func TestIndexAllocationPreservesGraphSemantics(t *testing.T) {
	nodes := indexAllocationFixture(128)
	want := countMapIndexes(nodes)
	got := buildIndexes(nodes)
	if !reflect.DeepEqual(got.nodes, want.nodes) || !reflect.DeepEqual(got.byNameAll, want.byNameAll) ||
		!reflect.DeepEqual(got.byName, want.byName) || !reflect.DeepEqual(got.byPkgName, want.byPkgName) ||
		!reflect.DeepEqual(got.byDirName, want.byDirName) {
		t.Fatal("node universe or ambiguity indexes differ from pre-change construction")
	}
	if _, exists := got.byPkgName["alpha.Third"]; exists {
		t.Fatal("triple package collision became a unique match")
	}
	if _, exists := got.byDirName["shared.DirCollision"]; exists {
		t.Fatal("triple directory collision became a unique match")
	}
	_, hasEmptyKey := got.nodes["::"]
	if len(got.byNameAll[""]) != 2 || !hasEmptyKey {
		t.Fatal("empty name or empty service key lost")
	}
	if Canonical(got, "svc::alpha.Unique") != nodes[1].Key() {
		t.Fatal("service-qualified package lookup lost")
	}
	for _, query := range []string{"Unique", "svc::alpha.Unique", "Shared", "alpha.Shared", "other.Shared", "alpha.Third", "svc::alpha.Third", "shared.DirCollision", "docs.Chunk", "Chunk", "field127.deep.value", "missing.None", "", "::", "svc::zero/zero.", "Vacant"} {
		if Canonical(got, query) != Canonical(want, query) || !reflect.DeepEqual(CandidateKeys(got, query), CandidateKeys(want, query)) {
			t.Errorf("public resolution changed for %q", query)
		}
		if got.resolve(nodes[0], query) != want.resolve(nodes[0], query) {
			t.Errorf("callee resolution changed for %q", query)
		}
	}
	if got.NodeCount() != len(nodes)-2 {
		t.Fatalf("nodes=%d, want %d", got.NodeCount(), len(nodes)-2)
	}
	if got.nodes[nodes[1].Key()].Signature != "last" || !reflect.DeepEqual(got.nodes[nodes[0].Key()].Calls,
		[]string{"Unique", "alpha.Shared", "other.Shared", "alpha.Third", "shared.DirCollision", "docs.Chunk", "missing.None"}) {
		t.Fatal("duplicate key did not keep the final observation")
	}
	for _, mode := range []string{"syntax", "both", "resolved"} {
		caller, target := nodes[0].Key(), nodes[1].Key()
		typed := []core.Edge{{FromSymbol: caller, ToSymbol: target, FromFile: nodes[0].FilePath, Kind: core.EdgeDirect}}
		var g *Graph
		switch mode {
		case "syntax":
			g = Build(nodes)
		case "both":
			g = BuildBoth(nodes, typed)
		case "resolved":
			g = BuildResolved(nodes, typed)
		}
		baseline := countMapGraph(nodes, typed, mode)
		if !reflect.DeepEqual(g, baseline) {
			t.Errorf("%s graph differs from count-map baseline", mode)
		}
		if g.NodeCount() != got.NodeCount() || !reflect.DeepEqual(CandidateKeys(g, "Shared"), CandidateKeys(want, "Shared")) {
			t.Errorf("%s lost nodes or candidates", mode)
		}
		if !Reachable(g, caller, target) {
			t.Errorf("%s lost call edge", mode)
		}
		if Reachable(g, caller, nodes[2].Key()) {
			t.Errorf("%s retained call from the overwritten observation", mode)
		}
	}
}

var indexAllocationSink *Graph

func TestIndexAllocationReducesAllocations(t *testing.T) {
	nodes := indexAllocationFixture(4096)
	baseline := testing.AllocsPerRun(3, func() { indexAllocationSink = countMapIndexes(nodes) })
	candidate := testing.AllocsPerRun(3, func() { indexAllocationSink = buildIndexes(nodes) })
	if candidate >= baseline {
		t.Fatalf("index allocations did not fall: baseline %.0f, candidate %.0f", baseline, candidate)
	}
}
