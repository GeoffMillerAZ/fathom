package main

// selfskip_test.go — O56 defect A acceptance tests: `fathom verify` against a
// throwaway git repo, covering the four fixture shapes the fix specifies —
// bare skip_test (skip-without-reason), a reasoned skip that predates the
// claim (passes), a skip added alongside the claim in the working tree
// (self-skipped), and a skip added in the SAME commit that flips the box
// (self-skipped) — plus the PostToolUse edit-hook note (edithook_skipnote.go).

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// selfSkipGitRepo is a throwaway git work tree for the self-skip fixtures:
// real commits, a real HEAD/HEAD~1 history, never the actual project repo.
type selfSkipGitRepo struct {
	t    *testing.T
	root string
}

func newSelfSkipGitRepo(t *testing.T) *selfSkipGitRepo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	r := &selfSkipGitRepo{t: t, root: t.TempDir()}
	r.run("init", "-q")
	r.run("config", "commit.gpgsign", "false")
	r.run("config", "user.name", "Alice")
	r.run("config", "user.email", "alice@example.com")
	return r
}

func (r *selfSkipGitRepo) run(args ...string) {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.root
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out.String())
	}
}

// writePlan writes rel (repo-relative, e.g. "docs/plans/ss.md") and returns
// its absolute path.
func (r *selfSkipGitRepo) writePlan(rel, content string) string {
	r.t.Helper()
	full := filepath.Join(r.root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
	return full
}

func (r *selfSkipGitRepo) commitAll(msg string) {
	r.t.Helper()
	r.run("add", "-A")
	r.run("commit", "-q", "-m", msg)
}

// runInDir chdirs to dir for the duration of a `run(args, …)` call —
// verifyRepoRoot resolves the git toplevel off the process cwd, so a
// throwaway repo's own history (never the real project repo's) must be
// what fathom verify sees.
func runInDir(t *testing.T, dir string, args []string) (code int, stdout, stderr string) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatal(err)
		}
	}()
	var out, errBuf bytes.Buffer
	code = run(args, &out, &errBuf)
	return code, out.String(), errBuf.String()
}

// TestCmdVerify_SkipWithoutReason: a bare `skip_test: true` (no `skip:`
// reason) on a claimed row is refused — grammar error, counted unproven —
// independent of any git history at all.
func TestCmdVerify_SkipWithoutReason(t *testing.T) {
	storePath := seedPlansFixtureStore(t)
	repo := newSelfSkipGitRepo(t)
	pf := repo.writePlan("docs/plans/ss.md",
		"- [x] Charge handler {id: SS-1, symbol: Charge, skip_test: true}\n")
	repo.commitAll("claim with a bare skip_test")

	code, out, errBuf := runInDir(t, repo.root, []string{
		"verify", "--service", "payment-service", "--plan", pf, "--store", storePath,
	})
	if code != 1 {
		t.Fatalf("want exit 1, got %d\nstdout=%s\nstderr=%s", code, out, errBuf)
	}
	// Text mode prints the criterion's name/detail, not the row label — the
	// label itself ("skip-without-reason") is a JSON/SARIF-mode field.
	if !strings.Contains(out, "bare skips are refused") {
		t.Errorf("want the skip-without-reason detail in output, got:\n%s", out)
	}

	// JSON mode: the label is the machine-readable name for this refusal.
	codeJSON, outJSON, errJSON := runInDir(t, repo.root, []string{
		"verify", "--service", "payment-service", "--plan", pf, "--store", storePath, "--json",
	})
	if codeJSON != 1 {
		t.Fatalf("json mode: want exit 1, got %d\nstdout=%s\nstderr=%s", codeJSON, outJSON, errJSON)
	}
	if !strings.Contains(outJSON, `"label": "skip-without-reason"`) {
		t.Errorf("json mode: want label skip-without-reason, got:\n%s", outJSON)
	}
}

// TestCmdVerify_SkipWithReason_PredatesClaim_Passes: the skip_test/skip is
// committed BEFORE the row is claimed (a separate, earlier commit) — the
// skip predates the claim and stands, whatever the second commit does.
func TestCmdVerify_SkipWithReason_PredatesClaim_Passes(t *testing.T) {
	storePath := seedPlansFixtureStore(t)
	repo := newSelfSkipGitRepo(t)
	pf := repo.writePlan("docs/plans/ss.md",
		"- [ ] Charge handler {id: SS-2, symbol: Charge, skip_test: true, skip: \"no unit test covers this wiring\"}\n")
	repo.commitAll("introduce the skip before any claim")
	repo.writePlan("docs/plans/ss.md",
		"- [x] Charge handler {id: SS-2, symbol: Charge, skip_test: true, skip: \"no unit test covers this wiring\"}\n")
	repo.commitAll("claim done — skip predates the claim")

	code, out, errBuf := runInDir(t, repo.root, []string{
		"verify", "--service", "payment-service", "--plan", pf, "--store", storePath, "--quiet-pass",
	})
	if code != 0 {
		t.Fatalf("want exit 0 (skip predates the claim, so it stands), got %d\nstdout=%s\nstderr=%s", code, out, errBuf)
	}
}

