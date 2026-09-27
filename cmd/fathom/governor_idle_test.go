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

// TestGovernorIdleCountsTicketsBetweenSamples (review finding, §5: "30 min
// without tickets"): a steady stream of short calls — hook point calls of
// 10–50 ms, each begun and ended between two 2 s samples — is not idle.
// The idle clock runs from the last ticket, not from the last sample that
// happened to see one in flight.
func TestGovernorIdleCountsTicketsBetweenSamples(t *testing.T) {
	r := newGovRig(t)
	r.g.actions.dropIdle = []namedDrop{{"memo registry", func() { r.record("drop idle") }}}
	per := int(r.g.budget.CacheIdleDrop / tenancy.SampleEvery)
	r.tick()
	for range 2 * per {
		r.now = r.now.Add(tenancy.SampleEvery / 2)
		_, end := r.g.inflight.Begin(context.Background(), tenancy.ClassPoint, "obligations", "svc", "pre-edit-hook")
		end()
		r.now = r.now.Add(tenancy.SampleEvery / 2)
		r.g.tick(r.now, 0)
	}
	if n := r.saw("drop idle"); n != 0 {
		t.Fatalf("idle drop ran %d time(s) under steady short calls", n)
	}
	r.ticks(per + 2)
	if r.saw("drop idle") != 1 {
		t.Fatalf("no idle drop once the calls stopped: %v", r.events)
	}
}
