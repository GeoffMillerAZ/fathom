// rematerialize.go — wave N20: RescoreSessionsFromRaw's own sibling for a
// task whose oracle NeedsLiveWorktree (rescore.go's own doc comment — go
// test, forbidden-path diffs, refusal checks, wave N14's post-edit
// detectors) — none of which RunOracle can answer against the zero
// Worktree{} rescoreOneCell otherwise hands it (cmd/scenariobench/rescore.go).
// RematerializeCellWorktree rebuilds that worktree deterministically — the
// SAME PrepareSubject a live cell used (same sparse exclusions, same
// instrument writes, same LocalBase semantics) — then applies the cell's
// own stored agent.diff (captureAgentDiff's own format, main.go: `git add
// -N .` then `git diff LocalBase`, so a new untracked file already carries
// full content) on top, reproducing exactly the tree the driver's own
// captureAgentDiff walked at live scoring time. A cell whose raw dir holds
// no agent.diff at all, or whose diff names a binary file with no
// byte-identical untracked-snapshot fallback (a plain `git diff` never
// embeds binary patch data), is never guessed at: the caller reports it not
// rescorable instead of trusting a partial or absent reconstruction.
package scenariobench

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// CellRawAgentDiffPath is the raw attempt dir's own stored whole-tree diff
// — the SAME captureAgentDiff format (main.go: `git add -N .` then `git
// diff LocalBase`, filtered of instrument-owned files/regions) a driver
// that captures one places alongside its session-<n>-*.stream.jsonl files.
// A missing file means "nothing was captured" — RematerializeCellWorktree
// is what decides what that means for a cell that needs one; this function
// only names where to look.
func CellRawAgentDiffPath(attemptDir string) string {
	return filepath.Join(attemptDir, "agent.diff")
}

// CellRawUntrackedSnapshotDir is the raw attempt dir's own optional
// byte-identical snapshot of files a stored agent.diff cannot losslessly
// carry (a binary file: captureAgentDiff never passes `--binary`, so a
// plain `git diff` records only "Binary files ... differ" for one, never
// its bytes) — repo-relative paths under this directory, applied
// file-for-file ON TOP of the diff by ApplyStoredAgentDiff. Absent entirely
// when a driver never captured one (every raw dir wave N20 examined,
// guards-pilot-7 included).
func CellRawUntrackedSnapshotDir(attemptDir string) string {
	return filepath.Join(attemptDir, "untracked-snapshot")
}

// binaryDiffFileRE matches a plain `git diff`'s own line for a binary file
// it recorded no patch data for: "Binary files a/<path> and b/<path>
// differ" for a modified binary file, "Binary files /dev/null and
// b/<path> differ" for one the agent CREATED, or "Binary files a/<path>
// and /dev/null differ" for one the agent DELETED (group 2, the b-side,
// empty — a deletion needs no snapshot to reproduce forward, the diff
// already says to remove it).
var binaryDiffFileRE = regexp.MustCompile(`(?m)^Binary files (?:a/\S+|/dev/null) and (?:b/(\S+)|/dev/null) differ$`)

// AgentDiffBinaryFilesMissingSnapshot returns, sorted and de-duplicated,
// the b-side repo-relative paths diffBytes names as a binary file the agent
// CREATED or MODIFIED (never a pure deletion — see binaryDiffFileRE) with
// no corresponding byte-identical file under snapshotDir
// (CellRawUntrackedSnapshotDir). Empty means the diff is fully
// self-contained: either it names no such binary change at all, or every
// one it does name has a snapshot file to fall back on.
func AgentDiffBinaryFilesMissingSnapshot(diffBytes []byte, snapshotDir string) []string {
	seen := map[string]bool{}
	var missing []string
	for _, m := range binaryDiffFileRE.FindAllSubmatch(diffBytes, -1) {
		bSide := string(m[1])
		if bSide == "" || seen[bSide] {
			continue
		}
		seen[bSide] = true
		if _, err := os.Stat(filepath.Join(snapshotDir, bSide)); err != nil {
			missing = append(missing, bSide)
		}
	}
	sort.Strings(missing)
	return missing
}

