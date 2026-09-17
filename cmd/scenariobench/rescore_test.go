package main

import (
	"os"
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
