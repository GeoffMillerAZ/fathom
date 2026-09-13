package review

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/coverage"
	"github.com/geoffmilleraz/fathom/internal/core"
)

func covFn(name, file string, line, endLine int) core.SymbolNode {
	return core.SymbolNode{
		Service: "fathom", Package: "widget", Name: name, Kind: core.KindGoFunc,
		FilePath: file, Line: line, EndLine: endLine,
	}
}

func TestDimTestCoverage_UncheckedWhenNoProfileSupplied(t *testing.T) {
	d := DimTestCoverage(Input{ChangedFiles: []string{"internal/widget/widget.go"}})
	if d.Checked {
		t.Fatal("expected Checked=false when no coverage profile was supplied")
	}
	if d.Reason != "no coverage profile supplied" {
		t.Errorf("Reason = %q", d.Reason)
	}
	if len(d.Coverage) != 0 || len(d.Findings) != 0 {
		t.Errorf("unchecked dimension must report zero coverage/findings, got %+v", d)
	}
}

func TestDimTestCoverage_CoveredAndUncoveredChangedSymbols(t *testing.T) {
	covered := covFn("Covered", "internal/widget/widget.go", 10, 14)
	uncovered := covFn("Uncovered", "internal/widget/widget.go", 20, 24)
	in := Input{
		ChangedFiles:    []string{"internal/widget/widget.go"},
		Nodes:           []core.SymbolNode{covered, uncovered},
		CoverageChecked: true,
		CoverageLayered: []coverage.SymbolLayerCoverage{
			{
				Symbol: covered.Key(), File: covered.FilePath, Line: covered.Line,
				ByLayer: map[string]coverage.LayerStat{"unit": {Statements: 4, Covered: 4, Percent: 100}},
				Any:     coverage.LayerStat{Statements: 4, Covered: 4, Percent: 100},
			},
			{
				Symbol: uncovered.Key(), File: uncovered.FilePath, Line: uncovered.Line,
				ByLayer: map[string]coverage.LayerStat{"unit": {Statements: 3, Covered: 0, Percent: 0}},
				Any:     coverage.LayerStat{Statements: 3, Covered: 0, Percent: 0},
			},
		},
	}
	d := DimTestCoverage(in)
	if !d.Checked {
		t.Fatalf("expected Checked=true, reason=%q", d.Reason)
	}
	if len(d.Coverage) != 2 {
		t.Fatalf("Coverage = %+v, want 2 entries", d.Coverage)
	}
	// Sorted by (File, Line): Covered (10) before Uncovered (20).
	if d.Coverage[0].Symbol != covered.Key() || d.Coverage[0].Uncovered {
		t.Errorf("Coverage[0] = %+v", d.Coverage[0])
	}
	if len(d.Coverage[0].Layers) != 1 || d.Coverage[0].Layers[0] != "unit" {
		t.Errorf("Coverage[0].Layers = %v, want [unit]", d.Coverage[0].Layers)
	}
	if d.Coverage[1].Symbol != uncovered.Key() || !d.Coverage[1].Uncovered {
		t.Errorf("Coverage[1] = %+v", d.Coverage[1])
	}
	if len(d.Coverage[1].UncoveredAt) != 1 || d.Coverage[1].UncoveredAt[0] != "unit" {
		t.Errorf("Coverage[1].UncoveredAt = %v, want [unit]", d.Coverage[1].UncoveredAt)
	}
	// Only the uncovered symbol produces an active finding.
	if len(d.Findings) != 1 || d.Findings[0].Symbol != uncovered.Key() {
		t.Fatalf("Findings = %+v, want exactly one for the uncovered symbol", d.Findings)
	}
	if d.Count != 1 {
		t.Errorf("Count = %d, want 1", d.Count)
	}
}

