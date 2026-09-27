package main

// governor.go — the seatbelt wired into serve-http (docs/design/
// responsible-tenant.md §2, §2.1; RT-34, RT-35, RT-155, RT-161, RT-172,
// RT-173). Every 2 s the governor samples the host (footprint, child tree,
// pressure, swap, volumes, runtime/metrics), steps the ladder and applies
// the rung: shed drops the console and fleet memos, pauses background work
// and scavenges; refuse drops every cache and cancels every in-flight tree
// and fleet ticket, youngest first; exit halts an in-flight ingest first
// (quarantining its service) and, if the next sample is still over — or
// nothing was in flight — kills the child tree, captures a heap profile,
// writes the exit row and exits 70 at once, with no drain. INC-23: the
// daemon reached ~30 GB under a 3 GiB soft limit, because a soft limit is
// not a bound. This is the bound, whatever the cause.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// namedDrop is one cache the ladder can drop.
type namedDrop struct {
	name string
	drop func()
}

// ladderActions are what the rungs act on. dropShed runs at shed and above
// (console and fleet memos); dropRefuse at refuse and above (every other
// cache: the graph, navigation and symbol caches).
type ladderActions struct {
	dropShed        []namedDrop
	dropRefuse      []namedDrop
	pauseBackground func(paused bool)
}

// governorConfig is newGovernor's input; every OS effect is injectable.
type governorConfig struct {
	budget       tenancy.Budget
	notes        []tenancy.Note
	host         tenancy.Host
	now          func() time.Time
	inflight     *tenancy.InFlight
	ledger       string // lifecycle.jsonl for this daemon's id
	pid          int
	version      string
	logf         func(string)
	actions      ladderActions
	freeOSMemory func()
	memRead      func() scavengeMem // RT-45: the scavenge line's readings; nil = readScavengeMem
	killChildren func() int
	captureDiag  func(trigger string) string // returns the capture's path
	exit         func(code int)
}

// governor owns the ladder for one daemon.
type governor struct {
	governorConfig
	rt *tenancy.RuntimeReader

	mu        sync.Mutex // guards everything below and serializes ticks
	st        tenancy.LadderState
	sample    tenancy.Sample
	haveSmp   bool
	lastTrans tenancy.Transition
	lastFree  time.Time
	halted    bool // an ingest was halted at the exit mark; the next exit sample exits
	exited    bool
	cancelled []string              // the last refuse cancellation, in order
	abandoned int64                 // calls cancelled as abandoned-by-governor, process lifetime
	lastDiag  time.Time             // the last refuse-entry diagnostic capture
	disk      tenancy.DiskWriteRing // RT-162: bytes written per minute, 24 h
	// childWritten is the children's last known bytes-written total: a
	// sample whose child reading is unknown holds it rather than reading as
	// a counter reset (which the ring would count as all-new writes).
	childWritten int64
}

// freeEvery bounds debug.FreeOSMemory while shed (§2.1).
const freeEvery = 30 * time.Second

// Diagnostic capture bounds (§13.4): at most one refuse-entry capture per
// diagEvery, and the capture directory keeps at most diagKeep captures and
// diagKeepBytes, oldest pruned first.
const (
	diagEvery     = 10 * time.Minute
	diagKeep      = 10
	diagKeepBytes = 200 * tenancy.MiB
)

// exitOverBudgetCode is the exit rung's code (EX_SOFTWARE): launchd
// relaunches it and the breaker counts it (§2.3).
const exitOverBudgetCode = 70

func newGovernor(cfg governorConfig) *governor {
	if cfg.now == nil {
		cfg.now = time.Now
	}
	if cfg.inflight == nil {
		cfg.inflight = tenancy.DefaultInFlight
	}
	if cfg.logf == nil {
		cfg.logf = func(string) {}
	}
	if cfg.memRead == nil {
		cfg.memRead = readScavengeMem
	}
	return &governor{governorConfig: cfg, rt: tenancy.NewRuntimeReader()}
}

