package verify_test

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// fakeDetailedEvidence extends fakeEvidence with TestReachesDetail, so
// VerifyPlan's DetailedEvidence type-assertion path (ADR-021 §3) can be
// exercised without depending on internal/analysis/callgraph or a real
// store — the three O9 residue shapes (adapter Complete, Store
// contract-suite dispatch, Ping) are reproduced as plain data here.
type fakeDetailedEvidence struct {
	fakeEvidence
	details map[string][]verify.ReachDetail
}

func (f fakeDetailedEvidence) TestReachesDetail(symbol string) []verify.ReachDetail {
	return f.details[symbol]
}

var _ verify.DetailedEvidence = fakeDetailedEvidence{}

func TestVerifyTask_DetailedEvidence_DispatchReach(t *testing.T) {
	ev := fakeDetailedEvidence{
		fakeEvidence: fakeEvidence{
			symbols: map[string][]core.SymbolNode{
				"AdapterA.Complete": {node("AdapterA.Complete", "adapter.go", 10, core.TierVerified)},
			},
		},
		details: map[string][]verify.ReachDetail{
			"AdapterA.Complete": {
				{Key: "TestRunAdapter", ViaDispatch: true, Implementers: 2},
			},
		},
	}
	plan := verify.Plan{Tasks: []verify.Task{{
		ID: "T-1", Title: "adapter residue", Claimed: true,
		Expect: verify.Expect{Symbol: "AdapterA.Complete"},
	}}}
	rep := verify.VerifyPlan(plan, ev)
	if rep.Tasks[0].Status != verify.Pass {
		t.Fatalf("status = %v, want Pass: %+v", rep.Tasks[0].Status, rep.Tasks[0])
	}
	var detail string
	for _, c := range rep.Tasks[0].Criteria {
		if c.Name == "test_reaches" {
			detail = c.Detail
		}
	}
	if !strings.Contains(detail, "reached via interface dispatch") {
		t.Errorf("test_reaches detail = %q, want it to mention interface dispatch", detail)
	}
	if !strings.Contains(detail, "2 implementer") {
		t.Errorf("test_reaches detail = %q, want it to mention 2 implementers", detail)
	}
	if !strings.Contains(detail, "TestRunAdapter") {
		t.Errorf("test_reaches detail = %q, want it to name the reaching test", detail)
	}
}

func TestVerifyTask_DetailedEvidence_FallsBackWhenEmpty(t *testing.T) {
	ev := fakeDetailedEvidence{
		fakeEvidence: fakeEvidence{
			symbols: map[string][]core.SymbolNode{
				"Lonely": {node("Lonely", "f.go", 1, core.TierVerified)},
			},
		},
		details: map[string][]verify.ReachDetail{}, // no test reaches it, even via typed edges
	}
	plan := verify.Plan{Tasks: []verify.Task{{
		ID: "T-1", Title: "untested", Claimed: true,
		Expect: verify.Expect{Symbol: "Lonely"},
	}}}
	rep := verify.VerifyPlan(plan, ev)
	if rep.Tasks[0].Status != verify.Fail || rep.Tasks[0].Label != "present-but-untested" {
		t.Errorf("status/label = %v/%q, want Fail/present-but-untested", rep.Tasks[0].Status, rep.Tasks[0].Label)
	}
}

func TestVerifyTask_DetailedEvidence_NonDispatchReachOmitsModality(t *testing.T) {
	ev := fakeDetailedEvidence{
		fakeEvidence: fakeEvidence{
			symbols: map[string][]core.SymbolNode{
				"Plain": {node("Plain", "f.go", 1, core.TierVerified)},
			},
		},
		details: map[string][]verify.ReachDetail{
			"Plain": {{Key: "TestPlain", ViaDispatch: false}},
		},
	}
	plan := verify.Plan{Tasks: []verify.Task{{
		ID: "T-1", Title: "plain reach", Claimed: true,
		Expect: verify.Expect{Symbol: "Plain"},
	}}}
	rep := verify.VerifyPlan(plan, ev)
	if rep.Tasks[0].Status != verify.Pass {
		t.Fatalf("status = %v, want Pass: %+v", rep.Tasks[0].Status, rep.Tasks[0])
	}
	var detail string
	for _, c := range rep.Tasks[0].Criteria {
		if c.Name == "test_reaches" {
			detail = c.Detail
		}
	}
	if strings.Contains(detail, "dispatch") {
		t.Errorf("test_reaches detail = %q, a non-dispatch reach must not mention dispatch", detail)
	}
	if !strings.Contains(detail, "TestPlain") {
		t.Errorf("test_reaches detail = %q, want it to name the reaching test", detail)
	}
}

// TestVerifyTask_PlainEvidenceUnaffected confirms existing Evidence fakes
// (that do NOT implement DetailedEvidence) are completely unaffected by the
// ADR-021 §3 upgrade — the plain TestReaches path still governs.
func TestVerifyTask_PlainEvidenceUnaffected(t *testing.T) {
	ev := fakeEvidence{
		symbols: map[string][]core.SymbolNode{
			"Foo": {node("Foo", "f.go", 1, core.TierVerified)},
		},
		tests: map[string][]string{"Foo": {"TestFoo"}},
	}
	plan := verify.Plan{Tasks: []verify.Task{{
		ID: "T-1", Title: "plain", Claimed: true,
		Expect: verify.Expect{Symbol: "Foo"},
	}}}
	rep := verify.VerifyPlan(plan, ev)
	if rep.Tasks[0].Status != verify.Pass {
		t.Fatalf("status = %v, want Pass: %+v", rep.Tasks[0].Status, rep.Tasks[0])
	}
}
