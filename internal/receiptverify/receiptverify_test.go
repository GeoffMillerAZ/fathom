package receiptverify_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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

// TestVerify_CallersReceipt_ZeroCallers_Reproduces is review F2's case
// (a): Charge is a real, declared symbol with truly zero callers in this
// fixture — an empty-callers receipt for it must reproduce (exit 0), not
// be confused with F2's bug (a map miss on an UNDECLARED symbol silently
// reading as the same zero).
func TestVerify_CallersReceipt_ZeroCallers_Reproduces(t *testing.T) {
	root, err := indexbench.StageFauxtownModule("payment-service", t.TempDir())
	if err != nil {
		t.Fatalf("stage fixture: %v", err)
	}
	commit := initGitRepo(t, root)

	r := core.Receipt{
		Kind:        core.ReceiptKindCallers,
		Symbol:      "Charge",
		IndexCommit: commit,
		Callers:     []string{},
	}
	outcome, err := receiptverify.Verify(context.Background(), root, r)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !outcome.OK {
		t.Fatalf("expected a real zero-caller symbol to reproduce, got a difference: %s", outcome.Diff)
	}
}

// TestVerify_CallersReceipt_NonexistentSymbol_DoesNotReproduce is review
// F2's case (b): a symbol that is not declared at this commit at all must
// NOT reproduce as an empty-callers match — the fix under test.
func TestVerify_CallersReceipt_NonexistentSymbol_DoesNotReproduce(t *testing.T) {
	root, err := indexbench.StageFauxtownModule("payment-service", t.TempDir())
	if err != nil {
		t.Fatalf("stage fixture: %v", err)
	}
	commit := initGitRepo(t, root)

	r := core.Receipt{
		Kind:        core.ReceiptKindCallers,
		Symbol:      "NoSuchFunc",
		IndexCommit: commit,
		Callers:     []string{},
	}
	outcome, err := receiptverify.Verify(context.Background(), root, r)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if outcome.OK {
		t.Fatal("expected a nonexistent symbol to NOT reproduce (map-miss-as-zero bug)")
	}
	if !strings.Contains(outcome.Diff, "not declared") {
		t.Errorf("diff = %q, want it to say the symbol is not declared", outcome.Diff)
	}
}

// TestVerify_ReferencesReceipt_Reproduces / _DoesNotReproduce cover
// ReceiptKindReferences (review F2c: no such pair existed before).
func TestVerify_ReferencesReceipt_Reproduces(t *testing.T) {
	root, err := indexbench.StageFauxtownModule("payment-service", t.TempDir())
	if err != nil {
		t.Fatalf("stage fixture: %v", err)
	}
	commit := initGitRepo(t, root)

	// validateCharge is declared AND called in payment.go — one file.
	r := core.Receipt{
		Kind:        core.ReceiptKindReferences,
		Tool:        "references",
		Symbol:      "validateCharge",
		IndexCommit: commit,
		Files:       []string{"payment.go"},
	}
	outcome, err := receiptverify.Verify(context.Background(), root, r)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !outcome.OK {
		t.Fatalf("expected the references receipt to reproduce, got a difference: %s", outcome.Diff)
	}
}

func TestVerify_ReferencesReceipt_DoesNotReproduce(t *testing.T) {
	root, err := indexbench.StageFauxtownModule("payment-service", t.TempDir())
	if err != nil {
		t.Fatalf("stage fixture: %v", err)
	}
	commit := initGitRepo(t, root)

	r := core.Receipt{
		Kind:        core.ReceiptKindReferences,
		Symbol:      "validateCharge",
		IndexCommit: commit,
		Files:       []string{"payment.go", "bogus.go"}, // tampered: fabricated file
	}
	outcome, err := receiptverify.Verify(context.Background(), root, r)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if outcome.OK {
		t.Fatal("expected a named difference for the tampered references receipt, got OK")
	}
	if !strings.Contains(outcome.Diff, "bogus.go") {
		t.Errorf("diff does not name the tampered file: %s", outcome.Diff)
	}
}

