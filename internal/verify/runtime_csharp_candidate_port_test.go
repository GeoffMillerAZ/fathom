package verify_test

import (
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

func TestRuntimeCandidateSelectionKeepsCSharpAndLegacyBoundaries(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name      string
		node      core.SymbolNode
		wantAsked bool
	}{
		{"Cases.KeepsStock", core.SymbolNode{Name: "Cases.KeepsStock", Kind: core.KindCSharpMethod, Signature: "[Test] public void KeepsStock()"}, true},
		{"TestWithoutAttribute", core.SymbolNode{Name: "TestWithoutAttribute", Kind: core.KindCSharpMethod}, false},
		{"Cases.Other", core.SymbolNode{Name: "Cases.Other", Kind: core.KindCSharpMethod, Signature: "[Other.Test] public void Other()"}, false},
		{"TestOrdinary", core.SymbolNode{Name: "TestOrdinary", Kind: core.KindGoFunc}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			charge := node("Charge", "pay/charge.go", 10, core.TierVerified)
			ev := &runtimeEvidence{fakeEvidence: fakeEvidence{
				symbols: map[string][]core.SymbolNode{"Charge": {charge}, tc.name: {tc.node}},
				reach:   map[string]bool{tc.name + "->Charge": true},
			}, now: now, runs: map[string]verify.AttestedRun{
				tc.name: {Test: tc.name, Status: "pass", At: now},
			}}
			row := verify.Task{ID: "R-candidate", Expect: verify.Expect{Symbol: "Charge", ReachableFrom: tc.name}}
			v := verify.VerifyPlan(verify.Plan{Tasks: []verify.Task{row}}, ev).Tasks[0]
			c, found := findCrit(v, "runtime_pass")
			want := verify.Unknown
			if tc.wantAsked {
				want = verify.Pass
			}
			if !found || c.Status != want || (len(ev.asked) > 0) != tc.wantAsked {
				t.Fatalf("candidate selection fabricated or lost runtime query: criterion=%+v asked=%v", c, ev.asked)
			}
		})
	}
}
