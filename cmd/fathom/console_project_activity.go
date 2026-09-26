package main

// console_project_activity.go — the workspace's Activity tab at
// /projects/{svc}/activity (vision 35 §3 "Activity: sessions and tool
// calls"; OC-39): one repository's tool calls from the RESOLVED call log —
// the log this daemon writes (OC-2), never a path relative to a working
// directory — and its sessions from the session ledger.
//
// The per-tool table is `fathom metrics traffic --service S --by tool` over
// the tab's window, computed by the verb's own function; the day strip, the
// recent calls and the sessions are console.ProjectActivity. The headline
// calls and sessions are the project's shared metrics (the model's one
// read), so this tab and the overview can never disagree about them — and
// the sessions panel lists the sessions of that same read under that same
// count.

import (
	"fmt"
	"html"
	"html/template"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/metrics"
)

const verbTrafficByTool = "fathom metrics traffic --service {svc} --by tool --from {from} --to {to} --json"

func init() {
	declarePage(consolePage{
		ID: "project-activity", Route: "/projects/{svc}/activity", Group: "Fleet", Order: 17, NavParent: "fleet",
		Title:    "Project activity",
		Question: "What did agents ask of this repository, and in which sessions?",
		Verbs:    []string{verbTrafficByTool},
		Shows:    []console.MetricID{console.MetricProjectCalls, console.MetricProjectSessions},
		Build:    buildProjectActivityPage,
	})
}

// projectActivityModel is the activity tab's JSON projection.
type projectActivityModel struct {
	Activity console.ProjectActivityModel `json:"activity"`
	Traffic  metrics.TrafficReport        `json:"traffic_by_tool"`
}

func buildProjectActivityPage(cr *consoleRequest) (consoleOut, error) {
	pc, err := loadProjectContext(cr)
	if err != nil {
		return consoleOut{}, err
	}
	out := pc.frame("Activity")
	out.Metrics = pc.metrics(cr.page)

	a := projectActivityFor(cr, pc)
	act, traffic, resolved := a.act, a.traffic, a.resolved
	verb := fillVerb(verbTrafficByTool, map[string]string{"svc": pc.svc, "from": a.from, "to": a.to})
	out.Verbs = []string{verb}
	out.Model = projectActivityModel{Activity: act, Traffic: traffic}
	out.Sections = map[string]any{verb: withCallLog(traffic, resolved)}
	metricsNow := out.Metrics
	out.Body = func() string {
		return renderProjectTabs(pc.svc, "activity") + renderProjectActivity(pc, act, traffic, verb, metricsNow)
	}
	return out, nil
}

// projectActivityData is one read of the call log and the ledger for a
// project's window.
type projectActivityData struct {
	act      console.ProjectActivityModel
	traffic  metrics.TrafficReport
	resolved console.ResolvedCallLog
	from, to string
	at       time.Time
}

// projectActivityTTL: the call log grows with every call — the console's
// own reads included — so a key on its size would never hit; one read per
// project and window is reused for 20 seconds instead, and the panels state
// when it was taken. The whole log is read twice per miss (the projection
// and the traffic verb's own reader), ~0.6 s on a 90,000-row log.
const projectActivityTTL = 20 * time.Second

// projectActivityMemo keeps recent reads (console_memo.go: one project's
// cold read never holds up another's kept one).
var projectActivityMemo = consoleMemo[projectActivityData]{max: 32}

// projectActivityKey names one read: the project, the window's day, the
// resolved call log and ledger, the model snapshot whose session count the
// tab shows (its sessions are listed from that snapshot's read), and the
// generations of the panels' live sources (index:{svc}, sessions:{svc}),
// so a refresh the "updated" affordance offers reads again.
func projectActivityKey(cr *consoleRequest, pc projectContext, src console.Sources) string {
	bus := cr.deps.bus
	return strings.Join([]string{pc.svc, cr.now.UTC().Format("2006-01-02"), src.CallLog.Path, src.SessionLedger, pc.m.GeneratedAt,
		strconv.FormatUint(bus.SourceGeneration("index:"+pc.svc), 10), strconv.FormatUint(bus.SourceGeneration("sessions:"+pc.svc), 10)}, "\x00")
}

