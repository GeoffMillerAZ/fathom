//go:build darwin || linux

package main

// hookcpu_unix.go — the CPU-time reader behind the hook-latency record's
// cpu_ms (EW-29) and the tests that bound work by CPU instead of a wall
// clock. Precedent: cmd/indexbench/cputime_unix.go.

import (
	"syscall"
	"time"
)

// processCPUTime is this process's own CPU time (RUSAGE_SELF) plus its
// reaped children's (RUSAGE_CHILDREN), user+sys, since process start — the
// CPU-time counterpart to time.Since. Subtracting two readings gives what
// the work between them COST, which is the same whether the machine ran it
// at once or made it queue for a core: what a hook's latency row needs to
// tell work from waiting, and the property
// TestNoHookRPCOutlivesItsDescriptorBudget's whole-hook bound needs, since
// the git forks a hook makes are children of this process, and under load
// it is their start latency, not their CPU, that balloons. (Same reading
// as internal/fusion/perfgate's cpuUsage, TestTypedEdges_PerformanceGate.)
//
// A child's CPU is added to RUSAGE_CHILDREN only once it is reaped;
// exec.Cmd.Output waits synchronously, so every git a hook forked and
// finished is counted by the time the hook returns. A child still running
// when the hook returns is not counted.
//
// ok is false only if getrusage itself failed; never a fabricated zero.
func processCPUTime() (_ time.Duration, ok bool) {
	var self, children syscall.Rusage
	if syscall.Getrusage(syscall.RUSAGE_SELF, &self) != nil {
		return 0, false
	}
	if syscall.Getrusage(syscall.RUSAGE_CHILDREN, &children) != nil {
		return 0, false
	}
	return rusageCPU(self) + rusageCPU(children), true
}

func rusageCPU(ru syscall.Rusage) time.Duration {
	return time.Duration(ru.Utime.Nano()) + time.Duration(ru.Stime.Nano())
}
