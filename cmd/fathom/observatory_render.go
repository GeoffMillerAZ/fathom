package main

// observatory_render.go — the HTML rendering half of vision 17, built on
// app-kit (D13, kits-first): dashboard.RenderWidget/Grid for layout,
// stat.RenderBand for KPIs, viz.Gauge for shares — the licensed, indexed
// components, never hand-rolled markup. CSS is built by the checksum-pinned
// Tailwind binary (`task ui:css`) from cmd/fathom/ui/input.css and embedded;
// role utilities re-theme light/dark through the kit tokens with no
// JavaScript. The honest-absence discipline stays: metricCell renders
// value-or-named-reason (law 3) and a nil metric never becomes a zero.

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/ui/dashboard"
	"github.com/geoffmilleraz/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
)

//go:embed ui/app.css
var obsAppCSS []byte

// metricCell renders value-or-named-reason (OB-2, law 3). A nil value
// renders absentNote (never "0"); a present value renders with its
// presentNote — the denominator stays visible NEXT TO the number.
func metricCell(v *int, absentNote, presentNote string) string {
	if v != nil {
		return `<span class="text-xl font-semibold text-ink tabular-nums">` + fmtThousands(*v) + `</span>` + dimNote(presentNote)
	}
	if absentNote == "" {
		absentNote = "not measured"
	}
	return `<span class="italic text-ink-faint">` + html.EscapeString(absentNote) + `</span>`
}

func dimNote(note string) string {
	if note == "" {
		return ""
	}
	return ` <span class="text-ink-muted">(` + html.EscapeString(note) + `)</span>`
}

// fmtThousands renders 1234567 as "1,234,567" — big real numbers stay
// readable without becoming approximations.
func fmtThousands(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	out := strings.Join(parts, ",")
	if neg {
		out = "-" + out
	}
	return out
}

// plural returns "1 call" / "2 calls" — no "1 calls" grammar.
func plural(n int, word string) string {
	if n == 1 {
		return fmtThousands(n) + " " + word
	}
	return fmtThousands(n) + " " + word + "s"
}

// shortCommit trims a hash for display.
func shortCommit(c string) string {
	if len(c) > 8 {
		return c[:8]
	}
	return c
}

// shortFreshness compacts "stale: indexed <full> vs HEAD <full>" for tile
// rows; the service page keeps the full form.
func shortFreshness(f string) string {
	if !strings.HasPrefix(f, "stale: indexed ") {
		return f
	}
	rest := strings.TrimPrefix(f, "stale: indexed ")
	parts := strings.SplitN(rest, " vs HEAD ", 2)
	if len(parts) != 2 {
		return f
	}
	return "stale (" + shortCommit(parts[0]) + " → " + shortCommit(parts[1]) + ")"
}

// kv renders one label/value row inside a widget.
func kv(k, v string) string {
	return `<div class="flex justify-between gap-2 py-0.5"><span class="text-ink-muted">` + html.EscapeString(k) + `</span><span class="text-right">` + v + `</span></div>`
}

// remedy renders the exact command that fixes a warning — the page is a
// to-do list, not a mood board.
func remedy(cmd string) string {
	return `<div class="mt-2 rounded-md bg-surface-sunken px-2 py-1 font-mono text-xs text-caution break-all">$ ` + html.EscapeString(cmd) + `</div>`
}

// obsLead is a page's plain-English framing (EX-5): one sentence a
// developer who has never heard of fathom can read, plus a no-JS
// "what am I looking at?" explainer.
type obsLead struct {
	Lead    string
	Explain string
}

