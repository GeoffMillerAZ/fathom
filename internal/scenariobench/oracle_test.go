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
			os.Environ(),
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
		wt := Worktree{Path: wtPath, SubjectClone: repoPath, CellDir: t.TempDir()}

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
		wt := Worktree{Path: wtPath, SubjectClone: repoPath, CellDir: t.TempDir()}

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
