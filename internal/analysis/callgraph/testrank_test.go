package callgraph_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestRankedTestImpact_SameSetNearestFirst: RankedTestImpact (O1128) is
// TestImpact's set, ranked by hops, then directory distance, then key.
func TestRankedTestImpact_SameSetNearestFirst(t *testing.T) {
	fn := func(file, pkg, name string, calls ...string) core.SymbolNode {
		return core.SymbolNode{Service: "svc", Package: pkg, Name: name, Kind: core.KindGoFunc, FilePath: file, Line: 1, Calls: calls}
	}
	g := callgraph.Build([]core.SymbolNode{
		fn("a/b/x.go", "b", "X"),
		fn("a/b/h.go", "b", "H", "X"),
		fn("z/z_test.go", "z", "TestADirect", "b.X"),  // 1 hop, far
		fn("a/b/x_test.go", "b", "TestZDirect", "X"),  // 1 hop, same dir
		fn("a/c/c_test.go", "c", "TestBNear", "b.H"),  // 2 hops, distance 2
		fn("q/r/s/s_test.go", "s", "TestAFar", "b.H"), // 2 hops, distance 5
		fn("a/b/y_test.go", "b", "TestYSame", "H"),    // 2 hops, same dir
	})
	ranked := callgraph.RankedTestImpact(g, "X")
	var got []string
	for _, r := range ranked {
		got = append(got, r.Key)
	}
	want := []string{
		"svc::a/b/b.TestZDirect", "svc::z/z.TestADirect",
		"svc::a/b/b.TestYSame", "svc::a/c/c.TestBNear", "svc::q/r/s/s.TestAFar",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RankedTestImpact(X) = %v, want %v", got, want)
	}
	set := append([]string(nil), got...)
	sort.Strings(set)
	if plain := callgraph.TestImpact(g, "X"); !reflect.DeepEqual(set, plain) {
		t.Errorf("RankedTestImpact set %v != TestImpact %v", set, plain)
	}
	if ranked[0].Hops != 1 || ranked[0].DirDist != 0 || ranked[4].Hops != 2 || ranked[4].DirDist != 5 {
		t.Errorf("hops/distance = %+v", ranked)
	}
}
