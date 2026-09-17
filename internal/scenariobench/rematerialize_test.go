package scenariobench

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// captureRawDiffForTest mirrors cmd/scenariobench/main.go's own
// captureAgentDiff (unimportable here — main is package main): `git add -N
// .` so an untracked new file shows up as a new-file hunk, then `git diff`
// against wt.LocalBase (covering everything changed since PrepareSubject
// left the tree at its own root commit), then the SAME instrument-owned
// filtering captureAgentDiff applies (StripInstrumentRegions,
// InstrumentOwnedPaths/FilterInstrumentOwnedDiff) — even an UNWIRED arm's
// PrepareSubject writes RenderArm's own instrument files (.claude/settings.json,
// .claude/scenariobench-mcp.json), which a re-materialized cell's OWN fresh
// PrepareSubject call also writes; an unfiltered diff would then try to
// recreate them and `git apply` would refuse with "already exists".
func captureRawDiffForTest(t *testing.T, wt Worktree) []byte {
	t.Helper()
	add := exec.Command("git", "add", "-N", ".")
	add.Dir = wt.Path
	if out, err := add.CombinedOutput(); err != nil {
		t.Fatalf("git add -N .: %v (%s)", err, out)
	}
	diff := exec.Command("git", "diff", wt.LocalBase)
	diff.Dir = wt.Path
	out, err := diff.Output()
	if err != nil {
		t.Fatalf("git diff %s: %v", wt.LocalBase, err)
	}
	ctx := context.Background()
	out, _ = StripInstrumentRegions(ctx, wt, out)
	owned, err := InstrumentOwnedPaths(wt)
	if err != nil {
		t.Fatalf("InstrumentOwnedPaths: %v", err)
	}
	gitIgnored := GitIgnoredWorkspacePaths(ctx, wt, out)
	out, _ = FilterInstrumentOwnedDiff(out, owned, gitIgnored)
	return out
}

// TestRematerializeCellWorktree_AppliesStoredDiffOntoFreshPrepareSubject is
// this wave's own failing-first proof: a raw dir holding a stored
// agent.diff (captureAgentDiff's own format) for a tiny, entirely local
// fixture subject — PrepareSubject's clone/worktree-add path needs no
// network — must, after the ORIGINAL live worktree is deleted exactly as an
// operator reclaiming disk would do, still reproduce the agent's own final
// tree byte for byte when re-materialized from raw alone.
func TestRematerializeCellWorktree_AppliesStoredDiffOntoFreshPrepareSubject(t *testing.T) {
	repoPath, commit := gitFixtureRepo(t)
	root := t.TempDir()
	task := Task{ID: "T1", Subject: Subject{RepoURL: repoPath, BaseCommit: commit}}
	arm := Arm{Instructions: "none"}
	ctx := context.Background()

	// The "live" cell: prepare once, let the agent edit a tracked file and
	// add a brand new untracked one, capture the diff exactly as a live
	// cell would, then destroy the worktree — the operator's own cleanup,
	// wave N20's own scenario.
	live, err := PrepareSubject(ctx, task, arm, root)
	if err != nil {
		t.Fatalf("PrepareSubject (live): %v", err)
	}
	writeTestFile(t, filepath.Join(live.Path, "marker.txt"), "hello v2 from the agent\n")
	writeTestFile(t, filepath.Join(live.Path, "newfile.txt"), "brand new from the agent\n")
	diff := captureRawDiffForTest(t, live)
	if len(diff) == 0 {
		t.Fatal("captured diff is empty; the fixture edit did not register")
	}
	if err := StopWorktree(&live); err != nil {
		t.Fatalf("StopWorktree: %v", err)
	}
	if err := os.RemoveAll(live.Path); err != nil {
		t.Fatalf("removing live worktree (simulating operator cleanup): %v", err)
	}

	attemptDir := t.TempDir()
	if err := os.WriteFile(CellRawAgentDiffPath(attemptDir), diff, 0o644); err != nil {
		t.Fatal(err)
	}

	rematerialized, cleanup, ok, reason := RematerializeCellWorktree(ctx, root, task, arm, attemptDir)
	defer cleanup()
	if !ok {
		t.Fatalf("RematerializeCellWorktree: not ok: %s", reason)
	}

	got, err := os.ReadFile(filepath.Join(rematerialized.Path, "marker.txt"))
	if err != nil {
		t.Fatalf("reading marker.txt from the rematerialized tree: %v", err)
	}
	if string(got) != "hello v2 from the agent\n" {
		t.Errorf("marker.txt = %q, want the agent's own edit", got)
	}
	got, err = os.ReadFile(filepath.Join(rematerialized.Path, "newfile.txt"))
	if err != nil {
		t.Fatalf("reading newfile.txt from the rematerialized tree: %v", err)
	}
	if string(got) != "brand new from the agent\n" {
		t.Errorf("newfile.txt = %q, want the agent's own new file", got)
	}

	// The cleanup must actually delete the worktree — disk-small, "one
	// re-materialised worktree at a time" (wave N20's own operating
	// constraint).
	cleanup()
	if _, err := os.Stat(rematerialized.Path); !os.IsNotExist(err) {
		t.Errorf("expected cleanup to remove the rematerialized worktree, got err=%v", err)
	}
}

