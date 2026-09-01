package gallery

import (
	"context"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/geoffmilleraz/app-kit/appkit/ui/callout"
	"github.com/geoffmilleraz/app-kit/appkit/ui/card"
	"github.com/geoffmilleraz/app-kit/appkit/ui/chat"
	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/app-kit/appkit/ui/dashboard"
	"github.com/geoffmilleraz/app-kit/appkit/ui/form"
	"github.com/geoffmilleraz/app-kit/appkit/ui/hitl"
	"github.com/geoffmilleraz/app-kit/appkit/ui/kanban"
	"github.com/geoffmilleraz/app-kit/appkit/ui/motion"
	"github.com/geoffmilleraz/app-kit/appkit/ui/overlay"
	"github.com/geoffmilleraz/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/app-kit/appkit/ui/tokens"
	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
)

// renderTempl renders a templ.Component to an HTML string for embedding in the
// Gallery's string-templated pages.
func renderTempl(c templ.Component) template.HTML {
	var b strings.Builder
	_ = c.Render(context.Background(), &b)
	return template.HTML(b.String())
}

// This file turns the kit's own data — the generated token vocabulary
// (tokens.ColorNames/RadiusNames/FontNames) and the ui/viz primitives — into
// view models the templates render. Nothing is hardcoded that the kit already
// declares: add a role to the CUE token set, regenerate, and it appears here.

// pageData is the shell's view model.
type pageData struct {
	Title   string
	Nav     []navLink
	Theme   string // "", "light", or "dark" — written to <html data-theme>
	Page    string // active page key, for nav highlighting + theme-link round-trips
	Content template.HTML
}

type navLink struct {
	Key, Label, Href string
	Active           bool
}

// swatch is one color role rendered three ways (fill, text, border). The class
// strings are built from the role name; input.css force-generates them.
type swatch struct {
	Role        string
	BgClass     string
	TextClass   string
	BorderClass string
}

func colorSwatches() []swatch {
	out := make([]swatch, 0, len(tokens.ColorNames))
	for _, r := range tokens.ColorNames {
		out = append(out, swatch{
			Role:        r,
			BgClass:     "bg-" + r,
			TextClass:   "text-" + r,
			BorderClass: "border-" + r,
		})
	}
	return out
}

type radiusSample struct {
	Name  string
	Class string
}

func radiusSamples() []radiusSample {
	out := make([]radiusSample, 0, len(tokens.RadiusNames))
	for _, n := range tokens.RadiusNames {
		out = append(out, radiusSample{Name: n, Class: "rounded-" + n})
	}
	return out
}

type fontSample struct {
	Name  string
	Class string
}

func fontSamples() []fontSample {
	out := make([]fontSample, 0, len(tokens.FontNames))
	for _, n := range tokens.FontNames {
		out = append(out, fontSample{Name: n, Class: "font-" + n})
	}
	return out
}

// vizCard is one ui/viz primitive rendered with representative data, captioned
// with the call that produced it. Wide cards span both grid columns (for the
// timeline, which needs the room).
type vizCard struct {
	Name string
	Call string
	SVG  template.HTML
	Wide bool
}

