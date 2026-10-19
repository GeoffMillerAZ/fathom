package callgraph_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestReachNameNamesEveryKeyOfAnAmbiguousName: ReachName places a name the
// way ReachableTyped does, and when that refuses it names every key that
// carries the name, sorted, so a refused reach can say why.
func TestReachNameNamesEveryKeyOfAnAmbiguousName(t *testing.T) {
	const svc = "rnfix"
	test := func(pkg, dir string) core.SymbolNode {
		return core.SymbolNode{Service: svc, Package: pkg, Name: "TestTwice", Kind: core.KindGoFunc, FilePath: dir + "/x_test.go"}
	}
	one := core.SymbolNode{Service: svc, Package: "lib", Name: "TestOne", Kind: core.KindGoFunc, FilePath: "lib/one_test.go"}
	b, a := test("main", "cmd/app"), test("lib", "lib")
	g := callgraph.Build([]core.SymbolNode{b, one, a})

	if key, cands := callgraph.ReachName(g, "TestOne"); key != one.Key() || cands != nil {
		t.Errorf("TestOne = %q %v, want %q and no candidates", key, cands, one.Key())
	}
	want := []string{a.Key(), b.Key()}
	if want[0] > want[1] {
		want[0], want[1] = want[1], want[0]
	}
	if key, cands := callgraph.ReachName(g, "TestTwice"); key != "" || !reflect.DeepEqual(cands, want) {
		t.Errorf("TestTwice = %q %v, want no key and %v", key, cands, want)
	}
	if ok, _ := callgraph.ReachableTyped(g, "TestTwice", one.Key()); ok {
		t.Error("an ambiguous name reached: ReachName would misreport the refusal")
	}
	if key, cands := callgraph.ReachName(g, "TestGhost"); key != "" || len(cands) != 0 {
		t.Errorf("TestGhost = %q %v, want nothing", key, cands)
	}
	if key, cands := callgraph.ReachName(nil, "TestOne"); key != "" || cands != nil {
		t.Errorf("nil graph = %q %v, want nothing", key, cands)
	}
}
