package scenariobench

import (
	"context"
	"fmt"
	"strings"
)

// absorbInstrumentFiles (O105) amends the worktree's root commit with every
// file the instrument wrote after it — the rendered arm (CLAUDE.md stanza,
// .claude/settings.json, .mcp.json), fathom init's wiring, an arm's own
// setup commands — and re-reads Worktree.LocalBase, so `git diff
// $SCENARIO_BASE_COMMIT` (what a corpus detector runs) names only what the
// SESSION changed. findings-loop-r2 read all 30 cells as violations because
// its no-unrelated-rewrite detector already saw the instrument's own
// settings and stanza edits at baseline; the recorded agent.diff had
// excluded those paths (InstrumentFilesExcluded) — the detectors had not.
// A worktree with nothing to absorb is amended empty and left identical.
func absorbInstrumentFiles(ctx context.Context, wt *Worktree) error {
	if out, err := runIn(ctx, wt.Path, "git", "add", "-A"); err != nil {
		return fmt.Errorf("absorbing instrument files: git add: %w (%s)", err, out)
	}
	commitEnv := map[string]string{
		"GIT_AUTHOR_NAME": "bench", "GIT_AUTHOR_EMAIL": "bench@example.invalid",
		"GIT_COMMITTER_NAME": "bench", "GIT_COMMITTER_EMAIL": "bench@example.invalid",
	}
	if out, err := runInEnv(ctx, wt.Path, commitEnv, "git", "-c", "commit.gpgsign=false", "commit", "-q", "--amend", "--no-edit", "--allow-empty"); err != nil {
		return fmt.Errorf("absorbing instrument files: git commit --amend: %w (%s)", err, out)
	}
	localBase, err := runIn(ctx, wt.Path, "git", "rev-list", "--max-parents=0", "HEAD")
	if err != nil {
		return fmt.Errorf("absorbing instrument files: re-reading the root commit: %w (%s)", err, localBase)
	}
	wt.LocalBase = strings.TrimSpace(localBase)
	return nil
}
