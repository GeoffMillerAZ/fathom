package main

// console_decide.go — the Decision port page at /decide (vision 35 §8,
// design §3 "Decision port"; OC-45). The provider and its sites (on or off
// under [decide] sites), the ruling log — site, kind, choice, confidence,
// latency, applied, error — newest first and paged, and the calibration
// table from the latest reading, with its path, so the reader can open the
// evidence behind "route suggestions stay hedged". It renders `fathom decide
// log --json` (decidelog.go) from the same sources.

import (
	"fmt"
	"html"
	"html/template"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
	"github.com/geoffmilleraz/fathom/internal/console"
)

const decideVerb = "fathom decide log --json"

func init() {
	declarePage(consolePage{
		ID: "decide", Route: "/decide", Href: "/decide", Group: "System", Order: 20, Nav: true,
		Title:    "Decision port",
		Question: "What did the decision port rule, how often was it applied, and how well calibrated is it?",
		Verbs:    []string{decideVerb},
		Build:    buildDecidePage,
	})
}

func buildDecidePage(cr *consoleRequest) (consoleOut, error) {
	m := cr.model()
	q := cr.r.URL.Query()
	var roots []string
	for _, s := range m.Services {
		roots = append(roots, s.Path)
	}
	site := strings.TrimSpace(q.Get("site"))
	resolved := cr.deps.sources().CallLog
	dm := console.DecideView(decideInputsFrom(readDecideSources(resolved, "", roots)), console.DecideFilter{Site: site}, evPage(q), 50)
	verb := decideVerb
	if site != "" || dm.Page > 1 {
		verb = evCommand("fathom decide log", "--site", site, "--page", pageFlag(dm.Page))
	}
	return consoleOut{
		Meta:     []string{"provider " + dm.Config.Provider, "log " + dm.LogPath},
		Model:    dm,
		Sections: map[string]any{verb: withCallLog(dm, resolved)},
		Body:     func() string { return renderDecide(cr, dm, verb) },
	}, nil
}

// pageFlag is a page number as a flag value ("" for the first page).
func pageFlag(page int) string {
	if page <= 1 {
		return ""
	}
	return strconv.Itoa(page)
}

func renderDecide(cr *consoleRequest, dm console.DecideModel, verb string) string {
	var b strings.Builder
	provider := dm.Config.Provider
	provSub := "no decision is ever asked (the default)"
	if dm.ProviderOn {
		provSub = "model " + nonEmptyStr(dm.Config.Model, "the provider's default")
	}
	b.WriteString(stat.RenderBand([]stat.Stat{
		{Label: "provider", Value: provider, Sub: provSub},
		valueStat("rulings logged", dm.Rulings),
		valueStat("applied", dm.Applied),
		valueStat("failed calls", dm.Errors),
		valueStat("median latency (ms)", dm.MedianLatencyMS),
	}, stat.BandOptions{AriaLabel: "the decision port"}))

	b.WriteString(`<div class="mt-4 grid gap-4 xl:grid-cols-2">`)
	// Provider and sites.
	var sites strings.Builder
	sites.WriteString(chrome.RenderDescriptionList([]chrome.DescItem{
		{Term: "provider", Detail: provider},
		{Term: "configured by", Detail: dm.Config.Source},
		{Term: "route confidence", Detail: strconv.FormatFloat(dm.Config.RouteConfidence, 'f', 2, 64) + " — the proceed floor; route suggestions stay hedged whatever the band (CV-M11)"},
	}))
	var rows []table.Row
	for _, s := range dm.Sites {
		state := `<span class="font-medium text-positive">✓ allowed</span>`
		if !dm.ProviderOn {
			state = `<span class="font-medium text-ink">allowed</span> <span class="text-xs text-ink-muted">(provider none: no call)</span>`
		}
		if !s.On {
			state = `<span class="font-medium text-ink-muted">✗ not in [decide] sites</span>`
		}
		// Every count is the log's: with no log each is "not measured", with
		// an unreadable one each names the failure — never a 0 beside it.
		rows = append(rows, table.Row{ID: "site-" + s.Site, HTMLCells: []templComponent{
			evRaw(`<a href="` + html.EscapeString(evHref("/decide", cr.r.URL.Query(), "site", s.Site)) + `#rulings" class="inline-flex min-h-11 items-center font-mono text-xs text-accent underline-offset-4 hover:underline">` + html.EscapeString(s.Site) + `</a>`),
			evRaw(state), evRaw(evCell(s.Rulings, "not measured")), evRaw(evCell(s.Applied, "not measured")), evRaw(evCell(s.Errors, "not measured"))}})
	}
	sites.WriteString(`<div class="mt-3">` + renderKitTable(table.TableParams{TableID: "decide-sites", Caption: "sites: where a decision may be asked",
		Columns: []table.Column{{Key: "site", Label: "Site"}, {Key: "on", Label: "Egress"}, {Key: "rulings", Label: "Rulings", Align: "right"}, {Key: "applied", Label: "Applied", Align: "right"}, {Key: "errors", Label: "Failed", Align: "right"}},
		Rows:    rows}) + `</div>`)
	b.WriteString(renderPanel(consolePanel{
		ID: "provider", Title: "Provider and sites", SourceKey: "daemon",
		Question: "Which provider answers, and which sites may ask it?",
		Verb:     verb, Source: "[decide] in config.toml, as this daemon resolves it; rulings from the log",
		Denominator: "the four sites the port knows, plus any logged site outside them", Freshness: "live — read for this request",
		Body: sites.String(),
	}))

	// Calibration.
	b.WriteString(renderPanel(consolePanel{
		ID: "calibration", Title: "Calibration", SourceKey: "daemon",
		Question: "When the port says it is sure, how often is it right?",
		Verb:     verb, Source: calibrationSource(dm), Denominator: calibrationDenominator(dm),
		Freshness: calibrationFreshness(dm),
		State:     calibrationState(dm), Reason: dm.CalibrationNote,
		Body: renderCalibration(dm),
	}))

	// The ruling log.
	b.WriteString(`<div class="min-w-0 xl:col-span-2">` + renderPanel(consolePanel{
		ID: "rulings", Title: "Ruling log", SourceKey: "daemon",
		Question: "What did each decision rule, how sure was it, how long did it take, and was it applied?",
		Verb:     verb, Source: "decisions log " + dm.LogPath, Denominator: fmt.Sprintf("%s, newest first", plural(dm.Matching, "ruling")),
		Freshness: "live — read for this request",
		State:     dm.Rulings.State, Reason: dm.Rulings.Reason,
		Body: renderRulings(cr, dm),
	}) + `</div>`)
	b.WriteString(`</div>`)
	return b.String()
}

