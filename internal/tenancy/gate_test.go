package tenancy

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// testBudget is a 1 GiB managed budget: tree 1 slot, fleet 1, point 16,
// index 2 on a 4-CPU host.
func testBudget(t *testing.T) Budget {
	t.Helper()
	b, _ := ResolveBudget(Config{Budget: 1 * GiB}, HostFacts{RAM: 16 * GiB, NumCPU: 4, Managed: true})
	return b
}

// fakeClock is an injectable, advanceable clock.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func newTestGate(t *testing.T, mode Mode, level Level) (*Gate, *[]string) {
	t.Helper()
	var mu sync.Mutex
	var logs []string
	g := NewGate(GateConfig{
		Budget:   testBudget(t),
		Mode:     mode,
		Level:    func() Level { return level },
		InFlight: NewInFlight(nil),
		Log: func(s string) {
			mu.Lock()
			logs = append(logs, s)
			mu.Unlock()
		},
	})
	return g, &logs
}

// TestGate_HookNeverWaitsForHeavy (RT-53): a hook asking for a full tree or
// fleet class is refused at once, named, never parked behind the heavy
// call — the 19-minute list_services behind 2-second hooks.
func TestGate_HookNeverWaitsForHeavy(t *testing.T) {
	g, _ := newTestGate(t, ModeEnforce, LevelNormal)
	ctx := context.Background()
	held, ref := g.Admit(ctx, Request{Tool: "search", Class: ClassTree, Origin: OriginAgent})
	if ref != nil {
		t.Fatalf("first tree call refused: %+v", ref)
	}
	defer held.Release(OutcomeOK)
	fleet, ref := g.Admit(ctx, Request{Tool: "list_services", Class: ClassFleet, Origin: OriginAgent})
	if ref != nil {
		t.Fatalf("first fleet call refused: %+v", ref)
	}
	defer fleet.Release(OutcomeOK)

	for _, c := range []Class{ClassTree, ClassFleet} {
		// Both heavy slots stay held until the test ends, so an Admit that
		// queued for one could never return: returning is "never waits".
		var tk *Ticket
		var ref *Refusal
		returnsUnaided(t, c.String()+": a hook asking a full heavy class", func() {
			tk, ref = g.Admit(ctx, Request{Tool: "obligations", Class: c, Origin: OriginHook})
		})
		if tk != nil {
			tk.Release(OutcomeOK)
			t.Fatalf("%s: hook admitted into a full class", c)
		}
		if ref == nil || ref.Code != RefusalSlots {
			t.Fatalf("%s: want a slots refusal, got %+v", c, ref)
		}
		if ref.RetryAfter <= 0 {
			t.Errorf("%s: refusal carries no Retry-After", c)
		}
	}
	_, ref = g.Admit(ctx, Request{Tool: "obligations", Class: ClassTree, Origin: OriginHook})
	if want := "tree-class slots full (1/1): search"; !strings.Contains(ref.Reason, want) {
		t.Fatalf("reason %q, want it to name %q", ref.Reason, want)
	}
	// A hook's point call is admitted (point slots are free).
	pt, ref := g.Admit(ctx, Request{Tool: "capabilities", Class: ClassPoint, Origin: OriginHook})
	if ref != nil {
		t.Fatalf("point refused: %+v", ref)
	}
	pt.Release(OutcomeOK)
	if n := g.Rejected(ClassTree, RefusalSlots, OriginHook); n != 2 {
		t.Fatalf("rejected{tree,slots,hook} = %d, want 2", n)
	}
}

// TestGate_WaiterHonoursContext (RT-53): a queued agent leaves the queue
// the moment its context ends, and a released slot goes to the next waiter.
func TestGate_WaiterHonoursContext(t *testing.T) {
	g, _ := newTestGate(t, ModeEnforce, LevelNormal)
	held, _ := g.Admit(context.Background(), Request{Tool: "search", Class: ClassTree, Origin: OriginAgent})

	ctx, cancel := context.WithCancel(context.Background())
	got := make(chan *Refusal, 1)
	go func() {
		tk, ref := g.Admit(ctx, Request{Tool: "cloc", Class: ClassTree, Origin: OriginAgent})
		if tk != nil {
			tk.Release(OutcomeOK)
		}
		got <- ref
	}()
	waitFor(t, func() bool { return g.Queued(ClassTree) == 1 })
	cancel()
	select {
	case ref := <-got:
		if ref == nil || ref.Code != RefusalCallerGone {
			t.Fatalf("want caller-gone, got %+v", ref)
		}
	case <-time.After(waitSafety):
		t.Fatal("a cancelled waiter stayed queued")
	}

	// The next waiter gets the slot once it is released.
	done := make(chan *Ticket, 1)
	go func() {
		tk, _ := g.Admit(context.Background(), Request{Tool: "cloc", Class: ClassTree, Origin: OriginAgent})
		done <- tk
	}()
	waitFor(t, func() bool { return g.Queued(ClassTree) == 1 })
	held.Release(OutcomeOK)
	select {
	case tk := <-done:
		if tk == nil {
			t.Fatal("waiter refused after the slot was released")
		}
		if tk.Queued() <= 0 {
			t.Errorf("queued time not recorded")
		}
		tk.Release(OutcomeOK)
	case <-time.After(waitSafety):
		t.Fatal("waiter never got the released slot")
	}
}

