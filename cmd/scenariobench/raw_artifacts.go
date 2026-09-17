// raw_artifacts.go — O76: captureAgentDiff (main.go) writes a cell's
// agent.diff only to wt.CellDir/agent.diff, a directory under
// workspace/artifacts/scenariobench/worktrees/… that a LATER cell sharing
// the same (subject, base_commit, arm) resets out from under it, and that
// an operator's own cleanup can remove outright. Once that happens, a cell
// that turned out LOST (guards-pilot-7's own 31 cells: sandbox_escape,
// after real sessions had already run and really edited files) can never
// be re-scored — wave N20's RematerializeCellWorktree needs the diff and
// this cell will never have one again.
//
// persistRawCellArtifacts closes that gap by writing the same durable
// record into the cell's own RAW attempt directory
// (workspace/artifacts/scenariobench/raw/<run>/<task>-<arm>-t<n>/attempt-<k>/)
// instead — the one directory allocateRawAttemptDir (sessions.go) never
// reuses and this driver never resets. runOneCell calls this
// unconditionally, right after captureAgentDiff and BEFORE RunOracle, so it
// runs for EVERY cell regardless of what the caller's later leak/escape/
// LOST decision (runCellForReport's scenariobench.CellLost) turns out to
// be — that decision is exactly what this fix must survive.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/scenariobench"
)

// maxRawUntrackedSnapshotFileBytes caps how large a single untracked file
// persistRawCellArtifacts will copy into a cell's own raw
// untracked-snapshot/ directory (scenariobench.CellRawUntrackedSnapshotDir).
// The raw dir is meant to stay disk-small and durable; a multi-hundred-MB
// binary an agent generated is not worth an unbounded copy just to keep a
// rescoring path complete. A file over the cap is named in
// untracked-snapshot.json's own "skipped" list instead of silently
// vanishing. A var, not a const, so a test can shrink it rather than
// generate an oversized fixture file.
var maxRawUntrackedSnapshotFileBytes int64 = 10 << 20 // 10 MiB

// rawCellArtifacts is cell-artifacts.json's own shape: not consumed by
// RematerializeCellWorktree today, but the smallest useful record of what
// this raw copy leaves out (the instrument's own writes) and what base it
// was taken against — alongside agent.diff and untracked-snapshot/, the
// three things this function persists.
type rawCellArtifacts struct {
	BaseCommit           string   `json:"base_commit"`
	LocalBase            string   `json:"local_base"`
	InstrumentOwnedPaths []string `json:"instrument_owned_paths"`
}

