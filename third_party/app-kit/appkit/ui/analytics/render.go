package analytics

import (
	"fmt"
	"html"
	"math"
	"strings"
	"time"

	"github.com/rivo/uniseg"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/viz"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/vizframe"
)

// The recipes' SVG labels use 11px text. Reserve a conservative 16px row so
// inter-row gaps cannot disguise clipping at the first or last row. This is
// deterministic admission, not proof of an arbitrary consumer font's geometry.
const minReadableBarRowHeight = 16

const barRowHeightFallback = `<p class="mb-3 text-sm text-ink">Bar chart omitted: row height is below the 16 logical pixels reserved for readable labels. All selected values remain in the data table. Set RowH to at least 16 or use an app-owned layout with verified text geometry.</p>`

// RenderBars validates and renders the selected rows with their meaning, direct
// numeric labels, source, truth states and a visible captioned data table.
// Negative values are refused: the underlying primitive clamps them to zero.
// ShowValue is always true. Max must cover all rounded values when supplied.
// Rows remain in app-selected order, including zeroes and missing observations.
// App-authored role meanings are repeated per row in the visible data table.
// Labels/values that exceed the conservative plot-space budget produce an
// explicit table-only fallback, never zero-width marks or truncated labels.
// Positive rounded values below one logical plot pixel also use that fallback;
// they are never inflated, omitted, or presented graphically as measured zero.
// Positive RowH below 16 uses the same non-error table-only fallback to preserve
// vertical label space; nonpositive RowH retains the 28px default.
func RenderBars(d Dataset, o viz.BarsOptions, now time.Time) (string, error) {
	if err := d.Validate(now); err != nil {
		return "", err
	}
	c := d.Contract
	if math.IsNaN(o.Max) || math.IsInf(o.Max, 0) || o.Max < 0 || o.Max > MaxMagnitude {
		return "", fmt.Errorf("analytics: bar maximum must be finite and between 0 and %.0f", MaxMagnitude)
	}
	var bars []viz.Bar
	plottedRoles := make(map[string]bool, len(d.Rows))
	for _, row := range d.Rows {
		if row.Value == nil {
			continue
		}
		if *row.Value < 0 {
			return "", fmt.Errorf("analytics: bar row %q is negative; use analytics.RenderDivergingBars or an app-owned signed representation", row.ID)
		}
		value := rounded(*row.Value, c.Decimals)
		if o.Max > 0 && value > o.Max {
			return "", fmt.Errorf("analytics: bar maximum clips row %q", row.ID)
		}
		bars = append(bars, viz.Bar{Label: row.Label, Value: value, Role: rowRole(row), Suffix: " " + c.Unit})
		plottedRoles[rowRole(row)] = true
	}
	if o.W <= 0 {
		o.W = 560
	}
	if o.LabelW <= 0 {
		o.LabelW = 180
	}
	if o.RowH <= 0 {
		o.RowH = 28
	}
	if o.W > 4096 || o.RowH > 256 || o.Gap > 256 || o.LabelW > o.W-120 {
		return "", fmt.Errorf("analytics: bar dimensions must leave space for labels and values and stay within the recipe budget")
	}
	rowHeightFits := o.RowH >= minReadableBarRowHeight
	plotFits := true
	valueWidth := float64(56)
	for _, row := range d.Rows {
		if row.Value == nil {
			continue
		}
		// Match viz.Bars' conservative 11px display-cell reservation. This is
		// deterministic admission, not a measurement of a consumer's font.
		if uniseg.StringWidth(row.Label)*11 > o.LabelW {
			plotFits = false
		}
		printed := number(*row.Value, c.Decimals, true) + " " + c.Unit
		valueWidth = math.Max(valueWidth, float64(uniseg.StringWidth(printed))*11+8)
	}
	plotWidth := float64(o.W-o.LabelW-6) - valueWidth
	if plotWidth < 48 {
		plotFits = false
	}
	maximum := o.Max
	if maximum == 0 {
		for _, bar := range bars {
			maximum = math.Max(maximum, bar.Value)
		}
	}
	subpixel := false
	if plotFits && maximum > 0 {
		for _, bar := range bars {
			if bar.Value > 0 && bar.Value/maximum*plotWidth < 1 {
				subpixel = true
				break
			}
		}
	}
	o.ShowValue = true
	o.AriaLabel = c.Title + " (" + c.Unit + ")"

	var b strings.Builder
	fmt.Fprintf(&b, `<p class="mb-2 text-sm text-ink-muted">Unit: %s. Grain: %s. Aggregation: %s. Values rounded to %d decimal places.</p>`, html.EscapeString(c.Unit), html.EscapeString(c.Grain), html.EscapeString(c.Aggregation), c.Decimals)
	fmt.Fprintf(&b, `<p class="mb-2 text-sm text-ink-muted">%s</p>`, html.EscapeString(c.Window.label()))
	fmt.Fprintf(&b, `<p class="mb-2 text-sm text-ink-muted">%s. Coverage: %s`, html.EscapeString(c.Freshness.label(now)), html.EscapeString(string(c.Completeness)))
	if c.CoverageNote != "" {
		fmt.Fprintf(&b, `; %s`, html.EscapeString(c.CoverageNote))
	}
	fmt.Fprintf(&b, `.</p><p class="mb-3 text-sm text-ink-muted">Uncertainty: %s</p>`, html.EscapeString(c.Uncertainty))
	if len(bars) == 0 {
		b.WriteString(`<p class="mb-3 text-sm text-ink">No measured values in this selection.</p>`)
	} else if !rowHeightFits {
		b.WriteString(barRowHeightFallback)
	} else if !plotFits {
		b.WriteString(`<p class="mb-3 text-sm text-ink">Bar chart omitted: labels or values exceed this layout's readable space. All selected values remain in the data table. Choose wider chart and label columns or an app-owned wrapping layout to show the bars.</p>`)
	} else if subpixel {
		b.WriteString(`<p class="mb-3 text-sm text-ink">Bar chart omitted: one or more positive values would render smaller than one logical pixel at this scale. All selected values remain in the data table. Choose a narrower selection or an app-owned alternative representation to compare these values graphically.</p>`)
	} else {
		fmt.Fprintf(&b, `<div class="overflow-x-auto focus-visible:outline-2 focus-visible:outline-focus" tabindex="0" role="region" aria-label="%s bar chart, horizontally scrollable if needed">`, html.EscapeString(c.Title))
		b.WriteString(viz.Bars(bars, o))
		b.WriteString(`</div>`)
	}
	// A missing row can retain its app-declared meaning in the table and CSV,
	// but must not claim a color encoding for a mark that was never plotted.
	var plottedLegend []LegendItem
	for _, item := range d.Legend {
		if plottedRoles[item.Role] {
			plottedLegend = append(plottedLegend, item)
		}
	}
	if len(plottedLegend) > 0 && rowHeightFits && plotFits && !subpixel {
		b.WriteString(`<ul aria-label="Chart legend" class="mt-2 flex flex-wrap gap-3 text-sm text-ink-muted">`)
		for _, item := range plottedLegend {
			fmt.Fprintf(&b, `<li class="inline-flex items-center gap-1.5"><svg id="%s" data-analytics-legend-swatch aria-hidden="true" focusable="false" width="12" height="12" viewBox="0 0 12 12" class="inline-block size-3 shrink-0"><rect width="12" height="12" rx="2" fill="var(--ak-color-%s)"></rect></svg>%s</li>`, html.EscapeString(c.ID+"-legend-"+item.Role), item.Role, html.EscapeString(item.Label))
		}
		b.WriteString(`</ul>`)
	}
	b.WriteString(renderTable(d))
	return vizframe.Render(b.String(), vizframe.Options{ID: c.ID, Title: c.Title, Source: c.Source}), nil
}

