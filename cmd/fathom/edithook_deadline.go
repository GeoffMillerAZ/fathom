package main

// edithook_deadline.go — EW-76 (G11, docs/bench/readings/efficiency-wave/
// hook-path-deadlines.md): a hook-path daemon call is sized to the hook it
// runs in, not to a fixed constant under it. The fleet's call log at
// 99aa205a read every hook-path abandonment at its caller's own client cap
// (2 s for the test-run hook's localize and the edit hook's obligations,
// 500 ms for the pre-edit blast radius) while the hook's own deadline, the
// descriptor's TimeoutSec less max(1 s, 20%), still had time left.

import "time"

// hookCallFloor is the least time a sized call is given, so a call made
// late in a hook is still a real attempt rather than a certain failure (the
// guard refuses any RPC once the deadline itself has passed).
const hookCallFloor = 200 * time.Millisecond

// postEditReserve is what the post-edit hook keeps back after its
// obligations call: the rendering and the local records that follow.
const postEditReserve = 500 * time.Millisecond

// hookCallTimeout is one hook-path call's client timeout: what is left of
// the running hook's deadline, less reserve for the work the hook still
// does after the call, never under hookCallFloor. Outside a hook (no guard,
// or a guard without a deadline) it is fallback, the call's old fixed cap.
func hookCallTimeout(reserve, fallback time.Duration) time.Duration {
	g := activeHookGuard()
	if g == nil || g.Deadline.IsZero() {
		return fallback
	}
	d := g.Deadline.Remaining() - reserve
	if d < hookCallFloor {
		d = hookCallFloor
	}
	return d
}