// TestVerify_SymbolLocationReceipt_Reproduces / _DoesNotReproduce cover
// ReceiptKindSymbolLocation (review F2c: no such pair existed before).
func TestVerify_SymbolLocationReceipt_Reproduces(t *testing.T) {
	root, err := indexbench.StageFauxtownModule("payment-service", t.TempDir())
	if err != nil {
		t.Fatalf("stage fixture: %v", err)
	}
	commit := initGitRepo(t, root)

	r := core.Receipt{
		Kind:        core.ReceiptKindSymbolLocation,
		Tool:        "find_symbol",
		Symbol:      "Charge",
		IndexCommit: commit,
		File:        "payment.go",
		Line:        26,
	}
	outcome, err := receiptverify.Verify(context.Background(), root, r)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !outcome.OK {
		t.Fatalf("expected the symbol_location receipt to reproduce, got a difference: %s", outcome.Diff)
	}
}

func TestVerify_SymbolLocationReceipt_DoesNotReproduce(t *testing.T) {
	root, err := indexbench.StageFauxtownModule("payment-service", t.TempDir())
	if err != nil {
		t.Fatalf("stage fixture: %v", err)
	}
	commit := initGitRepo(t, root)

	r := core.Receipt{
		Kind:        core.ReceiptKindSymbolLocation,
		Symbol:      "Charge",
		IndexCommit: commit,
		File:        "payment.go",
		Line:        999, // tampered: wrong line
	}
	outcome, err := receiptverify.Verify(context.Background(), root, r)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if outcome.OK {
		t.Fatal("expected a named difference for the tampered symbol_location receipt, got OK")
	}
	if !strings.Contains(outcome.Diff, "999") {
		t.Errorf("diff does not name the tampered line: %s", outcome.Diff)
	}
}

// TestVerify_CallersReceipt_TestFileCaller_Reproduces is the M11b defect
// (docs/bench/readings/vb21-receipts-m11b.md): find_callers/references
// correctly include _test.go callers/occurrences in their live answers —
// they are real, indexed facts, not noise — but publicloc.LoadGoTruth
// (this package's independent oracle) was loading with Tests: false, so
// ANY receipt built verbatim from a live answer for a symbol with a
// test-file caller reported a spurious DIFFERENCE. A stand-alone fixture
// module (not fauxtown, which no other test's goldens should have to grow
// a _test.go file to accommodate) with one production function and one
// internal _test.go file calling it pins the fix: a receipt naming the
// test function as a caller must reproduce (exit 0).
func TestVerify_CallersReceipt_TestFileCaller_Reproduces(t *testing.T) {
	root := stageTestCallerModule(t)
	commit := initGitRepo(t, root)

	r := core.Receipt{
		Kind:        core.ReceiptKindCallers,
		Tool:        "find_callers",
		Symbol:      "Build",
		IndexCommit: commit,
		Callers:     []string{"TestBuild"},
	}
	outcome, err := receiptverify.Verify(context.Background(), root, r)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !outcome.OK {
		t.Fatalf("expected a receipt naming a _test.go caller to reproduce, got a difference: %s", outcome.Diff)
	}
}

// TestVerify_ReferencesReceipt_TestFileReference_Reproduces is the
// references-kind half of the same defect: a receipt naming the _test.go
// file among Symbol's referencing files must reproduce.
func TestVerify_ReferencesReceipt_TestFileReference_Reproduces(t *testing.T) {
	root := stageTestCallerModule(t)
	commit := initGitRepo(t, root)

	r := core.Receipt{
		Kind:        core.ReceiptKindReferences,
		Tool:        "references",
		Symbol:      "Build",
		IndexCommit: commit,
		Files:       []string{"widget.go", "widget_test.go"},
	}
	outcome, err := receiptverify.Verify(context.Background(), root, r)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !outcome.OK {
		t.Fatalf("expected a receipt naming a _test.go reference to reproduce, got a difference: %s", outcome.Diff)
	}
}

// stageTestCallerModule writes a two-file fixture module — one production
// function (Build, in widget.go) and one internal _test.go file
// (widget_test.go, `package widget`) whose TestBuild calls and mentions
// it — into a fresh temp dir and returns its root. Kept separate from
// indexbench.StageFauxtownModule on purpose: fauxtown is a shared fixture
// many other benches' committed goldens are pinned against, and this
// defect's fix needs a _test.go file that fixture has never carried.
func stageTestCallerModule(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module receiptverify.test/testcaller\n\ngo 1.21\n",
		"widget.go": `package widget

// Build returns the widget's name.
func Build() string {
	return "widget"
}
`,
		"widget_test.go": `package widget

import "testing"

// TestBuild calls and references Build — the test-file caller/reference
// the live daemon's find_callers/references correctly include.
func TestBuild(t *testing.T) {
	if Build() == "" {
		t.Fatal("Build returned empty")
	}
}
`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return root
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
