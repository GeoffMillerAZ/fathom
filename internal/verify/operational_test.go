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

// A row held back by the fallback AND by an Unknown of another cause is a
// genuine shortfall: the fallback does not explain it, so it stays exit 1.
func TestOperationalRow_OtherUnknownKeepsExit1(t *testing.T) {
	mixed := TaskVerdict{TaskID: "M", Claimed: true, Status: Unknown, Label: GraphFallbackLabel,
		Criteria: []Criterion{
			{Name: "test_reaches", Status: Unknown, Detail: graphFallbackDetail("no Test* statically reaches it", "refused")},
			{Name: "runtime_pass", Status: Unknown, Detail: "no attested-run evidence on this evidence source: verify never runs a test"},
			{Name: "oracle", Status: Unknown, Detail: "the oracle could not be evaluated"},
		}}
	if OperationalRow(mixed) {
		t.Error("an Unknown the fallback does not explain must keep the row non-operational")
	}
	if OperationalShortfall([]Report{{Tasks: []TaskVerdict{mixed}}}, 1, true) {
		t.Error("mixed-cause row: want exit 1, not an operational shortfall")
	}
}
