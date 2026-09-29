package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// wrappedRowsMD carries a row whose annotation block sits on its
// continuation line (G2-5): every writer and follow-up reader that handles
// a row by its lines must handle all of them.
const wrappedRowsMD = "# p\n\n- [ ] First row {id: W-1, symbol: A}\n- [ ] Wire the route across\n  two lines {id: W-2, symbol: something, package: pkg/x}\n- [ ] Third row {id: W-3, symbol: C}\n"

// TestVerifyFollowUpPlanKeepsWrappedRowLines: a follow-up call carries a
// missing wrapped row with its continuation line, so the daemon verifies it
// with its contract rather than as a positional no-contract row.
func TestVerifyFollowUpPlanKeepsWrappedRowLines(t *testing.T) {
	plan := verify.ParseMarkdownPlan("svc", []byte(wrappedRowsMD))
	sub, ok := verifyFollowUpPlan([]byte(wrappedRowsMD), plan, map[string]bool{"W-1": true, "W-3": true})
	if !ok {
		t.Fatal("W-2 is missing; a follow-up plan is due")
	}
	got := verify.ParseMarkdownPlan("svc", sub)
	var w2 *verify.Task
	for i := range got.Tasks {
		if got.Tasks[i].ID == "W-2" {
			w2 = &got.Tasks[i]
		}
	}
	if w2 == nil || w2.Expect.Symbol != "something" || w2.Expect.Package != "pkg/x" {
		t.Fatalf("the follow-up lost W-2's contract:\n%s", sub)
	}
	if len(got.Tasks) != len(plan.Tasks) {
		t.Errorf("follow-up has %d rows, want %d (placeholders keep positions)", len(got.Tasks), len(plan.Tasks))
	}
}

// TestThreadPromoteRoute_WrappedRowKeepsItsContract: `thread promote
// --route` writes ctx: into the block on the continuation line, never a
// second block on the checkbox line (which would shadow the row's id and
// contract).
func TestThreadPromoteRoute_WrappedRowKeepsItsContract(t *testing.T) {
	threadEnv(t)
	storePath := seedPlansFixtureStore(t)
	root := t.TempDir()
	now := time.Now().UTC()
	logPath := seedRouteFixtureCallLog(t, root, "payment-service", "payments.Charge", now)
	planPath := root + "/plan.md"
	if err := os.WriteFile(planPath, []byte(wrappedRowsMD), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := cmdThreadPromote([]string{
		"--route", "payments", "--plan", planPath, "--row", "W-2",
		"--service", "payment-service", "--store", storePath,
		"--repo", root, "--call-log", logPath,
	}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	raw, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	p := verify.ParseMarkdownPlan("svc", raw)
	if len(p.Tasks) != 3 {
		t.Fatalf("got %d rows after the write:\n%s", len(p.Tasks), raw)
	}
	w2 := p.Tasks[1]
	if w2.ID != "W-2" || w2.Expect.Symbol != "something" || strings.Join(w2.Ctx, " ") != "payments.Charge" {
		t.Fatalf("W-2 after the write = %+v:\n%s", w2, raw)
	}
	if !strings.Contains(string(raw), "- [ ] Wire the route across\n  two lines {id: W-2") {
		t.Errorf("the checkbox line or the continuation's indentation changed:\n%s", raw)
	}
}
