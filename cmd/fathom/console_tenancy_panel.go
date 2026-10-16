package main

// console_tenancy_panel.go — Operate → Tenancy (RT-121,
// docs/design/responsible-tenant.md §14): the panel renders `fathom
// resources --json` (ADR-045: every panel renders a verb; the page carries
// the verb's JSON as its section, and the parity test compares them) with
// app-kit parts only — a stat band for the budget and its marks, a
// truth-state heatmap of the footprint per hour (an hour with no sample is
// hatched, never zero), a timeline of the ladder's transitions, and
// description lists and a table for lifecycle, in-flight work by class,
// the scratch census and the disk classes. What the report does not carry
// (per-tool latency: the surfaces and callers panels' `metrics traffic`)
// is not drawn here.

import (
	"context"
	"fmt"
	"html"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/viz"
)

// operateResourcesVerb is the verb the Tenancy panel renders.
const operateResourcesVerb = "fathom resources --json"

// operateResources is `fathom resources --url <this daemon> --json` over
// resourcesEnvNow. Inside the daemon its statusz is read in process: the
// panel never asks its own daemon over loopback (RT-115).
func operateResources(selfURL string) resourcesReport {
	env := resourcesEnvNow()
	if fetch := env.fetch; fetch != nil {
		env.fetch = func(u string) (*StatuszReport, error) {
			if r := inProcessDaemon(u); r != nil && r.statusz != nil {
				//fathom:tenancy-allow bg-ctx in-process statusz for the resources panel's shared background rebuild; no request owns it
				rep := r.statusz(context.Background())
				return &rep, nil
			}
			return fetch(u)
		}
	}
	return buildResources(env, "", selfURL, 24*time.Hour)
}

// operateResourcesTTL is how long the Tenancy panel's report is served
// before one flight refreshes it: the report forks a process census and
// walks ~/.fathom, whose cost grows with the files there, so it is kept
// longer than the model's TTL (its generated_at is shown with it).
const operateResourcesTTL = 2 * time.Minute

// operateResourcesBuilder builds the panel's report (tests hold it).
var operateResourcesBuilder = operateResources

var operateResourcesCache struct {
	sync.Mutex
	key      string
	rep      resourcesReport
	at       time.Time
	building bool
	done     chan struct{}
	gen      uint64 // bumped by a reset: an older flight stores nothing
}

// operateResourcesCached is operateResources kept for operateResourcesTTL:
// past it the last report is served while one flight, outside every lock,
// builds the next; only a cold cache waits for that flight.
func operateResourcesCached(selfURL string, now time.Time) resourcesReport {
	c := &operateResourcesCache
	c.Lock()
	defer c.Unlock()
	for attempt := 0; attempt < 3; attempt++ {
		warm := c.key == selfURL && !c.at.IsZero()
		if warm && now.Sub(c.at) < operateResourcesTTL && !now.Before(c.at) {
			return c.rep
		}
		if !c.building {
			c.building, c.done = true, make(chan struct{})
			//fathom:tenancy-allow bare-go one background rebuild at a time (c.building), signalled through c.done
			go refreshOperateResources(selfURL, now, c.gen, c.done)
		}
		if warm {
			return c.rep
		}
		done := c.done
		c.Unlock()
		<-done
		c.Lock()
		if c.key == selfURL && !c.at.IsZero() {
			return c.rep
		}
	}
	return resourcesReport{SchemaVersion: resourcesSchemaVersion, GeneratedAt: now.UTC().Format(time.RFC3339),
		Unreadable: []string{"the resources report could not be built in this process"}}
}

// refreshOperateResources is the one flight; a panicking build stores
// nothing and never takes the daemon down.
func refreshOperateResources(selfURL string, now time.Time, gen uint64, done chan struct{}) {
	var rep resourcesReport
	built := false
	defer func() {
		_ = recover()
		c := &operateResourcesCache
		c.Lock()
		if built && gen == c.gen {
			c.key, c.rep, c.at = selfURL, rep, now
		}
		c.building = false
		close(done)
		c.Unlock()
	}()
	rep = operateResourcesBuilder(selfURL)
	built = true
}

// resetOperateResources forgets the kept report (tests; the operate reset).
func resetOperateResources() {
	c := &operateResourcesCache
	c.Lock()
	c.key, c.at = "", time.Time{}
	c.gen++
	c.Unlock()
}

