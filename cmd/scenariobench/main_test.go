package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/scenariobench"
)

// captureStderr redirects os.Stderr for the duration of fn and returns
// what was written to it — used to assert on `scenariobench score`'s own
// warning line, which (like every subcommand here) writes straight to
// os.Stderr rather than returning it as data.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	defer func() { os.Stderr = orig }()
	fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

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

// TestCmdPlan_LCACorpusFlagWiring is VB-25's own CLI wiring proof: `-corpus
// lca -dataset <path.jsonl>` builds the corpus in memory from the
// operator's own downloaded dataset instead of reading a committed corpus
// file — using the same fixture internal/scenariobench's own loader tests
// read (internal/scenariobench/testdata/lca/fixture.jsonl), so this test
// never downloads anything either. Spends nothing: `plan` inspects the
// corpus and the cell store only.
func TestCmdPlan_LCACorpusFlagWiring(t *testing.T) {
	dir := t.TempDir()
	datasetPath := repoRelative(t, "internal", "scenariobench", "testdata", "lca", "fixture.jsonl")
	cellsDir := filepath.Join(dir, "cells")

	var stdout bytes.Buffer
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	rc := run([]string{"plan", "-corpus", "lca", "-dataset", datasetPath, "-cells", cellsDir, "-harness-version", "test-1"})
	w.Close()
	os.Stdout = origStdout
	io.Copy(&stdout, r)

	if rc != 0 {
		t.Fatalf("plan -corpus lca exited %d, stdout:\n%s", rc, stdout.String())
	}
	// Three records in the fixture, two arms (fathom/control): 6 cells.
	if !strings.Contains(stdout.String(), "6 cell(s) — 6 would run, 0 would borrow") {
		t.Errorf("expected 6 cells listed (3 tasks x 2 arms) with no spend, got:\n%s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "workspace")); !os.IsNotExist(err) {
		t.Errorf("plan should spend nothing and create no workspace, but found one (err=%v)", err)
	}
}

// TestCmdPlan_LCACorpusRequiresDataset proves the sentinel refuses to run
// silently against nothing: -corpus lca with no -dataset is a usage
// error, never a corpus of zero tasks.
func TestCmdPlan_LCACorpusRequiresDataset(t *testing.T) {
	dir := t.TempDir()
	rc := run([]string{"plan", "-corpus", "lca", "-cells", filepath.Join(dir, "cells"), "-harness-version", "test-1"})
	if rc == 0 {
		t.Error("expected -corpus lca with no -dataset to fail")
	}
}

// TestPlanCells_SkipsDroppedTasks is RC-15's own proof: a task the corpus
// marks disposition=dropped (an oracle that cannot be won from the task
// text) never becomes a cell — plan/run/ledger all build their cell list
// from planCells, so none of them ever runs or reports on it.
func TestPlanCells_SkipsDroppedTasks(t *testing.T) {
	corpus := scenariobench.Corpus{
		Arms: map[string]scenariobench.Arm{"control": {}},
		Tasks: []scenariobench.Task{
			{ID: "kept", Sessions: scenariobench.Sessions{Mode: scenariobench.ModeSingle, Count: 1}},
			{ID: "dropped-one", Disposition: "dropped", Reason: "fail_to_pass names a package absent at base", Sessions: scenariobench.Sessions{Mode: scenariobench.ModeSingle, Count: 1}},
		},
	}
	fs := newFlagSet("test")
	cf := addCellFlags(fs)
	if err := fs.Parse([]string{"-harness-version", "v1"}); err != nil {
		t.Fatal(err)
	}
	cells := planCells(corpus, cf, 1, "sha")
	if len(cells) != 1 {
		t.Fatalf("expected only the kept task's cell, got %d: %+v", len(cells), cells)
	}
	if cells[0].task.ID != "kept" {
		t.Errorf("expected the surviving cell to be for task %q, got %q", "kept", cells[0].task.ID)
	}
}

// TestBatchCellPlans_NeverBatchesTwoTrialsOfTheSameArm is the -parallel-arms
// race fix's own proof: planCells emits cells arm-major within a task
// (every trial of arm A, then every trial of arm B), so a naive "same
// task.ID" contiguous grouping put every trial of the SAME arm in one
// batch — and PrepareSubject's worktree key (base_commit + armContentHash)
// carries no trial, so those trials would race on one worktree under
// -parallel-arms. batchCellPlans must instead put every ARM of one trial
// together, and never two trials of one arm together.
func TestBatchCellPlans_NeverBatchesTwoTrialsOfTheSameArm(t *testing.T) {
	task := scenariobench.Task{ID: "T1"}
	cells := []cellPlan{
		{task: task, arm: "A", trial: 1},
		{task: task, arm: "A", trial: 2},
		{task: task, arm: "B", trial: 1},
		{task: task, arm: "B", trial: 2},
	}
	batches := batchCellPlans(cells)
	if len(batches) != 2 {
		t.Fatalf("expected 2 batches (one per trial), got %d: %+v", len(batches), batches)
	}
	for _, b := range batches {
		seen := map[string]bool{}
		for _, cp := range b {
			key := fmt.Sprintf("%s/t%d", cp.arm, cp.trial)
			if seen[key] {
				t.Fatalf("batch %+v carries the same arm/trial twice", b)
			}
			seen[key] = true
			trial := b[0].trial
			if cp.trial != trial {
				t.Errorf("batch mixes trial %d with trial %d: %+v", trial, cp.trial, b)
			}
		}
	}
	// Batch 0 must be trial 1's two arms, batch 1 trial 2's two arms —
	// first-seen order of (task, trial), matching planCells' own emission
	// order closely (arm A's trial 1 appears before arm B's trial 1).
	if len(batches[0]) != 2 || batches[0][0].arm != "A" || batches[0][1].arm != "B" {
		t.Errorf("batches[0] = %+v, want arms [A B] at trial 1", batches[0])
	}
	if len(batches[1]) != 2 || batches[1][0].arm != "A" || batches[1][1].arm != "B" {
		t.Errorf("batches[1] = %+v, want arms [A B] at trial 2", batches[1])
	}
}

// TestBatchCellPlans_SeparatesDifferentTasks covers the ordinary case:
// cells from two different tasks never share a batch even at the same
// trial number.
func TestBatchCellPlans_SeparatesDifferentTasks(t *testing.T) {
	cells := []cellPlan{
		{task: scenariobench.Task{ID: "T1"}, arm: "A", trial: 1},
		{task: scenariobench.Task{ID: "T2"}, arm: "A", trial: 1},
	}
	batches := batchCellPlans(cells)
	if len(batches) != 2 {
		t.Fatalf("expected 2 batches (one per task), got %d: %+v", len(batches), batches)
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
	// single-session.jsonl edits foo.go, never a .md checklist, so no
	// BoxCheck is ever recorded — FalseDoneRate correctly reports this as
	// not_measurable (no claimed row to judge at all), not a false 0/0.
	if cell.Readings.FalseDone == nil {
		t.Error("expected Readings.FalseDone to be present")
	} else if cell.Readings.FalseDone.NotMeasurable != "no claimed row carries a symbol contract" {
		t.Errorf("expected FalseDone not_measurable (no box checks were ever recorded): got %q", cell.Readings.FalseDone.NotMeasurable)
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
	if rescored.Readings.FalseDone.NotMeasurable != "no claimed row carries a symbol contract" {
		t.Errorf("expected score's FalseDone reading to be recomputed the same way: got not_measurable=%q", rescored.Readings.FalseDone.NotMeasurable)
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

// runGit runs a git command in dir and fails the test on error, returning
// trimmed combined output — shared by the reconstruction tests below and
// gitFixtureSubject's own local closure covers the simpler single-commit
// case those don't need.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
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

// TestCmdScore_ReconstructsTreeWhenWorktreeGone covers SB-21 follow-up
// (public-pilot-2/reuse-pilot-1, 2026-09-13): a cell whose shared
// WorktreePath was already reset by a later cell must still recompute
// FalseDone/Drift, by rebuilding the final tree from SubjectClone +
// BaseCommit + AgentDiffPath rather than reporting a blanket
// not_measurable.
func TestCmdScore_ReconstructsTreeWhenWorktreeGone(t *testing.T) {
	repoPath, commit := gitFixtureSubject(t)

	// Build the "final tree" a real cell would have captured: a detached
	// worktree off the clone at base_commit, edited by the "agent", with
	// its diff captured the same way captureAgentDiff does.
	editWT := filepath.Join(t.TempDir(), "edit-wt")
	runGit(t, repoPath, "worktree", "add", "--detach", editWT, commit)
	addFn := "package thing\n\nfunc Add(a, b int) int { return a + b }\n\nfunc Multiply(a, b int) int { return a * b }\n"
	if err := os.WriteFile(filepath.Join(editWT, "thing.go"), []byte(addFn), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, editWT, "add", "-N", ".")
	diff := runGit(t, editWT, "diff")
	dir := t.TempDir()
	diffPath := filepath.Join(dir, "agent.diff")
	if err := os.WriteFile(diffPath, []byte(diff+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoPath, "worktree", "remove", "--force", editWT)

	coverPath := filepath.Join(dir, "coverage.out")
	cover := "mode: set\nscenariobenchsubject/thing.go:1.1,100.2 1 1\n"
	if err := os.WriteFile(coverPath, []byte(cover), 0o644); err != nil {
		t.Fatal(err)
	}

	cellPath := filepath.Join(dir, "cell.json")
	raw, _ := json.Marshal(map[string]any{
		"task_id":         "T1",
		"arm":             "control",
		"cell_key":        "k1",
		"run_id":          "run-1",
		"worktree_path":   filepath.Join(dir, "does-not-exist"), // reset by a later cell
		"subject_clone":   repoPath,
		"base_commit":     commit,
		"agent_diff_path": diffPath,
		"sessions": []map[string]any{
			{
				"index": 0, "role": "single", "files_edited": []string{"thing.go"},
				"box_checks": []map[string]any{{"session": 0, "turn": 1, "row_id": "r1", "symbol": "Multiply", "file": "NOTES.md"}},
			},
		},
		"oracle":   map[string]any{"passed": true, "cover_profile": coverPath},
		"readings": map[string]any{},
	})
	if err := os.WriteFile(cellPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	if rc := run([]string{"score", "-in", cellPath}); rc != 0 {
		t.Fatalf("score exited %d", rc)
	}
	cell := readStoredCell(t, cellPath)
	if cell.Readings.FalseDone == nil {
		t.Fatal("expected Readings.FalseDone to be present")
	}
	if cell.Readings.FalseDone.NotMeasurable != "" {
		t.Errorf("expected FalseDone measurable via the reconstructed tree, got not_measurable=%q", cell.Readings.FalseDone.NotMeasurable)
	}
	if cell.Readings.FalseDone.Claimed != 1 || cell.Readings.FalseDone.Refuted != 0 {
		t.Errorf("expected Multiply to be claimed and not refuted (added by the diff, covered), got %+v", cell.Readings.FalseDone)
	}

	// The temp worktree reconstructFinalTree created must be cleaned up —
	// the clone's worktree list must be back down to just itself.
	list := runGit(t, repoPath, "worktree", "list")
	if strings.Count(list, "\n") != 0 {
		t.Errorf("expected the reconstructed temp worktree to be removed, worktree list:\n%s", list)
	}
}

// TestCmdScore_ReconstructionFailsGracefully covers a BaseCommit that
// cannot be resolved in SubjectClone (a corrupted or wrong record):
// reconstruction must fail closed to the ordinary not_measurable, never
// panic or fabricate a tree.
func TestCmdScore_ReconstructionFailsGracefully(t *testing.T) {
	repoPath, _ := gitFixtureSubject(t)
	dir := t.TempDir()
	cellPath := filepath.Join(dir, "cell.json")
	raw, _ := json.Marshal(map[string]any{
		"task_id":         "T1",
		"arm":             "control",
		"cell_key":        "k1",
		"run_id":          "run-1",
		"worktree_path":   filepath.Join(dir, "does-not-exist"),
		"subject_clone":   repoPath,
		"base_commit":     "0000000000000000000000000000000000000000",
		"agent_diff_path": filepath.Join(dir, "agent.diff"),
		"sessions":        []map[string]any{{"index": 0, "role": "single"}},
		"oracle":          map[string]any{"passed": true},
		"readings":        map[string]any{},
	})
	if err := os.WriteFile(cellPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if rc := run([]string{"score", "-in", cellPath}); rc != 0 {
		t.Fatalf("score exited %d", rc)
	}
	cell := readStoredCell(t, cellPath)
	if cell.Readings.FalseDone == nil || cell.Readings.FalseDone.NotMeasurable == "" {
		t.Errorf("expected FalseDone not_measurable (reconstruction cannot resolve BaseCommit), got %+v", cell.Readings.FalseDone)
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

// TestCmdRunScoreLedger_SurfaceCallsAndUnusedFlag is item 6's end-to-end
// proof: a competitor arm whose MCP server connected but was never called
// (docs/bench/scenarios.md "Instrument truth", second documented defect)
// must carry ToolCalls/SurfaceCalls==0 on the stored cell, a
// surface-unused stderr warning from `score`, and a surface_unused=true
// row from `ledger`.
func TestCmdRunScoreLedger_SurfaceCallsAndUnusedFlag(t *testing.T) {
	dir := t.TempDir()
	repoURL, commit := gitFixtureSubject(t)
	corpus := `{
  "arms": {
    "comp": {
      "wired": false,
      "instructions": "plain",
      "surface": "graphify@v0.9.61",
      "mcp": {"graphify": {"command": "graphify", "args": ["--mcp"]}}
    }
  },
  "tasks": [{
    "id": "T1",
    "category": "obligations",
    "prompt": "what does foo do",
    "derivation": "synthetic fixture for scenariobench's own tests",
    "subject": {"repo_url": "` + repoURL + `", "base_commit": "` + commit + `"},
    "sessions": {"mode": "single", "count": 1, "max_turns": 5},
    "oracle": {"answer_all_of": ["fusion.New"]},
    "predictions": {"comp": "predicted to tie"}
  }],
  "criterion": {"id": "C1", "treatment": "comp", "control": "comp", "min_trials": 1}
}`
	corpusPath := filepath.Join(dir, "corpus.json")
	if err := os.WriteFile(corpusPath, []byte(corpus), 0o644); err != nil {
		t.Fatal(err)
	}
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
		"-fake-streams", `{"T1/comp":"mcp-connected-unused.jsonl"}`,
	})
	if rc != 0 {
		t.Fatalf("run exited %d, want 0 (the fixture's answer-only oracle matches)", rc)
	}

	cellPath := findStoredCell(t, cellsDir)
	cell := readStoredCell(t, cellPath)
	if cell.Surface != "graphify@v0.9.61" {
		t.Errorf("Surface = %q, want graphify@v0.9.61", cell.Surface)
	}
	if cell.Readings.ToolCalls["Read"] != 1 {
		t.Errorf("ToolCalls = %v, want Read=1", cell.Readings.ToolCalls)
	}
	if cell.Readings.SurfaceCalls != 0 {
		t.Errorf("SurfaceCalls = %d, want 0 (the fixture never calls mcp__graphify__*)", cell.Readings.SurfaceCalls)
	}

	stderr := captureStderr(t, func() {
		if rc := run([]string{"score", "-in", cellPath}); rc != 0 {
			t.Fatalf("score exited %d", rc)
		}
	})
	if !strings.Contains(stderr, `surface "graphify@v0.9.61" unused`) {
		t.Errorf("expected a surface-unused warning on stderr, got %q", stderr)
	}

	ledgerPath := filepath.Join(dir, "ledger.jsonl")
	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Bench\n\n## Scenario runs\n\n"+
		scenariobench.ScenarioLedgerBeginMarker+"\n\n"+scenariobench.ScenarioLedgerEndMarker+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc = run([]string{
		"ledger",
		"-corpus", corpusPath,
		"-cells", cellsDir,
		"-ledger", ledgerPath,
		"-readme", readmePath,
		"-run-id", "run-1",
		"-harness-version", "fake-1",
	})
	if rc != 0 {
		t.Fatalf("ledger exited %d, want 0", rc)
	}
	rows, err := scenariobench.LoadScenarioLedger(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 filed row, got %d", len(rows))
	}
	if !rows[0].SurfaceUnused {
		t.Error("expected the ledger row to carry surface_unused=true")
	}
	if rows[0].Readings.SurfaceCalls != 0 {
		t.Errorf("row SurfaceCalls = %d, want 0", rows[0].Readings.SurfaceCalls)
	}
	readme, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "0 (unused)") {
		t.Errorf("expected the rendered README table to flag the unused surface:\n%s", readme)
	}
}

// captureStdout redirects os.Stdout for the duration of fn and returns
// what it wrote. Tests in this package run serially (none call
// t.Parallel()), so the global swap is safe. The copy runs concurrently so
// fn cannot deadlock writing more than the pipe's own buffer.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		done <- buf.String()
	}()
	fn()
	w.Close()
	os.Stdout = old
	return <-done
}

// writeEscalationCorpusFixture writes a corpus with a REAL treatment/control
// pair ("full"/"control", unlike writeCorpusFixture's single-arm criterion)
// so EscalationPlan's delta math has two distinct arms to compare — plan
// -escalate never calls PrepareSubject/RunOracle, so subject.repo_url and
// base_commit need only be present, never reachable.
func writeEscalationCorpusFixture(t *testing.T, dir string) string {
	t.Helper()
	corpus := `{
  "arms": {
    "full": {"wired": true, "instructions": "stanza", "surface": "fathom"},
    "control": {"wired": false, "instructions": "none", "surface": ""}
  },
  "tasks": [{
    "id": "T1",
    "category": "bug_fix",
    "prompt": "fix it",
    "derivation": "synthetic fixture for scenariobench's own tests",
    "subject": {"repo_url": "https://example.invalid/subject.git", "base_commit": "deadbeef"},
    "sessions": {"mode": "single", "count": 1, "max_turns": 5},
    "oracle": {"fail_to_pass": ["TestPass"]},
    "predictions": {"full": "wins", "control": "no-effect (this is the control)"}
  }],
  "criterion": {"id": "C1", "treatment": "full", "control": "control", "min_trials": 1, "require_success_at_least_equal": true, "max_tokens_per_success_ratio": 1.5}
}`
	path := filepath.Join(dir, "corpus.json")
	if err := os.WriteFile(path, []byte(corpus), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestCmdPlan_Escalate is RC-14 (vision 28 §2.6): `scenariobench plan
// -escalate -margin` reads a ledger and prints the n=2 tasks whose
// treatment/control delta sits within margin of the corpus criterion's own
// thresholds — no cell store touched, no spend.
func TestCmdPlan_Escalate(t *testing.T) {
	dir := t.TempDir()
	corpusPath := writeEscalationCorpusFixture(t, dir)

	ledgerPath := filepath.Join(dir, "ledger.jsonl")
	rows := []scenariobench.ScenarioLedgerRow{
		// full 1/2 = 0.5, control 1/2 = 0.5 -> delta 0.0, within 0.15.
		{RunID: "run-1", TaskID: "T1", Arm: "full", Trials: 2, Successes: 1},
		{RunID: "run-1", TaskID: "T1", Arm: "control", Trials: 2, Successes: 1},
	}
	for _, r := range rows {
		if err := scenariobench.AppendScenarioLedger(ledgerPath, r); err != nil {
			t.Fatal(err)
		}
	}

	var rc int
	out := captureStdout(t, func() {
		rc = run([]string{"plan", "-corpus", corpusPath, "-escalate", "-ledger", ledgerPath, "-margin", "0.15"})
	})
	if rc != 0 {
		t.Fatalf("plan -escalate exited %d", rc)
	}
	if !strings.Contains(out, "T1") {
		t.Errorf("expected T1 named in -escalate output, got:\n%s", out)
	}
	// plan -escalate must not have created any subject clone or workspace —
	// it reads the ledger, never the cell store.
	if _, err := os.Stat(filepath.Join(dir, "workspace")); !os.IsNotExist(err) {
		t.Errorf("plan -escalate should spend nothing and create no workspace (err=%v)", err)
	}
}

func TestCmdPlan_Escalate_NoCandidates(t *testing.T) {
	dir := t.TempDir()
	corpusPath := writeEscalationCorpusFixture(t, dir)
	ledgerPath := filepath.Join(dir, "empty-ledger.jsonl")

	var rc int
	out := captureStdout(t, func() {
		rc = run([]string{"plan", "-corpus", corpusPath, "-escalate", "-ledger", ledgerPath})
	})
	if rc != 0 {
		t.Fatalf("plan -escalate exited %d, want 0 (an empty ledger is not an error)", rc)
	}
	if !strings.Contains(out, "no n=2 cell") {
		t.Errorf("expected a no-candidates message, got:\n%s", out)
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

// TestCmdLedger_TrialsDefaultCountsAllStoredTrials is item 2's own proof:
// writeCorpusFixture's own criterion registers min_trials=1, but the run
// below stores 2 trials (-trials 2). `ledger` invoked with NO -trials must
// still count both — never silently drop t2 the way a bare default of 1
// once did — and the summary line must name the row's own trial count.
func TestCmdLedger_TrialsDefaultCountsAllStoredTrials(t *testing.T) {
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
		"-trials", "2",
	})
	if rc != 0 {
		t.Fatalf("run exited %d, want 0", rc)
	}
	matches, err := filepath.Glob(filepath.Join(cellsDir, "*", "t*.json"))
	if err != nil || len(matches) != 2 {
		t.Fatalf("expected 2 stored trial cells, got %v (err=%v)", matches, err)
	}

	ledgerPath := filepath.Join(dir, "ledger.jsonl")
	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Bench\n\n## Scenario runs\n\n"+
		scenariobench.ScenarioLedgerBeginMarker+"\n\n"+scenariobench.ScenarioLedgerEndMarker+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout := captureStdout(t, func() {
		rc = run([]string{
			"ledger",
			"-corpus", corpusPath,
			"-cells", cellsDir,
			"-ledger", ledgerPath,
			"-readme", readmePath,
			"-run-id", "run-1",
			"-harness-version", "fake-1",
			// deliberately no -trials
		})
	})
	if rc != 0 {
		t.Fatalf("ledger exited %d, want 0: stdout=%s", rc, stdout)
	}
	rows, err := scenariobench.LoadScenarioLedger(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 filed row, got %d", len(rows))
	}
	if rows[0].Trials != 2 {
		t.Errorf("row.Trials = %d, want 2 (both stored trials, not the corpus's min_trials=1 floor)", rows[0].Trials)
	}
	if !strings.Contains(stdout, "T1/control:2") {
		t.Errorf("expected the summary line to print the row's own trial count, got %q", stdout)
	}
}

// TestCmdScore_GoldSourceToolCorpusFlag is item 3's own proof: without
// -corpus, `score` must leave a stored gold_source_tool exactly as it was
// (never overwritten to empty/not_measured) and say so on stderr; with
// -corpus, it must recompute it from the task's own gold against the
// cell's raw transcript.
func TestCmdScore_GoldSourceToolCorpusFlag(t *testing.T) {
	streamPath := repoRelative(t, "testdata", "scenariobench", "streams", "single-session.jsonl")

	newCell := func(t *testing.T, dir string) string {
		cellPath := filepath.Join(dir, "cell.json")
		raw, _ := json.Marshal(map[string]any{
			"task_id":  "T1",
			"arm":      "control",
			"cell_key": "k1",
			"run_id":   "run-1",
			"sessions": []map[string]any{
				{"index": 0, "role": "single", "raw_path": streamPath},
			},
			"oracle": map[string]any{"passed": true},
			"readings": map[string]any{
				"gold_source_tool":     "mcp__fathom__obligations",
				"gold_source_evidence": "some-prior-evidence",
			},
		})
		if err := os.WriteFile(cellPath, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		return cellPath
	}

	t.Run("without -corpus preserves the stored value", func(t *testing.T) {
		dir := t.TempDir()
		cellPath := newCell(t, dir)
		stderr := captureStderr(t, func() {
			if rc := run([]string{"score", "-in", cellPath}); rc != 0 {
				t.Fatalf("score exited %d", rc)
			}
		})
		if !strings.Contains(stderr, "gold_source_tool: not recomputed (no -corpus)") {
			t.Errorf("expected the not-recomputed stderr line, got %q", stderr)
		}
		cell := readStoredCell(t, cellPath)
		if cell.Readings.GoldSourceTool != "mcp__fathom__obligations" {
			t.Errorf("GoldSourceTool = %q, want the preserved stored value", cell.Readings.GoldSourceTool)
		}
		if cell.Readings.GoldSourceEvidence != "some-prior-evidence" {
			t.Errorf("GoldSourceEvidence = %q, want the preserved stored value", cell.Readings.GoldSourceEvidence)
		}
	})

	t.Run("with -corpus recomputes from the task's own gold", func(t *testing.T) {
		dir := t.TempDir()
		cellPath := newCell(t, dir)
		corpus := `{
  "arms": {"control": {"wired": false, "instructions": "none", "surface": ""}},
  "tasks": [{
    "id": "T1",
    "category": "bug_fix",
    "prompt": "fix it",
    "derivation": "synthetic fixture for scenariobench's own tests",
    "subject": {"repo_url": "https://example.invalid/subject.git", "base_commit": "deadbeef"},
    "sessions": {"mode": "single", "count": 1, "max_turns": 5},
    "oracle": {"fail_to_pass": ["TestPass"]},
    "gold": {"files": ["foo.go"]},
    "predictions": {"control": "no-effect (this is the control)"}
  }],
  "criterion": {"id": "C1", "treatment": "control", "control": "control", "min_trials": 1}
}`
		corpusPath := filepath.Join(dir, "corpus.json")
		if err := os.WriteFile(corpusPath, []byte(corpus), 0o644); err != nil {
			t.Fatal(err)
		}
		if rc := run([]string{"score", "-in", cellPath, "-corpus", corpusPath}); rc != 0 {
			t.Fatalf("score exited %d", rc)
		}
		cell := readStoredCell(t, cellPath)
		if cell.Readings.GoldSourceTool != "Read" {
			t.Errorf("GoldSourceTool = %q, want Read (single-session.jsonl reads /repo/foo.go)", cell.Readings.GoldSourceTool)
		}
		if cell.Readings.GoldSourceEvidence != "foo.go" {
			t.Errorf("GoldSourceEvidence = %q, want foo.go", cell.Readings.GoldSourceEvidence)
		}
	})
}

// TestCmdVoid_MissingFlags mirrors TestCmdLedger_MissingFlags.
func TestCmdVoid_MissingFlags(t *testing.T) {
	if rc := run([]string{"void"}); rc != 2 {
		t.Errorf("void with no flags: exit %d, want 2", rc)
	}
}

// scenarioLedgerReadmeFixture writes a minimal README carrying the
// marker-fenced scenario-ledger section RewriteScenarioLedgerSection
// requires, and returns its path.
func scenarioLedgerReadmeFixture(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "README.md")
	content := "# bench\n\n" +
		"<!-- BEGIN scenario ledger (task bench-scenario-ledger) -->\n" +
		"(placeholder)\n" +
		"<!-- END scenario ledger -->\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestCmdVoid_RefusesWhenRowAbsent is O53 defect 7's own refusal proof at
// the CLI layer: no ledger row for (run_id, task_id, arm) is a non-zero
// exit, and neither the ledger nor the README are touched.
func TestCmdVoid_RefusesWhenRowAbsent(t *testing.T) {
	dir := t.TempDir()
	ledgerPath := filepath.Join(dir, "ledger.jsonl")
	if err := scenariobench.AppendScenarioLedger(ledgerPath, scenariobench.ScenarioLedgerRow{RunID: "run-1", TaskID: "T1", Arm: "full"}); err != nil {
		t.Fatal(err)
	}
	readmePath := scenarioLedgerReadmeFixture(t, dir)
	readmeBefore, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatal(err)
	}

	rc := run([]string{
		"void",
		"-ledger", ledgerPath,
		"-readme", readmePath,
		"-cells", filepath.Join(dir, "cells"),
		"-corpus-glob", filepath.Join(dir, "no-such-corpus", "*.json"),
		"-run-id", "run-1", "-task", "T1", "-arm", "control", // "control" was never filed
		"-reason", "should be refused",
	})
	if rc == 0 {
		t.Fatal("expected a non-zero exit voiding a row that was never filed")
	}

	rows, err := scenariobench.LoadScenarioLedger(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].InstrumentDefect != "" {
		t.Errorf("expected the ledger untouched by a refused void, got %+v", rows)
	}
	readmeAfter, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(readmeBefore) != string(readmeAfter) {
		t.Error("expected the README untouched by a refused void")
	}
}

// TestCmdVoid_MarksRowCellAndRerendersReadme is O53 defect 7's end-to-end
// proof: a filed ledger row AND its stored cell record both carry the
// void reason afterward, and the README's marker-fenced section is
// re-rendered exactly as `ledger` does (the table now shows VOIDED).
func TestCmdVoid_MarksRowCellAndRerendersReadme(t *testing.T) {
	dir := t.TempDir()
	ledgerPath := filepath.Join(dir, "ledger.jsonl")
	if err := scenariobench.AppendScenarioLedger(ledgerPath, scenariobench.ScenarioLedgerRow{
		RunID: "run-1", TaskID: "T1", Arm: "full", Trials: 3, Successes: 3, ModelID: "sonnet",
	}); err != nil {
		t.Fatal(err)
	}
	readmePath := scenarioLedgerReadmeFixture(t, dir)

	cellsDir := filepath.Join(dir, "cells")
	inv := scenariobench.CellInvariants{
		CorpusSHA256: "abc123", TaskID: "T1", Arm: "full", ModelID: "sonnet",
		HarnessVersion: "2.1.0", Scoring: "oracle-v1", PromptSubject: "x",
	}
	key := scenariobench.ScenarioCellKey(inv)
	if err := scenariobench.StoreScenarioCell(cellsDir, scenariobench.CellResult{
		TaskID: "T1", Arm: "full", Trial: 1, CellKey: key, RunID: "run-1", Success: true,
	}); err != nil {
		t.Fatal(err)
	}

	rc := run([]string{
		"void",
		"-ledger", ledgerPath,
		"-readme", readmePath,
		"-cells", cellsDir,
		"-corpus-glob", filepath.Join(dir, "no-such-corpus", "*.json"),
		"-run-id", "run-1", "-task", "T1", "-arm", "full",
		"-reason", "the operator daemon was reachable",
	})
	if rc != 0 {
		t.Fatalf("void exited %d, want 0", rc)
	}

	rows, err := scenariobench.LoadScenarioLedger(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].InstrumentDefect != "the operator daemon was reachable" {
		t.Fatalf("expected the ledger row voided, got %+v", rows)
	}

	cell := readStoredCell(t, scenariobench.ScenarioCellPath(cellsDir, key, 1))
	if cell.InstrumentDefect != "the operator daemon was reachable" {
		t.Errorf("cell InstrumentDefect = %q, want it voided too", cell.InstrumentDefect)
	}
	if !cell.Success {
		t.Error("expected every other field on the cell record to survive untouched")
	}

	readme, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "VOIDED (the operator daemon was reachable)") {
		t.Errorf("expected the re-rendered README table to show the voided row, got:\n%s", readme)
	}
}
