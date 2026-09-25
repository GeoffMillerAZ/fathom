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

	"github.com/geoffmilleraz/app-kit/appkit/ui/callout"
	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/app-kit/appkit/ui/dashboard"
	"github.com/geoffmilleraz/app-kit/appkit/ui/form"
	"github.com/geoffmilleraz/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
	"github.com/geoffmilleraz/app-kit/appkit/ui/vizframe"
	"github.com/geoffmilleraz/fathom/internal/console"
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
	return `<span class="italic text-ink-muted">` + html.EscapeString(absentNote) + `</span>`
}

// observationsCell renders a repo's open-observation count, "no
// observations file" when it has none, or the read error — in the error
// role — when the file exists and could not be read (never labelled
// missing: the survey found EPERM-wedged repos reading "no observations
// file").
func observationsCell(s fleetServiceRow) string {
	if s.OpenObservations == nil && s.ObservationsError != "" {
		return `<span class="text-danger" data-truth-state="error">unreadable — ` + html.EscapeString(s.ObservationsError) + `</span>`
	}
	return metricCell(s.OpenObservations, "no observations file", "")
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
	return `<div class="flex min-w-0 justify-between gap-2 py-0.5"><span class="shrink-0 text-ink-muted">` + html.EscapeString(k) + `</span><span class="min-w-0 text-right [overflow-wrap:anywhere]">` + v + `</span></div>`
}

// agentCallsTip wraps an "N agent" fragment with a learn-affordance
// (dotted underline + title): a first-time reader has no way to know what
// "agent calls" means versus fathom's own plumbing without it.
func agentCallsTip(n int) string {
	return `<span class="cursor-help underline decoration-dotted decoration-line underline-offset-2" title="agent calls = queries an AI agent chose to make (find_symbol, read_source, ...). The rest is fathom's own plumbing: hooks checking in, gates verifying plans.">` + fmtThousands(n) + ` agent</span>`
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
	"fleet":  {"Every repo fathom indexes, how well each is wired, and the exact command that fixes anything amber.", "Each tile is one repository. The letter is a wiring grade over the signals visible from the repo itself (hook wiring, docs stanza, index freshness, recorded traffic — signals that cannot be checked from here are excluded rather than counted against it). Sparklines show two weeks of query traffic. Amber monospace lines are copy-paste remedies. Grades: A = fully wired (hooks + stanza + fresh index + real traffic) · B = one signal missing · C/D = mostly unwired · index-only = a remote corpus other repos query (no sessions run here, so wiring is not expected)."},
	"impact": {"What fathom saved, prevented, and where it still falls short — every number names its instrument and its denominator.", "The exploration story compares what a discovery question costs when answered by raw file reading versus the index, measured inside the same sessions with the same estimator. 'Verify catches' are pieces of work an agent claimed finished that the verification gate proved were not. 'Obligations served' are project rules injected right before an agent edits a governed file. A dash with a note means the instrument has not reported — this dashboard never renders a number it did not measure."},
	"story":  {"The last 7 days composed from the instruments, with per-session replays.", "Every session an agent runs leaves a ledger row; this page folds the recent ones and links each session's replay timeline. Narrative here is a lens for humans — the evidence lives in the instruments."},
	"brief":  {"The shareable summary — same numbers, framed for someone who has never heard of fathom.", "This is the page to show your manager. It is generated from the live instruments, states what is NOT measured, and can be downloaded as markdown. Nothing on it is ever sent anywhere by fathom — sharing is a human decision."},
}

// obsNavigation is the console's grouped sidebar (OC-4) with the page for
// the active key (an ID or a legacy alias: "story" is the sessions page)
// marked current. The pinned kit owns link markup, current-page matching,
// focus and target-size treatment; the registry owns the destinations.
func obsNavigation(active string) string {
	reg := defaultConsoleRegistry()
	return dashboard.RenderNav(reg.navGroups(), reg.navCurrentPath(active))
}

