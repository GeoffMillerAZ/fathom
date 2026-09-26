package main

// console_operate.go — the Operate page at /operate (vision 35 §3 System,
// design §3 "Operate"; OC-44, SH-9). The daemon (version, commit, uptime,
// memory, goroutines); traffic per day and per surface with p50, p95 and
// error rate from the resolved call log, a day with no rows drawn as
// absent (hatched), never zero; the tools labelled by caller class and the
// zero-call list; the flywheel's loops; time-to-remedy from the fleet
// report snapshots; a project's capabilities; and `config --explain`.
//
// Every section is a verb's own computation (P10): `metrics traffic`
// (trafficReportFrom), `instrument --adoption` (instrumentAdoption),
// `fleet remedy` (buildRemedyReport), `config --explain` (configJSONValue),
// `flywheel` (runFlywheelChecks) and `capabilities` (capabilityJSON). The
// page reads the call log once per projection and keeps the projection for
// the model's TTL: the log is tens of megabytes and grows with every call.

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
	"github.com/geoffmilleraz/fathom/internal/metrics"
)

// The verbs whose --json the page carries (parity: TestConsolePagesRenderTheirVerbs).
const (
	operateTrafficVerb = "fathom metrics traffic --json"
	operateDaysVerb    = "fathom metrics traffic --by day --since 336h --json"
	operateCallerVerb  = "fathom metrics traffic --by caller --json"
	operateAdoptVerb   = "fathom instrument --adoption --json"
	operateRemedyVerb  = "fathom fleet remedy --json"
	operateConfigVerb  = "fathom config --explain --json"
)

func init() {
	declarePage(consolePage{
		ID: "operate", Route: "/operate", Href: "/operate", Group: "System", Order: -10, Nav: true,
		Title:    "Operate",
		Question: "Is the daemon healthy, what does it answer, for whom, and are the loops that improve fathom turning?",
		Verbs:    []string{operateTrafficVerb, operateDaysVerb, operateCallerVerb, operateAdoptVerb, operateRemedyVerb, operateConfigVerb},
		Shows:    []console.MetricID{console.MetricServices, console.MetricCalls, console.MetricAgentTools},
		Build:    buildOperatePage,
	})
}

// operateProjection is everything the page reads, computed once per TTL.
type operateProjection struct {
	View         console.OperateModel
	Traffic      metrics.TrafficReport
	Days         metrics.TrafficReport
	Callers      metrics.TrafficReport
	Adoption     AdoptionReport
	Remedy       remedyReport
	Config       map[string]any
	Flywheel     []flywheelCheck
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
}

// remedyNow is time-to-remedy's headline with its truth state: the median
// over closed gaps (a lower bound when any opened on or before the first
// snapshot), stale when the latest snapshot is over a week old.
type remedyNow struct {
	MedianHours console.Value[int] `json:"median_hours"`
	Latest      string             `json:"latest_snapshot,omitempty"`
}

var operateCache struct {
	sync.Mutex
	key string
	at  time.Time
	p   operateProjection
}

// operateNow reads (or reuses, within the model's TTL) the projection.
func operateNow(d *consoleDeps, m observatoryModel, now time.Time) operateProjection {
	resolved := d.sources().CallLog
	key := d.selfURL + "|" + resolved.Path
	operateCache.Lock()
	defer operateCache.Unlock()
	if operateCache.key == key && !operateCache.at.IsZero() && now.Sub(operateCache.at) < obsCacheTTL && !now.Before(operateCache.at) {
		return operateCache.p
	}
	p := buildOperateProjection(d, m, resolved, now)
	operateCache.key, operateCache.at, operateCache.p = key, now, p
	return p
}

