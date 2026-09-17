package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/scenariobench"
)

// TestPersistRawCellArtifacts_CopiesAgentDiffAndCellArtifacts covers the
// ordinary case: a real captureAgentDiff output, copied byte-for-byte into
// the raw attempt dir, plus cell-artifacts.json naming the base commit,
// LocalBase, and the instrument-owned paths captureAgentDiff excluded.
func TestPersistRawCellArtifacts_CopiesAgentDiffAndCellArtifacts(t *testing.T) {
	wtDir := t.TempDir()
	runGit(t, wtDir, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(wtDir, "real.go"), []byte("package thing\n\nfunc Old() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, wtDir, "add", "-A")
	runGit(t, wtDir, "commit", "-q", "-m", "base")
	localBase := runGit(t, wtDir, "rev-parse", "HEAD")

	if err := os.WriteFile(filepath.Join(wtDir, "real.go"), []byte("package thing\n\nfunc New() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	wt := scenariobench.Worktree{Path: wtDir, CellDir: t.TempDir(), LocalBase: localBase}
	diffPath, _ := captureAgentDiff(t.Context(), wt)
	if diffPath == "" {
		t.Fatal("expected a non-empty diff path")
	}
	wantDiff, err := os.ReadFile(diffPath)
	if err != nil {
		t.Fatal(err)
	}

	attemptDir := t.TempDir()
	persistRawCellArtifacts(wt, attemptDir, diffPath, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")

	rawDiffPath := scenariobench.CellRawAgentDiffPath(attemptDir)
	gotDiff, err := os.ReadFile(rawDiffPath)
	if err != nil {
		t.Fatalf("expected agent.diff under the raw attempt dir: %v", err)
	}
	if string(gotDiff) != string(wantDiff) {
		t.Errorf("raw agent.diff = %q, want byte-identical copy of %q", gotDiff, wantDiff)
	}

	artifactsRaw, err := os.ReadFile(filepath.Join(attemptDir, "cell-artifacts.json"))
	if err != nil {
		t.Fatalf("expected cell-artifacts.json under the raw attempt dir: %v", err)
	}
	var artifacts rawCellArtifacts
	if err := json.Unmarshal(artifactsRaw, &artifacts); err != nil {
		t.Fatal(err)
	}
	if artifacts.BaseCommit != "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef" {
		t.Errorf("cell-artifacts.json base_commit = %q", artifacts.BaseCommit)
	}
	if artifacts.LocalBase != localBase {
		t.Errorf("cell-artifacts.json local_base = %q, want %q", artifacts.LocalBase, localBase)
	}
	if len(artifacts.InstrumentOwnedPaths) == 0 {
		t.Error("expected the two static instrument-owned entries (.fathomignore, .fathom/owned.json)")
	}

	// No binary file was ever named, so untracked-snapshot/ must stay
	// entirely absent — "nothing to capture" is itself the honest signal
	// (rematerialize.go's own CellRawUntrackedSnapshotDir doc comment).
	if _, err := os.Stat(scenariobench.CellRawUntrackedSnapshotDir(attemptDir)); !os.IsNotExist(err) {
		t.Errorf("expected no untracked-snapshot dir when the diff names no binary file, err=%v", err)
	}
}

// TestPersistRawCellArtifacts_CapturesUntrackedBinaryFile proves the
// byte-identical fallback RematerializeCellWorktree needs: a plain `git
// diff` never embeds binary patch data (it records only "Binary files ...
// differ"), so the raw copy must carry the actual bytes under
// untracked-snapshot/ for ApplyStoredAgentDiff to ever reproduce them.
func TestPersistRawCellArtifacts_CapturesUntrackedBinaryFile(t *testing.T) {
	wtDir := t.TempDir()
	runGit(t, wtDir, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(wtDir, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, wtDir, "add", "-A")
	runGit(t, wtDir, "commit", "-q", "-m", "base")
	localBase := runGit(t, wtDir, "rev-parse", "HEAD")

	binary := []byte{0x00, 0x01, 0x02, 'b', 'i', 'n', 'a', 'r', 'y'}
	if err := os.WriteFile(filepath.Join(wtDir, "blob.bin"), binary, 0o644); err != nil {
		t.Fatal(err)
	}

	wt := scenariobench.Worktree{Path: wtDir, CellDir: t.TempDir(), LocalBase: localBase}
	diffPath, _ := captureAgentDiff(t.Context(), wt)
	if diffPath == "" {
		t.Fatal("expected a non-empty diff path")
	}
	diffBytes, err := os.ReadFile(diffPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(diffBytes), "Binary files") {
		t.Fatalf("expected the diff to record blob.bin as a binary file, got:\n%s", diffBytes)
	}

	attemptDir := t.TempDir()
	persistRawCellArtifacts(wt, attemptDir, diffPath, localBase)

	snapshotPath := filepath.Join(scenariobench.CellRawUntrackedSnapshotDir(attemptDir), "blob.bin")
	got, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("expected blob.bin under untracked-snapshot/: %v", err)
	}
	if string(got) != string(binary) {
		t.Errorf("untracked-snapshot/blob.bin = %v, want byte-identical %v", got, binary)
	}

	manifestRaw, err := os.ReadFile(filepath.Join(attemptDir, "untracked-snapshot.json"))
	if err != nil {
		t.Fatalf("expected untracked-snapshot.json: %v", err)
	}
	var manifest rawSnapshotManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Captured) != 1 || manifest.Captured[0] != "blob.bin" {
		t.Errorf("untracked-snapshot.json captured = %v, want [blob.bin]", manifest.Captured)
	}
	if len(manifest.Skipped) != 0 {
		t.Errorf("expected nothing skipped, got %+v", manifest.Skipped)
	}

	// AgentDiffBinaryFilesMissingSnapshot (RematerializeCellWorktree's own
	// refusal check) must now report the file as satisfied.
	missing := scenariobench.AgentDiffBinaryFilesMissingSnapshot(diffBytes, scenariobench.CellRawUntrackedSnapshotDir(attemptDir))
	if len(missing) != 0 {
		t.Errorf("expected no missing binary files after the snapshot, got %v", missing)
	}
}

// TestPersistRawCellArtifacts_SkipsOversizedBinaryFile proves the
// documented cap: a binary file over maxRawUntrackedSnapshotFileBytes is
// never copied, and is instead named in untracked-snapshot.json's own
// "skipped" list with its size and the reason — never silently vanishing.
func TestPersistRawCellArtifacts_SkipsOversizedBinaryFile(t *testing.T) {
	orig := maxRawUntrackedSnapshotFileBytes
	maxRawUntrackedSnapshotFileBytes = 4
	defer func() { maxRawUntrackedSnapshotFileBytes = orig }()

	wtDir := t.TempDir()
	runGit(t, wtDir, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(wtDir, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, wtDir, "add", "-A")
	runGit(t, wtDir, "commit", "-q", "-m", "base")
	localBase := runGit(t, wtDir, "rev-parse", "HEAD")

	binary := []byte{0x00, 0x01, 0x02, 'b', 'i', 'n', 'a', 'r', 'y', 'd', 'a', 't', 'a'}
	if err := os.WriteFile(filepath.Join(wtDir, "blob.bin"), binary, 0o644); err != nil {
		t.Fatal(err)
	}

	wt := scenariobench.Worktree{Path: wtDir, CellDir: t.TempDir(), LocalBase: localBase}
	diffPath, _ := captureAgentDiff(t.Context(), wt)
	if diffPath == "" {
		t.Fatal("expected a non-empty diff path")
	}

	attemptDir := t.TempDir()
	persistRawCellArtifacts(wt, attemptDir, diffPath, localBase)

	if _, err := os.Stat(filepath.Join(scenariobench.CellRawUntrackedSnapshotDir(attemptDir), "blob.bin")); !os.IsNotExist(err) {
		t.Errorf("expected blob.bin NOT to be captured (over the cap), err=%v", err)
	}

	manifestRaw, err := os.ReadFile(filepath.Join(attemptDir, "untracked-snapshot.json"))
	if err != nil {
		t.Fatalf("expected untracked-snapshot.json even for an entirely-skipped snapshot: %v", err)
	}
	var manifest rawSnapshotManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Captured) != 0 {
		t.Errorf("expected nothing captured, got %v", manifest.Captured)
	}
	if len(manifest.Skipped) != 1 || manifest.Skipped[0].Path != "blob.bin" {
		t.Fatalf("expected blob.bin in the skipped list, got %+v", manifest.Skipped)
	}
	if manifest.Skipped[0].SizeBytes != int64(len(binary)) {
		t.Errorf("skipped size_bytes = %d, want %d", manifest.Skipped[0].SizeBytes, len(binary))
	}
	if !strings.Contains(manifest.Skipped[0].Reason, "cap") {
		t.Errorf("expected the skip reason to name the cap, got %q", manifest.Skipped[0].Reason)
	}

	missing := scenariobench.AgentDiffBinaryFilesMissingSnapshot(mustReadFile(t, diffPath), scenariobench.CellRawUntrackedSnapshotDir(attemptDir))
	if len(missing) != 1 || missing[0] != "blob.bin" {
		t.Errorf("expected blob.bin to still be reported missing a snapshot, got %v", missing)
	}
}

// TestPersistRawCellArtifacts_EmptyAttemptDirIsANoOp covers the case
// LatestAttemptDir's own doc comment names: no session ever ran (an
// impossible cell key, or a raw dir that was never allocated) — never a
// panic or a write into "".
func TestPersistRawCellArtifacts_EmptyAttemptDirIsANoOp(t *testing.T) {
	wt := scenariobench.Worktree{Path: t.TempDir(), CellDir: t.TempDir()}
	persistRawCellArtifacts(wt, "", "", "deadbeef")
	// No panic is the whole assertion; nothing to read back.
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// escapingStreamFixture writes a single-session stream whose own
// tool_result names an absolute path outside every safe dir
// (SandboxEscapeScan's own "/pushclone/" marker) — the live-path sibling
// of TestCmdRun_RescoreLost_GenuineLeakStaysLost's own fixture
// (rescore_test.go), used here to drive a LOST cell through the LIVE `run`
// path (never -rescore-lost) so persistRawCellArtifacts' own call site in
// runOneCell is what is under test.
func escapingStreamFixture(t *testing.T, dir, name, escapedPath string) {
	t.Helper()
	stream := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu0","name":"Bash","input":{"command":"find / -iname \"*fathom*\""}}],"usage":{"input_tokens":5,"output_tokens":5}}}` + "\n" +
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu0","content":"` + escapedPath + `\n"}]}}` + "\n" +
		`{"type":"result","is_error":false,"num_turns":1,"duration_ms":1,"total_cost_usd":0.001,"result":"done","usage":{"input_tokens":5,"output_tokens":5}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(stream), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestPersistRawCellArtifacts_RematerializeAcceptsTheRawCopyUnchanged is
// the round-trip proof the task's own spec line asks for:
// scenariobench.RematerializeCellWorktree — wave N20's own consumer of a
// cell's raw dir — must accept exactly what persistRawCellArtifacts writes,
// with NO change to RematerializeCellWorktree/ApplyStoredAgentDiff/
// CellRawAgentDiffPath/CellRawUntrackedSnapshotDir at all: this function
// targets those exact same helpers/paths, so the SAME "live cell, destroy
// the worktree, rematerialize from raw alone" round trip
// internal/scenariobench/rematerialize_test.go proves for a hand-copied
// diff must also hold end-to-end through the REAL production
// captureAgentDiff -> persistRawCellArtifacts pipeline.
func TestPersistRawCellArtifacts_RematerializeAcceptsTheRawCopyUnchanged(t *testing.T) {
	ctx := t.Context()
	repoPath, commit := gitFixtureSubject(t)
	root := t.TempDir()
	task := scenariobench.Task{ID: "T1", Subject: scenariobench.Subject{RepoURL: repoPath, BaseCommit: commit}}
	arm := scenariobench.Arm{Instructions: "none"}

	// The "live" cell: prepare once, let the agent edit a tracked file and
	// add a brand new untracked one, capture and persist the diff exactly
	// as runOneCell does, then destroy the worktree — the operator's own
	// cleanup, or a later cell resetting the shared pool out from under
	// this one.
	live, err := scenariobench.PrepareSubject(ctx, task, arm, root)
	if err != nil {
		t.Fatalf("PrepareSubject (live): %v", err)
	}
	if err := os.WriteFile(filepath.Join(live.Path, "thing.go"), []byte("package thing\n\nfunc Add(a, b int) int { return a + b + 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(live.Path, "newfile.txt"), []byte("brand new from the agent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	live.CellDir = t.TempDir()
	diffPath, _ := captureAgentDiff(ctx, live)
	if diffPath == "" {
		t.Fatal("expected a non-empty diff path")
	}

	attemptDir := t.TempDir()
	persistRawCellArtifacts(live, attemptDir, diffPath, commit)

	if err := scenariobench.StopWorktree(&live); err != nil {
		t.Fatalf("StopWorktree: %v", err)
	}
	if err := os.RemoveAll(live.Path); err != nil {
		t.Fatalf("removing live worktree (simulating operator cleanup): %v", err)
	}

	rematerialized, cleanup, ok, reason := scenariobench.RematerializeCellWorktree(ctx, root, task, arm, attemptDir)
	defer cleanup()
	if !ok {
		t.Fatalf("RematerializeCellWorktree: not ok: %s", reason)
	}

	got, err := os.ReadFile(filepath.Join(rematerialized.Path, "thing.go"))
	if err != nil {
		t.Fatalf("reading thing.go from the rematerialized tree: %v", err)
	}
	if !strings.Contains(string(got), "a + b + 1") {
		t.Errorf("thing.go = %q, want the agent's own edit", got)
	}
	got, err = os.ReadFile(filepath.Join(rematerialized.Path, "newfile.txt"))
	if err != nil {
		t.Fatalf("reading newfile.txt from the rematerialized tree: %v", err)
	}
	if string(got) != "brand new from the agent\n" {
		t.Errorf("newfile.txt = %q, want the agent's own new file", got)
	}
}

// TestCmdRun_PersistsRawAgentDiffEvenWhenCellIsLost is O76's own
// failing-first proof: the LIVE `run` path (never -rescore-lost) must
// persist agent.diff into the cell's own raw attempt dir — the one place
// wave N20's RematerializeCellWorktree can ever read it back from once
// workspace/artifacts/scenariobench/worktrees/ has been cleaned or reset by
// a later cell sharing the same (subject, base_commit, arm) —
// unconditionally and BEFORE the caller's own leak/escape/LOST decision.
// guards-pilot-7 lost 31 sessions exactly this way: real sessions ran,
// agent.diff was written only to wt.CellDir, and once that directory was
// gone nothing durable remained to rescore from.
func TestCmdRun_PersistsRawAgentDiffEvenWhenCellIsLost(t *testing.T) {
	dir := t.TempDir()
	repoURL, commit := gitFixtureSubject(t)
	corpusPath := writeCorpusFixture(t, dir, repoURL, commit)
	root := filepath.Join(dir, "fathom-root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	cellsDir := filepath.Join(dir, "cells")
	rawDir := filepath.Join(dir, "raw")
	runID := "run-1"

	streamsDir := t.TempDir()
	escaped := filepath.Join(dir, "pushclone", "testdata", "scenariobench", "guards-fathom.json")
	escapingStreamFixture(t, streamsDir, "escape.jsonl", escaped)

	rc := captureStderr(t, func() {
		got := run([]string{
			"run",
			"-corpus", corpusPath,
			"-root", root,
			"-cells", cellsDir,
			"-raw-dir", rawDir,
			"-run-id", runID,
			"-harness-version", "fake-1",
			"-harness", "fake",
			"-fake-streams-dir", streamsDir,
			"-fake-streams", `{"T1/control":"escape.jsonl"}`,
		})
		if got != 3 {
			t.Fatalf("run exited %d, want 3 (the only cell is LOST to a genuine sandbox escape)", got)
		}
	})
	if !strings.Contains(rc, "LOST") {
		t.Errorf("expected a LOST line on stderr, got:\n%s", rc)
	}

	// LOST cells are never stored — the whole reason O76 matters is that
	// this cell's only durable record is what we are about to check for
	// under the raw dir, never the cell store.
	if entries, _ := os.ReadDir(cellsDir); len(entries) != 0 {
		t.Errorf("expected no cell stored for a LOST cell, found %v", entries)
	}

	attemptDir := filepath.Join(scenariobench.CellRawBase(rawDir, runID, "T1", "control", 1), "attempt-1")
	diffPath := scenariobench.CellRawAgentDiffPath(attemptDir)
	if _, err := os.Stat(diffPath); err != nil {
		t.Fatalf("expected agent.diff under the raw attempt dir even for a LOST cell: %v", err)
	}
	if _, err := os.Stat(filepath.Join(attemptDir, "cell-artifacts.json")); err != nil {
		t.Errorf("expected cell-artifacts.json under the raw attempt dir: %v", err)
	}

	// The diff captured this way must be exactly what RematerializeCellWorktree
	// consumes: never refused for a missing/mismatched byte-identical
	// binary fallback (this fixture edits no binary file, so the check
	// must trivially pass).
	diffBytes := mustReadFile(t, diffPath)
	if missing := scenariobench.AgentDiffBinaryFilesMissingSnapshot(diffBytes, scenariobench.CellRawUntrackedSnapshotDir(attemptDir)); len(missing) != 0 {
		t.Errorf("expected no binary files named with no snapshot fallback, got %v", missing)
	}
}
