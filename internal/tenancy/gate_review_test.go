package tenancy

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestGate_CallerGoneIsNeverAdmitted (review finding, RT-58/I49): a caller
// whose context has already ended is refused by name before any work —
// in enforce and in observe alike (no one is there to serve; refusing it is
// not a resource decision) — so its handler never runs and never takes a
// slot. Off mode admits without deciding.
func TestGate_CallerGoneIsNeverAdmitted(t *testing.T) {
	gone, cancel := context.WithCancel(context.Background())
	cancel()
	for _, mode := range []Mode{ModeEnforce, ModeObserve} {
		g, _ := newTestGate(t, mode, LevelNormal)
		tk, ref := g.Admit(gone, Request{Tool: "list_services", Class: ClassFleet, Origin: OriginHook})
		if tk != nil {
			tk.Release(OutcomeOK)
			t.Fatalf("%s: a gone caller was admitted", mode)
		}
		if ref == nil || ref.Code != RefusalCallerGone || !strings.Contains(ref.Reason, "went away") {
			t.Fatalf("%s: want a caller_gone refusal, got %+v", mode, ref)
		}
		if n := g.Rejected(ClassFleet, RefusalCallerGone, OriginHook); n != 1 {
			t.Fatalf("%s: rejected{fleet,caller_gone,hook} = %d", mode, n)
		}
	}
}

// TestGate_DeadlineMedianIsPerToolAndDecays (review finding, RT-53): the
// deadline-feasibility median is the tool's own (a whole-tree search never
// sets a hook's obligations median), it forgets samples older than its
// window (refused callers never sample it, so a stale median must not hold
// a refusal forever), and an abandoned call feeds it as the lower bound it
// is.
func TestGate_DeadlineMedianIsPerToolAndDecays(t *testing.T) {
	clk := &fakeClock{t: time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC)}
	g := NewGate(GateConfig{Budget: testBudget(t), Mode: ModeEnforce, Now: clk.Now, InFlight: NewInFlight(clk.Now)})
	run := func(tool string, took time.Duration, o Outcome) {
		t.Helper()
		tk, ref := g.Admit(context.Background(), Request{Tool: tool, Class: ClassTree, Origin: OriginAgent})
		if ref != nil {
			t.Fatalf("%s refused while learning: %+v", tool, ref)
		}
		clk.Advance(took)
		tk.Release(o)
	}
	hook := func() *Refusal {
		tk, ref := g.Admit(context.Background(), Request{Tool: "obligations", Class: ClassTree, Origin: OriginHook, Deadline: clk.Now().Add(800 * time.Millisecond)})
		if tk != nil {
			tk.Release(OutcomeOK)
		}
		return ref
	}
	for range 8 {
		run("search", 3*time.Second, OutcomeOK)
	}
	if ref := hook(); ref != nil {
		t.Fatalf("search's median refused a hook's obligations call: %+v", ref)
	}
	for range 8 {
		run("obligations", 3*time.Second, OutcomeOK)
	}
	if ref := hook(); ref == nil || ref.Code != RefusalDeadline {
		t.Fatalf("obligations' own 3 s median should refuse an 800 ms deadline, got %+v", ref)
	}
	clk.Advance(24 * time.Hour)
	if ref := hook(); ref != nil {
		t.Fatalf("a day-old median still refuses: %+v", ref)
	}
	// Abandoned calls are censored samples: at least as slow as observed.
	for range 8 {
		run("obligations", 5*time.Second, OutcomeAbandoned)
	}
	if p := g.P50Tool("obligations"); p != 5*time.Second {
		t.Fatalf("abandoned calls did not feed the median: p50 %s, want 5s", p)
	}
}

