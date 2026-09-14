// artifactdirs.go — O57 finding 6: "per-arm artifacts are shared across
// trials." PrepareSubject's own wt.CellDir is keyed on (task.ID, the
// shared worktree's own basename) — task and arm, but never TRIAL — so
// every trial of one (task, arm) used to write agent.diff,
// oracle-test-output-*.txt and detector-*.log to the exact same path,
// each trial silently overwriting the last. PerTrialCellDir gives each
// trial its own subdirectory of that same cell dir.
package scenariobench

import (
	"fmt"
	"path/filepath"
)

// PerTrialCellDir returns the per-TRIAL artifact directory a cell's
// caller should use for wt.CellDir from this point on (agent.diff,
// oracle-test-output, detector logs, plan.md — everything captured AFTER
// PrepareSubject returns): attemptDir verbatim when non-empty, else a
// `trial-<n>` subdirectory of cellDir.
//
// attemptDir is NOT wave-n1's allocateRawAttemptDir (sessions.go),
// checked post-rebase (wave-n6/wave-n4, 2026-09-14): that attempt-<k>
// directory lives under a different root (-raw-dir, raw session
// transcripts) than wt.CellDir (workspace/artifacts/.../worktrees, oracle
// and diff/plan capture), is allocated INSIDE RunSessions and never
// returned to its caller, and — the hard blocker — cannot be known before
// RunSessions runs, while wt.CellDir must already be final before then:
// sessionEnv reads wt.CellDir to build the clean-room env (FATHOM_THREADS,
// the baseline-isolation shim) for the very sessions that dir would
// belong to. Unifying the two would mean threading the allocated attempt
// dir back out of RunSessions and moving env construction off wt.CellDir
// — a real API change, not a wiring fix; left as a caller-supplied
// parameter for whichever future layout can satisfy that ordering.
func PerTrialCellDir(cellDir, attemptDir string, trial int) string {
	if attemptDir != "" {
		return attemptDir
	}
	return filepath.Join(cellDir, fmt.Sprintf("trial-%d", orOne(trial)))
}
