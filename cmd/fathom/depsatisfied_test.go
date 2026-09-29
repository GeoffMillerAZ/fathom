package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// TestDepSatisfiedDeferredBlocksAndRefusedNamesReplan pins G2-6: deferred is
// not done. A needs: target that is a deferred [-] row keeps its dependent
// blocked until the target revives; a refused target (or any disposition
// other than deferred — a superseded one, should the grammar gain it) never
// will be built, so its dependent is named as needing re-planning. Neither
// is ever silently eligible — in the same plan or across plans.
func TestDepSatisfiedDeferredBlocksAndRefusedNamesReplan(t *testing.T) {
	deferred := planRow{Task: verify.Task{ID: "D-1", Skipped: true, Disposition: "deferred", Reason: "later", Trigger: "when X"}}
	refused := planRow{Task: verify.Task{ID: "R-1", Skipped: true, Disposition: "refused", Reason: "out of scope"}}
	superseded := planRow{Task: verify.Task{ID: "S-1", Skipped: true, Disposition: "superseded", Reason: "by S-2"}}
	proven := planRow{Task: verify.Task{ID: "P-1", Claimed: true}, Verdict: verify.TaskVerdict{Status: verify.Pass}}
	for _, r := range []planRow{deferred, refused, superseded} {
		if depSatisfied(r) {
			t.Errorf("depSatisfied(%s, disposition %q) = true: a dispositioned [-] row is not done", r.Task.ID, r.Task.Disposition)
		}
	}
	if !depSatisfied(proven) {
		t.Error("a claimed, proven row satisfies needs:")
	}

	// Same plan.
	rows := []planRow{
		{PlanFile: "p.md", Task: verify.Task{ID: "A-1", Title: "waits on a deferred row", Needs: []string{"D-1"}}},
		{PlanFile: "p.md", Task: verify.Task{ID: "A-2", Title: "waits on a refused row", Needs: []string{"R-1", "P-1"}}},
		{PlanFile: "p.md", Task: verify.Task{ID: "A-3", Title: "waits on a superseded row", Needs: []string{"S-1"}}},
		{PlanFile: "p.md", Task: verify.Task{ID: "A-4", Title: "waits on a proven row", Needs: []string{"P-1"}}},
		deferred, refused, superseded, proven,
	}
	for i := range rows[4:] {
		rows[4+i].PlanFile = "p.md"
	}
	byID := map[string]frontierRow{}
	for _, r := range frontierForPlan(rows, nil, time.Now()) {
		byID[r.ID] = r
	}
	if a1 := byID["A-1"]; a1.Eligible || len(a1.BlockedOn) != 1 || a1.BlockedOn[0].State != "deferred" || len(a1.Replan) != 0 {
		t.Errorf("A-1 = %+v, want blocked on {D-1 deferred}, no re-plan", a1)
	}
	if a2 := byID["A-2"]; a2.Eligible || len(a2.BlockedOn) != 1 || a2.BlockedOn[0].State != "refused" || strings.Join(a2.Replan, ",") != "R-1" {
		t.Errorf("A-2 = %+v, want blocked on {R-1 refused} and named for re-planning", a2)
	}
	if a3 := byID["A-3"]; a3.Eligible || strings.Join(a3.Replan, ",") != "S-1" {
		t.Errorf("A-3 = %+v, want a superseded target to behave as a refused one", a3)
	}
	if a4 := byID["A-4"]; !a4.Eligible {
		t.Errorf("A-4 = %+v, want eligible", a4)
	}

	// Across plans: the sibling's authored state decides, with the same rule.
	dir := t.TempDir()
	sib := "# s\n\n- [-] Later {id: SD-1, disposition: deferred, reason: \"later\", trigger: \"when X\"}\n" +
		"- [-] Never {id: SR-1, disposition: refused, reason: \"no\"}\n" +
		"- [x] Done {id: SX-1, symbol: Foo}\n"
	if err := os.WriteFile(filepath.Join(dir, "sib.md"), []byte(sib), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		need, state string
		replan      bool
	}{
		{"sib.md#SD-1", "deferred (sibling plan)", false},
		{"sib.md#SR-1", "refused (sibling plan)", true},
	} {
		ok, blocked, replan := rowEligibility(verify.Task{ID: "X", Needs: []string{c.need}}, nil, dir)
		if ok || len(blocked) != 1 || blocked[0].State != c.state || (len(replan) == 1) != c.replan {
			t.Errorf("needs %s: eligible=%v blocked=%+v replan=%v, want blocked %q, replan %v", c.need, ok, blocked, replan, c.state, c.replan)
		}
	}
	if ok, blocked, _ := rowEligibility(verify.Task{ID: "X", Needs: []string{"sib.md#SX-1"}}, nil, dir); !ok {
		t.Errorf("a claimed sibling row satisfies needs:, blocked=%+v", blocked)
	}

	// The human frontier names the re-plan.
	storePath := seedPlansFixtureStore(t)
	pf := filepath.Join(dir, "plan.md")
	if err := os.WriteFile(pf, []byte("# p\n\n- [ ] Builds on a refused row {id: E-1, needs: E-2}\n- [-] Refused {id: E-2, disposition: refused, reason: \"no\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := run([]string{"plans", "next", "--service", "payment-service", "--store", storePath, "--plan", pf, "--root", dir}, &out, &errb); code != 0 {
		t.Fatalf("plans next exit %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "needs re-planning") || !strings.Contains(out.String(), "E-2") {
		t.Errorf("plans next does not name E-1 as needing re-planning:\n%s", out.String())
	}
}