// toolHourTools bounds the tools the per-hour latency heatmap draws (the
// busiest; the table lists every row of the report).
const toolHourTools = 8

// renderToolHourLatency renders `metrics traffic --by tool-hour`: the
// busiest tools' p95 per UTC hour across the report's window, an hour with
// no call of a tool hatched (absent, never a fast zero), and every row as
// a table.
func renderToolHourLatency(r trafficResult) string {
	if r.Reason != "" {
		return `<p class="italic text-ink-muted">` + html.EscapeString(r.Reason) + `</p>`
	}
	type cell struct {
		calls int
		p95   *float64
	}
	byTool := map[string]map[string]cell{}
	calls := map[string]int{}
	for _, row := range r.Report.Rows {
		hour, tool, ok := strings.Cut(row.Group, " ")
		if !ok {
			continue
		}
		if byTool[tool] == nil {
			byTool[tool] = map[string]cell{}
		}
		byTool[tool][hour] = cell{calls: row.Calls, p95: row.P95MS}
		calls[tool] += row.Calls
	}
	if len(byTool) == 0 {
		return `<p class="italic text-ink-muted">no call in the last 24 h — the daemon answered none, or its log holds none</p>`
	}
	tools := make([]string, 0, len(byTool))
	for t := range byTool {
		tools = append(tools, t)
	}
	sort.Slice(tools, func(i, j int) bool {
		if calls[tools[i]] != calls[tools[j]] {
			return calls[tools[i]] > calls[tools[j]]
		}
		return tools[i] < tools[j]
	})
	if len(tools) > toolHourTools {
		tools = tools[:toolHourTools]
	}
	since, serr := time.Parse(time.RFC3339, r.Report.Since)
	until, uerr := time.Parse(time.RFC3339, r.Report.Until)
	var hours []string
	if serr == nil && uerr == nil {
		for h := since.UTC().Truncate(time.Hour); !h.After(until); h = h.Add(time.Hour) {
			hours = append(hours, h.Format("2006-01-02T15:00"))
		}
	}
	grid := make([][]float64, len(tools))
	states := make([][]viz.HeatmapCell, len(tools))
	var tRows []table.Row
	for i, tool := range tools {
		grid[i] = make([]float64, len(hours))
		states[i] = make([]viz.HeatmapCell, len(hours))
		for j, h := range hours {
			c, ok := byTool[tool][h]
			if !ok || c.p95 == nil {
				states[i][j] = viz.HeatmapCell{State: viz.CellMissing, Reason: "no call of this tool this hour"}
				continue
			}
			grid[i][j] = *c.p95
		}
	}
	for _, row := range r.Report.Rows {
		hour, tool, _ := strings.Cut(row.Group, " ")
		p95 := "—"
		if row.P95MS != nil {
			p95 = evMS(*row.P95MS)
		}
		tRows = append(tRows, table.Row{ID: "th-" + row.Group, Cells: []string{hour, tool, fmtThousands(row.Calls), p95}})
	}
	labels := make([]string, len(hours))
	for i, h := range hours {
		labels[i] = h[11:13]
	}
	svg := viz.Heatmap(grid, viz.HeatmapOptions{XLabels: labels, YLabels: tools, States: states, Cell: 22,
		AriaLabel: "p95 per tool per hour; hatched: no call of this tool this hour"})
	tbl := renderKitTable(table.TableParams{TableID: "operate-tool-hours", Caption: "calls and p95 per UTC hour and tool",
		Columns: []table.Column{{Key: "hour", Label: "Hour (UTC)"}, {Key: "tool", Label: "Tool"}, {Key: "calls", Label: "Calls", Align: "right"}, {Key: "p95", Label: "p95", Align: "right"}},
		Rows:    tRows})
	return `<p class="mb-1 text-sm text-ink-muted">p95 per tool per hour (UTC), the ` + strconv.Itoa(len(tools)) + ` busiest tools; hatched: no call of this tool this hour</p>` +
		evChart("p95 per tool per hour", svg, "min-w-[600px]", "every hour and tool as a table", tbl)
}

