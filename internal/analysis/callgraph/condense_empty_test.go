package callgraph

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestClosureLeavesEmptyReachNil(t *testing.T) {
	c := condense(Build([]core.SymbolNode{{Service: "s", Package: "p", Name: "alone", Kind: core.KindGoFunc}}))
	called := false
	c.closure(c.order, c.succ, c.pred, nil, func(_ int, set bitset) {
		called = true
		if set != nil {
			t.Fatalf("empty reach set = %#v, want nil", set)
		}
	})
	if !called {
		t.Fatal("closure did not visit isolated component")
	}
}

func TestEmptyReachCasesAndCounts(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "s", Package: "p", Name: "isolated", Kind: core.KindGoFunc},
		{Service: "s", Package: "p", Name: "self", Kind: core.KindGoFunc, Calls: []string{"s::p.self"}},
		{Service: "s", Package: "p", Name: "cycleA", Kind: core.KindGoFunc, Calls: []string{"s::p.cycleB"}},
		{Service: "s", Package: "p", Name: "cycleB", Kind: core.KindGoFunc, Calls: []string{"s::p.cycleA"}},
		{Service: "s", Package: "p", Name: "source", Kind: core.KindGoFunc, Calls: []string{"s::p.sink"}},
		{Service: "s", Package: "p", Name: "sink", Kind: core.KindGoFunc},
	}
	g := Build(nodes)
	sources := []string{"s::p.isolated", "s::p.self", "s::p.cycleA", "s::p.source", "s::p.sink"}
	r := ForwardReach(g, sources)
	want := map[string][]string{
		"s::p.isolated": nil, "s::p.self": nil,
		"s::p.cycleA": {"s::p.cycleA", "s::p.cycleB"},
		"s::p.source": {"s::p.sink"}, "s::p.sink": nil,
	}
	for src, expected := range want {
		if got := r.Keys(src); !reflect.DeepEqual(got, expected) {
			t.Errorf("Keys(%s) = %v, want %v", src, got, expected)
		}
	}
	counts := TransitiveCallerCounts(g)
	for name, expected := range map[string]int{"isolated": 0, "self": 0, "cycleA": 1, "cycleB": 1, "source": 0, "sink": 1} {
		if got := counts["s::p."+name]; got != expected {
			t.Errorf("caller count for %s = %d, want %d", name, got, expected)
		}
	}
}

func TestTransitiveCallerCountsMatchesBFSWithIsolatedFields(t *testing.T) {
	const isolated = 256
	nodes := []core.SymbolNode{
		{Service: "s", Package: "p", Name: "a", Kind: core.KindGoFunc, Calls: []string{"s::p.b", "s::p.c"}},
		{Service: "s", Package: "p", Name: "b", Kind: core.KindGoFunc, Calls: []string{"s::p.d"}},
		{Service: "s", Package: "p", Name: "c", Kind: core.KindGoFunc, Calls: []string{"s::p.d"}},
		{Service: "s", Package: "p", Name: "d", Kind: core.KindGoFunc, Calls: []string{"s::p.c"}},
	}
	for i := 0; i < isolated; i++ {
		nodes = append(nodes, core.SymbolNode{Service: "s", Package: "fields", Name: fmt.Sprintf("F%d", i), Kind: core.KindGoField})
	}
	g := Build(nodes)
	got := TransitiveCallerCounts(g)
	for key := range g.nodes {
		seen := map[string]bool{}
		queue := append([]string(nil), g.callers[key]...)
		for len(queue) > 0 {
			x := queue[0]
			queue = queue[1:]
			if x == key || seen[x] {
				continue
			}
			seen[x] = true
			queue = append(queue, g.callers[x]...)
		}
		if got[key] != len(seen) {
			t.Errorf("count for %s = %d, BFS = %d", key, got[key], len(seen))
		}
	}
}