// TestCmdVerify_SelfSkipped_WorkingTree: a skip is added in an UNCOMMITTED
// edit alongside the claim — O56's first worker, who added `skip_test:
// true` to its own row "to make the automated verifier pass cleanly."
func TestCmdVerify_SelfSkipped_WorkingTree(t *testing.T) {
	storePath := seedPlansFixtureStore(t)
	repo := newSelfSkipGitRepo(t)
	pf := repo.writePlan("docs/plans/ss.md",
		"- [ ] Charge handler {id: SS-3, symbol: Charge}\n")
	repo.commitAll("row open, no skip yet")
	// Uncommitted: claim + skip added together, right before a push.
	repo.writePlan("docs/plans/ss.md",
		"- [x] Charge handler {id: SS-3, symbol: Charge, skip_test: true, skip: \"added right before committing\"}\n")

	code, out, errBuf := runInDir(t, repo.root, []string{
		"verify", "--service", "payment-service", "--plan", pf, "--store", storePath,
	})
	if code != 1 {
		t.Fatalf("want exit 1 (self-skipped), got %d\nstdout=%s\nstderr=%s", code, out, errBuf)
	}
	if !strings.Contains(out, "self-skipped") {
		t.Errorf("want self-skipped in output, got:\n%s", out)
	}
}

// TestCmdVerify_SelfSkipped_SameCommit: the working tree is already clean
// (nothing uncommitted — the ordinary state at push time) but the skip and
// the claim were BOTH introduced by the same commit — O56's second worker,
// who "checked a box straight past a fail."
func TestCmdVerify_SelfSkipped_SameCommit(t *testing.T) {
	storePath := seedPlansFixtureStore(t)
	repo := newSelfSkipGitRepo(t)
	pf := repo.writePlan("docs/plans/ss.md",
		"- [ ] Charge handler {id: SS-4, symbol: Charge}\n")
	repo.commitAll("row open, no skip yet")
	repo.writePlan("docs/plans/ss.md",
		"- [x] Charge handler {id: SS-4, symbol: Charge, skip_test: true, skip: \"straight past the fail\"}\n")
	repo.commitAll("checked a box straight past a fail")

	code, out, errBuf := runInDir(t, repo.root, []string{
		"verify", "--service", "payment-service", "--plan", pf, "--store", storePath,
	})
	if code != 1 {
		t.Fatalf("want exit 1 (self-skipped), got %d\nstdout=%s\nstderr=%s", code, out, errBuf)
	}
	if !strings.Contains(out, "self-skipped") {
		t.Errorf("want self-skipped in output, got:\n%s", out)
	}
}

// TestCmdVerify_SelfSkipped_InitialCommit: a plan file's very first commit
// already claims a row with a skip — there is no HEAD~1 to compare against,
// and (within a real repo) that reads the same as "the skip did not
// predate the claim," not as "inapplicable."
func TestCmdVerify_SelfSkipped_InitialCommit(t *testing.T) {
	storePath := seedPlansFixtureStore(t)
	repo := newSelfSkipGitRepo(t)
	pf := repo.writePlan("docs/plans/ss.md",
		"- [x] Charge handler {id: SS-5, symbol: Charge, skip_test: true, skip: \"born claimed and skipped\"}\n")
	repo.commitAll("the repo's first commit")

	code, out, errBuf := runInDir(t, repo.root, []string{
		"verify", "--service", "payment-service", "--plan", pf, "--store", storePath,
	})
	if code != 1 {
		t.Fatalf("want exit 1 (self-skipped — no parent commit to predate the claim), got %d\nstdout=%s\nstderr=%s", code, out, errBuf)
	}
	if !strings.Contains(out, "self-skipped") {
		t.Errorf("want self-skipped in output, got:\n%s", out)
	}
}

