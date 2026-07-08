package verify_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

func TestParseMarkdownPlan(t *testing.T) {
	md := []byte(`# My plan

Some prose that is not a task.

- [x] Implement token validation [US-042] {symbol: ValidateToken, reachable_from: Router, min_tier: verified}
- [ ] Refund endpoint {symbol: RefundHandler, skip_test: true}
* [-] Deferred thing
- [x] Just a vibe with no contract
- [x] Covered spec only [EP-013]
`)
	plan := verify.ParseMarkdownPlan("svc", md)

	if plan.Service != "svc" {
		t.Fatalf("service = %q", plan.Service)
	}
	if len(plan.Tasks) != 5 {
		t.Fatalf("want 5 tasks, got %d: %+v", len(plan.Tasks), plan.Tasks)
	}

	// IDs are deterministic + zero-padded in document order.
	if plan.Tasks[0].ID != "md-001" || plan.Tasks[4].ID != "md-005" {
		t.Fatalf("unexpected IDs: %s … %s", plan.Tasks[0].ID, plan.Tasks[4].ID)
	}

	// Task 1: symbol + reachable_from + spec id; annotation stripped from title.
	t1 := plan.Tasks[0]
	if t1.Expect.Symbol != "ValidateToken" || t1.Expect.ReachableFrom != "Router" {
		t.Fatalf("t1 expect = %+v", t1.Expect)
	}
	if t1.Expect.MinTier != core.TierVerified {
		t.Fatalf("t1 min_tier = %q", t1.Expect.MinTier)
	}
	if len(t1.SpecIDs) != 1 || t1.SpecIDs[0] != "[US-042]" {
		t.Fatalf("t1 spec_ids = %v", t1.SpecIDs)
	}
	if t1.Title != "Implement token validation [US-042]" {
		t.Fatalf("t1 title = %q", t1.Title)
	}

	// Task 2: skip_test parsed.
	if !plan.Tasks[1].Expect.SkipTest || plan.Tasks[1].Expect.Symbol != "RefundHandler" {
		t.Fatalf("t2 expect = %+v", plan.Tasks[1].Expect)
	}

	// Task 4: no contract → no symbol, no specs.
	if plan.Tasks[3].Expect.Symbol != "" || len(plan.Tasks[3].SpecIDs) != 0 {
		t.Fatalf("t4 should have no contract: %+v", plan.Tasks[3])
	}

	// Task 5: spec only.
	if len(plan.Tasks[4].SpecIDs) != 1 || plan.Tasks[4].SpecIDs[0] != "[EP-013]" {
		t.Fatalf("t5 spec_ids = %v", plan.Tasks[4].SpecIDs)
	}
}

// TestParseMarkdownPlan_CheckboxSemantics: [ ] open, [x]/[X] claimed, [-] skipped.
func TestParseMarkdownPlan_CheckboxSemantics(t *testing.T) {
	md := []byte("- [ ] open task\n- [x] lower x claimed\n- [X] upper X claimed\n- [-] skipped task\n")
	plan := verify.ParseMarkdownPlan("svc", md)
	if len(plan.Tasks) != 4 {
		t.Fatalf("want 4 tasks, got %d: %+v", len(plan.Tasks), plan.Tasks)
	}

	open := plan.Tasks[0]
	if open.Claimed || open.Skipped {
		t.Errorf("[ ] task: Claimed=%v Skipped=%v, want both false", open.Claimed, open.Skipped)
	}

	lowerX := plan.Tasks[1]
	if !lowerX.Claimed || lowerX.Skipped {
		t.Errorf("[x] task: Claimed=%v Skipped=%v, want Claimed=true Skipped=false", lowerX.Claimed, lowerX.Skipped)
	}

	upperX := plan.Tasks[2]
	if !upperX.Claimed || upperX.Skipped {
		t.Errorf("[X] task: Claimed=%v Skipped=%v, want Claimed=true Skipped=false", upperX.Claimed, upperX.Skipped)
	}

	skipped := plan.Tasks[3]
	if skipped.Claimed || !skipped.Skipped {
		t.Errorf("[-] task: Claimed=%v Skipped=%v, want Claimed=false Skipped=true", skipped.Claimed, skipped.Skipped)
	}
}

// TestParseMarkdownPlan_EndToEnd: a parsed markdown plan verifies correctly.
func TestParseMarkdownPlan_EndToEnd(t *testing.T) {
	md := []byte("- [x] do it {symbol: Foo}\n- [x] ghost {symbol: Ghost}\n")
	plan := verify.ParseMarkdownPlan("svc", md)
	ev := fakeEvidence{
		symbols: map[string][]core.SymbolNode{"Foo": {node("Foo", "a.go", 1, core.TierVerified)}},
		tests:   map[string][]string{"Foo": {"t"}},
	}
	rep := verify.VerifyPlan(plan, ev)
	if rep.Summary.Total != 2 {
		t.Fatalf("total = %d", rep.Summary.Total)
	}
	byID := map[string]verify.TaskVerdict{}
	for _, tv := range rep.Tasks {
		byID[tv.TaskID] = tv
	}
	if byID["md-001"].Label != "verified" {
		t.Fatalf("md-001 = %s", byID["md-001"].Label)
	}
	if byID["md-002"].Label != "claimed-but-absent" {
		t.Fatalf("md-002 = %s", byID["md-002"].Label)
	}
}