// TestGate_DeadlineFeasibility (RT-53): a request whose remaining deadline
// is below the class's observed p50 is refused at once, named.
func TestGate_DeadlineFeasibility(t *testing.T) {
	clk := &fakeClock{t: time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)}
	g := NewGate(GateConfig{Budget: testBudget(t), Mode: ModeEnforce, Now: clk.Now, InFlight: NewInFlight(clk.Now)})
	for range 9 {
		tk, ref := g.Admit(context.Background(), Request{Tool: "find_symbol", Class: ClassIndex, Origin: OriginAgent})
		if ref != nil {
			t.Fatalf("refused while learning: %+v", ref)
		}
		clk.Advance(2 * time.Second)
		tk.Release(OutcomeOK)
	}
	if p := g.P50(ClassIndex); p != 2*time.Second {
		t.Fatalf("p50 = %s, want 2s", p)
	}
	_, ref := g.Admit(context.Background(), Request{Tool: "find_symbol", Class: ClassIndex, Origin: OriginHook, Deadline: clk.Now().Add(time.Second)})
	if ref == nil || ref.Code != RefusalDeadline || !strings.Contains(ref.Reason, "cannot finish before your deadline") {
		t.Fatalf("want a deadline refusal, got %+v", ref)
	}
	tk, ref := g.Admit(context.Background(), Request{Tool: "find_symbol", Class: ClassIndex, Origin: OriginHook, Deadline: clk.Now().Add(5 * time.Second)})
	if ref != nil {
		t.Fatalf("a feasible deadline was refused: %+v", ref)
	}
	tk.Release(OutcomeOK)
}

// TestGate_ClassDeadlineAndRefuseRung (RT-53): the ticket's context carries
// min(client deadline, class deadline), and the refuse rung's
// cancellation reaches it through the in-flight registry.
func TestGate_ClassDeadlineAndRefuseRung(t *testing.T) {
	reg := NewInFlight(nil)
	// The gate reads one frozen instant, so the time each ticket has left is
	// exact (class deadline 60 s; the client's own nearer 3 s) and the
	// context's wall-clock deadline is now+that for the moment Admit builds
	// it: it lies between the readings either side of the call. A stalled
	// machine only widens the bracket, never breaks it.
	now0 := time.Now()
	g := NewGate(GateConfig{Budget: testBudget(t), Mode: ModeEnforce, InFlight: reg, Now: func() time.Time { return now0 }})
	admitDeadline := func(req Request) (dl, lo, hi time.Time, tk *Ticket) {
		lo = time.Now()
		tk, ref := g.Admit(context.Background(), req)
		hi = time.Now()
		if ref != nil {
			t.Fatal(ref.Reason)
		}
		dl, _ = tk.Context().Deadline()
		return dl, lo, hi, tk
	}
	dl, lo, hi, tk := admitDeadline(Request{Tool: "search", Class: ClassTree, Origin: OriginAgent})
	if dl.Before(lo.Add(60*time.Second)) || dl.After(hi.Add(60*time.Second)) {
		t.Fatalf("tree ticket deadline %v, want now+60s within [%v, %v]", dl, lo.Add(60*time.Second), hi.Add(60*time.Second))
	}
	dl2, lo2, hi2, tk2 := admitDeadline(Request{Tool: "find_symbol", Class: ClassIndex, Origin: OriginAgent, Deadline: now0.Add(3 * time.Second)})
	if dl2.Before(lo2.Add(3*time.Second)) || dl2.After(hi2.Add(3*time.Second)) {
		t.Fatalf("index ticket deadline %v, want the client's 3s within [%v, %v]", dl2, lo2.Add(3*time.Second), hi2.Add(3*time.Second))
	}
	tk2.Release(OutcomeOK)
	if got := reg.Cancel(ErrAbandonedByGovernor, ClassTree); len(got) != 1 || got[0].Tool != "search" {
		t.Fatalf("refuse rung did not see the tree ticket: %+v", got)
	}
	if !errors.Is(context.Cause(tk.Context()), ErrAbandonedByGovernor) {
		t.Fatalf("ticket ctx cause %v", context.Cause(tk.Context()))
	}
	tk.Release(OutcomeAbandoned)
	if n := len(reg.Snapshot()); n != 0 {
		t.Fatalf("%d entries left registered after release", n)
	}
}

