//go:build !unix

package fusion

import (
	"context"
	"os/exec"
	"time"
)

// commandTree is exec.CommandContext where process groups are not
// available: cancelling ctx kills the direct child.
func commandTree(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = 2 * time.Second
	return cmd
}