func projectActivityFor(cr *consoleRequest, pc projectContext) projectActivityData {
	src := cr.deps.sources()
	d, _ := projectActivityMemo.do(projectActivityKey(cr, pc, src), cr.now, func() (projectActivityData, time.Duration) {
		return readProjectActivity(cr, pc, src), projectActivityTTL
	})
	return d
}

// readProjectActivity is one read of the call log for a project's window,
// with its sessions from the model's one read of the ledger: the count is
// the shared project.sessions metric and the rows are folded from the same
// bytes (OC-9 — the panel can never count differently from the band).
func readProjectActivity(cr *consoleRequest, pc projectContext, src console.Sources) projectActivityData {
	roots := make(map[string]string, len(pc.m.Services))
	for _, s := range pc.m.Services {
		roots[s.Service] = s.Path
	}
	d := projectActivityData{at: cr.now, resolved: src.CallLog}
	d.act = console.ProjectActivity(src, console.ActivityInput{
		Service: pc.svc, LabelKey: sessionLabelKey(roots),
		Sessions: console.ProjectSessions(pc.facts()), Ledger: pc.m.sessionLedger,
	})
	since, _ := time.Parse("2006-01-02", d.act.From)
	until, _ := time.Parse("2006-01-02", d.act.To)
	until = until.Add(24*time.Hour - time.Second)
	d.from, d.to = since.UTC().Format(time.RFC3339), until.UTC().Format(time.RFC3339)
	d.traffic, d.resolved = metricsTrafficReport("", pc.svc, metrics.TrafficGroupByTool, since, until)
	return d
}

func renderProjectActivity(pc projectContext, act console.ProjectActivityModel, traffic metrics.TrafficReport, verb string, shared console.Metrics) string {
	var b strings.Builder
	b.WriteString(stat.RenderBand([]stat.Stat{
		valueStat("calls (the whole log)", shared[console.MetricProjectCalls]),
		valueStat("calls in the window", act.WindowCalls),
		valueStat("errors in the window", act.Errors),
		valueStat("distinct sessions", shared[console.MetricProjectSessions]),
	}, stat.BandOptions{AriaLabel: pc.svc + " activity"}))
	if act.ConsoleReads > 0 {
		b.WriteString(`<p class="mt-1 text-xs text-ink-muted">` + html.EscapeString(fmtThousands(act.ConsoleReads)+" rows were the console reading "+pc.svc+" to render its pages — logged, never counted as usage.") + `</p>`)
	}

	b.WriteString(`<div class="mt-4 grid gap-4">` + renderActivityDaysPanel(pc, act) + renderActivityToolsPanel(pc, act, traffic, verb) + `</div>`)
	b.WriteString(`<div class="mt-4 grid gap-4 lg:grid-cols-2">` + renderActivityRecentPanel(pc, act) + renderActivitySessionsPanel(pc, act) + `</div>`)
	return projectBody(b.String())
}

