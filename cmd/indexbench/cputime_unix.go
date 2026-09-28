//go:build unix

package main

import (
	"syscall"
	"time"
)

// processCPU is the process's CPU time so far (user + system), the basis
// of the throughput floor (owner decision D1): bytes per CPU-second of the
// extraction process, so a reading taken while other workers contend for
// the machine is not failed by the contention.
func processCPU() time.Duration {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}