// TestCmdVerify_SelfSkip_NoGitRepo: --store mode against a plan file OUTSIDE
// any git work tree (the common case in this test suite's other fixtures,
// e.g. writeTempPlan's t.TempDir()) never flags self-skipped — the check is
// inapplicable, not a punitive default, so it must never regress every
// other --store test in this package that has no git history at all.
func TestCmdVerify_SelfSkip_NoGitRepo(t *testing.T) {
	storePath := seedPlansFixtureStore(t)
	pf := writeTempPlan(t, "- [x] Charge handler {symbol: Charge, skip_test: true, skip: \"no git here\"}\n")

	var out, errBuf bytes.Buffer
	code := run([]string{
		"verify", "--service", "payment-service", "--plan", pf, "--store", storePath, "--quiet-pass",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("want exit 0 outside a git repo (self-skip inapplicable), got %d\nstdout=%s\nstderr=%s", code, out.String(), errBuf.String())
	}
}

// ─── edit hook note (fix item 3) ────────────────────────────────────────────

// TestSkipGainedNote_UncommittedGain: the PostToolUse note fires the moment
// a plan row gains skip_test in an uncommitted edit — daemon-free, and
// independent of whether the row is even claimed yet.
func TestSkipGainedNote_UncommittedGain(t *testing.T) {
	repo := newSelfSkipGitRepo(t)
	pf := repo.writePlan("docs/plans/ss.md",
		"- [ ] Charge handler {id: SN-1, symbol: Charge}\n")
	repo.commitAll("row open, no skip yet")
	repo.writePlan("docs/plans/ss.md",
		"- [ ] Charge handler {id: SN-1, symbol: Charge, skip_test: true, skip: \"just added\"}\n")

	note := skipGainedNote(repo.root, pf)
	want := "row SN-1 gained skip_test in this edit — verify will count it unproven until the skip predates the claim"
	if note != want {
		t.Fatalf("note = %q, want %q", note, want)
	}
}

// TestSkipGainedNote_NoGain: editing a row's title (or anything else) with
// no new skip_test produces no note.
func TestSkipGainedNote_NoGain(t *testing.T) {
	repo := newSelfSkipGitRepo(t)
	pf := repo.writePlan("docs/plans/ss.md",
		"- [ ] Charge handler {id: SN-2, symbol: Charge}\n")
	repo.commitAll("row open, no skip yet")
	repo.writePlan("docs/plans/ss.md",
		"- [ ] Charge handler, reworded {id: SN-2, symbol: Charge}\n")

	if note := skipGainedNote(repo.root, pf); note != "" {
		t.Fatalf("note = %q, want \"\" (no skip_test gained)", note)
	}
}

// TestSkipGainedNote_AlreadyHadSkip: a skip that predates this edit (the row
// already carried skip_test at HEAD) produces no note — only a GAIN notes.
func TestSkipGainedNote_AlreadyHadSkip(t *testing.T) {
	repo := newSelfSkipGitRepo(t)
	pf := repo.writePlan("docs/plans/ss.md",
		"- [ ] Charge handler {id: SN-3, symbol: Charge, skip_test: true, skip: \"long-standing\"}\n")
	repo.commitAll("row already has a skip")
	repo.writePlan("docs/plans/ss.md",
		"- [x] Charge handler {id: SN-3, symbol: Charge, skip_test: true, skip: \"long-standing\"}\n")

	if note := skipGainedNote(repo.root, pf); note != "" {
		t.Fatalf("note = %q, want \"\" (skip predates this edit)", note)
	}
}

// TestWithSkipGainedNote_Compose: withSkipGainedNote leads the payload and
// turns silence into an answering hook, mirroring withClaimMoment.
func TestWithSkipGainedNote_Compose(t *testing.T) {
	repo := newSelfSkipGitRepo(t)
	pf := repo.writePlan("docs/plans/ss.md",
		"- [ ] Charge handler {id: SN-4, symbol: Charge}\n")
	repo.commitAll("row open, no skip yet")
	repo.writePlan("docs/plans/ss.md",
		"- [ ] Charge handler {id: SN-4, symbol: Charge, skip_test: true, skip: \"just added\"}\n")

	text, ok := withSkipGainedNote(repo.root, pf, "", false)
	if !ok {
		t.Fatalf("ok = false, want true (the note alone answers)")
	}
	if !strings.Contains(text, "row SN-4 gained skip_test") {
		t.Fatalf("text = %q, want it to contain the note", text)
	}

	text2, ok2 := withSkipGainedNote(repo.root, pf, "existing obligations text", true)
	if !ok2 || !strings.HasPrefix(text2, "row SN-4 gained skip_test") || !strings.HasSuffix(text2, "existing obligations text") {
		t.Fatalf("compose with existing text = (%q, %v), want the note to lead", text2, ok2)
	}
}