// renderTenancyPanel renders one resources report.
func renderTenancyPanel(rep resourcesReport) string {
	var b strings.Builder
	bytesOr := func(n int64) string {
		if n < 0 {
			return "unknown"
		}
		return humanBytes(n)
	}
	level := stat.Absent("ladder", "no tenancy block: the daemon did not answer and the ring holds no sample")
	if t := rep.Tenancy; t != nil {
		sub := "mode " + firstNonEmpty(t.Mode, "normal") + " · " + firstNonEmpty(t.Source, "live")
		if t.Since != "" {
			sub += " · since " + t.Since
		}
		level = stat.Stat{Label: "ladder", Value: t.Level, Sub: sub}
	}
	now := stat.Absent("footprint now", "no current sample")
	if rep.Footprint.Now >= 0 {
		total := rep.Footprint.Now
		if rep.Footprint.Children > 0 {
			total += rep.Footprint.Children
		}
		sub := "self + children"
		if rep.Footprint.SampleAge != "" {
			sub += " · sampled " + rep.Footprint.SampleAge + " ago"
		}
		now = stat.Stat{Label: "footprint now", Value: humanBytes(total), Sub: sub}
	}
	peak := stat.Absent("peak, "+rep.Footprint.Window, "no ring sample in the window")
	if rep.Footprint.Samples > 0 {
		peak = stat.Stat{Label: "peak, " + rep.Footprint.Window, Value: humanBytes(rep.Footprint.Max), Sub: "at " + rep.Footprint.MaxAt + " · " + rep.Footprint.MaxLevel}
	}
	budget := stat.Absent("budget", "no budget could be read")
	if rep.Budget.Budget > 0 {
		exit := humanBytes(rep.Budget.Exit)
		if !rep.Budget.ExitEnabled {
			exit += " (off)"
		}
		budget = stat.Stat{Label: "budget", Value: humanBytes(rep.Budget.Budget),
			Sub: "shed mark " + humanBytes(rep.Budget.Shed) + " · refuse mark " + humanBytes(rep.Budget.Refuse) + " · exit mark " + exit}
	}
	b.WriteString(stat.RenderBand([]stat.Stat{level, now, peak, budget}, stat.BandOptions{AriaLabel: "the tenancy budget"}))

	// The footprint per hour as a share of the budget: one truth-state row.
	if n := len(rep.Footprint.Hourly); n > 0 && rep.Budget.Budget > 0 {
		row := make([]float64, n)
		states := make([]viz.HeatmapCell, n)
		labels := make([]string, n)
		for i, p := range rep.Footprint.Hourly {
			labels[i] = strconv.Itoa(i - n + 1)
			if p < 0 {
				states[i] = viz.HeatmapCell{State: viz.CellMissing, Reason: "no sample this hour"}
				continue
			}
			row[i] = float64(p) * 100 / float64(rep.Budget.Budget)
		}
		ceil := 100.0
		if rep.Budget.Exit > 0 {
			ceil = float64(rep.Budget.Exit) * 100 / float64(rep.Budget.Budget)
		}
		svg := viz.Heatmap([][]float64{row}, viz.HeatmapOptions{XLabels: labels, YLabels: []string{"% of budget"}, States: [][]viz.HeatmapCell{states},
			Min: 0, Max: ceil, Cell: 22, AriaLabel: "footprint per hour, as a share of the budget; hatched: no sample this hour"})
		var rows []table.Row
		for i, p := range rep.Footprint.Hourly {
			v := "no sample this hour"
			if p >= 0 {
				v = humanBytes(p) + fmt.Sprintf(" (%.0f%%)", row[i])
			}
			rows = append(rows, table.Row{ID: "hour-" + labels[i], Cells: []string{labels[i] + " h", v}})
		}
		tbl := renderKitTable(table.TableParams{TableID: "operate-tenancy-hours", Caption: "peak footprint per hour",
			Columns: []table.Column{{Key: "hour", Label: "Hour (relative)"}, {Key: "peak", Label: "Peak footprint", Align: "right"}}, Rows: rows})
		b.WriteString(`<div class="mt-3"><p class="mb-1 text-sm text-ink-muted">footprint per hour, as a share of the budget (the exit mark is the ramp's top)</p>` +
			evChart("footprint per hour", svg, "min-w-[600px]", "every hour as a table", tbl) + `</div>`)
	} else {
		b.WriteString(`<p class="mt-3 italic text-ink-muted">no hourly footprint: ` + html.EscapeString(rep.Footprint.Ring) + `</p>`)
	}

	// The ladder's transitions.
	if len(rep.Ladder) > 0 {
		var evs []viz.Event
		for _, s := range rep.Ladder {
			at, err := time.Parse(time.RFC3339, s.At)
			if err != nil {
				continue
			}
			role := "positive"
			switch s.Level {
			case "shed":
				role = "caution"
			case "refuse", "exit":
				role = "danger"
			}
			evs = append(evs, viz.Event{At: at, Label: "→ " + s.Level, Role: role})
		}
		b.WriteString(`<div class="mt-3"><p class="mb-1 text-sm text-ink-muted">ladder transitions, ` + html.EscapeString(rep.Footprint.Window) + `</p>` +
			viz.Timeline(evs, viz.TimelineOptions{Fluid: true, AriaLabel: "ladder transitions"}) + `</div>`)
	} else {
		b.WriteString(`<p class="mt-3 text-sm text-ink-muted">ladder transitions: none in the window</p>`)
	}

	// Lifecycle, in flight, scratch, disk.
	l := rep.Lifecycle
	starts := strconv.Itoa(l.Starts24h)
	if !l.StartsRecorded {
		starts = "not recorded — " + l.NotRecorded
	}
	life := []chrome.DescItem{
		{Term: "starts in 24 h", Detail: starts},
		{Term: "unclean exits in 24 h", Detail: strconv.Itoa(l.Unclean24h)},
		{Term: "over-budget exits in 24 h", Detail: strconv.Itoa(l.OverBudget24h)},
		{Term: "switch", Detail: l.Switch},
	}
	if l.LastExit != "" {
		life = append(life, chrome.DescItem{Term: "last exit", Detail: l.LastExit})
	}
	if l.Uptime != "" {
		life = append(life, chrome.DescItem{Term: "uptime", Detail: l.Uptime})
	}
	inflight := "no tenancy block"
	if t := rep.Tenancy; t != nil {
		if len(t.InFlightByClass) == 0 {
			inflight = "none"
		} else {
			keys := make([]string, 0, len(t.InFlightByClass))
			for k := range t.InFlightByClass {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			parts := make([]string, 0, len(keys))
			for _, k := range keys {
				parts = append(parts, k+" "+strconv.Itoa(t.InFlightByClass[k]))
			}
			inflight = strings.Join(parts, " · ")
		}
	}
	scratch := "not taken"
	if rep.Scratch.Taken {
		scratch = fmt.Sprintf("%d unmanaged, %d leaked, %d kept, %d stale records", rep.Scratch.Unmanaged, rep.Scratch.Leaked, rep.Scratch.Kept, rep.Scratch.Stale)
		if rep.Scratch.Error != "" {
			scratch += " — " + rep.Scratch.Error
		}
	}
	callLog := bytesOr(rep.Disk.CallLogBytes)
	if rep.Disk.CallLog == "" {
		callLog = "no call log named by the daemon's record"
	}
	other := []chrome.DescItem{
		{Term: "in flight by class", Detail: inflight},
		{Term: "scratch daemons", Detail: scratch},
		{Term: "fathom home", Detail: bytesOr(rep.Disk.FathomHome)},
		{Term: "call log", Detail: callLog},
		{Term: "written by the daemon", Detail: bytesOr(rep.Disk.WrittenByDaemon)},
		{Term: "disk writes, 24 h", Detail: rep.Disk.Judgment},
	}
	b.WriteString(`<div class="mt-3 grid gap-4 md:grid-cols-2 [overflow-wrap:anywhere]">` +
		`<div>` + chrome.RenderDescriptionList(life) + `</div><div>` + chrome.RenderDescriptionList(other) + `</div></div>`)
	if len(rep.Scratch.Rows) > 0 {
		var items []string
		for _, r := range rep.Scratch.Rows {
			items = append(items, `<li>`+html.EscapeString(r)+`</li>`)
		}
		b.WriteString(`<ul class="mt-2 list-disc pl-5 text-sm text-caution" aria-label="leaked scratch daemons">` + strings.Join(items, "") + `</ul>`)
	}
	if len(rep.Reasons) > 0 {
		b.WriteString(`<p class="mt-3 text-sm text-caution">verdict: exit ` + strconv.Itoa(rep.Exit) + ` — ` + html.EscapeString(strings.Join(rep.Reasons, "; ")) + `</p>`)
	}
	return b.String()
}
