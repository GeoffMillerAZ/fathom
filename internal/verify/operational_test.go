package verify

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// fallbackEvidence is a stubEvidence-style source that ran on the syntax
// graph: nothing reaches, and it says why.
type fallbackEvidence struct {
	cause string
}

func (fallbackEvidence) FindSymbol(string) []core.SymbolNode {
	return []core.SymbolNode{{Name: "Run", FilePath: "work/run.go", Line: 1, Kind: core.KindGoFunc, Tier: core.TierVerified}}
}
func (fallbackEvidence) TestReaches(string) []string          { return nil }
func (fallbackEvidence) Reachable(string, string) bool        { return false }
func (fallbackEvidence) Contradictions() []core.Contradiction { return nil }
func (fallbackEvidence) SpecImplementors(string) []string     { return nil }
func (e fallbackEvidence) GraphFallback() string              { return e.cause }

func TestGraphFallback_ReachCriteriaReadUnknownNotFail(t *testing.T) {
	plan := Plan{Service: "svc", Tasks: []Task{{ID: "R-1", Claimed: true,
		Expect: Expect{Symbol: "Run", ReachableFrom: "Main"}}}}
	rep := VerifyPlan(plan, fallbackEvidence{cause: "typed edges: refused"})
	tv := rep.Tasks[0]
	if tv.Status != Unknown || tv.Label != GraphFallbackLabel {
		t.Fatalf("status %s label %s, want Unknown graph-fallback", tv.Status, tv.Label)
	}
	for _, c := range tv.Criteria {
		if c.Status == Fail {
			t.Errorf("criterion %s failed under the fallback: %s", c.Name, c.Detail)
		}
	}
	if !OperationalRow(tv) {
		t.Error("graph-fallback row must be operational")
	}
	agg, ok := GraphFallbackOf([]Report{rep})
	if !ok || agg.Rows != 1 || agg.Reason != "typed edges: refused" || agg.Remedy != GraphFallbackRemedy {
		t.Errorf("agg = %+v ok=%v", agg, ok)
	}
	if !OperationalShortfall([]Report{rep}, 1, true) {
		t.Error("only graph-fallback rows unproven: want operational shortfall")
	}
	if !strings.Contains(GraphFallbackLine(agg), "typed edges: refused") {
		t.Error("line must name the cause")
	}
}

func TestGraphFallback_NoCauseStillFails(t *testing.T) {
	plan := Plan{Service: "svc", Tasks: []Task{{ID: "R-1", Claimed: true,
		Expect: Expect{Symbol: "Run", ReachableFrom: "Main"}}}}
	tv := VerifyPlan(plan, fallbackEvidence{}).Tasks[0]
	if tv.Status != Fail || OperationalRow(tv) {
		t.Fatalf("typed graph in use: status %s operational %v, want Fail", tv.Status, OperationalRow(tv))
	}
}

func TestOperationalShortfall_ElidedAndMixed(t *testing.T) {
	elided := TaskVerdict{TaskID: "E", Claimed: true, Status: Unknown, Label: VerdictElidedLabel,
		Criteria: []Criterion{{Name: "verdict", Status: Unknown, Detail: "never arrived"}}}
	failing := TaskVerdict{TaskID: "F", Claimed: true, Status: Fail, Label: "claimed-but-absent",
		Criteria: []Criterion{{Name: "symbol_present", Status: Fail, Detail: "x"}}}
	if !OperationalShortfall([]Report{{Tasks: []TaskVerdict{elided}}}, 1, true) {
		t.Error("only verdict-elided rows unproven: want operational")
	}
	if OperationalShortfall([]Report{{Tasks: []TaskVerdict{elided, failing}}}, 2, true) {
		t.Error("a genuine failing row keeps the gate at exit 1")
	}
}
