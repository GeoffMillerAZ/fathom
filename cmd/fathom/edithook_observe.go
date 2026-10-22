package main

// edithook_observe.go — observe-only hook paths exit before any git or
// daemon work (docs/design/responsible-tenant.md §11.3, RT-131). The
// PreToolUse Read|Grep|Glob observer records a discovery beat and pointer
// row and nothing else; a PostToolUse Bash call with no write targets has
// nothing to serve. Both fire on a large share of every agent's tool calls,
// so neither may fork git (the observer resolves its repository root by
// stat — the .git walk git itself does — never `git rev-parse`) or reach
// the daemon.

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

// observeOnlyExit handles hook when its call is observe-only and reports
// whether it did (the caller then exits 0 with no output).
func observeOnlyExit(ev harness.Event, hook harness.HookInput, armOff bool) bool {
	switch {
	case ev == harness.EventPreTool && isDiscoveryTool(hook.Tool):
		// No git fork on this path: the observed-discoveries log resolves
		// its repository root by stat (observedDiscoveriesLogPathFor).
		cmdEditHookPreDiscovery(hook, armOff)
		return true
	case ev == harness.EventPostTool && hook.FilePath == "" && hook.Tool == harness.ToolShell:
		if _, writeShaped := bashWriteTargets(hook.Command, hook.Cwd); !writeShaped || strings.TrimSpace(hook.Command) == "" {
			return true // a read-only or unrecognized command: silent, as before
		}
	case ev == harness.EventPostTool && hook.FilePath == "" && hook.Tool != harness.ToolShell:
		return true // neither a file nor a command: nothing to serve
	}
	return false
}

func isDiscoveryTool(t harness.ToolClass) bool {
	return t == harness.ToolDiscoveryRead || t == harness.ToolDiscoveryGrep || t == harness.ToolDiscoveryGlob
}

// gitToplevelByStat finds dir's work-tree root the way git does for an
// ordinary repository or worktree — the nearest ancestor holding a .git
// directory or file — without forking git ("" when there is none).
func gitToplevelByStat(dir string) string {
	d, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	if r, err := filepath.EvalSymlinks(d); err == nil {
		d = r
	}
	for {
		if _, err := os.Lstat(filepath.Join(d, ".git")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return ""
		}
		d = parent
	}
}

// localCandidateRoots is serviceCandidates' roots — the project dir, then
// its work-tree root — with the root found by stat: the observer paths run
// on every discovery call and must not fork git.
func localCandidateRoots(projDir string) []string {
	projDir = strings.TrimRight(projDir, "/")
	roots := []string{projDir}
	if top := gitToplevelByStat(projDir); top != "" && top != projDir {
		roots = append(roots, top)
	}
	return roots
}
