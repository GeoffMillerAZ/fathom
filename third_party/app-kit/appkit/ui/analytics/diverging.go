package analytics

import (
	"fmt"
	"html"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/rivo/uniseg"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/viz"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/vizframe"
)

type divergingRow struct {
	row   Row
	value float64
}

// RenderDivergingBars validates and renders a selected signed dataset around a
// centered zero baseline with a symmetric domain. A positive BarsOptions.Max is
// the absolute extent and must contain every rounded value; zero derives the
// extent from the data. Flat all-zero selections retain an explicit zero marker
// on a deterministic unit extent. Missing rows remain in the table and CSV but
// do not produce chart marks. RenderBars and viz.Bars retain their existing
// non-negative contracts.
// Positive RowH below 16 yields an explained table-only fallback, without
// changing the data or returning an error; nonpositive RowH defaults to 28.
func RenderDivergingBars(d Dataset, o viz.BarsOptions, now time.Time) (string, error) {
	if err := d.Validate(now); err != nil {
		return "", err
	}
	c := d.Contract
	if math.IsNaN(o.Max) || math.IsInf(o.Max, 0) || o.Max < 0 || o.Max > MaxMagnitude {
		return "", fmt.Errorf("analytics: diverging bar extent must be finite and between 0 and %.0f", MaxMagnitude)
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
	if o.Gap <= 0 {
		o.Gap = 6
	}
	if o.W > 4096 || o.RowH > 256 || o.Gap > 256 || o.LabelW > o.W-120 {
		return "", fmt.Errorf("analytics: diverging bar dimensions must leave space for labels, both sides of zero and values and stay within the recipe budget")
	}

	rowHeightFits := o.RowH >= minReadableBarRowHeight
	rows := make([]divergingRow, 0, len(d.Rows))
	extent := o.Max
	valueWidth := float64(56)
	plotFits := true
	plottedRoles := make(map[string]bool, len(d.Rows))
	for _, row := range d.Rows {
		if row.Value == nil {
			continue
		}
		value := rounded(*row.Value, c.Decimals)
		magnitude := math.Abs(value)
		if o.Max > 0 && magnitude > o.Max {
			return "", fmt.Errorf("analytics: diverging bar extent clips row %q", row.ID)
		}
		if o.Max == 0 && magnitude > extent {
			extent = magnitude
		}
		rows = append(rows, divergingRow{row: row, value: value})
		plottedRoles[rowRole(row)] = true
		if uniseg.StringWidth(row.Label)*11 > o.LabelW {
			plotFits = false
		}
		printed := number(value, c.Decimals, true) + " " + c.Unit
		valueWidth = math.Max(valueWidth, float64(uniseg.StringWidth(printed))*11+8)
	}
	if len(rows) > 0 && extent == 0 {
		extent = 1
	}
	plotWidth := float64(o.W-o.LabelW-6) - valueWidth
	halfPlotWidth := plotWidth / 2
	negativeExtentLabel := divergingDomainNumber(-extent)
	positiveExtentLabel := divergingDomainNumber(extent)
	if halfPlotWidth < 48 ||
		halfPlotWidth < float64(uniseg.StringWidth(negativeExtentLabel))*11+8 ||
		halfPlotWidth < float64(uniseg.StringWidth(positiveExtentLabel))*11+8 {
		plotFits = false
	}
	subpixel := false
	if plotFits {
		for _, plotted := range rows {
			if plotted.value != 0 && math.Abs(plotted.value)/extent*halfPlotWidth < 1 {
				subpixel = true
				break
			}
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<p class="mb-2 text-sm text-ink-muted">Unit: %s. Grain: %s. Aggregation: %s. Values rounded to %d decimal places.</p>`, html.EscapeString(c.Unit), html.EscapeString(c.Grain), html.EscapeString(c.Aggregation), c.Decimals)
	fmt.Fprintf(&b, `<p class="mb-2 text-sm text-ink-muted">%s</p>`, html.EscapeString(c.Window.label()))
	fmt.Fprintf(&b, `<p class="mb-2 text-sm text-ink-muted">%s. Coverage: %s`, html.EscapeString(c.Freshness.label(now)), html.EscapeString(string(c.Completeness)))
	if c.CoverageNote != "" {
		fmt.Fprintf(&b, `; %s`, html.EscapeString(c.CoverageNote))
	}
	fmt.Fprintf(&b, `.</p><p class="mb-3 text-sm text-ink-muted">Uncertainty: %s</p>`, html.EscapeString(c.Uncertainty))
	if len(rows) == 0 {
		b.WriteString(`<p class="mb-3 text-sm text-ink">No measured values in this selection.</p>`)
	} else if !rowHeightFits {
		b.WriteString(barRowHeightFallback)
	} else if !plotFits {
		b.WriteString(`<p class="mb-3 text-sm text-ink">Bar chart omitted: labels or values do not leave at least 48 pixels on each side of zero. All selected values remain in the data table. Choose wider chart and label columns or an app-owned wrapping layout to show the diverging bars.</p>`)
	} else if subpixel {
		b.WriteString(`<p class="mb-3 text-sm text-ink">Bar chart omitted: one or more nonzero values would render smaller than one logical pixel at this scale. All selected values remain in the data table. Choose a narrower selection or an app-owned alternative representation to compare these values graphically.</p>`)
	} else {
		fmt.Fprintf(&b, `<div class="overflow-x-auto focus-visible:outline-2 focus-visible:outline-focus" tabindex="0" role="region" aria-label="%s diverging bar chart, horizontally scrollable if needed">`, html.EscapeString(c.Title))
		b.WriteString(renderDivergingSVG(rows, c, o, extent, valueWidth))
		b.WriteString(`</div>`)
	}

	if rowHeightFits && plotFits && !subpixel && len(rows) > 0 {
		var plottedLegend []LegendItem
		for _, item := range d.Legend {
			if plottedRoles[item.Role] {
				plottedLegend = append(plottedLegend, item)
			}
		}
		if len(plottedLegend) > 0 {
			b.WriteString(`<ul aria-label="Chart legend" class="mt-2 flex flex-wrap gap-3 text-sm text-ink-muted">`)
			for _, item := range plottedLegend {
				fmt.Fprintf(&b, `<li class="inline-flex items-center gap-1.5"><svg id="%s" data-analytics-legend-swatch aria-hidden="true" focusable="false" width="12" height="12" viewBox="0 0 12 12" class="inline-block size-3 shrink-0"><rect width="12" height="12" rx="2" fill="var(--ak-color-%s)"></rect></svg>%s</li>`, html.EscapeString(c.ID+"-diverging-legend-"+item.Role), item.Role, html.EscapeString(item.Label))
			}
			b.WriteString(`</ul>`)
		}
	}
	b.WriteString(renderTable(d))
	return vizframe.Render(b.String(), vizframe.Options{ID: c.ID, Title: c.Title, Source: c.Source}), nil
}

func renderDivergingSVG(rows []divergingRow, c Contract, o viz.BarsOptions, extent, valueWidth float64) string {
	const axisHeight = 22
	w := float64(o.W)
	trackX := float64(o.LabelW + 6)
	plotEnd := w - valueWidth
	zeroX := trackX + (plotEnd-trackX)/2
	totalH := axisHeight + len(rows)*o.RowH + (len(rows)-1)*o.Gap
	aria := c.Title + " (" + c.Unit + ", symmetric around zero)"

	var b strings.Builder
	fmt.Fprintf(&b, `<svg data-analytics-diverging="true" width="%d" height="%d" viewBox="0 0 %d %d" role="img" aria-label="%s" xmlns="http://www.w3.org/2000/svg"><title>%s</title>`, o.W, totalH, o.W, totalH, html.EscapeString(aria), html.EscapeString(aria))
	fmt.Fprintf(&b, `<g><line data-analytics-zero-axis="true" x1="%s" x2="%s" y1="%d" y2="%d" stroke="var(--ak-color-ink-muted)"/>`, divergingCoordinate(zeroX), divergingCoordinate(zeroX), axisHeight, totalH)
	fmt.Fprintf(&b, `<text x="%s" y="11" text-anchor="start" fill="var(--ak-color-ink-muted)" font-size="11">%s</text>`, divergingCoordinate(trackX), html.EscapeString(divergingDomainNumber(-extent)))
	fmt.Fprintf(&b, `<text x="%s" y="11" text-anchor="middle" fill="var(--ak-color-ink-muted)" font-size="11">0</text>`, divergingCoordinate(zeroX))
	fmt.Fprintf(&b, `<text x="%s" y="11" text-anchor="end" fill="var(--ak-color-ink-muted)" font-size="11">%s</text></g>`, divergingCoordinate(plotEnd), html.EscapeString(divergingDomainNumber(extent)))

	for i, plotted := range rows {
		y := float64(axisHeight + i*(o.RowH+o.Gap))
		mid := y + float64(o.RowH)/2
		markHeight := float64(o.RowH) * .7
		markY := y + float64(o.RowH)*.15
		rowID := html.EscapeString(plotted.row.ID)
		fmt.Fprintf(&b, `<g data-analytics-row="%s"><text x="0" y="%s" dominant-baseline="middle" fill="var(--ak-color-ink-muted)" font-size="11">%s</text>`, rowID, divergingCoordinate(mid), html.EscapeString(plotted.row.Label))
		fmt.Fprintf(&b, `<line x1="%s" x2="%s" y1="%s" y2="%s" stroke="var(--ak-color-line)"/>`, divergingCoordinate(trackX), divergingCoordinate(plotEnd), divergingCoordinate(mid), divergingCoordinate(mid))
		if plotted.value == 0 {
			fmt.Fprintf(&b, `<line data-analytics-zero="true" data-value="0" data-extent="%s" x1="%s" x2="%s" y1="%s" y2="%s" stroke="var(--ak-color-%s)" stroke-width="4"/>`, divergingCoordinate(extent), divergingCoordinate(zeroX), divergingCoordinate(zeroX), divergingCoordinate(markY), divergingCoordinate(markY+markHeight), rowRole(plotted.row))
		} else {
			width := math.Abs(plotted.value) / extent * (plotEnd - trackX) / 2
			x := zeroX
			if plotted.value < 0 {
				x -= width
			}
			fmt.Fprintf(&b, `<rect data-analytics-bar="true" data-value="%s" data-extent="%s" x="%s" y="%s" width="%s" height="%s" rx="3" fill="var(--ak-color-%s)"><title>%s: %s %s</title></rect>`, html.EscapeString(number(plotted.value, c.Decimals, false)), divergingCoordinate(extent), divergingCoordinate(x), divergingCoordinate(markY), divergingCoordinate(width), divergingCoordinate(markHeight), rowRole(plotted.row), html.EscapeString(plotted.row.Label), html.EscapeString(number(plotted.value, c.Decimals, true)), html.EscapeString(c.Unit))
		}
		fmt.Fprintf(&b, `<text data-analytics-diverging-value="true" x="%s" y="%s" text-anchor="end" dominant-baseline="middle" fill="var(--ak-color-ink)" font-size="11">%s %s</text></g>`, divergingCoordinate(w), divergingCoordinate(mid), html.EscapeString(number(plotted.value, c.Decimals, true)), html.EscapeString(c.Unit))
	}
	b.WriteString(`</svg>`)
	return b.String()
}

func divergingCoordinate(value float64) string {
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(value, 'f', 6, 64), "0"), ".")
}

func divergingDomainNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
