package verify_test

import (
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

func TestRuntimePassNativeRefusalRemainsNamedAndInformational(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	sym := node("Charge", "pay/charge.go", 10, core.TierVerified)
	base := fakeEvidence{
		symbols: map[string][]core.SymbolNode{"Charge": {sym}},
		tests:   map[string][]string{"Charge": {"svc::pay.TestCharge"}},
		reach:   map[string]bool{"TestCharge->Charge": true},
	}
	row := verify.Task{ID: "R-native", Claimed: true, Expect: verify.Expect{Symbol: "Charge", ReachableFrom: "TestCharge"}}
	for _, reason := range []string{"", "native active receipt changed; previous result is not current"} {
		ev := &runtimeEvidence{fakeEvidence: base, now: now, runs: map[string]verify.AttestedRun{
			"TestCharge": {Test: "svc::pay.TestCharge", Status: "pass", At: now, UnknownReason: reason},
		}}
		v := verify.VerifyPlan(verify.Plan{Tasks: []verify.Task{row}}, ev).Tasks[0]
		c, found := findCrit(v, "runtime_pass")
		want := verify.Pass
		if reason != "" {
			want = verify.Unknown
		}
		if !found || c.Status != want || (reason != "" && c.Detail != reason) {
			t.Fatalf("native refusal lost or promoted: %+v, want %s / %q", c, want, reason)
		}
		if v.Status != verify.Pass || !verify.Gate(verify.Report{Tasks: []verify.TaskVerdict{v}}, true).Pass {
			t.Fatalf("informational native observation changed the static gate: %+v", v)
		}
	}
}