// tlDay is a fixture-date helper for the Gallery's timeline examples.
func tlDay(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// outcomeCards showcases the outcome-visualization primitives (recovery ranges,
// combined-recovery waterfall, reward-vs-cost) — the at-a-glance "what's it
// worth" surface.
func outcomeCards() []vizCard {
	h := func(s string) template.HTML { return template.HTML(s) }
	return []vizCard{
		{
			Name: "RangeBars — recovery low/high + most-likely", Call: `viz.RangeBars([]viz.RangeItem{{Low,High,Point,HasPoint,Prefix:"$"}}, …)`, Wide: true,
			SVG: h(viz.RangeBars([]viz.RangeItem{
				{Label: "habitability", Low: 2400, High: 9600, Point: 5200, HasPoint: true, Role: "positive", Prefix: "$"},
				{Label: "deposit", Low: 1200, High: 3600, Point: 2400, HasPoint: true, Role: "accent", Prefix: "$"},
				{Label: "filing fees", Low: 0, High: 1500, Role: "caution", Prefix: "$"},
			}, viz.RangeBarsOptions{W: 520, Max: 10000, AriaLabel: "recovery ranges"})),
		},
		{
			Name: "Waterfall — combined recovery → net", Call: `viz.Waterfall([]viz.WaterfallStep{{Delta…},{Total:true}}, …)`, Wide: true,
			SVG: h(viz.Waterfall([]viz.WaterfallStep{
				{Label: "habitability", Delta: 5200},
				{Label: "deposit", Delta: 2400},
				{Label: "overlap", Delta: -1800},
				{Label: "subtotal", Total: true},
				{Label: "fees", Delta: -240},
				{Label: "net", Total: true},
			}, viz.WaterfallOptions{W: 560, H: 150, Prefix: "$", AriaLabel: "combined recovery"})),
		},
		{
			Name: "CostReward — reward vs cost", Call: `viz.CostReward([]viz.CostRewardItem{{Reward,Cost}}, …)`, Wide: true,
			SVG: h(viz.CostReward([]viz.CostRewardItem{
				{Label: "habitability", Reward: 5200, Cost: 240},
				{Label: "deposit", Reward: 2400, Cost: 120},
				{Label: "trial path", Reward: 9600, Cost: 1500},
			}, viz.CostRewardOptions{W: 520, Prefix: "$", AriaLabel: "reward vs cost"})),
		},
		{
			Name: "Tornado — what moves the outcome most (sensitivity)", Call: `viz.Tornado([]viz.TornadoItem{{Low,High}}, viz.TornadoOptions{Baseline})`, Wide: true,
			SVG: h(viz.Tornado([]viz.TornadoItem{
				{Label: "habitability", Low: 3000, High: 9000, Prefix: "$"},
				{Label: "filing court", Low: 4200, High: 8000, Prefix: "$"},
				{Label: "deposit", Low: 4000, High: 5200, Prefix: "$"},
			}, viz.TornadoOptions{W: 520, Baseline: 4500, AriaLabel: "what moves recovery most"})),
		},
	}
}

// timelineCards renders the same case-timeline data under each view, so the
// Gallery shows what the views do — especially the condensed view's honest
// gap-eliding (the ~10-month stretch to the hearing collapses to a labeled
// break).
func timelineCards() []vizCard {
	events := []viz.Event{
		{At: tlDay(2026, 1, 5), Label: "filed", Role: "accent"},
		{At: tlDay(2026, 1, 20), Label: "served", Role: "info"},
		{At: tlDay(2026, 2, 3), Label: "answered", Role: "positive"},
		{At: tlDay(2026, 5, 12), Label: "discovery opens", Role: "accent"},
		{At: tlDay(2026, 8, 27), Label: "depositions", Role: "info"},
		{At: tlDay(2026, 11, 30), Label: "hearing", Role: "danger"},
	}
	base := func() viz.TimelineOptions {
		return viz.TimelineOptions{
			W: 900,
			Spans: []viz.Span{
				{Start: tlDay(2026, 1, 5), End: tlDay(2026, 2, 3), Label: "pleadings", Role: "accent-soft"},
				{Start: tlDay(2026, 5, 12), End: tlDay(2026, 8, 27), Label: "discovery", Role: "info-soft"},
			},
			Markers: []viz.Marker{{At: tlDay(2026, 6, 15), Label: "today", Role: "focus"}},
		}
	}
	prop := base()
	prop.AriaLabel = "case timeline, proportional"
	seq := base()
	seq.View = viz.ViewSequence
	seq.AriaLabel = "case timeline, sequence"
	cond := base()
	cond.View = viz.ViewCondensed
	cond.AriaLabel = "case timeline, condensed"

	// Swimlanes: the same events grouped into rows by where they came from.
	laneEvents := []viz.Event{
		{At: tlDay(2026, 1, 5), Label: "filed", Role: "accent", Lane: "court"},
		{At: tlDay(2026, 1, 20), Label: "served", Role: "info", Lane: "court"},
		{At: tlDay(2026, 2, 3), Label: "answered", Role: "positive", Lane: "court"},
		{At: tlDay(2026, 5, 12), Label: "discovery opens", Role: "accent", Lane: "court"},
		{At: tlDay(2026, 8, 27), Label: "depositions", Role: "info", Lane: "court"},
		{At: tlDay(2026, 11, 30), Label: "hearing", Role: "danger", Lane: "court"},
		{At: tlDay(2026, 1, 18), Label: "voicemail", Role: "info", Lane: "evidence"},
		{At: tlDay(2026, 6, 2), Label: "text thread", Role: "info", Lane: "evidence"},
		{At: tlDay(2026, 9, 14), Label: "email", Role: "info", Lane: "evidence"},
		{At: tlDay(2026, 4, 9), Label: "intake call", Role: "accent", Lane: "client"},
		{At: tlDay(2026, 10, 1), Label: "prep session", Role: "accent", Lane: "client"},
	}
	lane := viz.TimelineOptions{
		W:         900,
		AriaLabel: "case timeline by lane",
		Spans: []viz.Span{
			{Start: tlDay(2026, 1, 5), End: tlDay(2026, 2, 3), Label: "pleadings", Role: "accent-soft", Lane: "court"},
			{Start: tlDay(2026, 5, 12), End: tlDay(2026, 8, 27), Label: "discovery", Role: "info-soft", Lane: "court"},
		},
		Markers: []viz.Marker{{At: tlDay(2026, 6, 15), Label: "today", Role: "focus"}},
	}
	// Density: same lane data as a heatmap (where is activity concentrated?).
	dens := lane
	dens.View = viz.ViewDensity
	dens.AriaLabel = "case activity density"

	// Filtered: the kit's EventFilter is the search/facet toolkit; here, only the
	// evidence lane within a window — the renderer just draws the filtered set.
	fEvents, fOpts := viz.EventFilter{
		Lanes: []string{"evidence"},
		From:  tlDay(2026, 1, 1), To: tlDay(2026, 7, 1),
	}.Apply(laneEvents, lane)
	fOpts.AriaLabel = "filtered: evidence lane, H1"

	return []vizCard{
		{Name: "Timeline — proportional", Call: `viz.Timeline(events, viz.TimelineOptions{Spans, Markers})`, SVG: template.HTML(viz.Timeline(events, prop)), Wide: true},
		{Name: "Timeline — condensed (gaps elided + labeled)", Call: `viz.Timeline(events, viz.TimelineOptions{View: viz.ViewCondensed, …})`, SVG: template.HTML(viz.Timeline(events, cond)), Wide: true},
		{Name: "Timeline — sequence (even spacing)", Call: `viz.Timeline(events, viz.TimelineOptions{View: viz.ViewSequence, …})`, SVG: template.HTML(viz.Timeline(events, seq)), Wide: true},
		{Name: "Timeline — swimlanes (grouped by source)", Call: `viz.Event{…, Lane: "court"} // any Lane set ⇒ grouped rows`, SVG: template.HTML(viz.Timeline(laneEvents, lane)), Wide: true},
		{Name: "Timeline — density (activity heatmap)", Call: `viz.Timeline(events, viz.TimelineOptions{View: viz.ViewDensity})`, SVG: template.HTML(viz.Timeline(laneEvents, dens)), Wide: true},
		{Name: "Timeline — filtered (EventFilter: evidence lane, H1)", Call: `viz.EventFilter{Lanes:[]string{"evidence"}, From, To}.Apply(events, opts)`, SVG: template.HTML(viz.Timeline(fEvents, fOpts)), Wide: true},
	}
}

func vizCards() []vizCard {
	cards := []vizCard{
		{
			Name: "Sparkline",
			Call: `viz.Sparkline([]float64{4,7,5,9,8,12,10,14}, viz.SparklineOptions{FillRole: "accent-soft"})`,
			SVG:  template.HTML(viz.Sparkline([]float64{4, 7, 5, 9, 8, 12, 10, 14}, viz.SparklineOptions{W: 160, H: 40, FillRole: "accent-soft"})),
		},
		{
			Name: "StackBar",
			Call: `viz.StackBar([]viz.Segment{{Value:62,Role:"positive"},{Value:24,Role:"caution"},{Value:14,Role:"danger"}}, …)`,
			SVG: template.HTML(viz.StackBar([]viz.Segment{
				{Value: 62, Role: "positive", Label: "scanned"},
				{Value: 24, Role: "caution", Label: "pending"},
				{Value: 14, Role: "danger", Label: "failed"},
			}, viz.StackBarOptions{W: 280, H: 16, AriaLabel: "coverage"})),
		},
		{
			Name: "Donut",
			Call: `viz.Donut([]viz.Segment{{Value:5,Role:"accent"},{Value:3,Role:"info"},{Value:2,Role:"positive"}}, viz.DonutOptions{CenterText:"10"})`,
			SVG: template.HTML(viz.Donut([]viz.Segment{
				{Value: 5, Role: "accent", Label: "primary"},
				{Value: 3, Role: "info", Label: "secondary"},
				{Value: 2, Role: "positive", Label: "tertiary"},
			}, viz.DonutOptions{Size: 120, Thickness: 14, CenterText: "10", AriaLabel: "distribution"})),
		},
		{
			Name: "Funnel",
			Call: `viz.Funnel([]viz.Stage{{Label:"proposed",Value:120,Role:"accent"},{Label:"imported",Value:74,Role:"positive"},{Label:"excluded",Value:46,Role:"danger"}}, …)`,
			SVG: template.HTML(viz.Funnel([]viz.Stage{
				{Label: "proposed", Value: 120, Role: "accent"},
				{Label: "imported", Value: 74, Role: "positive"},
				{Label: "excluded", Value: 46, Role: "danger"},
			}, viz.FunnelOptions{W: 220, StageH: 24, AriaLabel: "intake funnel"})),
		},
		{
			Name: "MeterRow",
			Call: `viz.MeterRow(78, 100, viz.MeterOptions{Label:"confidence", ShowPct:true})`,
			SVG:  template.HTML(viz.MeterRow(78, 100, viz.MeterOptions{W: 280, Label: "confidence", ShowPct: true, AriaLabel: "confidence meter"})),
		},
		{
			Name: "Gauge — graded (case strength)",
			Call: `viz.Gauge(72, 100, viz.GaugeOptions{Graded: true, Label: "case strength"})`,
			SVG:  template.HTML(viz.Gauge(72, 100, viz.GaugeOptions{W: 180, Graded: true, Label: "case strength", AriaLabel: "case strength 72%"})),
		},
		{
			Name: "Gauge — graded (likelihood)",
			Call: `viz.Gauge(38, 100, viz.GaugeOptions{Graded: true, Label: "likely outcome"})`,
			SVG:  template.HTML(viz.Gauge(38, 100, viz.GaugeOptions{W: 180, Graded: true, Label: "likely outcome", AriaLabel: "likelihood 38%"})),
		},
	}
	cards = append(cards, outcomeCards()...)
	return append(cards, timelineCards()...)
}

// ---- templates ------------------------------------------------------------

var funcs = template.FuncMap{
	"upper": strings.ToUpper,
}

var shellTmpl = template.Must(template.New("shell").Funcs(funcs).Parse(shellHTML))

func render(d pageData) ([]byte, error) {
	var b strings.Builder
	if err := shellTmpl.Execute(&b, d); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

// htmlf renders a sub-template body to template.HTML for embedding in the shell.
func htmlf(name, body string, data any) (template.HTML, error) {
	t, err := template.New(name).Funcs(funcs).Parse(body)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := t.Execute(&b, data); err != nil {
		return "", err
	}
	return template.HTML(b.String()), nil
}

const shellHTML = `<!doctype html>
<html lang="en"{{if .Theme}} data-theme="{{.Theme}}"{{end}}>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}} · app-kit Gallery</title>
<link rel="stylesheet" href="/static/app.css">
</head>
<body class="min-h-screen bg-surface text-ink font-sans">
<header class="border-b border-line bg-surface-raised">
  <div class="mx-auto max-w-5xl px-6 py-4 flex items-center justify-between gap-4">
    <div class="flex items-center gap-3">
      <span class="inline-block size-4 rounded-full bg-accent"></span>
      <a href="/" class="font-semibold text-ink">app-kit Gallery</a>
    </div>
    <nav class="flex items-center gap-1 text-sm">
      {{range .Nav}}
      <a href="{{.Href}}" class="rounded-md px-3 py-1.5 {{if .Active}}bg-accent text-accent-ink{{else}}text-ink-muted{{end}}">{{.Label}}</a>
      {{end}}
    </nav>
    <div class="flex items-center gap-1 text-xs">
      <span class="text-ink-faint">theme</span>
      <a href="?theme=light&page={{.Page}}" class="rounded-md border border-line px-2 py-1 text-ink-muted">light</a>
      <a href="?theme=dark&page={{.Page}}" class="rounded-md border border-line px-2 py-1 text-ink-muted">dark</a>
      <a href="?page={{.Page}}" class="rounded-md border border-line px-2 py-1 text-ink-muted">OS</a>
    </div>
  </div>
</header>
<main class="mx-auto max-w-5xl px-6 py-8">
{{.Content}}
</main>
<footer class="mx-auto max-w-5xl px-6 py-10 text-xs text-ink-faint">
  The living style guide (D12). Every role and component here renders through the kit's token CSS — toggle the theme and watch it re-bind with no rebuild.
</footer>
</body>
</html>`

const overviewHTML = `
<h1 class="text-2xl font-semibold text-ink">The living style guide</h1>
<p class="mt-2 max-w-2xl text-ink-muted">A kit app, built on the kit, that <em>is</em> the proof of the kit (VISION #2, D12). Every design token is visualized and every <code class="font-mono text-accent">ui/viz</code> primitive is rendered live, themed through the token CSS. This is v0: tokens + visualization. Knob-driven component pages and service exercisers arrive with the Phase 1 spine.</p>
<div class="mt-6 grid gap-4 sm:grid-cols-2">
  <a href="/tokens" class="rounded-lg border border-line bg-surface-raised p-5 hover:border-line-strong">
    <h2 class="font-medium text-ink">Tokens</h2>
    <p class="mt-1 text-sm text-ink-muted">{{.NumColors}} color roles, {{.NumRadius}} radii, {{.NumFonts}} font roles — the full semantic vocabulary.</p>
  </a>
  <a href="/components" class="rounded-lg border border-line bg-surface-raised p-5 hover:border-line-strong">
    <h2 class="font-medium text-ink">Components</h2>
    <p class="mt-1 text-sm text-ink-muted">The {{.NumViz}} ui/viz primitives, server-rendered SVG, themed by role.</p>
  </a>
</div>`

const tokensHTML = `
<h1 class="text-2xl font-semibold text-ink">Tokens</h1>
<p class="mt-2 text-ink-muted">The semantic role vocabulary. Each swatch renders the actual utility — toggle the theme to see roles re-bind.</p>

<h2 class="mt-8 text-lg font-medium text-ink">Color roles</h2>
<div class="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
  {{range .Colors}}
  <div class="rounded-lg border border-line bg-surface-raised p-3">
    <div class="flex items-center gap-3">
      <div class="size-10 shrink-0 rounded-md border border-line {{.BgClass}}"></div>
      <div class="min-w-0">
        <div class="font-mono text-sm text-ink">{{.Role}}</div>
        <div class="{{.TextClass}} text-sm">Aa text sample</div>
      </div>
    </div>
  </div>
  {{end}}
</div>

<h2 class="mt-10 text-lg font-medium text-ink">Radius</h2>
<div class="mt-4 flex flex-wrap gap-6">
  {{range .Radius}}
  <div class="text-center">
    <div class="size-16 border border-line-strong bg-accent-soft {{.Class}}"></div>
    <div class="mt-2 font-mono text-xs text-ink-muted">{{.Class}}</div>
  </div>
  {{end}}
</div>

<h2 class="mt-10 text-lg font-medium text-ink">Fonts</h2>
<div class="mt-4 space-y-3">
  {{range .Fonts}}
  <div class="rounded-lg border border-line bg-surface-raised p-4">
    <div class="font-mono text-xs text-ink-faint">{{.Class}}</div>
    <div class="{{.Class}} text-lg text-ink">The quick brown fox jumps over the lazy dog — 0123456789</div>
  </div>
  {{end}}
</div>`

const componentsHTML = `
<h1 class="text-2xl font-semibold text-ink">Components</h1>
<p class="mt-2 text-ink-muted">The <code class="font-mono text-accent">appkit/ui/viz</code> primitives — pure-Go server-rendered SVG, every color resolved to a token role, accessible (role=img + title). Same inputs → byte-identical output.</p>
<div class="mt-6 grid gap-4 lg:grid-cols-2">
  {{range .Cards}}
  <figure class="rounded-lg border border-line bg-surface-raised p-5{{if .Wide}} lg:col-span-2{{end}}">
    <figcaption class="font-medium text-ink">{{.Name}}</figcaption>
    <div class="mt-4 flex min-h-24 items-center justify-center overflow-x-auto rounded-md bg-surface p-4">{{.SVG}}</div>
    <pre class="mt-4 overflow-x-auto rounded-md bg-surface-sunken p-3 text-xs text-ink-muted"><code class="font-mono">{{.Call}}</code></pre>
  </figure>
  {{end}}
</div>`

const messagingHTML = `
<h1 class="text-2xl font-semibold text-ink">Messaging &amp; at-a-glance</h1>
<p class="mt-2 max-w-2xl text-ink-muted">Components for understanding state fast and for the disclaimers, warnings, and emphasis a trustworthy app needs. All themed by role; the composite below combines viz primitives + callouts into one case summary.</p>

<h2 class="mt-8 text-lg font-medium text-ink">Key figures</h2>
<div class="mt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">{{range .Stats}}{{.}}{{end}}</div>

<h2 class="mt-10 text-lg font-medium text-ink">At a glance — case summary</h2>
<div class="mt-4 rounded-lg border border-line bg-surface-raised p-6">
  <div class="flex flex-wrap items-start justify-between gap-4">
    <div>
      <h3 class="text-lg font-medium text-ink">Doe v. Roe</h3>
      <div class="mt-2 flex flex-wrap gap-2">{{range .Badges}}{{.}}{{end}}</div>
      <div class="mt-3 flex items-center gap-2">
        <span class="text-sm text-ink-muted">evidence strength</span>{{.Rating}}
      </div>
    </div>
    <div class="shrink-0">{{.Strength}}</div>
  </div>
  <div class="mt-6 border-t border-line pt-5">
    <p class="text-sm text-ink-muted">Case status</p>
    <div class="mt-3 overflow-x-auto">{{.Status}}</div>
  </div>
  <div class="mt-6 grid gap-6 sm:grid-cols-2">
    <div>
      <p class="text-sm text-ink-muted">Expected outcome</p>
      <div class="mt-2">{{.Outcome}}</div>
    </div>
    <div>
      <p class="text-sm text-ink-muted">Evidence</p>
      <div class="mt-2">{{.Coverage}}</div>
    </div>
  </div>
  <div class="mt-6">{{.Disclaimer}}</div>
</div>

<h2 class="mt-10 text-lg font-medium text-ink">Callouts</h2>
<div class="mt-4 space-y-3">
  {{range .Callouts}}{{.}}{{end}}
</div>

<h2 class="mt-10 text-lg font-medium text-ink">Status badges</h2>
<div class="mt-4 flex flex-wrap items-center gap-2">
  {{range .BadgeRow}}{{.}}{{end}}
</div>`

// buildOverview / buildTokens / buildComponents assemble the page content.

func buildOverview() (template.HTML, error) {
	return htmlf("overview", overviewHTML, map[string]int{
		"NumColors": len(tokens.ColorNames),
		"NumRadius": len(tokens.RadiusNames),
		"NumFonts":  len(tokens.FontNames),
		"NumViz":    len(vizCards()),
	})
}

func buildTokens() (template.HTML, error) {
	return htmlf("tokens", tokensHTML, struct {
		Colors []swatch
		Radius []radiusSample
		Fonts  []fontSample
	}{colorSwatches(), radiusSamples(), fontSamples()})
}

func buildComponents() (template.HTML, error) {
	return htmlf("components", componentsHTML, struct{ Cards []vizCard }{vizCards()})
}

// messagingData is the Messaging page view model: an at-a-glance composite plus
// the callout kinds and badges.
type messagingData struct {
	Stats      []template.HTML
	Status     template.HTML
	Strength   template.HTML
	Rating     template.HTML
	Outcome    template.HTML
	Coverage   template.HTML
	Badges     []template.HTML
	Disclaimer template.HTML
	Callouts   []template.HTML
	BadgeRow   []template.HTML
}

func buildMessaging() (template.HTML, error) {
	h := func(s string) template.HTML { return template.HTML(s) }
	d := messagingData{
		Stats: []template.HTML{
			h(stat.Render(stat.Stat{Label: "Evidence items", Value: "156", Delta: "+18 this week", DeltaRole: "positive", Spark: []float64{90, 104, 112, 120, 138, 156}, SparkRole: "info"})),
			h(stat.Render(stat.Stat{Label: "Open deadlines", Value: "3", Delta: "1 within 9 days", DeltaRole: "caution"})),
			h(stat.Render(stat.Stat{Label: "Unresolved conflicts", Value: "2", Delta: "+1", DeltaRole: "danger"})),
		},
		Status: h(viz.Steps([]viz.Step{
			{Label: "filed", State: viz.StepDone},
			{Label: "discovery", State: viz.StepDone},
			{Label: "motions", State: viz.StepCurrent},
			{Label: "trial", State: viz.StepUpcoming},
			{Label: "resolved", State: viz.StepUpcoming},
		}, viz.StepsOptions{W: 560, AriaLabel: "case status: motions phase"})),
		Strength: h(viz.Gauge(72, 100, viz.GaugeOptions{W: 150, Graded: true, Label: "case strength", AriaLabel: "case strength 72%"})),
		Rating:   h(viz.Rating(4, viz.RatingOptions{Max: 5, PipR: 7, Graded: true, AriaLabel: "evidence strength 4 of 5"})),
		Outcome: h(viz.Bars([]viz.Bar{
			{Label: "favorable", Value: 55, Role: "positive", Suffix: "%"},
			{Label: "mixed", Value: 30, Role: "caution", Suffix: "%"},
			{Label: "unfavorable", Value: 15, Role: "danger", Suffix: "%"},
		}, viz.BarsOptions{W: 320, Max: 100, ShowValue: true, AriaLabel: "expected outcome distribution"})),
		Coverage: h(viz.MeterRow(156, 200, viz.MeterOptions{W: 300, Label: "evidence reviewed", ShowPct: true, FillRole: "info", AriaLabel: "evidence coverage 78%"})),
		Badges: []template.HTML{
			h(callout.Badge("Filed", "info")),
			h(callout.Badge("Discovery", "accent")),
			h(callout.Badge("Strong evidence", "positive")),
			h(callout.Badge("Deadline near", "caution")),
		},
		Disclaimer: h(callout.Disclaimer("")),
		Callouts: []template.HTML{
			h(callout.Render(callout.Callout{Kind: callout.Info, Title: "Filing accepted", Body: "The clerk recorded the complaint on 5 Jan 2026."})),
			h(callout.Render(callout.Callout{Kind: callout.Success, Title: "Strong corroboration", Body: "Three independent records support this assertion."})),
			h(callout.Render(callout.Callout{Kind: callout.Caution, Title: "Deadline approaching", Body: "The response is due in 9 days — calendar it now."})),
			h(callout.Render(callout.Callout{Kind: callout.Danger, Title: "Conflicting testimony", Body: "Two records contradict the stated timeline; reconcile before filing."})),
			h(callout.Render(callout.Callout{Kind: callout.Note, Body: "Tip: group the timeline by source to spot evidentiary gaps."})),
		},
		BadgeRow: []template.HTML{
			h(callout.Badge("Strong", "positive")),
			h(callout.Badge("Filed", "info")),
			h(callout.Badge("At risk", "caution")),
			h(callout.Badge("Overdue", "danger")),
			h(callout.Badge("Draft", "accent")),
			h(callout.Badge("Unknown", "ink")),
		},
	}
	return htmlf("messaging", messagingHTML, d)
}

// layoutData is the Layout page view model: the composition primitives
// (card, kanban, table) rendered with representative data.
type layoutData struct {
	Card     template.HTML
	Kanban   template.HTML
	Table    template.HTML
	StatBand template.HTML
	DescCard template.HTML
}

// buildLayout assembles the Layout & composition page — the templ-based
// composition primitives unified into the kit (card, kanban, table) plus a note
// on the embedded client router.
func buildLayout() (template.HTML, error) {
	raw := func(s string) templ.Component { return templ.Raw(s) }

	cardEl := card.Card(card.Params{
		Header: raw(`<h3 class="font-medium text-ink">Security deposit return</h3>`),
		Body:   raw(`<p class="text-sm text-ink-muted">$2,400 withheld; itemized statement delivered 14 days late under § 1950.5.</p>`),
		Footer: raw(`<a href="#" class="text-sm text-accent">View evidence →</a>`),
	})

	cols := []kanban.Column{
		{ID: "gather", Label: "To gather", Count: 2},
		{ID: "review", Label: "In review", Count: 1},
		{ID: "confirmed", Label: "Confirmed", Count: 1},
	}
	items := []kanban.CardItem{
		{ID: "k1", StageID: "gather", Title: "Lease agreement", Subtitle: "from tenant", Badges: []kanban.Badge{{Label: "critical", Role: "danger-soft"}}, Href: "#"},
		{ID: "k2", StageID: "gather", Title: "Move-out photos", Meta: "3 files"},
		{ID: "k3", StageID: "review", Title: "Deposit receipt", Badges: []kanban.Badge{{Label: "documentary", Role: "info-soft"}}},
		{ID: "k4", StageID: "confirmed", Title: "Itemization letter", Badges: []kanban.Badge{{Label: "confirmed", Role: "positive-soft"}}},
	}

	// Rich cells (HTMLCells) — the Tier column carries a kit Badge instead of
	// plain text, demonstrating badges/links/viz inside a table cell.
	tierCell := func(label, role string) templ.Component { return templ.Raw(callout.Badge(label, role)) }
	tbl := table.Table(table.TableParams{
		Caption: "Evidence by tier",
		Columns: []table.Column{
			{Key: "item", Label: "Item", Sortable: true},
			{Key: "tier", Label: "Tier", Sortable: true},
			{Key: "conf", Label: "Confidence", Align: "right", Sortable: true},
		},
		Rows: []table.Row{
			{ID: "r1", Cells: []string{"Deposit receipt", "", "92%"}, HTMLCells: []templ.Component{nil, tierCell("documentary", "info"), nil}, Href: "#"},
			{ID: "r2", Cells: []string{"Move-out photos", "", "85%"}, HTMLCells: []templ.Component{nil, tierCell("documentary", "info"), nil}, Href: "#"},
			{ID: "r3", Cells: []string{"Tenant statement", "", "60%"}, HTMLCells: []templ.Component{nil, tierCell("testimonial", "caution"), nil}},
		},
		SortKey: "conf", SortDir: "desc", RowHrefs: true,
	})

	statBand := stat.RenderBand([]stat.Stat{
		{Label: "Met", Value: "5", ValueRole: "positive"},
		{Label: "Under-evidenced", Value: "1", ValueRole: "caution"},
		{Label: "Unmet", Value: "0", ValueRole: "danger"},
		{Label: "Required", Value: "6", Sub: "elements"},
	}, stat.BandOptions{AriaLabel: "element coverage KPIs"})

	descCard := chrome.RenderDescriptionListCard([]chrome.DescItem{
		{Term: "Theory", Detail: "Security Deposit Return (§ 1950.5)"},
		{Term: "Coverage", DetailHTML: template.HTML(callout.Badge("Well-evidenced", "positive"))},
		{Term: "Recovery", Detail: "$4,450 – $9,250 if established"},
		{Term: "Filed", Detail: "Not yet filed"},
	}, chrome.DescListOptions{Title: "Case facts", Subtitle: "Detail-screen metadata."})

	return htmlf("layout", layoutHTML, layoutData{
		Card:     renderTempl(cardEl),
		Kanban:   renderTempl(kanban.Board(cols, items)),
		Table:    renderTempl(tbl),
		StatBand: template.HTML(statBand),
		DescCard: template.HTML(descCard),
	})
}

const layoutHTML = `
<h1 class="text-2xl font-semibold text-ink">Layout &amp; composition</h1>
<p class="mt-2 max-w-2xl text-ink-muted">The templ-based composition primitives — <code class="font-mono text-accent">appkit/ui/card</code>, <code class="font-mono text-accent">kanban</code>, <code class="font-mono text-accent">table</code> — server-rendered, token-themed, and loopback-safe (links/markup only). The bordered <code class="font-mono text-accent">card.Card</code> is the single canonical surface; <code class="font-mono text-accent">kanban</code> composes it.</p>

<h2 class="mt-8 text-lg font-medium text-ink">Card — <code class="font-mono text-sm">card.Card</code></h2>
<p class="mt-1 text-sm text-ink-muted">Header / Body / Footer slots (templ.Component); the base for higher-level layouts.</p>
<div class="mt-4 max-w-md">{{.Card}}</div>

<h2 class="mt-10 text-lg font-medium text-ink">Kanban — <code class="font-mono text-sm">kanban.Board</code></h2>
<p class="mt-1 text-sm text-ink-muted">Columns + cards (StageID → Column.ID), composed from <code class="font-mono">card.Card</code>; emits <code class="font-mono">data-*</code> hooks for the app's hypermedia layer. Presentational only.</p>
<div class="mt-4 overflow-x-auto rounded-lg border border-line bg-surface p-4">{{.Kanban}}</div>

<h2 class="mt-10 text-lg font-medium text-ink">Table — <code class="font-mono text-sm">table.Table</code></h2>
<p class="mt-1 text-sm text-ink-muted">Typed columns with sortable header links + length-tolerant rows; optional per-row "Open" links and an accessible caption. Cells accept rich content via <code class="font-mono">HTMLCells</code> — the Tier column below carries a kit Badge. Adapted from the TailwindPlus tables pattern.</p>
<div class="mt-4 overflow-x-auto rounded-lg border border-line bg-surface-raised p-4">{{.Table}}</div>

<h2 class="mt-10 text-lg font-medium text-ink">Stat band — <code class="font-mono text-sm">stat.RenderBand</code></h2>
<p class="mt-1 text-sm text-ink-muted">Shared-border KPI row (TailwindPlus "Stats · With shared borders"), role-colored values + optional sub-caption and trend pill. Stacks on mobile, divides into columns above.</p>
<div class="mt-4">{{.StatBand}}</div>

<h2 class="mt-10 text-lg font-medium text-ink">Description list (card) — <code class="font-mono text-sm">chrome.RenderDescriptionListCard</code></h2>
<p class="mt-1 text-sm text-ink-muted">Bordered card with header + divided term/detail rows (TailwindPlus "Description Lists · Left-aligned in card") for detail screens; <code class="font-mono">DetailHTML</code> carries rich values like badges.</p>
<div class="mt-4 max-w-2xl">{{.DescCard}}</div>

<h2 class="mt-10 text-lg font-medium text-ink">Router — <code class="font-mono text-sm">appkit/ui/router</code></h2>
<p class="mt-1 max-w-2xl text-sm text-ink-muted">An embedded, dependency-free History-API client router for static-client surfaces (e.g. GitHub Pages SPAs) — no build step, no Node (ADR-0002). Serve it via <code class="font-mono">router.Handler()</code> or inline with <code class="font-mono">router.Script()</code>:</p>
<pre class="mt-3 max-w-2xl overflow-x-auto rounded-md bg-surface-sunken p-3 text-xs text-ink-muted"><code class="font-mono">const router = AppKitRouter.create({
  routes: [
    { id: "home",  path: "/",      title: "Home" },
    { id: "case",  path: "/c/:id", title: "Case" },
  ],
  onRoute: (m) =&gt; render(m.id, m.params),
});</code></pre>`

// agenticData is the Agentic & chrome page view model: the human-in-the-loop
// surfaces (approval card, run trace) plus the structural chrome comforts.
type agenticData struct {
	Approve    template.HTML
	Trace      template.HTML
	Empty      template.HTML
	Crumbs     template.HTML
	Tabs       template.HTML
	Paginate   template.HTML
	Skeleton   template.HTML
	Disclosure template.HTML
	DescList   template.HTML
	Diff       template.HTML
	ToolCall   template.HTML
	Chat       template.HTML
	Script     template.JS
}

// dashboardData is the Dashboard page view model.
type dashboardData struct {
	Shell template.HTML
}

// buildDashboard assembles the Dashboard page — appkit/ui/dashboard's Shell
// (sidebar + topbar + content) wrapping a WidgetGrid of KPI band + viz widgets +
// a table, all token-themed. Layout adapted from MatDash; charts are kit ui/viz.
func buildDashboard() (template.HTML, error) {
	h := func(s string) template.HTML { return template.HTML(s) }

	band := stat.RenderBand([]stat.Stat{
		{Label: "Open cases", Value: "24", Delta: "+3", DeltaRole: "positive"},
		{Label: "Deadlines (30d)", Value: "7", ValueRole: "caution"},
		{Label: "Evidence items", Value: "1,284", Delta: "+58", DeltaRole: "positive"},
		{Label: "Conflicts", Value: "2", ValueRole: "danger"},
	}, stat.BandOptions{AriaLabel: "dashboard KPIs"})

	outcome := dashboard.RenderWidget(dashboard.Widget{
		Title: "Expected outcome", Span: 1,
		Body: h(viz.Bars([]viz.Bar{
			{Label: "favorable", Value: 55, Role: "positive", Suffix: "%"},
			{Label: "mixed", Value: 30, Role: "caution", Suffix: "%"},
			{Label: "unfavorable", Value: 15, Role: "danger", Suffix: "%"},
		}, viz.BarsOptions{W: 300, Max: 100, ShowValue: true, AriaLabel: "expected outcome"})),
	})
	mix := dashboard.RenderWidget(dashboard.Widget{
		Title: "Evidence mix", Span: 1,
		Body: h(viz.Donut([]viz.Segment{
			{Value: 5, Role: "accent", Label: "documentary"},
			{Value: 3, Role: "info", Label: "testimonial"},
			{Value: 2, Role: "positive", Label: "physical"},
		}, viz.DonutOptions{CenterText: "10"})),
	})
	intake := dashboard.RenderWidget(dashboard.Widget{
		Title: "Intake (7d)", Span: 1,
		Body: h(stat.Render(stat.Stat{
			Label: "New evidence", Value: "58", Delta: "+12%", DeltaRole: "positive",
			Spark: []float64{4, 6, 5, 9, 8, 12, 14}, SparkRole: "info",
		})),
	})
	deadlines := dashboard.RenderWidget(dashboard.Widget{
		Title: "Upcoming deadlines", Span: 3,
		Body: renderTempl(table.Table(table.TableParams{
			Columns: []table.Column{{Key: "case", Label: "Case"}, {Key: "item", Label: "Deadline"}, {Key: "due", Label: "Due", Align: "right"}},
			Rows: []table.Row{
				{ID: "1", Cells: []string{"Doe v. Roe", "Discovery cutoff", "9 days"}},
				{ID: "2", Cells: []string{"Smith Estate", "Response to motion", "14 days"}},
				{ID: "3", Cells: []string{"Acme Lease", "Demand letter", "21 days"}},
			},
		})),
	})

	content := `<div class="mb-4">` + band + `</div>` +
		dashboard.RenderWidgetGrid([]string{outcome, mix, intake, deadlines}, dashboard.WidgetGridOptions{Cols: 3, AriaLabel: "dashboard widgets"})

	nav := `<ul class="flex flex-col gap-1 text-sm">` +
		`<li><a href="#" class="block rounded-md bg-accent-soft px-3 py-2 font-medium text-accent">Overview</a></li>` +
		`<li><a href="#" class="block rounded-md px-3 py-2 text-ink-muted hover:bg-surface-sunken hover:text-ink">Cases</a></li>` +
		`<li><a href="#" class="block rounded-md px-3 py-2 text-ink-muted hover:bg-surface-sunken hover:text-ink">Evidence</a></li>` +
		`<li><a href="#" class="block rounded-md px-3 py-2 text-ink-muted hover:bg-surface-sunken hover:text-ink">Deadlines</a></li>` +
		`<li><a href="#" class="block rounded-md px-3 py-2 text-ink-muted hover:bg-surface-sunken hover:text-ink">Reports</a></li>` +
		`</ul>`

	shell := dashboard.RenderShell(dashboard.Shell{
		Title:   "Overview",
		Brand:   `<span class="text-base font-semibold text-ink">⬡ Acme Legal</span>`,
		Nav:     template.HTML(nav),
		Actions: `<button class="rounded-md bg-accent px-3 py-1.5 text-sm font-medium text-accent-ink hover:bg-accent-hover">New case</button>`,
		Content: template.HTML(content),
	})
	return htmlf("dashboard", dashboardHTML, dashboardData{Shell: h(shell)})
}

const dashboardHTML = `
<h1 class="text-2xl font-semibold text-ink">Dashboard</h1>
<p class="mt-2 max-w-2xl text-ink-muted"><code class="font-mono text-accent">appkit/ui/dashboard</code> — a <code class="font-mono">Shell</code> (sidebar + topbar + content) wrapping a <code class="font-mono">WidgetGrid</code> of a KPI band, <code class="font-mono">ui/viz</code> charts, and a table. The shell layout is adapted from <strong>MatDash</strong>, re-themed to kit role tokens and rendered in pure Go (no Node). Full-page component — shown here in a contained preview; the sidebar collapses to a "Menu" disclosure below <code class="font-mono">md</code>.</p>
<div class="mt-4 h-[640px] overflow-hidden rounded-lg border border-line shadow-sm">{{.Shell}}</div>`

// motionData is the Motion page view model: the two-state vocabulary shown as a
// living specimen — both states statically (the no-motion baseline), plus a live
// SVG grid and HTML grid a checkbox flips through the optional motion.js helper.
type motionData struct {
	StaticSVGA template.HTML
	StaticSVGB template.HTML
	LiveSVG    template.HTML
	LiveHTML   template.HTML
	Script     template.JS
}

// motionSVGGrid renders an 18-cell grid inside a .ak-motion root: an accent
// layer that belongs to state A and a positive layer at the same positions that
// belongs to state B, so toggling the root swaps which layer is opaque — the
// "same cells change tone" case from the package doc. Columns stagger by index;
// the B layer trails one full wave (.ak-after). Color comes only through role
// tokens (var(--ak-color-*)), so it themes with no Tailwind scanner (L3a-style).
func motionSVGGrid(id string, stateB bool) string {
	state := "A"
	if stateB {
		state = "B"
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<svg viewBox="0 0 360 128" width="100%%" height="128" role="img" aria-label="motion grid, state %s">`, state)
	fmt.Fprintf(&b, `<g id="%s" class="%s" style="--ak-motion-dim:.14">`, id, motion.RootClass(stateB))
	layer := func(when, fill string, after bool) {
		for row := 0; row < 3; row++ {
			for col := 0; col < 6; col++ {
				cls := when + " " + motion.Tint + " " + motion.Step(col+1)
				if after {
					cls += " " + motion.After
				}
				fmt.Fprintf(&b, `<rect x="%d" y="%d" width="48" height="30" rx="6" class="%s" style="fill:%s"/>`,
					8+col*58, 8+row*40, cls, fill)
			}
		}
	}
	layer(motion.WhenA, "var(--ak-color-accent)", false)
	layer(motion.WhenB, "var(--ak-color-positive)", true)
	b.WriteString(`</g></svg>`)
	return b.String()
}

// motionHTMLGrid renders three cards inside a .ak-motion root; each card stacks a
// state-A label over a state-B label (absolutely positioned so they share the
// cell), fading between them and staggering by column. Pure role utilities, so
// it themes through the same token layer as everything else on the page.
func motionHTMLGrid(id string, stateB bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<div id="%s" class="%s grid grid-cols-3 gap-3" style="--ak-motion-dim:.14">`, id, motion.RootClass(stateB))
	a := []string{"queued", "running", "review"}
	bb := []string{"holding", "syncing", "shipped"}
	for i := 0; i < 3; i++ {
		fmt.Fprintf(&b, `<div class="relative rounded-lg border border-line bg-surface-raised px-3 py-6 text-center text-sm">`+
			`<span class="%s %s %s block text-ink">%s</span>`+
			`<span class="%s %s %s absolute inset-0 flex items-center justify-center font-medium text-positive">%s</span>`+
			`</div>`,
			motion.WhenA, motion.Fade, motion.Step(i+1), a[i],
			motion.WhenB, motion.Fade, motion.Step(i+1), bb[i])
	}
	b.WriteString(`</div>`)
	return b.String()
}

// buildMotion assembles the Motion page — appkit/ui/motion (L2) shown as a
// living specimen. It renders both states statically (the readable no-motion
// baseline the invariant requires), then a live SVG grid + HTML grid a checkbox
// flips via the optional motion.js helper. The page proves the L2 contract by
// working: motion.css needs no @source entry (it is plain class selectors, not
// scanned utilities), and the state classes render correctly with motion off.
func buildMotion() (template.HTML, error) {
	return htmlf("motion", motionHTML, motionData{
		StaticSVGA: template.HTML(motionSVGGrid("static-svg-a", false)),
		StaticSVGB: template.HTML(motionSVGGrid("static-svg-b", true)),
		LiveSVG:    template.HTML(motionSVGGrid("demo-svg", false)),
		LiveHTML:   template.HTML(motionHTMLGrid("demo-html", false)),
		Script:     template.JS(motion.Script()),
	})
}

const motionHTML = `
<h1 class="text-2xl font-semibold text-ink">Motion</h1>
<p class="mt-2 max-w-2xl text-ink-muted"><code class="font-mono text-accent">appkit/ui/motion</code> — the kit's motion layer (L2): a two-state transition vocabulary, a bounded stagger, and <code class="font-mono">prefers-reduced-motion</code> welded to the class that grants motion. Both states live in the DOM at once; a class on the root (<code class="font-mono">ak-state-b</code>) selects which one shows and CSS interpolates between them. It costs nothing at runtime, it prints, and it survives a theme swap.</p>
<p class="mt-3 max-w-2xl text-sm text-ink-muted">This page <strong>imports <code class="font-mono">motion.css</code> with no <code class="font-mono">@source</code> entry</strong> — the classes are plain selectors, not scanned Tailwind utilities, so they resolve whether or not the scanner ever saw the markup. That is the whole L2 contract, proven by the page rendering.</p>

<h2 class="mt-8 text-lg font-medium text-ink">Both states are legible without motion</h2>
<p class="mt-1 max-w-2xl text-sm text-ink-muted">The invariant: the animation is a transition <em>between</em> two readable states, never the thing that makes them readable. Here are the two end states as static pictures — what a reduced-motion reader, a screenshot, or a printout sees. The accent layer belongs to state A; the positive (green) layer, to state B.</p>
<div class="mt-4 grid gap-4 sm:grid-cols-2">
  <figure class="rounded-lg border border-line bg-surface p-4">
    <figcaption class="mb-3 font-mono text-xs text-ink-muted">state A — <code>motion.RootClass(false)</code></figcaption>
    {{.StaticSVGA}}
  </figure>
  <figure class="rounded-lg border border-line bg-surface p-4">
    <figcaption class="mb-3 font-mono text-xs text-ink-muted">state B — <code>motion.RootClass(true)</code></figcaption>
    {{.StaticSVGB}}
  </figure>
</div>

<h2 class="mt-10 text-lg font-medium text-ink">Live — flip the state</h2>
<p class="mt-1 max-w-2xl text-sm text-ink-muted">The same two grids, driven by the optional <code class="font-mono">motion.js</code> helper (<code class="font-mono">AppKitMotion.toggle</code>). Columns stagger by index (<code class="font-mono">ak-step-1…8</code>); the SVG's B layer trails one full wave (<code class="font-mono">ak-after</code>). Turn on “reduce motion” in your OS and the swap is instant — both states still correct, only the tween gone.</p>
<label class="mt-4 inline-flex cursor-pointer items-center gap-2 rounded-md border border-line bg-surface-raised px-3 py-1.5 text-sm text-ink">
  <input type="checkbox" class="accent-accent" onchange="AppKitMotion.toggle(document.getElementById('demo-svg'), this.checked); AppKitMotion.toggle(document.getElementById('demo-html'), this.checked)">
  Play state A ⇄ B
</label>
<div class="mt-4 grid gap-4 lg:grid-cols-2">
  <figure class="rounded-lg border border-line bg-surface p-4">
    <figcaption class="mb-3 font-mono text-xs text-ink-muted">SVG marks — <code>ak-tone</code> layers, <code>ak-tint</code></figcaption>
    {{.LiveSVG}}
  </figure>
  <figure class="rounded-lg border border-line bg-surface p-4">
    <figcaption class="mb-3 font-mono text-xs text-ink-muted">HTML cards — <code>ak-when-a/-b</code>, <code>ak-fade</code></figcaption>
    {{.LiveHTML}}
  </figure>
</div>
<p class="mt-4 max-w-2xl text-xs text-ink-faint">Both demos set <code class="font-mono">--ak-motion-dim:.14</code> so the receding state recedes rather than vanishes (the default is <code class="font-mono">0</code>). Wire the toggle any way you like — a checkbox, a signal, a server-rendered initial state; <code class="font-mono">motion.js</code> is a convenience, <code class="font-mono">motion.css</code> is the contribution.</p>
<script>{{.Script}}</script>`

// buildAgentic assembles the Agentic & chrome page — the kit's human-in-the-loop
// UI (ApprovalCard, RunTrace) for putting a person in an agent's decision path
// and showing "what's happening under the hood", plus the structural chrome
// comforts every app expects.
func buildAgentic() (template.HTML, error) {
	h := func(s string) template.HTML { return template.HTML(s) }

	approve := hitl.RenderApprovalCard(hitl.ApprovalCard{
		Intent:        "Send email",
		Title:         "Send the deposit-demand letter to the landlord?",
		SummaryHTML:   template.HTML(`Drafts a demand for the <strong>$2,400</strong> withheld deposit, citing the late itemization under § 1950.5. Attaches the move-out photos.`),
		Risk:          hitl.RiskMedium,
		Confidence:    0.82,
		HasConfidence: true,
		Actions: template.HTML(`<form><button class="rounded-md bg-accent px-3 py-1.5 text-sm text-accent-ink">Approve &amp; send</button> ` +
			`<button class="rounded-md border border-line px-3 py-1.5 text-sm text-ink">Edit draft</button> ` +
			`<button class="rounded-md px-3 py-1.5 text-sm text-danger">Reject</button></form>`),
	})

	trace := hitl.RunTrace([]hitl.TraceStep{
		{Label: "Read case file", Detail: "12 evidence spans, 2 theories", At: "10:02:14", State: hitl.TraceDone},
		{Label: "Drafted demand letter", Detail: "cited § 1950.5; attached photos", At: "10:02:18", State: hitl.TraceDone},
		{Label: "Awaiting human approval", Detail: "medium-risk action paused for review", At: "10:02:19", State: hitl.TraceActive},
		{Label: "Send email", State: hitl.TracePending},
		{Label: "Record in case ledger", State: hitl.TracePending},
	})

	d := agenticData{
		Approve: h(approve),
		Trace:   h(trace),
		Empty: h(chrome.RenderEmptyState(chrome.EmptyState{
			Heading: "No sources registered",
			Body:    "Register a local directory to begin ingesting evidence.",
			Action:  template.HTML(`<a href="#" class="text-sm text-accent">Register a source →</a>`),
		})),
		Crumbs: h(chrome.Breadcrumbs([]chrome.Crumb{
			{Label: "Cases", Href: "#"}, {Label: "Doe v. Roe", Href: "#"}, {Label: "Scenarios"},
		})),
		Tabs: h(chrome.Tabs([]chrome.Tab{
			{Label: "Proportional", Href: "#", Active: true},
			{Label: "Condensed", Href: "#"},
			{Label: "Sequence", Href: "#"},
		}, "Timeline view")),
		Paginate: h(chrome.RenderPagination(chrome.Pagination{Page: 3, TotalPages: 8, AriaLabel: "evidence pages"})),
		Skeleton: h(chrome.SkeletonCard(3)),
		Disclosure: h(chrome.RenderDisclosure(chrome.Disclosure{
			Summary: "Show 3 more elements",
			Body:    template.HTML(`<p>Hidden detail revealed on demand — progressive disclosure with zero JS (native &lt;details&gt;).</p>`),
		})),
		DescList: h(chrome.RenderDescriptionList([]chrome.DescItem{
			{Term: "Value", Detail: "Security deposit of $2,400"},
			{Term: "Tier", Detail: "documentary · 92%"},
			{Term: "Source", Detail: "lease-addendum.pdf · p.1"},
		})),
		Diff: h(hitl.DiffView([]hitl.Change{
			{Label: "Disposition", Before: "open", After: "settled"},
			{Label: "Amount", Before: "$2,400", After: "$3,100"},
			{Label: "Tag", After: "demand-sent"},
		})),
		ToolCall: h(hitl.RenderToolCall(hitl.ToolCall{
			Name: "search_evidence", Status: hitl.ToolOK, Summary: "12 spans matched", Duration: "320ms",
			Args:   "{\n  \"query\": \"security deposit withheld\",\n  \"tier\": \"documentary\"\n}",
			Result: "12 results — top: lease-addendum.pdf p.1 (92%)",
		})),
		Chat:   h(galleryChat()),
		Script: template.JS(chat.Script()),
	}
	return htmlf("agentic", agenticHTML, d)
}

// galleryChat renders a representative chat surface: a transcript (with an
// assistant turn that shows its tool call inline — "under the hood" in the
// thread) plus the composer, inside a fixed-height shell.
func galleryChat() string {
	transcript := chat.RenderTranscript([]chat.Message{
		{ID: "u1", Role: chat.RoleUser, AuthorName: "You", At: "10:02", BodyHTML: "Is the deposit demand supported by documentary evidence?"},
		{
			ID: "a1", Role: chat.RoleAssistant, AuthorName: "claude-opus-4-8", AuthorIsAI: true, At: "10:02",
			BodyHTML:  template.HTML(`Yes — the lease addendum (p.1, 92%) records the <strong>$2,400</strong> deposit, and the itemization was delivered 14 days late under § 1950.5.`),
			ToolCalls: []hitl.ToolCall{{Name: "search_evidence", Status: hitl.ToolOK, Summary: "12 spans matched", Duration: "320ms", Result: "top: lease-addendum.pdf p.1 (92%)"}},
			Citations: []chat.Citation{
				{ID: "span-7", Kind: "span", Label: "lease-addendum.pdf · p.1"},
				{ID: "auth-3", Kind: "authority", Label: "Cal. Civ. Code § 1950.5"},
				{ID: "x-9", Kind: "span", Label: "unmatched ref", Unresolved: true},
			},
			SafetyFlags: []chat.SafetyFlag{{Severity: "low", RuleID: "PI-002", Section: "tool result", Excerpt: "(none material)"}},
			Tokens:      "1,240 tokens · $0.02",
			Copyable:    true,
		},
	}, chat.TranscriptOptions{AriaLabel: "Example conversation"})
	composer := chat.RenderComposer(chat.Composer{
		PostURL:     "#",
		DraftKey:    "gallery-chat-draft",
		Stop:        true,
		Hint:        "Enter to send · Shift+Enter for a newline",
		ModelPicker: template.HTML(`<select aria-label="Model" class="rounded-md border border-line bg-surface-raised px-2 py-2 text-sm text-ink"><option>Opus</option><option>Sonnet</option></select>`),
	})
	return `<div class="flex h-[28rem] flex-col overflow-hidden rounded-lg border border-line bg-surface">` + transcript + composer + `</div>`
}

const agenticHTML = `
<h1 class="text-2xl font-semibold text-ink">Agentic &amp; chrome</h1>
<p class="mt-2 max-w-2xl text-ink-muted">Human-in-the-loop UI for agentic apps — put a person in the decision path and show what's happening under the hood — plus the structural creature comforts every app expects. All server-rendered, token-themed, loopback-safe.</p>

<h2 class="mt-8 text-lg font-medium text-ink">Approval card — <code class="font-mono text-sm">hitl.ApprovalCard</code></h2>
<p class="mt-1 max-w-2xl text-sm text-ink-muted">A proposed agent action awaiting a decision: intent + risk badges, a confidence pip rating, a rich summary, and caller-supplied action HTML (the app wires the loopback POST — the kit imposes no transport).</p>
<div class="mt-4 max-w-xl">{{.Approve}}</div>

<h2 class="mt-10 text-lg font-medium text-ink">Run trace — <code class="font-mono text-sm">hitl.RunTrace</code></h2>
<p class="mt-1 max-w-2xl text-sm text-ink-muted">A vertical event rail of an agent run — done / active / failed / pending — the "what's going on under the hood" view.</p>
<div class="mt-4 max-w-xl rounded-lg border border-line bg-surface-raised p-5">{{.Trace}}</div>

<h2 class="mt-10 text-lg font-medium text-ink">Diff — <code class="font-mono text-sm">hitl.DiffView</code></h2>
<p class="mt-1 max-w-2xl text-sm text-ink-muted">Before → after review of a proposed agent change — current value (danger) vs proposed (positive); a missing side reads as an addition or removal. Pair with the approval card to let a person approve or reject.</p>
<div class="mt-4 max-w-xl rounded-lg border border-line bg-surface-raised p-5">{{.Diff}}</div>

<h2 class="mt-10 text-lg font-medium text-ink">Tool call — <code class="font-mono text-sm">hitl.ToolCall</code></h2>
<p class="mt-1 max-w-2xl text-sm text-ink-muted">Inspect a single agent tool invocation — name, status, and the arguments &amp; result under a collapsible (native <code class="font-mono">&lt;details&gt;</code>, no JS). The "what did the agent actually do" view.</p>
<div class="mt-4 max-w-xl">{{.ToolCall}}</div>

<h2 class="mt-10 text-lg font-medium text-ink">Chat — <code class="font-mono text-sm">appkit/ui/chat</code></h2>
<p class="mt-1 max-w-2xl text-sm text-ink-muted">A reusable conversational surface: a transcript of role-styled bubbles + a composer. "Under the hood" lives <em>in</em> the thread — the assistant turn shows its <code class="font-mono">hitl.ToolCall</code> inline. The creature comforts (auto-grow, Enter-to-send, draft persistence, autoscroll + jump-to-latest, copy, stop/regenerate) come from a dependency-free <code class="font-mono">chat.js</code> (served like <code class="font-mono">overlay.js</code>); streaming is the app's own SSE driving <code class="font-mono">AppKitChat.appendDelta/finalize</code>. Loopback-safe — the app owns every endpoint.</p>
<div class="mt-4 max-w-xl">{{.Chat}}</div>

<h2 class="mt-10 text-lg font-medium text-ink">Structural chrome — <code class="font-mono text-sm">appkit/ui/chrome</code></h2>
<p class="mt-1 max-w-2xl text-sm text-ink-muted">Breadcrumbs, tabs, pagination, empty states, and loading skeletons — plain links/markup, no client JS.</p>
<div class="mt-4 space-y-5">
  <div>{{.Crumbs}}</div>
  <div>{{.Tabs}}</div>
  <div>{{.Paginate}}</div>
  <div class="grid gap-4 sm:grid-cols-2">
    <div>{{.Empty}}</div>
    <div>{{.Skeleton}}</div>
  </div>
  <div>{{.Disclosure}}</div>
  <div class="rounded-lg border border-line bg-surface-raised p-4">{{.DescList}}</div>
</div>
<script>{{.Script}}</script>`

// formsData is the Forms page view model: representative fields + buttons.
type formsData struct {
	Text     template.HTML
	Select   template.HTML
	Textarea template.HTML
	Checkbox template.HTML
	Radios   template.HTML
	Invalid  template.HTML
	Buttons  []template.HTML
}

// buildForms assembles the Forms page — the appkit/ui/form vocabulary every app
// needs: labeled fields with help/error, inputs, selects, textareas, checkboxes,
// radio groups, and buttons. All token-themed, accessible, server-rendered.
func buildForms() (template.HTML, error) {
	h := func(s string) template.HTML { return template.HTML(s) }
	d := formsData{
		Text: h(form.RenderField(form.Field{
			Label: "Case name", For: "case-name", Required: true,
			Help:    "As it appears on the caption.",
			Control: template.HTML(form.RenderInput(form.Input{ID: "case-name", Name: "name", Placeholder: "Smith v. Jones", Required: true, HasHelp: true})),
		})),
		Select: h(form.RenderField(form.Field{
			Label: "Evidence tier", For: "tier",
			Control: template.HTML(form.RenderSelect(form.Select{
				ID: "tier", Name: "tier", Placeholder: "Select tier…", Value: "documentary",
				Options: []form.Option{
					{Value: "direct", Label: "Direct"},
					{Value: "documentary", Label: "Documentary"},
					{Value: "testimonial", Label: "Testimonial"},
				},
			})),
		})),
		Textarea: h(form.RenderField(form.Field{
			Label: "Notes", For: "notes", Help: "Where is the evidence? File path, binder tab, etc.",
			Control: template.HTML(form.RenderTextarea(form.Textarea{ID: "notes", Name: "notes", Rows: 3, Placeholder: "Optional notes…", HasHelp: true})),
		})),
		Checkbox: h(form.RenderCheckbox(form.Checkbox{ID: "treble", Name: "treble", Label: "Apply statutory treble (if the jurisdiction provides it)", Checked: true})),
		Radios: h(form.RenderRadioGroup(form.RadioGroup{
			Name: "has-evidence", Legend: "Do you have evidence for this element?", Value: "yes", Inline: true,
			Options: []form.Option{{Value: "yes", Label: "Yes"}, {Value: "no", Label: "No"}},
			Help:    "Answering Yes lets the matcher propose links once the record is ingested.",
		})),
		Invalid: h(form.RenderField(form.Field{
			Label: "Amount", For: "amt", Required: true,
			Error:   "Enter an amount greater than zero.",
			Control: template.HTML(form.RenderInput(form.Input{ID: "amt", Name: "amount", Type: "number", Value: "0", Required: true, Invalid: true})),
		})),
		Buttons: []template.HTML{
			h(form.RenderButton(form.Button{Label: "Save changes", Variant: form.ButtonPrimary})),
			h(form.RenderButton(form.Button{Label: "Cancel", Variant: form.ButtonGhost, Href: "#"})),
			h(form.RenderButton(form.Button{Label: "Delete case", Variant: form.ButtonDanger})),
		},
	}
	return htmlf("forms", formsHTML, d)
}

const formsHTML = `
<h1 class="text-2xl font-semibold text-ink">Forms</h1>
<p class="mt-2 max-w-2xl text-ink-muted">The <code class="font-mono text-accent">appkit/ui/form</code> vocabulary every app needs — labeled fields with help/error text, inputs, selects, textareas, checkboxes, radio groups, and buttons. Token-themed, accessible (label association, <code class="font-mono">aria-describedby</code>, <code class="font-mono">aria-invalid</code>, fieldset/legend), server-rendered, escaped. No Node.</p>

<div class="mt-6 grid gap-6 lg:grid-cols-2">
  <div class="rounded-lg border border-line bg-surface-raised p-5">
    <h2 class="mb-4 text-lg font-medium text-ink">Fields</h2>
    <div class="flex flex-col gap-4">
      {{.Text}}
      {{.Select}}
      {{.Textarea}}
      {{.Checkbox}}
    </div>
  </div>
  <div class="rounded-lg border border-line bg-surface-raised p-5">
    <h2 class="mb-4 text-lg font-medium text-ink">Choice &amp; validation</h2>
    <div class="flex flex-col gap-4">
      {{.Radios}}
      {{.Invalid}}
    </div>
    <h3 class="mt-6 mb-3 text-sm font-medium text-ink">Buttons</h3>
    <div class="flex flex-wrap items-center gap-3">{{range .Buttons}}{{.}}{{end}}</div>
  </div>
</div>`

// overlaysData is the Overlays page view model.
type overlaysData struct {
	Triggers []template.HTML // open buttons + a live-toast button
	Dialog   template.HTML
	Drawer   template.HTML
	Toasts   []template.HTML // static role swatches (rendered inline, not fixed)
	Script   template.JS
}

// buildOverlays assembles the Overlays page — modal Dialog, slide-over Drawer,
// and Toast, on native <dialog> + the dependency-free overlay.js. The script is
// inlined so the demos work in the static catalog.
func buildOverlays() (template.HTML, error) {
	h := func(s string) template.HTML { return template.HTML(s) }
	d := overlaysData{
		Triggers: []template.HTML{
			h(overlay.TriggerButton("demo-dialog", "Open dialog")),
			h(overlay.TriggerButton("demo-drawer", "Open drawer")),
			h(`<button type="button" id="ak-toast-demo" class="inline-flex items-center justify-center rounded-md border border-line bg-surface px-3 py-2 text-sm font-medium text-ink hover:bg-surface-sunken focus:outline-none focus:ring-2 focus:ring-focus">Show toast</button>`),
		},
		Dialog: h(overlay.RenderDialog(overlay.Dialog{
			ID: "demo-dialog", Title: "Delete this case?", Size: "sm", LightDismiss: true,
			Body:   template.HTML(`<p>This permanently removes the case and its ledger. This cannot be undone.</p>`),
			Footer: template.HTML(`<button type="button" class="rounded-md border border-line bg-surface px-3 py-2 text-sm text-ink" data-overlay-dismiss>Cancel</button><button type="button" class="rounded-md bg-danger px-3 py-2 text-sm font-medium text-ink-inverse" data-overlay-dismiss>Delete</button>`),
		})),
		Drawer: h(overlay.RenderDrawer(overlay.Drawer{
			ID: "demo-drawer", Title: "Evidence provenance", LightDismiss: true,
			Body:   template.HTML(`<dl class="space-y-3"><div><dt class="text-ink-muted">Value</dt><dd class="text-ink">Security deposit of $2,400</dd></div><div><dt class="text-ink-muted">Tier</dt><dd class="text-ink">documentary · 92%</dd></div><div><dt class="text-ink-muted">Source</dt><dd class="text-ink">lease-addendum.pdf · p.1</dd></div></dl>`),
			Footer: template.HTML(`<button type="button" class="rounded-md border border-line bg-surface px-3 py-2 text-sm text-ink" data-overlay-dismiss>Close</button>`),
		})),
		Toasts: []template.HTML{
			h(overlay.RenderToast(overlay.Toast{Message: "Filing accepted.", Role: "success"})),
			h(overlay.RenderToast(overlay.Toast{Message: "Response due in 9 days.", Role: "caution"})),
			h(overlay.RenderToast(overlay.Toast{Message: "Conflicting testimony detected.", Role: "danger"})),
			h(overlay.RenderToast(overlay.Toast{Message: "Synced 12 records.", Role: "info"})),
		},
		Script: template.JS(overlay.Script() +
			`;document.getElementById('ak-toast-demo').addEventListener('click',function(){window.AppKitOverlay.toast({message:'Saved — changes recorded.',role:'success'});});`),
	}
	return htmlf("overlays", overlaysHTML, d)
}

const overlaysHTML = `
<h1 class="text-2xl font-semibold text-ink">Overlays</h1>
<p class="mt-2 max-w-2xl text-ink-muted">Modal <code class="font-mono text-accent">Dialog</code>, slide-over <code class="font-mono text-accent">Drawer</code>, and <code class="font-mono text-accent">Toast</code> — built on the native <code class="font-mono">&lt;dialog&gt;</code> element + a dependency-free <code class="font-mono">overlay.js</code> (served like <code class="font-mono">router.js</code>). Loopback-safe: the markup is meaningful without JS (native dialog + Escape); the script only adds open-by-id, light-dismiss, and client toasts.</p>

<div class="mt-6 flex flex-wrap items-center gap-3">{{range .Triggers}}{{.}}{{end}}</div>

<h2 class="mt-10 text-lg font-medium text-ink">Toast roles</h2>
<p class="mt-1 text-sm text-ink-muted">Server-rendered after a POST-redirect (the flash pattern, no JS), or spawned client-side via <code class="font-mono">AppKitOverlay.toast()</code>. Shown inline here:</p>
<div class="mt-4 flex flex-col gap-2">{{range .Toasts}}{{.}}{{end}}</div>

{{.Dialog}}
{{.Drawer}}
<script>{{.Script}}</script>`

// sanitizeTheme constrains the ?theme= param to the values the contract allows.
func sanitizeTheme(v string) string {
	switch v {
	case "light", "dark":
		return v
	default:
		return "" // OS-follow
	}
}
