package callgraph

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func suffixDemandFixture(noise int) []core.SymbolNode {
	nodes := []core.SymbolNode{
		{Service: "fixture", Package: "api", Name: "TestEntry", Kind: core.KindGoFunc, FilePath: "api/entry.go", Calls: []string{"Tail.Run", "one/One.Tail.Run", "r.Close", "q.Flush", "odd.End", "p.End", "NoSuch", "Tail.Run"}},
		{Service: "fixture", Package: "One", Name: "Tail.Run", Kind: core.KindCSharpMethod, FilePath: "root/one/run.cs"},
		{Service: "fixture", Package: "Two", Name: "Handle.Close", Kind: core.KindGoMethod, FilePath: "two/close.go"},
		{Service: "fixture", Package: "Three", Name: "Handle.Flush", Kind: core.KindGoMethod, FilePath: "three/flush.go"},
		{Service: "fixture", Package: "Four", Name: "Other.Flush", Kind: core.KindGoMethod, FilePath: "four/flush.go"},
		// Nonmethod End contributes to ambiguity too. A kind filter would lie.
		{Service: "fixture", Package: "odd", Name: "End", Kind: core.KindJSONKey, FilePath: "data/end.json"},
		{Service: "fixture", Package: "Five", Name: "Handle.End", Kind: core.KindGoMethod, FilePath: "five/end.go"},
	}
	nodes = append(nodes, nodes[1]) // same canonical identity must count once
	for i := 0; i < noise; i++ {
		nodes = append(nodes, core.SymbolNode{Service: "fixture", Package: "Document", Name: fmt.Sprintf("section%d.payload.deep.value%d", i, i), Kind: core.KindJSONKey, FilePath: fmt.Sprintf("data/section%d/document.json", i)})
	}
	return nodes
}

// Build an independent eager suffix universe, then use the unchanged resolver
// to compare every adjacency list, including sinks and repeated observations.
func suffixDemandEagerReference(nodes []core.SymbolNode) *Graph {
	g := buildIndexes(nodes)
	g.bySuffix = make(map[string]suffixCandidate)
	g.byDotSuffix = make(map[string]suffixCandidate)
	for key := range g.nodes {
		for i := 1; i < len(key); i++ {
			if key[i-1] == '.' || key[i-1] == '/' {
				addSuffixCandidate(g.bySuffix, key[i:], key)
				if key[i-1] == '.' {
					addSuffixCandidate(g.byDotSuffix, key[i:], key)
				}
			}
		}
	}
	for key, n := range g.nodes {
		seen := map[string]bool{}
		for _, raw := range n.Calls {
			target := g.resolve(n, raw)
			if target != "" && !seen[target] {
				seen[target] = true
				g.callees[key] = append(g.callees[key], target)
				g.callers[target] = append(g.callers[target], key)
			}
		}
	}
	for key := range g.callees {
		sort.Strings(g.callees[key])
	}
	for key := range g.callers {
		sort.Strings(g.callers[key])
	}
	return g
}

func TestSuffixDemandSemanticEquivalence(t *testing.T) {
	nodes := suffixDemandFixture(1024)
	caller, target := nodes[0].Key(), nodes[1].Key()
	typed := []core.Edge{{FromSymbol: caller, ToSymbol: target, FromFile: nodes[0].FilePath, Kind: core.EdgeDirect}}
	for _, mode := range []string{"syntax", "both", "resolved"} {
		t.Run(mode, func(t *testing.T) {
			want := suffixDemandEagerReference(nodes)
			var got *Graph
			switch mode {
			case "syntax":
				got = Build(nodes)
			case "both":
				got = BuildBoth(nodes, typed)
				AddTypedEdges(want, typed)
			case "resolved":
				got = BuildResolved(nodes, typed)
				want = buildIndexes(nodes)
				AddTypedEdges(want, typed)
			}
			if !reflect.DeepEqual(got.nodes, want.nodes) || !reflect.DeepEqual(got.callees, want.callees) || !reflect.DeepEqual(got.callers, want.callers) {
				t.Fatal("complete node or adjacency universe differs from eager reference")
			}
			wantEdges := 3 // Tail.Run, unique Close, explicit odd.End
			if mode == "both" {
				wantEdges = 2
			} // typed file suppresses syntax-only Go method Close
			if mode == "resolved" {
				wantEdges = 1
			}
			if got.EdgeCount() != wantEdges || got.NodeCount() != len(nodes)-1 {
				t.Fatalf("nodes=%d edges=%d", got.NodeCount(), got.EdgeCount())
			}
			for _, q := range []string{"Tail.Run", "Handle.Flush", "Document.section999.payload.deep.value999", "payload.deep.value999", "NoSuch"} {
				if Canonical(got, q) != Canonical(want, q) || !reflect.DeepEqual(CandidateKeys(got, q), CandidateKeys(want, q)) {
					t.Fatalf("arbitrary public query %q changed", q)
				}
			}
			if mode == "resolved" && (got.bySuffix != nil || got.byDotSuffix != nil) {
				t.Fatal("resolved build retained syntax suffix indexes")
			}
		})
	}
}

