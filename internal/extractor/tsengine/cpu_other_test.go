//go:build !unix

package tsengine

import (
	"testing"
	"time"
)

// processCPU: no getrusage here, so the CPU-bounded assertions cannot be
// made honestly — skip rather than fall back to wall clock.
func processCPU(t *testing.T) time.Duration {
	t.Helper()
	t.Skip("process CPU accounting (getrusage) unavailable on this platform")
	return 0
}
