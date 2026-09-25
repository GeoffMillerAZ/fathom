package gallery

import (
	"bytes"
	"html/template"
	"net/http"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/ui/analytics"
	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
	"github.com/geoffmilleraz/app-kit/appkit/ui/vizframe"
)

const analyticsSignedPath = "/analytics/signed"
const analyticsSignedExportPath = "/analytics/signed/export.csv"

// signedAnalyticsDataset is a fixed, synthetic single-window fixture. No paired
// series, financial policy or live source is implied by signed observations.
func signedAnalyticsDataset() (analytics.Dataset, time.Time) {
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	values := []float64{-2, -1, 0, 1, 2}
	ids := []string{"n2", "n1", "zero", "p1", "p2"}
	labels := []string{"Negative two", "Negative one", "Zero", "Positive one", "Positive two"}
	d := analytics.Dataset{Contract: analytics.Contract{
		ID: "signed-observations", Title: "Signed observations", Unit: "points", Decimals: 2,
		Grain: "synthetic observation", Aggregation: "one selected observation per row; no aggregation",
		Window:       analytics.Window{Kind: analytics.SnapshotWindow, Start: now, TimeZone: "UTC"},
		Source:       vizframe.Derived("Fixed synthetic signed observations; not production measurements."),
		Freshness:    analytics.Freshness{UnknownReason: "Synthetic fixture has no independent source observation time"},
		Completeness: analytics.Partial, CoverageNote: "Five measured values and one unobserved row; not a complete total.",
		Uncertainty: "Illustrative values only; uncertainty is not quantified.",
	}, Legend: []analytics.LegendItem{{Role: "accent", Label: "Synthetic observation; sign is not a quality rating"}}}
	for i := range values {
		d.Rows = append(d.Rows, analytics.Row{ID: ids[i], Label: labels[i], Value: &values[i]})
	}
	d.Rows = append(d.Rows, analytics.Row{ID: "missing", Label: "Unobserved", MissingReason: "Not reported"})
	return d, now
}

func serveAnalyticsSigned(w http.ResponseWriter, r *http.Request) {
	defaults, _ := parseAnalyticsSelection(nil)
	if r.URL.Opaque != "" || r.URL.EscapedPath() != r.URL.Path || (r.URL.Path != analyticsSignedPath && r.URL.Path != analyticsSignedExportPath) {
		writeAnalyticsError(w, r, http.StatusNotFound, "This signed analytics specimen does not exist.", defaults, "")
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		writeAnalyticsError(w, r, http.StatusMethodNotAllowed, "This synthetic specimen is read-only. Use GET or HEAD; no data has changed.", defaults, "")
		return
	}
	selection, theme, err := analyticsRequestSelection(r)
	// The shared frame emits the four exact default selection values in its
	// appearance links. Admit those inert defaults, never changed filters or
	// arbitrary keys that could misleadingly look like a successful selection.
	queryOK := true
	for key, values := range r.URL.Query() {
		if len(values) != 1 || (key != "theme" && key != "period" && key != "compare" && key != "category" && key != "state") {
			queryOK = false
		}
	}
	if err != nil || selection != defaults || !queryOK {
		writeAnalyticsError(w, r, http.StatusBadRequest, "This fixed signed specimen accepts appearance changes only; use /analytics for selectable reporting windows.", defaults, theme)
		return
	}
	d, now := signedAnalyticsDataset()
	if r.URL.Path == analyticsSignedExportPath {
		var out bytes.Buffer
		if err := analytics.WriteCSV(&out, d, now); err != nil {
			writeAnalyticsError(w, r, http.StatusInternalServerError, "The signed export could not be prepared.", defaults, theme)
			return
		}
		w.Header().Set("Content-Disposition", `attachment; filename="signed-observations.csv"`)
		writeAnalyticsResponse(w, r, http.StatusOK, "text/csv; charset=utf-8", out.Bytes())
		return
	}
	markup, err := analytics.RenderDivergingBars(d, viz.BarsOptions{W: 640}, now)
	if err != nil {
		writeAnalyticsPage(w, r, http.StatusOK, "Signed analytics", "", err, defaults, theme, analyticsSignedPath)
		return
	}
	content, err := htmlf("analytics-signed", analyticsSignedHTML, struct {
		Back, Export, Catalog string
		Rendered              template.HTML
	}{previewURL(analyticsRecipePath, theme), previewURL(analyticsSignedExportPath, theme), previewURL("/catalog/analytics.comparison", theme), template.HTML(markup)})
	writeAnalyticsPage(w, r, http.StatusOK, "Signed analytics", content, err, defaults, theme, analyticsSignedPath)
}

const analyticsSignedHTML = `
<header class="border-b border-line pb-6">
  <a href="{{.Back}}" class="inline-flex min-h-11 items-center text-sm text-accent underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-focus">Back to analytics recipe</a>
  <div class="mt-3 flex flex-wrap items-start justify-between gap-4">
    <div><p class="text-sm font-medium text-ink-muted">Analytics recipe · fixed synthetic data</p>
      <h1 class="mt-2 text-2xl font-semibold tracking-tight text-ink">Compare signed observations</h1>
      <p class="mt-2 max-w-[72ch] text-sm text-ink-muted">Negative values extend left of zero; positive values extend right. Equal magnitudes have equal lengths. A measured zero is a mark; missing observations remain in the table, not the plot.</p></div>
    <a href="{{.Export}}" class="inline-flex min-h-11 items-center justify-center rounded-md border border-line-strong bg-surface-raised px-4 py-2 text-sm font-medium text-ink hover:bg-surface-sunken focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">Export signed CSV</a>
  </div>
</header>
<section class="mt-8" aria-label="Signed renderer output">{{.Rendered}}</section>
<aside class="mt-8 border-t border-line pt-5 text-sm text-ink-muted" aria-label="Integration and proof status">
  <p class="max-w-[72ch]">The public <code class="font-mono">analytics.RenderDivergingBars</code> and <code class="font-mono">analytics.WriteCSV</code> use the same dataset and fixed clock. One reporting window only; app-owned paired-series and detail navigation remain separate.</p>
  <p class="mt-2 max-w-[72ch]">Experimental Gallery integration is not consumer or human-usability acceptance. <a href="{{.Catalog}}" class="text-accent underline underline-offset-4">Inspect the capability and its limits</a>.</p>
</aside>`
