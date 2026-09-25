package receipts

import (
	"fmt"
	"html"
	"html/template"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/ui/analytics"
	"github.com/geoffmilleraz/app-kit/appkit/ui/dashboard"
	"github.com/geoffmilleraz/app-kit/appkit/ui/vizframe"
)

func renderReport(p projection, theme string) (string, error) {
	if theme != "light" && theme != "dark" {
		return "", fmt.Errorf("receipts: unsupported theme")
	}
	if err := p.Current.Validate(p.Now); err != nil {
		return "", err
	}
	detailURL := func(id string) string {
		selected := p.Selection
		selected.DetailID = id
		return reportURL("/", selected, theme)
	}
	chart, err := renderSignedComparison(p, detailURL)
	if err != nil {
		return "", err
	}
	overview := p.Selection
	overview.DetailID = ""
	backURL := reportURL("/", overview, theme)
	var b strings.Builder
	fmt.Fprintf(&b, `<section class="receipts-report" data-selection-period="%s" data-selection-channel="%s" data-selection-compare="%s" data-selection-stale="%t" data-selection-product="%s">`, esc(p.Selection.Period), esc(p.Selection.Channel), esc(p.Selection.Compare), p.Selection.Stale, esc(p.Selection.DetailID))
	b.WriteString(`<div class="receipts-intro"><p class="receipts-eyebrow">App-kit analytical reference · synthetic data</p>`)
	if p.Detail != nil {
		fmt.Fprintf(&b, `<a class="receipts-back" href="%s">Back to the selected report</a><h2>%s</h2><p>Receipts, refunds and net cash flow for this product. The report's period, channel and comparison remain selected.</p>`, esc(backURL), esc(p.Detail.Product.Label))
	} else {
		b.WriteString(`<h2>Monthly receipts</h2><p>See what changed, inspect each product's contribution, and export the same evidence. This is cash-flow analysis, not accounting revenue or a forecast.</p>`)
	}
	b.WriteString(`</div>`)
	b.WriteString(renderFilters(p.Selection, theme))
	b.WriteString(`<div class="receipts-reading"><div class="receipts-chart-column">`)
	chartTitle := "Net receipts by product"
	if p.Detail != nil {
		chartTitle = "Product net receipts"
	}
	b.WriteString(vizframe.Render(chart, vizframe.Options{
		ID: "receipts-comparison", Title: chartTitle,
		Note:   "USD · receipts minus refunds · one shared scale for the selected periods",
		Source: p.Current.Contract.Source,
	}))
	b.WriteString(`</div><section class="receipts-decision" aria-labelledby="decision-heading">`)
	b.WriteString(renderDecision(p, detailURL))
	b.WriteString(`</section></div>`)
	b.WriteString(renderEvidenceTable(p, detailURL))
	b.WriteString(`<section class="receipts-context" aria-labelledby="context-heading"><h3 id="context-heading">Read the evidence in context</h3>`)
	b.WriteString(renderPeriodContext("Current period", p.Current.Contract, p.Now))
	if p.Previous != nil {
		b.WriteString(renderPeriodContext("Previous period", p.Previous.Contract, p.Now))
	}
	b.WriteString(`<details id="receipts-method"><summary>How this report works</summary><div class="receipts-method-body"><p>Net receipts equal receipts minus refunds, calculated in integer cents before USD presentation. A negative net means refunds exceed receipts. It is not automatically a loss or a cause of a business outcome.</p><p>Exact monthly totals require full calendar-month coverage and every selected net value. Comparisons and the largest-decline contribution require both periods to meet that condition. A known signed subtotal is not a lower bound: missing refunds can move the final amount either way.</p><p>Changing filters returns to the report overview. Chart links and product links inspect the same selected product; Back restores the overview selection. CSV includes raw receipts, refunds, net, missing reasons and period/source context, with measure-specific completeness. No values are extrapolated.</p>`)
	fmt.Fprintf(&b, `<p>All records are first-party historical fixtures. Evaluation is fixed at <time>%s</time>; freshness means age at that fixture clock, not freshness today. No live data, network provider, account or persistence service is connected.</p>`, esc(p.Now.Format(time.RFC3339)))
	b.WriteString(`</div></details></section></section>`)
	items := []dashboard.NavItem{{Href: backURL, Label: "Monthly report"}}
	for _, product := range p.Facts {
		items = append(items, dashboard.NavItem{Href: detailURL(product.Product.ID), Label: product.Product.Label})
	}
	currentPath := "/"
	if p.Detail != nil {
		currentPath = "/product/" + p.Detail.Product.ID
	}
	nextTheme, themeLabel := "dark", "Use dark theme"
	if theme == "dark" {
		nextTheme, themeLabel = "light", "Use light theme"
	}
	actions := `<a class="receipts-theme" href="` + esc(reportURL("/", p.Selection, nextTheme)) + `">` + themeLabel + `</a>`
	shell := dashboard.RenderShell(dashboard.Shell{
		Title: "Receipts", Brand: template.HTML(`<span class="text-base font-semibold text-ink">app-kit / receipts</span>`),
		Nav:     template.HTML(dashboard.RenderNav([]dashboard.NavGroup{{Heading: "Applied selection", Items: items}}, currentPath)),
		Actions: template.HTML(actions), Content: template.HTML(b.String()), ContentID: "receipts-main", SkipLink: true,
		Footer: template.HTML(`Experimental authored task. The original MatDash Dashboard2 and its 13-panel conversion remain separate; this reference does not certify that conversion.`),
	})
	return `<!doctype html><html lang="en" data-theme="` + theme + `"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Receipts | app-kit</title><link rel="icon" href="data:,"><link rel="stylesheet" href="/assets/app.css"></head><body data-app="receipts">` + shell + `</body></html>`, nil
}

