package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// govHost is the governor tests' injectable host.
type govHost struct {
	mu                  sync.Mutex
	footprint, children int64
	pressure            tenancy.PressureLevel
}

func (h *govHost) set(footprint, children int64) {
	h.mu.Lock()
	h.footprint, h.children = footprint, children
	h.mu.Unlock()
}

func (h *govHost) ReadHost(s *tenancy.Sample) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s.Footprint, s.Children = h.footprint, h.children
	s.Pressure = h.pressure
	if s.Pressure == tenancy.PressureUnknown {
		s.Pressure = tenancy.PressureNormal
	}
	s.SwapUsed, s.SwapTotal = 4*tenancy.GiB, 6*tenancy.GiB
	s.StoreFree, s.TmpFree, s.SwapVolumeFree = 40*tenancy.GiB, 40*tenancy.GiB, 40*tenancy.GiB
}

// govRig is a governor over a fake host, an injected clock and recorded
// actions.
type govRig struct {
	t      *testing.T
	g      *governor
	host   *govHost
	now    time.Time
	ledger string
	events []string // the actions, in order
	exited []int
	mu     sync.Mutex
}

func (r *govRig) record(e string) {
	r.mu.Lock()
	r.events = append(r.events, e)
	r.mu.Unlock()
}

func (r *govRig) saw(e string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, x := range r.events {
		if x == e {
			n++
		}
	}
	return n
}

func newGovRig(t *testing.T) *govRig {
	t.Helper()
	t.Cleanup(func() { tenancy.PublishLevel(tenancy.LevelNormal); tenancy.SetQuarantined(nil) })
	b, _ := tenancy.ResolveBudget(tenancy.Config{}, tenancy.HostFacts{RAM: 32 * tenancy.GiB, NumCPU: 12, Managed: true}) // 4 GiB, exit 8 GiB
	r := &govRig{t: t, host: &govHost{footprint: tenancy.GiB}, now: time.Date(2026, 9, 26, 21, 0, 0, 0, time.UTC)}
	r.ledger = filepath.Join(t.TempDir(), "daemon", "fathom-test", "lifecycle.jsonl")
	clock := func() time.Time { return r.now }
	r.g = newGovernor(governorConfig{
		budget:   b,
		host:     r.host,
		now:      clock,
		inflight: tenancy.NewInFlight(clock),
		ledger:   r.ledger,
		pid:      4242,
		version:  "test",
		logf:     func(string) {},
		actions: ladderActions{
			dropShed:        []namedDrop{{"console memos", func() { r.record("drop console") }}, {"fleet memos", func() { r.record("drop fleet") }}},
			dropRefuse:      []namedDrop{{"graph cache", func() { r.record("drop graph") }}, {"symbol cache", func() { r.record("drop symbols") }}},
			pauseBackground: func(p bool) { r.record(map[bool]string{true: "pause", false: "resume"}[p]) },
		},
		freeOSMemory: func() { r.record("free") },
		killChildren: func() int { r.record("kill children"); return 1 },
		captureDiag:  func(trigger string) string { r.record("diag " + trigger); return "/diag/" + trigger },
		exit:         func(code int) { r.record("exit"); r.exited = append(r.exited, code) },
	})
	return r
}

func (r *govRig) tick() tenancy.Transition {
	r.now = r.now.Add(tenancy.SampleEvery)
	return r.g.tick(r.now, 0)
}

func (r *govRig) ticks(n int) {
	for range n {
		r.tick()
	}
}