// TestRematerializeCellWorktree_NoStoredDiffIsNotOK is the "never guess"
// half of the same spec line: an attempt dir with no agent.diff at all
// (guards-pilot-7's own real shape — session streams and mcp.json only)
// must refuse re-materialization outright rather than fabricate a tree from
// the session stream's own Edit/Write tool_use payloads (a Bash call in the
// same session could have mutated a tracked file too, which no structured
// tool_use event would show).
func TestRematerializeCellWorktree_NoStoredDiffIsNotOK(t *testing.T) {
	repoPath, commit := gitFixtureRepo(t)
	root := t.TempDir()
	task := Task{ID: "T1", Subject: Subject{RepoURL: repoPath, BaseCommit: commit}}
	arm := Arm{Instructions: "none"}
	attemptDir := t.TempDir() // no agent.diff written

	wt, cleanup, ok, reason := RematerializeCellWorktree(context.Background(), root, task, arm, attemptDir)
	defer cleanup()
	if ok {
		t.Fatalf("expected not ok with no stored agent.diff, got wt=%+v", wt)
	}
	if !strings.Contains(reason, "no agent.diff was captured") {
		t.Errorf("reason = %q, want it to name the missing agent.diff", reason)
	}
	// PrepareSubject must never even have been attempted: nothing under
	// root's own worktrees pool.
	if entries, _ := os.ReadDir(filepath.Join(root, "workspace", "scenariobench", "subjects")); len(entries) != 0 {
		t.Errorf("expected no subject prep to have been attempted, found %v", entries)
	}
}