func renderFilters(selected selection, theme string) string {
	var b strings.Builder
	b.WriteString(`<form class="receipts-filters" method="get" action="/" aria-label="Report selection">`)
	fmt.Fprintf(&b, `<input type="hidden" name="theme" value="%s">`, esc(theme))
	selectField := func(name, label, current string, options [][2]string) {
		fmt.Fprintf(&b, `<label for="filter-%s"><span>%s</span><select id="filter-%s" name="%s">`, name, label, name, name)
		for _, option := range options {
			chosen := ""
			if option[0] == current {
				chosen = " selected"
			}
			fmt.Fprintf(&b, `<option value="%s"%s>%s</option>`, option[0], chosen, option[1])
		}
		b.WriteString(`</select></label>`)
	}
	selectField("period", "Period", selected.Period, [][2]string{{"2024-09", "September 2024"}, {"2024-10", "October 2024"}, {"2024-11", "November 2024 (partial)"}})
	selectField("channel", "Channel", selected.Channel, [][2]string{{"all", "All channels"}, {"app", "App"}, {"mobile", "Mobile"}})
	selectField("compare", "Comparison", selected.Compare, [][2]string{{"previous", "Previous month"}, {"none", "No comparison"}})
	selectField("stale", "Fixture source age", strconv.FormatBool(selected.Stale), [][2]string{{"false", "1 hour old"}, {"true", "48 hours old (stale)"}})
	b.WriteString(`<button class="receipts-primary" type="submit">Apply selection</button></form>`)
	sourceAge := "fixture source fresh (1 hour old)"
	if selected.Stale {
		sourceAge = "fixture source stale (48 hours old)"
	}
	fmt.Fprintf(&b, `<div class="receipts-applied"><p>Applied: <strong>%s</strong> · %s · %s · %s</p><a class="receipts-export" href="%s">Export applied selection (CSV)</a></div>`, esc(periodName(selected.Period)), esc(selected.Channel), esc(map[string]string{"previous": "previous month included", "none": "no comparison"}[selected.Compare]), sourceAge, esc(reportURL("/export.csv", selected, theme)))
	return b.String()
}

