package scenariobench

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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

// gitEnvFor is what subject.go's runInEnv actually applies to every git
// subprocess (keyed on name=="git") — gitConfigIsolation's own
// GIT_CONFIG_GLOBAL/GIT_CONFIG_NOSYSTEM isolation, plus
// GIT_TERMINAL_PROMPT=0 and GIT_ASKPASS="" (unset) unconditionally. A
// caller-supplied key in extra still wins on collision (gitConfigIsolation's
// own convention, extended here the same way): ensureClone/ensureCommit
// pass a GIT_CONFIG_GLOBAL pointing at operatorGitConfigFile's derived file
// instead of accepting gitConfigIsolation's own /dev/null default.
//
// GIT_TERMINAL_PROMPT=0 exists so a git operation with no reachable
// credential fails FAST with exit 128 instead of hanging the whole run
// waiting at a terminal prompt nothing will ever answer — guards-pilot-7
// appeared to simply hang, 2026-09-17, rather than fail the cell LOST with
// a reason. GIT_ASKPASS="" closes the same hole for a GUI-style askpass
// helper the operator's environment might otherwise supply.
func gitEnvFor(extra map[string]string) map[string]string {
	base := map[string]string{
		"GIT_TERMINAL_PROMPT": "0",
		"GIT_ASKPASS":         "",
	}
	for k, v := range extra {
		base[k] = v
	}
	return gitConfigIsolation(base)
}

// allowedOperatorConfigKey reports whether a key from the operator's real
// global git config is needed to REACH a remote — a credential helper, a
// url.*.insteadOf/pushInsteadOf rewrite, an http proxy or CA bundle, the
// ssh command — and so must survive into the driver-owned file
// operatorGitConfigFile points GIT_CONFIG_GLOBAL at. Everything else
// (autocrlf, safecrlf, fsck*, hooks path, aliases, commit identity,
// signing, gpg.* generally) stays isolated behind gitConfigIsolation's
// /dev/null default: VB-27's whole point was keeping the OPERATOR's policy
// choices away from subject preparation, and this widens that hole by
// exactly the keys a clone or fetch of a PRIVATE remote cannot work
// without — dropping credential.https://github.com.helper (the gh
// credential helper) cost vs-pilot-6 all 140 cells at subject preparation,
// 2026-09-17.
func allowedOperatorConfigKey(key string) bool {
	lower := strings.ToLower(key)
	switch {
	case strings.HasPrefix(lower, "credential."):
		return true
	case strings.HasPrefix(lower, "url.") &&
		(strings.HasSuffix(lower, ".insteadof") || strings.HasSuffix(lower, ".pushinsteadof")):
		return true
	case lower == "http.proxy", lower == "http.sslcainfo":
		return true
	case strings.HasPrefix(lower, "http.") && strings.HasSuffix(lower, ".sslcainfo"):
		return true
	case lower == "core.sshcommand":
		return true
	}
	return false
}

// readOperatorGlobalConfig runs `git config --global --list -z` in the
// UNISOLATED operator environment (gitEnv() — never gitEnvFor's own
// GIT_CONFIG_GLOBAL override, which this is what feeds) and returns every
// key/value pair it defines, in the order git reports them. Best-effort:
// an operator with no reachable global config at all (a hermetic HOME, or
// GIT_CONFIG_GLOBAL already pointing nowhere) returns no pairs rather than
// an error — a config file this driver-owned mechanism cannot read is
// exactly the case gitConfigIsolation's own /dev/null default already
// covered, never a new failure mode.
func readOperatorGlobalConfig(ctx context.Context) [][2]string {
	cmd := exec.CommandContext(ctx, "git", "config", "--global", "--list", "-z")
	cmd.Env = gitEnv()
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var pairs [][2]string
	for _, entry := range strings.Split(string(out), "\x00") {
		if entry == "" {
			continue
		}
		name, value, _ := strings.Cut(entry, "\n")
		pairs = append(pairs, [2]string{name, value})
	}
	return pairs
}

// deriveOperatorGitConfig writes the operator's remote-reaching global git
// config keys (allowedOperatorConfigKey) into a fresh file at path — via
// `git config --file --add`, so subsection/key parsing exactly matches how
// git itself will later read it back, and a key the operator legitimately
// repeats (gh's own credential.helper=<empty> reset followed by its real
// helper) is carried over in full rather than erroring on "cannot
// overwrite multiple values with a single value" — and returns path.
func deriveOperatorGitConfig(ctx context.Context, path string) (string, error) {
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		return "", fmt.Errorf("deriving the operator git config: %w", err)
	}
	for _, kv := range readOperatorGlobalConfig(ctx) {
		if !allowedOperatorConfigKey(kv[0]) {
			continue
		}
		cmd := exec.CommandContext(ctx, "git", "config", "--file", path, "--add", kv[0], kv[1])
		cmd.Env = gitEnv()
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("deriving the operator git config: writing %s: %w (%s)", kv[0], err, out)
		}
	}
	return path, nil
}

// operatorGitConfigCache and operatorGitConfigMu back operatorGitConfigFile
// below: derived once per distinct root and cached, so two ensureClone (or
// ensureClone/ensureCommit) calls sharing a root never race writing the
// same file (lockClone's own lock is per-cloneDir, never global), and a
// test suite juggling many distinct t.TempDir() roots in one process gets
// a fresh derivation — reflecting THAT test's own env — per root, never a
// stale one left over from an earlier test.
var (
	operatorGitConfigCache sync.Map // map[string]operatorGitConfigEntry
	operatorGitConfigMu    sync.Mutex
)

type operatorGitConfigEntry struct {
	path string
	err  error
}

// operatorGitConfigFile returns the path to a driver-owned git config file
// under root's workspace/tmp carrying over only the operator's
// remote-reaching global config keys (allowedOperatorConfigKey) — built
// once per run (per distinct root) so ensureClone/ensureCommit can point
// GIT_CONFIG_GLOBAL at it instead of gitConfigIsolation's own /dev/null,
// without a dropped credential helper locking a private subject's clone
// out entirely (vs-pilot-6, 2026-09-17).
func operatorGitConfigFile(ctx context.Context, root string) (string, error) {
	if v, ok := operatorGitConfigCache.Load(root); ok {
		e := v.(operatorGitConfigEntry)
		return e.path, e.err
	}
	operatorGitConfigMu.Lock()
	defer operatorGitConfigMu.Unlock()
	if v, ok := operatorGitConfigCache.Load(root); ok {
		e := v.(operatorGitConfigEntry)
		return e.path, e.err
	}
	dir := filepath.Join(root, "workspace", "tmp")
	var entry operatorGitConfigEntry
	if err := os.MkdirAll(dir, 0o755); err != nil {
		entry.err = fmt.Errorf("deriving the operator git config: %w", err)
	} else {
		entry.path, entry.err = deriveOperatorGitConfig(ctx, filepath.Join(dir, "operator-gitconfig"))
	}
	operatorGitConfigCache.Store(root, entry)
	return entry.path, entry.err
}
