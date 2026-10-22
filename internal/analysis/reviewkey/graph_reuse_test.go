package reviewkey_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestComputeReusesResolvedGraph(t *testing.T) {
	in := baseInputs()
	for i := range 512 {
		in.Nodes = append(in.Nodes, core.SymbolNode{Service: "svc", Package: "other", Name: fmt.Sprintf("F%d", i), FilePath: "other/x.go"})
	}
	want := compute(t, in)
	private := testing.AllocsPerRun(3, func() { _ = compute(t, in) })
	in.ResolvedGraph = callgraph.BuildResolved(in.Nodes, in.TypedEdges)
	if got := compute(t, in); !reflect.DeepEqual(got, want) {
		t.Fatalf("shared resolved graph changed key: got %+v want %+v", got, want)
	}
	shared := testing.AllocsPerRun(3, func() { _ = compute(t, in) })
	t.Logf("private=%.0f shared=%.0f allocations", private, shared)
	if shared >= private*0.85 {
		t.Fatalf("shared graph allocations=%.0f, want <85%% of private graph %.0f", shared, private)
	}
}
