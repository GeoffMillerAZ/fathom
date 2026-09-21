package scenariobench

import (
	"context"
	"fmt"
	"strings"
)

// absorbInstrumentFiles (O105) commits every file the instrument wrote
// after the worktree's root commit — the rendered arm (CLAUDE.md stanza,
// .claude/settings.json, .mcp.json), fathom init's wiring, an arm's own
// setup commands — as ONE commit above the root, and points
// Worktree.LocalBase at it, so `git diff $SCENARIO_BASE_COMMIT` (what a
// corpus detector runs) names only what the SESSION changed.
// findings-loop-r2 read all 30 cells as violations because its
// no-unrelated-rewrite detector already saw the instrument's own settings
// and stanza edits at baseline; the recorded agent.diff had excluded those
// paths (InstrumentFilesExcluded) — the detectors had not.
//
// A commit ABOVE the root, never an amend of it: a shared worktree resets
// to its root commit between cells (PrepareSubject's reuse path), so the
// root must stay the pristine export. The first cut amended the root and
// so carried one cell's arm setup into every later cell's baseline — a
// corpus setup that appends a `[findings]` table appended it twice, the
// scratch daemon refused the config, and findings-loop-r3 lost 28 cells.
// A worktree with nothing to absorb gets an empty commit and is otherwise
// left identical.
func absorbInstrumentFiles(ctx context.Context, wt *Worktree) error {
	if out, err := runIn(ctx, wt.Path, "git", "add", "-A"); err != nil {
		return fmt.Errorf("absorbing instrument files: git add: %w (%s)", err, out)
	}
	commitEnv := map[string]string{
		"GIT_AUTHOR_NAME": "bench", "GIT_AUTHOR_EMAIL": "bench@example.invalid",
		"GIT_COMMITTER_NAME": "bench", "GIT_COMMITTER_EMAIL": "bench@example.invalid",
	}
	if out, err := runInEnv(ctx, wt.Path, commitEnv, "git", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "instrument files (arm, wiring, setup)", "--allow-empty"); err != nil {
		return fmt.Errorf("absorbing instrument files: git commit: %w (%s)", err, out)
	}
	localBase, err := runIn(ctx, wt.Path, "git", "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("absorbing instrument files: reading the instrument commit: %w (%s)", err, localBase)
	}
	wt.LocalBase = strings.TrimSpace(localBase)
	return nil
}