func nonEmptyStr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func calibrationSource(dm console.DecideModel) string {
	if dm.Calibration == nil {
		return "the newest docs/bench/readings/decision-port-calibration-*.md"
	}
	return dm.Calibration.Reading
}

func calibrationDenominator(dm console.DecideModel) string {
	if dm.Calibration == nil {
		return ""
	}
	n := 0
	for _, r := range dm.Calibration.Buckets {
		n += r.N
	}
	return fmt.Sprintf("%d graded route decisions against their sessions' gold tool", n)
}

func calibrationFreshness(dm console.DecideModel) string {
	if dm.Calibration == nil || dm.Calibration.Date == "" {
		return "the latest reading"
	}
	return "measured " + dm.Calibration.Date + " (a reading; not recomputed — the log carries no gold labels)"
}

func calibrationState(dm console.DecideModel) console.State {
	if dm.Calibration == nil {
		return console.StateAbsent
	}
	return console.StatePresent
}

// renderCalibration is the reading's bucket table as range-free bars with
// the 0.9 bar marked in words, its sweep behind a disclosure, and the
// reading's own verdict.
func renderCalibration(dm console.DecideModel) string {
	c := dm.Calibration
	if c == nil {
		return ""
	}
	var b strings.Builder
	if c.Verdict != "" {
		b.WriteString(`<p class="mb-3 max-w-prose text-ink" data-calibration-verdict>` + html.EscapeString(c.Verdict) + `</p>`)
	}
	var bars []viz.Bar
	var rows []table.Row
	for _, r := range c.Buckets {
		role := "positive"
		if r.Fraction < 0.9 {
			role = "caution"
		}
		bars = append(bars, viz.Bar{Label: r.Bucket, Value: r.Fraction * 100, Role: role, Suffix: "%"})
		rows = append(rows, table.Row{ID: "bucket-" + strconv.Itoa(len(rows)), Cells: []string{r.Bucket, fmt.Sprintf("%d of %d", r.Hits, r.N), evPercent(r.Fraction * 100)}})
	}
	chart := viz.Bars(bars, viz.BarsOptions{W: 420, LabelW: 120, Max: 100, ShowValue: true, Fluid: true, AriaLabel: "share of route decisions that were right, by confidence bucket"})
	b.WriteString(`<div class="max-w-[26rem]">` + chart + `</div>`)
	b.WriteString(`<p class="mt-1 text-xs text-ink-muted">the bar the check set: 90% right at confidence ≥ 0.9</p>`)
	tbl := renderKitTable(table.TableParams{TableID: "decide-buckets", Caption: "route decisions right, by confidence bucket",
		Columns: []table.Column{{Key: "bucket", Label: "Confidence"}, {Key: "right", Label: "Right", Align: "right"}, {Key: "rate", Label: "Rate", Align: "right"}},
		Rows:    rows})
	b.WriteString(`<div class="mt-3">` + tbl + `</div>`)
	if len(c.Sweep) > 0 {
		var srows []table.Row
		for i, r := range c.Sweep {
			srows = append(srows, table.Row{ID: "sweep-" + strconv.Itoa(i), Cells: []string{"≥ " + r.Bucket, fmt.Sprintf("%d of %d", r.Hits, r.N), evPercent(r.Fraction * 100)}})
		}
		sweep := renderKitTable(table.TableParams{TableID: "decide-sweep", Caption: "every cutoff: right at or above it",
			Columns: []table.Column{{Key: "cutoff", Label: "Cutoff"}, {Key: "right", Label: "Right", Align: "right"}, {Key: "rate", Label: "Rate", Align: "right"}},
			Rows:    srows})
		b.WriteString(`<div class="mt-3">` + chrome.RenderDisclosure(chrome.Disclosure{Summary: "the cutoff sweep (confidence is not monotonic with correctness)", Body: template.HTML(sweep)}) + `</div>`)
	}
	return b.String()
}

