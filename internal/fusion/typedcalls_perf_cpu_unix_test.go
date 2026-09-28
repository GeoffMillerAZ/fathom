//go:build darwin || linux

package fusion_test

import (
	"syscall"
	"time"
)

// cpuUsage returns this process's own (RUSAGE_SELF) CPU time plus its
// reaped children's (RUSAGE_CHILDREN) CPU time, user+sys combined, since
// process start. It is the CPU-time counterpart to time.Since(start): take
// a reading before and after a phase and subtract to get that phase's true
// CPU cost — immune to how much wall-clock time the phase spent queued for
// a core under machine load. That's exactly the property
// TestTypedEdges_PerformanceGate needs: the typed pass shells out to
// go/packages child processes (go list / go build) that queue far worse
// under contention than the in-process syntax baseline does, which made
// the old wall-clock ratio flip on load, not on a real regression.
//
// Child CPU time is only added to RUSAGE_CHILDREN once a child has been
// reaped (wait4'd); exec.Cmd.Run (what go/packages uses internally) waits
// synchronously, so by the time EnrichTypedEdges returns, every child it
// spawned has already been reaped and counted here.
//
// ok is false only if the getrusage syscall itself failed; never a
// fabricated zero.
func cpuUsage() (_ time.Duration, ok bool) {
	var self, children syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &self); err != nil {
		return 0, false
	}
	if err := syscall.Getrusage(syscall.RUSAGE_CHILDREN, &children); err != nil {
		return 0, false
	}
	return rusageCPU(self) + rusageCPU(children), true
}

func rusageCPU(ru syscall.Rusage) time.Duration {
	return time.Duration(ru.Utime.Nano()) + time.Duration(ru.Stime.Nano())
}
