package scenariobench

import (
	"os"
	"testing"
)

// TestMain scrubs the variables git exports to hook processes before any
// fixture spawns git: the pre-push gate runs this suite inside a hook, and
// an inherited GIT_DIR made `git init` in a fixture directory re-initialise
// the checkout as a bare repository (2026-09-14).
func TestMain(m *testing.M) {
	for _, k := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_PREFIX", "GIT_COMMON_DIR"} {
		os.Unsetenv(k)
	}
	os.Setenv("LEFTHOOK", "0")
	os.Exit(m.Run())
}