func resetOperateCache() {
	operateCache.Lock()
	operateCache.key, operateCache.at = "", time.Time{}
	operateCache.Unlock()
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
	var entries []adoptionCallEntry
	var skipped, consoleReads int
	var readErr error
	if resolved.Path != "" {
		entries, skipped, consoleReads, readErr = callLogReadCounting(resolved.Path)
	}
	facts := callLogFactsFor(resolved, entries, readErr)
	if readErr == nil {
		facts.ConsoleReads = consoleReads
	}
	reason, isErr, _ := callLogState(facts)
	p.View = console.OperateView(console.OperateInputs{
		Daemon: daemonFacts(d), Calls: callRowsOf(entries), CallLog: resolved,
		CallLogReason: reason, CallLogError: isErr, Now: now, Days: 14, WindowDays: 7, TopTools: 10,
	})
	week, fortnight := now.Add(-7*24*time.Hour), now.Add(-336*time.Hour)
	p.Traffic = trafficReportFrom(entries, "", metrics.TrafficGroupBySurface, week, now)
	p.Days = trafficReportFrom(entries, "", metrics.TrafficGroupByDay, fortnight, now)
	p.Callers = trafficReportFrom(entries, "", metrics.TrafficGroupByCaller, week, now)
	pre := &adoptionPreread{entries: entries, skipped: skipped, err: readErr}
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
	p.Flywheel = flywheelChecksFrom(m.Impact, entries, logErr, now)
	return p
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
	v := console.Measured(int(math.Round(*r.MedianHours)), source).WithDenominator(fmt.Sprintf("median over %s; %d still open", plural(r.Closed, "closed gap"), r.Open))
	if latest, err := time.Parse(time.RFC3339, r.Latest); err == nil {
		v = v.WithAsOf(latest)
		if age := now.Sub(latest); age > 7*24*time.Hour {
			v = console.Stale(v.V, fmt.Sprintf("the latest snapshot is %d days old — fathom fleet report records today's", int(age.Hours()/24)), latest).WithSource(source).WithDenominator(v.Denominator)
		}
	}
	for _, repo := range r.Repos {
		for _, e := range repo.Episodes {
			if e.OpenedOnOrBefore && e.Hours != nil && v.State != console.StateStale {
				v = console.Partial(v.V, "a lower bound — gaps already open in the first snapshot opened on or before it").WithSource(source).WithDenominator(v.Denominator).WithAsOf(v.AsOf)
			}
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
	out := consoleOut{
		Meta: []string{"generated " + p.GeneratedAt.UTC().Format(time.RFC3339), "fathom " + buildinfo.Version(), callLogLine(p.CallLog)},
		Actions: `<span class="flex flex-wrap gap-2 [&>a]:min-h-11">` + form.RenderButton(form.Button{Label: "Open /metrics", Href: "/metrics", Variant: form.ButtonGhost}) +
			form.RenderButton(form.Button{Label: "Daemon status", Href: "/statusz", Variant: form.ButtonGhost}) + `</span>`,
		Model:   operatePageModel{Operate: p.View, Flywheel: p.Flywheel, Capabilities: caps, RemedyNow: rem},
		Metrics: pageMetrics(cr.page, metricsAll),
		Sections: map[string]any{
			operateTrafficVerb: withCallLog(p.Traffic, p.CallLog),
			operateDaysVerb:    withCallLog(p.Days, p.CallLog),
			operateCallerVerb:  withCallLog(p.Callers, p.CallLog),
			operateAdoptVerb:   withCallLog(p.Adoption, p.CallLog),
			operateRemedyVerb:  p.Remedy,
			operateConfigVerb:  p.Config,
		},
		Body: func() string { return renderOperate(cr, p, caps, svc, names, rem, metricsAll) },
	}
	return out, nil
}

func renderOperate(cr *consoleRequest, p operateProjection, caps *capabilityModel, svc string, names []string, rem remedyNow, mets console.Metrics) string {
	v := p.View
	fresh := "computed " + p.GeneratedAt.UTC().Format("15:04:05 UTC") + ", reused for 30 s"
	var b strings.Builder

	// The band: the daemon, memory, and the surfaces' headline latency.
	surfaceRow := func(group string) *metrics.TrafficRow {
		for i := range p.Traffic.Rows {
			if p.Traffic.Rows[i].Group == group {
				return &p.Traffic.Rows[i]
			}
		}
		return nil
	}
	latencyStat := func(label, group string) stat.Stat {
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
	}, stat.BandOptions{AriaLabel: "the daemon"}))

	b.WriteString(`<div class="mt-4 grid gap-4 xl:grid-cols-2">`)
	// Traffic per day and surface: a truth-state heatmap, then its table.
	b.WriteString(`<div class="min-w-0 xl:col-span-2">` + renderPanel(consolePanel{
		ID: "traffic", Title: "Calls per day, per surface", SourceKey: "sessions",
		Question: "Which surfaces answered how much, how fast and how reliably, day by day? A hatched day has no call-log rows — absent, not zero.",
		Verb:     operateDaysVerb, Source: p.CallLog.Path + " (rule " + string(p.CallLog.Rule) + ")",
		Denominator: "usage calls per UTC day, 14 days; console reads excluded", Freshness: fresh,
		State: v.WindowCalls.State, Reason: v.WindowCalls.Reason,
		Body: renderTrafficHeatmap(v),
	}) + `</div>`)

	// Latency and errors per surface and per caller class (7 days).
	b.WriteString(`<div class="grid min-w-0 gap-4 xl:col-span-2 xl:grid-cols-2">`)
	b.WriteString(renderPanel(consolePanel{
		ID: "surfaces", Title: "Latency and errors, by surface and by caller", SourceKey: "sessions",
		Question: "What does each surface and each caller class cost in latency, and how often does it fail?",
		Verb:     operateTrafficVerb, Source: p.CallLog.Path, Denominator: "the last 7 days; console reads excluded", Freshness: fresh,
		Body: renderTrafficTable("by surface", "operate-surfaces", p.Traffic) + `<div class="mt-4">` + renderTrafficTable("by caller class", "operate-callers", p.Callers) + `</div>`,
	}))

	// What carries the load: tools by caller class, and the zero-call list.
	b.WriteString(renderPanel(consolePanel{
		ID: "tools", Title: "What carries the load", SourceKey: "sessions",
		Question: "Which tools are called, and by whom — so plumbing never passes for adoption?",
		Verb:     operateAdoptVerb, Source: p.CallLog.Path + " (caller: origin and surface)",
		Denominator: fmt.Sprintf("%d of %s called in the last 7 days", len(v.Tools), plural(v.ToolsCalled, "tool")), Freshness: fresh,
		Body: renderToolLoads(v, p.Adoption),
	}))
	b.WriteString(`</div>`)

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
		capVerb = "fathom capabilities --service " + svc + " --repo " + caps.RepoRoot + " --json"
	}
	b.WriteString(`<div class="min-w-0 xl:col-span-2">` + renderPanel(consolePanel{
		ID: "capabilities", Title: "Capabilities", SourceKey: "index",
		Question: "What is switched on in a project, and why is anything off?",
		Verb:     capVerb, Source: "the capability registry's probes over the project root and this daemon's model",
		Denominator: capDenominator(caps), Freshness: "as of the model read",
		Body: renderCapabilities(cr, caps, svc, names),
	}) + `</div>`)

	b.WriteString(`</div>`)
	return b.String()
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
	return evChart("calls per day per surface", svg, 560, "every day and surface as a table", tbl) +
		`<ul class="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-xs" aria-label="calls per day, every surface">` + strings.Join(totals, "") + `</ul>`
}

