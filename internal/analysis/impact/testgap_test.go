package impact_test

import (
	"encoding/json"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/impact"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// gapFixture: a.go defines Covered (reached by TestCovered via Mid) and
// Naked (no test reaches it); Mid reaches Covered through a typed edge.
func gapFixture() ([]core.SymbolNode, []core.Edge) {
	n := func(name, file string, line int, calls ...string) core.SymbolNode {
		return core.SymbolNode{Service: "svc", Package: "pkg", Name: name, Kind: core.KindGoFunc, FilePath: file, Line: line, Calls: calls}
	}
	nodes := []core.SymbolNode{
		n("Covered", "a.go", 3),
		n("Naked", "a.go", 9),
		n("Mid", "b.go", 3, "svc::pkg.Covered"),
		n("TestCovered", "b_test.go", 5, "svc::pkg.Mid"),
	}
	typed := []core.Edge{{FromService: "svc", FromSymbol: "svc::pkg.Mid", ToSymbol: "svc::pkg.Covered", Kind: core.EdgeDirect}}
	return nodes, typed
}

func TestTestGap(t *testing.T) {
	nodes, typed := gapFixture()
	got := impact.TestGap(nodes, []string{"a.go"}, typed)
	if !got.Checked {
		t.Fatalf("Checked = false with indexed nodes: %+v", got)
	}
	if len(got.AffectedTests) != 1 {
		t.Fatalf("AffectedTests = %+v, want TestCovered", got.AffectedTests)
	}
	at := got.AffectedTests[0]
	if at.Test != "svc::pkg.TestCovered" || at.File != "b_test.go" || at.Line != 5 || at.Citation != "b_test.go#L5-L5" {
		t.Errorf("affected test = %+v: want key, file:line and citation", at)
	}
	if len(at.Reaches) != 1 || at.Reaches[0] != "svc::pkg.Covered" {
		t.Errorf("Reaches = %v, want [svc::pkg.Covered]", at.Reaches)
	}
	if len(got.Gap) != 1 || got.Gap[0].Symbol != "svc::pkg.Naked" || got.Gap[0].Citation != "a.go#L9-L9" {
		t.Errorf("Gap = %+v, want Naked with its citation", got.Gap)
	}

	// Payload: the ONE JSON shape both review_diff and `fathom review --json`
	// emit; arrays never null.
	p := got.Payload()
	if p["checked"] != true || len(p["affected_tests"].([]any)) != 1 || len(p["test_gap"].([]any)) != 1 {
		t.Errorf("Payload = %v", p)
	}
	empty := impact.TestGap(nil, []string{"a.go"}, nil)
	if empty.Checked || empty.Reason == "" {
		t.Errorf("no nodes must be checked:false with a reason, got %+v", empty)
	}
	b, _ := json.Marshal(empty.Payload())
	if string(b) == "" || !json.Valid(b) || !containsAll(string(b), `"affected_tests":[]`, `"test_gap":[]`, `"checked":false`) {
		t.Errorf("empty payload = %s", b)
	}

	// Determinism.
	a1, _ := json.Marshal(got.Payload())
	a2, _ := json.Marshal(impact.TestGap(nodes, []string{"a.go"}, typed).Payload())
	if string(a1) != string(a2) {
		t.Fatalf("TestGap not deterministic:\n%s\n%s", a1, a2)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// TestTestGapSeeds pins TQ-21's seam: a symbol named by key counts as
// changed though no changed file defines it (the function reading a
// //go:embed variable whose embedded file changed); an unknown key and a
// test key are ignored.
func TestTestGapSeeds(t *testing.T) {
	nodes, typed := gapFixture()
	got := impact.TestGapSeeds(nodes, []string{"README.md"}, typed, []string{"svc::pkg.Mid", "svc::pkg.Gone", "svc::pkg.TestCovered"})
	if len(got.AffectedTests) != 1 || got.AffectedTests[0].Test != "svc::pkg.TestCovered" {
		t.Fatalf("AffectedTests = %+v, want TestCovered through the seeded Mid", got.AffectedTests)
	}
	if r := got.AffectedTests[0].Reaches; len(r) != 1 || r[0] != "svc::pkg.Mid" {
		t.Errorf("Reaches = %v, want only the seed", r)
	}
	if len(got.Gap) != 0 {
		t.Errorf("Gap = %+v, want none", got.Gap)
	}
	if none := impact.TestGapSeeds(nodes, []string{"README.md"}, typed, nil); len(none.AffectedTests) != 0 {
		t.Errorf("no seed selected %+v", none.AffectedTests)
	}
}
