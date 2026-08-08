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

func obsPage(title, active string, body string, m observatoryModel) string {
	var b strings.Builder
	b.WriteString(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>` + html.EscapeString(title) + `</title><link rel="stylesheet" href="/assets/app.css"></head>`)
	b.WriteString(`<body class="min-h-screen bg-surface p-5 font-sans text-sm text-ink">`)
	b.WriteString(`<h1 class="text-lg font-semibold">fathom observatory</h1>`)
	b.WriteString(`<nav class="mb-4 mt-1 flex gap-4">`)
	for _, p := range [][2]string{{"/", "home"}, {"/observatory", "fleet"}, {"/observatory/impact", "impact"}, {"/statusz", "statusz"}} {
		cls := "text-accent no-underline"
		if p[1] == active {
			cls += " font-semibold"
		}
		b.WriteString(`<a class="` + cls + `" href="` + p[0] + `">` + p[1] + `</a>`)
	}
	b.WriteString(`</nav>`)
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
		if s.FreshnessRemedy != "" {
			body.WriteString(remedy(s.FreshnessRemedy))
		}
		if s.WiringRemedy != "" {
			body.WriteString(remedy(s.WiringRemedy))
		}
		widgets = append(widgets, dashboard.RenderWidget(dashboard.Widget{
			Title:  s.Service,
			Action: template.HTML(`<a class="text-accent" href="/observatory/service?id=` + html.EscapeString(s.Service) + `">detail →</a>`),
			Body:   template.HTML(body.String()),
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
		widgets := []string{dashboard.RenderWidget(dashboard.Widget{Title: id, Body: template.HTML(body.String())})}
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
	b.WriteString(sectionHeading("impact — every number carries its instrument"))
	sessions := imp.LedgerSessions
	b.WriteString(stat.RenderBand([]stat.Stat{
		impactStat("verify catches", imp.VerifyCatchesTotal, imp.VerifyCatchesNote),
		impactStat("discovery actions", imp.DiscoveryTotal, imp.DiscoveryNote),
		{Label: "ledger sessions", Value: fmtThousands(sessions), Sub: imp.DateRange},
	}, stat.BandOptions{AriaLabel: "impact metrics"}))

	b.WriteString(sectionHeading("counterfactual pilot (law 12: a savings figure exists only as an arm range)"))
	var card strings.Builder
	card.WriteString(kv("state", `<span class="font-semibold">`+html.EscapeString(imp.PilotState)+`</span>`))
	card.WriteString(kv("ON arm", plural(imp.TokensOnSessions, "session")+` · median tokens `+metricCell(imp.TokensOnMedian, "no token totals", "")))
	card.WriteString(kv("OFF arm", plural(imp.TokensOffSessions, "session")+` · median tokens `+metricCell(imp.TokensOffMedian, "no token totals", "")))
	card.WriteString(`<div class="mt-2 text-ink-muted">` + html.EscapeString(imp.PilotDetail) + `</div>`)
	b.WriteString(dashboard.RenderWidget(dashboard.Widget{Body: template.HTML(card.String())}))

	b.WriteString(sectionHeading("stories"))
	b.WriteString(`<div class="rounded-md border border-dashed border-caution px-3 py-2 text-ink-muted">narrative is a lens, never evidence (law 14). ` + html.EscapeString(imp.StoriesNote) + `</div>`)
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
		type entry struct{ href, name, desc string }
		var widgets []string
		for _, e := range []entry{
			{"/observatory", "observatory", "fleet tiles: wiring, freshness, traffic, open observations — every warning names its remedy"},
			{"/observatory/impact", "impact", "verify catches and discovery with denominators; the counterfactual pilot state machine (law 12)"},
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
		mach.WriteString(kv("shareable brief", `<code class="font-mono text-xs">fathom brief</code> — writes a local file; sharing is a human verb (law 13)`))
		body := sectionHeading("surfaces") +
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