// tick takes one sample, steps the ladder and applies the rung.
func (g *governor) tick(at time.Time, late time.Duration) tenancy.Transition {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.exited {
		return tenancy.Transition{}
	}
	tenancy.SampleHost(g.host, g.rt, at, late, &g.sample)
	g.haveSmp = true
	// RT-162 (§12.4): self and the sampled children.
	if g.sample.ChildDiskWritten >= 0 {
		g.childWritten = g.sample.ChildDiskWritten
	}
	if g.sample.DiskWritten >= 0 {
		g.disk.Add(g.sample.At, g.sample.DiskWritten+g.childWritten, g.inflight.ExplicitIngestInFlight())
	}
	tr := tenancy.StepLadder(&g.st, &g.sample, &g.budget)
	if tr.Changed {
		g.lastTrans = tr
		g.applyLadderLevel(tr)
	}
	if g.st.Level == tenancy.LevelExit {
		g.exitOverBudget(tr)
		return tr
	}
	// While shed or refusing, memory freed by finished work goes back to
	// the OS at most once per freeEvery, so the ladder can step down.
	if g.st.Level >= tenancy.LevelShed {
		g.scavenge(at, false, g.st.Level.String()+", every "+freeEvery.String())
	}
	return tr
}

// applyLadderLevel publishes the new level (producers read it before their
// next unit) and runs the entered rung's actions (§2.1).
func (g *governor) applyLadderLevel(tr tenancy.Transition) {
	tenancy.PublishLevel(tr.To)
	if tr.From == tr.To {
		g.logf(fmt.Sprintf("tenancy: %s", tr.Reason))
		return
	}
	g.logf(fmt.Sprintf("tenancy: %s → %s: %s", tr.From, tr.To, tr.Reason))
	switch {
	case tr.To > tr.From:
		if tr.From < tenancy.LevelShed {
			g.drop(g.actions.dropShed)
			if g.actions.pauseBackground != nil {
				g.actions.pauseBackground(true)
			}
		}
		if tr.To >= tenancy.LevelRefuse && tr.From < tenancy.LevelRefuse {
			if tr.From >= tenancy.LevelShed {
				g.drop(g.actions.dropShed) // rebuilt while shed
			}
			g.drop(g.actions.dropRefuse)
			g.cancelHeavy()
			if tr.To == tenancy.LevelRefuse && g.captureDiag != nil && (g.lastDiag.IsZero() || tr.At.Sub(g.lastDiag) >= diagEvery) {
				g.lastDiag = tr.At
				if p := g.captureDiag("refuse"); p != "" {
					g.logf("tenancy: diagnostics captured → " + p)
				}
			}
		}
		g.scavenge(tr.At, tr.To >= tenancy.LevelRefuse, tr.To.String()+" entry")
	case tr.To == tenancy.LevelNormal:
		g.halted = false
		if g.actions.pauseBackground != nil {
			g.actions.pauseBackground(false)
		}
	default: // a step down that stays at shed or above
		g.halted = false
	}
}

func (g *governor) drop(ds []namedDrop) {
	for _, d := range ds {
		d.drop()
	}
}

// scavenge returns memory to the OS: once on entry, then at most once per
// freeEvery (force: a refuse or exit entry always scavenges). RT-45: each
// pass logs the tick's scavenge line — total−released and phys_footprint
// before and after — labelled with the event that caused it.
func (g *governor) scavenge(at time.Time, force bool, event string) {
	if g.freeOSMemory == nil {
		return
	}
	if !force && !g.lastFree.IsZero() && at.Sub(g.lastFree) < freeEvery {
		return
	}
	g.lastFree = at
	before := g.memRead()
	g.freeOSMemory()
	g.logf(scavengeLine(event, before, g.memRead()))
}

// cancelHeavy cancels every in-flight tree and fleet ticket, youngest
// first, as abandoned-by-governor (RT-172).
func (g *governor) cancelHeavy() {
	got := g.inflight.Cancel(tenancy.ErrAbandonedByGovernor, tenancy.ClassTree, tenancy.ClassFleet)
	g.abandoned += int64(len(got))
	g.cancelled = g.cancelled[:0]
	for _, e := range got {
		g.cancelled = append(g.cancelled, e.Tool)
	}
	if len(got) > 0 {
		g.logf(fmt.Sprintf("tenancy: refuse cancelled %d in-flight tree/fleet call(s), youngest first: %s", len(got), strings.Join(g.cancelled, ", ")))
	}
}

// lastCancelled is the last refuse cancellation's tools, in order.
func (g *governor) lastCancelled() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.cancelled...)
}

