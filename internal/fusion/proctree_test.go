//go:build unix

package fusion

// RT-178: a child the typed pass starts runs in its own process group, and
// cancelling the pass's context kills the whole group — the child and
// anything it forked — not only the direct child.

import (
	"bufio"
	"context"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCommandTreeKillsGrandchildOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := commandTree(ctx, "sh", "-c", "sleep 3600 & echo $!; wait")
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatalf("read grandchild pid: %v", err)
	}
	grandchild, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatalf("grandchild pid %q: %v", line, err)
	}
	// The grandchild sleeps for an hour and holds the pipe Wait drains, so a
	// tree that was not killed leaves Wait blocked for good: returning is
	// "the tree was killed promptly". Never leak the sleep, whatever happens.
	t.Cleanup(func() { _ = syscall.Kill(grandchild, syscall.SIGKILL) })
	cancel()
	returnsUnaided(t, "cmd.Wait after cancel with a grandchild holding the pipe", func() { _ = cmd.Wait() })
	deadline := time.Now().Add(waitSafety)
	for syscall.Kill(grandchild, 0) == nil {
		if time.Now().After(deadline) {
			_ = syscall.Kill(grandchild, syscall.SIGKILL)
			t.Fatalf("grandchild %d still alive after its group's context was cancelled", grandchild)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
