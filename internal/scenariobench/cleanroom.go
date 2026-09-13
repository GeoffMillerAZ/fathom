// The clean room (RC-6, vision 28 §2 F4) is Claude Code's own `--restricted`
// mode (buildClaudeArgs, harness_claude.go): it ignores user, project and
// local settings files — so the operator's global hooks, plugins and slash
// commands never reach a cell — while keeping auth, still honoring
// `--settings <file>` (the arm's own rendered `.claude/settings.json`,
// RenderArm) and `--mcp-config`/`--strict-mcp-config`, and confining the
// file tools to the working directory. `--tools` names exactly the arm's
// own allowed tools (buildClaudeArgs' restrictedToolsList), and
// `--disable-slash-commands` removes skills on top.
//
// An earlier design here built a scratch CLAUDE_CONFIG_DIR per run and
// copied a stripped ~/.claude.json/~/.claude/settings.json into it — RC-6's
// own smoke test found that broke authentication ("Not logged in · Please
// run /login") under every variant tried, because whatever backs this
// CLI's macOS-keychain login lookup did not resolve under a scratch
// CLAUDE_CONFIG_DIR in this environment. `--restricted` needs no config-dir
// copying at all: its own smoke test (docs/bench/scenarios.md "The clean
// room") proved auth, a per-arm `--settings` hook firing, and the absence
// of the operator's stanza together in one real (paid) run.
//
// This file's only remaining job is the per-cell FATHOM_THREADS env var,
// so two tasks sharing a worktree basename never share a thread scope
// either (F5).
package scenariobench

import (
	"fmt"
	"os"
	"path/filepath"
)

// cleanRoomEnv returns the environment overrides one session's clean room
// needs: FATHOM_THREADS pointing at a per-cell threads.jsonl under
// cellDir (Worktree.CellDir), which is created if it does not already
// exist.
func cleanRoomEnv(cellDir string) (map[string]string, error) {
	if err := os.MkdirAll(cellDir, 0o755); err != nil {
		return nil, fmt.Errorf("clean room: creating %s: %w", cellDir, err)
	}
	return map[string]string{
		"FATHOM_THREADS": filepath.Join(cellDir, "threads.jsonl"),
	}, nil
}