// exitOverBudget is the exit rung (RT-173, RT-35). First, if an ingest is
// in flight, every producer is halted (the level is published), the ingest
// cancelled, its children killed, its service quarantined and memory
// returned — and the next sample decides. Otherwise, or when the next
// sample is still over: kill the child tree, capture the heap, write the
// exit row with the reason and the top in-flight work, and exit 70 at
// once. No drain: in-flight requests are not waited for.
func (g *governor) exitOverBudget(tr tenancy.Transition) {
	own := g.sample.Footprint
	if !g.halted {
		ingests := g.inflightOf(tenancy.ClassIngest)
		if len(ingests) > 0 {
			g.halted = true
			tenancy.PublishLevel(tenancy.LevelExit)
			g.inflight.Cancel(tenancy.ErrHaltedOverBudget, tenancy.ClassIngest)
			if g.killChildren != nil {
				g.killChildren()
			}
			names := g.quarantine(ingests, own)
			g.drop(g.actions.dropShed)
			g.drop(g.actions.dropRefuse)
			g.scavenge(g.sample.At, true, "exit-mark halt")
			var tools []string
			for _, e := range ingests {
				tools = append(tools, e.Tool+"("+firstNonEmpty(e.Service, "-")+")")
			}
			quarantined := "none quarantined"
			if len(names) > 0 {
				quarantined = "quarantined " + strings.Join(names, ", ")
			}
			g.logf(fmt.Sprintf("tenancy: exit mark (%s): halted in-flight ingest-class work %s; %s; the next sample decides", tr.Reason, strings.Join(tools, ", "), quarantined))
			return
		}
	}
	// Exiting: anything still ingesting was in flight at an over-budget
	// exit and is quarantined too.
	g.quarantine(g.inflightOf(tenancy.ClassIngest), own)
	if g.killChildren != nil {
		g.killChildren()
	}
	diag := ""
	if g.captureDiag != nil {
		diag = g.captureDiag("exit")
	}
	detail := tr.Reason
	if detail == "" || !tr.Changed {
		detail = fmt.Sprintf("own footprint %s ≥ exit mark %s after halting the in-flight ingest", tenancy.FormatBytes(own), tenancy.FormatBytes(g.budget.Exit))
	}
	g.appendRow(overBudgetExitRow{
		SchemaVersion: 1, FathomVersion: g.version, Row: "exit", At: g.sample.At.UTC().Format(time.RFC3339Nano),
		PID: g.pid, Code: exitOverBudgetCode, Reason: "over_budget", Detail: detail,
		Footprint: own, Children: g.sample.Children, Budget: g.budget.Budget, ExitMark: g.budget.Exit,
		TopInFlight: g.topInFlight(8), Diag: diag,
	})
	g.logf(fmt.Sprintf("tenancy: exiting %d over budget: %s (diag %s) — fathom resources", exitOverBudgetCode, detail, firstNonEmpty(diag, "none")))
	g.exited = true
	g.exit(exitOverBudgetCode)
}

// overBudgetExitRow is the lifecycle ledger's exit row for the exit rung
// (§2.3's `exit` row, reason over_budget).
type overBudgetExitRow struct {
	SchemaVersion int      `json:"schema_version"`
	FathomVersion string   `json:"fathom_version,omitempty"`
	Row           string   `json:"row"`
	At            string   `json:"at"`
	PID           int      `json:"pid"`
	Code          int      `json:"code"`
	Reason        string   `json:"reason"`
	Detail        string   `json:"detail"`
	Footprint     int64    `json:"footprint"`
	Children      int64    `json:"children"`
	Budget        int64    `json:"budget"`
	ExitMark      int64    `json:"exit_mark"`
	TopInFlight   []string `json:"top_in_flight"`
	Diag          string   `json:"diag,omitempty"`
}

func (g *governor) inflightOf(c tenancy.Class) []tenancy.InFlightEntry {
	var out []tenancy.InFlightEntry
	for _, e := range g.inflight.Snapshot() {
		if e.Class == c {
			out = append(out, e)
		}
	}
	return out
}

// quarantine records each ingest's service (once) in the ledger and the
// process set; it returns the services named.
func (g *governor) quarantine(ingests []tenancy.InFlightEntry, footprint int64) []string {
	var names []string
	seen := map[string]bool{}
	for _, e := range ingests {
		if e.Service == "" || seen[e.Service] || !quarantinesOnHalt(e.Tool) {
			continue
		}
		seen[e.Service] = true
		names = append(names, e.Service)
		q := tenancy.Quarantine{Service: e.Service, Since: g.sample.At, Footprint: footprint, Reason: tenancy.QuarantineReasonIngest}
		if tenancy.AddQuarantined(q) {
			g.appendRow(tenancy.NewQuarantineRow(e.Service, footprint, g.pid, g.sample.At, g.version))
			g.logf("tenancy: " + q.Attention())
		}
	}
	sort.Strings(names)
	return names
}

