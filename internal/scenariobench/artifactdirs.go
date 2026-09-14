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
// PrepareSubject returns): attemptDir verbatim when non-empty (wave-n1's
// own per-session attempt-layout directory for this trial, when the
// caller has one), else a `trial-<n>` subdirectory of cellDir.
func PerTrialCellDir(cellDir, attemptDir string, trial int) string {
	if attemptDir != "" {
		return attemptDir
	}
	return filepath.Join(cellDir, fmt.Sprintf("trial-%d", orOne(trial)))
}
