//go:build unix

package boundedexec

import (
	"os/exec"
	"syscall"
	"time"
)

// setProcGroup places the child in its own process group (Setpgid) so the
// entire group — the child plus any grandchildren that do not move to a
// group of their own — can be signaled together on timeout or
// cancellation. The child becomes the group leader, so pgid == child pid.
func setProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killTree terminates the child's whole process group: SIGTERM to the
// negative pgid first, then SIGKILL once the child has been reaped or the
// grace period elapses, whichever comes first. The final SIGKILL is always
// sent so grandchildren that survived the leader are swept too.
//
// Cleanup is BEST-EFFORT: a descendant that called setpgid/setsid escapes
// the group, and signal delivery errors (e.g. the group already gone) are
// deliberately ignored.
func killTree(cmd *exec.Cmd, grace time.Duration, exited <-chan struct{}) {
	p := cmd.Process
	if p == nil {
		return
	}
	pgid := p.Pid // Setpgid made the child the group leader.
	_ = syscall.Kill(-pgid, syscall.SIGTERM)
	t := time.NewTimer(grace)
	defer t.Stop()
	select {
	case <-exited:
	case <-t.C:
	}
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
}