func renderDecision(p projection, detailURL func(string) string) string {
	var b strings.Builder
	b.WriteString(`<p class="receipts-eyebrow">What changed</p><h3 id="decision-heading">`)
	s := p.Summary
	subject, currentLabel, previousLabel := "Net receipts", "Current monthly total (USD)", "Previous monthly total (USD)"
	if p.Detail != nil {
		subject, currentLabel, previousLabel = "Product net receipts", "Current product net receipts (USD)", "Previous product net receipts (USD)"
	}
	switch {
	case s.ExactTotalCents == nil:
		if p.Detail != nil {
			b.WriteString(`This product's full-month net receipts are not known`)
		} else {
			b.WriteString(`The monthly total is not yet known`)
		}
	case p.Previous == nil:
		b.WriteString(`A single-period view`)
	case s.DeltaCents == nil:
		b.WriteString(`These periods cannot be compared yet`)
	case *s.DeltaCents < 0:
		fmt.Fprintf(&b, `%s fell %s`, subject, moneyCents(-*s.DeltaCents))
	case *s.DeltaCents > 0:
		fmt.Fprintf(&b, `%s rose %s`, subject, moneyCents(*s.DeltaCents))
	default:
		fmt.Fprintf(&b, `%s were unchanged`, subject)
	}
	b.WriteString(`</h3><dl class="receipts-totals">`)
	amount := s.ExactTotalCents
	label := currentLabel
	if amount == nil {
		label, amount = "Known subtotal only (USD)", &s.KnownSubtotalCents
	}
	currentValue := moneyCents(*amount)
	if s.KnownCount == 0 {
		label, currentValue = "Selected net receipts (USD)", "Not known"
	}
	fmt.Fprintf(&b, `<div><dt>%s</dt><dd data-total="current">%s</dd></div>`, label, currentValue)
	if p.Previous != nil {
		value := "Not known"
		if s.PreviousExactTotalCents != nil {
			value = moneyCents(*s.PreviousExactTotalCents)
		}
		fmt.Fprintf(&b, `<div><dt>%s</dt><dd data-total="previous">%s</dd></div>`, previousLabel, value)
	}
	b.WriteString(`</dl>`)
	fmt.Fprintf(&b, `<p>%d of %d selected product net values are known.</p>`, s.KnownCount, s.SelectedCount)
	if s.ExactTotalCents == nil {
		b.WriteString(`<p>Missing inputs or incomplete calendar coverage prevent a full-month amount. Known values are not extrapolated, and a signed subtotal is not a lower bound.</p>`)
	}
	if len(s.LargestDeclineIDs) > 0 {
		b.WriteString(`<p>Largest declining contribution:</p><ul class="receipts-contributions">`)
		for _, id := range s.LargestDeclineIDs {
			current, previous := factByID(p.Facts, id), factByID(p.PreviousFacts, id)
			fmt.Fprintf(&b, `<li><a href="%s">Inspect %s</a><span>%s USD</span></li>`, esc(detailURL(id)), esc(current.Product.Label), moneyCents(*current.NetCents-*previous.NetCents))
		}
		b.WriteString(`</ul><p>Contribution describes the observed change, not its cause.</p>`)
	} else if s.DeltaCents != nil {
		b.WriteString(`<p>No selected product has a declining contribution.</p>`)
	} else if p.Previous != nil {
		b.WriteString(`<p>No product-decline ranking is asserted until both selected periods are complete.</p>`)
	}
	b.WriteString(`<a class="receipts-method-link" href="#receipts-method">How to interpret these values</a>`)
	return b.String()
}

