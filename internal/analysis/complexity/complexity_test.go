package complexity_test

import (
	"math"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/complexity"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// makeNode is a helper to build a SymbolNode for a func/method with given complexity.
func makeNode(name string, cx int) core.SymbolNode {
	return core.SymbolNode{
		Service:    "svc",
		Package:    "pkg",
		Name:       name,
		Kind:       core.KindGoFunc,
		FilePath:   "pkg/foo.go",
		Line:       1,
		Signature:  "func " + name + "()",
		Complexity: cx,
		Tier:       core.TierVerified,
	}
}

func makeMethodNode(name string, cx int) core.SymbolNode {
	n := makeNode(name, cx)
	n.Kind = core.KindGoMethod
	return n
}

func makeTypeNode(name string) core.SymbolNode {
	return core.SymbolNode{
		Service:  "svc",
		Package:  "pkg",
		Name:     name,
		Kind:     core.KindGoType,
		FilePath: "pkg/foo.go",
		Line:     1,
	}
}

// ------------------------------------------------------------------ //
// Report
// ------------------------------------------------------------------ //

func TestReport_Empty(t *testing.T) {
	result := complexity.Report(nil)
	if len(result) != 0 {
		t.Errorf("Report(nil) = %d items, want 0", len(result))
	}
}

func TestReport_OnlyFuncsAndMethods(t *testing.T) {
	nodes := []core.SymbolNode{
		makeNode("Alpha", 5),
		makeNode("Beta", 10),
		makeTypeNode("MyType"), // should be excluded
		makeMethodNode("Foo.Bar", 3),
	}
	result := complexity.Report(nodes)
	// Only func/method nodes should appear (2 funcs + 1 method = 3).
	if len(result) != 3 {
		t.Errorf("Report: got %d results, want 3 (types excluded)", len(result))
	}
}

func TestReport_Sorted(t *testing.T) {
	nodes := []core.SymbolNode{
		makeNode("High", 20),
		makeNode("Low", 1),
		makeNode("Mid", 10),
	}
	result := complexity.Report(nodes)
	if len(result) != 3 {
		t.Fatalf("Report: got %d results, want 3", len(result))
	}
	// Should be sorted ascending by complexity.
	for i := 1; i < len(result); i++ {
		if result[i].Complexity < result[i-1].Complexity {
			t.Errorf("Report not sorted ascending at index %d: %d < %d",
				i, result[i].Complexity, result[i-1].Complexity)
		}
	}
	if result[0].Name != "Low" {
		t.Errorf("first entry: got %q, want %q", result[0].Name, "Low")
	}
	if result[2].Name != "High" {
		t.Errorf("last entry: got %q, want %q", result[2].Name, "High")
	}
}

func TestReport_Fields(t *testing.T) {
	nodes := []core.SymbolNode{makeNode("Func1", 7)}
	result := complexity.Report(nodes)
	if len(result) != 1 {
		t.Fatalf("want 1 result, got %d", len(result))
	}
	fc := result[0]
	if fc.Name != "Func1" {
		t.Errorf("Name = %q, want %q", fc.Name, "Func1")
	}
	if fc.Complexity != 7 {
		t.Errorf("Complexity = %d, want 7", fc.Complexity)
	}
	if fc.FilePath != "pkg/foo.go" {
		t.Errorf("FilePath = %q, want %q", fc.FilePath, "pkg/foo.go")
	}
	if fc.Line != 1 {
		t.Errorf("Line = %d, want 1", fc.Line)
	}
}

// ------------------------------------------------------------------ //
// Hotspots
// ------------------------------------------------------------------ //

func TestHotspots_TopN(t *testing.T) {
	nodes := []core.SymbolNode{
		makeNode("A", 3),
		makeNode("B", 20),
		makeNode("C", 7),
		makeNode("D", 15),
		makeNode("E", 1),
	}
	hot := complexity.Hotspots(nodes, 3)
	if len(hot) != 3 {
		t.Fatalf("Hotspots(nodes, 3): got %d results, want 3", len(hot))
	}
	// Should be sorted descending by complexity.
	for i := 1; i < len(hot); i++ {
		if hot[i].Complexity > hot[i-1].Complexity {
			t.Errorf("Hotspots not sorted descending at index %d", i)
		}
	}
	if hot[0].Name != "B" {
		t.Errorf("first hotspot: got %q, want %q", hot[0].Name, "B")
	}
	if hot[1].Name != "D" {
		t.Errorf("second hotspot: got %q, want %q", hot[1].Name, "D")
	}
	if hot[2].Name != "C" {
		t.Errorf("third hotspot: got %q, want %q", hot[2].Name, "C")
	}
}

func TestHotspots_TopNExceedsLen(t *testing.T) {
	nodes := []core.SymbolNode{makeNode("Only", 5)}
	hot := complexity.Hotspots(nodes, 10)
	if len(hot) != 1 {
		t.Errorf("Hotspots with topN > len: got %d results, want 1", len(hot))
	}
}

func TestHotspots_ZeroN(t *testing.T) {
	nodes := []core.SymbolNode{makeNode("F", 5)}
	hot := complexity.Hotspots(nodes, 0)
	if len(hot) != 0 {
		t.Errorf("Hotspots with topN=0: got %d results, want 0", len(hot))
	}
}

// ------------------------------------------------------------------ //
// MaintainabilityIndex
// ------------------------------------------------------------------ //

// MaintainabilityIndex uses the formula:
//   MI = max(0, (171 - 5.2*ln(avgComplexity) - 0.23*avgComplexity - 16.2*ln(avgLOC)) * 100 / 171)
// where avgComplexity = mean cyclomatic complexity of func/method nodes,
//       avgLOC = mean LOC (estimated as line count of signature, minimum 1).
// Result is clamped to [0, 100].

func TestMaintainabilityIndex_Empty(t *testing.T) {
	mi := complexity.MaintainabilityIndex(nil)
	if mi < 0 || mi > 100 {
		t.Errorf("MaintainabilityIndex(nil) = %f, want in [0,100]", mi)
	}
}

func TestMaintainabilityIndex_Range(t *testing.T) {
	nodes := []core.SymbolNode{
		makeNode("A", 1),
		makeNode("B", 5),
		makeNode("C", 20),
	}
	mi := complexity.MaintainabilityIndex(nodes)
	if mi < 0 || mi > 100 {
		t.Errorf("MaintainabilityIndex = %f, want in [0,100]", mi)
	}
}

func TestMaintainabilityIndex_Deterministic(t *testing.T) {
	nodes := []core.SymbolNode{
		makeNode("A", 3),
		makeNode("B", 7),
	}
	mi1 := complexity.MaintainabilityIndex(nodes)
	mi2 := complexity.MaintainabilityIndex(nodes)
	if math.Abs(mi1-mi2) > 1e-9 {
		t.Errorf("MaintainabilityIndex not deterministic: %f vs %f", mi1, mi2)
	}
}

func TestMaintainabilityIndex_HighComplexityLower(t *testing.T) {
	simple := []core.SymbolNode{makeNode("Simple", 1)}
	complex := []core.SymbolNode{makeNode("Complex", 50)}
	miSimple := complexity.MaintainabilityIndex(simple)
	miComplex := complexity.MaintainabilityIndex(complex)
	if miSimple <= miComplex {
		t.Errorf("simple MI (%f) should be > complex MI (%f)", miSimple, miComplex)
	}
}

func TestMaintainabilityIndex_NonFuncExcluded(t *testing.T) {
	// Pure type nodes should yield the same as empty (or a default).
	noFuncs := []core.SymbolNode{makeTypeNode("T1"), makeTypeNode("T2")}
	mi := complexity.MaintainabilityIndex(noFuncs)
	if mi < 0 || mi > 100 {
		t.Errorf("MaintainabilityIndex(types only) = %f, want in [0,100]", mi)
	}
}