// rawSnapshotSkip is one entry in untracked-snapshot.json's own "skipped"
// list — a binary file the stored agent.diff names as created or modified
// that untracked-snapshot/ does NOT carry, and why.
type rawSnapshotSkip struct {
	Path      string `json:"path"`
	Reason    string `json:"reason"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
}

// rawSnapshotManifest is untracked-snapshot.json's own shape: what the cap
// was at capture time, which binary paths were captured byte-identically,
// and which were skipped (and why) — so a later reader (a human, or
// RematerializeCellWorktree's own AgentDiffBinaryFilesMissingSnapshot
// check) never has to guess whether an absence is "nothing to capture" or
// "capture refused".
type rawSnapshotManifest struct {
	CapBytes int64             `json:"cap_bytes"`
	Captured []string          `json:"captured"`
	Skipped  []rawSnapshotSkip `json:"skipped,omitempty"`
}

// persistRawCellArtifacts writes into attemptDir (the raw attempt directory
// RunSessions allocated for this cell — see scenariobench.LatestAttemptDir)
// exactly what wave N20's RematerializeCellWorktree consumes to rebuild
// this cell's worktree from raw alone, once
// workspace/artifacts/scenariobench/worktrees/… has been cleaned:
//
//   - agent.diff (scenariobench.CellRawAgentDiffPath): a byte-for-byte copy
//     of diffPath, captureAgentDiff's own output — never recomputed, so the
//     raw copy always matches exactly what this cell was scored against.
//   - untracked-snapshot/ (scenariobench.CellRawUntrackedSnapshotDir),
//     plus its own untracked-snapshot.json manifest: a byte-identical copy
//     of every binary file the diff names as created or modified (a plain
//     `git diff` never embeds binary patch data — see
//     scenariobench.AgentDiffBinaryFilesMissingSnapshot), each capped at
//     maxRawUntrackedSnapshotFileBytes.
//   - cell-artifacts.json (rawCellArtifacts): the base commit, wt.LocalBase,
//     and the instrument-owned paths captureAgentDiff excluded.
//
// Best-effort throughout, exactly like captureAgentDiff/captureAgentPlan: a
// raw copy is a convenience for later re-scoring, never a precondition for
// the readings runOneCell already computed from the live worktree, so any
// failure here is reported on stderr and never fails the cell. diffPath ==
// "" (captureAgentDiff itself failed) skips the agent.diff/
// untracked-snapshot steps but still writes cell-artifacts.json.
func persistRawCellArtifacts(wt scenariobench.Worktree, attemptDir, diffPath, baseCommit string) {
	if attemptDir == "" {
		return
	}

	if diffPath != "" {
		if diffBytes, err := os.ReadFile(diffPath); err != nil {
			fmt.Fprintf(os.Stderr, "scenariobench: persisting raw agent.diff: reading %s: %v\n", diffPath, err)
		} else if err := os.WriteFile(scenariobench.CellRawAgentDiffPath(attemptDir), diffBytes, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "scenariobench: persisting raw agent.diff: %v\n", err)
		} else {
			persistRawUntrackedSnapshot(wt, attemptDir, diffBytes)
		}
	}

	owned, err := scenariobench.InstrumentOwnedPaths(wt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench: persisting cell-artifacts.json: %v\n", err)
		owned = nil
	}
	ownedList := make([]string, 0, len(owned))
	for p := range owned {
		ownedList = append(ownedList, p)
	}
	sort.Strings(ownedList)
	artifacts := rawCellArtifacts{BaseCommit: baseCommit, LocalBase: wt.LocalBase, InstrumentOwnedPaths: ownedList}
	if data, err := json.MarshalIndent(artifacts, "", "  "); err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench: marshaling cell-artifacts.json: %v\n", err)
	} else if err := os.WriteFile(filepath.Join(attemptDir, "cell-artifacts.json"), data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench: writing cell-artifacts.json: %v\n", err)
	}
}

// persistRawUntrackedSnapshot copies, byte-identically, every binary file
// diffBytes names as created or modified into attemptDir's own
// untracked-snapshot/ directory (scenariobench.CellRawUntrackedSnapshotDir),
// capped at maxRawUntrackedSnapshotFileBytes, and records what was captured
// and what was skipped (and why) in untracked-snapshot.json alongside it. A
// diff naming no binary change at all writes nothing — CellRawUntrackedSnapshotDir
// staying absent is itself the honest "nothing to capture" signal
// (rematerialize.go's own doc comment).
func persistRawUntrackedSnapshot(wt scenariobench.Worktree, attemptDir string, diffBytes []byte) {
	snapshotDir := scenariobench.CellRawUntrackedSnapshotDir(attemptDir)
	// Called against a snapshot dir that does not exist yet, this reports
	// every binary create/modify the diff names at all — exactly the set
	// this function is about to try to satisfy.
	binaryPaths := scenariobench.AgentDiffBinaryFilesMissingSnapshot(diffBytes, snapshotDir)
	if len(binaryPaths) == 0 {
		return
	}

	manifest := rawSnapshotManifest{CapBytes: maxRawUntrackedSnapshotFileBytes}
	for _, rel := range binaryPaths {
		src := filepath.Join(wt.Path, rel)
		info, err := os.Stat(src)
		if err != nil {
			manifest.Skipped = append(manifest.Skipped, rawSnapshotSkip{Path: rel, Reason: fmt.Sprintf("not found in worktree: %v", err)})
			continue
		}
		if info.Size() > maxRawUntrackedSnapshotFileBytes {
			manifest.Skipped = append(manifest.Skipped, rawSnapshotSkip{
				Path: rel, SizeBytes: info.Size(),
				Reason: fmt.Sprintf("exceeds the %d byte cap", maxRawUntrackedSnapshotFileBytes),
			})
			continue
		}
		data, err := os.ReadFile(src)
		if err != nil {
			manifest.Skipped = append(manifest.Skipped, rawSnapshotSkip{Path: rel, Reason: fmt.Sprintf("reading: %v", err)})
			continue
		}
		dest := filepath.Join(snapshotDir, rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			manifest.Skipped = append(manifest.Skipped, rawSnapshotSkip{Path: rel, Reason: fmt.Sprintf("creating snapshot dir: %v", err)})
			continue
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			manifest.Skipped = append(manifest.Skipped, rawSnapshotSkip{Path: rel, Reason: fmt.Sprintf("writing snapshot: %v", err)})
			continue
		}
		manifest.Captured = append(manifest.Captured, rel)
	}
	sort.Strings(manifest.Captured)
	sort.Slice(manifest.Skipped, func(i, j int) bool { return manifest.Skipped[i].Path < manifest.Skipped[j].Path })

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench: marshaling untracked-snapshot.json: %v\n", err)
		return
	}
	if err := os.WriteFile(filepath.Join(attemptDir, "untracked-snapshot.json"), data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "scenariobench: writing untracked-snapshot.json: %v\n", err)
	}
}
