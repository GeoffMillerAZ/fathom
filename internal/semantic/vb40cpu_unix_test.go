//go:build unix

package semantic

import (
	"syscall"
	"time"
)

// processCPU is this process's user+system CPU time so far (RS-M2's latency
// reading measures CPU, not wall clock).
func processCPU() time.Duration {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}