// renderTrafficTable is one `metrics traffic` report as a table.
func renderTrafficTable(caption, id string, r metrics.TrafficReport) string {
	if r.TotalCalls == 0 {
		return `<p class="italic text-ink-muted">` + html.EscapeString(caption) + `: ` + html.EscapeString(r.Note) + `</p>`
	}
	var rows []table.Row
	for _, row := range r.Rows {
		er := row.ErrorRateNote
		if row.ErrorRate != nil {
			er = evPercent(*row.ErrorRate)
		}
		share := evPercent(float64(row.Calls) * 100 / float64(r.TotalCalls))
		rows = append(rows, table.Row{ID: id + "-" + row.Group, Cells: []string{row.Group, fmtThousands(row.Calls), share, evMS(*row.P50MS), evMS(*row.P95MS), er}})
	}
	return renderKitTable(table.TableParams{TableID: id, Caption: caption + ", " + r.Since[:10] + " … " + r.Until[:10],
		Columns: []table.Column{{Key: "group", Label: "Group"}, {Key: "calls", Label: "Calls", Align: "right"}, {Key: "share", Label: "Share", Align: "right"}, {Key: "p50", Label: "p50", Align: "right"}, {Key: "p95", Label: "p95", Align: "right"}, {Key: "err", Label: "Errors", Align: "right"}},
		Rows:    rows})
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

// renderToolLoads is the load table (a share bar split by caller class per
// tool, its calls and its dominant caller) and the zero-call lists.
func renderToolLoads(v console.OperateModel, a AdoptionReport) string {
	var b strings.Builder
	if len(v.Tools) == 0 {
		b.WriteString(`<p class="italic text-ink-muted">no usage call in the last 7 days</p>`)
	} else {
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
	// The zero-call list, from `instrument --adoption`.
	switch {
	case a.CallLogReadError != "":
		b.WriteString(`<p class="mt-3 text-danger">zero-call list not computed — the call log is unreadable: ` + html.EscapeString(a.CallLogReadError) + `</p>`)
	case !a.CallLogRead:
		b.WriteString(`<p class="mt-3 italic text-ink-muted">zero-call list not measured — no call log was read</p>`)
	default:
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
		b.WriteString(`<div class="mt-3">` + chrome.RenderDisclosure(chrome.Disclosure{Summary: fmt.Sprintf("%d registered tools with no call at all", len(a.ZeroCallTools)), Body: template.HTML(list.String())}) + `</div>`)
	}
	return b.String()
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
// the recorded baseline.
func renderRemedy(r remedyReport, rem remedyNow) string {
	var b strings.Builder
	head := valueStat("median time to remedy (hours)", rem.MedianHours)
	if rem.MedianHours.Known() {
		days := float64(rem.MedianHours.V) / 24
		head.Sub = strconv.FormatFloat(days, 'f', 1, 64) + " days · " + rem.MedianHours.Denominator
	}
	b.WriteString(stat.RenderBand([]stat.Stat{
		head,
		{Label: "gaps closed", Value: fmtThousands(r.Closed), Sub: "across " + plural(r.Snapshots, "snapshot")},
		{Label: "gaps still open", Value: fmtThousands(r.Open), Sub: "no time to remedy yet"},
		{Label: "below grade A, latest snapshot", Value: fmtThousands(r.BelowA), Sub: fmt.Sprintf("of %s graded", plural(r.Graded, "repo"))},
	}, stat.BandOptions{AriaLabel: "time to remedy"}))
	var rows []table.Row
	for _, repo := range r.Repos {
		for i, e := range repo.Episodes {
			opened := e.OpenedAt[:10]
			if e.OpenedOnOrBefore {
				opened = "≤ " + opened
			}
			span := "open"
			if e.Hours != nil {
				span = strconv.FormatFloat(*e.Hours, 'f', 1, 64) + " h"
			} else if e.OpenHours != nil {
				span = "open ≥ " + strconv.FormatFloat(*e.OpenHours, 'f', 1, 64) + " h"
			}
			closed := "—"
			if e.ClosedAt != "" {
				closed = e.ClosedAt[:10]
			}
			nowrap := func(t string) templComponent {
				return evRaw(`<span class="whitespace-nowrap">` + html.EscapeString(t) + `</span>`)
			}
			rows = append(rows, table.Row{ID: "gap-" + repo.Service + "-" + strconv.Itoa(i), Cells: []string{repo.Service, strings.Join(e.Grades, " → "), "", "", "", repo.Latest},
				HTMLCells: []templComponent{nil, nil, nowrap(opened), nowrap(closed), nowrap(span)}})
		}
	}
	if len(rows) > 0 {
		b.WriteString(`<div class="mt-3">` + renderKitTable(table.TableParams{TableID: "operate-remedy", Caption: "wiring gaps, per repository (≤: open on or before the first snapshot)",
			Columns: []table.Column{{Key: "repo", Label: "Repository"}, {Key: "grades", Label: "Grade during the gap"}, {Key: "opened", Label: "Opened"}, {Key: "closed", Label: "Closed"}, {Key: "span", Label: "Time to remedy", Align: "right"}, {Key: "latest", Label: "Grade now"}},
			Rows:    rows}) + `</div>`)
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
