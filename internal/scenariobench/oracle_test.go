package scenariobench

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// goModuleFixture writes a tiny, self-contained Go module with two tests —
// TestPass (always passes) and TestFail (always fails) — so RunOracle can
// be exercised against a real `go test` run with no network and no other
// dependency on this repo's own module.
func goModuleFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "go.mod"), "module scenariooraclefixture\n\ngo 1.21\n")
	writeTestFile(t, filepath.Join(dir, "thing.go"), "package thing\n\nfunc Add(a, b int) int { return a + b }\n")
	writeTestFile(t, filepath.Join(dir, "thing_test.go"), `package thing

import "testing"

func TestPass(t *testing.T) {
	if Add(2, 3) != 5 {
		t.Fatal("bad")
	}
}

func TestFail(t *testing.T) {
	t.Fatal("this test always fails")
}
`)
	return dir
}

func TestRunOracle_AllPass(t *testing.T) {
	dir := goModuleFixture(t)
	wt := Worktree{Path: dir, CellDir: t.TempDir()}
	task := Task{Oracle: Oracle{FailToPass: []string{"TestPass"}, PassToPass: []string{"TestPass"}}}

	result, err := RunOracle(context.Background(), wt, task, nil, t.TempDir())
	if err != nil {
		t.Fatalf("RunOracle: %v", err)
	}
	if !result.FailToPassOK {
		t.Error("expected FailToPassOK=true")
	}
	if !result.PassToPassOK {
		t.Error("expected PassToPassOK=true")
	}
	if !result.Passed {
		t.Error("expected Passed=true")
	}
	if result.TestOutputRef == "" {
		t.Error("expected a TestOutputRef")
	}
	raw, err := os.ReadFile(result.TestOutputRef)
	if err != nil || !strings.Contains(string(raw), "PASS: TestPass") {
		t.Errorf("expected the output file to record TestPass's PASS line: err=%v content=%q", err, raw)
	}
}

func TestRunOracle_FailToPassFails(t *testing.T) {
	dir := goModuleFixture(t)
	wt := Worktree{Path: dir, CellDir: t.TempDir()}
	task := Task{Oracle: Oracle{FailToPass: []string{"TestFail"}, PassToPass: []string{"TestPass"}}}

	result, err := RunOracle(context.Background(), wt, task, nil, t.TempDir())
	if err != nil {
		t.Fatalf("RunOracle: %v", err)
	}
	if result.FailToPassOK {
		t.Error("expected FailToPassOK=false (TestFail always fails)")
	}
	if !result.PassToPassOK {
		t.Error("expected PassToPassOK=true (TestPass still passes)")
	}
	if result.Passed {
		t.Error("expected Passed=false")
	}
}

func TestRunOracle_Coverage(t *testing.T) {
	dir := goModuleFixture(t)
	wt := Worktree{Path: dir, CellDir: t.TempDir()}
	task := Task{Oracle: Oracle{FailToPass: []string{"TestPass"}, Coverage: true}}

	result, err := RunOracle(context.Background(), wt, task, nil, t.TempDir())
	if err != nil {
		t.Fatalf("RunOracle: %v", err)
	}
	if result.CoverProfile == "" {
		t.Fatal("expected a CoverProfile path")
	}
	if _, err := os.Stat(result.CoverProfile); err != nil {
		t.Errorf("expected the coverage profile to exist: %v", err)
	}
}

