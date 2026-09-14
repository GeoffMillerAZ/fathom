// liveprocs.go — O57 finding 1: "a driver kills what it started." Every
// subprocess this package starts in its own process group (a session's
// `claude` invocation, a wired cell's scratch daemon) registers its pgid
// here while it runs; cmd/scenariobench's top-level SIGTERM/SIGINT handler
// calls KillAllLiveProcessGroups before the driver exits, so a killed
// driver never leaves an orphan running against a shared, reused worktree
// (the O52 pattern — reapStaleScratchDaemon/subject.go already fixed this
// for the scratch daemon; this is the same fix generalized to every live
// process group, plus the guarantee that a *live* driver cleans up after
// itself instead of relying solely on the NEXT driver's own reap-on-prepare).
package scenariobench

import (
	"sync"
	"syscall"
)

// liveProcessGroups maps a registered process-group id (pgid) to a short
// label for logging — every entry here is a process this driver started
// with Setpgid: true and has not yet reaped.
var liveProcessGroups sync.Map // map[int]string

// registerLiveProcessGroup records pgid as live, under label (used only for
// logging/debugging — KillAllLiveProcessGroups does not need it to kill
// correctly). A pgid <= 1 is never registered: it can never be a real
// child's own process group and blindly SIGKILLing it would be
// catastrophic (pgid 1 owns every session-leaderless process the OS has).
func registerLiveProcessGroup(pgid int, label string) {
	if pgid <= 1 {
		return
	}
	liveProcessGroups.Store(pgid, label)
}

// unregisterLiveProcessGroup removes pgid — called once the process it
// named has been waited on (or reaped some other way) so KillAllLiveProcessGroups
// never targets a pgid that might since have been recycled by an unrelated
// process.
func unregisterLiveProcessGroup(pgid int) {
	liveProcessGroups.Delete(pgid)
}

// KillAllLiveProcessGroups SIGKILLs the process group of every pgid
// currently registered (a live session's `claude` subprocess, a wired
// cell's scratch daemon) and unregisters each as it does — the driver's
// SIGTERM/SIGINT handler (cmd/scenariobench) calls this before exiting so a
// killed driver never leaves a child running against a shared worktree.
// Returns the pgids it signalled, in no particular order, so a caller can
// log what it did.
func KillAllLiveProcessGroups() []int {
	var killed []int
	liveProcessGroups.Range(func(k, _ any) bool {
		pgid := k.(int)
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		killed = append(killed, pgid)
		liveProcessGroups.Delete(pgid)
		return true
	})
	return killed
}

// LiveProcessGroupCount reports how many process groups are currently
// registered — a test/diagnostic seam, never used to decide correctness.
func LiveProcessGroupCount() int {
	n := 0
	liveProcessGroups.Range(func(_, _ any) bool { n++; return true })
	return n
}
