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
	"fmt"
	"os"
	"path/filepath"
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
	// flight is the flight recorder started at shed and stopped back at
	// normal (§13.4); nil records no trace.
	flight *flightRecorder
	exit   func(code int)
	// observe sees each sample and the level after it (nil: none).
	observe func(s *tenancy.Sample, level tenancy.Level)
	// ring is the resource ring (§13.1, RT-47): one record a minute and
	// one per ladder transition; nil writes none.
	ring ringWriter
	// safeMode reports the daemon's mode for the ring and /statusz; nil
	// reads safeModeActive.
	safeMode func() bool
}

// ringWriter is the resource ring as the governor writes it.
type ringWriter interface {
	WriteRingSample(tenancy.RingRecord) error
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
	lastDiag  map[string]time.Time  // the last capture per trigger kind (§13.4 rate)
	crossed   int                   // powers of two of the footprint crossed this process: 1 GiB → 1, 2 → 2, 4 → 3, 8 → 4
	critical  bool                  // the last sample's pressure was critical
	disk      tenancy.DiskWriteRing // RT-162: bytes written per minute, 24 h
	// childWritten is the children's last known bytes-written total: a
	// sample whose child reading is unknown holds it rather than reading as
	// a counter reset (which the ring would count as all-new writes).
	childWritten int64
	// pendingDiag is the captures this tick asked for, taken after g.mu is
	// released: a capture renders the governor's own /metrics lines
	// (g.snapshot takes g.mu) and writes profiles, neither of which may
	// run under the lock (RT-122 hygiene).
	pendingDiag []pendingCapture
	diagMu      sync.Mutex // serializes captures (and their pruning)
	diagPath    string     // the newest capture's directory
	diagAt      time.Time  // and when it was taken
	diagWhy     string     // and its trigger

	ringMinute int64 // unix minute of the last minute record (0: none yet)
	ringErr    bool  // a ring write failed (logged once until one succeeds)
}

// writeRing writes one ring record from the current sample (§13.1).
// Called under g.mu. A failed write is logged once, not every minute.
func (g *governor) writeRing(at time.Time, transition bool) {
	if g.ring == nil {
		return
	}
	rec := g.ringRecord(at, transition)
	if err := g.ring.WriteRingSample(rec); err != nil {
		if !g.ringErr {
			g.ringErr = true
			g.logf(fmt.Sprintf("tenancy: resource ring not written (%v) — `fathom resources` shows the last readable sample", err))
		}
		return
	}
	g.ringErr = false
}

// ringRecord projects the governor's state into a ring record.
func (g *governor) ringRecord(at time.Time, transition bool) tenancy.RingRecord {
	s := &g.sample
	rec := tenancy.RingRecord{
		At: at, Transition: transition, Level: g.st.Level, SafeMode: g.safeMode(), LadderOff: g.st.Off,
		Footprint: s.Footprint, Children: s.Children, GoTotalMinusReleased: s.GoTotalMinusReleased,
		HeapLive: s.HeapLive, GCCPUFraction: s.GCCPUFraction, Pressure: s.Pressure, SwapUsed: s.SwapUsed, SwapTotal: s.SwapTotal,
		StoreFree: s.StoreFree, TmpFree: s.TmpFree, SwapVolumeFree: s.SwapVolumeFree, DiskWritten: s.DiskWritten,
		Abandoned: uint32(min(g.abandoned, int64(^uint32(0)))),
	}
	snap := g.inflight.Snapshot()
	for _, e := range snap {
		if int(e.Class) < len(rec.InFlight) && rec.InFlight[e.Class] < ^uint16(0) {
			rec.InFlight[e.Class]++
		}
	}
	if len(snap) > 0 { // Snapshot is oldest first
		rec.OldestTool, rec.OldestClass, rec.OldestAge = snap[0].Tool, snap[0].Class, at.Sub(snap[0].Started)
	}
	return rec
}

// freeEvery bounds debug.FreeOSMemory while shed (§2.1).
const freeEvery = 30 * time.Second