// quarantinesOnHalt reports whether halting ingest-class work named tool
// quarantines its service: an ingest does (the MCP ingest tool, pkg/client's
// BeginIngest, onboard_service); other ingest-class work — compile_layer —
// is halted but is not an ingest, so its service keeps auto-refreshing.
func quarantinesOnHalt(tool string) bool {
	return tool == "ingest" || tool == "onboard_service"
}

// topInFlight names the oldest n in-flight entries: tool(class, service, age).
func (g *governor) topInFlight(n int) []string {
	snap := g.inflight.Snapshot()
	out := []string{}
	for i, e := range snap {
		if i == n {
			break
		}
		out = append(out, fmt.Sprintf("%s(%s, %s, %s)", e.Tool, e.Class, firstNonEmpty(e.Service, "-"), g.sample.At.Sub(e.Started).Round(time.Second)))
	}
	return out
}

// appendRow appends one JSON row to the daemon's lifecycle ledger.
func (g *governor) appendRow(row any) {
	if g.ledger == "" {
		return
	}
	b, err := json.Marshal(row)
	if err == nil {
		err = os.MkdirAll(filepath.Dir(g.ledger), 0o700)
	}
	if err == nil {
		var f *os.File
		if f, err = os.OpenFile(g.ledger, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
			_, err = f.Write(append(b, '\n'))
			if cerr := f.Close(); err == nil {
				err = cerr
			}
		}
	}
	if err != nil {
		g.logf(fmt.Sprintf("tenancy: lifecycle ledger %s not written: %v", g.ledger, err))
	}
}

// loadQuarantine reads the ledger's quarantine at start and registers the
// ledger as where an explicit ingest records a clear. A missing ledger is
// no quarantine; an unreadable one is an error (I38).
func (g *governor) loadQuarantine() error {
	tenancy.SetQuarantineClearer(func(service, by string) {
		g.appendRow(tenancy.NewQuarantineClearRow(service, by, g.now(), g.version))
		g.logf(fmt.Sprintf("tenancy: quarantine of %s cleared by %s", service, by))
	})
	if g.ledger == "" {
		return nil
	}
	f, err := os.Open(g.ledger)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	qs, skipped, err := tenancy.QuarantinedServices(f)
	if err != nil {
		return err
	}
	tenancy.SetQuarantined(qs)
	for _, q := range qs {
		g.logf("tenancy: " + q.Attention())
	}
	if skipped > 0 {
		g.logf(fmt.Sprintf("tenancy: %d unreadable lifecycle ledger line(s) skipped in %s", skipped, g.ledger))
	}
	return nil
}

// governorSnapshot is the projection /metrics, /statusz and (later)
// `resources` and the primer render.
type governorSnapshot struct {
	Level       string `json:"level"`
	Since       string `json:"since,omitempty"`
	LadderOff   bool   `json:"ladder_off,omitempty"`
	Budget      int64  `json:"budget"`
	Shed        int64  `json:"shed_mark"`
	Refuse      int64  `json:"refuse_mark"`
	Exit        int64  `json:"exit_mark"`
	ExitEnabled bool   `json:"exit_enabled"`
	Footprint   int64  `json:"footprint"`
	Children    int64  `json:"children"`
	ChildCount  int    `json:"child_count"`
	Pressure    string `json:"pressure"`
	SwapUsed    int64  `json:"swap_used"`
	DiskWritten int64  `json:"disk_written"`
	// ChildDiskWritten is the sampled children's bytes written (a floor:
	// a child's last two seconds are not seen); -1 = unknown.
	ChildDiskWritten int64 `json:"children_disk_written"`
	// DiskWrites is RT-162's judgment: bytes written in the last 24 h
	// outside explicit ingest against 2 GiB/day (tenancy.DiskWriteVerdict).
	DiskWrites     string `json:"disk_writes,omitempty"`
	DiskWritesOver bool   `json:"disk_writes_over,omitempty"`
	diskOutside    int64
	diskIngest     int64
	diskKnown      bool
	diskCovered    time.Duration
	GCCPUFraction  float64  `json:"gc_cpu_fraction"`
	Unknowns       []string `json:"unknowns,omitempty"`
	LastTransition string   `json:"last_transition,omitempty"`
	Quarantined    []string `json:"quarantined,omitempty"`
	Abandoned      int64    `json:"abandoned_by_governor"`
}

