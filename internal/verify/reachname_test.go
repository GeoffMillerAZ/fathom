package verify_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// reachNameEvidence is a fakeEvidence that also says how a reach endpoint's
// name resolves (verify.ReachNameEvidence).
type reachNameEvidence struct {
	fakeEvidence
	keys  map[string]string   // name -> the one key it resolves to
	cands map[string][]string // name -> every key carrying it, when it resolves to none
}

func (r reachNameEvidence) ResolveReachName(name string) (string, []string) {
	return r.keys[name], r.cands[name]
}

// TestEdgeWiredNamesAnAmbiguousReachableFrom: a refused edge_wired tells
// the three cases apart. A reachable_from two symbols carry is ambiguous
// (the graph walks from neither), named with up to five candidate keys and
// how to write it; a name nothing carries says so; a name that resolves
// keeps "does not reach". Each stays a failure.
func TestEdgeWiredNamesAnAmbiguousReachableFrom(t *testing.T) {
	sym := map[string][]core.SymbolNode{"Target": {node("Target", "lib/t.go", 3, core.TierVerified)}}
	tests := map[string][]string{"Target": {"svc::lib/lib.TestTarget"}}
	seven := make([]string, 7)
	for i := range seven {
		seven[i] = fmt.Sprintf("svc::p%d/p%d.TestMany", i, i)
	}
	ev := reachNameEvidence{
		fakeEvidence: fakeEvidence{symbols: sym, tests: tests},
		keys:         map[string]string{"TestOne": "svc::lib/lib.TestOne"},
		cands: map[string][]string{
			"TestTwice": {"svc::cmd/app/main.TestTwice", "svc::lib/lib.TestTwice"},
			"TestMany":  seven,
		},
	}
	plan := verify.Plan{Service: "svc", Tasks: []verify.Task{
		{ID: "A", Claimed: true, Expect: verify.Expect{Symbol: "Target", ReachableFrom: "TestTwice"}},
		{ID: "B", Claimed: true, Expect: verify.Expect{Symbol: "Target", ReachableFrom: "TestGhost"}},
		{ID: "C", Claimed: true, Expect: verify.Expect{Symbol: "Target", ReachableFrom: "TestOne"}},
		{ID: "D", Claimed: true, Expect: verify.Expect{Symbol: "Target", ReachableFrom: "TestMany"}},
	}}
	rep := verify.VerifyPlan(plan, ev)
	crit := func(id string) verify.Criterion {
		t.Helper()
		v := verdict(t, rep, id)
		if v.Status == verify.Pass {
			t.Fatalf("%s passed with no reach: %+v", id, v)
		}
		c, ok := findCrit(v, "edge_wired")
		if !ok || c.Status != verify.Fail {
			t.Fatalf("%s: edge_wired = %+v, want a failure", id, c)
		}
		return c
	}

	a := crit("A").Detail
	for _, want := range []string{"ambiguous", "svc::cmd/app/main.TestTwice", "svc::lib/lib.TestTwice", "Recv.Method", "rename"} {
		if !strings.Contains(a, want) {
			t.Errorf("ambiguous detail %q lacks %q", a, want)
		}
	}
	if strings.Contains(a, "does not reach") {
		t.Errorf("ambiguous detail still reads as a missing path: %q", a)
	}

	b := crit("B").Detail
	if !strings.Contains(b, "TestGhost") || !strings.Contains(b, "no symbol") || strings.Contains(b, "does not reach") {
		t.Errorf("unresolved detail %q, want it to say the name resolves to no symbol", b)
	}

	if c := crit("C").Detail; c != "TestOne does not reach Target" {
		t.Errorf("resolved detail = %q, want the missing-path wording", c)
	}

	d := crit("D").Detail
	for i, k := range seven {
		if got := strings.Contains(d, k); got != (i < 5) {
			t.Errorf("detail names %s = %v, want the first five candidates only: %q", k, got, d)
		}
	}
	if !strings.Contains(d, "7 symbols") || !strings.Contains(d, "2 more") {
		t.Errorf("detail %q does not count the candidates it left out", d)
	}
}