var obsLeads = map[string]obsLead{
	"home":   {"fathom is a code index your AI coding agents query instead of re-reading the codebase. This page answers: is it wired, is it used, and is it paying off?", "Agents normally rediscover a codebase every session by reading files and grepping — that costs tokens and time, and every subagent repeats it. fathom indexes the code once and answers where/what/who-calls questions directly. The four cards summarize what that is worth; each links to its evidence."},
	"fleet":  {"Every repo fathom indexes, how well each is wired, and the exact command that fixes anything amber.", "Each tile is one repository. The letter is a wiring grade over the signals visible from the repo itself (hook wiring, docs stanza, index freshness, recorded traffic — signals that cannot be checked from here are excluded rather than counted against it). Sparklines show two weeks of query traffic. Amber monospace lines are copy-paste remedies."},
	"impact": {"What fathom saved, prevented, and where it still falls short — every number names its instrument and its denominator.", "The exploration story compares what a discovery question costs when answered by raw file reading versus the index, measured inside the same sessions with the same estimator. 'Verify catches' are pieces of work an agent claimed finished that the verification gate proved were not. 'Obligations served' are project rules injected right before an agent edits a governed file. A dash with a note means the instrument has not reported — this dashboard never renders a number it did not measure."},
	"story":  {"The last 7 days composed from the instruments, with per-session replays.", "Every session an agent runs leaves a ledger row; this page folds the recent ones and links each session's replay timeline. Narrative here is a lens for humans — the evidence lives in the instruments."},
	"brief":  {"The shareable summary — same numbers, framed for someone who has never heard of fathom.", "This is the page to show your manager. It is generated from the live instruments, states what is NOT measured, and can be downloaded as markdown. Nothing on it is ever sent anywhere by fathom — sharing is a human decision."},
}