// TestGate_RefuseLevelRefusesHeavyServesPoint (RT-53, §2.1): at the refuse
// rung tree, fleet, ingest and egress are refused; point and index run.
func TestGate_RefuseLevelRefusesHeavyServesPoint(t *testing.T) {
	g, _ := newTestGate(t, ModeEnforce, LevelRefuse)
	for _, c := range []Class{ClassTree, ClassFleet, ClassIngest, ClassEgress} {
		if _, ref := g.Admit(context.Background(), Request{Tool: "x", Class: c, Origin: OriginAgent}); ref == nil || ref.Code != RefusalLevel || ref.Level != "refuse" {
			t.Fatalf("%s at refuse: want a level refusal, got %+v", c, ref)
		}
	}
	for _, c := range []Class{ClassPoint, ClassIndex} {
		tk, ref := g.Admit(context.Background(), Request{Tool: "x", Class: c, Origin: OriginAgent})
		if ref != nil {
			t.Fatalf("%s at refuse refused: %+v", c, ref)
		}
		tk.Release(OutcomeOK)
	}
}

// TestGate_MeterReturnsPartial (RT-53): the per-call byte meter stops a
// producer at the class ceiling, and the producer returns what it has,
// named as partial with the bytes read.
func TestGate_MeterReturnsPartial(t *testing.T) {
	g, _ := newTestGate(t, ModeEnforce, LevelNormal)
	tk, _ := g.Admit(context.Background(), Request{Tool: "read_source", Class: ClassPoint, Origin: OriginAgent})
	ctx := tk.Context()
	files := []int64{3 * MiB, 3 * MiB, 3 * MiB, 3 * MiB}
	var kept int
	var stop error
	for _, n := range files {
		if err := Charge(ctx, n); err != nil {
			stop = err
			break
		}
		kept++
	}
	if !errors.Is(stop, ErrOverBudget) {
		t.Fatalf("producer never stopped: %v", stop)
	}
	if kept != 2 {
		t.Fatalf("kept %d files, want 2 under an 8 MiB point ceiling", kept)
	}
	tk.Elide(len(files) - kept)
	p := tk.Partial()
	if !p.Partial || p.Elided != 2 || p.BytesRead != 9*MiB || !strings.Contains(p.Reason, "point") {
		t.Fatalf("partial = %+v", p)
	}
	tk.Release(OutcomePartial)
	// Charge with no ticket is a no-op, never an error.
	if err := Charge(context.Background(), 1<<40); err != nil {
		t.Fatalf("Charge without a ticket: %v", err)
	}
}

// TestGate_ObserveNeverRefuses (RT-54): observe mode runs the whole
// decision, logs and counts every would-refuse, and admits — without ever
// waiting and without the meter stopping a producer.
func TestGate_ObserveNeverRefuses(t *testing.T) {
	g, logs := newTestGate(t, ModeObserve, LevelNormal)
	var held []*Ticket
	for range 3 {
		// Each admitted ticket stays held, so the third Admit, over the
		// class's one slot, could not return if observe mode queued it.
		var tk *Ticket
		var ref *Refusal
		returnsUnaided(t, "observe mode: an Admit into a full class", func() {
			tk, ref = g.Admit(context.Background(), Request{Tool: "search", Class: ClassTree, Origin: OriginHook})
		})
		if ref != nil || tk == nil {
			t.Fatalf("observe refused: %+v", ref)
		}
		held = append(held, tk)
	}
	if n := g.WouldRefuse(ClassTree, RefusalSlots, OriginHook); n != 2 {
		t.Fatalf("would-refuse{tree,slots,hook} = %d, want 2", n)
	}
	if n := g.Rejected(ClassTree, RefusalSlots, OriginHook); n != 0 {
		t.Fatalf("observe counted %d real refusals", n)
	}
	if len(*logs) != 2 || !strings.Contains((*logs)[0], "would refuse") || !strings.Contains((*logs)[0], "tree-class slots full") {
		t.Fatalf("logs = %q", *logs)
	}
	// The meter counts but never stops a producer in observe mode.
	if err := held[0].Charge(1 << 40); err != nil {
		t.Fatalf("observe meter stopped a producer: %v", err)
	}
	if !held[0].OverBudget() {
		t.Fatal("observe meter did not record the over-ceiling call")
	}
	for _, tk := range held {
		tk.Release(OutcomeOK)
	}

	lv, _ := newTestGate(t, ModeObserve, LevelRefuse)
	tk, ref := lv.Admit(context.Background(), Request{Tool: "list_services", Class: ClassFleet, Origin: OriginAgent})
	if ref != nil {
		t.Fatalf("observe refused at the refuse rung: %+v", ref)
	}
	tk.Release(OutcomeOK)
	if lv.WouldRefuse(ClassFleet, RefusalLevel, OriginAgent) != 1 {
		t.Fatal("level would-refuse not counted")
	}

	off, offLogs := newTestGate(t, ModeOff, LevelRefuse)
	tk, ref = off.Admit(context.Background(), Request{Tool: "list_services", Class: ClassFleet, Origin: OriginAgent})
	if ref != nil {
		t.Fatalf("off refused: %+v", ref)
	}
	tk.Release(OutcomeOK)
	if len(*offLogs) != 0 || off.WouldRefuse(ClassFleet, RefusalLevel, OriginAgent) != 0 {
		t.Fatal("off mode decided")
	}
}