// RT-34 (§2.1): shed pauses background work, drops the console and fleet
// memos but keeps the graph and symbol caches, and scavenges once on
// entry; refuse drops every cache.
func TestShedDropsCachesAndPausesBackground(t *testing.T) {
	r := newGovRig(t)
	r.ticks(3)
	if len(r.events) != 0 {
		t.Fatalf("normal does nothing: %v", r.events)
	}
	r.host.set(3200*tenancy.MiB, 0)
	r.ticks(2)
	if tenancy.CurrentLevel() != tenancy.LevelShed {
		t.Fatalf("published level %s, want shed", tenancy.CurrentLevel())
	}
	for _, e := range []string{"drop console", "drop fleet", "pause", "free"} {
		if r.saw(e) != 1 {
			t.Fatalf("shed entry must %q once: %v", e, r.events)
		}
	}
	if r.saw("drop graph") != 0 || r.saw("drop symbols") != 0 {
		t.Fatalf("shed keeps the graph and symbol caches: %v", r.events)
	}
	// Staying in shed: no second scavenge within 30 s.
	r.ticks(5)
	if r.saw("free") != 1 {
		t.Fatalf("FreeOSMemory at most once per 30 s: %v", r.events)
	}
	// Refuse drops every cache.
	r.host.set(4500*tenancy.MiB, 0)
	r.ticks(2)
	if tenancy.CurrentLevel() != tenancy.LevelRefuse {
		t.Fatalf("level %s", tenancy.CurrentLevel())
	}
	if r.saw("drop graph") != 1 || r.saw("drop symbols") != 1 || r.saw("drop console") != 2 {
		t.Fatalf("refuse drops every cache: %v", r.events)
	}
	if r.saw("diag refuse") != 1 {
		t.Fatalf("refuse entry captures diagnostics once: %v", r.events)
	}
	// Back down to normal resumes background work.
	r.host.set(tenancy.GiB, 0)
	r.ticks(70)
	if tenancy.CurrentLevel() != tenancy.LevelNormal || r.saw("resume") != 1 {
		t.Fatalf("normal again resumes background work: level %s, %v", tenancy.CurrentLevel(), r.events)
	}
	if st := r.g.snapshot(); st.Level != "normal" || st.Budget != 4*tenancy.GiB || st.Footprint != tenancy.GiB {
		t.Fatalf("snapshot: %+v", st)
	}
}

// RT-172 (§2.1): refuse cancels every in-flight tree and fleet ticket,
// youngest first, as abandoned-by-governor; point and index work runs on.
func TestRefuseCancelsAllHeavyTicketsYoungestFirst(t *testing.T) {
	r := newGovRig(t)
	type held struct {
		ctx  context.Context
		done func()
		name string
	}
	var hs []held
	begin := func(c tenancy.Class, tool string) {
		ctx, done := r.g.inflight.Begin(context.Background(), c, tool, "svc", "agent")
		hs = append(hs, held{ctx, done, tool})
		r.now = r.now.Add(time.Second)
	}
	begin(tenancy.ClassTree, "search")
	begin(tenancy.ClassPoint, "read_source")
	begin(tenancy.ClassFleet, "list_services")
	begin(tenancy.ClassIndex, "find_symbol")
	begin(tenancy.ClassTree, "obligations")
	defer func() {
		for _, h := range hs {
			h.done()
		}
	}()

	r.host.set(4500*tenancy.MiB, 0)
	r.ticks(2)
	if tenancy.CurrentLevel() != tenancy.LevelRefuse {
		t.Fatalf("level %s", tenancy.CurrentLevel())
	}
	for _, h := range hs {
		cancelled := h.ctx.Err() != nil
		heavy := h.name == "search" || h.name == "list_services" || h.name == "obligations"
		if cancelled != heavy {
			t.Fatalf("%s: cancelled %v, want %v", h.name, cancelled, heavy)
		}
		if heavy && !errors.Is(context.Cause(h.ctx), tenancy.ErrAbandonedByGovernor) {
			t.Fatalf("%s: cause %v, want abandoned-by-governor", h.name, context.Cause(h.ctx))
		}
	}
	got := strings.Join(r.g.lastCancelled(), ",")
	if got != "obligations,list_services,search" {
		t.Fatalf("cancelled youngest first: %q", got)
	}
	// The abandoned outcome is counted where /metrics reads it.
	if n := r.g.snapshot().Abandoned; n != 3 {
		t.Fatalf("snapshot abandoned = %d, want 3", n)
	}
	activeGovernor.Store(r.g)
	defer activeGovernor.Store(nil)
	if m := governorMetricLines(); !strings.Contains(m, "fathom_governor_abandoned_total 3\n") {
		t.Fatalf("/metrics lacks the abandoned count:\n%s", m)
	}

	// The declared classes (RT-55) the admission ticket registers under.
	for _, tc := range []struct {
		tool, args string
		want       tenancy.Class
	}{
		{"search", `{}`, tenancy.ClassTree},
		{"symbol_context", `{}`, tenancy.ClassTree},
		{"references", `{"symbol":"X"}`, tenancy.ClassTree},
		{"references", `{"symbol":"X","services":["*"]}`, tenancy.ClassFleet},
		{"cross_repo_callers", `{}`, tenancy.ClassFleet},
		{"find_symbol", `{}`, tenancy.ClassIndex},
		{"read_source", `{}`, tenancy.ClassPoint},
		{"ingest", `{}`, tenancy.ClassIngest},
	} {
		if got := rpcToolClass(tc.tool, json.RawMessage(tc.args)); got != tc.want {
			t.Errorf("%s %s: class %s, want %s", tc.tool, tc.args, got, tc.want)
		}
	}
}