// TestGate_TimingUsesInjectedClock (review finding, determinism): the
// queue wait, the queued duration and the class deadline run on the
// injected clock and timer, never the wall clock.
func TestGate_TimingUsesInjectedClock(t *testing.T) {
	clk := &fakeClock{t: time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC)}
	fire := make(chan time.Time)
	var asked time.Duration
	g := NewGate(GateConfig{Budget: testBudget(t), Mode: ModeEnforce, Now: clk.Now, InFlight: NewInFlight(clk.Now),
		Timer: func(d time.Duration) (<-chan time.Time, func() bool) {
			asked = d
			return fire, func() bool { return true }
		}})
	lo := time.Now()
	held, ref := g.Admit(context.Background(), Request{Tool: "search", Class: ClassTree, Origin: OriginAgent})
	hi := time.Now()
	if ref != nil {
		t.Fatal(ref.Reason)
	}
	// The class deadline is computed on the injected clock: a year-2000
	// clock must still give a live 60 s ticket, not an expired one — 60 s
	// from the instant Admit built the ctx, between the readings either side.
	if err := held.Context().Err(); err != nil {
		t.Fatalf("class deadline computed on the wall clock: ticket ctx %v", err)
	}
	if dl, ok := held.Context().Deadline(); !ok || dl.Before(lo.Add(60*time.Second)) || dl.After(hi.Add(60*time.Second)) {
		t.Fatalf("tree ticket deadline %v, want 60 s out within [%v, %v]", dl, lo.Add(60*time.Second), hi.Add(60*time.Second))
	}
	got := make(chan *Refusal, 1)
	go func() {
		tk, ref := g.Admit(context.Background(), Request{Tool: "cloc", Class: ClassTree, Origin: OriginAgent})
		if tk != nil {
			tk.Release(OutcomeOK)
		}
		got <- ref
	}()
	waitFor(t, func() bool { return g.Queued(ClassTree) == 1 })
	clk.Advance(5 * time.Second)
	fire <- clk.Now()
	ref = <-got
	if asked != 5*time.Second || ref == nil || ref.Code != RefusalSlots || !strings.Contains(ref.Reason, "after waiting 5s") {
		t.Fatalf("timer asked %s, refusal %+v", asked, ref)
	}

	// A waiter that gets the slot records its queued time on the injected clock.
	done := make(chan *Ticket, 1)
	go func() {
		tk, _ := g.Admit(context.Background(), Request{Tool: "cloc", Class: ClassTree, Origin: OriginAgent})
		done <- tk
	}()
	waitFor(t, func() bool { return g.Queued(ClassTree) == 1 })
	clk.Advance(1500 * time.Millisecond)
	held.Release(OutcomeOK)
	tk := <-done
	if tk == nil || tk.Queued() != 1500*time.Millisecond {
		t.Fatalf("queued %v, want 1.5s on the injected clock", tk)
	}
	tk.Release(OutcomeOK)
}

// TestDetachStripsTheTicket (review finding, RT-149): work detached from a
// request (the auto-refresh ingest) outlives it, carries neither the
// read's released ticket (whose per-call ceiling would stop the ingest)
// nor the read's producer class, and is metered as ingest.
func TestDetachStripsTheTicket(t *testing.T) {
	g, _ := newTestGate(t, ModeEnforce, LevelNormal)
	parent, cancel := context.WithCancel(context.Background())
	tk, ref := g.Admit(parent, Request{Tool: "find_symbol", Class: ClassPoint, Origin: OriginAgent})
	if ref != nil {
		t.Fatal(ref.Reason)
	}
	d := Detach(tk.Context())
	tk.Release(OutcomeOK)
	cancel()
	if d.Err() != nil {
		t.Fatal("detached ctx was cancelled with its request")
	}
	if TicketFrom(d) != nil {
		t.Fatal("detached ctx still carries the read's ticket")
	}
	if err := Charge(d, 1<<40); err != nil {
		t.Fatalf("the read's per-call ceiling reached the detached work: %v", err)
	}
	if c := ProducerClassOf(d, ClassTree); c != ClassIngest {
		t.Fatalf("detached producer class %s, want ingest", c)
	}
}

// TestTicket_StoppedOnlyWhenEnforced (review finding, RT-54/RT-57): only an
// enforcing meter stops a producer, so only an enforcing ticket names its
// answer partial; observe records the over-ceiling call and the answer
// stays complete. A producer reports what it did not reach (Elide) and the
// budget block carries it as a lower bound.
func TestTicket_StoppedOnlyWhenEnforced(t *testing.T) {
	o, _ := newTestGate(t, ModeObserve, LevelNormal)
	tk, _ := o.Admit(context.Background(), Request{Tool: "read_source", Class: ClassPoint, Origin: OriginAgent})
	_ = tk.Charge(1 << 40)
	if !tk.OverBudget() || tk.Stopped() {
		t.Fatalf("observe: over %v stopped %v, want over and not stopped", tk.OverBudget(), tk.Stopped())
	}
	tk.Release(OutcomeOK)

	e, _ := newTestGate(t, ModeEnforce, LevelNormal)
	tk, _ = e.Admit(context.Background(), Request{Tool: "read_source", Class: ClassPoint, Origin: OriginAgent})
	if err := tk.Charge(1 << 40); err == nil || !tk.Stopped() {
		t.Fatalf("enforce: err %v stopped %v", err, tk.Stopped())
	}
	tk.Elide(3)
	tk.Elide(2)
	if p := tk.Partial(); p.Elided != 5 || !p.Partial {
		t.Fatalf("partial %+v, want elided 5", p)
	}
	tk.Release(OutcomePartial)
}
