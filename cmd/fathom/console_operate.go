package main

// console_operate.go — the Operate page at /operate (vision 35 §3 System,
// design §3 "Operate"; OC-44, SH-9). The daemon (version, commit, uptime,
// memory, goroutines); traffic per day and per surface with p50, p95 and
// error rate from the resolved call log, a day with no rows drawn as
// absent (hatched), never zero; the tools labelled by caller class and the
// zero-call list; the flywheel's loops; time-to-remedy from the fleet
// report snapshots; a project's capabilities; `config --explain`; and the
// harness matrix.
//
// Every panel is a verb's own computation and names that verb (P10):
// `metrics load` (console.LoadView: calls per day per surface, tools by
// caller class), `metrics traffic` (trafficResultFrom), `instrument
// --adoption` (instrumentAdoption: the zero-call list), `fleet remedy`
// (buildRemedyReport), `config --explain` (configJSONValue), `flywheel`
// (runFlywheelChecks), `capabilities` (capabilityJSON) and `harness matrix`
// (harness.AdoptionRows). An unreadable call log is an error on every panel
// that reads it — never an idle daemon. The page reads the call log once
// per projection and keeps the projection for the model's TTL: the log is
// tens of megabytes and grows with every call.

import (
	"fmt"
	"html"
	"html/template"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/app-kit/appkit/ui/form"
	"github.com/geoffmilleraz/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
	"github.com/geoffmilleraz/fathom/internal/buildinfo"
	"github.com/geoffmilleraz/fathom/internal/capability"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/metrics"
)

// The verbs whose --json the page carries (parity: TestConsolePagesRenderTheirVerbs).
const (
	operateLoadVerb    = "fathom metrics load --json"
	operateTrafficVerb = "fathom metrics traffic --json"
	operateCallerVerb  = "fathom metrics traffic --by caller --json"
	operateAdoptVerb   = "fathom instrument --adoption --json"
	operateRemedyVerb  = "fathom fleet remedy --json"
	operateConfigVerb  = "fathom config --explain --json"
	operateHarnessVerb = "fathom harness matrix --json"
)

func init() {
	declarePage(consolePage{
		ID: "operate", Route: "/operate", Href: "/operate", Group: "System", Order: -10, Nav: true,
		Title:    "Operate",
		Question: "Is the daemon healthy, what does it answer, for whom, and are the loops that improve fathom turning?",
		Verbs:    []string{operateLoadVerb, operateTrafficVerb, operateCallerVerb, operateAdoptVerb, operateRemedyVerb, operateConfigVerb, operateHarnessVerb, operateResourcesVerb},
		Shows:    []console.MetricID{console.MetricServices, console.MetricCalls, console.MetricAgentTools},
		Build:    buildOperatePage,
	})
}

// operateProjection is everything the page reads, computed once per TTL.
type operateProjection struct {
	View         console.OperateModel
	Traffic      trafficResult
	Callers      trafficResult
	Adoption     AdoptionReport
	Harnesses    []harness.AdoptionRow
	Remedy       remedyReport
	Config       map[string]any
	Flywheel     []flywheelCheck
	Resources    resourcesReport
	CallLog      console.ResolvedCallLog
	GeneratedAt  time.Time
	computedFrom string
}

// operatePageModel is the page's JSON projection.
type operatePageModel struct {
	Operate      console.OperateModel `json:"operate"`
	Flywheel     []flywheelCheck      `json:"flywheel"`
	Capabilities *capabilityModel     `json:"capabilities,omitempty"`
	RemedyNow    remedyNow            `json:"remedy_now"`
	// Tenancy is /statusz's tenancy block (tenancyStatus, RT-52), read at
	// render and never cached: nil where no governor runs in this process.
	Tenancy *governorSnapshot `json:"tenancy,omitempty"`
}

// remedyNow is time-to-remedy's headline with its truth state: the median
// over closed gaps, stale when the latest snapshot is over a week old, and
// a lower bound when any closed gap opened on or before the first snapshot.
// A value carries one state, so a stale lower bound is state stale (its
// reason naming both) with LowerBound set: "≥" is kept everywhere (design
// §3 Impact), stale or not.
type remedyNow struct {
	MedianHours console.Value[int] `json:"median_hours"`
	LowerBound  bool               `json:"lower_bound,omitempty"`
	Latest      string             `json:"latest_snapshot,omitempty"`
}

var operateCache struct {
	sync.Mutex
	key string
	at  time.Time
	p   operateProjection
	// flight is the build in progress (nil when none): the projection is
	// built outside the lock, once, and every reader past the TTL waits
	// for that one build instead of queueing on the lock (RT-121, RT-113's
	// rule).
	flight *operateFlight
}

type operateFlight struct {
	key  string
	done chan struct{}
	p    operateProjection
	ok   bool
}

// operateProjectionBuilder builds the projection (tests hold it).
var operateProjectionBuilder = buildOperateProjection

// operateNow reads (or reuses, within the model's TTL) the projection.
func operateNow(d *consoleDeps, m observatoryModel, now time.Time) operateProjection {
	resolved := d.sources().CallLog
	key := d.selfURL + "|" + resolved.Path
	operateCache.Lock()
	if operateCache.key == key && !operateCache.at.IsZero() && now.Sub(operateCache.at) < obsCacheTTL && !now.Before(operateCache.at) {
		p := operateCache.p
		operateCache.Unlock()
		return p
	}
	if fl := operateCache.flight; fl != nil && fl.key == key {
		operateCache.Unlock()
		<-fl.done
		if fl.ok {
			return fl.p
		}
		// The flight failed: build this reader's own.
		return operateProjectionBuilder(d, m, resolved, now)
	}
	fl := &operateFlight{key: key, done: make(chan struct{})}
	operateCache.flight = fl
	operateCache.Unlock()
	defer func() {
		operateCache.Lock()
		if fl.ok {
			operateCache.key, operateCache.at, operateCache.p = key, now, fl.p
		}
		if operateCache.flight == fl {
			operateCache.flight = nil
		}
		operateCache.Unlock()
		close(fl.done)
	}()
	fl.p = operateProjectionBuilder(d, m, resolved, now)
	fl.ok = true
	return fl.p
}

func resetOperateCache() {
	operateCache.Lock()
	operateCache.key, operateCache.at = "", time.Time{}
	operateCache.Unlock()
	resetOperateResources()
}

