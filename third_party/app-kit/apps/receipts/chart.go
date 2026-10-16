package receipts

import (
	"fmt"
	"html"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/analytics"
)

const chartZero = int64(300)

// renderSignedComparison renders the private P11 chart body. Its caller owns
// framing, provenance, page composition and the canonical detail URL policy.
func renderSignedComparison(p projection, detailURL func(string) string) (string, error) {
	if detailURL == nil {
		return "", fmt.Errorf("receipts chart: detail URL callback is required")
	}
	if err := validateChartSeries(p.Current, p.Facts, p.Now); err != nil {
		return "", fmt.Errorf("receipts chart: current: %w", err)
	}
	if p.Selection.Compare != "none" && p.Selection.Compare != "previous" {
		return "", fmt.Errorf("receipts chart: unsupported comparison %q", p.Selection.Compare)
	}
	if p.Selection.Compare == "none" {
		if p.Previous != nil || len(p.PreviousFacts) != 0 {
			return "", fmt.Errorf("receipts chart: comparison-none contains previous values")
		}
	} else {
		if p.Previous == nil {
			return "", fmt.Errorf("receipts chart: previous comparison is missing")
		}
		if err := validateChartSeries(*p.Previous, p.PreviousFacts, p.Now); err != nil {
			return "", fmt.Errorf("receipts chart: previous: %w", err)
		}
		if len(p.Facts) != len(p.PreviousFacts) {
			return "", fmt.Errorf("receipts chart: series product counts differ")
		}
		for i := range p.Facts {
			if p.Facts[i].Product.ID != p.PreviousFacts[i].Product.ID || p.Facts[i].Product.Label != p.PreviousFacts[i].Product.Label {
				return "", fmt.Errorf("receipts chart: series product identity differs")
			}
		}
	}

	extent := int64(100) // Flat and all-missing selections retain a USD 1.00 domain.
	for _, facts := range [][]projectedFact{p.Facts, p.PreviousFacts} {
		for _, item := range facts {
			if item.NetCents == nil {
				continue
			}
			if *item.NetCents < -maxAmountCents || *item.NetCents > maxAmountCents {
				return "", fmt.Errorf("receipts chart: net value for %q exceeds the chart domain", item.Product.ID)
			}
			if magnitude := absChartCents(*item.NetCents); magnitude > extent {
				extent = magnitude
			}
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<div class="receipts-chart" data-extent="%d"><div class="receipts-chart-axis" data-extent="%d"><span>%s</span><span>%s</span><span>%s</span></div>`,
		extent, extent, html.EscapeString(axisMoney(-extent)), html.EscapeString(moneyCents(0)+" USD"), html.EscapeString(axisMoney(extent)))
	for index, current := range p.Facts {
		href := detailURL(current.Product.ID)
		if err := validateLocalChartHref(href); err != nil {
			return "", fmt.Errorf("receipts chart: detail URL for %q: %w", current.Product.ID, err)
		}
		escapedHref := html.EscapeString(href)
		fmt.Fprintf(&b, `<section class="receipts-chart-product" data-product="%s"><a class="receipts-chart-product-link" href="%s">%s</a>`,
			html.EscapeString(current.Product.ID), escapedHref, html.EscapeString(current.Product.Label))
		writeChartSeries(&b, current, "current", periodLabel(p.Current.Contract), extent, escapedHref)
		if p.Previous != nil {
			writeChartSeries(&b, p.PreviousFacts[index], "previous", periodLabel(p.Previous.Contract), extent, escapedHref)
		}
		b.WriteString(`</section>`)
	}
	b.WriteString(`</div>`)
	return b.String(), nil
}

func validateChartSeries(dataset analytics.Dataset, facts []projectedFact, now time.Time) error {
	if err := dataset.Validate(now); err != nil {
		return err
	}
	if dataset.Contract.Unit != "USD" || dataset.Contract.Decimals != 2 {
		return fmt.Errorf("net dataset must use USD with two decimal places")
	}
	if len(dataset.Rows) != len(facts) {
		return fmt.Errorf("dataset and fact row counts differ")
	}
	for i, item := range facts {
		row := dataset.Rows[i]
		if row.ID != item.Product.ID || row.Label != item.Product.Label {
			return fmt.Errorf("dataset and fact identity differ at row %d", i)
		}
		if (row.Value == nil) != (item.NetCents == nil) {
			return fmt.Errorf("dataset and fact missingness differ for %q", row.ID)
		}
		if row.MissingReason != item.MissingReason {
			return fmt.Errorf("dataset and fact missing reason differ for %q", row.ID)
		}
		if item.NetCents != nil && *row.Value != float64(*item.NetCents)/100 {
			return fmt.Errorf("dataset and fact value differ for %q", row.ID)
		}
	}
	return nil
}

func writeChartSeries(b *strings.Builder, item projectedFact, series, period string, extent int64, escapedHref string) {
	seriesName := "Current"
	if series == "previous" {
		seriesName = "Previous"
	}
	valueText := "Missing: " + item.MissingReason
	accessibleValue := "missing, " + item.MissingReason
	if item.NetCents != nil {
		valueText = moneyCents(*item.NetCents) + " USD"
		accessibleValue = valueText
	}
	linkLabel := fmt.Sprintf("Open %s detail: %s, %s net receipts, %s", item.Product.Label, period, strings.ToLower(seriesName), accessibleValue)
	svgLabel := fmt.Sprintf("%s, %s, %s net receipts, %s", item.Product.Label, period, strings.ToLower(seriesName), accessibleValue)
	fmt.Fprintf(b, `<div class="receipts-chart-series" data-product="%s" data-series="%s"><span class="receipts-chart-series-label">%s · %s</span> <span class="receipts-chart-value" data-product="%s" data-series="%s"`,
		html.EscapeString(item.Product.ID), series, seriesName, html.EscapeString(period), html.EscapeString(item.Product.ID), series)
	if item.NetCents != nil {
		fmt.Fprintf(b, ` data-cents="%d" data-zero="%t" data-extent="%d"`, *item.NetCents, *item.NetCents == 0, extent)
	}
	fmt.Fprintf(b, `>%s</span>`, html.EscapeString(valueText))
	fmt.Fprintf(b, `<svg class="receipts-chart-svg" viewBox="0 0 600 44" preserveAspectRatio="none" role="group" aria-label="%s"><line class="receipts-chart-axis-line" x1="300" x2="300" y1="4" y2="40" stroke="var(--ak-color-line-strong)"/>`, html.EscapeString(svgLabel))
	fmt.Fprintf(b, `<a href="%s" aria-label="%s" data-product="%s" data-series="%s"`, escapedHref, html.EscapeString(linkLabel), html.EscapeString(item.Product.ID), series)
	if item.NetCents != nil {
		fmt.Fprintf(b, ` data-cents="%d" data-zero="%t" data-extent="%d"`, *item.NetCents, *item.NetCents == 0, extent)
	}
	b.WriteString(`><rect class="receipts-chart-hit" x="0" y="0" width="600" height="44" fill="transparent"/>`)
	if item.NetCents != nil {
		writeChartMark(b, *item.NetCents, series, extent, item.Product.ID)
	}
	b.WriteString(`</a></svg></div>`)
}

func writeChartMark(b *strings.Builder, cents int64, series string, extent int64, productID string) {
	width := float64(absChartCents(cents)) / float64(extent) * float64(chartZero)
	x := float64(chartZero)
	if cents < 0 {
		x -= width
	}
	fill, stroke, strokeWidth := "var(--ak-color-accent)", "none", "0"
	if series == "previous" {
		fill, stroke, strokeWidth = "none", "var(--ak-color-ink-muted)", "2"
	}
	fmt.Fprintf(b, `<rect class="receipts-chart-bar" data-product="%s" data-series="%s" data-cents="%d" data-zero="%t" data-extent="%d" x="%s" y="14" width="%s" height="16" fill="%s" stroke="%s" stroke-width="%s"/>`,
		html.EscapeString(productID), series, cents, cents == 0, extent, chartCoordinate(x), chartCoordinate(width), fill, stroke, strokeWidth)
	if cents == 0 {
		stroke, strokeWidth = "var(--ak-color-accent)", "4"
		if series == "previous" {
			stroke, strokeWidth = "var(--ak-color-ink-muted)", "2"
		}
		fmt.Fprintf(b, `<line class="receipts-chart-zero" data-product="%s" data-series="%s" data-cents="0" data-zero="true" data-extent="%d" x1="300" x2="300" y1="10" y2="34" stroke="%s" stroke-width="%s"/>`,
			html.EscapeString(productID), series, extent, stroke, strokeWidth)
	}
}

func validateLocalChartHref(href string) error {
	if href == "" || strings.ContainsAny(href, "\r\n\t\\") {
		return fmt.Errorf("must be a nonempty local URL")
	}
	u, err := url.Parse(href)
	if err != nil || u.Scheme != "" || u.Host != "" || u.User != nil || u.Opaque != "" || u.Fragment != "" || u.RawPath != "" ||
		!strings.HasPrefix(u.Path, "/") || strings.HasPrefix(u.Path, "//") || path.Clean(u.Path) != u.Path {
		return fmt.Errorf("must be a canonical local URL")
	}
	return nil
}

func absChartCents(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

func chartCoordinate(value float64) string {
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(value, 'f', 6, 64), "0"), ".")
}

func axisMoney(cents int64) string {
	if cents > 0 {
		return "+" + moneyCents(cents) + " USD"
	}
	return moneyCents(cents) + " USD"
}