// RT-173 (§2.1): at the exit mark an in-flight ingest (or typed pass) is
// halted first — every producer stops, the ingest's ctx is cancelled, its
// children killed, its service quarantined, memory returned — and the
// daemon exits only if the next sample is still over.
func TestExitMarkHaltsIngestBeforeExiting(t *testing.T) {
	r := newGovRig(t)
	ictx, done := r.g.inflight.Begin(context.Background(), tenancy.ClassIngest, "ingest", "app-kit", "")
	defer done()

	r.host.set(9*tenancy.GiB, 0)
	r.ticks(2)
	if len(r.exited) != 0 {
		t.Fatalf("with an ingest in flight the first exit sample halts it, never exits: %v", r.events)
	}
	if !errors.Is(context.Cause(ictx), tenancy.ErrHaltedOverBudget) {
		t.Fatalf("the ingest is halted: cause %v", context.Cause(ictx))
	}
	if tenancy.CurrentLevel() != tenancy.LevelExit || tenancy.ProducerAllows(tenancy.ClassPoint) == nil {
		t.Fatalf("at the exit mark every producer halts: level %s", tenancy.CurrentLevel())
	}
	if r.saw("kill children") != 1 || r.saw("free") < 1 {
		t.Fatalf("the halt kills the ingest's children and returns memory: %v", r.events)
	}
	if !tenancy.IsQuarantined("app-kit") {
		t.Fatalf("the halted ingest's service is quarantined")
	}

	// The halt freed memory: the next sample is below the exit mark — no
	// exit, the ladder holds refuse.
	r.host.set(5*tenancy.GiB, 0)
	r.tick()
	if len(r.exited) != 0 || tenancy.CurrentLevel() != tenancy.LevelRefuse {
		t.Fatalf("below the exit mark after the halt: exited %v, level %s", r.exited, tenancy.CurrentLevel())
	}

	// A second halt that frees nothing: the next sample still over exits.
	r2 := newGovRig(t)
	_, done2 := r2.g.inflight.Begin(context.Background(), tenancy.ClassIngest, "ingest", "big-repo", "")
	defer done2()
	r2.host.set(9*tenancy.GiB, 0)
	r2.ticks(2)
	if len(r2.exited) != 0 {
		t.Fatalf("halt first")
	}
	r2.tick()
	if len(r2.exited) != 1 || r2.exited[0] != 70 {
		t.Fatalf("still over on the next sample: exit 70, got %v (%v)", r2.exited, r2.events)
	}
}

// RT-35 (§2.1): with nothing to halt, the exit rung kills the child tree,
// captures diagnostics, writes the exit row with the reason and the top
// in-flight work, and exits 70 at once — no drain of in-flight requests.
func TestLadderExitsOverBudget(t *testing.T) {
	r := newGovRig(t)
	_, done := r.g.inflight.Begin(context.Background(), tenancy.ClassPoint, "read_source", "svc", "hook")
	defer done() // never released before the exit: no drain waits for it
	r.now = r.now.Add(3 * time.Second)
	_, done2 := r.g.inflight.Begin(context.Background(), tenancy.ClassTree, "search", "svc", "agent")
	defer done2()

	r.host.set(9*tenancy.GiB, 512*tenancy.MiB)
	r.tick()
	if len(r.exited) != 0 {
		t.Fatalf("one sample is not an exit")
	}
	r.tick()
	if len(r.exited) != 1 || r.exited[0] != 70 {
		t.Fatalf("exit 70, got %v", r.exited)
	}
	order := strings.Join(r.events, "|")
	if !strings.Contains(order, "kill children|diag exit|exit") {
		t.Fatalf("children killed, then diagnostics, then exit: %s", order)
	}
	rows := readGovLedgerRows(t, r.ledger)
	var exitRow map[string]any
	for _, row := range rows {
		if row["row"] == "exit" {
			exitRow = row
		}
	}
	if exitRow == nil {
		t.Fatalf("no exit row in %v", rows)
	}
	if exitRow["reason"] != "over_budget" || exitRow["code"] != float64(70) || exitRow["pid"] != float64(4242) {
		t.Fatalf("exit row: %v", exitRow)
	}
	if exitRow["footprint"] != float64(9*tenancy.GiB) || exitRow["children"] != float64(512*tenancy.MiB) || exitRow["diag"] != "/diag/exit" {
		t.Fatalf("exit row sample: %v", exitRow)
	}
	top, _ := json.Marshal(exitRow["top_in_flight"])
	if !strings.Contains(string(top), "read_source") || !strings.Contains(string(top), "search") {
		t.Fatalf("the exit row names the top in-flight work: %s", top)
	}
	if !strings.Contains(exitRow["detail"].(string), "exit mark") {
		t.Fatalf("the reason names the rung: %v", exitRow["detail"])
	}
	// Nothing acts after the exit.
	r.tick()
	if len(r.exited) != 1 {
		t.Fatalf("the governor stops after exiting: %v", r.exited)
	}
}