// This intentional baseline-red structural regression exposes the new memory
// behavior without noisy machine-specific allocation/time thresholds.
func TestSuffixDemandRetainsOnlyQueries(t *testing.T) {
	nodes := suffixDemandFixture(2048)
	g := Build(nodes)
	wantedRaw, wantedMethod := map[string]bool{}, map[string]bool{}
	for _, raw := range nodes[0].Calls {
		wantedRaw[raw] = true
	}
	for _, method := range []string{"Run", "Close", "Flush", "End"} {
		wantedMethod[method] = true
	}
	for suffix := range g.bySuffix {
		if !wantedRaw[suffix] {
			t.Fatalf("undemanded raw suffix retained: %q", suffix)
		}
	}
	for suffix := range g.byDotSuffix {
		if !wantedMethod[suffix] {
			t.Fatalf("undemanded method suffix retained: %q", suffix)
		}
	}
	if g.bySuffix["Tail.Run"].count != 1 || g.byDotSuffix["Flush"].count != 2 || g.byDotSuffix["End"].count != 2 {
		t.Fatal("duplicate identity or ambiguity count changed")
	}
}

func TestSuffixDemandPackageNewQueryAndRemoval(t *testing.T) {
	target := core.SymbolNode{Service: "fixture", Package: "Handlers", Name: "Handle.Execute", Kind: core.KindCSharpMethod, FilePath: "lib/handle.cs"}
	caller := core.SymbolNode{Service: "fixture", Package: "api", Name: "Entry", Kind: core.KindGoFunc, FilePath: "api/entry.go"}
	base := Build([]core.SymbolNode{caller, target})
	caller.Calls = []string{"receiver.Execute"}
	patched := base.ReplacePackage("api", []core.SymbolNode{caller}, nil)
	if !Reachable(patched, caller.Key(), target.Key()) {
		t.Fatal("new method demand failed to scan preexisting noncalling target")
	}
	if base.EdgeCount() != 0 {
		t.Fatal("patch mutated old graph")
	}
	// A newly inserted noncalling target can make existing demand ambiguous.
	duplicate := target
	duplicate.Package = "api"
	duplicate.Name = "Other.Execute"
	duplicate.FilePath = "other/handle.cs"
	ambiguous := patched.ReplacePackage("api", []core.SymbolNode{caller, duplicate}, nil)
	if len(CalleesDetail(ambiguous, caller.Key())) != 0 {
		t.Fatal("new target did not contribute to ambiguity")
	}
	// Removing the extra target refreshes the edited caller's edges.
	// Untouched packages are deliberately not re-resolved.
	restored := ambiguous.ReplacePackage("api", []core.SymbolNode{caller}, nil)
	if !Reachable(restored, caller.Key(), target.Key()) {
		t.Fatal("removed ambiguity survived caller refresh")
	}
	caller.Calls = nil
	noDemand := restored.ReplacePackage("api", []core.SymbolNode{caller}, nil)
	if noDemand.EdgeCount() != 0 || len(noDemand.bySuffix) != 0 || len(noDemand.byDotSuffix) != 0 {
		t.Fatal("removed query retained edges or suffix demand")
	}
	if !Reachable(patched, caller.Key(), target.Key()) {
		t.Fatal("subsequent patch mutated previous graph")
	}
}

var suffixDemandBenchmarkSink *Graph

// Fixture creation is outside timing. No source/type exclusion: all JSONKey
// nodes are retained alongside existing calls. Run both arms identically.
func BenchmarkSuffixDemandCorpus(b *testing.B) {
	for _, count := range []int{2048, 53671} {
		nodes := suffixDemandFixture(count)
		for _, mode := range []string{"syntax", "both", "resolved"} {
			b.Run(fmt.Sprintf("%d/%s", count, mode), func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					switch mode {
					case "syntax":
						suffixDemandBenchmarkSink = Build(nodes)
					case "both":
						suffixDemandBenchmarkSink = BuildBoth(nodes, nil)
					case "resolved":
						suffixDemandBenchmarkSink = BuildResolved(nodes, nil)
					}
				}
			})
		}
	}
}