func (g *governor) snapshot() governorSnapshot {
	g.mu.Lock()
	defer g.mu.Unlock()
	s := governorSnapshot{
		Level: g.st.Level.String(), LadderOff: g.st.Off,
		Budget: g.budget.Budget, Shed: g.budget.Shed, Refuse: g.budget.Refuse, Exit: g.budget.Exit, ExitEnabled: g.budget.ExitEnabled,
		Footprint: tenancy.Unknown, Children: tenancy.Unknown, SwapUsed: tenancy.Unknown, DiskWritten: tenancy.Unknown, ChildDiskWritten: tenancy.Unknown,
		Pressure: tenancy.PressureUnknown.String(), Abandoned: g.abandoned,
	}
	if !g.st.Since.IsZero() {
		s.Since = g.st.Since.UTC().Format(time.RFC3339)
	}
	if g.haveSmp {
		s.Footprint, s.Children, s.ChildCount = g.sample.Footprint, g.sample.Children, g.sample.ChildCount
		s.Pressure, s.SwapUsed, s.DiskWritten = g.sample.Pressure.String(), g.sample.SwapUsed, g.sample.DiskWritten
		if g.sample.ChildDiskWritten >= 0 || g.childWritten > 0 {
			s.ChildDiskWritten = g.childWritten
		}
		s.GCCPUFraction = g.sample.GCCPUFraction
		s.Unknowns = g.sample.Unknowns()
		dv := tenancy.DiskWriteVerdict(&g.disk, g.sample.At, tenancy.DiskWriteBudgetPerDay)
		s.DiskWrites, s.DiskWritesOver = dv.Line, dv.Over
		s.diskOutside, s.diskIngest, s.diskKnown, s.diskCovered = dv.Outside, dv.Ingest, !dv.Unknown, dv.Covered
	}
	if g.lastTrans.Changed {
		s.LastTransition = fmt.Sprintf("%s %s → %s: %s", g.lastTrans.At.UTC().Format(time.RFC3339), g.lastTrans.From, g.lastTrans.To, g.lastTrans.Reason)
	}
	for _, q := range tenancy.QuarantinedNow() {
		s.Quarantined = append(s.Quarantined, q.Attention())
	}
	return s
}

// run samples every tenancy.SampleEvery until ctx ends, recording each
// sample's lateness against its schedule (RT-171).
func (g *governor) run(ctx context.Context) {
	t := time.NewTicker(tenancy.SampleEvery)
	defer t.Stop()
	prevStart := g.now()
	prevEnd := prevStart
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		at := g.now()
		g.tick(at, sampleLateness(prevStart, prevEnd, at))
		prevStart, prevEnd = at, g.now()
	}
}

// sampleLateness is how far a sample taken at `at` is behind schedule: the
// later of the previous tick's start plus the period and the previous
// tick's end. A stall of the host (the process not scheduled) shows; the
// governor's own slow tick — FreeOSMemory, a heap capture, cache drops,
// after which the ticker's buffered tick is received at once — does not.
func sampleLateness(prevStart, prevEnd, at time.Time) time.Duration {
	due := prevStart.Add(tenancy.SampleEvery)
	if prevEnd.After(due) {
		due = prevEnd
	}
	if late := at.Sub(due); late > 0 {
		return late
	}
	return 0
}

// activeGovernor is the serving daemon's governor (nil in CLI verbs), read
// by /metrics and /statusz.
var activeGovernor atomic.Pointer[governor]

// statuszQuarantined is /statusz's quarantine list (RT-155).
func statuszQuarantined() []string {
	var out []string
	for _, q := range tenancy.QuarantinedNow() {
		out = append(out, q.Attention())
	}
	return out
}