// TestRematerializeCellWorktree_EmptyDiffIsAZeroEditCell proves a
// zero-byte agent.diff (captureAgentDiff's own shape when the agent
// touched nothing at all) is a real, scorable outcome — never confused
// with "nothing was captured".
func TestRematerializeCellWorktree_EmptyDiffIsAZeroEditCell(t *testing.T) {
	repoPath, commit := gitFixtureRepo(t)
	root := t.TempDir()
	task := Task{ID: "T1", Subject: Subject{RepoURL: repoPath, BaseCommit: commit}}
	arm := Arm{Instructions: "none"}
	attemptDir := t.TempDir()
	if err := os.WriteFile(CellRawAgentDiffPath(attemptDir), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	wt, cleanup, ok, reason := RematerializeCellWorktree(context.Background(), root, task, arm, attemptDir)
	defer cleanup()
	if !ok {
		t.Fatalf("expected ok for an empty (zero-edit) diff, got reason=%q", reason)
	}
	got, err := os.ReadFile(filepath.Join(wt.Path, "marker.txt"))
	if err != nil {
		t.Fatalf("reading marker.txt: %v", err)
	}
	if string(got) != "hello from the fixture subject\n" {
		t.Errorf("marker.txt = %q, want the fixture's own unedited content", got)
	}
}

// TestAgentDiffBinaryFilesMissingSnapshot_FlagsAndClearsOnSnapshot proves
// the binary-file gap check both fires (no snapshot) and clears (a matching
// snapshot file present) — the "never guess" half for a byte range a plain
// `git diff` (no --binary) can never carry at all.
func TestAgentDiffBinaryFilesMissingSnapshot_FlagsAndClearsOnSnapshot(t *testing.T) {
	diff := []byte(`diff --git a/img.png b/img.png
new file mode 100644
index 0000000..abcdef1
Binary files /dev/null and b/img.png differ
`)
	snapshotDir := t.TempDir()

	missing := AgentDiffBinaryFilesMissingSnapshot(diff, snapshotDir)
	if len(missing) != 1 || missing[0] != "img.png" {
		t.Fatalf("missing = %v, want [img.png]", missing)
	}

	if err := os.WriteFile(filepath.Join(snapshotDir, "img.png"), []byte("\x89PNG\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing = AgentDiffBinaryFilesMissingSnapshot(diff, snapshotDir)
	if len(missing) != 0 {
		t.Errorf("missing = %v, want none once the snapshot covers it", missing)
	}
}

// TestAgentDiffBinaryFilesMissingSnapshot_DeletionNeedsNoSnapshot: a binary
// file the agent DELETED ("and /dev/null differ") needs no snapshot fallback
// at all — the diff alone already says to remove it.
func TestAgentDiffBinaryFilesMissingSnapshot_DeletionNeedsNoSnapshot(t *testing.T) {
	diff := []byte(`diff --git a/old.bin b/old.bin
deleted file mode 100644
index abcdef1..0000000
Binary files a/old.bin and /dev/null differ
`)
	missing := AgentDiffBinaryFilesMissingSnapshot(diff, t.TempDir())
	if len(missing) != 0 {
		t.Errorf("missing = %v, want none for a binary deletion", missing)
	}
}

// TestApplyStoredAgentDiff_OverlaysUntrackedSnapshotOnTopOfDiff proves the
// untracked-snapshot fallback actually lands its bytes: a diff that could
// not carry a binary file's content is applied first (creating the file
// with WRONG/empty content, as `git apply` does for a binary hunk with no
// patch data), then the snapshot's own byte-identical copy overwrites it.
func TestApplyStoredAgentDiff_OverlaysUntrackedSnapshotOnTopOfDiff(t *testing.T) {
	repoPath, commit := gitFixtureRepo(t)
	root := t.TempDir()
	task := Task{ID: "T1", Subject: Subject{RepoURL: repoPath, BaseCommit: commit}}
	arm := Arm{Instructions: "none"}
	wt, err := PrepareSubject(context.Background(), task, arm, root)
	if err != nil {
		t.Fatalf("PrepareSubject: %v", err)
	}
	defer StopWorktree(&wt)

	attemptDir := t.TempDir()
	// No agent.diff at all this time — only a snapshot, proving
	// ApplyStoredAgentDiff applies a snapshot-only reconstruction too (a
	// text-diff-less cell whose only change was an untracked binary file).
	snapshotDir := CellRawUntrackedSnapshotDir(attemptDir)
	if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapshotDir, "img.bin"), []byte("real bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ApplyStoredAgentDiff(context.Background(), wt, attemptDir); err != nil {
		t.Fatalf("ApplyStoredAgentDiff: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(wt.Path, "img.bin"))
	if err != nil {
		t.Fatalf("reading img.bin: %v", err)
	}
	if string(got) != "real bytes" {
		t.Errorf("img.bin = %q, want the snapshot's own bytes", got)
	}
}