// renderActivityDaysPanel is the window as a one-row heatmap with truth
// states — a day with no rows in a read log is a drawn zero, a log that
// could not be read hatches every day — and its table alternative.
func renderActivityDaysPanel(pc projectContext, act console.ProjectActivityModel) string {
	row := make([]float64, 0, len(act.Days))
	states := make([]viz.HeatmapCell, 0, len(act.Days))
	labels := make([]string, 0, len(act.Days))
	trows := make([]table.Row, 0, len(act.Days))
	for i, d := range act.Days {
		lbl := ""
		if i == 0 || i == len(act.Days)-1 || strings.HasSuffix(d.Date, "-01") {
			lbl = d.Date[5:]
		}
		labels = append(labels, lbl)
		cell := viz.HeatmapCell{}
		switch d.Calls.State {
		case console.StateAbsent:
			cell = viz.HeatmapCell{State: viz.CellMissing, Reason: d.Calls.Reason}
			row = append(row, math.NaN())
		case console.StateError:
			cell = viz.HeatmapCell{State: viz.CellError, Reason: d.Calls.Reason}
			row = append(row, math.NaN())
		case console.StateZero:
			cell = viz.HeatmapCell{State: viz.CellZero}
			row = append(row, 0)
		default:
			row = append(row, float64(d.Calls.V))
		}
		states = append(states, cell)
		trows = append(trows, table.Row{ID: "day-" + d.Date, Cells: []string{d.Date, valuePlain(d.Calls)}})
	}
	chart := viz.Heatmap([][]float64{row}, viz.HeatmapOptions{
		Cell: 32, Gap: 3, XLabels: labels, YLabels: []string{"calls"}, States: [][]viz.HeatmapCell{states},
		AriaLabel: pc.svc + " calls per day, " + act.From + " to " + act.To,
	})
	tbl := renderKitTable(table.TableParams{
		TableID: "project-activity-days", Caption: "calls per day, " + act.From + " … " + act.To + " (UTC)",
		Columns: []table.Column{{Key: "date", Label: "Date"}, {Key: "calls", Label: "Calls", Align: "right"}},
		Rows:    trows,
	})
	body := `<div class="overflow-x-auto" role="region" tabindex="0" aria-label="calls per day — scrolls sideways" data-activity-days>` + chart + `</div>` +
		`<div class="mt-3">` + chrome.RenderDisclosure(chrome.Disclosure{Summary: "Calls per day as a table", Body: template.HTML(tbl)}) + `</div>`
	return renderPanel(consolePanel{
		ID: "activity-days", Title: "Calls per day",
		Question: "When did agents query this repository? A hatched day was not measured; an empty one is a measured zero.",
		Source:   act.CallLog.Path + " (rule " + string(act.CallLog.Rule) + ")", Denominator: act.WindowCalls.Denominator,
		State: act.WindowCalls.State, Reason: act.WindowCalls.Reason, SourceKey: "index:" + pc.svc, Body: body,
	})
}

func renderActivityToolsPanel(pc projectContext, act console.ProjectActivityModel, traffic metrics.TrafficReport, verb string) string {
	var body strings.Builder
	state, reason := act.WindowCalls.State, act.WindowCalls.Reason
	if act.WindowCalls.Known() {
		if len(traffic.Rows) == 0 {
			body.WriteString(`<p class="text-ink-muted">0 calls in the window.</p>`)
		} else {
			rows := make([]table.Row, 0, len(traffic.Rows))
			for _, r := range traffic.Rows {
				p50, p95 := "—", "—"
				if r.P50MS != nil {
					p50 = fmt.Sprintf("%.0f ms", *r.P50MS)
				}
				if r.P95MS != nil {
					p95 = fmt.Sprintf("%.0f ms", *r.P95MS)
				}
				errRate := r.ErrorRateNote
				if r.ErrorRate != nil {
					errRate = fmt.Sprintf("%.1f%%", *r.ErrorRate)
				}
				if errRate == "" {
					errRate = "not recorded"
				}
				rows = append(rows, table.Row{ID: "tool-" + r.Group, Cells: []string{r.Group, fmtThousands(r.Calls), p50, p95, errRate}})
			}
			body.WriteString(renderKitTable(table.TableParams{
				TableID: "project-activity-tools", Caption: fmt.Sprintf("%s calls by tool (%d discovery, %d plumbing, %d other)", fmtThousands(traffic.TotalCalls), traffic.DiscoveryCalls, traffic.PlumbingCalls, traffic.OtherCalls),
				Columns: []table.Column{{Key: "tool", Label: "Tool"}, {Key: "calls", Label: "Calls", Align: "right"}, {Key: "p50", Label: "p50", Align: "right"}, {Key: "p95", Label: "p95", Align: "right"}, {Key: "err", Label: "Error rate", Align: "right"}},
				Rows:    rows,
			}))
			body.WriteString(`<p class="mt-2 text-xs text-ink-muted">` + html.EscapeString(traffic.Note) + `</p>`)
		}
	}
	return renderPanel(consolePanel{
		ID: "activity-tools", Title: "Tools used here",
		Question: "Which tools did agents call against this repository, how fast, and how often did they fail?",
		Verb:     verb, Source: "the resolved call log (" + string(act.CallLog.Rule) + "): " + act.CallLog.Path,
		Denominator: "calls attributed to " + pc.svc + " from " + traffic.Since + " to " + traffic.Until,
		State:       state, Reason: reason, SourceKey: "index:" + pc.svc, Body: body.String(),
	})
}

