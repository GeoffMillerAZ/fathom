package doccoverage_test

import (
	"math"
	"testing"

	"github.com/geoffmilleraz/sextant/internal/analysis/doccoverage"
	"github.com/geoffmilleraz/sextant/internal/core"
)

func sym(name string) core.SymbolNode {
	return core.SymbolNode{Name: name, Kind: core.KindGoFunc}
}

func TestCoverage_AllDocumented(t *testing.T) {
	nodes := []core.SymbolNode{sym("Foo"), sym("Bar"), sym("Baz")}
	docs := map[string]bool{"Foo": true, "Bar": true, "Baz": true}
	r := doccoverage.Coverage(nodes, docs)
	if len(r.Uncovered) != 0 {
		t.Errorf("expected 0 uncovered, got %v", r.Uncovered)
	}
	if len(r.Covered) != 3 {
		t.Errorf("expected 3 covered, got %d", len(r.Covered))
	}
	if r.Pct != 100.0 {
		t.Errorf("expected 100%%, got %f", r.Pct)
	}
}

func TestCoverage_NoneDocumented(t *testing.T) {
	nodes := []core.SymbolNode{sym("Foo"), sym("Bar")}
	r := doccoverage.Coverage(nodes, map[string]bool{})
	if len(r.Covered) != 0 {
		t.Errorf("expected 0 covered, got %v", r.Covered)
	}
	if len(r.Uncovered) != 2 {
		t.Errorf("expected 2 uncovered, got %d", len(r.Uncovered))
	}
	if r.Pct != 0.0 {
		t.Errorf("expected 0%%, got %f", r.Pct)
	}
}

func TestCoverage_Partial(t *testing.T) {
	nodes := []core.SymbolNode{sym("A"), sym("B"), sym("C"), sym("D")}
	docs := map[string]bool{"A": true, "C": true}
	r := doccoverage.Coverage(nodes, docs)
	if len(r.Covered) != 2 {
		t.Errorf("expected 2 covered, got %d", len(r.Covered))
	}
	if len(r.Uncovered) != 2 {
		t.Errorf("expected 2 uncovered, got %d", len(r.Uncovered))
	}
	want := 50.0
	if math.Abs(r.Pct-want) > 0.001 {
		t.Errorf("expected 50%%, got %f", r.Pct)
	}
}

func TestCoverage_SkipsUnexported(t *testing.T) {
	nodes := []core.SymbolNode{sym("Exported"), sym("unexported"), sym("_blank")}
	docs := map[string]bool{"Exported": true}
	r := doccoverage.Coverage(nodes, docs)
	// only Exported is in scope
	if len(r.Covered)+len(r.Uncovered) != 1 {
		t.Errorf("expected 1 exported symbol, got covered=%v uncovered=%v", r.Covered, r.Uncovered)
	}
}

func TestCoverage_EmptyNodes(t *testing.T) {
	r := doccoverage.Coverage(nil, map[string]bool{})
	if r.Pct != 100.0 {
		t.Errorf("empty nodes: expected 100%%, got %f", r.Pct)
	}
}

func TestCoverage_Deduplication(t *testing.T) {
	nodes := []core.SymbolNode{sym("Foo"), sym("Foo"), sym("Foo")}
	docs := map[string]bool{"Foo": true}
	r := doccoverage.Coverage(nodes, docs)
	if len(r.Covered) != 1 {
		t.Errorf("expected deduplication: got %v", r.Covered)
	}
}

func TestCoverage_SortedOutput(t *testing.T) {
	nodes := []core.SymbolNode{sym("Zebra"), sym("Apple"), sym("Mango")}
	docs := map[string]bool{}
	r := doccoverage.Coverage(nodes, docs)
	for i := 1; i < len(r.Uncovered); i++ {
		if r.Uncovered[i] < r.Uncovered[i-1] {
			t.Errorf("uncovered not sorted: %v", r.Uncovered)
		}
	}
}

func TestCoverage_Determinism(t *testing.T) {
	nodes := []core.SymbolNode{sym("B"), sym("A"), sym("C")}
	docs := map[string]bool{"B": true}
	var prev doccoverage.Report
	for i := 0; i < 5; i++ {
		r := doccoverage.Coverage(nodes, docs)
		if i > 0 {
			if len(r.Covered) != len(prev.Covered) || len(r.Uncovered) != len(prev.Uncovered) {
				t.Fatalf("non-deterministic at iteration %d", i)
			}
			for j := range r.Covered {
				if r.Covered[j] != prev.Covered[j] {
					t.Fatalf("covered non-deterministic at iteration %d", i)
				}
			}
		}
		prev = r
	}
}