func TestSuffixDemandMethodGuard(t *testing.T) {
	caller := core.SymbolNode{Service: "fixture", Package: "api", Name: "Entry", Kind: core.KindGoFunc, FilePath: "api/entry.go", Calls: []string{"receiver.Execute"}}
	// One dot-boundary candidate is insufficient: the Name must actually
	// have receiver.method shape, even when that candidate is a JSONKey.
	target := core.SymbolNode{Service: "fixture", Package: "data", Name: "Execute", Kind: core.KindJSONKey, FilePath: "data/doc.json"}
	g := Build([]core.SymbolNode{caller, target})
	if g.EdgeCount() != 0 {
		t.Fatal("bare nonmethod target acquired method fallback edge")
	}
	// The same noncalling JSONKey remains eligible through an explicit key.
	caller.Calls = []string{target.Key()}
	if g := Build([]core.SymbolNode{caller, target}); !Reachable(g, caller.Key(), target.Key()) {
		t.Fatal("noncalling JSONKey was filtered from target universe")
	}
}

// ReplacePackage resolves every newNodes entry, even when node membership
// takes the last observation for a repeated key. Earlier observations'
// suffix-only calls must retain the same union semantics as exact/name calls.
// This control should pass on the eager baseline and fail on H1 revision01.
func TestSuffixDemandPackageDuplicateCallerCalls(t *testing.T) {
	caller := core.SymbolNode{Service: "fixture", Package: "api", Name: "Entry", Kind: core.KindGoFunc, FilePath: "api/entry.go", Calls: []string{"Named"}}
	suffix := core.SymbolNode{Service: "fixture", Package: "Odd", Name: "Layer.Run", Kind: core.KindCSharpMethod, FilePath: "src/lib/odd.cs"}
	method := core.SymbolNode{Service: "fixture", Package: "Handle", Name: "Handle.Close", Kind: core.KindCSharpMethod, FilePath: "helpers/close.cs"}
	exact := core.SymbolNode{Service: "fixture", Package: "helpers", Name: "Exact", Kind: core.KindGoFunc, FilePath: "helpers/exact.go"}
	named := core.SymbolNode{Service: "fixture", Package: "helpers", Name: "Named", Kind: core.KindGoFunc, FilePath: "helpers/named.go"}
	original := Build([]core.SymbolNode{caller, suffix, method, exact, named})
	before := CalleesDetail(original, caller.Key())
	earlier := caller
	earlier.Calls = []string{"lib/Odd.Layer.Run", "receiver.Close", exact.Key()}
	later := caller
	later.Calls = []string{"Named"}
	patched := original.ReplacePackage("api", []core.SymbolNode{earlier, later}, nil)
	want := []string{suffix.Key(), method.Key(), exact.Key(), named.Key()}
	sort.Strings(want)
	var got []string
	for _, edge := range CalleesDetail(patched, caller.Key()) {
		got = append(got, edge.Key)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("duplicate caller's suffix/method/exact/name union=%v, want %v", got, want)
	}
	for _, target := range want {
		if got := CallersDetail(patched, target); !reflect.DeepEqual(got, []EdgeInfo{{Key: caller.Key()}}) {
			t.Fatalf("CallersDetail(%q)=%v", target, got)
		}
	}
	if !reflect.DeepEqual(patched.nodes[caller.Key()].Calls, later.Calls) {
		t.Fatal("replacement node stopped taking the last observation")
	}
	if !reflect.DeepEqual(CalleesDetail(original, caller.Key()), before) || original.EdgeCount() != 1 || original.NodeCount() != 5 || !reflect.DeepEqual(original.nodes[caller.Key()].Calls, caller.Calls) {
		t.Fatal("duplicate replacement mutated held original graph")
	}
}

// This spelling cannot resolve by exact, bare, package or directory name.
// The slash-boundary suffix itself must produce the edge.
func TestSuffixDemandSlashBoundaryOnly(t *testing.T) {
	nodes := suffixDemandFixture(0)
	raw := "one/One.Tail.Run"
	caller, target := nodes[0], nodes[1]
	caller.Calls = []string{raw}
	g := Build([]core.SymbolNode{caller, target})
	if _, exists := g.nodes[raw]; exists || g.byName[raw] != "" || g.byPkgName[raw] != "" || g.byDirName[raw] != "" {
		t.Fatal("fixture bypassed suffix-only resolution")
	}
	if c := g.bySuffix[raw]; c.count != 1 || c.key != target.Key() {
		t.Fatalf("slash suffix candidate=%+v, want %s", c, target.Key())
	}
	if g.EdgeCount() != 1 || !Reachable(g, caller.Key(), target.Key()) {
		t.Fatal("successful slash-boundary suffix-only call missing")
	}
}
