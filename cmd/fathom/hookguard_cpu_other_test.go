//go:build !darwin && !linux

package main

import "time"

// processCPUTime has no implementation outside darwin/linux — syscall.Rusage
// and RUSAGE_CHILDREN are unix-specific, the same precedent as
// rss_maxrss_other.go: no gauge rather than a build failure.
// TestNoHookRPCOutlivesItsDescriptorBudget then falls back to bounding the
// whole hook by wall-clock time, as it did before it had a CPU reading.
func processCPUTime() (time.Duration, bool) { return 0, false }
