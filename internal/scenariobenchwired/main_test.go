package scenariobenchwired_test

import (
	"os"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/testisolate"
)

// TestMain is internal/scenariobench's own: it scrubs the variables git
// exports to hook processes before any fixture spawns git or go (the
// pre-push gate runs this suite inside a hook, and an inherited GIT_DIR once
// made `git init` in a fixture directory re-initialise the checkout as a
// bare repository, 2026-09-14), and runs the package under testisolate
// (RT-19, §10.4): private HOME/state/call log, unroutable daemon URL.
func TestMain(m *testing.M) {
	for _, k := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_PREFIX", "GIT_COMMON_DIR"} {
		os.Unsetenv(k)
	}
	os.Setenv("LEFTHOOK", "0")
	testisolate.Main(m)
}
