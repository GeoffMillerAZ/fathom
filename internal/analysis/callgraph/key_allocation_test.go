package callgraph_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// Distinct directories remain distinct identities; repeated observations of
// one partial type share one identity. Allocation changes must not substitute
// a bare, namespace-only or file-basename key in any graph's name indexes.
func TestGraphKeyAllocationPreservesIdentity(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "fixture", Package: "Acme.Tools", Name: "Panel", Kind: core.KindCSharpClass, FilePath: "src/UI/Panel.cs"},
		{Service: "fixture", Package: "Acme.Tools", Name: "Panel", Kind: core.KindCSharpClass, FilePath: "src/UI/Panel.Partial.cs"},
		{Service: "fixture", Package: "Acme.Tools", Name: "Panel.Run", Kind: core.KindCSharpMethod, FilePath: "src/UI/Panel.cs"},
		{Service: "fixture", Package: "Acme.Tools", Name: "Panel", Kind: core.KindCSharpClass, FilePath: "src/Other/Panel.cs"},
		{Service: "fixture", Package: "root", Name: "Entry", Kind: core.KindGoFunc, FilePath: "entry.go"},
		{Service: "fixture", Package: "evidencesignature", Name: "Check", Kind: core.KindGoFunc, FilePath: "lib/evidence-signature/check.go"},
		{Service: "fixture", Package: "tests", Name: "Probe", Kind: core.KindGoFunc, FilePath: "tests/probe.go",
			Calls: []string{"evidence-signature.Check", "fixture::src/UI/Acme.Tools.Panel.Run"}},
	}
	caller := "fixture::tests/tests.Probe"
	target := "fixture::lib/evidence-signature/evidencesignature.Check"
	typed := []core.Edge{{FromService: "fixture", FromFile: "tests/probe.go", FromSymbol: caller, ToSymbol: target, Kind: core.EdgeDirect}}
	for _, mode := range []string{"syntax", "both", "resolved"} {
		t.Run(mode, func(t *testing.T) {
			var g *callgraph.Graph
			switch mode {
			case "syntax":
				g = callgraph.Build(nodes)
			case "both":
				g = callgraph.BuildBoth(nodes, typed)
			case "resolved":
				g = callgraph.BuildResolved(nodes, typed)
			}
			if g.NodeCount() != 6 {
				t.Fatalf("nodes=%d, want six canonical identities", g.NodeCount())
			}
			for query, want := range map[string][]string{
				"Panel":     {"fixture::src/Other/Acme.Tools.Panel", "fixture::src/UI/Acme.Tools.Panel"},
				"Panel.Run": {"fixture::src/UI/Acme.Tools.Panel.Run"},
				"Entry":     {"fixture::root.Entry"},
				"Check":     {target},
			} {
				if got := callgraph.CandidateKeys(g, query); !reflect.DeepEqual(got, want) {
					t.Errorf("CandidateKeys(%q)=%v, want %v", query, got, want)
				}
			}
			if !callgraph.Reachable(g, caller, target) {
				t.Fatal("canonical import alias target lost")
			}
			if mode == "resolved" && callgraph.Reachable(g, caller, "fixture::src/UI/Acme.Tools.Panel.Run") {
				t.Fatal("resolved-only graph acquired a syntax-only edge")
			}
		})
	}
}

var graphKeyAllocationSink *callgraph.Graph

// Use this identical benchmark on the original and repaired graph.go. Fixture
// preparation is outside timing. B/op and allocs/op expose key reconstruction
// cost without machine-specific elapsed-time or memory-budget assertions.
func BenchmarkGraphKeyAllocation(b *testing.B) {
	nodes := make([]core.SymbolNode, 2048)
	for i := range nodes {
		nodes[i] = core.SymbolNode{Service: "fixture", Package: "Acme.Subsystem.Tools",
			Name: fmt.Sprintf("Widget%d.Run", i), Kind: core.KindCSharpMethod,
			FilePath: fmt.Sprintf("src/modules/feature%03d/controls/widget%04d.cs", i%64, i)}
	}
	for _, mode := range []string{"syntax", "both", "resolved"} {
		b.Run(mode, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				switch mode {
				case "syntax":
					graphKeyAllocationSink = callgraph.Build(nodes)
				case "both":
					graphKeyAllocationSink = callgraph.BuildBoth(nodes, nil)
				case "resolved":
					graphKeyAllocationSink = callgraph.BuildResolved(nodes, nil)
				}
			}
		})
	}
}
