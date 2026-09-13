package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/scenariobench"
)

// gitFixtureSubject creates a local, self-contained go module git repo (one
// commit) with a passing and a failing test, so cmdRun's oracle step has
// something real (but tiny, local, no network) to run.
func gitFixtureSubject(t *testing.T) (repoPath, commit string) {
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
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q", "-b", "main")
	write("go.mod", "module scenariobenchsubject\n\ngo 1.21\n")
	write("thing.go", "package thing\n\nfunc Add(a, b int) int { return a + b }\n")
	write("thing_test.go", "package thing\n\nimport \"testing\"\n\nfunc TestPass(t *testing.T) {\n\tif Add(2, 3) != 5 {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n")
	run("add", ".")
	run("commit", "-q", "-m", "initial commit")
	sha := run("rev-parse", "HEAD")
	return dir, sha
}

func writeCorpusFixture(t *testing.T, dir, repoURL, commit string) string {
	t.Helper()
	corpus := `{
  "arms": {
    "control": {"wired": false, "instructions": "none", "surface": ""}
  },
  "tasks": [{
    "id": "T1",
    "category": "bug_fix",
    "prompt": "fix it",
    "derivation": "synthetic fixture for scenariobench's own tests",
    "subject": {"repo_url": "` + repoURL + `", "base_commit": "` + commit + `"},
    "sessions": {"mode": "single", "count": 1, "max_turns": 5},
    "oracle": {"fail_to_pass": ["TestPass"], "coverage": true},
    "predictions": {"control": "no-effect (this is the control)"}
  }],
  "criterion": {"id": "C1", "treatment": "control", "control": "control", "min_trials": 1}
}`
	path := filepath.Join(dir, "corpus.json")
	if err := os.WriteFile(path, []byte(corpus), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCmdPlan_NoSpend(t *testing.T) {
	dir := t.TempDir()
	repoURL, commit := gitFixtureSubject(t)
	corpusPath := writeCorpusFixture(t, dir, repoURL, commit)
	cellsDir := filepath.Join(dir, "cells")

	rc := run([]string{"plan", "-corpus", corpusPath, "-cells", cellsDir, "-harness-version", "test-1"})
	if rc != 0 {
		t.Fatalf("plan exited %d", rc)
	}
	// plan must not have created any subject clone or session artifact —
	// it inspects the corpus and the cell store only.
	if _, err := os.Stat(filepath.Join(dir, "workspace")); !os.IsNotExist(err) {
		t.Errorf("plan should spend nothing and create no workspace, but found one (err=%v)", err)
	}
}

func TestCmdPlan_MissingFlags(t *testing.T) {
	if rc := run([]string{"plan"}); rc != 2 {
		t.Errorf("plan with no -corpus: exit %d, want 2", rc)
	}
}

func TestCmdRun_FakeHarnessControlArm(t *testing.T) {
	dir := t.TempDir()
	repoURL, commit := gitFixtureSubject(t)
	corpusPath := writeCorpusFixture(t, dir, repoURL, commit)
	root := filepath.Join(dir, "fathom-root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	cellsDir := filepath.Join(dir, "cells")
	rawDir := filepath.Join(dir, "raw")
	streamsDir := repoRelative(t, "testdata", "scenariobench", "streams")

	rc := run([]string{
		"run",
		"-corpus", corpusPath,
		"-root", root,
		"-cells", cellsDir,
		"-raw-dir", rawDir,
		"-run-id", "run-1",
		"-harness-version", "fake-1",
		"-harness", "fake",
		"-fake-streams-dir", streamsDir,
		"-fake-streams", `{"T1/control":"single-session.jsonl"}`,
	})
	if rc != 0 {
		t.Fatalf("run exited %d, want 0 (the fixture's oracle test passes)", rc)
	}

	entries, err := os.ReadDir(cellsDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected at least one cell filed under %s: err=%v entries=%v", cellsDir, err, entries)
	}

	// The SDD readings (plan coverage, false-done, resumption, drift) must
	// be present on every stored cell — not_measurable is an honest
	// reading; a nil pointer is a bug.
	cellPath := findStoredCell(t, cellsDir)
	cell := readStoredCell(t, cellPath)
	if cell.Readings.PlanCoverage == nil {
		t.Error("expected Readings.PlanCoverage to be present")
	}
	if cell.Readings.FalseDone == nil {
		t.Error("expected Readings.FalseDone to be present")
	} else if cell.Readings.FalseDone.NotMeasurable != "" {
		t.Errorf("expected FalseDone measurable (coverage was requested, nothing was claimed to refute): not_measurable=%q", cell.Readings.FalseDone.NotMeasurable)
	}
	if cell.Readings.Resumption == nil {
		t.Error("expected Readings.Resumption to be present")
	}
	if cell.Readings.Drift == nil {
		t.Error("expected Readings.Drift to be present")
	}
	if cell.AgentDiffPath == "" {
		t.Error("expected AgentDiffPath to be captured (a `git diff` runs regardless of content)")
	} else if _, err := os.Stat(cell.AgentDiffPath); err != nil {
		t.Errorf("AgentDiffPath %q does not exist: %v", cell.AgentDiffPath, err)
	}

	// `score` must recompute the same SDD readings from what was
	// captured (plan.md/agent.diff/coverage.out, plus the persisted Gold
	// and WorktreePath) — never by re-preparing the subject.
	if rc := run([]string{"score", "-in", cellPath}); rc != 0 {
		t.Fatalf("score exited %d", rc)
	}
	rescored := readStoredCell(t, cellPath)
	if rescored.Readings.PlanCoverage == nil || rescored.Readings.FalseDone == nil ||
		rescored.Readings.Resumption == nil || rescored.Readings.Drift == nil {
		t.Errorf("expected score to recompute every SDD reading too, got %+v", rescored.Readings)
	}
	if rescored.Readings.FalseDone.NotMeasurable != "" {
		t.Errorf("expected score's FalseDone to stay measurable (the worktree still exists): not_measurable=%q", rescored.Readings.FalseDone.NotMeasurable)
	}

	// Running it again must borrow rather than re-run (the second run
	// still exits 0 and needs no fake stream mapping since it never calls
	// the harness).
	rc2 := run([]string{
		"run", "-corpus", corpusPath, "-root", root, "-cells", cellsDir, "-raw-dir", rawDir,
		"-run-id", "run-2", "-harness-version", "fake-1", "-harness", "fake", "-fake-streams-dir", streamsDir,
	})
	if rc2 != 0 {
		t.Fatalf("second run exited %d, want 0 (should borrow)", rc2)
	}
}

func TestCmdScore_RecomputesReadings(t *testing.T) {
	dir := t.TempDir()
	cellPath := filepath.Join(dir, "cell.json")
	raw, _ := json.Marshal(map[string]any{
		"task_id":  "T1",
		"arm":      "control",
		"cell_key": "k1",
		"run_id":   "run-1",
		"sessions": []map[string]any{
			{"index": 0, "role": "single", "total_tokens": 42, "turns": 3, "cost_usd": 0.01, "payload_bytes": 100, "files_edited": []string{"pkg/a.go"}},
		},
		"oracle":   map[string]any{"passed": true},
		"readings": map[string]any{},
	})
	if err := os.WriteFile(cellPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	rc := run([]string{"score", "-in", cellPath})
	if rc != 0 {
		t.Fatalf("score exited %d", rc)
	}
	out, err := os.ReadFile(cellPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"total_tokens": 42`) {
		t.Errorf("expected recomputed readings to include total_tokens=42: %s", out)
	}
	if !strings.Contains(string(out), `"success": true`) {
		t.Errorf("expected success to be recomputed from oracle.passed AND an edited file: %s", out)
	}
}

// TestCmdScore_SDDReadingsWithoutWorktree covers the honesty rule "re-score
// must not need the worktree": PlanCoverage recomputes fully from a
// captured plan.md + the persisted Gold with no worktree at all, while
// FalseDone/Drift — which need the final tree — report an explicit
// not_measurable when WorktreePath no longer exists (a later cell sharing
// the same subject/base_commit/arm reset it), never a false zero.
func TestCmdScore_SDDReadingsWithoutWorktree(t *testing.T) {
	dir := t.TempDir()
	planPath := filepath.Join(dir, "plan.md")
	if err := os.WriteFile(planPath, []byte("- [ ] fix it `Add`\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cellPath := filepath.Join(dir, "cell.json")
	raw, _ := json.Marshal(map[string]any{
		"task_id":       "T1",
		"arm":           "control",
		"cell_key":      "k1",
		"run_id":        "run-1",
		"plan_path":     planPath,
		"worktree_path": filepath.Join(dir, "does-not-exist"),
		"gold":          map[string]any{"functions": []string{"Add"}},
		"sessions":      []map[string]any{{"index": 0, "role": "single", "files_edited": []string{"pkg/a.go"}}},
		"oracle":        map[string]any{"passed": true},
		"readings":      map[string]any{},
	})
	if err := os.WriteFile(cellPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	if rc := run([]string{"score", "-in", cellPath}); rc != 0 {
		t.Fatalf("score exited %d", rc)
	}
	cell := readStoredCell(t, cellPath)
	if cell.Readings.PlanCoverage == nil || cell.Readings.PlanCoverage.Named != 1 || cell.Readings.PlanCoverage.Fraction != 1 {
		t.Errorf("expected PlanCoverage recomputed from plan.md with no worktree needed, got %+v", cell.Readings.PlanCoverage)
	}
	if cell.Readings.FalseDone == nil || cell.Readings.FalseDone.NotMeasurable == "" {
		t.Errorf("expected FalseDone to be explicitly not_measurable (worktree gone), got %+v", cell.Readings.FalseDone)
	}
	if cell.Readings.Drift == nil || cell.Readings.Drift.NotMeasurable == "" {
		t.Errorf("expected Drift to be explicitly not_measurable (worktree gone), got %+v", cell.Readings.Drift)
	}
}

func TestCmdScore_MissingIn(t *testing.T) {
	if rc := run([]string{"score"}); rc != 2 {
		t.Errorf("score with no -in: exit %d, want 2", rc)
	}
}

func TestCmdLedger_MissingFlags(t *testing.T) {
	if rc := run([]string{"ledger"}); rc != 2 {
		t.Errorf("ledger with no flags: exit %d, want 2", rc)
	}
}

// repoRelative resolves a path relative to the fathom repo root, walking up
// from the test's working directory (cmd/scenariobench) — cmd tests run
// with cwd set to the package dir, two levels below the repo root.
func repoRelative(t *testing.T, parts ...string) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(wd, "..", "..")
	return filepath.Join(append([]string{root}, parts...)...)
}

// findStoredCell returns the single trial-1 cell file under the
// content-addressed cell store rooted at cellsDir, failing the test unless
// there is exactly one (this package's fixtures only ever run one cell).
func findStoredCell(t *testing.T, cellsDir string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(cellsDir, "*", "t1.json"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("expected exactly one stored cell under %s: matches=%v err=%v", cellsDir, matches, err)
	}
	return matches[0]
}

func readStoredCell(t *testing.T, path string) scenariobench.CellResult {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading stored cell %s: %v", path, err)
	}
	var cell scenariobench.CellResult
	if err := json.Unmarshal(raw, &cell); err != nil {
		t.Fatalf("parsing stored cell %s: %v", path, err)
	}
	return cell
}
