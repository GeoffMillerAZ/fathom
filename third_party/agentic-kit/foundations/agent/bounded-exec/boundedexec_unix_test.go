//go:build unix

package boundedexec

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestProcessGroupKillReapsGrandchild verifies the best-effort,
// unix-only group kill: the child shell backgrounds a long sleep
// (a grandchild in the same process group), the run times out, and the
// grandchild must die with the group. The shell prints the grandchild's
// pid before blocking so the test can probe it with kill(pid, 0).
//
// The assertion is deliberately robust: the grandchild is polled with a
// generous deadline, since reaping by init/launchd after the group kill
// is asynchronous.
func TestProcessGroupKillReapsGrandchild(t *testing.T) {
	t.Parallel()
	requireTool(t, shPath)

	r := mustRunner(t, Config{
		Timeout:        200 * time.Millisecond,
		MaxStdoutBytes: 1 << 10,
		MaxStderrBytes: 1 << 10,
	})

	res, err := r.Run(context.Background(), Spec{
		Path: shPath,
		Args: []string{"-c", "sleep 30 & echo $!; sleep 30"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.TimedOut {
		t.Fatal("TimedOut = false, want true")
	}

	pidStr := strings.TrimSpace(string(res.Stdout))
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 1 {
		t.Fatalf("could not parse grandchild pid from stdout %q: %v", pidStr, err)
	}

	// Poll until the grandchild is gone. kill(pid, 0) returns ESRCH once
	// the process no longer exists (zombies count as existing until
	// reaped by init/launchd, hence the polling).
	deadline := time.Now().Add(5 * time.Second)
	for {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return // group kill swept the grandchild
		}
		if time.Now().After(deadline) {
			t.Fatalf("grandchild pid %d still alive %s after group kill (kill(pid,0) err=%v)",
				pid, 5*time.Second, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