func TestRunOracle_Detectors(t *testing.T) {
	dir := goModuleFixture(t)
	wt := Worktree{Path: dir, CellDir: t.TempDir()}

	t.Run("expected exit satisfied", func(t *testing.T) {
		task := Task{Oracle: Oracle{Detectors: []Detector{{Name: "file-exists", Command: "test -f thing.go", ExpectExit: 0}}}}
		result, err := RunOracle(context.Background(), wt, task, nil, t.TempDir())
		if err != nil {
			t.Fatalf("RunOracle: %v", err)
		}
		if len(result.Detectors) != 1 || result.Detectors[0].Violated {
			t.Errorf("expected a satisfied, non-violated detector: %+v", result.Detectors)
		}
		if !result.Passed {
			t.Error("expected Passed=true (no named tests, no violation)")
		}
	})

	t.Run("violated detector fails the oracle", func(t *testing.T) {
		task := Task{Oracle: Oracle{Detectors: []Detector{{Name: "no-such-file", Command: "test -f does-not-exist.go", ExpectExit: 0}}}}
		result, err := RunOracle(context.Background(), wt, task, nil, t.TempDir())
		if err != nil {
			t.Fatalf("RunOracle: %v", err)
		}
		if len(result.Detectors) != 1 || !result.Detectors[0].Violated {
			t.Errorf("expected a violated detector: %+v", result.Detectors)
		}
		if result.Passed {
			t.Error("expected Passed=false when a detector is violated")
		}
	})
}

func TestRunOracle_NoNamedTestsIsVacuouslyOK(t *testing.T) {
	dir := goModuleFixture(t)
	wt := Worktree{Path: dir, CellDir: t.TempDir()}
	task := Task{Oracle: Oracle{}}
	result, err := RunOracle(context.Background(), wt, task, nil, t.TempDir())
	if err != nil {
		t.Fatalf("RunOracle: %v", err)
	}
	if !result.FailToPassOK || !result.PassToPassOK || !result.Passed {
		t.Errorf("expected a vacuous pass with no named tests and no detectors: %+v", result)
	}
}