func renderTable(d Dataset) string {
	c := d.Contract
	labels := d.roleLabels()
	showMeaning := false
	for _, row := range d.Rows {
		if _, ok := labels[rowRole(row)]; ok {
			showMeaning = true
			break
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<div class="mt-4 overflow-x-auto focus-visible:outline-2 focus-visible:outline-focus" tabindex="0" role="region" aria-label="%s data table, horizontally scrollable if needed"><table class="w-full min-w-[32rem] text-left text-sm text-ink [&_th]:align-top [&_td]:align-top">`, html.EscapeString(c.Title))
	fmt.Fprintf(&b, `<caption class="mb-2 text-left font-medium">Values for %s</caption>`, html.EscapeString(c.Title))
	fmt.Fprintf(&b, `<thead><tr class="border-b border-line"><th scope="col" class="px-2 py-2">Label</th><th scope="col" class="px-2 py-2 text-right">Value (%s)</th><th scope="col" class="px-2 py-2">Bounds (%s)</th><th scope="col" class="px-2 py-2">State</th>`, html.EscapeString(c.Unit), html.EscapeString(c.Unit))
	if showMeaning {
		b.WriteString(`<th scope="col" class="px-2 py-2">Meaning</th>`)
	}
	b.WriteString(`</tr></thead><tbody>`)
	for _, row := range d.Rows {
		label := html.EscapeString(row.Label)
		if row.DetailHref != "" {
			label = fmt.Sprintf(`<a href="%s" class="flex min-h-11 items-center text-ink underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"><span class="min-w-0 whitespace-normal [overflow-wrap:anywhere]">%s</span></a>`, html.EscapeString(row.DetailHref), label)
		}
		fmt.Fprintf(&b, `<tr class="border-b border-line"><th scope="row" class="min-w-32 max-w-xs px-2 py-2 font-medium [overflow-wrap:anywhere]">%s</th>`, label)
		if row.Value == nil {
			fmt.Fprintf(&b, `<td class="px-2 py-2 text-right">Missing</td><td class="px-2 py-2">Not provided</td><td class="min-w-48 max-w-xs px-2 py-2 [overflow-wrap:anywhere]">%s</td>`, html.EscapeString(row.MissingReason))
		} else {
			fmt.Fprintf(&b, `<td class="px-2 py-2 text-right tabular-nums">%s</td><td class="px-2 py-2">`, number(*row.Value, c.Decimals, true))
			if row.Bounds == nil {
				b.WriteString(`Not provided`)
			} else {
				fmt.Fprintf(&b, `%s to %s`, number(row.Bounds.Low, c.Decimals, true), number(row.Bounds.High, c.Decimals, true))
			}
			b.WriteString(`</td><td class="px-2 py-2">Present</td>`)
		}
		if showMeaning {
			label, ok := labels[rowRole(row)]
			if !ok {
				label = "Not specified"
			}
			fmt.Fprintf(&b, `<td class="min-w-32 max-w-xs px-2 py-2 [overflow-wrap:anywhere]">%s</td>`, html.EscapeString(label))
		}
		b.WriteString(`</tr>`)
	}
	if len(d.Rows) == 0 {
		b.WriteString(`<tr><td colspan="4" class="px-2 py-2">No rows in this selection.</td></tr>`)
	}
	b.WriteString(`</tbody></table></div>`)
	return b.String()
}