// Diagnostic capture bounds (§13.4): at most one capture of each trigger
// kind per diagEvery, and the capture directory keeps at most diagKeep captures and
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
	if cfg.safeMode == nil {
		cfg.safeMode = func() bool { _, on := tenancy.SafeModeNow(); return on }
	}
	return &governor{governorConfig: cfg, rt: tenancy.NewRuntimeReader()}
}

// pendingCapture is one threshold capture a tick asked for.
type pendingCapture struct {
	trigger string
	at      time.Time
}

// tick takes one sample, steps the ladder and applies the rung. The
// threshold captures the tick asked for run after g.mu is released.
func (g *governor) tick(at time.Time, late time.Duration) tenancy.Transition {
	g.mu.Lock()
	tr := g.tickLocked(at, late)
	pending := g.pendingDiag
	g.pendingDiag = nil
	g.mu.Unlock()
	g.runCaptures(pending)
	return tr
}

// runCaptures takes the queued captures outside g.mu, then records the
// newest one under it.
func (g *governor) runCaptures(pending []pendingCapture) {
	for _, pc := range pending {
		g.diagMu.Lock()
		p := g.captureDiag(pc.trigger)
		g.diagMu.Unlock()
		if p == "" {
			continue
		}
		g.mu.Lock()
		g.diagPath, g.diagAt, g.diagWhy = p, pc.at, pc.trigger
		g.mu.Unlock()
		g.logf("tenancy: diagnostics captured (" + pc.trigger + ") → " + p)
	}
}

// tickLocked is tick under g.mu.
func (g *governor) tickLocked(at time.Time, late time.Duration) tenancy.Transition {
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
	g.captureCrossings(at)
	tr := tenancy.StepLadder(&g.st, &g.sample, &g.budget)
	if tr.Changed {
		g.lastTrans = tr
		g.applyLadderLevel(tr)
		g.writeRing(at, true)
	}
	if m := at.Unix() / 60; m != g.ringMinute {
		g.ringMinute = m
		g.writeRing(at, false)
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
	if g.observe != nil {
		g.observe(&g.sample, g.st.Level)
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
			if err := g.flight.start(); err != nil {
				g.logf(fmt.Sprintf("tenancy: flight recorder not started (%v) — captures carry no trace.out", err))
			}
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
			if tr.To == tenancy.LevelRefuse {
				g.captureRateLimited("refuse", "refuse", tr.At)
			}
		}
		g.scavenge(tr.At, tr.To >= tenancy.LevelRefuse, tr.To.String()+" entry")
	case tr.To == tenancy.LevelNormal:
		g.halted = false
		g.flight.stop()
		if g.actions.pauseBackground != nil {
			g.actions.pauseBackground(false)
		}
	default: // a step down that stays at shed or above
		g.halted = false
	}
}

// captureCrossings takes the threshold captures a sample can trigger
// (§13.4, RT-122): the first crossing of each power of two of the
// process's own footprint (1, 2, 4, 8 GiB — one capture, named for the
// highest power newly crossed) and entry to pressure critical. Called
// under g.mu.
func (g *governor) captureCrossings(at time.Time) {
	if fp := g.sample.Footprint; fp > 0 {
		p := 0
		for p < 4 && fp > tenancy.GiB<<p {
			p++
		}
		if p > g.crossed {
			g.crossed = p
			g.captureRateLimited("footprint", fmt.Sprintf("footprint-%dGiB", 1<<(p-1)), at)
		}
	}
	critical := g.sample.Pressure == tenancy.PressureCritical
	if critical && !g.critical {
		g.captureRateLimited("pressure", "pressure-critical", at)
	}
	g.critical = critical
}

