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