// renderRulings is the ruling log: site chips, the table, the pages.
func renderRulings(cr *consoleRequest, dm console.DecideModel) string {
	q := cr.r.URL.Query()
	var b strings.Builder
	chips := []evChip{{Label: "every site", Href: evHref("/decide", q, "site", "") + "#rulings", Selected: dm.Filter.Site == ""}}
	for _, s := range dm.Sites {
		if !s.Rulings.Known() || s.Rulings.V == 0 {
			continue // no chip for a site with nothing to filter to
		}
		chips = append(chips, evChip{Label: s.Site, Href: evHref("/decide", q, "site", s.Site) + "#rulings", Selected: dm.Filter.Site == s.Site, Count: fmtThousands(s.Rulings.V)})
	}
	b.WriteString(evChips("Site", chips))
	if !dm.Rulings.Known() {
		return b.String()
	}
	if dm.Matching == 0 {
		b.WriteString(chrome.RenderEmptyState(chrome.EmptyState{Heading: "No ruling", Body: "The log holds no ruling for this site — a provider records a row only when it answers or fails."}))
		return b.String()
	}
	var rows []table.Row
	var stack []evStackItem
	for i, r := range dm.Rows {
		applied := "no"
		if r.Applied {
			applied = "yes"
		}
		outcome := r.Choice
		if r.Error != "" {
			outcome = "failed: " + r.Error
			applied = "—"
		}
		conf := "—"
		if r.Error == "" {
			conf = strconv.FormatFloat(r.Confidence, 'f', 2, 64)
		}
		when := evShortTime(r.TS)
		rows = append(rows, table.Row{ID: "ruling-" + strconv.Itoa(i), Cells: []string{"", r.Site, r.Kind, "", conf, fmtThousands(int(r.LatencyMS)), applied},
			HTMLCells: []templComponent{evRaw(`<span class="whitespace-nowrap tabular-nums">` + html.EscapeString(when) + `</span>`), nil, nil, evRaw(`<span class="[overflow-wrap:anywhere]">` + html.EscapeString(outcome) + `</span>`)}})
		// Below md the choice, confidence, latency and applied columns would
		// scroll off; each ruling reads whole instead.
		stack = append(stack, evStackItem{
			Heading: `<span class="font-mono text-xs">` + html.EscapeString(r.Site) + `</span> <span class="text-xs font-normal text-ink-muted">· <span class="tabular-nums">` + html.EscapeString(when) + `</span></span>`,
			Items: []chrome.DescItem{
				{Term: "kind · choice", Detail: strings.TrimPrefix(r.Kind+" · "+outcome, " · ")},
				{Term: "confidence · latency", Detail: conf + " · " + fmtThousands(int(r.LatencyMS)) + " ms"},
				{Term: "applied", Detail: applied},
			},
		})
	}
	caption := fmt.Sprintf("rulings %s–%s of %s, newest first", fmtThousands((dm.Page-1)*dm.PerPage+1), fmtThousands((dm.Page-1)*dm.PerPage+len(dm.Rows)), fmtThousands(dm.Matching))
	tbl := renderKitTable(table.TableParams{TableID: "decide-rulings", Caption: caption,
		Columns: []table.Column{{Key: "ts", Label: "When (UTC)"}, {Key: "site", Label: "Site"}, {Key: "kind", Label: "Kind"}, {Key: "choice", Label: "Choice"}, {Key: "conf", Label: "Confidence", Align: "right"}, {Key: "latency", Label: "Latency (ms)", Align: "right"}, {Key: "applied", Label: "Applied"}},
		Rows:    rows})
	b.WriteString(`<p class="mb-2 text-xs text-ink-muted md:hidden">` + html.EscapeString(caption) + `</p>` + evTableOrStack("decide-rulings", "rulings", tbl, stack))
	b.WriteString(`<p class="mt-2 text-xs text-ink-muted">applied: the ruling changed what was served; an unsure-band answer is never counted as applied (I42). A failed call answered nothing and applied nothing (DP-22).</p>`)
	b.WriteString(evPagination("/decide", q, dm.Page, dm.Pages, "ruling pages"))
	return b.String()
}