// captureRateLimited captures trigger unless a capture of the same kind
// ran within diagEvery. Each kind is limited on its own, so a fast climb
// that crosses 1 GiB, then enters refuse a minute later, keeps both
// captures (the refuse one is the one that matters); the exit rung is
// never limited. Called under g.mu: it only queues the capture, which
// tick takes once the lock is released.
func (g *governor) captureRateLimited(kind, trigger string, at time.Time) {
	if g.captureDiag == nil {
		return
	}
	if last, ok := g.lastDiag[kind]; ok && at.Sub(last) < diagEvery {
		return
	}
	if g.lastDiag == nil {
		g.lastDiag = map[string]time.Time{}
	}
	g.lastDiag[kind] = at
	g.pendingDiag = append(g.pendingDiag, pendingCapture{trigger: trigger, at: at})
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
		diag = g.captureDiag(diagTriggerExit)
		if diag != "" {
			g.diagPath, g.diagAt, g.diagWhy = diag, g.sample.At, "exit"
		}
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

// topInFlight names the oldest n in-flight entries: tool(class, service,
// age, since start). The age is a wall-clock difference (Round(0) strips
// the monotonic reading), the clock the ledger and the ring record: a
// monotonic difference stops while the machine sleeps, which made the
// live reading's exit row say 1m15s for an ingest its timeline put at 10
// minutes (F7). The start instant lets the row be checked against both.
func (g *governor) topInFlight(n int) []string {
	snap := g.inflight.Snapshot()
	out := []string{}
	for i, e := range snap {
		if i == n {
			break
		}
		age := g.sample.At.Round(0).Sub(e.Started.Round(0)).Round(time.Second)
		out = append(out, fmt.Sprintf("%s(%s, %s, %s since %s)", e.Tool, e.Class, firstNonEmpty(e.Service, "-"), age, e.Started.UTC().Format(time.RFC3339)))
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
		err = appendLedgerBytes(g.ledger, b) // rotates at 1 MiB, one generation kept (§2.3)
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
	f, done, err := openLedgerGenerations(g.ledger)
	if err != nil {
		return err
	}
	defer done()
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
	// The lifecycle modes (§2.3, §2.1): safe mode's sentence, the warm-up
	// window's end, store distress's reason; empty when not in effect.
	SafeMode      string `json:"safe_mode,omitempty"`
	WarmupUntil   string `json:"warmup_until,omitempty"`
	StoreDistress string `json:"store_distress,omitempty"`

	// The projection's remaining fields (tenancy_view.go, RT-52): filled
	// by tenancyStatus (live) and tenancyFromRing (the ring), not by
	// snapshot (the /metrics read).
	Source          string         `json:"source,omitempty"` // live | ring
	SampleAt        string         `json:"sample_at,omitempty"`
	SwapTotal       int64          `json:"swap_total,omitempty"`
	Mode            string         `json:"mode,omitempty"` // normal | safe
	InFlightByClass map[string]int `json:"in_flight_by_class,omitempty"`
	InFlight        []inflightView `json:"in_flight,omitempty"` // live only
	Oldest          *inflightView  `json:"oldest_in_flight,omitempty"`
	Refusals        string         `json:"refusals,omitempty"`
	LastDiag        *diagView      `json:"last_diag,omitempty"`
}

func (g *governor) snapshot() governorSnapshot {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.snapshotLocked()
}

// snapshotLocked is snapshot under a held g.mu, so a caller that adds
// fields from the same sample (tenancyStatus) reads one sample.
func (g *governor) snapshotLocked() governorSnapshot {
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
	now := g.now()
	if sm, on := tenancy.SafeModeNow(); on {
		s.SafeMode = sm.Message()
	}
	if tenancy.InWarmup(now) {
		start, d := tenancy.Warmup()
		s.WarmupUntil = start.Add(d).UTC().Format(time.RFC3339)
	}
	if why, on := tenancy.StoreDistressNow(now); on {
		s.StoreDistress = why
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
	fmt.Fprintf(&b, "fathom_safe_mode %d\n", boolGauge(s.SafeMode != ""))
	fmt.Fprintf(&b, "fathom_warmup %d\n", boolGauge(s.WarmupUntil != ""))
	fmt.Fprintf(&b, "fathom_store_distress %d\n", boolGauge(s.StoreDistress != ""))
	return b.String()
}

func boolGauge(b bool) int {
	if b {
		return 1
	}
	return 0
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
