package verify_test

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// O1453: a bare anchor that two packages define fails symbol_present naming
// both and asking for package:, and test_reaches and edge_wired are n/a
// rather than a false "untested".
func TestBareAnchorDefinedTwiceIsAmbiguous(t *testing.T) {
	rep := verify.VerifyPlan(verify.Plan{Tasks: []verify.Task{
		{ID: "T-1", Expect: verify.Expect{Symbol: "Run", ReachableFrom: "Router"}},
	}}, ambiguousRunEvidence())
	v := rep.Tasks[0]
	if v.Status != verify.Fail || v.Label != "ambiguous-anchor" {
		t.Fatalf("got %s/%s, want fail/ambiguous-anchor; criteria=%+v", v.Status, v.Label, v.Criteria)
	}
	c, _ := findCrit(v, "symbol_present")
	if c.Status != verify.Fail || !strings.Contains(c.Detail, "ambiguous: 2 definitions (") ||
		!strings.Contains(c.Detail, "ladder") || !strings.Contains(c.Detail, "harness") || !strings.HasSuffix(c.Detail, "add package:") {
		t.Fatalf("symbol_present = %+v", c)
	}
	for _, n := range []string{"test_reaches", "edge_wired"} {
		if c, ok := findCrit(v, n); !ok || c.Status != verify.NA {
			t.Fatalf("%s = %+v, want n/a", n, c)
		}
	}
}

// A bare name two packages define that the call graph still resolves by its
// bare name keeps passing: the row is not made to fail by this check.
func TestBareAnchorDefinedTwiceStillPassesWhenGraphResolvesIt(t *testing.T) {
	ev := ambiguousRunEvidence()
	ev.tests["Run"] = []string{"svc::foundations/ladder/ladder_test.TestRun"}
	rep := verify.VerifyPlan(verify.Plan{Tasks: []verify.Task{
		{ID: "T-1", Expect: verify.Expect{Symbol: "Run"}},
	}}, ev)
	if v := rep.Tasks[0]; v.Status != verify.Pass {
		t.Fatalf("got %s/%s, want pass; criteria=%+v", v.Status, v.Label, v.Criteria)
	}
}
