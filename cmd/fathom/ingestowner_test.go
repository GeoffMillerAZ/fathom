package main

import (
	"testing"
	"time"
)

// TestIngestOwnerProbeRejectsDeadAndReusedPids pins review finding 3 on
// RT-109: an in_progress ingest_state row is an interruption only when its
// owner is gone. A live owner that started no later than the row is the
// ingest itself (a concurrent CLI ingest — never duplicated); a pid now
// held by a process that started after the row is a reused pid, and a
// missing pid is dead. Answers are cached briefly so a live concurrent
// ingest costs one ps per window, not one per read.
func TestIngestOwnerProbeRejectsDeadAndReusedPids(t *testing.T) {
	now := time.Date(2026, 9, 26, 22, 0, 0, 0, time.Local)
	rowStart := now.Add(-time.Minute)
	ops := newFakeDaemonOps(now)
	ops.procs[100] = procInfo{PID: 100, Start: rowStart.Add(-time.Second).Truncate(time.Second)}
	ops.procs[200] = procInfo{PID: 200, Start: rowStart.Add(10 * time.Second)}
	probe := ingestOwnerProbe(ops)
	if !probe(100, rowStart) {
		t.Error("a live owner that started before its row reads as gone")
	}
	delete(ops.procs, 100)
	if !probe(100, rowStart) {
		t.Error("the answer was not cached inside its window")
	}
	ops.now = now.Add(ingestOwnerCacheTTL)
	if probe(100, rowStart) {
		t.Error("a cached answer outlived its window")
	}
	if probe(200, rowStart) {
		t.Error("a pid reused by a process that started after the row reads as the owner")
	}
	if probe(300, rowStart) {
		t.Error("a missing pid reads as a live owner")
	}
}
