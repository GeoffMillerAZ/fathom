package scenariobench

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

var homeCorpusExcludePaths = []string{
	"testdata/contextbench/**",
	"testdata/scenariobench/**",
	"docs/bench/**",
}

// TestAnswerKeyLeakScan_DetectsExcludedPathInGrepResult is O53 defect 3's
// leak-scan proof: a Grep tool_result whose text names an excluded path
// must be caught even though prepare-time removal was supposed to have
// kept it out of the worktree entirely — the scan is the backstop for
// whatever removal missed (a symlink, a subject Setup that re-creates the
// file, a corpus author who forgot a glob).
func TestAnswerKeyLeakScan_DetectsExcludedPathInGrepResult(t *testing.T) {
	dir := t.TempDir()
	path := writeOverlapTranscript(t, dir, "leak.jsonl", []overlapRead{
		{Name: "Grep", File: "testdata", Result: "testdata/contextbench/tasks-home.json:3:  \"ground_truth\": \"fusion.New\""},
	})
	session := SessionResult{RawPath: path}
	if !AnswerKeyLeakScan(session, homeCorpusExcludePaths) {
		t.Error("expected a leak to be detected")
	}
}

// TestAnswerKeyLeakScan_NoLeakReturnsFalse covers the converse: ordinary
// tool results naming none of the excluded paths must not false-positive.
func TestAnswerKeyLeakScan_NoLeakReturnsFalse(t *testing.T) {
	dir := t.TempDir()
	path := writeOverlapTranscript(t, dir, "clean.jsonl", []overlapRead{
		{Name: "Read", File: "internal/core/model.go", Result: "package core\n\ntype Edge struct{}\n"},
	})
	session := SessionResult{RawPath: path}
	if AnswerKeyLeakScan(session, homeCorpusExcludePaths) {
		t.Error("expected no leak from an unrelated tool result")
	}
}

// TestAnswerKeyLeakScan_EmptyInputsAreFalse: no raw transcript, or no
// exclude_paths declared, is "not measurable", never a false positive —
// AND never a false "leak detected" that would wrongly void a cell whose
// task carries no exclusions at all (a task not derived from the home
// corpus).
func TestAnswerKeyLeakScan_EmptyInputsAreFalse(t *testing.T) {
	dir := t.TempDir()
	path := writeOverlapTranscript(t, dir, "x.jsonl", []overlapRead{
		{Name: "Grep", File: "testdata", Result: "testdata/contextbench/tasks-home.json:1: leak"},
	})
	if AnswerKeyLeakScan(SessionResult{RawPath: ""}, homeCorpusExcludePaths) {
		t.Error("expected false with no RawPath")
	}
	if AnswerKeyLeakScan(SessionResult{RawPath: path}, nil) {
		t.Error("expected false with no excludePaths")
	}
}

// TestAnswerKeyLeakScan_BashCatOfExcludedPathIsCaught: the same defect the
// vs-pilot-3 evidence names — a Bash tool result (a `cat`, a directory
// listing) can carry the excluded path just as readily as Grep/Read.
func TestAnswerKeyLeakScan_BashCatOfExcludedPathIsCaught(t *testing.T) {
	dir := t.TempDir()
	path := writeOverlapTranscript(t, dir, "bash-leak.jsonl", []overlapRead{
		{Name: "Bash", File: "n/a", Result: "$ cat testdata/contextbench/tasks-home.json\n{\"H1-T01\":{\"ground_truth\":\"fusion.New\"}}\n"},
	})
	session := SessionResult{RawPath: path}
	if !AnswerKeyLeakScan(session, homeCorpusExcludePaths) {
		t.Error("expected a leak to be detected in a Bash tool result")
	}
}

// TestExcludeNeedle covers the glob-to-literal conversion directly.
func TestExcludeNeedle(t *testing.T) {
	cases := []struct{ pattern, want string }{
		{"testdata/contextbench/**", "testdata/contextbench/"},
		{"docs/bench/**", "docs/bench/"},
		{"exact/file.json", "exact/file.json"},
	}
	for _, c := range cases {
		if got := excludeNeedle(c.pattern); got != c.want {
			t.Errorf("excludeNeedle(%q) = %q, want %q", c.pattern, got, c.want)
		}
	}
}

// TestDerivesFromHomeCorpus / TestExcludePathsCoverHomeCorpus cover the
// corpus-validation helpers directly (corpus_test.go covers them again
// end-to-end through LoadScenarioCorpus).
func TestDerivesFromHomeCorpus(t *testing.T) {
	yes := Task{Derivation: "Copied verbatim from testdata/contextbench/tasks-home.json id H1-T01."}
	no := Task{Derivation: "New task, not from tasks-home.json."}
	if !derivesFromHomeCorpus(yes) {
		t.Error("expected derivesFromHomeCorpus(yes) = true")
	}
	if derivesFromHomeCorpus(no) {
		t.Error("expected derivesFromHomeCorpus(no) = false")
	}
}

func TestExcludePathsCoverHomeCorpus(t *testing.T) {
	if missing := excludePathsCoverHomeCorpus(homeCorpusExcludePaths); len(missing) != 0 {
		t.Errorf("expected full coverage to report nothing missing, got %v", missing)
	}
	if missing := excludePathsCoverHomeCorpus([]string{"testdata/contextbench/**"}); len(missing) != 2 {
		t.Errorf("expected 2 missing globs, got %v", missing)
	}
	if missing := excludePathsCoverHomeCorpus(nil); len(missing) != 3 {
		t.Errorf("expected all 3 missing, got %v", missing)
	}
}

