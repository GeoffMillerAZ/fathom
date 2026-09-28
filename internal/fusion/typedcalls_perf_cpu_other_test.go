//go:build !darwin && !linux

package fusion_test

import "time"

// cpuUsage has no implementation outside darwin/linux — syscall.Rusage's
// shape (and RUSAGE_CHILDREN) is unix-specific, matching
// cmd/fathom/rss_maxrss_other.go's precedent: no gauge rather than a build
// failure. TestTypedEdges_PerformanceGate falls back to logging wall-clock
// only, with the CPU-based blowup guard skipped, on such platforms.
func cpuUsage() (time.Duration, bool) { return 0, false }