// governorMetricLines renders the governor's /metrics gauges (§13.3).
func governorMetricLines() string {
	g := activeGovernor.Load()
	if g == nil {
		return ""
	}
	s := g.snapshot()
	levels := map[string]int{"normal": 0, "shed": 1, "refuse": 2, "exit": 3}
	var b strings.Builder
	fmt.Fprintf(&b, "fathom_tenancy_level %d\n", levels[s.Level])
	fmt.Fprintf(&b, "fathom_budget_bytes %d\n", s.Budget)
	if s.Footprint >= 0 {
		fmt.Fprintf(&b, "fathom_footprint_bytes %d\n", s.Footprint)
	}
	if s.Children >= 0 {
		fmt.Fprintf(&b, "fathom_children_footprint_bytes %d\n", s.Children)
		fmt.Fprintf(&b, "fathom_children_processes %d\n", s.ChildCount)
	}
	pl := map[string]int{"unknown": 0, "normal": 1, "warn": 2, "critical": 4}
	fmt.Fprintf(&b, "fathom_host_pressure_level %d\n", pl[s.Pressure])
	if s.SwapUsed >= 0 {
		fmt.Fprintf(&b, "fathom_swap_used_bytes %d\n", s.SwapUsed)
	}
	fmt.Fprintf(&b, "fathom_gc_cpu_fraction %g\n", s.GCCPUFraction)
	// RT-162 (§12.4): by subsystem — the daemon's own rusage and its
	// sampled child tree (per-writer attribution, call log vs WAL vs
	// ingest, is not in rusage).
	if s.DiskWritten >= 0 {
		fmt.Fprintf(&b, "fathom_bytes_written_total{subsystem=\"self\"} %d\n", s.DiskWritten)
	}
	if s.ChildDiskWritten >= 0 {
		fmt.Fprintf(&b, "fathom_bytes_written_total{subsystem=\"children\"} %d\n", s.ChildDiskWritten)
	}
	if s.diskKnown { // RT-162: never a zero for an unknown reading (I38)
		fmt.Fprintf(&b, "fathom_bytes_written_24h_outside_ingest %d\n", s.diskOutside)
		fmt.Fprintf(&b, "fathom_bytes_written_24h_explicit_ingest %d\n", s.diskIngest)
		// The ring is in memory until RT-47: the "24h" pair covers only
		// this much (review finding 4).
		fmt.Fprintf(&b, "fathom_bytes_written_window_seconds %d\n", int64(s.diskCovered/time.Second))
		fmt.Fprintf(&b, "fathom_disk_write_budget_bytes %d\n", int64(tenancy.DiskWriteBudgetPerDay))
	}
	fmt.Fprintf(&b, "fathom_quarantined_services %d\n", len(s.Quarantined))
	fmt.Fprintf(&b, "fathom_governor_abandoned_total %d\n", s.Abandoned)
	return b.String()
}

// captureExitDiag writes the exit rung's diagnostics — the heap profile and
// the in-flight list, nothing slower — under dir/<ts>-<trigger>/ (§13.4)
// and returns the directory.
func captureExitDiag(dir string, inflight *tenancy.InFlight, now func() time.Time) func(trigger string) string {
	return func(trigger string) string {
		if dir == "" {
			return ""
		}
		d := filepath.Join(dir, now().UTC().Format("20060102T150405Z")+"-"+trigger)
		if err := os.MkdirAll(d, 0o700); err != nil {
			return ""
		}
		if f, err := os.Create(filepath.Join(d, "heap.pb.gz")); err == nil {
			_ = pprof.Lookup("heap").WriteTo(f, 0)
			_ = f.Close()
		}
		if b, err := json.MarshalIndent(inflight.Snapshot(), "", "  "); err == nil {
			_ = os.WriteFile(filepath.Join(d, "inflight.json"), b, 0o600)
		}
		pruneDiagCaptures(dir, diagKeep, diagKeepBytes)
		return d
	}
}

// pruneDiagCaptures keeps dir's newest captures — at most keep of them and
// at most keepBytes in all, the newest always kept — and removes the rest.
// Capture directories are named <UTC timestamp>-<trigger>, so name order is
// age order. Best-effort: an unreadable entry is left alone.
func pruneDiagCaptures(dir string, keep int, keepBytes int64) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var names []string
	for _, e := range ents {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	var total int64
	for i, name := range names {
		p := filepath.Join(dir, name)
		size := dirSize(p)
		if i == 0 || (i < keep && total+size <= keepBytes) {
			total += size
			continue
		}
		_ = os.RemoveAll(p)
	}
}

func dirSize(dir string) int64 {
	var n int64
	_ = filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, ierr := d.Info(); ierr == nil {
				n += info.Size()
			}
		}
		return nil
	})
	return n
}