// TestRunSessions_AnswerKeyLeakLosesTheCell is the end-to-end proof: a
// session whose transcript leaks an excluded path gets AnswerKeyLeak set,
// and CellLost reports the cell lost because of it — exactly the "marks
// the cell answer_key_leak: true and LOST" the defect asks for.
func TestRunSessions_AnswerKeyLeakLosesTheCell(t *testing.T) {
	streamsTmp := t.TempDir()
	writeOverlapTranscript(t, streamsTmp, "leaky.jsonl", []overlapRead{
		{Name: "Grep", File: "testdata", Result: "testdata/contextbench/tasks-home.json:3: ground truth here"},
	})

	wtDir := t.TempDir()
	task := Task{
		ID:       "T1",
		Prompt:   "do the thing",
		Subject:  Subject{ExcludePaths: homeCorpusExcludePaths},
		Sessions: Sessions{Mode: ModeSingle, Count: 1, MaxTurns: 10},
	}
	wt := Worktree{Path: wtDir}
	h := &FakeHarness{
		StreamsDir: streamsTmp,
		TaskID:     "T1",
		ArmName:    "full",
		Streams:    map[string]string{"T1/full": "leaky.jsonl"},
	}
	opts := RunOptions{Trial: 1, Model: "sonnet", RawDir: filepath.Join(t.TempDir(), "raw")}

	results, err := RunSessions(context.Background(), h, task, "full", Arm{}, wt, opts)
	if err != nil {
		t.Fatalf("RunSessions: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 session, got %d", len(results))
	}
	if !results[0].AnswerKeyLeak {
		t.Error("expected AnswerKeyLeak = true")
	}
	reason := CellLost(results)
	if reason == "" {
		t.Fatal("expected CellLost to report this cell as lost")
	}
	if got := reason; got != "answer_key_leak: a tool result named a path subject.exclude_paths should have removed from the worktree" {
		t.Errorf("CellLost reason = %q", got)
	}
}

// TestRunSessions_NoLeakArmNotAffected is the converse of the end-to-end
// test: a clean transcript against the same excludePaths does not flag
// the session or lose the cell.
func TestRunSessions_NoLeakArmNotAffected(t *testing.T) {
	wtDir := t.TempDir()
	task := Task{
		ID:       "T1",
		Prompt:   "do the thing",
		Subject:  Subject{ExcludePaths: homeCorpusExcludePaths},
		Sessions: Sessions{Mode: ModeSingle, Count: 1, MaxTurns: 10},
	}
	wt := Worktree{Path: wtDir}
	h := &FakeHarness{
		StreamsDir: streamsDir,
		TaskID:     "T1",
		ArmName:    "full",
		Streams:    map[string]string{"T1/full": "single-session.jsonl"},
	}
	opts := RunOptions{Trial: 1, Model: "sonnet", RawDir: filepath.Join(t.TempDir(), "raw")}

	results, err := RunSessions(context.Background(), h, task, "full", Arm{}, wt, opts)
	if err != nil {
		t.Fatalf("RunSessions: %v", err)
	}
	if results[0].AnswerKeyLeak {
		t.Error("expected AnswerKeyLeak = false for a clean transcript")
	}
	if reason := CellLost(results); reason != "" {
		t.Errorf("expected the cell not to be lost, got %q", reason)
	}
}

// TestRemoveExcludedPaths_NestedPatternIsFound proves removeExcludedWalk's
// recursion reaches a pattern anchored below root's own top-level entries
// — not just entries directly under root, the shape every real exclusion
// glob in this corpus actually is ("testdata/contextbench/**" sits two
// levels below the worktree root).
func TestRemoveExcludedPaths_NestedPatternIsFound(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "testdata", "contextbench", "tasks-home.json")
	if err := os.MkdirAll(filepath.Dir(nested), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nested, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	kept := filepath.Join(root, "testdata", "keep.txt")
	if err := os.WriteFile(kept, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := removeExcludedPaths(root, []string{"testdata/contextbench/**"}); err != nil {
		t.Fatalf("removeExcludedPaths: %v", err)
	}
	if _, err := os.Stat(nested); !os.IsNotExist(err) {
		t.Errorf("expected %s to be removed, got err=%v", nested, err)
	}
	if _, err := os.Stat(kept); err != nil {
		t.Errorf("expected %s to survive: %v", kept, err)
	}
}

// TestRemoveExcludedPaths_EmptyPatternsIsNoop: a task with no exclusions
// declared (the ordinary case) must not touch the worktree at all.
func TestRemoveExcludedPaths_EmptyPatternsIsNoop(t *testing.T) {
	root := t.TempDir()
	kept := filepath.Join(root, "marker.txt")
	if err := os.WriteFile(kept, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := removeExcludedPaths(root, nil); err != nil {
		t.Fatalf("removeExcludedPaths: %v", err)
	}
	if _, err := os.Stat(kept); err != nil {
		t.Errorf("expected %s to survive a no-op call: %v", kept, err)
	}
}