func obsPage(title, active string, body string, m observatoryModel) string {
	var b strings.Builder
	b.WriteString(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>` + html.EscapeString(title) + `</title><link rel="icon" href="data:image/svg+xml,<svg xmlns=%22http://www.w3.org/2000/svg%22 viewBox=%220 0 100 100%22><text y=%22.9em%22 font-size=%2290%22>🔱</text></svg>"><link rel="stylesheet" href="/assets/app.css"></head>`)
	b.WriteString(`<body class="min-h-screen bg-surface p-5 font-sans text-sm text-ink">`)
	b.WriteString(`<h1 class="text-lg font-semibold">fathom observatory</h1>`)
	b.WriteString(`<nav class="mb-4 mt-1 flex gap-4">`)
	for _, p := range [][2]string{{"/", "home"}, {"/observatory", "fleet"}, {"/observatory/impact", "impact"}, {"/observatory/story", "story"}, {"/observatory/brief", "brief"}, {"/statusz", "statusz"}} {
		cls := "text-accent no-underline"
		if p[1] == active {
			cls += " font-semibold"
		}
		b.WriteString(`<a class="` + cls + `" href="` + p[0] + `">` + p[1] + `</a>`)
	}
	b.WriteString(`</nav>`)
	if l, ok := obsLeads[active]; ok {
		b.WriteString(`<p class="mb-2 max-w-3xl text-ink-muted">` + html.EscapeString(l.Lead) + `</p>`)
		if l.Explain != "" {
			b.WriteString(`<details class="mb-3 max-w-3xl rounded-md border border-line bg-surface-raised px-3 py-2"><summary class="cursor-pointer text-accent">what am I looking at?</summary><p class="mt-2 text-ink-muted">` + html.EscapeString(l.Explain) + `</p></details>`)
		}
	}
	banner := func(active bool, detail string) {
		cls := "mb-2 rounded-md border border-line bg-surface-raised px-3 py-2"
		if active {
			cls = "mb-2 rounded-md border border-caution bg-caution-soft px-3 py-2"
		}
		b.WriteString(`<div class="` + cls + `">` + html.EscapeString(detail) + `</div>`)
	}
	banner(m.Arm.Active, m.Arm.Detail)
	banner(m.Narrate.Active, m.Narrate.Detail)
	if m.DegradedNote != "" {
		banner(true, m.DegradedNote)
	}
	b.WriteString(body)
	b.WriteString(`<footer class="mt-8 text-xs text-ink-muted">generated ` + html.EscapeString(m.GeneratedAt) + ` · fathom ` + html.EscapeString(m.FathomVersion) + ` · loopback-only; sharing is a human verb (law 13) — <code class="font-mono">fathom brief</code> writes the shareable file</footer></body></html>`)
	return b.String()
}

// sectionHeading renders a muted uppercase section label.
func sectionHeading(s string) string {
	return `<h2 class="mb-2 mt-6 text-xs font-semibold uppercase tracking-wider text-ink-muted">` + html.EscapeString(s) + `</h2>`
}

func renderObsFleet(m observatoryModel) string {
	totalSymbols, totalCalls, openObs, obsKnown, staleCount := 0, 0, 0, 0, 0
	for _, s := range m.Services {
		totalSymbols += s.Symbols
		totalCalls += s.CallsTotal
		if s.OpenObservations != nil {
			openObs += *s.OpenObservations
			obsKnown++
		}
		if strings.HasPrefix(s.Freshness, "stale") {
			staleCount++
		}
	}
	freshRole, freshVal := "positive", "all fresh"
	if staleCount > 0 {
		freshRole, freshVal = "caution", plural(staleCount, "stale repo")
	}
	var b strings.Builder
	b.WriteString(stat.RenderBand([]stat.Stat{
		{Label: "services indexed", Value: fmtThousands(len(m.Services))},
		{Label: "symbols", Value: fmtThousands(totalSymbols)},
		{Label: "tool calls (log)", Value: fmtThousands(totalCalls)},
		{Label: "index freshness", Value: freshVal, ValueRole: freshRole},
		{Label: "open observations", Value: fmtThousands(openObs), Sub: "across " + plural(obsKnown, "repo") + " with a backlog"},
	}, stat.BandOptions{AriaLabel: "fleet totals"}))

	if len(m.TrafficDays) > 0 {
		var spark []float64
		hasTraffic := false
		for _, d := range m.TrafficDays {
			spark = append(spark, float64(d.Calls))
			if d.Calls > 0 {
				hasTraffic = true
			}
		}
		if hasTraffic {
			b.WriteString(sectionHeading("traffic — last 14 days (call log)"))
			b.WriteString(dashboard.RenderWidget(dashboard.Widget{
				Body: template.HTML(viz.Sparkline(spark, viz.SparklineOptions{W: 640, H: 48, FillRole: "accent-soft"})),
			}))
		}
	}

	var widgets []string
	for _, s := range m.Services {
		var body strings.Builder
		body.WriteString(kv("symbols", `<span class="font-semibold tabular-nums">`+fmtThousands(s.Symbols)+`</span>`))
		fr := `<span class="text-positive">fresh</span>`
		if strings.HasPrefix(s.Freshness, "stale") {
			fr = `<span class="text-caution">` + html.EscapeString(shortFreshness(s.Freshness)) + `</span>`
		} else if s.Freshness != "fresh" {
			fr = `<span class="italic text-ink-faint">` + html.EscapeString(s.Freshness) + `</span>`
		}
		body.WriteString(kv("freshness", fr))
		body.WriteString(kv("traffic", html.EscapeString(plural(s.CallsTotal, "call"))+` <span class="text-ink-muted">(`+fmtThousands(s.CallsAgent)+` agent)</span>`))
		body.WriteString(kv("open observations", metricCell(s.OpenObservations, "no observations file", "")))
		if len(s.TrafficDays) > 0 && s.CallsTotal > 0 {
			spark := make([]float64, len(s.TrafficDays))
			for i, c := range s.TrafficDays {
				spark[i] = float64(c)
			}
			body.WriteString(`<div class="mt-2">` + viz.Sparkline(spark, viz.SparklineOptions{W: 220, H: 24, FillRole: "accent-soft", AriaLabel: s.Service + " 14-day traffic"}) + `</div>`)
		}
		if s.FreshnessRemedy != "" {
			body.WriteString(remedy(s.FreshnessRemedy))
		}
		if s.WiringRemedy != "" {
			body.WriteString(remedy(s.WiringRemedy))
		}
		gradeRole := map[string]string{"A": "positive", "B": "positive", "C": "caution", "D": "caution", "F": "danger"}[s.Grade]
		gradeChip := `<span class="mr-2 inline-block rounded-full bg-` + gradeRole + `-soft px-2 py-0.5 text-xs font-semibold text-` + gradeRole + `" title="wiring grade — the continuous rewire audit">` + html.EscapeString(s.Grade) + `</span>`
		widgets = append(widgets, dashboard.RenderWidget(dashboard.Widget{
			Title:  s.Service,
			Action: template.HTML(gradeChip + `<a class="text-accent" href="/observatory/service?id=` + html.EscapeString(s.Service) + `">detail →</a>`),
			Body:   template.HTML(body.String()),
		}))
	}
	if len(m.Misses) > 0 {
		bars := make([]viz.Bar, 0, 8)
		for i, r := range m.Misses {
			if i == 8 {
				break
			}
			bars = append(bars, viz.Bar{Label: r.Tool, Value: float64(r.Calls)})
		}
		b.WriteString(sectionHeading("which tools carry the load (call log, all time)"))
		b.WriteString(dashboard.RenderWidget(dashboard.Widget{
			Body: template.HTML(viz.Bars(bars, viz.BarsOptions{W: 560, LabelW: 160, AriaLabel: "calls per tool"})),
		}))
	}

	b.WriteString(sectionHeading("fleet (" + plural(len(m.Services), "service") + ")"))
	b.WriteString(dashboard.RenderWidgetGrid(widgets, dashboard.WidgetGridOptions{Cols: 3, AriaLabel: "fleet services"}))
	return b.String()
}

func renderObsService(m observatoryModel, id string) (string, bool) {
	for _, s := range m.Services {
		if s.Service != id {
			continue
		}
		var body strings.Builder
		body.WriteString(kv("path", `<span class="font-mono text-xs">`+html.EscapeString(s.Path)+`</span>`))
		body.WriteString(kv("symbols", fmtThousands(s.Symbols)))
		body.WriteString(kv("freshness", html.EscapeString(s.Freshness)))
		body.WriteString(kv("indexed commit", `<span class="font-mono text-xs">`+html.EscapeString(s.IndexedCommit)+`</span>`))
		body.WriteString(kv("repo HEAD", `<span class="font-mono text-xs">`+html.EscapeString(s.RepoHead)+`</span>`))
		body.WriteString(kv("hooks", html.EscapeString(s.HooksNote)))
		body.WriteString(kv("stanza", html.EscapeString(s.StanzaNote)))
		body.WriteString(kv("traffic", plural(s.CallsTotal, "call")+` <span class="text-ink-muted">(`+fmtThousands(s.CallsAgent)+` agent · `+fmtThousands(s.CallsAgent)+`/`+fmtThousands(s.CallsTotal)+`)</span>`))
		body.WriteString(kv("open observations", metricCell(s.OpenObservations, "no observations file", "")))
		if s.FreshnessRemedy != "" {
			body.WriteString(remedy(s.FreshnessRemedy))
		}
		if s.WiringRemedy != "" {
			body.WriteString(remedy(s.WiringRemedy))
		}
		var cfg strings.Builder
		if s.Config != nil {
			c := s.Config
			if len(c.Layers) > 0 {
				for _, l := range c.Layers {
					state := `<span class="text-caution">installed, not eval-gated (not yet bindable)</span>`
					if l.Evaled {
						state = `<span class="text-positive">eval-gated</span>`
					}
					cfg.WriteString(kv("layer · "+l.Name, state))
				}
			} else {
				cfg.WriteString(kv("semantic layers", `<span class="italic text-ink-faint">`+html.EscapeString(c.LayersNote)+`</span>`))
			}
			if c.GuardCount > 0 {
				cfg.WriteString(kv("guards", fmtThousands(c.GuardCount)+` pre-edit warnings configured`))
			} else {
				cfg.WriteString(kv("guards", `<span class="italic text-ink-faint">`+html.EscapeString(c.GuardNote)+`</span>`))
			}
			cfg.WriteString(kv("bindings", html.EscapeString(c.BindingNote)))
			cfg.WriteString(kv("rules served here (calls)", fmtThousands(c.ObligationServes)))
			cfg.WriteString(kv("obligations compile", `<span class="text-xs">`+html.EscapeString(s.CompileNote)+`</span>`))
		}
		widgets := []string{
			dashboard.RenderWidget(dashboard.Widget{Title: id, Body: template.HTML(body.String())}),
			dashboard.RenderWidget(dashboard.Widget{Title: "configuration (layers · guards · bindings)", Body: template.HTML(cfg.String())}),
		}
		if s.CallsTotal > 0 {
			g := viz.Gauge(float64(s.CallsAgent), float64(s.CallsTotal), viz.GaugeOptions{
				Label:     "of this service's recorded calls arrived over the agent surface",
				AriaLabel: "agent-surface share",
			})
			widgets = append(widgets, dashboard.RenderWidget(dashboard.Widget{Title: "agent-surface share", Body: template.HTML(g)}))
		} else {
			widgets = append(widgets, dashboard.RenderWidget(dashboard.Widget{Title: "agent-surface share", Body: template.HTML(`<span class="italic text-ink-faint">no recorded calls for this service in the current log</span>`)}))
		}
		return sectionHeading(id) + dashboard.RenderWidgetGrid(widgets, dashboard.WidgetGridOptions{Cols: 2, AriaLabel: "service detail"}), true
	}
	return "", false
}

// impactStat converts an honest metric into a kit stat tile: a present
// value keeps its denominator as the Sub caption; an absent instrument
// renders "—" with the named reason (law 3) — never a zero.
func impactStat(label string, v *int, note string) stat.Stat {
	if v != nil {
		return stat.Stat{Label: label, Value: fmtThousands(*v), Sub: note}
	}
	if note == "" {
		note = "not measured"
	}
	return stat.Stat{Label: label, Value: "—", ValueRole: "ink-faint", Sub: note}
}

func renderObsImpact(m observatoryModel) string {
	imp := m.Impact
	var b strings.Builder

	// EX-4 — the exploration story leads: the owner's 90x, measured.
	b.WriteString(sectionHeading("the exploration story — what a question costs, with and without the index"))
	if imp.ExpSessions > 0 && imp.ExpRawCalls > 0 && imp.ExpFathomCalls > 0 {
		rawPerQ := imp.ExpRawTokens / imp.ExpRawCalls
		idxPerQ := imp.ExpFathomTokens / imp.ExpFathomCalls
		ratio := "—"
		if imp.ExpRatioMedianX10 != nil {
			ratio = fmtThousands(*imp.ExpRatioMedianX10/10) + "." + strconv.Itoa(*imp.ExpRatioMedianX10%10) + "×"
		}
		b.WriteString(stat.RenderBand([]stat.Stat{
			{Label: "raw exploration, per question", Value: "~" + fmtThousands(rawPerQ) + " tok", Sub: "Grep/Read/Glob results, " + fmtThousands(imp.ExpRawCalls) + " calls", ValueRole: "caution"},
			{Label: "indexed discovery, per question", Value: "~" + fmtThousands(idxPerQ) + " tok", Sub: "fathom answers, " + fmtThousands(imp.ExpFathomCalls) + " calls", ValueRole: "positive"},
			{Label: "observed ratio (median)", Value: ratio, Sub: "per-session, both modes present"},
			{Label: "duplicated exploration", Value: "~" + fmtThousands(imp.ExpRepeatTokens) + " tok", Sub: fmtThousands(imp.ExpRepeatFiles) + " files re-read — a shared index answers once"},
		}, stat.BandOptions{AriaLabel: "exploration story"}))
	} else {
		b.WriteString(dashboard.RenderWidget(dashboard.Widget{Body: template.HTML(`<span class="italic text-ink-faint">` + html.EscapeString(imp.ExpNote) + `</span>`)}))
	}
	b.WriteString(`<div class="mb-4 mt-2 text-xs text-ink-muted">` + html.EscapeString(imp.ExpNote) + `</div>`)

	// Q2 — Prevented.
	b.WriteString(sectionHeading("Q2 · what did fathom prevent?"))
	b.WriteString(stat.RenderBand([]stat.Stat{
		impactStat("verify catches", imp.VerifyCatchesTotal, imp.VerifyCatchesNote),
		{Label: "obligations served (calls)", Value: fmtThousands(imp.ObligationCalls), Sub: "rules injected before edits; conformity checking is IQ-13"},
		{Label: "ledger sessions", Value: fmtThousands(imp.LedgerSessions), Sub: imp.DateRange},
	}, stat.BandOptions{AriaLabel: "prevented"}))

	// Q1 — Saved (law 12: arms or darkness).
	b.WriteString(sectionHeading("Q1 · what did the agent not have to do? (law 12: a savings figure exists only as an arm range)"))
	var card strings.Builder
	card.WriteString(kv("state", `<span class="font-semibold">`+html.EscapeString(imp.PilotState)+`</span>`))
	card.WriteString(kv("ON arm", plural(imp.TokensOnSessions, "session")+` · median tokens `+metricCell(imp.TokensOnMedian, "no token totals", "")))
	card.WriteString(kv("OFF arm", plural(imp.TokensOffSessions, "session")+` · median tokens `+metricCell(imp.TokensOffMedian, "no token totals", "")))
	card.WriteString(kv("discovery actions (with fathom)", metricCell(imp.DiscoveryTotal, imp.DiscoveryNote, imp.DiscoveryNote)))
	card.WriteString(`<div class="mt-2 text-ink-muted">` + html.EscapeString(imp.PilotDetail) + `</div>`)
	b.WriteString(dashboard.RenderWidget(dashboard.Widget{Body: template.HTML(card.String())}))

	// Q3 — Friction (the flywheel's fuel).
	b.WriteString(sectionHeading("Q3 · where does the agent still flounder? (misses are the roadmap)"))
	if len(m.Misses) > 0 {
		var tbl strings.Builder
		tbl.WriteString(`<div class="overflow-x-auto"><table class="w-full border-collapse text-sm"><tr class="border-b border-line text-left"><th class="px-2 py-1 font-medium text-ink-muted">tool</th><th class="px-2 py-1 font-medium text-ink-muted">calls</th><th class="px-2 py-1 font-medium text-ink-muted">hits</th><th class="px-2 py-1 font-medium text-ink-muted">misses</th><th class="px-2 py-1 font-medium text-ink-muted">did-you-mean</th><th class="px-2 py-1 font-medium text-ink-muted">outcome not recorded</th></tr>`)
		for i, r := range m.Misses {
			if i == 10 {
				break
			}
			tbl.WriteString(`<tr class="border-b border-line"><td class="px-2 py-1 font-mono text-xs">` + html.EscapeString(r.Tool) + `</td><td class="px-2 py-1 tabular-nums">` + fmtThousands(r.Calls) + `</td><td class="px-2 py-1 tabular-nums text-positive">` + fmtThousands(r.Hits) + `</td><td class="px-2 py-1 tabular-nums text-danger">` + fmtThousands(r.Misses) + `</td><td class="px-2 py-1 tabular-nums text-caution">` + fmtThousands(r.DidYouMean) + `</td><td class="px-2 py-1 tabular-nums text-ink-faint">` + fmtThousands(r.NotRecorded) + `</td></tr>`)
		}
		tbl.WriteString(`</table></div><div class="mt-2 text-xs text-ink-muted">rows logged before the outcome instrument (IQ-1) count as not-recorded, never as hits. Query text stays out of the log by design — it is user-global and metadata-only.</div>`)
		b.WriteString(dashboard.RenderWidget(dashboard.Widget{Title: "per-tool outcomes", Body: template.HTML(tbl.String())}))
	} else {
		b.WriteString(dashboard.RenderWidget(dashboard.Widget{Body: template.HTML(`<span class="italic text-ink-faint">no call-log rows readable from here</span>`)}))
	}
	b.WriteString(dashboard.RenderWidget(dashboard.Widget{Title: "friction filed by hand", Body: template.HTML(kv("open observations across the fleet", fmtThousands(fleetOpenObservations(m))+` <span class="text-ink-muted">(each is a bug spec with a repro — the backlog that improves fathom)</span>`))}))

	// Q4 — Compounding.
	b.WriteString(sectionHeading("Q4 · is the knowledge compounding?"))
	b.WriteString(stat.RenderBand([]stat.Stat{
		impactStat("median warm-up (turns to first edit)", imp.FirstEditMedian, imp.FirstEditNote),
		{Label: "obligations served", Value: fmtThousands(imp.ObligationCalls), Sub: "growth curve arrives with weekly rollups"},
	}, stat.BandOptions{AriaLabel: "compounding"}))

	b.WriteString(sectionHeading("stories"))
	b.WriteString(`<div class="rounded-md border border-dashed border-caution px-3 py-2 text-ink-muted">narrative is a lens, never evidence (law 14). ` + html.EscapeString(imp.StoriesNote) + ` <a class="text-accent" href="/observatory/story">the week →</a></div>`)
	if imp.LedgerNote != "" {
		b.WriteString(`<div class="mt-2 rounded-md border border-caution bg-caution-soft px-3 py-2">` + html.EscapeString(imp.LedgerNote) + `</div>`)
	}
	return b.String()
}

// ─── handlers (OB-4/5/6) ─────────────────────────────────────────────────

func observatoryHandler(baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		m := cachedObservatoryModel(baseURL, time.Now())
		if wantsJSON(r) {
			writeObsJSON(w, m)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(obsPage("fathom observatory", "fleet", renderObsFleet(m), m)))
	}
}

func observatoryServicePage(baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := r.URL.Query().Get("id")
		m := cachedObservatoryModel(baseURL, time.Now())
		body, ok := renderObsService(m, id)
		if !ok {
			http.Error(w, "unknown service "+id, http.StatusNotFound)
			return
		}
		if wantsJSON(r) {
			for _, s := range m.Services {
				if s.Service == id {
					writeObsJSON(w, s)
					return
				}
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(obsPage("fathom · "+id, "fleet", body, m)))
	}
}

func observatoryImpactPage(baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		m := cachedObservatoryModel(baseURL, time.Now())
		if wantsJSON(r) {
			writeObsJSON(w, m.Impact)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(obsPage("fathom · impact", "impact", renderObsImpact(m), m)))
	}
}

// assetsHandler serves the embedded, kit-built stylesheet.
func assetsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/assets/app.css" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write(obsAppCSS)
	}
}

// landingHandler serves GET / — the front door (owner ask 2026-08-08):
// every browsable and machine surface the daemon offers, one screen, no
// dead root. Unknown paths still 404.
func landingHandler(baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		m := cachedObservatoryModel(baseURL, time.Now())
		imp := m.Impact
		totalCalls := 0
		for _, svc := range m.Services {
			totalCalls += svc.CallsTotal
		}
		saved := "dark until OFF-arm sessions exist"
		if imp.TokensOffSessions > 0 {
			saved = plural(imp.TokensOffSessions, "OFF-arm session") + " recorded"
		}
		q := func(n, question, answer, sub, href string) string {
			return dashboard.RenderWidget(dashboard.Widget{
				Title: n + " · " + question,
				Body:  template.HTML(`<div class="text-xl font-semibold text-ink">` + html.EscapeString(answer) + `</div><div class="mt-1 text-ink-muted">` + html.EscapeString(sub) + `</div><div class="mt-2"><a class="text-accent" href="` + href + `">see the evidence →</a></div>`),
			})
		}
		questions := []string{
			q("Q1", "what did the agent not have to do?", saved, "a savings figure exists only as a counterfactual arm range (law 12)", "/observatory/impact"),
			q("Q2", "what did fathom prevent?", fmtThousands(intOrZero(imp.VerifyCatchesTotal))+" verify catches", imp.VerifyCatchesNote, "/observatory/impact"),
			q("Q3", "where does the agent still flounder?", fmtThousands(fleetOpenObservations(m))+" open observations", "plus per-tool miss rates from "+fmtThousands(totalCalls)+" logged calls", "/observatory/impact"),
			q("Q4", "is the knowledge compounding?", firstEditAnswer(imp), "warm-up turns before the first edit, per session", "/observatory/story"),
		}
		type entry struct{ href, name, desc string }
		var widgets []string
		for _, e := range []entry{
			{"/observatory", "observatory", "fleet tiles: wiring grades, freshness, traffic sparklines — every warning names its remedy"},
			{"/observatory/impact", "impact", "the four questions with their instruments; the counterfactual pilot state machine (law 12)"},
			{"/observatory/story", "the week", "the trailing 7 days composed from the instruments, with session replays"},
			{"/statusz", "statusz", "daemon health: uptime, per-service index freshness, channel liveness, last compile per layer"},
			{"/metrics", "metrics", "machine-readable counters"},
			{"/healthz", "healthz", "liveness probe (plain ok)"},
		} {
			widgets = append(widgets, dashboard.RenderWidget(dashboard.Widget{
				Title: e.name,
				Body:  template.HTML(`<div class="text-ink-muted">` + html.EscapeString(e.desc) + `</div><div class="mt-2"><a class="text-accent" href="` + e.href + `">open →</a></div>`),
			}))
		}
		var mach strings.Builder
		mach.WriteString(kv("MCP over HTTP", `<code class="font-mono text-xs">/rpc/agent</code> · <code class="font-mono text-xs">/rpc/companion</code> · <code class="font-mono text-xs">/rpc</code>`))
		mach.WriteString(kv("services indexed", fmtThousands(len(m.Services))))
		// SurfaceTax (TD-1): the standing context cost every MCP-connected
		// session pays before any work — measured, or named unmeasured.
		mach.WriteString(kv("context tax", contextTaxValue(m.AgentSurfaceTools, m.AgentSchemaBytes)))
		mach.WriteString(kv("shareable brief", `<code class="font-mono text-xs">fathom brief</code> — writes a local file; sharing is a human verb (law 13)`))
		body := sectionHeading("the four questions") +
			dashboard.RenderWidgetGrid(questions, dashboard.WidgetGridOptions{Cols: 4, AriaLabel: "the four questions"}) +
			sectionHeading("surfaces") +
			dashboard.RenderWidgetGrid(widgets, dashboard.WidgetGridOptions{Cols: 3, AriaLabel: "surfaces"}) +
			sectionHeading("machine surfaces") +
			dashboard.RenderWidget(dashboard.Widget{Body: template.HTML(mach.String())})
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(obsPage("fathom", "home", body, m)))
	}
}

func wantsJSON(r *http.Request) bool {
	return r.URL.Query().Get("format") == "json" || strings.Contains(r.Header.Get("Accept"), "application/json")
}

// writeObsJSON writes v as pretty JSON (the HTTP sibling of pluginrender's
// file-writing writeJSON).
func writeObsJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// intOrZero is a display helper for landing-page counts (the sibling note
// carries the honesty; the headline needs a number-shaped string).
func intOrZero(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

// contextTaxValue renders TD-1's measurement: the agent surface's tool
// count and schema weight per connected session (tokens ≈ bytes/4). Zeros
// mean the surface did not answer — named, never rendered as a free
// surface (law 3).
func contextTaxValue(tools, schemaBytes int) string {
	if tools == 0 && schemaBytes == 0 {
		return `<span class="italic text-ink-faint">not measured (agent surface unreachable)</span>`
	}
	kb := float64(schemaBytes) / 1024
	tokens := float64(schemaBytes) / 4 / 1000
	return html.EscapeString(fmt.Sprintf("%d tools · ~%.1f KB schemas (≈%.1fk tokens) per connected session", tools, kb, tokens))
}

// firstEditAnswer renders Q4's landing headline.
func firstEditAnswer(imp obsImpact) string {
	if imp.FirstEditMedian != nil {
		return fmtThousands(*imp.FirstEditMedian) + "-turn median warm-up"
	}
	return "not yet measured"
}
