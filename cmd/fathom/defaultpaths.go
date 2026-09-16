package main

// defaultpaths.go — PC-15 / invariant I31 (docs/plans/proof-credibility.md,
// docs/intent/invariants.md): the `-store` default resolves under the
// enclosing repository's `workspace/` — the gitignored home every fathom
// artifact already has — or under `~/.fathom/` when the cwd is not inside
// a git repository. Never the bare cwd: a relative
// "workspace/db/fathom.sqlite" from `<repo>/deploy` created a second store
// (O11) and a call log nobody read (O12).
//
// The `-call-log` default resolves the SAME way but under the repo's
// state directory (internal/statedir.Resolve) rather than workspace/
// itself (O63 item 5): the call log is fathom's own runtime state, not
// the code index — unlike the store, it has no reason to live inside the
// subject repository's tree at all, and by default now does not.
//
// Explicit flags, `[daemon].store` / `[daemon].call_log` in config.toml,
// and FATHOM_CALL_LOG still win — these functions only replace the
// literal fallback.

import (
	"os"
	"path/filepath"

	"github.com/geoffmilleraz/fathom/internal/statedir"
)

// workspacePath resolves rel (slash-separated, relative) under the repo's
// workspace/ or, outside any repo, under ~/.fathom/. With no resolvable
// home either, the relative path is returned as the last honest fallback.
func workspacePath(rel string) string {
	rel = filepath.FromSlash(rel)
	if cwd, err := os.Getwd(); err == nil {
		if top := gitToplevel(cwd); top != "" {
			return filepath.Join(top, "workspace", rel)
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".fathom", rel)
	}
	return rel
}

// stateLogPath resolves rel (slash-separated, relative) under the repo's
// STATE directory (statedir.Resolve — outside the repo tree by default,
// root/workspace/... when the repo's own config opts in) or, outside any
// repo, under ~/.fathom/. Mirrors workspacePath exactly for the artifacts
// that are fathom's own runtime state rather than the code index.
func stateLogPath(rel string) string {
	rel = filepath.FromSlash(rel)
	if cwd, err := os.Getwd(); err == nil {
		if top := gitToplevel(cwd); top != "" {
			return filepath.Join(statedir.Resolve(top), rel)
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".fathom", rel)
	}
	return rel
}

// defaultStorePath is the store every store-reading command falls back to
// when neither --store nor [daemon].store names one.
func defaultStorePath() string { return workspacePath("db/fathom.sqlite") }

// defaultCallLogPath is the MCP call log the daemon writes and the metrics
// verbs read when neither --call-log, FATHOM_CALL_LOG nor
// [daemon].call_log names one.
func defaultCallLogPath() string { return stateLogPath("logs/mcp-calls.jsonl") }
