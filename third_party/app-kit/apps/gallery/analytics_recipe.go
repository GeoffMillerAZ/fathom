package gallery

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"

	"github.com/geoffmilleraz/app-kit/appkit/ui/analytics"
	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
)

const analyticsRecipePath = "/analytics/recipe"
const analyticsRecipeExportPath = "/analytics/recipe/export.csv"
const analyticsRecipeTableExportPath = "/analytics/recipe/table.csv"

type analyticsRecipeView struct {
	Selection             analyticsSelection
	Theme                 string
	Back, Export, Catalog string
	TableExport           string
	AllGroups             string
	Empty                 bool
	Rendered              template.HTML
}

// serveAnalyticsRecipe is a runnable specimen of the public analytics renderer.
// It intentionally fixes comparison to none because RenderBars owns one dataset
// and one reporting window; the Gallery's paired-period journey remains separate.
func serveAnalyticsRecipe(w http.ResponseWriter, r *http.Request) {
	serveAnalyticsRecipeWithBuilder(w, r, buildAnalyticsJourney)
}

func serveAnalyticsRecipeWithBuilder(w http.ResponseWriter, r *http.Request, build func(analyticsSelection) (analyticsJourney, error)) {
	defaults, _ := parseAnalyticsSelection(nil)
	defaults.Compare = analyticsCompareNone
	if r.URL.Opaque != "" || r.URL.EscapedPath() != r.URL.Path ||
		(r.URL.Path != analyticsRecipePath && r.URL.Path != analyticsRecipeExportPath && r.URL.Path != analyticsRecipeTableExportPath) {
		writeAnalyticsError(w, r, http.StatusNotFound, "This analytics recipe page does not exist. Return to the service comparison.", defaults, "")
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		writeAnalyticsError(w, r, http.StatusMethodNotAllowed, "This synthetic recipe is read-only. Open it with GET or HEAD; no data has changed.", defaults, "")
		return
	}

	selection, theme, err := analyticsRequestSelection(r)
	if err != nil {
		writeAnalyticsError(w, r, http.StatusBadRequest, "These recipe filters could not be read. Choose one listed value per filter and light, dark or system appearance.", defaults, theme)
		return
	}
	selection.Compare = analyticsCompareNone
	if selection.State == analyticsStateError {
		writeAnalyticsSourceError(w, r, selection, theme, analyticsRecipePath)
		return
	}
	j, err := build(selection)
	if err != nil {
		writeAnalyticsError(w, r, http.StatusInternalServerError, "The synthetic recipe fixture could not be prepared. Return to the comparison and retry.", selection, theme)
		return
	}
	dataset := analyticsRecipeDataset(j, theme)

	if r.URL.Path == analyticsRecipeExportPath || r.URL.Path == analyticsRecipeTableExportPath {
		var body bytes.Buffer
		writeCSV := analytics.WriteCSV
		filename := fmt.Sprintf("service-operations-recipe-%s.csv", selection.Period)
		if r.URL.Path == analyticsRecipeTableExportPath {
			writeCSV = analytics.WriteTableCSV
			filename = fmt.Sprintf("service-operations-table-%s.csv", selection.Period)
		}
		if err := writeCSV(&body, dataset, j.Now); err != nil {
			writeAnalyticsError(w, r, http.StatusInternalServerError, "The recipe export could not be prepared. Return to the comparison and retry.", selection, theme)
			return
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		writeAnalyticsResponse(w, r, http.StatusOK, "text/csv; charset=utf-8", body.Bytes())
		return
	}

	rendered, err := analytics.RenderBars(dataset, viz.BarsOptions{}, j.Now)
	if err != nil {
		writeAnalyticsPage(w, r, http.StatusOK, "Public analytics renderer", "", err, selection, theme, analyticsRecipePath)
		return
	}
	// RenderBars escapes dataset text and returns complete first-party markup.
	allGroups := selection
	allGroups.Category = analyticsCategoryAll
	content, err := htmlf("analytics-recipe", analyticsRecipeHTML, analyticsRecipeView{
		Selection:   selection,
		Theme:       theme,
		Back:        analyticsURL("/analytics", selection, theme),
		Export:      analyticsURL(analyticsRecipeExportPath, selection, theme),
		TableExport: analyticsURL(analyticsRecipeTableExportPath, selection, theme),
		AllGroups:   analyticsURL(analyticsRecipePath, allGroups, theme),
		Empty:       len(dataset.Rows) == 0,
		Catalog:     previewURL("/catalog/analytics.comparison", theme),
		Rendered:    template.HTML(rendered),
	})
	writeAnalyticsPage(w, r, http.StatusOK, "Public analytics renderer", content, err, selection, theme, analyticsRecipePath)
}

// analyticsRecipeDataset adds only destinations that the built journey can
// resolve. The row copy keeps app-owned presentation state out of source data.
func analyticsRecipeDataset(j analyticsJourney, theme string) analytics.Dataset {
	dataset := j.Current
	dataset.Rows = append([]analytics.Row(nil), j.Current.Rows...)
	for i := range dataset.Rows {
		detail, ok := j.Detail(dataset.Rows[i].ID)
		if !ok {
			continue
		}
		dataset.Rows[i].DetailHref = analyticsURL("/analytics/"+detail.ID, j.Selection, theme)
	}
	return dataset
}

const analyticsRecipeHTML = `
<header class="border-b border-line pb-6">
  <a href="{{.Back}}" class="inline-flex min-h-11 items-center text-sm text-accent underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-focus">Back to the comparison journey</a>
  <div class="mt-2 flex flex-wrap items-start justify-between gap-4">
    <div class="min-w-0">
      <p class="text-xs font-medium uppercase tracking-wide text-ink-muted">Analytics recipe · synthetic data</p>
      <h1 class="mt-2 text-2xl font-semibold tracking-tight text-ink">Public bar renderer</h1>
      <p class="mt-2 max-w-[72ch] text-sm text-ink-muted">A single selected reporting window rendered directly by <code class="font-mono">analytics.RenderBars</code>. Open a table row label for service detail; browser Back returns here. Use the comparison journey for paired periods and differences.</p>
    </div>
    <div>
      <a href="{{.Export}}" class="inline-flex min-h-11 items-center justify-center rounded-md border border-line-strong bg-surface-raised px-4 py-2 text-sm font-medium text-ink hover:bg-surface-sunken focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">Export renderer CSV</a>
      <a href="{{.TableExport}}" aria-describedby="analytics-table-export-help" class="mt-2 flex min-h-11 items-center text-sm text-accent underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-focus">Export spreadsheet CSV</a>
      <p id="analytics-table-export-help" class="mt-1 max-w-[72ch] text-xs text-ink-muted">{{if .Empty}}No selected rows: spreadsheet CSV contains only column headers, with no context values. Use renderer CSV to retain current-period context.{{else}}One table, with source and reporting context on every row.{{end}}</p>
    </div>
  </div>
</header>

<form action="/analytics/recipe" method="get" class="mt-6">
  <input type="hidden" name="compare" value="none">
  {{if .Theme}}<input type="hidden" name="theme" value="{{.Theme}}">{{end}}
  <fieldset class="grid min-w-0 gap-4 sm:grid-cols-3">
    <legend class="mb-3 text-sm font-medium text-ink">Choose the renderer fixture</legend>
    <label class="block min-w-0 text-sm text-ink">Reporting month
      <select name="period" class="mt-1 block min-h-11 w-full rounded-md border border-line-strong bg-surface px-3 py-2 text-ink focus-visible:outline-2 focus-visible:outline-focus">
        <option value="2026-09"{{if eq .Selection.Period "2026-09"}} selected{{end}}>September 2026</option>
        <option value="2026-08"{{if eq .Selection.Period "2026-08"}} selected{{end}}>August 2026</option>
      </select>
    </label>
    <label class="block min-w-0 text-sm text-ink">Service group
      <select name="category" class="mt-1 block min-h-11 w-full rounded-md border border-line-strong bg-surface px-3 py-2 text-ink focus-visible:outline-2 focus-visible:outline-focus">
        <option value="all"{{if eq .Selection.Category "all"}} selected{{end}}>All services</option>
        <option value="api"{{if eq .Selection.Category "api"}} selected{{end}}>API services</option>
        <option value="worker"{{if eq .Selection.Category "worker"}} selected{{end}}>Worker services</option>
        <option value="empty"{{if eq .Selection.Category "empty"}} selected{{end}}>Empty group (no fixture services)</option>
      </select>
    </label>
    <label class="block min-w-0 text-sm text-ink">Source condition
      <select name="state" class="mt-1 block min-h-11 w-full rounded-md border border-line-strong bg-surface px-3 py-2 text-ink focus-visible:outline-2 focus-visible:outline-focus">
        <option value="complete"{{if eq .Selection.State "complete"}} selected{{end}}>Complete reporting window</option>
        <option value="missing"{{if eq .Selection.State "missing"}} selected{{end}}>Missing source reports</option>
        <option value="stale"{{if eq .Selection.State "stale"}} selected{{end}}>Stale source observation</option>
        <option value="partial"{{if eq .Selection.State "partial"}} selected{{end}}>Partial month: first 20 days</option>
        <option value="error"{{if eq .Selection.State "error"}} selected{{end}}>Source read failure</option>
      </select>
    </label>
    <button type="submit" class="min-h-11 justify-self-start rounded-md bg-accent px-4 py-2 text-sm font-medium text-accent-ink hover:bg-accent-hover focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">Apply fixture</button>
  </fieldset>
</form>

{{if .Empty}}
<section aria-labelledby="analytics-empty" class="mt-8">
  <h2 id="analytics-empty" class="text-lg font-semibold text-ink">No matching services</h2>
  <p class="mt-2 max-w-[72ch] text-sm text-ink-muted">This group contains no fixture services, not zero failed operations. Source availability is a separate condition. The shared renderer below retains the selected period and source context without inventing measurements.</p>
  <a href="{{.AllGroups}}" class="mt-3 inline-flex min-h-11 items-center text-sm text-accent underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-focus">Show all service groups</a>
</section>
{{end}}

<section aria-labelledby="analytics-recipe-output" class="mt-8">
  <h2 id="analytics-recipe-output" class="sr-only">Rendered analytics output</h2>
  {{.Rendered}}
</section>

<aside class="mt-8 border-t border-line pt-5 text-sm text-ink-muted" aria-label="Integration and proof status">
  <p class="max-w-[72ch]">The figure and exports use the same dataset and fixed fixture clock. <code class="font-mono">analytics.WriteCSV</code> keeps a separate context section; <code class="font-mono">analytics.WriteTableCSV</code> repeats it on each data row. With no selected rows, the spreadsheet format contains only headers; use renderer CSV to retain empty-selection context.</p>
  <p class="mt-2 max-w-[72ch]">This runnable specimen proves Gallery integration, not consumer adoption or production-data correctness. <a href="{{.Catalog}}" class="text-accent underline underline-offset-4">Inspect the capability and its limits</a>.</p>
  <p class="mt-2 max-w-[72ch]">Need negative values? <a href="/analytics/signed{{if .Theme}}?theme={{.Theme}}{{end}}" class="inline-flex min-h-11 items-center text-accent underline underline-offset-4">Inspect the signed renderer and matching CSV</a>.</p>
</aside>`
