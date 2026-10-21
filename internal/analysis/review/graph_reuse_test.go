package review_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestRunSuppliedCompleteGraphMatchesStandalone(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "p", Name: "Changed", Kind: core.KindGoFunc, FilePath: "p/changed.go", Line: 2},
		{Service: "svc", Package: "p", Name: "TestChanged", Kind: core.KindGoFunc, FilePath: "p/changed_test.go", Line: 3},
	}
	typed := []core.Edge{{FromSymbol: nodes[1].Key(), ToSymbol: nodes[0].Key(), Kind: core.EdgeDirect}}
	in := review.Input{Service: "svc", ChangedFiles: []string{"p/changed.go"}, Nodes: nodes, TypedEdges: typed}
	want := review.Run(in)
	in.GraphBoth = callgraph.BuildBoth(nodes, typed)
	got := review.Run(in)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("shared graph verdict differs: got %#v; want %#v", got, want)
	}
	// The supplied graph, not a rebuilt graph from TypedEdges, must carry
	// the typed reachability for this call.
	in.TypedEdges = nil
	if withoutDuplicateEdges := review.Run(in); !reflect.DeepEqual(withoutDuplicateEdges, want) {
		t.Fatalf("supplied graph was not used: got %#v; want %#v", withoutDuplicateEdges, want)
	}
}
