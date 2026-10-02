package langbench

import (
	"strings"
	"testing"
)

// TestComparePortParityD1 (ADR-048 amendment 2026-10-01, D1 "dispatch at
// parity", LB-F118): a port merges when every metric measured on both sides,
// at the levels up to the dialect's demonstrated level_met, is at least the
// incumbent's and no SourceKind is lost without a stated reason, whatever the
// target level. A gated metric the incumbent measured and the dialect did not,
// an incumbent that meets a higher level, and a dialect that meets none refuse.
func TestComparePortParityD1(t *testing.T) {
	mk := func(level string, m map[string]*float64) Reading {
		return Reading{Dialect: "x", Kind: "language", LevelMet: level, Metrics: m}
	}
	inc := mk("L1", map[string]*float64{MSymRecall: f(0.90), MSymPrecision: f(0.90), MRangeAgreement: f(0.80), MCallRecall: f(0.90)})
	better := mk("L2", map[string]*float64{MSymRecall: f(0.95), MSymPrecision: f(0.90), MRangeAgreement: f(0.85), MCallRecall: f(0.95), MCallPrecision: f(0.95)})

	// The target is reported, never a reason: L2 of an L4 target merges.
	p := ComparePortParity(inc, better, "old", "L4", map[string]int{"K": 1}, map[string]int{"K": 2, "N": 1}, nil)
	if !p.Merge || len(p.Reasons) != 0 || p.Target != "L4" || len(p.KindsGained) != 1 {
		t.Fatalf("better dialect below its target refused: %+v", p)
	}
	gated := map[string]bool{}
	for _, m := range p.Metrics {
		gated[m.Metric] = m.Gated
	}
	if !gated[MSymRecall] || !gated[MCallRecall] || !gated[MCallPrecision] || gated[MImportResolution] {
		t.Errorf("gated = %v, want L1 and L2 metrics (the dialect's level_met) and nothing above", gated)
	}

	// A metric of the dialect's demonstrated level below the incumbent's refuses,
	// an L2 metric included once the dialect meets L2.
	worseCalls := mk("L2", map[string]*float64{MSymRecall: f(0.95), MSymPrecision: f(0.90), MRangeAgreement: f(0.85), MCallRecall: f(0.80), MCallPrecision: f(0.95)})
	if p := ComparePortParity(inc, worseCalls, "old", "L4", nil, nil, nil); p.Merge || len(p.Reasons) != 1 || !strings.Contains(p.Reasons[0], MCallRecall) {
		t.Fatalf("worse call_recall at the dialect's level merged: %+v", p)
	}
	worseL1 := mk("L1", map[string]*float64{MSymRecall: f(0.89), MSymPrecision: f(0.90), MRangeAgreement: f(0.85), MCallRecall: f(0.10)})
	if p := ComparePortParity(inc, worseL1, "old", "L4", nil, nil, nil); p.Merge || len(p.Reasons) != 1 || !strings.Contains(p.Reasons[0], MSymRecall) {
		t.Fatalf("worse sym_recall merged, or an L2 metric above an L1 dialect gated: %+v", p)
	}
	unmeasured := mk("L1", map[string]*float64{MSymRecall: f(0.95), MSymPrecision: f(0.95)})
	if p := ComparePortParity(inc, unmeasured, "old", "L1", nil, nil, nil); p.Merge {
		t.Fatalf("a gated metric the incumbent alone measured merged: %+v", p)
	}

	// An incumbent at a higher level, and a dialect at no level, refuse.
	incL2 := mk("L2", map[string]*float64{MSymRecall: f(0.90), MSymPrecision: f(0.90)})
	dialL1 := mk("L1", map[string]*float64{MSymRecall: f(0.95), MSymPrecision: f(0.95)})
	if p := ComparePortParity(incL2, dialL1, "old", "L4", nil, nil, nil); p.Merge || !strings.Contains(strings.Join(p.Reasons, ";"), "incumbent meets L2") {
		t.Fatalf("a dialect below the incumbent's level merged: %+v", p)
	}
	if p := ComparePortParity(inc, mk(LevelNone, map[string]*float64{MSymRecall: f(0.99)}), "old", "L4", nil, nil, nil); p.Merge {
		t.Fatalf("a dialect that meets no level merged: %+v", p)
	}

	// A lost SourceKind refuses unless a reason names it.
	if p := ComparePortParity(inc, better, "old", "L4", map[string]int{"Gone": 1, "K": 1}, map[string]int{"K": 1}, nil); p.Merge || p.KindsLost[0] != "Gone" {
		t.Fatalf("an unexplained lost kind merged: %+v", p)
	}
	p = ComparePortParity(inc, better, "old", "L4", map[string]int{"Gone": 1, "K": 1}, map[string]int{"K": 1}, map[string]string{"Gone": "renamed to K"})
	if !p.Merge || p.KindsExplained["Gone"] != "renamed to K" {
		t.Fatalf("an explained lost kind refused: %+v", p)
	}
}