func TestDimTestCoverage_TwoLayersOneCoveringOneNot(t *testing.T) {
	// The real "by layer" scenario: a symbol covered by integration but
	// not unit — Layers names the layer that covers it, UncoveredAt names
	// the one that doesn't, and the overall verdict (Any) is covered.
	sym := covFn("OnlyIntegration", "internal/widget/widget.go", 10, 14)
	in := Input{
		ChangedFiles:    []string{"internal/widget/widget.go"},
		Nodes:           []core.SymbolNode{sym},
		CoverageChecked: true,
		CoverageLayered: []coverage.SymbolLayerCoverage{
			{
				Symbol: sym.Key(), File: sym.FilePath, Line: sym.Line,
				ByLayer: map[string]coverage.LayerStat{
					"unit":        {Statements: 2, Covered: 0, Percent: 0},
					"integration": {Statements: 2, Covered: 2, Percent: 100},
				},
				Any: coverage.LayerStat{Statements: 2, Covered: 2, Percent: 100},
			},
		},
	}
	d := DimTestCoverage(in)
	if len(d.Coverage) != 1 {
		t.Fatalf("Coverage = %+v", d.Coverage)
	}
	got := d.Coverage[0]
	if len(got.Layers) != 1 || got.Layers[0] != "integration" {
		t.Errorf("Layers = %v, want [integration]", got.Layers)
	}
	if len(got.UncoveredAt) != 1 || got.UncoveredAt[0] != "unit" {
		t.Errorf("UncoveredAt = %v, want [unit]", got.UncoveredAt)
	}
	if got.Uncovered {
		t.Error("Uncovered should be false — the union (Any) is covered by integration")
	}
	// Covered by at least one layer: no active finding.
	if len(d.Findings) != 0 {
		t.Errorf("Findings = %+v, want none (covered by the union)", d.Findings)
	}
}

func TestDimTestCoverage_OnlyChangedSymbolsReported(t *testing.T) {
	changed := covFn("Changed", "internal/widget/widget.go", 10, 14)
	untouched := covFn("Untouched", "internal/widget/other.go", 10, 14)
	in := Input{
		ChangedFiles:    []string{"internal/widget/widget.go"},
		Nodes:           []core.SymbolNode{changed, untouched},
		CoverageChecked: true,
		CoverageLayered: []coverage.SymbolLayerCoverage{
			{Symbol: changed.Key(), File: changed.FilePath, Line: changed.Line, ByLayer: map[string]coverage.LayerStat{"unit": {Statements: 1, Covered: 1, Percent: 100}}, Any: coverage.LayerStat{Statements: 1, Covered: 1, Percent: 100}},
			{Symbol: untouched.Key(), File: untouched.FilePath, Line: untouched.Line, ByLayer: map[string]coverage.LayerStat{"unit": {Statements: 1, Covered: 0}}, Any: coverage.LayerStat{Statements: 1, Covered: 0}},
		},
	}
	d := DimTestCoverage(in)
	if len(d.Coverage) != 1 || d.Coverage[0].Symbol != changed.Key() {
		t.Fatalf("expected only the changed-file symbol, got %+v", d.Coverage)
	}
}

func TestDimTestCoverage_SymbolWithNoProfileDataOmitted(t *testing.T) {
	changed := covFn("Changed", "internal/widget/widget.go", 10, 14)
	in := Input{
		ChangedFiles:    []string{"internal/widget/widget.go"},
		Nodes:           []core.SymbolNode{changed},
		CoverageChecked: true,
		CoverageLayered: nil, // profile has no data for this symbol at all
	}
	d := DimTestCoverage(in)
	if !d.Checked {
		t.Fatal("expected Checked=true (a profile WAS supplied)")
	}
	if len(d.Coverage) != 0 || len(d.Findings) != 0 {
		t.Errorf("a symbol with no profile data must be omitted, not fabricated as 0%%: %+v", d)
	}
}

func TestResolveCoverage_JoinsLayeredProfiles(t *testing.T) {
	fn := covFn("Widget", "internal/widget/widget.go", 10, 14)
	unit := coverage.LabeledProfile{Layer: "unit", Profile: coverage.Profile{Mode: coverage.ModeSet, Blocks: []coverage.Block{
		{ImportPath: "github.com/geoffmilleraz/fathom/internal/widget/widget.go", StartLine: 10, EndLine: 12, NumStmt: 2, Count: 1},
	}}}
	integration := coverage.LabeledProfile{Layer: "integration", Profile: coverage.Profile{Mode: coverage.ModeSet, Blocks: []coverage.Block{
		{ImportPath: "github.com/geoffmilleraz/fathom/internal/widget/widget.go", StartLine: 10, EndLine: 12, NumStmt: 2, Count: 0},
	}}}
	syms := ResolveCoverage([]coverage.LabeledProfile{unit, integration}, []core.SymbolNode{fn})
	if len(syms) != 1 || syms[0].Symbol != fn.Key() {
		t.Fatalf("ResolveCoverage symbols = %+v", syms)
	}
	if syms[0].ByLayer["unit"].Covered == 0 {
		t.Errorf("unit should cover this symbol: %+v", syms[0].ByLayer)
	}
	if syms[0].ByLayer["integration"].Covered != 0 {
		t.Errorf("integration should NOT cover this symbol: %+v", syms[0].ByLayer)
	}
}
