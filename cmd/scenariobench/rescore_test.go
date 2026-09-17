package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/scenariobench"
)

// answerOnlyCorpusFixture writes a corpus with ONE task per id in
// taskIDs, all sharing the same answer-only oracle shape (SB-32
// lookups-vs.json's own shape: answer_all_of, no fail_to_pass/detectors) —
// scenariobench.NeedsLiveWorktree is false for every one of them, so
// rescoreOneCell never needs PrepareSubject to have ever run; repo_url/
// base_commit are therefore fixture placeholders, never a real clone.
func answerOnlyCorpusFixture(t *testing.T, dir string, taskIDs []string) string {
	t.Helper()
	var tasks []string
	for _, id := range taskIDs {
		tasks = append(tasks, `{
    "id": "`+id+`",
    "category": "bug_fix",
    "prompt": "where is fusion.New defined?",
    "derivation": "synthetic fixture for scenariobench's own rescore tests",
    "subject": {"repo_url": "https://example.invalid/repo.git", "base_commit": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"},
    "sessions": {"mode": "single", "count": 1, "max_turns": 5},
    "oracle": {"answer_all_of": ["fusion.New"]},
    "predictions": {"control": "no-effect (this is the control)"}
  }`)
	}
	corpus := `{
  "arms": {
    "control": {"wired": false, "instructions": "none", "surface": ""}
  },
  "tasks": [` + strings.Join(tasks, ",") + `],
  "criterion": {"id": "C1", "treatment": "control", "control": "control", "min_trials": 1}
}`
	path := filepath.Join(dir, "corpus.json")
	if err := os.WriteFile(path, []byte(corpus), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestCmdRun_RescoreLost_ReconstructsCellFromRaw is wave N18 defect 2's
// own required proof: a raw attempt dir holding one complete session
// stream (a Bash `cd` into the cell's own subject-worktree directory —
// the exact shape that false-positived SandboxEscapeScan before this
// wave's defect-1 fix) and NO stored cell record. `run -rescore-lost`
// must never launch a session (no -harness/-fake-streams wiring at all)
// and must produce a stored cell carrying the expected verdict
// (Success=true, the oracle's answer token is in the transcript) and
// rescored_from_raw=true.
func TestCmdRun_RescoreLost_ReconstructsCellFromRaw(t *testing.T) {
	dir := t.TempDir()
	corpusPath := answerOnlyCorpusFixture(t, dir, []string{"T1"})
	root := filepath.Join(dir, "fathom-root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	cellsDir := filepath.Join(dir, "cells")
	rawDir := filepath.Join(dir, "raw")
	runID := "run-1"

	absRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	task := scenariobench.Task{ID: "T1", Subject: scenariobench.Subject{RepoURL: "https://example.invalid/repo.git", BaseCommit: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"}}
	arm := scenariobench.Arm{Wired: false}
	worktreeHint := scenariobench.HistoricalWorktreePath(absRoot, task, arm)

	attemptDir := filepath.Join(scenariobench.CellRawBase(rawDir, runID, "T1", "control", 1), "attempt-1")
	if err := os.MkdirAll(attemptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stream := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu0","name":"Bash","input":{"command":"cd ` + worktreeHint + ` && grep -rn fusion.New ."}}],"usage":{"input_tokens":5,"output_tokens":5}}}` + "\n" +
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu0","content":"internal/fusion/registry.go:12:func New() *Registry {}\n"}]}}` + "\n" +
		`{"type":"result","is_error":false,"num_turns":1,"duration_ms":1,"total_cost_usd":0.001,"result":"fusion.New is defined in internal/fusion/registry.go","usage":{"input_tokens":5,"output_tokens":5}}` + "\n"
	if err := os.WriteFile(filepath.Join(attemptDir, "session-0-single.stream.jsonl"), []byte(stream), 0o644); err != nil {
		t.Fatal(err)
	}

	rc := run([]string{
		"run",
		"-corpus", corpusPath,
		"-root", root,
		"-cells", cellsDir,
		"-raw-dir", rawDir,
		"-run-id", runID,
		"-harness-version", "test-1",
		"-rescore-lost",
	})
	if rc != 0 {
		t.Fatalf("run -rescore-lost exited %d, want 0", rc)
	}

	cellPath := findStoredCell(t, cellsDir)
	cell := readStoredCell(t, cellPath)
	if !cell.RescoredFromRaw {
		t.Error("expected rescored_from_raw=true")
	}
	if !cell.Success {
		t.Errorf("expected Success=true (the transcript's own result names the gold token); oracle=%+v", cell.Oracle)
	}
	if len(cell.Sessions) != 1 {
		t.Fatalf("expected 1 reconstructed session, got %d", len(cell.Sessions))
	}
	if cell.Sessions[0].SandboxEscape {
		t.Error("expected no sandbox escape: the cd targets the cell's own historical subject-worktree directory")
	}
}

// TestCmdRun_RescoreLost_SubjectsPoolListingRecoveredEndToEnd is wave N19
// defect 1's wiring proof: a bare `ls` of the shared subjects/ pool root —
// ONE level above the cell's own worktree, never itself a safe dir the
// test hands in — must be recovered end-to-end through the REAL `run
// -rescore-lost` wiring (cmdRun -> rescoreCellForReport ->
// rescoreOneCell), which is the only place that actually COMPUTES
// scenariobench.HistoricalSubjectsDir(root) and adds it to the safe-dir
// list; a lower-level SandboxEscapeScan/RescoreSessionsFromRaw unit test
// that hands subjectsDir in directly would prove pathUnderAnyDir's own
// prefix logic but never prove this wiring exists at all.
func TestCmdRun_RescoreLost_SubjectsPoolListingRecoveredEndToEnd(t *testing.T) {
	dir := t.TempDir()
	corpusPath := answerOnlyCorpusFixture(t, dir, []string{"T4"})
	root := filepath.Join(dir, "fathom-root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	cellsDir := filepath.Join(dir, "cells")
	rawDir := filepath.Join(dir, "raw")
	runID := "run-1"

	absRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	subjectsDir := scenariobench.HistoricalSubjectsDir(absRoot)

	attemptDir := filepath.Join(scenariobench.CellRawBase(rawDir, runID, "T4", "control", 1), "attempt-1")
	if err := os.MkdirAll(attemptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// The ONLY tool call in this session names subjectsDir itself — never
	// the cell's own worktree/cellDir/rawBase — so this cell can only
	// stay unLOST if the driver's own wiring adds subjectsDir as a safe
	// dir on its own.
	stream := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu0","name":"Bash","input":{"command":"ls ` + subjectsDir + ` 2>/dev/null"}}],"usage":{"input_tokens":5,"output_tokens":5}}}` + "\n" +
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu0","content":"deadbeef\ndeadbeef-worktrees\n"}]}}` + "\n" +
		`{"type":"result","is_error":false,"num_turns":1,"duration_ms":1,"total_cost_usd":0.001,"result":"fusion.New is defined in internal/fusion/registry.go","usage":{"input_tokens":5,"output_tokens":5}}` + "\n"
	if err := os.WriteFile(filepath.Join(attemptDir, "session-0-single.stream.jsonl"), []byte(stream), 0o644); err != nil {
		t.Fatal(err)
	}

	rc := run([]string{
		"run",
		"-corpus", corpusPath,
		"-root", root,
		"-cells", cellsDir,
		"-raw-dir", rawDir,
		"-run-id", runID,
		"-harness-version", "test-1",
		"-rescore-lost",
	})
	if rc != 0 {
		t.Fatalf("run -rescore-lost exited %d, want 0", rc)
	}

	cellPath := findStoredCell(t, cellsDir)
	cell := readStoredCell(t, cellPath)
	if cell.Sessions[0].SandboxEscape {
		t.Error("expected no sandbox escape: a bare listing of the shared subjects/ pool root names only opaque directory names, and the driver's own wiring must add subjectsDir as a safe dir")
	}
	if !cell.Success {
		t.Errorf("expected Success=true; oracle=%+v", cell.Oracle)
	}
}

// TestCmdRun_RescoreLost_GenuineLeakStaysLost is the converse required by
// the same spec line: a raw attempt dir whose own tool result names a
// path OUTSIDE the cell's own worktree entirely (the operator's push
// clone) is a genuine sandbox escape and must stay LOST — never stored,
// rescore or not.
func TestCmdRun_RescoreLost_GenuineLeakStaysLost(t *testing.T) {
	dir := t.TempDir()
	corpusPath := answerOnlyCorpusFixture(t, dir, []string{"T2"})
	root := filepath.Join(dir, "fathom-root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	cellsDir := filepath.Join(dir, "cells")
	rawDir := filepath.Join(dir, "raw")
	runID := "run-1"

	attemptDir := filepath.Join(scenariobench.CellRawBase(rawDir, runID, "T2", "control", 1), "attempt-1")
	if err := os.MkdirAll(attemptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	escaped := filepath.Join(dir, "pushclone", "testdata", "scenariobench", "guards-fathom.json")
	stream := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu0","name":"Bash","input":{"command":"find ` + dir + ` -maxdepth 8 -iname \"*fathom*\""}}],"usage":{"input_tokens":5,"output_tokens":5}}}` + "\n" +
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu0","content":"` + escaped + `\n"}]}}` + "\n" +
		`{"type":"result","is_error":false,"num_turns":1,"duration_ms":1,"total_cost_usd":0.001,"result":"fusion.New is defined somewhere","usage":{"input_tokens":5,"output_tokens":5}}` + "\n"
	if err := os.WriteFile(filepath.Join(attemptDir, "session-0-single.stream.jsonl"), []byte(stream), 0o644); err != nil {
		t.Fatal(err)
	}

	stderr := captureStderr(t, func() {
		rc := run([]string{
			"run",
			"-corpus", corpusPath,
			"-root", root,
			"-cells", cellsDir,
			"-raw-dir", rawDir,
			"-run-id", runID,
			"-harness-version", "test-1",
			"-rescore-lost",
		})
		if rc != 3 {
			t.Fatalf("run -rescore-lost exited %d, want 3 (every selected cell LOST)", rc)
		}
	})
	if !strings.Contains(stderr, "LOST") {
		t.Errorf("expected a LOST line on stderr, got:\n%s", stderr)
	}
	if entries, _ := os.ReadDir(cellsDir); len(entries) != 0 {
		t.Errorf("expected no cell stored for a genuine escape, found %v", entries)
	}
}

// TestCmdRun_RescoreLost_NeverInvokesHarness proves the safety property
// the package comment claims: -rescore-lost with an invalid -harness
// (one resolveHarness would refuse) still succeeds — because rescore mode
// never calls resolveHarness or RunSessions at all.
func TestCmdRun_RescoreLost_NeverInvokesHarness(t *testing.T) {
	dir := t.TempDir()
	corpusPath := answerOnlyCorpusFixture(t, dir, []string{"T3"})
	root := filepath.Join(dir, "fathom-root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	cellsDir := filepath.Join(dir, "cells")
	rawDir := filepath.Join(dir, "raw")
	runID := "run-1"

	attemptDir := filepath.Join(scenariobench.CellRawBase(rawDir, runID, "T3", "control", 1), "attempt-1")
	if err := os.MkdirAll(attemptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stream := `{"type":"assistant","message":{"content":[{"type":"text","text":"looking it up"}],"usage":{"input_tokens":5,"output_tokens":5}}}` + "\n" +
		`{"type":"result","is_error":false,"num_turns":1,"duration_ms":1,"total_cost_usd":0.001,"result":"fusion.New is defined here","usage":{"input_tokens":5,"output_tokens":5}}` + "\n"
	if err := os.WriteFile(filepath.Join(attemptDir, "session-0-single.stream.jsonl"), []byte(stream), 0o644); err != nil {
		t.Fatal(err)
	}

	rc := run([]string{
		"run",
		"-corpus", corpusPath,
		"-root", root,
		"-cells", cellsDir,
		"-raw-dir", rawDir,
		"-run-id", runID,
		"-harness-version", "test-1",
		"-harness", "not-a-real-harness",
		"-rescore-lost",
	})
	if rc != 0 {
		t.Fatalf("run -rescore-lost exited %d, want 0 (an invalid -harness must never matter — no harness is invoked)", rc)
	}
}

// TestCmdRun_RescoreLost_ReasonOverwritesMatchingStoredCell is the OTHER
// case Feature 2's own spec names: a cell that DOES have a stored record,
// but whose own instrument_defect matches -rescore-reason, is rescored
// and its stored record overwritten — never left standing as a borrowed
// reading. A stored record whose instrument_defect does NOT match is left
// alone (borrowed), even under -rescore-lost -rescore-reason.
func TestCmdRun_RescoreLost_ReasonOverwritesMatchingStoredCell(t *testing.T) {
	dir := t.TempDir()
	corpusPath := answerOnlyCorpusFixture(t, dir, []string{"T4", "T5"})
	root := filepath.Join(dir, "fathom-root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	cellsDir := filepath.Join(dir, "cells")
	rawDir := filepath.Join(dir, "raw")
	runID := "run-1"

	corpus, sha, err := loadCorpus(corpusPath)
	if err != nil {
		t.Fatal(err)
	}
	fs := newFlagSet("test")
	cf := addCellFlags(fs)
	if err := fs.Parse([]string{"-harness-version", "test-1"}); err != nil {
		t.Fatal(err)
	}
	cells := planCells(corpus, cf, 1, sha)

	// Pre-store a "sound" cell for both T4 and T5, one marked with the
	// instrument defect being rescored, the other not.
	for _, cp := range cells {
		defect := ""
		if cp.task.ID == "T4" {
			defect = "sandbox_escape"
		}
		stale := scenariobench.CellResult{
			TaskID: cp.task.ID, Arm: cp.arm, Trial: cp.trial, CellKey: cp.key,
			RunID: runID, Success: false, InstrumentDefect: defect,
			CorpusSHA256: cp.inv.CorpusSHA256, ModelID: cp.inv.ModelID, HarnessVersion: cp.inv.HarnessVersion,
		}
		if err := scenariobench.StoreScenarioCell(cellsDir, stale); err != nil {
			t.Fatal(err)
		}

		attemptDir := filepath.Join(scenariobench.CellRawBase(rawDir, runID, cp.task.ID, cp.arm, cp.trial), "attempt-1")
		if err := os.MkdirAll(attemptDir, 0o755); err != nil {
			t.Fatal(err)
		}
		stream := `{"type":"assistant","message":{"content":[{"type":"text","text":"looking it up"}],"usage":{"input_tokens":5,"output_tokens":5}}}` + "\n" +
			`{"type":"result","is_error":false,"num_turns":1,"duration_ms":1,"total_cost_usd":0.001,"result":"fusion.New is defined here","usage":{"input_tokens":5,"output_tokens":5}}` + "\n"
		if err := os.WriteFile(filepath.Join(attemptDir, "session-0-single.stream.jsonl"), []byte(stream), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	rc := run([]string{
		"run",
		"-corpus", corpusPath,
		"-root", root,
		"-cells", cellsDir,
		"-raw-dir", rawDir,
		"-run-id", runID,
		"-harness-version", "test-1",
		"-rescore-lost",
		"-rescore-reason", "sandbox_escape",
	})
	if rc != 0 {
		t.Fatalf("run -rescore-lost -rescore-reason exited %d, want 0", rc)
	}

	for _, cp := range cells {
		cell := readStoredCell(t, scenariobench.ScenarioCellPath(cellsDir, cp.key, cp.trial))
		switch cp.task.ID {
		case "T4":
			if !cell.RescoredFromRaw || !cell.Success || cell.InstrumentDefect != "" {
				t.Errorf("T4: expected the matching-defect cell to be rescored and overwritten (success=true, defect cleared), got %+v", cell)
			}
		case "T5":
			if cell.RescoredFromRaw || cell.Success {
				t.Errorf("T5: expected the non-matching stored cell to be left alone (borrowed), got %+v", cell)
			}
		}
	}
}

// runGitForRescoreTest runs one git command in dir with a fixed test
// author/committer identity — this package (main) has no access to
// internal/scenariobench's own unexported gitFixtureRepo test helper (a
// _test.go-only helper in a different package), so this file grows its own
// entirely-local (no network) fixture builder for a repo-shaped subject.
func runGitForRescoreTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// detectorCorpusFixture writes a ONE-task corpus whose oracle carries a
// real detector (scenariobench.NeedsLiveWorktree is true — the exact shape
// this test is proving: guards-pilot-7's own oracle.detectors shape) that
// passes once, and only once, "newfile.txt" exists in the worktree — the
// live signal a rematerialized cell's own `git apply`'d agent.diff must
// actually produce for RunOracle to see.
func detectorCorpusFixture(t *testing.T, dir, repoPath, baseCommit string) string {
	t.Helper()
	corpus := `{
  "arms": {
    "control": {"wired": false, "instructions": "none", "surface": ""}
  },
  "tasks": [{
    "id": "D1",
    "category": "bug_fix",
    "prompt": "add newfile.txt",
    "derivation": "synthetic fixture for scenariobench's own rematerialize-rescore test",
    "subject": {"repo_url": "` + repoPath + `", "base_commit": "` + baseCommit + `"},
    "sessions": {"mode": "single", "count": 1, "max_turns": 5},
    "oracle": {"detectors": [{"name": "change-present", "command": "test -f newfile.txt", "expect_exit": 0, "completion": true}]},
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

// TestCmdRun_RescoreLost_RematerializesLiveWorktreeOracleAndScores is wave
// N20's own required failing-first proof: a raw attempt dir holding one
// complete session stream (recording a real Write tool_use, so AnyEdits
// holds) PLUS a stored agent.diff (captureAgentDiff's own `git add -N .`
// then `git diff` shape) for a tiny, entirely local fixture subject — and a
// task whose oracle carries a real detector, so NeedsLiveWorktree is true
// and the OLD (pre-wave-N20) code refused it outright as not rescorable.
// `run -rescore-lost` must now re-materialize a real worktree, apply the
// stored diff, and run the SAME detector a live cell would — producing a
// stored cell with Success=true, RescoredFromRaw=true, Rematerialized=true.
func TestCmdRun_RescoreLost_RematerializesLiveWorktreeOracleAndScores(t *testing.T) {
	dir := t.TempDir()
	repoPath := filepath.Join(dir, "subject-repo")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatal(err)
	}
	runGitForRescoreTest(t, repoPath, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(repoPath, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForRescoreTest(t, repoPath, "add", "base.txt")
	runGitForRescoreTest(t, repoPath, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "initial commit")
	baseCommit := runGitForRescoreTest(t, repoPath, "rev-parse", "HEAD")

	// The "agent's own edit", captured exactly like captureAgentDiff would
	// (git add -N . then git diff HEAD) — directly in repoPath's own
	// working tree. This never touches what ensureClone later reads (git
	// clone reads only committed objects, never working-tree state), so no
	// cleanup of repoPath is needed afterward.
	if err := os.WriteFile(filepath.Join(repoPath, "newfile.txt"), []byte("hello from the agent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForRescoreTest(t, repoPath, "add", "-N", ".")
	diffCmd := exec.Command("git", "diff", "HEAD")
	diffCmd.Dir = repoPath
	diffCmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	diff, err := diffCmd.Output()
	if err != nil {
		t.Fatalf("git diff HEAD: %v", err)
	}
	if len(diff) == 0 {
		t.Fatal("captured diff is empty; the fixture edit did not register")
	}

	corpusPath := detectorCorpusFixture(t, dir, repoPath, baseCommit)
	root := filepath.Join(dir, "fathom-root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	cellsDir := filepath.Join(dir, "cells")
	rawDir := filepath.Join(dir, "raw")
	runID := "run-1"

	attemptDir := filepath.Join(scenariobench.CellRawBase(rawDir, runID, "D1", "control", 1), "attempt-1")
	if err := os.MkdirAll(attemptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scenariobench.CellRawAgentDiffPath(attemptDir), diff, 0o644); err != nil {
		t.Fatal(err)
	}
	stream := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu0","name":"Write","input":{"file_path":"` + filepath.Join(repoPath, "newfile.txt") + `","content":"hello from the agent\n"}}],"usage":{"input_tokens":5,"output_tokens":5}}}` + "\n" +
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu0","content":"File created"}]}}` + "\n" +
		`{"type":"result","is_error":false,"num_turns":1,"duration_ms":1,"total_cost_usd":0.001,"result":"added newfile.txt","usage":{"input_tokens":5,"output_tokens":5}}` + "\n"
	if err := os.WriteFile(filepath.Join(attemptDir, "session-0-single.stream.jsonl"), []byte(stream), 0o644); err != nil {
		t.Fatal(err)
	}

	rc := run([]string{
		"run",
		"-corpus", corpusPath,
		"-root", root,
		"-cells", cellsDir,
		"-raw-dir", rawDir,
		"-run-id", runID,
		"-harness-version", "test-1",
		"-rescore-lost",
	})
	if rc != 0 {
		t.Fatalf("run -rescore-lost exited %d, want 0", rc)
	}

	cellPath := findStoredCell(t, cellsDir)
	cell := readStoredCell(t, cellPath)
	if !cell.RescoredFromRaw {
		t.Error("expected rescored_from_raw=true")
	}
	if !cell.Rematerialized {
		t.Error("expected rematerialized=true")
	}
	if !cell.Success {
		t.Errorf("expected Success=true (the rematerialized worktree's own newfile.txt satisfies the detector); oracle=%+v", cell.Oracle)
	}
	if len(cell.Oracle.Detectors) != 1 || cell.Oracle.Detectors[0].Violated {
		t.Errorf("expected the change-present detector to have run and passed against the rematerialized tree, got %+v", cell.Oracle.Detectors)
	}
	// The worktree must be gone afterward — disk-small, "one
	// re-materialised worktree at a time" (wave N20's own constraint).
	if cell.WorktreePath == "" {
		t.Fatal("expected a non-empty worktree_path on the stored cell")
	}
	if _, err := os.Stat(cell.WorktreePath); !os.IsNotExist(err) {
		t.Errorf("expected the rematerialized worktree to be deleted after scoring, got err=%v", err)
	}
}

// TestCmdRun_RescoreLost_LiveWorktreeOracleWithNoStoredDiffStaysNotRescorable
// is the negative half of the same spec line: a task whose oracle
// NeedsLiveWorktree, and a raw dir holding only a session stream (guards-
// pilot-7's own real shape — no agent.diff, no untracked-snapshot, nothing
// else) must stay LOST with a reason naming the missing diff — never
// guessed at from the stream's own Write tool_use payload, however
// tempting a "just replay the Edit/Write calls" shortcut looks.
func TestCmdRun_RescoreLost_LiveWorktreeOracleWithNoStoredDiffStaysNotRescorable(t *testing.T) {
	dir := t.TempDir()
	repoPath := filepath.Join(dir, "subject-repo")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatal(err)
	}
	runGitForRescoreTest(t, repoPath, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(repoPath, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForRescoreTest(t, repoPath, "add", "base.txt")
	runGitForRescoreTest(t, repoPath, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "initial commit")
	baseCommit := runGitForRescoreTest(t, repoPath, "rev-parse", "HEAD")

	corpusPath := detectorCorpusFixture(t, dir, repoPath, baseCommit)
	root := filepath.Join(dir, "fathom-root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	cellsDir := filepath.Join(dir, "cells")
	rawDir := filepath.Join(dir, "raw")
	runID := "run-1"

	attemptDir := filepath.Join(scenariobench.CellRawBase(rawDir, runID, "D1", "control", 1), "attempt-1")
	if err := os.MkdirAll(attemptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// No agent.diff written — the guards-pilot-7 real shape.
	stream := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu0","name":"Write","input":{"file_path":"` + filepath.Join(repoPath, "newfile.txt") + `","content":"hello from the agent\n"}}],"usage":{"input_tokens":5,"output_tokens":5}}}` + "\n" +
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu0","content":"File created"}]}}` + "\n" +
		`{"type":"result","is_error":false,"num_turns":1,"duration_ms":1,"total_cost_usd":0.001,"result":"added newfile.txt","usage":{"input_tokens":5,"output_tokens":5}}` + "\n"
	if err := os.WriteFile(filepath.Join(attemptDir, "session-0-single.stream.jsonl"), []byte(stream), 0o644); err != nil {
		t.Fatal(err)
	}

	stderr := captureStderr(t, func() {
		rc := run([]string{
			"run",
			"-corpus", corpusPath,
			"-root", root,
			"-cells", cellsDir,
			"-raw-dir", rawDir,
			"-run-id", runID,
			"-harness-version", "test-1",
			"-rescore-lost",
		})
		if rc != 3 {
			t.Fatalf("run -rescore-lost exited %d, want 3 (every selected cell LOST)", rc)
		}
	})
	if !strings.Contains(stderr, "no agent.diff was captured") {
		t.Errorf("expected a LOST line naming the missing agent.diff, got:\n%s", stderr)
	}
	if entries, _ := os.ReadDir(cellsDir); len(entries) != 0 {
		t.Errorf("expected no cell stored, found %v", entries)
	}
	// PrepareSubject must never even have been attempted for this cell.
	if entries, _ := os.ReadDir(filepath.Join(root, "workspace", "scenariobench", "subjects")); len(entries) != 0 {
		t.Errorf("expected no subject prep to have been attempted, found %v", entries)
	}
}