// obsContent is a model-backed page's content: its plain-English lead and
// explainer, the arm/narrate dials and any degraded-registry banner, then
// the body.
func obsContent(active string, body string, m observatoryModel) string {
	var b strings.Builder
	if l, ok := obsLeads[active]; ok {
		b.WriteString(`<p class="mb-2 max-w-3xl text-ink-muted">` + html.EscapeString(l.Lead) + `</p>`)
		if l.Explain != "" {
			b.WriteString(`<details class="mb-3 max-w-3xl rounded-md border border-line bg-surface-raised px-3 py-2"><summary class="min-h-11 cursor-pointer py-2.5 text-accent">what am I looking at?</summary><p class="mt-2 text-ink-muted">` + html.EscapeString(l.Explain) + `</p></details>`)
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
	return b.String()
}

// obsPage renders a model-backed page on the console shell (OC-4). active
// is a registered page ID or a legacy alias; title is the legacy
// "fathom · …" title, reduced to its subject for the page heading.
func obsPage(title, active string, body string, m observatoryModel) string {
	return obsPageWithCrumbs(title, active, nil, body, m)
}

// obsPageWithCrumbs is obsPage with an explicit trail (nil: the page's own).
func obsPageWithCrumbs(title, active string, crumbs []chrome.Crumb, body string, m observatoryModel) string {
	reg := defaultConsoleRegistry()
	heading := strings.TrimPrefix(title, "fathom · ")
	question := ""
	pageID := ""
	if p, ok := reg.page(active); ok {
		pageID = p.ID
		question = p.Question
		if heading == "fathom observatory" || heading == "fathom" || heading == "" {
			heading = p.Title
		}
	}
	footer := `<p class="mx-auto max-w-7xl text-xs text-ink-muted">generated ` + html.EscapeString(m.GeneratedAt) + ` · fathom ` + html.EscapeString(m.FathomVersion) + ` · loopback-only; sharing is a human verb (law 13) — <code class="font-mono">fathom brief</code> writes the shareable file · JSON: add <code class="font-mono">?format=json</code> to any page</p>`
	if crumbs == nil {
		crumbs = consoleCrumbs(reg, pageID, heading)
	}
	return renderConsoleShell(reg, consoleShell{
		PageID: pageID, Title: heading, Crumbs: crumbs,
		Question: question, Meta: modelMeta(m), Body: obsContent(active, body, m), Footer: footer,
	})
}

// sectionHeading renders a muted uppercase section label.
func sectionHeading(s string) string {
	return `<h2 class="mb-2 mt-6 text-xs font-semibold uppercase tracking-wider text-ink-muted">` + html.EscapeString(s) + `</h2>`
}

// absentCell renders a kv-row value whose source could not be read: the
// named reason in the house absence style (metricCell's own), never a
// digit (RG-8, law 3).
func absentCell(reason string) string {
	// Reasons are prose, not numeric values. Keep their reading edge stable
	// when a long path wraps inside a right-aligned key/value column.
	return `<span class="block text-left">` + metricCell(nil, reason, "") + `</span>`
}

// fleetFreshnessStat projects known observations and their coverage together.
// An unavailable registry cannot establish a fleet total, even with populated
// rows: the model carries no independent provenance for those rows.
func fleetFreshnessStat(m observatoryModel) stat.Stat {
	if m.DegradedNote != "" {
		return stat.Absent("index freshness", "freshness unavailable: "+m.DegradedNote)
	}
	fresh, stale, unknown := 0, 0, 0
	unknownReason := ""
	for _, s := range m.Services {
		switch {
		case s.Freshness == "fresh":
			fresh++
		case s.Freshness == "stale" || strings.HasPrefix(s.Freshness, "stale:"):
			stale++
		default:
			unknown++
			if unknownReason == "" {
				unknownReason = s.Freshness
				if unknownReason == "" {
					unknownReason = "unknown: no freshness state reported"
				}
			}
		}
	}
	coverage := fmt.Sprintf("%d of %d listed repositories known: %d fresh, %d stale, %d unknown", fresh+stale, len(m.Services), fresh, stale, unknown)
	if unknown > 0 {
		coverage += "; first unknown reason: " + unknownReason
	}
	if stale > 0 {
		value := plural(stale, "stale repo")
		if unknown > 0 {
			value = "≥ " + value
		}
		return stat.Stat{Label: "index freshness", Value: value, ValueRole: "caution", Sub: coverage}
	}
	if len(m.Services) == 0 {
		return stat.Absent("index freshness", "freshness unknown: no services available to evaluate")
	}
	if unknown > 0 {
		return stat.Absent("index freshness", "freshness unknown; "+coverage)
	}
	return stat.Stat{Label: "index freshness", Value: "all fresh", ValueRole: "positive", Sub: coverage}
}

// A failed fleet read cannot turn an absent registry into a measured zero.
// Rows retained through a partial read are a lower bound, not a fleet total.
// The count is the shared projection (console.ServiceCount, OC-9) so the
// fleet, home and every page's JSON agree.
func fleetServiceCountStat(m observatoryModel) stat.Stat {
	return valueStat("services indexed", console.ServiceCount(fleetFactsFromModel(m)))
}

func renderObsFleet(m observatoryModel) string {
	// RG-8 / OC-9: a tile whose source was not read is Absent with the
	// reason — the registry (symbols) and the call log (tool calls) each
	// carry their own read state, and a sum over nothing is not a zero.
	// Both are the shared projections: the call total is every row in the
	// resolved log, unattributed rows named in the caption, never dropped.
	symbolsTile := valueStat("symbols", console.SymbolTotal(fleetFactsFromModel(m)))
	callFacts := modelCallFacts(m)
	callsTile := valueStat("tool calls (log)", console.CallTotal(callFacts))
	if caption := callLogRuleCaption(callFacts); caption != "" {
		callsTile.Sub = strings.TrimPrefix(callsTile.Sub+" · "+caption, " · ")
	}
	var b strings.Builder
	b.WriteString(stat.RenderBand([]stat.Stat{
		fleetServiceCountStat(m),
		symbolsTile,
		callsTile,
	}, stat.BandOptions{AriaLabel: "fleet totals"}))
	// The pinned kit's five-column band clips evidence captions beside the
	// desktop sidebar at tablet widths. Compose bounded groups with its existing
	// API; do not change the stamped mirror to repair consumer layout.
	b.WriteString(stat.RenderBand([]stat.Stat{
		fleetFreshnessStat(m),
		fleetOpenObservations(m),
	}, stat.BandOptions{AriaLabel: "fleet evidence status"}))

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
				// The kit's SVG has a fixed width; let it scale down at 390 px
				// rather than push the page sideways.
				Body: template.HTML(`<div class="[&>svg]:h-auto [&>svg]:max-w-full">` + viz.Sparkline(spark, viz.SparklineOptions{W: 640, H: 48, FillRole: "accent-soft"}) + `</div>`),
			}))
		}
	}

	b.WriteString(renderGradeDistribution(m))
	var widgets []string
	for _, s := range m.Services {
		var body strings.Builder
		if s.SymbolsNote != "" {
			body.WriteString(kv("symbols", absentCell(s.SymbolsNote)))
		} else {
			body.WriteString(kv("symbols", `<span class="font-semibold tabular-nums">`+fmtThousands(s.Symbols)+`</span>`))
		}
		fr := `<span class="text-positive">fresh</span>`
		if strings.HasPrefix(s.Freshness, "stale") {
			fr = `<span class="text-caution">` + html.EscapeString(shortFreshness(s.Freshness)) + `</span>`
		} else if s.Freshness != "fresh" {
			fr = `<span class="italic text-ink-muted">` + html.EscapeString(s.Freshness) + `</span>`
		}
		body.WriteString(kv("freshness", fr))
		if m.CallLogNote != "" {
			body.WriteString(kv("traffic", absentCell(m.CallLogNote)))
		} else {
			body.WriteString(kv("traffic", html.EscapeString(plural(s.CallsTotal, "call"))+` <span class="text-ink-muted">(`+agentCallsTip(s.CallsAgent)+`)</span>`))
		}
		body.WriteString(kv("open observations", observationsCell(s.fleetServiceRow)))
		if m.CallLogNote == "" && len(s.TrafficDays) > 0 && s.CallsTotal > 0 {
			spark := make([]float64, len(s.TrafficDays))
			for i, c := range s.TrafficDays {
				spark[i] = float64(c)
			}
			body.WriteString(`<div class="mt-2">` + viz.Sparkline(spark, viz.SparklineOptions{W: 220, H: 24, FillRole: "accent-soft", AriaLabel: s.Service + " 14-day traffic"}) + `</div>`)
		}
		// indexOnly (owner ask 2026-08-08): a C/D grade with zero ledger
		// sessions AND zero agent-originated calls is a remote corpus other
		// repos query, not a failure — the letter grade only applies to
		// repos where coding sessions actually run, so the remedy (which
		// would say "open a session here") is moot and gets suppressed too.
		indexOnly := (s.Grade == "C" || s.Grade == "D") && s.SessionCount != nil && *s.SessionCount == 0 && s.CallsAgent == 0
		if !indexOnly {
			remedyShown := false
			if s.FreshnessRemedy != "" {
				body.WriteString(remedy(s.FreshnessRemedy))
				remedyShown = true
			}
			if s.WiringRemedy != "" {
				body.WriteString(remedy(s.WiringRemedy))
				remedyShown = true
			}
			// Fallback (the fix): FreshnessRemedy/WiringRemedy only cover
			// the stale-index and missing-hooks/stanza cases. A C-grade
			// card whose only unmet signal is traffic ("wired but unused")
			// matched neither condition and rendered NO remedy at all —
			// wiringGrade's own GradeRaise is the general case; use it
			// whenever the specific fields above didn't already cover it.
			if !remedyShown && s.Grade != "A" && s.GradeRaise != "" {
				body.WriteString(remedy(s.GradeRaise))
			}
		}
		var gradeChip string
		if indexOnly {
			gradeChip = `<span class="mr-2 inline-block rounded-full bg-surface-sunken px-2 py-0.5 text-xs font-semibold text-ink-muted" title="This repo is indexed and queried from other repos&#39; sessions, but no one runs coding sessions inside it — wiring hooks here would add nothing. The grade letters apply to repos where sessions run.">index-only</span>`
		} else {
			gradeRole := map[string]string{"A": "positive", "B": "positive", "C": "caution", "D": "caution", "F": "danger"}[s.Grade]
			gradeChip = `<span class="mr-2 inline-block rounded-full bg-` + gradeRole + `-soft px-2 py-0.5 text-xs font-semibold text-` + gradeRole + `" title="wiring grade — the continuous rewire audit">` + html.EscapeString(s.Grade) + `</span>`
		}
		widgets = append(widgets, dashboard.RenderWidget(dashboard.Widget{
			Title:  s.Service,
			Action: template.HTML(gradeChip + `<a class="inline-flex min-h-11 items-center px-2 text-accent underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus" href="` + html.EscapeString(projectHref(s.Service)) + `">detail →</a>`),
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
		// The chart keeps its legible 11 px labels and scrolls inside its own
		// keyboard-reachable region at 390 px (scaling the fixed 560 px SVG
		// down drew them at ~6 px); its table alternative sits behind a
		// disclosure (design §2: every chart has one).
		rows := make([]table.Row, 0, len(bars))
		for i, bar := range bars {
			rows = append(rows, table.Row{ID: "tool-" + itoa(i), Cells: []string{bar.Label, fmtThousands(int(bar.Value))}})
		}
		alt := chrome.RenderDisclosure(chrome.Disclosure{
			Summary: "calls per tool as a table",
			Body: template.HTML(renderKitTable(table.TableParams{
				TableID: "fleet-tool-load", Caption: "calls per tool (call log, all time)",
				Columns: []table.Column{{Key: "tool", Label: "Tool"}, {Key: "calls", Label: "Calls"}},
				Rows:    rows,
			})),
		})
		b.WriteString(dashboard.RenderWidget(dashboard.Widget{
			Body: template.HTML(`<div class="overflow-x-auto" role="region" tabindex="0" aria-label="calls per tool — scrolls sideways">` + viz.Bars(bars, viz.BarsOptions{W: 560, LabelW: 160, ShowValue: true, AriaLabel: "calls per tool"}) + `</div><div class="mt-3">` + alt + `</div>`),
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
		if s.SymbolsNote != "" {
			body.WriteString(kv("symbols", absentCell(s.SymbolsNote)))
		} else {
			body.WriteString(kv("symbols", fmtThousands(s.Symbols)))
		}
		body.WriteString(kv("freshness", html.EscapeString(s.Freshness)))
		body.WriteString(kv("indexed commit", `<span class="font-mono text-xs">`+html.EscapeString(s.IndexedCommit)+`</span>`))
		body.WriteString(kv("repo HEAD", `<span class="font-mono text-xs">`+html.EscapeString(s.RepoHead)+`</span>`))
		body.WriteString(kv("hooks", html.EscapeString(s.HooksNote)))
		body.WriteString(kv("stanza", html.EscapeString(s.StanzaNote)))
		if m.CallLogNote != "" {
			body.WriteString(kv("traffic", absentCell(m.CallLogNote)))
		} else {
			body.WriteString(kv("traffic", plural(s.CallsTotal, "call")+` <span class="text-ink-muted">(`+agentCallsTip(s.CallsAgent)+` · `+fmtThousands(s.CallsAgent)+`/`+fmtThousands(s.CallsTotal)+`)</span>`))
		}
		body.WriteString(kv("open observations", observationsCell(s.fleetServiceRow)))
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
				cfg.WriteString(kv("semantic layers", `<span class="italic text-ink-muted">`+html.EscapeString(c.LayersNote)+`</span>`))
			}
			if c.GuardCount > 0 {
				cfg.WriteString(kv("guards", fmtThousands(c.GuardCount)+` pre-edit warnings configured`))
			} else {
				cfg.WriteString(kv("guards", `<span class="italic text-ink-muted">`+html.EscapeString(c.GuardNote)+`</span>`))
			}
			cfg.WriteString(kv("bindings", html.EscapeString(c.BindingNote)))
			if m.CallLogNote != "" {
				cfg.WriteString(kv("rules served here (calls)", absentCell(m.CallLogNote)))
			} else {
				cfg.WriteString(kv("rules served here (calls)", fmtThousands(c.ObligationServes)))
			}
			cfg.WriteString(kv("obligations compile", `<span class="text-xs">`+html.EscapeString(s.CompileNote)+`</span>`))
		}
		configuration := dashboard.RenderWidget(dashboard.Widget{Title: "configuration (layers · guards · bindings)", Body: template.HTML(cfg.String())})
		if s.Config == nil {
			configuration = callout.Render(callout.Callout{
				Kind:  callout.Info,
				Title: "Configuration snapshot unavailable",
				Body:  "This snapshot does not contain configuration evidence. Installed layers, guards and bindings are unknown here, not confirmed absent.",
			})
		}
		widgets := []string{
			dashboard.RenderWidget(dashboard.Widget{Title: id, Body: template.HTML(body.String())}),
			configuration,
		}
		switch {
		case m.CallLogNote != "":
			widgets = append(widgets, dashboard.RenderWidget(dashboard.Widget{Title: "agent-surface share", Body: template.HTML(`<span class="italic text-ink-muted">not measured — ` + html.EscapeString(m.CallLogNote) + `</span>`)}))
		case s.CallsTotal > 0:
			g := viz.Gauge(float64(s.CallsAgent), float64(s.CallsTotal), viz.GaugeOptions{
				AriaLabel: "agent-surface share",
			})
			// Prose belongs outside the fixed SVG viewport. The existing frame
			// supplies wrapping meaning and provenance in both kit resolutions.
			source := fmtThousands(s.CallsAgent) + " of " + fmtThousands(s.CallsTotal) + " recorded calls; current local call-log snapshot"
			if m.GeneratedAt != "" {
				source += "; snapshot generated " + m.GeneratedAt
			}
			widgets = append(widgets, vizframe.MustRender(g, vizframe.Options{
				ID: "service-agent-share", Title: "agent-surface share",
				Note:   "of this service's recorded calls arrived over the agent surface",
				Source: vizframe.Derived(source),
			}))
		default:
			widgets = append(widgets, dashboard.RenderWidget(dashboard.Widget{Title: "agent-surface share", Body: template.HTML(`<span class="italic text-ink-muted">no recorded calls for this service in the current log</span>`)}))
		}
		widgets = append(widgets, renderDimensionSeries(s, m.CallLogNote))
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

// Obligation serves come from the call log; ledger sessions come from the
// session ledger. Each keeps its own source failure instead of sharing one.
func impactSourceCount(label string, value int, caption, sourceFailure string) stat.Stat {
	if sourceFailure != "" {
		return stat.Absent(label, sourceFailure)
	}
	return stat.Stat{Label: label, Value: fmtThousands(value), Sub: caption}
}

// renderCostPanel is BA-8's Cost panel (docs/design/budgeted-answers.md
// §6): tokens fathom put into agent contexts by tool (an ESTIMATE — bytes/4,
// labeled every time it renders), the standing schema tax with an honest
// trend absence, elision rate per tool, and the tokens-spent vs
// discovery-served ratio — OBSERVATIONAL, never a savings claim (law 12).
func renderCostPanel(cp costPanel, callLogNote string) string {
	var b strings.Builder
	b.WriteString(sectionHeading("cost · what fathom put into your context (estimates, never a savings claim)"))

	schemaTrend := stat.Absent("schema tax trend", cp.SchemaTaxTrendNote)
	ratio := stat.Absent("tokens per discovery action", cp.RatioNote)
	responseCost := stat.Stat{Label: "total response cost (est.)", Value: "~" + fmtThousands(cp.TotalEstTokens) + " tok", Sub: fmtThousands(cp.TotalBytes) + " B measured across accounted calls"}
	// The schema is independently measured; only call-log-derived figures lose
	// their authority when that source is unavailable, even with stale rows present.
	if callLogNote != "" {
		responseCost = stat.Absent("total response cost (est.)", callLogNote)
		responseCost.ValueRole = "ink-muted"
		ratio = stat.Absent("tokens per discovery action", callLogNote)
		ratio.ValueRole = "ink-muted"
	} else if cp.RatioTokensPerDiscoveryX10 != nil {
		whole := *cp.RatioTokensPerDiscoveryX10 / 10
		tenth := *cp.RatioTokensPerDiscoveryX10 % 10
		ratio = stat.Stat{Label: "tokens per discovery action (est.)", Value: fmt.Sprintf("%d.%d", whole, tenth), Sub: cp.RatioNote}
	}
	b.WriteString(stat.RenderBand([]stat.Stat{
		{Label: "agent surface schema tax", Value: fmtThousands(cp.SchemaTaxBytes) + " B", Sub: plural(cp.SchemaTaxTools, "tool") + " carried by every connected session"},
		schemaTrend,
		responseCost,
		ratio,
	}, stat.BandOptions{AriaLabel: "cost"}))

	if callLogNote != "" {
		b.WriteString(dashboard.RenderWidget(dashboard.Widget{Title: "response cost by tool", Body: template.HTML(`<p class="text-ink-muted">` + html.EscapeString(callLogNote) + `</p>`)}))
		return b.String()
	}
	if len(cp.Tools) == 0 {
		b.WriteString(dashboard.RenderWidget(dashboard.Widget{Body: template.HTML(`<span class="italic text-ink-muted">no accounted response-cost rows in the current log</span>`)}))
		return b.String()
	}

	var tbl strings.Builder
	tbl.WriteString(`<div class="overflow-x-auto"><table class="w-full border-collapse text-sm"><tr class="border-b border-line text-left"><th scope="col" class="px-2 py-1 font-medium text-ink-muted">tool</th><th scope="col" class="px-2 py-1 font-medium text-ink-muted">calls</th><th scope="col" class="px-2 py-1 font-medium text-ink-muted">tokens (est.)</th><th scope="col" class="px-2 py-1 font-medium text-ink-muted">elision rate</th></tr>`)
	for i, r := range cp.Tools {
		if i == 10 {
			break
		}
		rate := `<span class="text-ink-faint">—</span>`
		if pct, ok := r.ElisionRatePct(); ok {
			role := "text-ink"
			if pct >= 30 {
				role = "text-caution"
			}
			rate = `<span class="` + role + ` tabular-nums">` + strconv.Itoa(pct) + `%</span>`
		}
		tbl.WriteString(`<tr class="border-b border-line"><td class="px-2 py-1 font-mono text-xs">` + html.EscapeString(r.Tool) + `</td><td class="px-2 py-1 tabular-nums">` + fmtThousands(r.Calls) + `</td><td class="px-2 py-1 tabular-nums">~` + fmtThousands(r.EstTokens) + `</td><td class="px-2 py-1">` + rate + `</td></tr>`)
	}
	tbl.WriteString(`</table></div><div class="mt-2 text-xs text-ink-muted">tokens are an ESTIMATE (response bytes ÷ 4); elision rate is truncating calls only — a tool with no cut shows "—", not 0%.</div>`)
	b.WriteString(dashboard.RenderWidget(dashboard.Widget{Title: "response cost by tool", Body: template.HTML(tbl.String())}))
	return b.String()
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
			// BP-23 / ADR-032 §5: the harness tax, named on the same band as
			// the modes it distorts — never folded into either one.
			{Label: "deferred-schema turns", Value: fmtThousands(imp.ExpToolSearchTurns), Sub: "ToolSearch turns loading MCP schemas — a cost only the indexed mode pays", ValueRole: "caution"},
		}, stat.BandOptions{AriaLabel: "exploration story"}))
		if strings.TrimSpace(imp.ExpNote) != "" {
			b.WriteString(`<div class="mb-4 mt-2 text-xs text-ink-muted">` + html.EscapeString(imp.ExpNote) + `</div>`)
		}
	} else {
		note := imp.ExpNote
		if strings.TrimSpace(note) == "" {
			note = "This snapshot has no comparison with both raw exploration and indexed discovery. This is not evidence of zero cost or savings."
		}
		b.WriteString(callout.Render(callout.Callout{
			Kind:  callout.Info,
			Title: "No comparable exploration sample",
			BodyHTML: template.HTML(`<p class="max-w-prose">` + html.EscapeString(note) + `</p>` +
				`<p class="mt-3 [&>a]:min-h-[44px]">` + form.RenderButton(form.Button{Label: "Inspect session evidence", Href: "/sessions", Variant: form.ButtonGhost}) + `</p>`),
		}))
	}

	// Q2 — Prevented.
	b.WriteString(sectionHeading("Q2 · what did fathom prevent?"))
	b.WriteString(stat.RenderBand([]stat.Stat{
		impactStat("verify catches", imp.VerifyCatchesTotal, imp.VerifyCatchesNote),
		impactSourceCount("obligations served (calls)", imp.ObligationCalls, "rules injected before edits; conformity checking is IQ-13", m.CallLogNote),
		impactSourceCount("ledger sessions", imp.LedgerSessions, imp.DateRange, imp.LedgerNote),
	}, stat.BandOptions{AriaLabel: "prevented"}))

	// Q1 — Saved (law 12: arms or darkness).
	b.WriteString(sectionHeading("Q1 · what did the agent not have to do? (law 12: a savings figure exists only as an arm range)"))
	var card strings.Builder
	card.WriteString(kv("state", `<span class="font-semibold">`+html.EscapeString(imp.PilotState)+`</span>`))
	armTip := `<span title="Real model tokens the session spent: input + output. Cache reads are excluded — they are re-served context, not new spend, and once inflated this median 3,000x." class="cursor-help underline decoration-dotted decoration-line underline-offset-2">median real tokens</span>`
	if imp.LedgerNote != "" {
		card.WriteString(kv("ON arm", absentCell(imp.LedgerNote)))
		card.WriteString(kv("OFF arm", absentCell(imp.LedgerNote)))
	} else {
		card.WriteString(kv("ON arm", plural(imp.TokensOnSessions, "session")+` · `+armTip+` `+metricCell(imp.TokensOnMedian, "—", "")))
		card.WriteString(kv("OFF arm", plural(imp.TokensOffSessions, "session")+` · `+armTip+` `+metricCell(imp.TokensOffMedian, "—", "")))
	}
	if imp.TokensPreSplitRows != nil {
		card.WriteString(kv("pre-split rows", fmtThousands(*imp.TokensPreSplitRows)+` <span class="text-ink-muted">sessions recorded before the in/out split — their totals conflated cache reads, so they count as unknown, never as data</span>`))
	}
	if imp.CacheReadMedianCtx != nil {
		note := `avg per session — re-served context, shown for scale, never in the arm math`
		// IT-12: name the exclusion — never let a truncated row's frozen
		// partial total silently vanish from the count without a trace.
		if imp.CacheReadFrozenExcluded != nil {
			note += fmt.Sprintf("; %d session(s) excluded: totals frozen by the transcript read cap", *imp.CacheReadFrozenExcluded)
		}
		card.WriteString(kv("cache reads (context)", fmtThousands(*imp.CacheReadMedianCtx)+` <span class="text-ink-muted">`+html.EscapeString(note)+`</span>`))
	}
	card.WriteString(kv("discovery actions (with fathom)", metricCell(imp.DiscoveryTotal, imp.DiscoveryNote, imp.DiscoveryNote)))
	card.WriteString(`<div class="mt-2 text-ink-muted">` + html.EscapeString(imp.PilotDetail) + `</div>`)
	b.WriteString(dashboard.RenderWidget(dashboard.Widget{Body: template.HTML(card.String())}))

	// Q3 — Friction (the flywheel's fuel).
	b.WriteString(sectionHeading("Q3 · where does the agent still flounder? (misses are the roadmap)"))
	if m.CallLogNote != "" {
		b.WriteString(dashboard.RenderWidget(dashboard.Widget{Title: "per-tool outcomes", Body: template.HTML(`<p class="text-ink-muted">` + html.EscapeString(m.CallLogNote) + `</p>`)}))
	} else if len(m.Misses) > 0 {
		var tbl strings.Builder
		tbl.WriteString(`<div class="overflow-x-auto"><table class="w-full border-collapse text-sm"><tr class="border-b border-line text-left"><th scope="col" class="px-2 py-1 font-medium text-ink-muted">tool</th><th scope="col" class="px-2 py-1 font-medium text-ink-muted">calls</th><th scope="col" class="px-2 py-1 font-medium text-ink-muted">hits</th><th scope="col" class="px-2 py-1 font-medium text-ink-muted">misses</th><th scope="col" class="px-2 py-1 font-medium text-ink-muted">did-you-mean</th><th scope="col" class="px-2 py-1 font-medium text-ink-muted">outcome not recorded</th></tr>`)
		for i, r := range m.Misses {
			if i == 10 {
				break
			}
			tbl.WriteString(`<tr class="border-b border-line"><td class="px-2 py-1 font-mono text-xs">` + html.EscapeString(r.Tool) + `</td><td class="px-2 py-1 tabular-nums">` + fmtThousands(r.Calls) + `</td><td class="px-2 py-1 tabular-nums text-positive">` + fmtThousands(r.Hits) + `</td><td class="px-2 py-1 tabular-nums text-danger">` + fmtThousands(r.Misses) + `</td><td class="px-2 py-1 tabular-nums text-caution">` + fmtThousands(r.DidYouMean) + `</td><td class="px-2 py-1 tabular-nums text-ink-muted">` + fmtThousands(r.NotRecorded) + `</td></tr>`)
		}
		tbl.WriteString(`</table></div><div class="mt-2 text-xs text-ink-muted">rows logged before the outcome instrument (IQ-1) count as not-recorded, never as hits. Query text stays out of the log by design — it is user-global and metadata-only.</div>`)
		b.WriteString(dashboard.RenderWidget(dashboard.Widget{Title: "per-tool outcomes", Body: template.HTML(tbl.String())}))
	} else {
		b.WriteString(dashboard.RenderWidget(dashboard.Widget{Body: template.HTML(`<span class="italic text-ink-muted">no per-tool outcomes recorded in the current log</span>`)}))
	}
	observations := fleetOpenObservations(m)
	b.WriteString(dashboard.RenderWidget(dashboard.Widget{Title: "friction filed by hand", Body: template.HTML(kv("open observations across the fleet", `<span>`+html.EscapeString(observations.Value)+`</span> <span class="text-ink-muted">`+html.EscapeString(observations.Sub)+`</span>`))}))

	// Cost (BA-8): what fathom itself put into agent contexts — the
	// denominator half of the impact story, folded from the same call-log
	// entries the Q3 table above already reads.
	b.WriteString(renderCostPanel(m.Cost, m.CallLogNote))

	// Q4 — Compounding.
	b.WriteString(sectionHeading("Q4 · is the knowledge compounding?"))
	b.WriteString(stat.RenderBand([]stat.Stat{
		impactStat("median warm-up (turns to first edit)", imp.FirstEditMedian, imp.FirstEditNote),
		impactSourceCount("obligations served", imp.ObligationCalls, "growth curve arrives with weekly rollups", m.CallLogNote),
	}, stat.BandOptions{AriaLabel: "compounding"}))

	b.WriteString(sectionHeading("stories"))
	b.WriteString(`<div class="rounded-md border border-dashed border-caution px-3 py-2 text-ink-muted">narrative is a lens, never evidence (law 14). ` + html.EscapeString(imp.StoriesNote) + ` <a class="inline-flex min-h-11 items-center text-accent" href="/sessions">the week →</a></div>`)
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

// renderObsLandingBody is the front door's pure projection, also exercised
// by the fixed preview without invoking the cache, registry or local
// evidence readers.
func renderObsLandingBody(m observatoryModel) string {
	imp := m.Impact
	saved := "dark until OFF-arm sessions exist"
	if imp.TokensOffSessions > 0 {
		saved = plural(imp.TokensOffSessions, "OFF-arm session") + " recorded"
	}
	q := func(n, question, answer, sub, href string) string {
		return dashboard.RenderWidget(dashboard.Widget{
			Title: n + " · " + question,
			Body:  template.HTML(`<div class="text-xl font-semibold text-ink">` + html.EscapeString(answer) + `</div><div class="mt-1 text-ink-muted">` + html.EscapeString(sub) + `</div><div class="mt-2 [&>a]:min-h-[44px]">` + form.RenderButton(form.Button{Label: "see the evidence →", Href: href, Variant: form.ButtonGhost}) + `</div>`),
		})
	}
	observations := fleetOpenObservations(m)
	obsAnswer := observations.Value + " open observations"
	if observations.ValueRole == "danger" {
		obsAnswer = "open observations unreadable"
	}
	// OC-9: the call total is the shared projection — every row in the
	// resolved log, not the per-service sum that silently dropped rows
	// naming no service.
	calls := console.CallTotal(modelCallFacts(m))
	callContext := "plus per-tool miss rates from " + fmtThousands(calls.V) + " logged calls"
	if !calls.Known() {
		callContext = "per-tool outcomes unavailable: " + calls.Reason
	} else if m.DegradedNote != "" {
		callContext = "per-tool outcomes unavailable: " + m.DegradedNote
	}
	catches := "—"
	if imp.VerifyCatchesTotal != nil {
		catches = fmtThousands(*imp.VerifyCatchesTotal)
	}
	questions := []string{
		q("Q1", "what did the agent not have to do?", saved, "a savings figure exists only as a counterfactual arm range (law 12)", "/impact"),
		q("Q2", "what did fathom prevent?", catches+" verify catches", imp.VerifyCatchesNote, "/impact"),
		q("Q3", "where does the agent still flounder?", obsAnswer, observations.Sub+"; "+callContext, "/impact"),
		q("Q4", "is the knowledge compounding?", firstEditAnswer(imp), "warm-up turns before the first edit, per session", "/sessions"),
	}
	type entry struct{ href, name, desc string }
	var widgets []string
	for _, e := range []entry{
		{"/fleet", "fleet", "fleet tiles: wiring grades, freshness, traffic sparklines — every warning names its remedy"},
		{"/impact", "impact", "the four questions with their instruments; the counterfactual pilot state machine (law 12)"},
		{"/sessions", "the week", "the trailing 7 days composed from the instruments, with session replays"},
		{"/statusz", "statusz", "daemon health: uptime, per-service index freshness, channel liveness, last compile per layer"},
		{"/metrics", "metrics", "machine-readable counters"},
		{"/healthz", "healthz", "liveness probe (plain ok)"},
	} {
		widgets = append(widgets, dashboard.RenderWidget(dashboard.Widget{
			Title: e.name,
			Body:  template.HTML(`<div class="text-ink-muted">` + html.EscapeString(e.desc) + `</div><div class="mt-2"><a class="inline-flex min-h-11 items-center text-accent" href="` + e.href + `">open →</a></div>`),
		}))
	}
	var mach strings.Builder
	mach.WriteString(kv("MCP over HTTP", `<code class="font-mono text-xs">/rpc/agent</code> · <code class="font-mono text-xs">/rpc/companion</code> · <code class="font-mono text-xs">/rpc</code>`))
	mach.WriteString(kv("services indexed", valueText(console.ServiceCount(fleetFactsFromModel(m)))))
	// SurfaceTax (TD-1): the standing context cost every MCP-connected
	// session pays before any work — measured, or named unmeasured.
	mach.WriteString(kv("context tax", contextTaxValue(m.AgentSurfaceTools, m.AgentSchemaBytes)))
	mach.WriteString(kv("shareable brief", `<code class="font-mono text-xs">fathom brief</code> — writes a local file; sharing is a human verb (law 13)`))
	return sectionHeading("the four questions") +
		dashboard.RenderWidgetGrid(questions, dashboard.WidgetGridOptions{Cols: 4, AriaLabel: "the four questions"}) +
		sectionHeading("surfaces") +
		dashboard.RenderWidgetGrid(widgets, dashboard.WidgetGridOptions{Cols: 3, AriaLabel: "surfaces"}) +
		sectionHeading("machine surfaces") +
		dashboard.RenderWidget(dashboard.Widget{Body: template.HTML(mach.String())})
}

// renderObsLanding is the front door as a whole document (the fixed
// previews render it without the cache, registry or local evidence).
func renderObsLanding(m observatoryModel) string {
	return obsPage("fathom", "home", renderObsLandingBody(m), m)
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

// contextTaxValue renders TD-1's measurement: the agent surface's tool
// count and schema weight per connected session (tokens ≈ bytes/4). Zeros
// mean the surface did not answer — named, never rendered as a free
// surface (law 3).
func contextTaxValue(tools, schemaBytes int) string {
	if tools == 0 && schemaBytes == 0 {
		return `<span class="italic text-ink-muted">not measured (agent surface unreachable)</span>`
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