func renderEvidenceTable(p projection, detailURL func(string) string) string {
	var b strings.Builder
	b.WriteString(`<section class="receipts-evidence" aria-labelledby="evidence-heading"><h3 id="evidence-heading">Receipts and refunds behind every value</h3><p>All amounts in USD. Missing is not zero. Scroll the table horizontally on a narrow screen.</p><div class="receipts-table-scroll" role="region" aria-labelledby="evidence-heading" tabindex="0"><table id="receipts-values"><caption>Selected product facts and net receipts</caption><thead><tr><th scope="col">Product</th><th scope="col">Period</th><th scope="col">Receipts</th><th scope="col">Refunds</th><th scope="col">Net</th></tr></thead><tbody>`)
	writeRows := func(series string, contract analytics.Contract, facts []projectedFact) {
		for _, fact := range facts {
			fmt.Fprintf(&b, `<tr data-product="%s" data-series="%s"><th scope="row"><a href="%s">%s</a></th><td>%s<br><span>%s</span></td>`, esc(fact.Product.ID), series, esc(detailURL(fact.Product.ID)), esc(fact.Product.Label), esc(periodLabel(contract)), series)
			for _, measure := range []struct {
				name  string
				cents *int64
			}{{"receipts", fact.ReceiptsCents}, {"refunds", fact.RefundsCents}, {"net", fact.NetCents}} {
				if measure.cents == nil {
					fmt.Fprintf(&b, `<td data-measure="%s" data-state="missing">Missing: <span class="receipts-missing-reason">%s</span></td>`, measure.name, esc(fact.MissingReason))
				} else {
					fmt.Fprintf(&b, `<td data-measure="%s" data-state="present" data-cents="%d">%s</td>`, measure.name, *measure.cents, moneyCents(*measure.cents))
				}
			}
			b.WriteString(`</tr>`)
		}
	}
	writeRows("current", p.Current.Contract, p.Facts)
	if p.Previous != nil {
		writeRows("previous", p.Previous.Contract, p.PreviousFacts)
	}
	b.WriteString(`</tbody></table></div></section>`)
	return b.String()
}

func renderPeriodContext(label string, c analytics.Contract, now time.Time) string {
	age := now.Sub(c.Freshness.ObservedAt)
	state := "fresh"
	if age > c.Freshness.MaxAge {
		state = "stale"
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<section class="receipts-period-context" data-coverage="%s" data-freshness="%s"><h4>%s</h4><dl><div><dt>Observed interval (UTC, end exclusive)</dt><dd><time>%s</time> to <time>%s</time></dd></div><div><dt>Coverage</dt><dd>%s</dd></div><div><dt>Source observed at</dt><dd><time>%s</time></dd></div><div><dt>Fixture freshness</dt><dd>%s, %s old at the fixed evaluation time</dd></div></dl>`, c.Completeness, state, label, c.Window.Start.Format(time.RFC3339), c.Window.End.Format(time.RFC3339), c.Completeness, c.Freshness.ObservedAt.Format(time.RFC3339), state, age.String())
	if c.CoverageNote != "" {
		fmt.Fprintf(&b, `<p>%s</p>`, esc(c.CoverageNote))
	}
	fmt.Fprintf(&b, `<p>Uncertainty: %s</p></section>`, esc(c.Uncertainty))
	return b.String()
}

func periodName(period string) string {
	t, err := time.Parse("2006-01", period)
	if err != nil {
		return period
	}
	return t.Format("January 2006")
}

func periodLabel(c analytics.Contract) string {
	label := periodName(c.Window.Start.Format("2006-01"))
	if c.Window.End.Before(c.Window.Start.AddDate(0, 1, 0)) {
		label += " (partial, before " + c.Window.End.UTC().Format("2 Jan 15:04 UTC") + ")"
	}
	return label
}

func moneyCents(value int64) string {
	sign := ""
	if value < 0 {
		sign, value = "-", -value
	}
	whole := strconv.FormatInt(value/100, 10)
	for i := len(whole) - 3; i > 0; i -= 3 {
		whole = whole[:i] + "," + whole[i:]
	}
	return fmt.Sprintf("%s$%s.%02d", sign, whole, value%100)
}

func esc(value string) string { return html.EscapeString(value) }
