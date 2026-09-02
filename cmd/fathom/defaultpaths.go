package main

// defaultpaths.go — PC-15 / invariant I31 (docs/plans/proof-credibility.md,
// docs/intent/invariants.md): the `-store` and `-call-log` defaults resolve
// under the enclosing repository's `workspace/` — the gitignored home every
// fathom artifact already has — or under `~/.fathom/` when the cwd is not
// inside a git repository. Never the bare cwd: a relative
// "workspace/db/fathom.sqlite" from `<repo>/deploy` created a second store
// (O11) and a call log nobody read (O12). Explicit flags, `[daemon].store`
// / `[daemon].call_log` in config.toml, and FATHOM_CALL_LOG still win —
// these functions only replace the literal fallback.

import (
	"os"
	"path/filepath"
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

// defaultStorePath is the store every store-reading command falls back to
// when neither --store nor [daemon].store names one.
func defaultStorePath() string { return workspacePath("db/fathom.sqlite") }

// defaultCallLogPath is the MCP call log the daemon writes and the metrics
// verbs read when neither --call-log, FATHOM_CALL_LOG nor
// [daemon].call_log names one.
func defaultCallLogPath() string { return workspacePath("logs/mcp-calls.jsonl") }

// workspaceCallLogRel is the same default as a path RELATIVE to a repo root
// — for readers that must join it against a root they know to be the
// daemon's working directory rather than their own cwd (doctor, O23).
const workspaceCallLogRel = "workspace/logs/mcp-calls.jsonl"