// daemonFacts is what this process knows about itself.
func daemonFacts(d *consoleDeps) console.DaemonFacts {
	bi := statuszBuildInfo()
	f := console.DaemonFacts{Version: buildinfo.Version(), Commit: bi.BinaryCommit, Dirty: bi.BinaryDirty,
		Addr: strings.TrimPrefix(strings.TrimPrefix(d.selfURL, "http://"), "https://"), StartedAt: d.startTime,
		Goroutines: runtime.NumGoroutine()}
	if n, ok := processPhysFootprintBytes(); ok && n > 0 {
		f.RSSBytes = uint64(n)
	} else if n, ok := processResidentMemoryBytes(); ok && n > 0 {
		f.RSSBytes = uint64(n)
	} else {
		f.MemNote = "this platform reports no resident set for the process"
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	f.HeapBytes = ms.HeapInuse
	return f
}

func buildOperateProjection(d *consoleDeps, m observatoryModel, resolved console.ResolvedCallLog, now time.Time) operateProjection {
	p := operateProjection{CallLog: resolved, GeneratedAt: now}
	// A tail (RT-113): all-time figures from its fold, the 14-day load
	// and the week's traffic from its 35-day ring (RT-112).
	var snap callLogSnapshot
	var readErr error
	if resolved.Path != "" {
		snap, readErr = consoleCallLogTail.read(resolved.Path)
	}
	fold, entries := snap.fold, snap.ring
	if fold == nil {
		fold = newCallLogFold()
	}
	facts := fold.facts(resolved, readErr)
	if readErr == nil {
		facts.ConsoleReads = snap.consoleReads
	}
	reason, isErr, _ := callLogState(facts)
	in := operateLoadInputs(entries, resolved, reason, isErr, now)
	in.Daemon = daemonFacts(d)
	p.View = console.OperateView(in)
	week := now.Add(-7 * 24 * time.Hour)
	p.Traffic = trafficResultFrom(entries, reason, isErr, "", metrics.TrafficGroupBySurface, week, now)
	p.Callers = trafficResultFrom(entries, reason, isErr, "", metrics.TrafficGroupByCaller, week, now)
	p.Harnesses = harness.AdoptionRows(d.selfURL, now)
	p.Resources = operateResourcesCached(d.selfURL, now) // RT-121: kept, refreshed off the request path
	pre := &adoptionPreread{fold: fold, skipped: snap.skipped, err: readErr}
	if resolved.Path == "" {
		pre = nil
	}
	p.Adoption, _ = instrumentAdoption(adoptionQuery{Root: ".", BaseURL: d.selfURL, preread: pre}, ioDiscard{})
	p.Remedy = buildRemedyReport(fleetSnapshotDir())
	loaded, lerr := config.Load(config.DefaultUserPath(), config.DefaultProjectPath())
	if lerr == nil {
		if profile, prov, perr := config.ResolveProfile(loaded.Config, loaded.Provenance, ""); perr == nil {
			p.Config = configJSONValue(loaded, profile, prov, loaded.Config.DefaultProfile, true)
		} else {
			p.Config = map[string]any{"error": perr.Error()}
		}
	} else {
		p.Config = map[string]any{"error": lerr.Error()}
	}
	// The flywheel over the model's impact fold and this projection's read
	// of the log (flywheelChecksFrom): the verb reads both itself.
	var logErr error
	if facts.ReadErr != "" {
		logErr = fmt.Errorf("%s", facts.ReadErr)
	} else if facts.Missing {
		logErr = os.ErrNotExist
	}
	p.Flywheel = flywheelChecksFold(m.Impact, fold, logErr, now)
	return p
}

// operateLoadInputs is the load's inputs over one read of the call log —
// what `fathom metrics load` and the page both project (console.LoadView).
func operateLoadInputs(entries []adoptionCallEntry, resolved console.ResolvedCallLog, reason string, isErr bool, now time.Time) console.OperateInputs {
	return console.OperateInputs{Calls: callRowsOf(entries), CallLog: resolved,
		CallLogReason: reason, CallLogError: isErr, Now: now, Days: 14, WindowDays: 7, TopTools: 10}
}

// ioDiscard swallows a verb computation's warnings on the page path (the
// page states read failures in its own truth states).
type ioDiscard struct{}

func (ioDiscard) Write(b []byte) (int, error) { return len(b), nil }

// operateCapabilities is the capability report for one registered project:
// ?svc=, else the project whose root holds the daemon's working directory,
// else the first by name.
func operateCapabilities(cr *consoleRequest, m observatoryModel) (*capabilityModel, string, []string) {
	var names []string
	roots := map[string]string{}
	for _, s := range m.Services {
		names = append(names, s.Service)
		roots[s.Service] = s.Path
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil, "", nil
	}
	svc := cr.r.URL.Query().Get("svc")
	if _, ok := roots[svc]; !ok {
		svc = ""
		if wd, err := os.Getwd(); err == nil {
			for _, n := range names {
				if r := roots[n]; r != "" && (sameRepoPath(r, wd) || strings.HasPrefix(wd, r+string(filepath.Separator))) {
					svc = n
					break
				}
			}
		}
		if svc == "" {
			svc = names[0]
		}
	}
	root := roots[svc]
	reg := capability.Default()
	probe := buildCapabilityProbe(cr.deps.selfURL, root, svc, cr.now)
	res, _ := loadCapabilityConfig(reg, root)
	model := capabilityJSON(probe, reg.DetectAll(probe, res))
	return &model, svc, names
}

// remedyHeadline is time-to-remedy's headline value.
func remedyHeadline(r remedyReport, now time.Time) remedyNow {
	out := remedyNow{Latest: r.Latest}
	const source = "the fleet report snapshots (~/.fathom/fleet/report-<date>.json)"
	switch {
	case r.Snapshots == 0:
		out.MedianHours = console.Absent[int](r.Note).WithSource(source)
		return out
	case r.MedianHours == nil:
		out.MedianHours = console.Absent[int](r.Note).WithSource(source)
		return out
	}
	closed, open := 0, 0
	if r.Closed != nil {
		closed = *r.Closed
	}
	if r.Open != nil {
		open = *r.Open
	}
	denom := fmt.Sprintf("median over %s; %d still open", plural(closed, "closed gap"), open)
	const lowerBound = "a lower bound — a gap already open in the first snapshot opened on or before it, so its interval (and the median over it) is at least this"
	v := console.Measured(int(math.Round(*r.MedianHours)), source).WithDenominator(denom)
	out.LowerBound = r.MedianLowerBound
	if out.LowerBound {
		v = console.Partial(v.V, lowerBound).WithSource(source).WithDenominator(denom)
	}
	if latest, err := time.Parse(time.RFC3339, r.Latest); err == nil {
		v = v.WithAsOf(latest)
		if age := now.Sub(latest); age > 7*24*time.Hour {
			reason := fmt.Sprintf("the latest snapshot is %d days old — fathom fleet report records today's", int(age.Hours()/24))
			if out.LowerBound {
				reason += "; and " + lowerBound
			}
			v = console.Stale(v.V, reason, latest).WithSource(source).WithDenominator(denom)
		}
	}
	out.MedianHours = v
	return out
}

func buildOperatePage(cr *consoleRequest) (consoleOut, error) {
	m := cr.model()
	p := operateNow(cr.deps, m, cr.now)
	caps, svc, names := operateCapabilities(cr, m)
	rem := remedyHeadline(p.Remedy, cr.now)
	metricsAll := consoleModelMetrics(m)
	ten := tenancyStatus(activeGovernor.Load())
	out := consoleOut{
		Meta: []string{"generated " + p.GeneratedAt.UTC().Format(time.RFC3339), "fathom " + buildinfo.Version(), callLogLine(p.CallLog)},
		Actions: `<span class="flex flex-wrap gap-2 [&>a]:min-h-11">` + form.RenderButton(form.Button{Label: "Open /metrics", Href: "/metrics", Variant: form.ButtonGhost}) +
			form.RenderButton(form.Button{Label: "Daemon status", Href: "/statusz", Variant: form.ButtonGhost}) + `</span>`,
		Model:   operatePageModel{Operate: p.View, Flywheel: p.Flywheel, Capabilities: caps, RemedyNow: rem, Tenancy: ten},
		Metrics: pageMetrics(cr.page, metricsAll),
		Sections: map[string]any{
			operateLoadVerb:      p.View.LoadModel,
			operateTrafficVerb:   withCallLog(p.Traffic, p.CallLog),
			operateCallerVerb:    withCallLog(p.Callers, p.CallLog),
			operateAdoptVerb:     withCallLog(p.Adoption, p.CallLog),
			operateRemedyVerb:    p.Remedy,
			operateConfigVerb:    p.Config,
			operateHarnessVerb:   p.Harnesses,
			operateResourcesVerb: p.Resources,
		},
		Body: func() string { return renderOperate(cr, p, caps, svc, names, rem, metricsAll, ten) },
	}
	return out, nil
}

func renderOperate(cr *consoleRequest, p operateProjection, caps *capabilityModel, svc string, names []string, rem remedyNow, mets console.Metrics, ten *governorSnapshot) string {
	v := p.View
	fresh := "computed " + p.GeneratedAt.UTC().Format("15:04:05 UTC") + ", reused for 30 s"
	var b strings.Builder

	// The band: the daemon, memory, and the surfaces' headline latency.
	surfaceRow := func(group string) *metrics.TrafficRow {
		for i := range p.Traffic.Report.Rows {
			if p.Traffic.Report.Rows[i].Group == group {
				return &p.Traffic.Report.Rows[i]
			}
		}
		return nil
	}
	latencyStat := func(label, group string) stat.Stat {
		if p.Traffic.Reason != "" {
			// An unread log is not a quiet surface: the tile says why.
			return valueStat(label, v.WindowCalls)
		}
		r := surfaceRow(group)
		if r == nil || r.P95MS == nil {
			return stat.Absent(label, "no call on this surface in the last 7 days")
		}
		sub := "p50 " + evMS(*r.P50MS) + " · " + plural(r.Calls, "call")
		if r.ErrorRate != nil {
			sub += " · " + evPercent(*r.ErrorRate) + " errors"
		}
		return stat.Stat{Label: label, Value: evMS(*r.P95MS), Sub: sub}
	}
	commit := shortCommit(v.Daemon.Commit)
	if commit == "" {
		commit = "unstamped"
	}
	if v.Daemon.Dirty {
		commit += " (dirty)"
	}
	uptime := "uptime not recorded"
	if v.Daemon.UptimeSec.Known() {
		uptime = "up " + humanUptime(float64(v.Daemon.UptimeSec.V))
	}
	memory := valueStat("memory (MB)", v.Daemon.MemoryMB)
	if v.Daemon.MemoryMB.Known() {
		memory.Sub = "phys footprint (resident set elsewhere), one reading at render; Go heap " + valuePlain(v.Daemon.HeapMB) + " MB"
	}
	calls := valueStat("calls, last 7 days", v.WindowCalls)
	if total := mets[console.MetricCalls]; total.Known() {
		calls.Sub = valuePlain(total) + " in the whole log; console reads excluded"
	}
	b.WriteString(stat.RenderBand([]stat.Stat{
		{Label: "daemon", Value: v.Daemon.Version, Sub: "@ " + commit + " · " + v.Daemon.Addr + " · " + uptime},
		memory,
		{Label: "goroutines", Value: fmtThousands(v.Daemon.Goroutines.V), Sub: "at render"},
		calls,
		latencyStat("agent surface p95", "agent"),
		latencyStat("unscoped p95", "(unscoped)"),
		operateTenancyStat(ten),
	}, stat.BandOptions{AriaLabel: "the daemon"}))

	b.WriteString(`<div class="mt-4 grid gap-4 xl:grid-cols-2">`)
	// Tenancy (RT-121): `fathom resources --json`, full width.
	tenState, tenReason := console.StatePresent, ""
	if !p.Resources.Daemon.Reachable && p.Resources.Tenancy == nil {
		tenState, tenReason = console.StateAbsent, "the daemon did not answer and its ring holds no sample: "+p.Resources.Daemon.Unreachable
	}
	b.WriteString(`<div class="min-w-0 xl:col-span-2">` + renderPanel(consolePanel{
		ID: "tenancy", Title: "Tenancy", SourceKey: "daemon",
		Question: "How much memory does the daemon hold against its budget, did the ladder move, and is anything leaking — processes or disk?",
		Verb:     operateResourcesVerb, Source: p.Resources.Footprint.Ring + " · " + firstNonEmpty(p.Resources.Budget.Source, "no budget read") + " · the lifecycle ledger",
		Denominator: "the last " + p.Resources.Footprint.Window + "; " + plural(p.Resources.Footprint.Samples, "ring sample"), Freshness: fresh,
		State: tenState, Reason: tenReason,
		Body: renderTenancyPanel(p.Resources),
	}) + `</div>`)
	// Traffic per day and surface: a truth-state heatmap, then its table.
	// The panels that read the call log carry its read state: an unreadable
	// log is an error on each of them, never an idle daemon.
	logState, logReason := console.StatePresent, ""
	if !v.WindowCalls.Known() {
		logState, logReason = v.WindowCalls.State, v.WindowCalls.Reason
	}
	b.WriteString(`<div class="min-w-0 xl:col-span-2">` + renderPanel(consolePanel{
		ID: "traffic", Title: "Calls per day, per surface", SourceKey: "sessions",
		Question: "Which surfaces answered how much, how fast and how reliably, day by day? A hatched day has no call-log rows — absent, not zero.",
		Verb:     operateLoadVerb, Source: p.CallLog.Path + " (rule " + string(p.CallLog.Rule) + ")",
		Denominator: "usage calls per UTC day, 14 days; console reads excluded", Freshness: fresh,
		State: v.WindowCalls.State, Reason: v.WindowCalls.Reason,
		Body: renderTrafficHeatmap(v),
	}) + `</div>`)

	// Latency and errors per surface and per caller class (7 days): one
	// panel per verb, so each table's chip is the command that prints it.
	b.WriteString(`<div class="grid min-w-0 gap-4 xl:col-span-2 xl:grid-cols-2">`)
	b.WriteString(renderPanel(consolePanel{
		ID: "surfaces", Title: "Latency and errors, by surface", SourceKey: "sessions",
		Question: "What does each surface cost in latency, and how often does it fail?",
		Verb:     operateTrafficVerb, Source: p.CallLog.Path, Denominator: "the last 7 days; console reads excluded", Freshness: fresh,
		State: logState, Reason: logReason,
		Body: renderTrafficTable("by surface", "operate-surfaces", p.Traffic),
	}))
	b.WriteString(renderPanel(consolePanel{
		ID: "callers", Title: "Latency and errors, by caller class", SourceKey: "sessions",
		Question: "Who calls — agents, the edit hook, gates, refresh, plumbing — and what does each cost?",
		Verb:     operateCallerVerb, Source: p.CallLog.Path + " (caller: origin and surface)", Denominator: "the last 7 days; console reads excluded", Freshness: fresh,
		State: logState, Reason: logReason,
		Body: renderTrafficTable("by caller class", "operate-callers", p.Callers),
	}))
	b.WriteString(`</div>`)
	b.WriteString(`<div class="min-w-0 xl:col-span-2">`)

	// What carries the load: the tools by caller class (`metrics load`).
	toolsDenom := ""
	if v.ToolsCalled.Known() {
		toolsDenom = fmt.Sprintf("the %d most-called of %s called in the last 7 days", len(v.Tools), plural(v.ToolsCalled.V, "tool"))
	}
	b.WriteString(renderPanel(consolePanel{
		ID: "tools", Title: "What carries the load", SourceKey: "sessions",
		Question: "Which tools are called, and by whom — so plumbing never passes for adoption?",
		Verb:     operateLoadVerb, Source: p.CallLog.Path + " (caller: origin and surface)",
		Denominator: toolsDenom, Freshness: fresh,
		State: logState, Reason: logReason,
		Body: renderToolLoads(v),
	}))
	b.WriteString(`</div>`)

	// The tools never called, from `instrument --adoption`.
	zeroState, zeroReason := console.StatePresent, ""
	switch {
	case p.Adoption.CallLogReadError != "":
		zeroState, zeroReason = console.StateError, "the call log is unreadable: "+p.Adoption.CallLogReadError
	case !p.Adoption.CallLogRead:
		zeroState, zeroReason = console.StateAbsent, "no call log was read, so no tool can be said to be uncalled"
	}
	b.WriteString(`<div class="min-w-0 xl:col-span-2">` + renderPanel(consolePanel{
		ID: "zero-calls", Title: "Tools never called", SourceKey: "sessions",
		Question: "Which registered tools has no one ever called — use them or prune them?",
		Verb:     operateAdoptVerb, Source: p.CallLog.Path + " and the served binary's own tool list",
		Denominator: zeroCallDenominator(p.Adoption), Freshness: fresh,
		State: zeroState, Reason: zeroReason,
		Body: renderZeroCalls(p.Adoption),
	}) + `</div>`)

	// The flywheel.
	b.WriteString(renderPanel(consolePanel{
		ID: "flywheel", Title: "Flywheel", SourceKey: "sessions plans",
		Question: "Is each feedback loop that improves fathom closed — producing, arriving and consumed?",
		Verb:     "fathom flywheel --json", Source: "the ledgers, the call log, the hooks log and this repository's concerns",
		Denominator: plural(len(p.Flywheel), "loop"), Freshness: fresh,
		Body: renderFlywheel(p.Flywheel),
	}))

	// config --explain.
	b.WriteString(renderPanel(consolePanel{
		ID: "config", Title: "Configuration", SourceKey: "daemon",
		Question: "Which settings is this daemon running with, and where did each come from?",
		Verb:     operateConfigVerb, Source: "config.toml (user, then project) as this daemon's working directory resolves it",
		Freshness: fresh,
		Body:      renderConfigExplain(p.Config),
	}))
	// Time to remedy (SH-9), full width: its gap table has six columns.
	b.WriteString(`<div class="min-w-0 xl:col-span-2">` + renderPanel(consolePanel{
		ID: "remedy", Title: "Time to remedy", SourceKey: "index",
		Question: "How long does a wiring gap stay open before someone closes it?",
		Verb:     operateRemedyVerb, Source: "the fleet report snapshots in " + p.Remedy.SnapshotDir,
		Denominator: plural(p.Remedy.Snapshots, "snapshot"), Freshness: fresh,
		State: rem.MedianHours.State, Reason: rem.MedianHours.Reason,
		Body: renderRemedy(p.Remedy, rem),
	}) + `</div>`)

	// Capabilities of one project.
	capVerb := "fathom capabilities --json"
	if caps != nil {
		capVerb = "fathom capabilities --service " + shellQuote(svc) + " --repo " + shellQuote(caps.RepoRoot) + " --json"
	}
	b.WriteString(`<div class="min-w-0 xl:col-span-2">` + renderPanel(consolePanel{
		ID: "capabilities", Title: "Capabilities", SourceKey: "index",
		Question: "What is switched on in a project, and why is anything off?",
		Verb:     capVerb, Source: "the capability registry's probes over the project root and this daemon's model",
		Denominator: capDenominator(caps), Freshness: "as of the model read",
		Body: renderCapabilities(cr, caps, svc, names),
	}) + `</div>`)

	// The harness matrix: which coding-agent harnesses fathom wires, how.
	harnessVerb := operateHarnessVerb
	if cr.deps.selfURL != "" && cr.deps.selfURL != harnessMatrixDefaultURL {
		harnessVerb = "fathom harness matrix --url " + shellQuote(cr.deps.selfURL) + " --json"
	}
	b.WriteString(`<div class="min-w-0 xl:col-span-2">` + renderPanel(consolePanel{
		ID: "harnesses", Title: "Harnesses", SourceKey: "daemon",
		Question: "Which coding-agent harnesses can fathom wire, at which tier, through which files?",
		Verb:     harnessVerb, Source: "the harness registry (internal/harness), each row's MCP line rendered for this daemon",
		Denominator: plural(len(p.Harnesses), "harness"), Freshness: fresh,
		Body: renderHarnessMatrix(p.Harnesses),
	}) + `</div>`)

	b.WriteString(`</div>`)
	return b.String()
}

// renderHarnessMatrix is `fathom harness matrix --json` as a table (from md
// up) and a stacked list (below md): tier, the instruction files, the
// skills path, the MCP config and its add line, and what each descriptor
// was verified against — "unverified" said in its own cell.
func renderHarnessMatrix(rows []harness.AdoptionRow) string {
	if len(rows) == 0 {
		return `<p class="italic text-ink-muted">the harness registry is empty</p>`
	}
	var trows []table.Row
	var stack []evStackItem
	for _, r := range rows {
		mcp := `<span class="[overflow-wrap:anywhere]">` + html.EscapeString(strings.Join(r.MCPConfig, ", ")) + `</span>`
		if r.MCPAdd != "" {
			mcp += `<code class="mt-1 block font-mono text-xs text-ink-muted [overflow-wrap:anywhere]">` + html.EscapeString(r.MCPAdd) + `</code>`
		}
		files := `<span class="[overflow-wrap:anywhere]">` + html.EscapeString(strings.Join(r.InstructionFile, ", ")) + `</span>`
		skills := `<span class="[overflow-wrap:anywhere]">` + html.EscapeString(strings.Join(r.SkillsPath, ", ")) + `</span>`
		verified := nonEmptyStr(r.VerifiedAgainst, "unverified")
		trows = append(trows, table.Row{ID: "harness-" + r.ID, Cells: []string{r.Display, r.Tier, "", "", "", verified},
			HTMLCells: []templComponent{nil, nil, evRaw(files), evRaw(skills), evRaw(mcp)}})
		stack = append(stack, evStackItem{
			Heading: html.EscapeString(r.Display) + ` <span class="text-xs font-normal text-ink-muted">· tier ` + html.EscapeString(r.Tier) + `</span>`,
			Items: []chrome.DescItem{
				{Term: "instruction files", DetailHTML: template.HTML(files)},
				{Term: "skills", DetailHTML: template.HTML(skills)},
				{Term: "MCP config", DetailHTML: template.HTML(mcp)},
				{Term: "verified against", Detail: verified},
			},
		})
	}
	tbl := renderKitTable(table.TableParams{TableID: "operate-harnesses", Caption: "the harness matrix (tier after the 60-day demotion clock)",
		Columns: []table.Column{{Key: "harness", Label: "Harness"}, {Key: "tier", Label: "Tier"}, {Key: "files", Label: "Instruction files"}, {Key: "skills", Label: "Skills"}, {Key: "mcp", Label: "MCP config"}, {Key: "verified", Label: "Verified against"}},
		Rows:    trows})
	return evTableOrStack("operate-harnesses", "harnesses", tbl, stack)
}

// renderTrafficHeatmap is the surfaces × days heatmap with truth states:
// a day the log has no row for is missing (hatched), a surface silent on a
// busy day is a measured zero; the table alternative lists every cell.
func renderTrafficHeatmap(v console.OperateModel) string {
	if !v.WindowCalls.Known() && len(v.Surfaces) == 0 {
		return `<p class="italic text-ink-muted">` + html.EscapeString(v.WindowCalls.Reason) + `</p>`
	}
	idx := map[string]int{}
	for i, d := range v.Days {
		idx[d] = i
	}
	sIdx := map[string]int{}
	for i, s := range v.Surfaces {
		sIdx[s] = i
	}
	rows := make([][]float64, len(v.Surfaces))
	states := make([][]viz.HeatmapCell, len(v.Surfaces))
	for i := range rows {
		rows[i] = make([]float64, len(v.Days))
		states[i] = make([]viz.HeatmapCell, len(v.Days))
	}
	var tRows []table.Row
	for _, c := range v.Traffic {
		r, col := sIdx[c.Surface], idx[c.Day]
		switch c.Calls.State {
		case console.StateAbsent, console.StateError:
			states[r][col] = viz.HeatmapCell{State: viz.CellMissing, Reason: "no call-log rows that day"}
		case console.StateZero:
			states[r][col] = viz.HeatmapCell{State: viz.CellZero, Reason: "none on this surface that day"}
		default:
			rows[r][col] = float64(c.Calls.V)
		}
		p50, p95, er := "—", "—", "—"
		if c.P50MS != nil {
			p50, p95, er = evMS(*c.P50MS), evMS(*c.P95MS), evPercent(*c.ErrorRate)
		}
		tRows = append(tRows, table.Row{ID: "t-" + c.Day + "-" + c.Surface, Cells: []string{c.Day, c.Surface, "", p50, p95, er},
			HTMLCells: []templComponent{nil, nil, evRaw(evCell(c.Calls, "no rows"))}})
	}
	xl := make([]string, len(v.Days))
	for i, d := range v.Days {
		if len(d) == 10 {
			xl[i] = d[5:]
		} else {
			xl[i] = d
		}
	}
	svg := viz.Heatmap(rows, viz.HeatmapOptions{XLabels: xl, YLabels: v.Surfaces, States: states, Cell: 40,
		AriaLabel: "usage calls per day per surface; hatched: no call-log rows that day"})
	tbl := renderKitTable(table.TableParams{TableID: "operate-traffic-days", Caption: "calls, p50, p95 and error rate per day and surface",
		Columns: []table.Column{{Key: "day", Label: "Day (UTC)"}, {Key: "surface", Label: "Surface"}, {Key: "calls", Label: "Calls", Align: "right"}, {Key: "p50", Label: "p50", Align: "right"}, {Key: "p95", Label: "p95", Align: "right"}, {Key: "err", Label: "Errors", Align: "right"}},
		Rows:    tRows})
	var totals []string
	for _, d := range v.DayTotals {
		totals = append(totals, `<li class="whitespace-nowrap"><span class="text-ink-muted">`+html.EscapeString(d.Day[5:])+`</span> `+evCell(d.Calls, "no rows")+`</li>`)
	}
	return evChart("calls per day per surface", svg, "min-w-[560px]", "every day and surface as a table", tbl) +
		`<ul class="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-xs" aria-label="calls per day, every surface">` + strings.Join(totals, "") + `</ul>`
}

// renderTrafficTable is one `metrics traffic` report as a table (from md
// up) and a stacked list (below md, where p95 and errors would scroll off).
// An unread log renders nothing here: the panel's truth line names why.
func renderTrafficTable(caption, id string, res trafficResult) string {
	if res.Reason != "" {
		return ""
	}
	r := res.Report
	if r.TotalCalls == 0 {
		return `<p class="italic text-ink-muted">` + html.EscapeString(caption) + `: ` + html.EscapeString(r.Note) + `</p>`
	}
	var rows []table.Row
	var stack []evStackItem
	for _, row := range r.Rows {
		er := row.ErrorRateNote
		if row.ErrorRate != nil {
			er = evPercent(*row.ErrorRate)
		}
		share := evPercent(float64(row.Calls) * 100 / float64(r.TotalCalls))
		rows = append(rows, table.Row{ID: id + "-" + row.Group, Cells: []string{row.Group, fmtThousands(row.Calls), share, evMS(*row.P50MS), evMS(*row.P95MS), er}})
		stack = append(stack, evStackItem{
			Heading: html.EscapeString(row.Group) + ` <span class="text-xs font-normal text-ink-muted">· ` + plural(row.Calls, "call") + ` · ` + share + `</span>`,
			Items: []chrome.DescItem{
				{Term: "p50 · p95", Detail: evMS(*row.P50MS) + " · " + evMS(*row.P95MS)},
				{Term: "errors", Detail: er},
			},
		})
	}
	title := caption + ", " + r.Since[:10] + " … " + r.Until[:10]
	tbl := renderKitTable(table.TableParams{TableID: id, Caption: title,
		Columns: []table.Column{{Key: "group", Label: "Group"}, {Key: "calls", Label: "Calls", Align: "right"}, {Key: "share", Label: "Share", Align: "right"}, {Key: "p50", Label: "p50", Align: "right"}, {Key: "p95", Label: "p95", Align: "right"}, {Key: "err", Label: "Errors", Align: "right"}},
		Rows:    rows})
	return `<p class="mb-2 text-xs text-ink-muted md:hidden">` + html.EscapeString(title) + `</p>` + evTableOrStack(id, caption, tbl, stack)
}

// classRole colors a caller class in the share bars.
func classRole(c console.CallerClass) string {
	switch c {
	case console.ClassAgent:
		return "accent"
	case console.ClassEditHook:
		return "info"
	case console.ClassGate:
		return "caution"
	case console.ClassRefresh:
		return "positive"
	case console.ClassPlumbing:
		return "ink-muted"
	}
	return "line-strong"
}

// renderToolLoads is the load table: a share bar split by caller class per
// tool, its calls and its dominant caller. An unread log renders nothing
// (the panel's truth line names why); a read log with no call in the week
// is a measured nothing, said so.
func renderToolLoads(v console.OperateModel) string {
	var b strings.Builder
	switch {
	case !v.ToolsCalled.Known():
		return ""
	case len(v.Tools) == 0:
		b.WriteString(`<p class="italic text-ink-muted">no usage call in the last 7 days</p>`)
	default:
		maxCalls := v.Tools[0].Calls
		var rows []table.Row
		for _, t := range v.Tools {
			var segs []viz.Segment
			var split []string
			for _, c := range t.By {
				segs = append(segs, viz.Segment{Value: float64(c.Calls), Role: classRole(c.Class), Label: string(c.Class) + ": " + fmtThousands(c.Calls)})
				split = append(split, string(c.Class)+" "+fmtThousands(c.Calls))
			}
			if rest := maxCalls - t.Calls; rest > 0 {
				segs = append(segs, viz.Segment{Value: float64(rest), Role: "surface-sunken", Label: "scale"})
			}
			bar := viz.StackBar(segs, viz.StackBarOptions{W: 160, H: 10, Fluid: true, AriaLabel: t.Tool + " calls by caller class"})
			rows = append(rows, table.Row{ID: "load-" + t.Tool, Cells: []string{"", "", fmtThousands(t.Calls), string(t.Class)},
				HTMLCells: []templComponent{evRaw(`<code class="font-mono text-xs">` + html.EscapeString(t.Tool) + `</code>`),
					evRaw(`<div class="w-24 sm:w-40">` + bar + `</div><span class="sr-only">` + html.EscapeString(strings.Join(split, ", ")) + `</span>`)}})
		}
		b.WriteString(renderKitTable(table.TableParams{TableID: "operate-tools", Caption: fmt.Sprintf("the %d most-called tools, last 7 days", len(v.Tools)),
			Columns: []table.Column{{Key: "tool", Label: "Tool"}, {Key: "share", Label: "Calls by caller"}, {Key: "calls", Label: "Calls", Align: "right"}, {Key: "class", Label: "Mostly"}},
			Rows:    rows}))
		var legend []string
		for _, c := range console.CallerClasses {
			legend = append(legend, `<span class="inline-flex items-center gap-1"><svg width="10" height="10" aria-hidden="true"><rect width="10" height="10" rx="2" fill="var(--ak-color-`+classRole(c)+`)"/></svg>`+html.EscapeString(string(c))+`</span>`)
		}
		b.WriteString(`<p class="mt-2 flex flex-wrap gap-3 text-xs text-ink-muted" aria-label="caller classes">` + strings.Join(legend, "") + `</p>`)
	}
	return b.String()
}

// zeroCallDenominator is the zero-call list's count over the served
// binary's registered tools ("" when no log was read).
func zeroCallDenominator(a AdoptionReport) string {
	if a.CallLogReadError != "" || !a.CallLogRead {
		return ""
	}
	return fmt.Sprintf("%d of %d registered tools never called", len(a.ZeroCallTools), a.RegisteredToolCount)
}

// renderZeroCalls is `instrument --adoption`'s zero-call list. An unread
// log renders nothing: the panel's truth line names why.
func renderZeroCalls(a AdoptionReport) string {
	if a.CallLogReadError != "" || !a.CallLogRead {
		return ""
	}
	var list strings.Builder
	list.WriteString(`<p class="mb-2 text-ink-muted">` + fmt.Sprintf("%d of %d registered tools were never called; %d of them are on the agent surface (use them or prune them).", len(a.ZeroCallTools), a.RegisteredToolCount, len(a.ZeroCallAgentTools)) + `</p>`)
	list.WriteString(`<ul class="flex flex-wrap gap-2" aria-label="tools never called">`)
	agent := map[string]bool{}
	for _, t := range a.ZeroCallAgentTools {
		agent[t] = true
	}
	for _, t := range a.ZeroCallTools {
		mark := ""
		if agent[t] {
			mark = ` <span class="text-ink-muted">(agent surface)</span>`
		}
		list.WriteString(`<li class="rounded-md border border-line px-2 py-1 font-mono text-xs">` + html.EscapeString(t) + mark + `</li>`)
	}
	list.WriteString(`</ul>`)
	return chrome.RenderDisclosure(chrome.Disclosure{Summary: fmt.Sprintf("%d registered tools with no call at all", len(a.ZeroCallTools)), Body: template.HTML(list.String())})
}

// renderFlywheel lists each loop with its status in words (never color
// alone), its evidence and its remedy.
func renderFlywheel(loops []flywheelCheck) string {
	var b strings.Builder
	b.WriteString(`<ul class="divide-y divide-line" aria-label="feedback loops">`)
	for _, c := range loops {
		role := "text-positive"
		switch c.Status {
		case loopOpen:
			role = "text-danger"
		case loopIdle:
			role = "text-ink-muted"
		}
		b.WriteString(`<li class="grid gap-1 py-2 sm:grid-cols-[10rem_5rem_1fr]" data-loop-status="` + html.EscapeString(string(c.Status)) + `">`)
		b.WriteString(`<span class="font-mono text-xs text-ink [overflow-wrap:anywhere]">` + html.EscapeString(c.Loop) + `</span>`)
		b.WriteString(`<span class="text-xs font-semibold ` + role + `">` + html.EscapeString(string(c.Status)) + `</span>`)
		b.WriteString(`<span class="min-w-0 text-ink-muted [overflow-wrap:anywhere]">` + html.EscapeString(c.Evidence))
		if c.Remedy != "" {
			b.WriteString(remedy(c.Remedy))
		}
		b.WriteString(`</span></li>`)
	}
	b.WriteString(`</ul><p class="mt-2 text-xs text-ink-muted">CLOSED: producing, arriving and consumed. OPEN: a defect — data should flow and does not. IDLE: honestly nothing to measure yet.</p>`)
	return b.String()
}

// renderRemedy is time-to-remedy's headline, the gaps per repository and
// the recorded baseline. With no snapshot every count is not measured —
// never "0 gaps across 0 snapshots"; a lower-bound median and a closed gap
// that opened on or before the first snapshot read "≥"; the grade column is
// the latest snapshot's, dated, never "now".
func renderRemedy(r remedyReport, rem remedyNow) string {
	var b strings.Builder
	head := valueStat("median time to remedy (hours)", rem.MedianHours)
	if rem.MedianHours.Known() {
		if rem.LowerBound && !strings.HasPrefix(head.Value, "≥") {
			head.Value = "≥\u00a0" + head.Value
		}
		days := float64(rem.MedianHours.V) / 24
		head.Sub = strconv.FormatFloat(days, 'f', 1, 64) + " days · " + rem.MedianHours.Denominator
		if rem.MedianHours.State == console.StateStale {
			head.Sub += " · stale: " + rem.MedianHours.Reason
		}
	}
	count := func(label string, n *int, sub string) stat.Stat {
		if n == nil {
			s := stat.Absent(label, r.Note)
			s.ValueRole = "ink-muted"
			return s
		}
		return stat.Stat{Label: label, Value: fmtThousands(*n), Sub: sub}
	}
	graded := "of the repositories graded"
	if r.Graded != nil {
		graded = fmt.Sprintf("of %s graded", plural(*r.Graded, "repo"))
	}
	b.WriteString(stat.RenderBand([]stat.Stat{
		head,
		count("gaps closed", r.Closed, "across "+plural(r.Snapshots, "snapshot")),
		count("gaps still open", r.Open, "no time to remedy yet"),
		count("below grade A, latest snapshot", r.BelowA, graded),
	}, stat.BandOptions{AriaLabel: "time to remedy"}))
	latest := ""
	if len(r.Latest) >= 10 {
		latest = r.Latest[:10]
	}
	var rows []table.Row
	var stack []evStackItem
	for _, repo := range r.Repos {
		for i, e := range repo.Episodes {
			opened := e.OpenedAt[:10]
			if e.OpenedOnOrBefore {
				opened = "≤ " + opened
			}
			span := "open"
			switch {
			case e.Hours != nil && e.OpenedOnOrBefore:
				span = "≥ " + strconv.FormatFloat(*e.Hours, 'f', 1, 64) + " h"
			case e.Hours != nil:
				span = strconv.FormatFloat(*e.Hours, 'f', 1, 64) + " h"
			case e.OpenHours != nil:
				span = "open ≥ " + strconv.FormatFloat(*e.OpenHours, 'f', 1, 64) + " h"
			}
			closed := "—"
			if e.ClosedAt != "" {
				closed = e.ClosedAt[:10]
			}
			nowrap := func(t string) templComponent {
				return evRaw(`<span class="whitespace-nowrap">` + html.EscapeString(t) + `</span>`)
			}
			grades := strings.Join(e.Grades, " → ")
			rows = append(rows, table.Row{ID: "gap-" + repo.Service + "-" + strconv.Itoa(i), Cells: []string{repo.Service, grades, "", "", "", repo.Latest},
				HTMLCells: []templComponent{nil, nil, nowrap(opened), nowrap(closed), nowrap(span)}})
			stack = append(stack, evStackItem{
				Heading: html.EscapeString(repo.Service) + ` <span class="text-xs font-normal text-ink-muted">· ` + html.EscapeString(span) + `</span>`,
				Items: []chrome.DescItem{
					{Term: "grade during the gap", Detail: grades},
					{Term: "opened · closed", Detail: opened + " · " + closed},
					{Term: "grade at the latest snapshot", Detail: nonEmptyStr(repo.Latest, "not graded")},
				},
			})
		}
	}
	if len(rows) > 0 {
		caption := "wiring gaps, per repository (≤: open on or before the first snapshot; ≥: a lower bound)"
		if latest != "" {
			caption += "; the latest snapshot is " + latest
		}
		tbl := renderKitTable(table.TableParams{TableID: "operate-remedy", Caption: caption,
			Columns: []table.Column{{Key: "repo", Label: "Repository"}, {Key: "grades", Label: "Grade during the gap"}, {Key: "opened", Label: "Opened"}, {Key: "closed", Label: "Closed"}, {Key: "span", Label: "Time to remedy", Align: "right"}, {Key: "latest", Label: "Grade at the latest snapshot"}},
			Rows:    rows})
		b.WriteString(`<div class="mt-3"><p class="mb-2 text-xs text-ink-muted md:hidden">` + html.EscapeString(caption) + `</p>` + evTableOrStack("operate-remedy", "wiring gaps, per repository", tbl, stack) + `</div>`)
	}
	b.WriteString(`<p class="mt-2 text-xs text-ink-muted">` + html.EscapeString(r.Note) + `. Baseline ` + html.EscapeString(r.Baseline.Date) + `: ` + html.EscapeString(r.Baseline.Note) + `</p>`)
	return b.String()
}

func capDenominator(c *capabilityModel) string {
	if c == nil {
		return "no registered project"
	}
	return fmt.Sprintf("%d of %d operating in %s", c.Operating, c.Total, c.Service)
}

// renderCapabilities is one project's capabilities: a project picker (a
// GET form, no JavaScript) and each capability's state, reason and remedy.
func renderCapabilities(cr *consoleRequest, c *capabilityModel, svc string, names []string) string {
	if c == nil {
		return `<p class="italic text-ink-muted">no registered project — capabilities are per project</p>`
	}
	var b strings.Builder
	opts := make([]form.Option, 0, len(names))
	for _, n := range names {
		opts = append(opts, form.Option{Value: n, Label: n})
	}
	q := url.Values{}
	for k, vs := range cr.r.URL.Query() {
		if k != "svc" && k != "format" {
			q[k] = vs
		}
	}
	b.WriteString(`<form method="get" action="/operate#capabilities" class="mb-3 flex flex-wrap items-end gap-2 [&_select]:min-h-11 [&_button]:min-h-11" aria-label="choose a project">`)
	for k, vs := range q {
		for _, v := range vs {
			b.WriteString(`<input type="hidden" name="` + html.EscapeString(k) + `" value="` + html.EscapeString(v) + `">`)
		}
	}
	b.WriteString(`<div class="min-w-48">` + form.RenderField(form.Field{Label: "Project", For: "operate-svc", Control: template.HTML(form.RenderSelect(form.Select{ID: "operate-svc", Name: "svc", Value: svc, Options: opts}))}) + `</div>`)
	b.WriteString(form.RenderButton(form.Button{Label: "Show", Variant: form.ButtonGhost}) + `</form>`)
	var rows []table.Row
	for _, cp := range c.Capabilities {
		state := `<span class="font-medium text-positive">✓ operating</span>`
		why := cp.Detail
		if !cp.Operating {
			state = `<span class="font-medium text-ink-muted">✗ off</span>`
			why = cp.Reason
		}
		fix := ""
		if cp.Cmd != "" {
			fix = `<code class="font-mono text-xs [overflow-wrap:anywhere]">` + html.EscapeString(cp.Cmd) + `</code>`
		}
		rows = append(rows, table.Row{ID: "cap-" + cp.ID, Cells: []string{cp.ID, "", "", cp.Scope}, HTMLCells: []templComponent{nil, evRaw(state), evRaw(`<span class="[overflow-wrap:anywhere]">` + html.EscapeString(why) + `</span>`), nil, evRaw(fix)}})
	}
	b.WriteString(renderKitTable(table.TableParams{TableID: "operate-capabilities", Caption: capDenominator(c),
		Columns: []table.Column{{Key: "id", Label: "Capability"}, {Key: "state", Label: "State"}, {Key: "why", Label: "Evidence or reason"}, {Key: "scope", Label: "Scope"}, {Key: "fix", Label: "Remedy"}},
		Rows:    rows}))
	return b.String()
}

// renderConfigExplain renders `config --explain --json`: the daemon block
// and the active profile, each key with the source that supplied it.
func renderConfigExplain(cfg map[string]any) string {
	if e, ok := cfg["error"].(string); ok {
		return `<p class="font-medium text-danger" data-truth-line="error">could not be read — ` + html.EscapeString(e) + `</p>`
	}
	var items []chrome.DescItem
	str := func(v any) string {
		switch x := v.(type) {
		case nil:
			return "unset"
		case string:
			if x == "" {
				return "unset"
			}
			return x
		case bool:
			return strconv.FormatBool(x)
		}
		return fmt.Sprint(v)
	}
	for _, layer := range []string{"user", "project"} {
		detail := str(cfg[layer+"_config"])
		if found, _ := cfg[layer+"_found"].(bool); !found {
			detail += " (not found)"
		}
		items = append(items, chrome.DescItem{Term: layer + " config", Detail: detail})
	}
	if d, ok := cfg["daemon"].(map[string]any); ok {
		keys := make([]string, 0, len(d))
		for k := range d {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			items = append(items, chrome.DescItem{Term: "[daemon] " + k, Detail: str(d[k])})
		}
	}
	items = append(items, chrome.DescItem{Term: "active profile", Detail: str(cfg["active_profile"])})
	prof, _ := cfg["profile"].(map[string]any)
	explain, _ := cfg["explain"].(map[string]string)
	keys := make([]string, 0, len(prof))
	for k := range prof {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		items = append(items, chrome.DescItem{Term: "profile " + k, DetailHTML: template.HTML(html.EscapeString(str(prof[k])) + ` <span class="text-xs text-ink-muted">← ` + html.EscapeString(explain[k]) + `</span>`)})
	}
	return `<div class="[overflow-wrap:anywhere]">` + chrome.RenderDescriptionList(items) + `</div>`
}

// operateTenancyStat is the band's tenancy tile: the ladder level with
// footprint (self + children) against the budget and the mode, from the
// same block /statusz serves (RT-52). No governor is said in words, and
// an unknown footprint is named, never drawn as zero (I38).
func operateTenancyStat(t *governorSnapshot) stat.Stat {
	if t == nil {
		return stat.Absent("tenancy", "no governor runs in this process — fathom resources reads the ring")
	}
	level := t.Level
	if t.LadderOff {
		level += " (ladder off)"
	}
	used := "footprint unknown"
	if t.Footprint >= 0 {
		total := t.Footprint
		if t.Children > 0 {
			total += t.Children
		}
		used = humanBytes(total) + " of " + humanBytes(t.Budget)
	}
	sub := used + " · mode " + t.Mode
	if n := len(t.InFlight); n > 0 {
		sub += " · " + plural(n, "call") + " in flight"
	}
	return stat.Stat{Label: "tenancy", Value: level, Sub: sub + " · fathom resources for history"}
}
