package dataflow_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/sextant/internal/analysis/dataflow"
	"github.com/geoffmilleraz/sextant/internal/core"
)

// fixture: A -> B -> C; D is an orphan; TestFoo -> A.
//
// Key format: "svc::pkg.Name"
func fixture() []core.SymbolNode {
	return []core.SymbolNode{
		{Service: "svc", Package: "pkg", Name: "A", Calls: []string{"svc::pkg.B"}},
		{Service: "svc", Package: "pkg", Name: "B", Calls: []string{"C"}}, // bare name
		{Service: "svc", Package: "pkg", Name: "C", Calls: nil},
		{Service: "svc", Package: "pkg", Name: "D", Calls: nil},
		{Service: "svc", Package: "pkg", Name: "TestFoo", Calls: []string{"svc::pkg.A"}},
	}
}

func TestTrace_DirectCallee(t *testing.T) {
	got := dataflow.Trace(fixture(), "svc::pkg.A")
	// A -> B -> C; B and C are reachable.
	want := []string{"svc::pkg.B", "svc::pkg.C"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Trace(A) = %v, want %v", got, want)
	}
}

func TestTrace_LeafNode(t *testing.T) {
	got := dataflow.Trace(fixture(), "svc::pkg.C")
	if len(got) != 0 {
		t.Errorf("Trace(C) = %v, want empty (C has no callees)", got)
	}
}

func TestTrace_Orphan(t *testing.T) {
	got := dataflow.Trace(fixture(), "svc::pkg.D")
	if len(got) != 0 {
		t.Errorf("Trace(D) = %v, want empty", got)
	}
}

func TestTrace_BareName(t *testing.T) {
	// "A" resolves unambiguously to svc::pkg.A.
	got := dataflow.Trace(fixture(), "A")
	want := []string{"svc::pkg.B", "svc::pkg.C"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Trace(bare A) = %v, want %v", got, want)
	}
}

func TestTrace_UnknownSymbol(t *testing.T) {
	got := dataflow.Trace(fixture(), "DoesNotExist")
	if got != nil {
		t.Errorf("Trace(unknown) = %v, want nil", got)
	}
}

func TestTrace_NilNodes(t *testing.T) {
	got := dataflow.Trace(nil, "A")
	if got != nil {
		t.Errorf("Trace(nil, A) = %v, want nil", got)
	}
}

func TestTrace_Cycle(t *testing.T) {
	// X -> Y -> X: cycle must not cause infinite loop.
	nodes := []core.SymbolNode{
		{Service: "s", Package: "p", Name: "X", Calls: []string{"s::p.Y"}},
		{Service: "s", Package: "p", Name: "Y", Calls: []string{"s::p.X"}},
	}
	got := dataflow.Trace(nodes, "s::p.X")
	want := []string{"s::p.Y"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Trace(X) with cycle = %v, want %v", got, want)
	}
}

func TestTrace_Determinism(t *testing.T) {
	a := dataflow.Trace(fixture(), "svc::pkg.TestFoo")
	b := dataflow.Trace(fixture(), "svc::pkg.TestFoo")
	if !reflect.DeepEqual(a, b) {
		t.Errorf("Trace not deterministic: %v vs %v", a, b)
	}
}

func TestTrace_MultipleCallees(t *testing.T) {
	// Fan-out: Root -> [Alpha, Beta, Gamma].
	nodes := []core.SymbolNode{
		{Service: "s", Package: "p", Name: "Root", Calls: []string{"s::p.Alpha", "s::p.Beta", "s::p.Gamma"}},
		{Service: "s", Package: "p", Name: "Alpha", Calls: nil},
		{Service: "s", Package: "p", Name: "Beta", Calls: nil},
		{Service: "s", Package: "p", Name: "Gamma", Calls: nil},
	}
	got := dataflow.Trace(nodes, "s::p.Root")
	want := []string{"s::p.Alpha", "s::p.Beta", "s::p.Gamma"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Trace(Root) fan-out = %v, want %v", got, want)
	}
}

func TestTrace_StartNotIncluded(t *testing.T) {
	got := dataflow.Trace(fixture(), "svc::pkg.A")
	for _, k := range got {
		if k == "svc::pkg.A" {
			t.Errorf("Trace result must not contain start node, got %v", got)
		}
	}
}
