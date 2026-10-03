package findings

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestSymbolNeighborhoodKeepsTypeReferences pins wave lg's D (LB-F200) on
// the findings neighborhoods: a reference to a Go type is a neighbor as a
// call is, in both directions, though the callers and callees views leave
// it out (it is core.EdgeTypeRef, read through DependentsDetail and
// DependenciesDetail).
func TestSymbolNeighborhoodKeepsTypeReferences(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "svc", Name: "Config", Kind: core.KindGoType, FilePath: "svc/store.go"},
		{Service: "svc", Package: "svc", Name: "NewConfig", Kind: core.KindGoFunc, FilePath: "svc/store.go"},
		{Service: "svc", Package: "app", Name: "Build", Kind: core.KindGoFunc, FilePath: "app/app.go", Calls: []string{"svc.Config", "svc.NewConfig"}},
	}
	cfg, newCfg, build := nodes[0].Key(), nodes[1].Key(), nodes[2].Key()
	g := callgraph.BuildBoth(nodes, []core.Edge{{FromService: "svc", FromFile: "app/app.go", FromSymbol: build, ToSymbol: newCfg, Kind: core.EdgeDirect}})
	if cs := callgraph.CallersDetail(g, cfg); len(cs) != 0 {
		t.Fatalf("fixture: Config's callers = %+v, want none (a type reference is not a call)", cs)
	}

	if h, ok := symbolNeighborhood(g, []string{cfg}, 1)[build]; !ok || h != 1 {
		t.Errorf("Config's neighborhood: Build at hop %d (present %v), want hop 1: a type's consumer is its neighbor", h, ok)
	}
	if h, ok := symbolNeighborhood(g, []string{build}, 1)[cfg]; !ok || h != 1 {
		t.Errorf("Build's neighborhood: Config at hop %d (present %v), want hop 1: the type it references is its neighbor", h, ok)
	}
}
