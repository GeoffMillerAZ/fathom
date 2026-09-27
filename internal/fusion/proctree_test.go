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
	cmd := commandTree(ctx, "sh", "-c", "sleep 60 & echo $!; wait")
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
	start := time.Now()
	cancel()
	_ = cmd.Wait()
	if waited := time.Since(start); waited > 5*time.Second {
		t.Fatalf("Wait took %v after cancel, want the tree killed promptly", waited)
	}
	deadline := time.Now().Add(3 * time.Second)
	for syscall.Kill(grandchild, 0) == nil {
		if time.Now().After(deadline) {
			_ = syscall.Kill(grandchild, syscall.SIGKILL)
			t.Fatalf("grandchild %d still alive after its group's context was cancelled", grandchild)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
