//go:build unix

package fusion

import (
	"context"
	"os/exec"
	"syscall"
	"time"
)

// commandTree is exec.CommandContext for a child the typed pass starts
// (RT-178): the child leads its own process group, and cancelling ctx
// SIGKILLs the whole group — the child and whatever it forked — so a halted
// pass never leaves a toolchain subprocess running. WaitDelay bounds Wait
// when a straggler still holds the output pipe.
func commandTree(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 2 * time.Second
	return cmd
}

// processAlive reports whether pid names a running process (EPERM: it
// runs, as another user).
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
