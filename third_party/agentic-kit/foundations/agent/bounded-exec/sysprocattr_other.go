//go:build !unix

package boundedexec

import (
	"os/exec"
	"time"
)

// setProcGroup is a no-op on non-unix platforms: the stdlib offers no
// portable process-group primitive here, so only the direct child can be
// targeted. Grandchildren may survive a timeout — grandchild cleanup is
// best-effort and unix-only (see the package documentation).
func setProcGroup(cmd *exec.Cmd) {}

// killTree kills only the direct child on non-unix platforms. The grace
// and exited parameters are unused: there is no graceful signal to wait
// out, and no group to sweep afterwards.
func killTree(cmd *exec.Cmd, grace time.Duration, exited <-chan struct{}) {
	if p := cmd.Process; p != nil {
		_ = p.Kill()
	}
}
