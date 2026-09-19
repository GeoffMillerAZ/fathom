package verify_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// TestVerifyPlan_BareNameIgnoresDocHeadingCollision (INC-21): a `symbol:`
// contract names code. When a Markdown heading shares the bare name (a
// reading titled "Gate" beside verify.Gate), the doc chunk is dropped from
// the match set and the single remaining code key — not the ambiguous bare
// name — is what reachability is asked about, so the row verifies the same
// on every store regardless of which docs happen to be indexed.
func TestVerifyPlan_BareNameIgnoresDocHeadingCollision(t *testing.T) {
	code := core.SymbolNode{Service: "svc", Name: "Gate", Kind: core.KindGoFunc, Package: "verify", FilePath: "internal/verify/verify.go", Line: 203, Tier: core.TierVerified}
	doc := core.SymbolNode{Service: "svc", Name: "Gate", Kind: core.KindDocChunk, FilePath: "docs/bench/readings/claims.md", Line: 9, Tier: core.TierDocClaimed}
	ev := fakeEvidence{
		symbols: map[string][]core.SymbolNode{"Gate": {doc, code}},
		// Only the canonical code key is reachable; the bare name resolves
		// to nothing in a graph where two nodes share it.
		tests: map[string][]string{code.Key(): {"svc::internal/verify.TestGate_Pass"}},
	}
	plan := verify.Plan{Service: "svc", Tasks: []verify.Task{
		{ID: "md-002", Title: "Done-means-proven gate function", Claimed: true, Expect: verify.Expect{Symbol: "Gate"}},
	}}
	rep := verify.VerifyPlan(plan, ev)
	v := rep.Tasks[0]
	if v.Status != verify.Pass {
		t.Fatalf("want Pass, got %s (%s): %+v", v.Status, v.Label, v.Criteria)
	}
	if v.EvidenceFile != code.FilePath {
		t.Fatalf("symbol_present cited %q, want the code definition %q", v.EvidenceFile, code.FilePath)
	}
	if c, ok := findCrit(v, "test_reaches"); !ok || c.Status != verify.Pass {
		t.Fatalf("test_reaches must pass through the canonical key, got %+v", c)
	}
}