// oracleCalibrationFixtureRepo builds a tiny git module at base_commit
// with a bug (Add subtracts instead of adding) and one pre-existing test,
// TestBroken, that fails deterministically regardless of code — modeling a
// pass_to_pass test broken by the environment, not by the agent's change.
// TestFix (the fail_to_pass test) is NOT part of base_commit: SWE-bench
// semantics hide it in a test patch applied at evaluation time, never
// visible to the agent's sessions.
func oracleCalibrationFixtureRepo(t *testing.T) (repoPath, commit string) {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(
			gitEnv(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", "-b", "main")
	writeTestFile(t, filepath.Join(dir, "go.mod"), "module scenariocalibfixture\n\ngo 1.21\n")
	writeTestFile(t, filepath.Join(dir, "thing.go"), "package thing\n\nfunc Add(a, b int) int { return a - b }\n")
	writeTestFile(t, filepath.Join(dir, "thing_test.go"), `package thing

import "testing"

func TestBroken(t *testing.T) {
	t.Fatal("environment broken: this test always fails regardless of code")
}
`)
	run("add", ".")
	run("commit", "-q", "-m", "initial commit")
	sha := run("rev-parse", "HEAD")
	return dir, sha
}

// oracleCalibrationTestPatch is the hidden test patch: it adds TestFix,
// the fail_to_pass test, in a new file — never touching thing.go or
// thing_test.go, so it applies cleanly to any fresh checkout of base_commit
// regardless of whether thing.go has since been fixed.
const oracleCalibrationTestPatch = `diff --git a/thing_fix_test.go b/thing_fix_test.go
new file mode 100644
--- /dev/null
+++ b/thing_fix_test.go
@@ -0,0 +1,9 @@
+package thing
+
+import "testing"
+
+func TestFix(t *testing.T) {
+	if Add(2, 3) != 5 {
+		t.Fatalf("Add(2,3) = %d, want 5", Add(2, 3))
+	}
+}
`

// cloneAtCommit makes a fresh, local, network-free clone of src checked
// out at commit — used so each subtest gets its own working tree without
// disturbing the fixture repo or any other subtest's tree.
func cloneAtCommit(t *testing.T, src, commit string) string {
	t.Helper()
	dir := t.TempDir()
	wt := filepath.Join(dir, "wt")
	if out, err := exec.Command("git", "clone", "--quiet", src, wt).CombinedOutput(); err != nil {
		t.Fatalf("git clone: %v (%s)", err, out)
	}
	if out, err := exec.Command("git", "-C", wt, "checkout", "--quiet", commit).CombinedOutput(); err != nil {
		t.Fatalf("git checkout %s: %v (%s)", commit, err, out)
	}
	return wt
}

// TestOracle_TestPatchAndCalibration proves the SWE-bench test-patch
// semantics end to end on the git fixture repo (see gitFixtureRepo in
// subject_test.go for the plain-fixture pattern this extends): the
// fail_to_pass test fails at base (before any fix), passes once the fix is
// made, and the pass_to_pass test broken by the environment is excluded
// from the verdict rather than blamed on the agent.
func TestOracle_TestPatchAndCalibration(t *testing.T) {
	repoPath, commit := oracleCalibrationFixtureRepo(t)
	root := t.TempDir()
	task := Task{
		ID:      "T-CALIB",
		Subject: Subject{RepoURL: repoPath, BaseCommit: commit},
		Oracle:  Oracle{FailToPass: []string{"TestFix"}, PassToPass: []string{"TestBroken"}},
		Gold:    Gold{TestPatch: oracleCalibrationTestPatch},
	}

	t.Run("fails at base, broken p2p excluded", func(t *testing.T) {
		wtPath := cloneAtCommit(t, repoPath, commit)
		wt := Worktree{Path: wtPath, SubjectClone: repoPath, CellDir: t.TempDir(), LocalBase: commit} // cloneAtCommit: full history, commit IS resolvable

		result, err := RunOracle(context.Background(), wt, task, nil, root)
		if err != nil {
			t.Fatalf("RunOracle: %v", err)
		}
		if result.FailToPassOK {
			t.Error("expected FailToPassOK=false before the fix (TestFix fails against the buggy Add)")
		}
		if !result.PassToPassOK {
			t.Error("expected PassToPassOK=true — TestBroken already failed at base and must be excluded, not blamed")
		}
		if result.Passed {
			t.Error("expected Passed=false")
		}
		if result.Calibration == nil {
			t.Fatal("expected an OracleResult.Calibration summary")
		}
		if result.Calibration.FailToPassFailedAtBase != 1 || result.Calibration.FailToPassTotal != 1 {
			t.Errorf("Calibration = %+v, want 1 f2p failing at base of 1 total", result.Calibration)
		}
		if result.Calibration.PassToPassEffective != 0 || result.Calibration.PassToPassTotal != 1 {
			t.Errorf("Calibration = %+v, want 0 effective p2p of 1 total (TestBroken excluded)", result.Calibration)
		}
		if result.Calibration.NotMeasurable != "" {
			t.Errorf("Calibration.NotMeasurable = %q, want empty", result.Calibration.NotMeasurable)
		}
	})

	t.Run("passes after the fix, broken p2p still excluded", func(t *testing.T) {
		wtPath := cloneAtCommit(t, repoPath, commit)
		// Simulate the agent's session: fix the bug directly in the tree,
		// BEFORE the oracle applies the hidden test patch — exactly the
		// order RunOracle enforces (sessions, then ApplyTestPatch, then
		// tests).
		writeTestFile(t, filepath.Join(wtPath, "thing.go"), "package thing\n\nfunc Add(a, b int) int { return a + b }\n")
		wt := Worktree{Path: wtPath, SubjectClone: repoPath, CellDir: t.TempDir(), LocalBase: commit} // cloneAtCommit: full history, commit IS resolvable

		result, err := RunOracle(context.Background(), wt, task, nil, root)
		if err != nil {
			t.Fatalf("RunOracle: %v", err)
		}
		if !result.FailToPassOK {
			t.Error("expected FailToPassOK=true after the fix")
		}
		if !result.PassToPassOK {
			t.Error("expected PassToPassOK=true — TestBroken stays excluded")
		}
		if !result.Passed {
			t.Errorf("expected Passed=true, got %+v", result)
		}
	})

	// The calibration cache is keyed on (subject repo, base_commit,
	// test-patch sha256) and computed once — the second subtest's call
	// must have reused the first's, not recomputed it.
	if _, err := os.Stat(filepath.Join(root, "workspace", "scenariobench", "calibration", calibrationKey(task)+".json")); err != nil {
		t.Errorf("expected a calibration cache file to exist: %v", err)
	}
}

// TestRunOracle_AnswerAllOf covers SB-32's answer-only oracle: every token
// must appear, case-insensitive, in the LAST session's FinalAnswer.
func TestRunOracle_AnswerAllOf(t *testing.T) {
	wt := Worktree{Path: t.TempDir(), CellDir: t.TempDir()}
	task := Task{Oracle: Oracle{AnswerAllOf: []string{"fusion.New", "golden-test"}}}

	t.Run("matches, case-insensitive", func(t *testing.T) {
		sessions := []SessionResult{
			{FinalAnswer: "not this one — FUSION.NEW appears here"},
			{FinalAnswer: "The answer is fusion.New and also golden-test appear here."},
		}
		result, err := RunOracle(context.Background(), wt, task, sessions, t.TempDir())
		if err != nil {
			t.Fatalf("RunOracle: %v", err)
		}
		if !result.Passed {
			t.Errorf("expected Passed=true, got %+v", result)
		}
	})

	t.Run("missing token fails", func(t *testing.T) {
		sessions := []SessionResult{{FinalAnswer: "fusion.New only, no second token"}}
		result, err := RunOracle(context.Background(), wt, task, sessions, t.TempDir())
		if err != nil {
			t.Fatalf("RunOracle: %v", err)
		}
		if result.Passed {
			t.Errorf("expected Passed=false when a token is missing, got %+v", result)
		}
	})

	t.Run("only the last session counts", func(t *testing.T) {
		sessions := []SessionResult{
			{FinalAnswer: "fusion.New and golden-test both here"},
			{FinalAnswer: "neither token in this final session"},
		}
		result, err := RunOracle(context.Background(), wt, task, sessions, t.TempDir())
		if err != nil {
			t.Fatalf("RunOracle: %v", err)
		}
		if result.Passed {
			t.Errorf("expected Passed=false — only the LAST session's answer counts, got %+v", result)
		}
	})
}

// testPatchTargetFixtureRepo builds a tiny git module at base_commit with a
// buggy Add and ONE pre-existing test file (thing_test.go, carrying
// TestBroken only) — the same file the hidden test patch below also
// modifies, so an agent that independently edits it lands on the exact
// file the patch's own headers name. testPatch is generated from a REAL
// `git diff` of a second, uncommitted edit to that file (adding TestFix),
// so its hunks carry genuine base-relative line numbers.
func testPatchTargetFixtureRepo(t *testing.T) (repoPath, baseCommit, testPatch string) {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(
			gitEnv(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	testFilePath := filepath.Join(dir, "thing_test.go")
	run("init", "-q", "-b", "main")
	writeTestFile(t, filepath.Join(dir, "go.mod"), "module scenariotestpatchtargetfixture\n\ngo 1.21\n")
	writeTestFile(t, filepath.Join(dir, "thing.go"), "package thing\n\nfunc Add(a, b int) int { return a - b }\n")
	writeTestFile(t, testFilePath, "package thing\n\nimport \"testing\"\n\nfunc TestBroken(t *testing.T) {\n\tt.Fatal(\"environment broken: this test always fails regardless of code\")\n}\n")
	run("add", ".")
	run("commit", "-q", "-m", "base")
	base := run("rev-parse", "HEAD")

	// The gold edit to the SAME file the fixture's own agent will also
	// touch below — left uncommitted so `git diff` yields the real patch,
	// then reverted so the fixture repo itself stays at base_commit.
	writeTestFile(t, testFilePath, `package thing

import "testing"

func TestBroken(t *testing.T) {
	t.Fatal("environment broken: this test always fails regardless of code")
}

func TestFix(t *testing.T) {
	if Add(2, 3) != 5 {
		t.Fatalf("Add(2,3) = %d, want 5", Add(2, 3))
	}
}
`)
	// NOT via run() above: its strings.TrimSpace strips the diff's own
	// trailing newline, which corrupts the last hunk line for git apply
	// ("corrupt patch") — the patch text itself must be captured verbatim.
	diffCmd := exec.Command("git", "diff", "--", "thing_test.go")
	diffCmd.Dir = dir
	diffOut, err := diffCmd.Output()
	if err != nil {
		t.Fatalf("git diff: %v", err)
	}
	patch := string(diffOut)
	run("checkout", "--", "thing_test.go")
	return dir, base, patch
}

// TestApplyTestPatch_ResetsAgentEditedTarget covers the public pilot 1
// failure (2026-09-13): an agent edits a file the hidden test patch ALSO
// touches, and a plain `git apply` used to fail with "does not match
// index"/"patch does not apply". ApplyTestPatch must reset that file to
// base_commit first — undoing only the agent's edit to THAT path, not any
// other file — so the patch still applies and the oracle stays measurable.
func TestApplyTestPatch_ResetsAgentEditedTarget(t *testing.T) {
	repoPath, baseCommit, testPatch := testPatchTargetFixtureRepo(t)
	wtPath := cloneAtCommit(t, repoPath, baseCommit)

	// Simulate the agent: it edited thing_test.go too (a comment, nothing
	// that changes behavior), leaving the file's content diverged from
	// base — the condition that broke a plain `git apply` in the pilot.
	writeTestFile(t, filepath.Join(wtPath, "thing_test.go"), `package thing

import "testing"

func TestBroken(t *testing.T) {
	// the agent left this note while investigating, unrelated to the fix
	t.Fatal("environment broken: this test always fails regardless of code")
}
`)
	// A non-test file the patch does NOT name — must survive untouched.
	writeTestFile(t, filepath.Join(wtPath, "NOTES.md"), "agent's own scratch notes\n")

	wt := Worktree{Path: wtPath, SubjectClone: repoPath, CellDir: t.TempDir(), LocalBase: baseCommit} // cloneAtCommit: full history, baseCommit IS resolvable
	task := Task{
		ID:      "T-RESET",
		Subject: Subject{RepoURL: repoPath, BaseCommit: baseCommit},
		Oracle:  Oracle{FailToPass: []string{"TestFix"}, PassToPass: []string{"TestBroken"}},
		Gold:    Gold{TestPatch: testPatch},
	}

	result, err := RunOracle(context.Background(), wt, task, nil, t.TempDir())
	if err != nil {
		t.Fatalf("RunOracle: %v", err)
	}
	if result.Calibration == nil {
		t.Fatal("expected a Calibration summary")
	}
	if result.Calibration.NotMeasurable != "" {
		t.Fatalf("expected the test patch to apply despite the agent's edit to the same file, got not_measurable=%q", result.Calibration.NotMeasurable)
	}
	if result.FailToPassOK {
		t.Error("expected FailToPassOK=false — Add is still buggy, TestFix should still fail")
	}

	raw, err := os.ReadFile(filepath.Join(wtPath, "thing_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "func TestFix(") {
		t.Errorf("expected thing_test.go to carry TestFix after the patch applied: %s", raw)
	}
	notes, err := os.ReadFile(filepath.Join(wtPath, "NOTES.md"))
	if err != nil || string(notes) != "agent's own scratch notes\n" {
		t.Errorf("expected NOTES.md (not named by the patch) to survive untouched: content=%q err=%v", notes, err)
	}
}

// TestAnswerOnly proves the shape AnswerOnly reports: an AnswerAllOf oracle
// with no tree-level check at all is answer-only; any tree-level check
// (even alongside AnswerAllOf) is not.
func TestAnswerOnly(t *testing.T) {
	cases := []struct {
		name string
		o    Oracle
		want bool
	}{
		{"answer only", Oracle{AnswerAllOf: []string{"tok"}}, true},
		{"no oracle at all", Oracle{}, false},
		{"answer plus fail_to_pass", Oracle{AnswerAllOf: []string{"tok"}, FailToPass: []string{"TestX"}}, false},
		{"tree oracle only", Oracle{FailToPass: []string{"TestX"}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := AnswerOnly(Task{Oracle: c.o}); got != c.want {
				t.Errorf("AnswerOnly(%+v) = %v, want %v", c.o, got, c.want)
			}
		})
	}
}

// TestSuccessFor is the fix-first review's direct unit test of the ONE
// any-edits-floor formula runOneCell, runExternalCell and cmdScore all
// now share: oracle.Passed alone is never enough for a change task with
// no edits, UNLESS the task's oracle is one of the three exempt shapes
// (AnswerOnly, LocalizationOnly, RefusalAware) — and oracle.Passed=false
// always loses regardless of shape.
func TestSuccessFor(t *testing.T) {
	changeTask := Task{Oracle: Oracle{FailToPass: []string{"TestX"}}}
	answerOnlyTask := Task{Oracle: Oracle{AnswerAllOf: []string{"tok"}}}
	localizationOnlyTask := Task{Oracle: Oracle{Localization: true}}
	refusalTask := Task{Oracle: Oracle{
		FailToPass: []string{"TestX"}, // a refusal-aware task still carries a tree oracle
		Refusal:    &Refusal{ForbiddenPaths: []string{"CLAUDE.md"}, MustCite: []string{"rule"}},
	}}
	edited := []SessionResult{{FilesEdited: []string{"foo.go"}}}
	unedited := []SessionResult{{FinalAnswer: "done, nothing to change"}}
	// wave N15 item 3: a session that claimed a row AND gave itself the
	// skip_test escape in the same edit (O56 self-grading) — SuccessFor
	// must fail this regardless of how favorably the oracle itself reads.
	selfSkipped := []SessionResult{{
		FilesEdited: []string{"foo.go"},
		BoxChecks: []BoxCheck{{
			RowID: "GIST-TTY-3",
			Text:  "- [x] Add a test case [GIST-TTY-3] {symbol: TestNewCmdView, package: pkg/cmd/gist/view, skip_test: true}",
		}},
	}}

	cases := []struct {
		name     string
		task     Task
		passed   bool
		sessions []SessionResult
		want     bool
	}{
		{"change task, edited, passed", changeTask, true, edited, true},
		{"change task, unedited, passed: any-edits floor fails it", changeTask, true, unedited, false},
		{"change task, edited, not passed", changeTask, false, edited, false},
		{"answer-only, unedited, passed: exempt", answerOnlyTask, true, unedited, true},
		{"answer-only, unedited, not passed", answerOnlyTask, false, unedited, false},
		{"localization-only, unedited, passed: exempt", localizationOnlyTask, true, unedited, true},
		{"refusal-aware, unedited, passed: exempt (O57 finding 2)", refusalTask, true, unedited, true},
		{"refusal-aware, unedited, not passed", refusalTask, false, unedited, false},
		{"change task, edited, passed, but self-skipped: fails regardless (O56)", changeTask, true, selfSkipped, false},
		{"refusal-aware, self-skipped: even an exempt shape fails", refusalTask, true, selfSkipped, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := SuccessFor(c.task, OracleResult{Passed: c.passed}, c.sessions); got != c.want {
				t.Errorf("SuccessFor(...) = %v, want %v", got, c.want)
			}
		})
	}
}

// TestRunDetector_TrustsParsedGoTestFailureOverExitCode is wave N14 item 5's
// regression test (guards-i19, guards-pilot-6 second reading): a real cell
// recorded oracle.detectors["no-org-names"] as exit=0/violated=false while
// its OWN logged raw test output showed TestNoOrgSpecificNames failing with
// five real violations — runDetector trusted the bare process exit code
// alone and never looked at what the test itself actually printed. This
// synthetic command reproduces exactly that shape (a go-test-style
// "--- FAIL: ..." / "FAIL" summary in the output, but the wrapping command
// itself exits 0): runDetector must now parse the real test result and
// override Violated to true, with a Note explaining the mismatch, rather
// than trusting an exit code the output itself contradicts.
func TestRunDetector_TrustsParsedGoTestFailureOverExitCode(t *testing.T) {
	wt := Worktree{Path: t.TempDir(), CellDir: t.TempDir()}
	d := Detector{
		Name: "no-org-names",
		Command: `sh -c 'printf -- "--- FAIL: TestNoOrgSpecificNames (0.01s)\n"; ` +
			`printf "    orgnames_test.go:287: org-specific token found in a fixture file\n"; ` +
			`printf "FAIL\n"; exit 0'`,
		ExpectExit: 0,
	}
	dr, err := runDetector(context.Background(), wt, Task{}, d)
	if err != nil {
		t.Fatalf("runDetector: %v", err)
	}
	if !dr.Violated {
		t.Errorf("expected Violated=true — the test output itself shows a real FAIL despite the wrapping command's exit=0, got %+v", dr)
	}
	if dr.Note == "" {
		t.Error("expected a Note explaining the exit-code/output mismatch")
	}
}

// TestRunDetector_GenuinePassStillReadsClean proves the fix above is not
// over-eager: a detector whose output legitimately shows "--- PASS:"/"PASS"
// (a real go test success) must still read Violated=false.
func TestRunDetector_GenuinePassStillReadsClean(t *testing.T) {
	wt := Worktree{Path: t.TempDir(), CellDir: t.TempDir()}
	d := Detector{
		Name:       "no-org-names",
		Command:    `sh -c 'printf -- "--- PASS: TestNoOrgSpecificNames (0.01s)\n"; printf "PASS\n"; exit 0'`,
		ExpectExit: 0,
	}
	dr, err := runDetector(context.Background(), wt, Task{}, d)
	if err != nil {
		t.Fatalf("runDetector: %v", err)
	}
	if dr.Violated {
		t.Errorf("expected a genuine PASS to read clean, got %+v", dr)
	}
	if dr.Note != "" {
		t.Errorf("expected no mismatch note on a genuine, consistent PASS, got %q", dr.Note)
	}
}

// TestRunDetector_GenuineFailAtNonZeroExitUnaffected proves the fix does
// not disturb the ordinary case: an actually-failing go test that ALSO
// exits nonzero (the overwhelmingly common real shape) still reads
// Violated=true with no spurious mismatch note.
func TestRunDetector_GenuineFailAtNonZeroExitUnaffected(t *testing.T) {
	wt := Worktree{Path: t.TempDir(), CellDir: t.TempDir()}
	d := Detector{
		Name:       "no-org-names",
		Command:    `sh -c 'printf -- "--- FAIL: TestNoOrgSpecificNames (0.01s)\n"; printf "FAIL\n"; exit 1'`,
		ExpectExit: 0,
	}
	dr, err := runDetector(context.Background(), wt, Task{}, d)
	if err != nil {
		t.Fatalf("runDetector: %v", err)
	}
	if !dr.Violated {
		t.Errorf("expected Violated=true for a consistent FAIL, got %+v", dr)
	}
	if dr.Note != "" {
		t.Errorf("expected no mismatch note when the exit code and output already agree, got %q", dr.Note)
	}
}
