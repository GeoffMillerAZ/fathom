package main

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// TestGovernorIdleDropsMemoOnce is RT-73's idle clause (§5): after
// CacheIdleDrop with no in-flight ticket the governor drops the memo
// registry once and returns the memory to the OS; a ticket re-arms it.
func TestGovernorIdleDropsMemoOnce(t *testing.T) {
	r := newGovRig(t)
	r.g.actions.dropIdle = []namedDrop{{"memo registry", func() { r.record("drop idle") }}}
	if r.g.budget.CacheIdleDrop <= 0 {
		t.Fatalf("CacheIdleDrop = %s, want the 30 min default", r.g.budget.CacheIdleDrop)
	}
	per := int(r.g.budget.CacheIdleDrop / tenancy.SampleEvery)
	r.ticks(per - 2)
	if r.saw("drop idle") != 0 {
		t.Fatal("dropped before the idle window elapsed")
	}
	r.ticks(4)
	if r.saw("drop idle") != 1 || r.saw("free") == 0 {
		t.Fatalf("idle drop not run once with a scavenge: %v", r.events)
	}
	r.ticks(per + 2)
	if r.saw("drop idle") != 1 {
		t.Fatal("idle drop repeated without a ticket in between")
	}
	_, end := r.g.inflight.Begin(context.Background(), tenancy.ClassPoint, "find_symbol", "svc", "agent")
	r.tick()
	end()
	r.ticks(per + 2)
	if r.saw("drop idle") != 2 {
		t.Fatalf("a ticket did not re-arm the idle drop: %v", r.events)
	}
}