// ApplyStoredAgentDiff applies attemptDir's own CellRawAgentDiffPath onto
// wt.Path (a freshly PrepareSubject-ed worktree, at base_commit exactly as
// PrepareSubject left it) via `git apply`, then overlays
// CellRawUntrackedSnapshotDir's own files on top, file-for-file — the only
// bytes a diff-alone reconstruction can never carry.
//
// Before applying, the stored diff is re-filtered through
// rematerializeInstrumentOwnedPaths(wt) — the SAME whole-file exclusion
// FilterInstrumentOwnedDiff applies to a live cell's own captureAgentDiff,
// widened to also cover wt.Arm's own rendered paths (SettingsPath,
// MCPConfigPath): a stored diff a driver captured against a DIFFERENT live
// worktree may still carry a "new file" hunk for one of these (an unwired
// arm's own RenderArm write is never ledger-tracked, so an unfiltered
// captureAgentDiff would include it) — but THIS worktree's own PrepareSubject
// call already wrote it fresh, so applying that hunk unfiltered would fail
// outright ("already exists in working directory") rather than merely
// mis-attribute a byte count. Re-filtering here is idempotent when the
// stored diff was already filtered (the common case for a driver that
// mirrors captureAgentDiff exactly), so this is always safe to do.
//
// The caller (RematerializeCellWorktree) has already refused a cell whose
// diff names a binary file with no snapshot entry
// (AgentDiffBinaryFilesMissingSnapshot); this function trusts that check
// was made. A missing, or empty (0-byte — captureAgentDiff writes one even
// when the agent made no changes at all) after filtering, diff is a no-op,
// never an error: an agent that touched nothing (or touched only
// instrument-owned files) is a real, scorable outcome.
func ApplyStoredAgentDiff(ctx context.Context, wt Worktree, attemptDir string) error {
	diffPath := CellRawAgentDiffPath(attemptDir)
	if diffBytes, err := os.ReadFile(diffPath); err == nil {
		owned, err := rematerializeInstrumentOwnedPaths(wt)
		if err != nil {
			return fmt.Errorf("finding instrument-owned paths to exclude: %w", err)
		}
		filtered, _ := FilterInstrumentOwnedDiff(diffBytes, owned, nil)
		if len(strings.TrimSpace(string(filtered))) > 0 {
			tmp, err := os.CreateTemp("", "fathom-scenariobench-rematerialize-diff-")
			if err != nil {
				return fmt.Errorf("writing filtered diff to a temp file: %w", err)
			}
			tmpPath := tmp.Name()
			defer os.Remove(tmpPath)
			if _, err := tmp.Write(filtered); err != nil {
				tmp.Close()
				return fmt.Errorf("writing filtered diff to a temp file: %w", err)
			}
			if err := tmp.Close(); err != nil {
				return fmt.Errorf("writing filtered diff to a temp file: %w", err)
			}
			out, err := runIn(ctx, wt.Path, "git", "apply", tmpPath)
			if err != nil {
				return fmt.Errorf("applying stored agent.diff %s: %w (%s)", diffPath, err, out)
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("statting stored agent.diff %s: %w", diffPath, err)
	}

	// The snapshot overlay runs regardless of whether a diff was applied
	// above — a cell whose ONLY change was an untracked binary file (no
	// text change at all) has a snapshot with nothing for the diff to
	// carry, and still needs its bytes to land.
	snapshotDir := CellRawUntrackedSnapshotDir(attemptDir)
	if st, err := os.Stat(snapshotDir); err == nil && st.IsDir() {
		if err := overlaySnapshotFiles(snapshotDir, wt.Path); err != nil {
			return fmt.Errorf("applying stored untracked-file snapshot under %s: %w", snapshotDir, err)
		}
	}
	return nil
}

// rematerializeInstrumentOwnedPaths is InstrumentOwnedPaths(wt) widened to
// also name wt.Arm's own rendered files (SettingsPath, MCPConfigPath) —
// RenderArm writes both for EVERY arm, wired or not, but the wiring ledger
// InstrumentOwnedPaths reads only exists after a wired arm's `fathom init`
// (wireScratchDaemon); an unwired arm's own rendered files would otherwise
// be un-owned as far as diff filtering is concerned, even though this
// worktree's own PrepareSubject call just wrote them fresh.
func rematerializeInstrumentOwnedPaths(wt Worktree) (map[string]bool, error) {
	owned, err := InstrumentOwnedPaths(wt)
	if err != nil {
		return nil, err
	}
	for _, abs := range []string{wt.Arm.SettingsPath, wt.Arm.MCPConfigPath} {
		if abs == "" {
			continue
		}
		if rel, err := filepath.Rel(wt.Path, abs); err == nil {
			owned[filepath.ToSlash(rel)] = true
		}
	}
	return owned, nil
}

// overlaySnapshotFiles copies every regular file under snapshotDir to the
// same repo-relative path under wtPath, creating parent directories as
// needed and overwriting whatever ApplyStoredAgentDiff's own `git apply`
// left there.
func overlaySnapshotFiles(snapshotDir, wtPath string) error {
	return filepath.WalkDir(snapshotDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(snapshotDir, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(wtPath, rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0o644)
	})
}

// RematerializeCellWorktree rebuilds one cell's live worktree
// deterministically from raw alone — the same PrepareSubject(ctx, task,
// arm, root) a live cell used (same sparse exclusions, same instrument
// writes, same LocalBase semantics) — with attemptDir's own stored
// agent.diff (and untracked-snapshot, if any) applied on top, reproducing
// exactly the tree captureAgentDiff walked at live scoring time.
//
// ok is false, with reason naming why, whenever reproduction cannot be
// attempted or trusted: no agent.diff was ever captured under attemptDir,
// the diff names a binary file with no byte-identical snapshot entry to
// fall back on, PrepareSubject itself failed, or the stored diff failed to
// apply cleanly — never a guess either way (in particular: this never falls
// back to replaying the raw session stream's own Edit/Write tool_use
// payloads — a Bash call in the same session may have mutated a tracked
// file too, which no structured tool_use event would show, so that
// reconstruction can never be proven byte-identical to what the driver
// actually scored).
//
// The returned cleanup ALWAYS stops any wired scratch daemon
// (StopWorktree) and removes the worktree directory from disk — call it
// unconditionally (defer it immediately after the call, exactly like a live
// cell's own `defer StopWorktree(&wt)`) whether ok is true or false, since
// PrepareSubject may have partially succeeded before a later step failed.
// Removing the directory rather than merely resetting it is deliberate and
// safe: checkoutCommitIntoWorktree makes wtPath its OWN freestanding git
// repository (never a `git worktree add` linked worktree of the shared
// clone), and disk here is small — "one re-materialised worktree at a
// time" is this wave's own operating constraint, and the next cell's
// PrepareSubject call for the same (base_commit, arm) checks out fresh
// exactly as if this one had never run.
func RematerializeCellWorktree(ctx context.Context, root string, task Task, arm Arm, attemptDir string) (wt Worktree, cleanup func(), ok bool, reason string) {
	noop := func() {}

	diffPath := CellRawAgentDiffPath(attemptDir)
	diffBytes, err := os.ReadFile(diffPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Worktree{}, noop, false, fmt.Sprintf("oracle needs a live worktree and no agent.diff was captured under %s", attemptDir)
		}
		return Worktree{}, noop, false, fmt.Sprintf("reading stored agent.diff %s: %v", diffPath, err)
	}
	if missing := AgentDiffBinaryFilesMissingSnapshot(diffBytes, CellRawUntrackedSnapshotDir(attemptDir)); len(missing) > 0 {
		return Worktree{}, noop, false, fmt.Sprintf("stored agent.diff names binary file(s) %v with no untracked-snapshot fallback under %s — the diff alone cannot reproduce them", missing, attemptDir)
	}

	prepared, err := PrepareSubject(ctx, task, arm, root)
	cleanup = func() {
		_ = StopWorktree(&prepared)
		if prepared.Path != "" {
			_ = os.RemoveAll(prepared.Path)
		}
	}
	if err != nil {
		return Worktree{}, cleanup, false, fmt.Sprintf("re-materializing subject: %v", err)
	}

	if err := ApplyStoredAgentDiff(ctx, prepared, attemptDir); err != nil {
		return prepared, cleanup, false, fmt.Sprintf("applying stored agent.diff: %v", err)
	}

	return prepared, cleanup, true, ""
}