// TestParseOrigin maps the declared origin and the caller's test flag.
func TestParseOrigin(t *testing.T) {
	cases := map[string]Origin{"": OriginAgent, "edit-hook": OriginHook, "prime": OriginHook, "console": OriginConsole,
		"console-inprocess": OriginConsoleInProcess, "verify-cli": OriginCLI, "doctor": OriginCLI}
	for in, want := range cases {
		if got := ParseOrigin(in, false); got != want {
			t.Errorf("ParseOrigin(%q) = %s, want %s", in, got, want)
		}
	}
	if ParseOrigin("edit-hook", true) != OriginTest {
		t.Error("test flag must win")
	}
}

// TestGate_MetricLinesSorted: the exposition is deterministic.
func TestGate_MetricLinesSorted(t *testing.T) {
	g, _ := newTestGate(t, ModeEnforce, LevelRefuse)
	g.Admit(context.Background(), Request{Tool: "x", Class: ClassTree, Origin: OriginHook})
	g.Admit(context.Background(), Request{Tool: "x", Class: ClassFleet, Origin: OriginAgent})
	a, b := g.MetricLines(), g.MetricLines()
	if a != b || !strings.Contains(a, `fathom_rejected_total{class="fleet",reason="level",origin="agent"} 1`) {
		t.Fatalf("metrics:\n%s", a)
	}
	if !strings.Contains(a, "fathom_admission_mode{mode=\"enforce\"} 1") {
		t.Fatalf("mode gauge missing:\n%s", a)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(waitSafety)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition never held")
		}
		time.Sleep(time.Millisecond)
	}
}

// TestRefusalNamesSwapVolumeFloor (EW-37): a refuse rung lifted by the
// swap-volume floor reads as the swap-volume floor, never as a memory
// budget; an RSS-mark cause keeps the budget wording.
func TestRefusalNamesSwapVolumeFloor(t *testing.T) {
	r := newRig(t)
	r.h.pressure, r.h.swapVolFree = PressureWarn, 4*GiB
	var tr Transition
	for range 2 {
		tr = r.step(0)
	}
	if tr.To != LevelRefuse || tr.Cause.BudgetCause() {
		t.Fatalf("swap-volume rise: %+v", tr)
	}
	g := NewGate(GateConfig{
		Budget: testBudget(t), Mode: ModeEnforce, InFlight: NewInFlight(nil),
		Level: func() Level { return tr.To }, Cause: func() Cause { return tr.Cause },
	})
	_, ref := g.Admit(context.Background(), Request{Tool: "x", Class: ClassTree, Origin: OriginAgent})
	if ref == nil || ref.Code != RefusalLevel {
		t.Fatalf("want a level refusal, got %+v", ref)
	}
	if !strings.Contains(ref.Reason, "swap volume free") || strings.Contains(ref.Reason, "memory budget") {
		t.Fatalf("refusal must name the swap-volume floor, not a memory budget: %q", ref.Reason)
	}
	if err := AbandonedBy(tr.Cause); !errors.Is(err, ErrAbandonedByGovernor) || strings.Contains(err.Error(), "memory budget") {
		t.Fatalf("abandon cause: %v", err)
	}

	m := newRig(t)
	m.h.footprint = 4200 * MiB
	var mt Transition
	for range 2 {
		mt = m.step(0)
	}
	g2 := NewGate(GateConfig{
		Budget: testBudget(t), Mode: ModeEnforce, InFlight: NewInFlight(nil),
		Level: func() Level { return mt.To }, Cause: func() Cause { return mt.Cause },
	})
	_, ref = g2.Admit(context.Background(), Request{Tool: "x", Class: ClassTree, Origin: OriginAgent})
	if ref == nil || !strings.Contains(ref.Reason, "memory budget") {
		t.Fatalf("RSS-mark cause keeps the budget wording: %+v", ref)
	}
}
