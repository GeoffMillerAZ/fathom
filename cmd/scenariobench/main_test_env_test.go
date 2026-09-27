package main

import (
	"os"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/testisolate"
)

// TestMain scrubs the variables git exports to hook processes before any
// test spawns the driver's git commands (see internal/scenariobench/gitenv.go).
func TestMain(m *testing.M) {
	for _, k := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_PREFIX", "GIT_COMMON_DIR"} {
		os.Unsetenv(k)
	}
	os.Setenv("LEFTHOOK", "0")
	testisolate.Main(m) // RT-19 (§10.4): private HOME/state/call log, unroutable daemon URL
}