func renderActivityRecentPanel(pc projectContext, act console.ProjectActivityModel) string {
	var body strings.Builder
	if act.WindowCalls.Known() {
		if len(act.Recent) == 0 {
			body.WriteString(`<p class="text-ink-muted">No call in the window.</p>`)
		} else {
			rows := make([]table.Row, 0, len(act.Recent))
			for i, c := range act.Recent {
				surface := c.Surface
				if surface == "" {
					surface = "unscoped"
				}
				outcome := "ok"
				if c.Error != "" {
					outcome = "error: " + c.Error
				}
				rows = append(rows, table.Row{ID: fmt.Sprintf("call-%d", i), Cells: []string{activityStamp(c.At), c.Tool, surface, fmt.Sprintf("%.0f ms", c.LatencyMS), outcome}})
			}
			body.WriteString(renderKitTable(table.TableParams{
				TableID: "project-activity-recent", Caption: fmt.Sprintf("the newest %d calls, newest first", len(act.Recent)),
				Columns: []table.Column{{Key: "at", Label: "At (UTC)"}, {Key: "tool", Label: "Tool"}, {Key: "surface", Label: "Surface"}, {Key: "latency", Label: "Latency", Align: "right"}, {Key: "outcome", Label: "Outcome"}},
				Rows:    rows,
			}))
		}
	}
	return renderPanel(consolePanel{
		ID: "activity-recent", Title: "Recent calls",
		Question: "What were the latest questions asked of this repository?",
		Source:   "the resolved call log: " + act.CallLog.Path, Denominator: fmt.Sprintf("the newest of the window's calls, at most %d", 20),
		State: act.WindowCalls.State, Reason: act.WindowCalls.Reason, SourceKey: "index:" + pc.svc, Body: body.String(),
	})
}

func renderActivitySessionsPanel(pc projectContext, act console.ProjectActivityModel) string {
	var body strings.Builder
	if act.Sessions.Known() {
		if len(act.SessionRows) == 0 {
			body.WriteString(`<p class="text-ink-muted">No session in the ledger names this repository.</p>`)
		} else {
			rows := make([]table.Row, 0, len(act.SessionRows))
			for _, s := range act.SessionRows {
				id := s.ID
				link := templ.Raw(html.EscapeString("(no session id)"))
				if id != "" {
					link = templ.Raw(`<a class="inline-flex min-h-11 items-center font-mono text-xs text-accent hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus" href="` + html.EscapeString(sessionHref(id)) + `">` + html.EscapeString(shortSessionID(id)) + `</a>`)
				}
				rows = append(rows, table.Row{ID: "session-" + id, Cells: []string{id, s.Arm, activityStamp(s.At), intPtrText(s.Turns), intPtrText(s.FirstEditTurn), intPtrText(s.FathomCalls), fmtThousands(s.Rows)},
					HTMLCells: []templ.Component{link, nil, nil, nil, nil, nil, nil}})
			}
			body.WriteString(renderKitTable(table.TableParams{
				TableID: "project-activity-sessions", Caption: fmt.Sprintf("the newest %d of %s distinct sessions", len(act.SessionRows), fmtThousands(act.Sessions.V)),
				Columns: []table.Column{{Key: "id", Label: "Session"}, {Key: "arm", Label: "Arm"}, {Key: "at", Label: "Last row (UTC)"}, {Key: "turns", Label: "Turns", Align: "right"}, {Key: "edit", Label: "First edit", Align: "right"}, {Key: "calls", Label: "fathom calls", Align: "right"}, {Key: "rows", Label: "Ledger rows", Align: "right"}},
				Rows:    rows,
			}))
		}
	}
	return renderPanel(consolePanel{
		ID: "activity-sessions", Title: "Sessions",
		Question: "Which agent sessions worked in this repository? One row per session, its newest ledger row.",
		Source:   act.Sessions.Source, Denominator: act.Sessions.Denominator,
		State: act.Sessions.State, Reason: act.Sessions.Reason, SourceKey: "sessions:" + pc.svc, Body: body.String(),
	})
}

// intPtrText renders an optional count: the number, or "not recorded".
func intPtrText(p *int) string {
	if p == nil {
		return "not recorded"
	}
	return fmtThousands(*p)
}

// activityStamp renders an RFC3339 UTC time as "09-26 03:17" — the year and
// seconds cost a table a column's width at 390 px and add nothing to a
// two-week window; an unparsable value is shown as recorded.
func activityStamp(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	return t.UTC().Format("01-02 15:04")
}
