package testisolate

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// GoCacheVars are the Go toolchain's shared caches. Both default under HOME
// (GOCACHE in the user cache directory, GOMODCACHE in $HOME/go), so an
// isolated HOME handed every go command a test starts, such as the typed
// pass's go/packages load, an empty build cache and an empty module cache:
// it recompiled every dependency and downloaded every module, the CT-5
// performance gate measured that instead of the pass, and the read-only
// module cache outlived the root's removal. They are content-addressed
// toolchain caches, not fathom state, so IsolateAndRun pins each one the
// environment leaves unset to what the go command resolves while HOME is
// still the operator's. An inherited value is kept as it was.
var GoCacheVars = []string{"GOCACHE", "GOMODCACHE"}

// resolveGoCaches returns the GoCacheVars the environment leaves unset,
// resolved by goEnv under the current (not yet isolated) environment, with
// only absolute answers kept; nil when none is unset or goEnv fails.
func resolveGoCaches(goEnv func(keys []string) (map[string]string, error)) map[string]string {
	var unset []string
	for _, k := range GoCacheVars {
		if os.Getenv(k) == "" {
			unset = append(unset, k)
		}
	}
	if len(unset) == 0 {
		return nil
	}
	got, err := goEnv(unset)
	if err != nil {
		return nil
	}
	pinned := map[string]string{}
	for _, k := range unset {
		if v := got[k]; filepath.IsAbs(v) {
			pinned[k] = v
		}
	}
	return pinned
}

// goEnv runs `go env -json keys...` from PATH under this process's
// environment with GOTOOLCHAIN=local (no TestMain fetches a toolchain a
// go.mod names), bounded at 30 s.
func goEnv(keys []string) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", append([]string{"env", "-json"}, keys...)...)
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	got := map[string]string{}
	if err := json.Unmarshal(out, &got); err != nil {
		return nil, err
	}
	return got, nil
}
