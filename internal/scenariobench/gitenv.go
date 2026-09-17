package scenariobench

import (
	"os"
	"strings"
)

// scrubGitEnv returns env without the variables git exports to hook
// processes (GIT_DIR, GIT_WORK_TREE, GIT_INDEX_FILE, GIT_PREFIX,
// GIT_COMMON_DIR). A driver or a test fixture that spawns git while running
// inside a hook (the pre-push gate runs the whole suite) would otherwise
// operate on the HOOK's repository: `git init` in a fixture directory
// re-initialised the checkout as a bare repository and `git commit` ran the
// checkout's lefthook stack, which failed for want of a config file
// (2026-09-14). LEFTHOOK=0 keeps any inherited hook runner quiet as well.
func scrubGitEnv(env []string) []string {
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		switch {
		case strings.HasPrefix(kv, "GIT_DIR="),
			strings.HasPrefix(kv, "GIT_WORK_TREE="),
			strings.HasPrefix(kv, "GIT_INDEX_FILE="),
			strings.HasPrefix(kv, "GIT_PREFIX="),
			strings.HasPrefix(kv, "GIT_COMMON_DIR="),
			strings.HasPrefix(kv, "LEFTHOOK="):
			continue
		}
		out = append(out, kv)
	}
	return append(out, "LEFTHOOK=0")
}

// gitEnv is scrubGitEnv over the process environment.
func gitEnv() []string { return scrubGitEnv(os.Environ()) }

// gitConfigIsolation returns extra plus GIT_CONFIG_GLOBAL=/dev/null and
// GIT_CONFIG_NOSYSTEM=1 (a caller-supplied key in extra always wins on
// collision, matching runInEnv's own documented merge convention) — applied
// to every git invocation subject.go's runInEnv makes (keyed on
// name=="git"), so the OPERATOR's own ~/.gitconfig and /etc/gitconfig never
// govern subject preparation or worktree checkout. gitEnv above already
// scrubs the GIT_* variables git itself exports to hook processes; this is
// the same isolation instinct aimed at the operator's own settings instead
// — a fetch.fsckObjects/transfer.fsckObjects the operator's machine turned
// on, or a core.autocrlf/core.safecrlf policy (already covered per-key by
// checkoutCommitIntoWorktree's own core.autocrlf=false/safecrlf=false, the
// CRLF fix in 2fdaaf0 — now doubly covered, redundant but harmless) have
// nothing to do with what a bench subject's clone or checkout should
// accept (VB-27, tootallnate/java-websocket: `fatal: fsck error in packed
// object`, lca-run-1 2026-09-17). Local (repo) config is untouched by
// either variable, so checkoutCommitIntoWorktree's own `git config
// core.autocrlf false` still lands and is still read back.
func gitConfigIsolation(extra map[string]string) map[string]string {
	merged := map[string]string{
		"GIT_CONFIG_GLOBAL":   "/dev/null",
		"GIT_CONFIG_NOSYSTEM": "1",
	}
	for k, v := range extra {
		merged[k] = v
	}
	return merged
}
