package impact_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/sextant/internal/analysis/impact"
	"github.com/geoffmilleraz/sextant/internal/core"
)

// ─── fixtures ────────────────────────────────────────────────────────────────

// Graph layout:
//
//	TestFoo → Bar → Baz    (TestFoo calls Bar, Bar calls Baz)
//	Qux → Baz              (Qux also calls Baz)
//
// Files:
//
//	"pkg/baz.go"  → Baz
//	"pkg/bar.go"  → Bar
//	"pkg/qux.go"  → Qux
//	"pkg/test.go" → TestFoo
var nodes = []core.SymbolNode{
	{Service: "svc", Package: "pkg", Name: "Baz", FilePath: "pkg/baz.go", Calls: nil},
	{Service: "svc", Package: "pkg", Name: "Bar", FilePath: "pkg/bar.go", Calls: []string{"svc::pkg.Baz"}},
	{Service: "svc", Package: "pkg", Name: "Qux", FilePath: "pkg/qux.go", Calls: []string{"svc::pkg.Baz"}},
	{Service: "svc", Package: "pkg", Name: "TestFoo", FilePath: "pkg/test.go", Calls: []string{"svc::pkg.Bar"}},
}

// ─── BlastRadius ─────────────────────────────────────────────────────────────

func TestBlastRadius_DirectSymbols(t *testing.T) {
	r := impact.BlastRadius(nodes, []string{"pkg/baz.go"})
	// Baz is in baz.go, Bar calls Baz, TestFoo calls Bar, Qux calls Baz.
	// All four should be in Symbols.
	want := []string{
		"svc::pkg.Bar",
		"svc::pkg.Baz",
		"svc::pkg.Qux",
		"svc::pkg.TestFoo",
	}
	if !reflect.DeepEqual(r.Symbols, want) {
		t.Errorf("Symbols = %v, want %v", r.Symbols, want)
	}
}

func TestBlastRadius_Tests(t *testing.T) {
	r := impact.BlastRadius(nodes, []string{"pkg/baz.go"})
	want := []string{"svc::pkg.TestFoo"}
	if !reflect.DeepEqual(r.Tests, want) {
		t.Errorf("Tests = %v, want %v", r.Tests, want)
	}
}

func TestBlastRadius_Files(t *testing.T) {
	r := impact.BlastRadius(nodes, []string{"pkg/baz.go"})
	want := []string{"pkg/bar.go", "pkg/baz.go", "pkg/qux.go", "pkg/test.go"}
	if !reflect.DeepEqual(r.Files, want) {
		t.Errorf("Files = %v, want %v", r.Files, want)
	}
}

func TestBlastRadius_LeafFile(t *testing.T) {
	// Changing TestFoo's file should not cascade further (no callers).
	r := impact.BlastRadius(nodes, []string{"pkg/test.go"})
	want := []string{"svc::pkg.TestFoo"}
	if !reflect.DeepEqual(r.Symbols, want) {
		t.Errorf("Symbols = %v, want %v", r.Symbols, want)
	}
}

func TestBlastRadius_UnknownFile(t *testing.T) {
	r := impact.BlastRadius(nodes, []string{"pkg/unknown.go"})
	if len(r.Symbols) != 0 || len(r.Tests) != 0 || len(r.Files) != 0 {
		t.Errorf("expected empty result for unknown file, got %+v", r)
	}
}

func TestBlastRadius_EmptyNodes(t *testing.T) {
	r := impact.BlastRadius(nil, []string{"pkg/baz.go"})
	if len(r.Symbols) != 0 {
		t.Errorf("expected empty result for nil nodes, got %+v", r)
	}
}

func TestBlastRadius_EmptyFiles(t *testing.T) {
	r := impact.BlastRadius(nodes, nil)
	if len(r.Symbols) != 0 {
		t.Errorf("expected empty result for nil files, got %+v", r)
	}
}

// ─── DiffImpact (alias) ───────────────────────────────────────────────────────

func TestDiffImpact_SameAsBlastRadius(t *testing.T) {
	a := impact.BlastRadius(nodes, []string{"pkg/bar.go"})
	b := impact.DiffImpact(nodes, []string{"pkg/bar.go"})
	if !reflect.DeepEqual(a, b) {
		t.Errorf("DiffImpact differs from BlastRadius: %+v vs %+v", b, a)
	}
}

// ─── Reachable ───────────────────────────────────────────────────────────────

func TestReachable_True(t *testing.T) {
	// baz.go contains Baz; from Bar the path Bar→Baz exists.
	if !impact.Reachable(nodes, []string{"pkg/bar.go"}, "svc::pkg.Baz") {
		t.Error("expected Reachable=true from bar.go to Baz")
	}
}

func TestReachable_False(t *testing.T) {
	// TestFoo calls Bar, but Qux is not reachable from test.go.
	if impact.Reachable(nodes, []string{"pkg/test.go"}, "svc::pkg.Qux") {
		t.Error("expected Reachable=false from test.go to Qux")
	}
}

func TestReachable_EmptyNodes(t *testing.T) {
	if impact.Reachable(nil, []string{"pkg/bar.go"}, "Baz") {
		t.Error("expected false for nil nodes")
	}
}

// ─── AffectedTests ───────────────────────────────────────────────────────────

func TestAffectedTests_Found(t *testing.T) {
	tests := impact.AffectedTests(nodes, []string{"pkg/baz.go"})
	want := []string{"svc::pkg.TestFoo"}
	if !reflect.DeepEqual(tests, want) {
		t.Errorf("AffectedTests = %v, want %v", tests, want)
	}
}

func TestAffectedTests_None(t *testing.T) {
	// Changing qux.go: Qux calls Baz, no test calls Qux.
	tests := impact.AffectedTests(nodes, []string{"pkg/qux.go"})
	if len(tests) != 0 {
		t.Errorf("expected no affected tests for qux.go, got %v", tests)
	}
}

// ─── Determinism ─────────────────────────────────────────────────────────────

func TestBlastRadius_Determinism(t *testing.T) {
	a := impact.BlastRadius(nodes, []string{"pkg/baz.go"})
	b := impact.BlastRadius(nodes, []string{"pkg/baz.go"})
	if !reflect.DeepEqual(a, b) {
		t.Errorf("non-deterministic: %+v vs %+v", a, b)
	}
}
