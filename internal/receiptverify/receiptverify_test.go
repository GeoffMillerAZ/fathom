package receiptverify_test

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/indexbench"
	"github.com/geoffmilleraz/fathom/internal/receiptverify"
)

// TestVerify_FindCallersReceipt_Reproduces is VB-20's golden test
// (docs/plans/verbs.md): a receipt shaped like find_callers' own answer
// over the fauxtown fixture fleet (internal/CLAUDE.md's deterministic
// fixture fleet; payment-service, staged as its own module —
// StageFauxtownModule, indexbench's own oracle-hermetic pattern) reproduces
// from a BARE git checkout, using go/types and git alone — no fathom
// daemon, no store, no internal/mcp.
func TestVerify_FindCallersReceipt_Reproduces(t *testing.T) {
	root, err := indexbench.StageFauxtownModule("payment-service", t.TempDir())
	if err != nil {
		t.Fatalf("stage fixture: %v", err)
	}
	commit := initGitRepo(t, root)

	// Charge calls validateCharge (payment.go) — a real, indexed edge.
	r := core.Receipt{
		Kind:        core.ReceiptKindCallers,
		Tool:        "find_callers",
		Symbol:      "validateCharge",
		IndexCommit: commit,
		Callers:     []string{"Charge"},
	}
	outcome, err := receiptverify.Verify(context.Background(), root, r)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !outcome.OK {
		t.Fatalf("expected the receipt to reproduce, got a difference: %s", outcome.Diff)
	}
}

// TestVerify_TamperedReceipt_NamesDifference is VB-20's second golden
// case: a receipt claiming a caller that does not exist at this commit
// fails with exactly that difference named, never a silent pass.
func TestVerify_TamperedReceipt_NamesDifference(t *testing.T) {
	root, err := indexbench.StageFauxtownModule("payment-service", t.TempDir())
	if err != nil {
		t.Fatalf("stage fixture: %v", err)
	}
	commit := initGitRepo(t, root)

	r := core.Receipt{
		Kind:        core.ReceiptKindCallers,
		Tool:        "find_callers",
		Symbol:      "validateCharge",
		IndexCommit: commit,
		Callers:     []string{"Charge", "NotARealCaller"}, // tampered: fabricated caller
	}
	outcome, err := receiptverify.Verify(context.Background(), root, r)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if outcome.OK {
		t.Fatal("expected a named difference for the tampered receipt, got OK")
	}
	if !strings.Contains(outcome.Diff, "NotARealCaller") {
		t.Errorf("diff does not name the tampered caller: %s", outcome.Diff)
	}
}

// TestVerify_MissingCommit_Exit2Shape is VB-20's "cannot run" case: a
// receipt naming a commit the repository does not have returns an error
// (cmd/fathom maps this to exit 2), never a false OK or a false diff.
func TestVerify_MissingCommit_Exit2Shape(t *testing.T) {
	root, err := indexbench.StageFauxtownModule("payment-service", t.TempDir())
	if err != nil {
		t.Fatalf("stage fixture: %v", err)
	}
	initGitRepo(t, root)

	r := core.Receipt{
		Kind:        core.ReceiptKindCallers,
		Symbol:      "validateCharge",
		IndexCommit: "0000000000000000000000000000000000dead",
		Callers:     []string{"Charge"},
	}
	if _, err := receiptverify.Verify(context.Background(), root, r); err == nil {
		t.Fatal("expected an error for a commit the repository does not have")
	}
}

func TestBareSymbolName(t *testing.T) {
	cases := map[string]string{
		"Charge":                          "Charge",
		"svc::pkg.Charge":                 "Charge",
		"svc::internal/mcp.Server.Handle": "Server.Handle",
		"internal/mcp/pkg.Recv.Method":    "Recv.Method",
		"svc::dir/subdir/pkg.Name":        "Name",
	}
	for in, want := range cases {
		if got := receiptverify.BareSymbolName(in); got != want {
			t.Errorf("BareSymbolName(%q) = %q, want %q", in, got, want)
		}
	}
}

// initGitRepo commits every file under dir on a fresh repo and returns
// HEAD's commit SHA — a bare checkout Verify can `git archive` from.
func initGitRepo(t *testing.T, dir string) string {
	t.Helper()
	env := append(
		os.Environ(),
		"GIT_AUTHOR_NAME=fathom-test", "GIT_AUTHOR_EMAIL=fathom-test@example.com",
		"GIT_COMMITTER_NAME=fathom-test", "GIT_COMMITTER_EMAIL=fathom-test@example.com",
	)
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("add", "-A")
	run("commit", "-q", "-m", "fixture")
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("git rev-parse: %v", err)
	}
	return strings.TrimSpace(string(out))
}
