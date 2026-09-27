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
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
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
				rep := r.statusz(context.Background())
				return &rep, nil
			}
			return fetch(u)
		}
	}
	return buildResources(env, "", selfURL, 24*time.Hour)
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