// RT-155 (§2.3): a service whose ingest was halted, or was in flight at an
// over-budget exit, is quarantined in the ledger, named, loaded by the next
// start and never re-queued until an explicit ingest clears it.
func TestIngestInFlightAtOverBudgetExitQuarantines(t *testing.T) {
	r := newGovRig(t)
	_, doneA := r.g.inflight.Begin(context.Background(), tenancy.ClassIngest, "ingest", "app-kit", "")
	defer doneA()
	r.host.set(9*tenancy.GiB, 0)
	r.ticks(2) // halt + quarantine app-kit
	// A second ingest slipped in after the halt (it is refused by its
	// producers, but still registered) when the daemon exits.
	_, doneB := r.g.inflight.Begin(context.Background(), tenancy.ClassIngest, "ingest", "zeta", "")
	defer doneB()
	r.tick() // still over: exit
	if len(r.exited) != 1 {
		t.Fatalf("exit: %v", r.events)
	}
	f, err := os.Open(r.ledger)
	if err != nil {
		t.Fatal(err)
	}
	qs, skipped, err := tenancy.QuarantinedServices(f)
	f.Close()
	if err != nil || skipped != 0 {
		t.Fatalf("read quarantine: %v (skipped %d)", err, skipped)
	}
	if len(qs) != 2 || qs[0].Service != "app-kit" || qs[1].Service != "zeta" {
		t.Fatalf("both services quarantined, one row each, sorted: %+v", qs)
	}
	if want := "quarantined: app-kit exceeded budget during ingest — retry with the daemon's `ingest` tool (service \"app-kit\"); a completed explicit ingest lifts it"; qs[0].Attention() != want {
		t.Fatalf("attention line %q", qs[0].Attention())
	}

	// The next start loads the quarantine: the service is named on
	// /statusz and in the governor's snapshot, and not re-queued.
	tenancy.SetQuarantined(nil)
	next := newGovRig(t)
	next.ledger = r.ledger
	next.g.ledger = r.ledger
	if err := next.g.loadQuarantine(); err != nil {
		t.Fatal(err)
	}
	if !tenancy.IsQuarantined("app-kit") || !tenancy.IsQuarantined("zeta") {
		t.Fatalf("the next start carries the quarantine")
	}
	st := statuszQuarantined()
	if len(st) != 2 || !strings.Contains(st[0], "app-kit exceeded budget during ingest") {
		t.Fatalf("/statusz names the quarantine: %v", st)
	}
	if snap := next.g.snapshot(); len(snap.Quarantined) != 2 {
		t.Fatalf("the snapshot (resources, primer) names it: %+v", snap)
	}

	// An explicit ingest clears it, in memory and in the ledger.
	tenancy.ClearQuarantine("app-kit", "ingest tool")
	if tenancy.IsQuarantined("app-kit") {
		t.Fatalf("an explicit ingest clears the quarantine")
	}
	f, _ = os.Open(r.ledger)
	qs, _, _ = tenancy.QuarantinedServices(f)
	f.Close()
	if len(qs) != 1 || qs[0].Service != "zeta" {
		t.Fatalf("the clear row is in the ledger: %+v", qs)
	}
}

func readGovLedgerRows(t *testing.T, path string) []map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("ledger line %q: %v", line, err)
		}
		out = append(out, m)
	}
	return out
}
