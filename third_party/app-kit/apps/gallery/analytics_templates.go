package gallery

const analyticsHTML = `
<header class="border-b border-line pb-6">
  {{if .Detail}}<a href="{{.Back}}" class="inline-flex min-h-11 items-center text-sm text-accent underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-focus">Back to selected report</a>{{end}}
  <div class="flex flex-wrap items-start justify-between gap-4">
    <div class="min-w-0">
      <p class="text-xs font-medium uppercase tracking-wide text-ink-muted">Analytics reference · synthetic data</p>
      <h1 class="mt-2 text-2xl font-semibold tracking-tight text-ink [overflow-wrap:anywhere]">{{.Title}}</h1>
      <p class="mt-2 text-sm text-ink-muted">{{.CurrentLabel}}{{if .PreviousLabel}}{{if .PlotPrevious}} compared with {{else}}; reference month: {{end}}{{.PreviousLabel}}{{end}} · UTC</p>
    </div>
    <a href="{{.Export}}"{{if not .Rows}} aria-describedby="analytics-empty-export-help"{{end}} class="inline-flex min-h-11 items-center justify-center rounded-md border border-line-strong bg-surface-raised px-4 py-2 text-sm font-medium text-ink hover:bg-surface-sunken focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">{{.ExportLabel}}</a>
  </div>
</header>

{{if not .Detail}}
<details id="analytics-selection" class="mt-6 border-b border-line pb-4">
  <summary class="min-h-11 cursor-pointer py-3 text-sm font-medium text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">Change comparison <span class="mt-1 block text-sm font-normal text-ink-muted">{{.SelectionLabel}}</span></summary>
<form id="analytics-filter" action="/analytics" method="get" class="mt-6 space-y-4">
  {{if .Theme}}<input type="hidden" name="theme" value="{{.Theme}}">{{end}}
  <fieldset class="grid min-w-0 gap-4 sm:grid-cols-2 lg:grid-cols-4">
    <legend class="mb-3 text-sm font-medium text-ink">Choose the comparison</legend>
    <label class="block min-w-0 text-sm text-ink">Reporting month
      <select name="period" class="mt-1 block min-h-11 w-full rounded-md border border-line-strong bg-surface px-3 py-2 text-ink focus-visible:outline-2 focus-visible:outline-focus">
        <option value="2026-09"{{if eq .Journey.Selection.Period "2026-09"}} selected{{end}}>September 2026</option>
        <option value="2026-08"{{if eq .Journey.Selection.Period "2026-08"}} selected{{end}}>August 2026</option>
      </select>
    </label>
    <label class="block min-w-0 text-sm text-ink">Compare with
      <select name="compare" class="mt-1 block min-h-11 w-full rounded-md border border-line-strong bg-surface px-3 py-2 text-ink focus-visible:outline-2 focus-visible:outline-focus">
        <option value="previous"{{if eq .Journey.Selection.Compare "previous"}} selected{{end}}>Previous calendar month</option>
        <option value="none"{{if eq .Journey.Selection.Compare "none"}} selected{{end}}>No comparison</option>
      </select>
    </label>
    <label class="block min-w-0 text-sm text-ink">Service group
      <select name="category" class="mt-1 block min-h-11 w-full rounded-md border border-line-strong bg-surface px-3 py-2 text-ink focus-visible:outline-2 focus-visible:outline-focus">
        <option value="all"{{if eq .Journey.Selection.Category "all"}} selected{{end}}>All services</option>
        <option value="api"{{if eq .Journey.Selection.Category "api"}} selected{{end}}>API services</option>
        <option value="worker"{{if eq .Journey.Selection.Category "worker"}} selected{{end}}>Worker services</option>
        <option value="empty"{{if eq .Journey.Selection.Category "empty"}} selected{{end}}>Empty group (no fixture services)</option>
      </select>
    </label>
    <div class="flex items-end gap-3">
      <button type="submit" class="min-h-11 rounded-md bg-accent px-4 py-2 text-sm font-medium text-accent-ink hover:bg-accent-hover focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">Apply filters</button>
      <a href="{{.Reset}}" class="inline-flex min-h-11 items-center text-sm text-ink-muted underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-focus">Reset</a>
    </div>
  </fieldset>
  <details class="rounded-md border border-line bg-surface-raised px-4 py-3"{{if ne .Journey.Selection.State "complete"}} open{{end}}>
    <summary class="min-h-11 cursor-pointer py-3 text-sm font-medium text-ink focus-visible:outline-2 focus-visible:outline-focus">Test a data condition</summary>
    <div class="mt-3 flex flex-wrap items-end gap-4">
      <label class="block text-sm text-ink">Source condition
        <select name="state" class="mt-1 block min-h-11 max-w-full rounded-md border border-line-strong bg-surface px-3 py-2 text-ink focus-visible:outline-2 focus-visible:outline-focus">
          <option value="complete"{{if eq .Journey.Selection.State "complete"}} selected{{end}}>Complete reporting window</option>
          <option value="missing"{{if eq .Journey.Selection.State "missing"}} selected{{end}}>Missing source reports</option>
          <option value="stale"{{if eq .Journey.Selection.State "stale"}} selected{{end}}>Stale source observation</option>
          <option value="partial"{{if eq .Journey.Selection.State "partial"}} selected{{end}}>Partial month: first 20 days</option>
          <option value="error"{{if eq .Journey.Selection.State "error"}} selected{{end}}>Source read failure</option>
        </select>
      </label>
      <p class="max-w-lg text-sm text-ink-muted">These controls switch reproducible fixtures, not a live service. Choose a condition, then Apply filters. Zero remains a measured value in the worker fixture.</p>
    </div>
  </details>
</form>
</details>
{{end}}

<section aria-labelledby="analytics-reading" class="mt-8">
  <h2 id="analytics-reading" class="text-lg font-semibold text-ink">{{if not .Rows}}No matching services{{else if .Detail}}What this service count means{{else}}What changed in the selection{{end}}</h2>
  <p class="mt-3 max-w-[72ch] leading-relaxed text-ink">{{.Summary}}</p>
  <div class="mt-4 rounded-md border border-line bg-surface-raised p-4">
    <h3 class="text-sm font-semibold text-ink">{{.StateLabel}}</h3>
    <p class="mt-1 max-w-[72ch] text-sm leading-relaxed text-ink-muted">{{.StateDescription}}</p>
    {{if and .Rows (ne .Journey.Selection.State "complete")}}<a href="{{.CompleteFixture}}" class="mt-2 inline-flex min-h-11 items-center text-sm text-accent underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-focus">Load the complete fixture</a>{{end}}
  </div>
  {{if not .Rows}}
  <a href="{{.AllGroups}}" class="mt-3 inline-flex min-h-11 items-center text-sm text-accent underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-focus">Show all service groups</a>
  <p id="analytics-empty-export-help" class="mt-3 max-w-[72ch] text-sm text-ink-muted">The comparison CSV contains only column headers, with no row or context values. No zero row is manufactured. To retain source and reporting context for the current period only, use the separate renderer export below.</p>
  <a href="{{.EmptyContextExport}}" class="mt-2 inline-flex min-h-11 items-center text-sm text-accent underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-focus">Export current-period context CSV</a>
  {{end}}
</section>

{{if .Rows}}
<section aria-labelledby="analytics-comparison" class="mt-8">
  <div class="flex flex-wrap items-baseline justify-between gap-2">
    <h2 id="analytics-comparison" class="text-lg font-semibold text-ink">{{if .Detail}}Period values{{else}}Compare services{{end}}</h2>
    {{if .HasPlottedValues}}<p class="text-sm text-ink-muted">Failed operations · shared scale 0 to {{.Scale}}</p>{{else}}<p class="text-sm text-ink-muted">No observed failed-operation values are available to plot.</p>{{end}}
  </div>
  <p class="mt-2 max-w-[72ch] text-sm text-ink-muted">{{if .PlotPrevious}}Each pair labels its actual month. The selected period uses accent; the previous period uses neutral.{{else}}Only observed values in the selected period are plotted. Missing counts have no bar.{{end}} Failure counts are not normalized for workload volume and are not reliability rates.</p>
  <ol class="mt-3 divide-y divide-line border-y border-line">
    {{range .Rows}}
    <li id="analytics-chart-{{.ID}}" data-service-id="{{.ID}}" class="grid min-w-0 items-center gap-3 py-5 sm:grid-cols-[minmax(0,1fr)_20rem]">
      <div class="min-w-0">
        <h3 class="font-medium text-ink [overflow-wrap:anywhere]">{{if $.Detail}}{{.Label}}{{else}}<a href="{{.Href}}" class="inline-flex min-h-11 items-center text-accent underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-focus">{{.Label}}</a>{{end}}</h3>
        <p class="mt-1 text-sm text-ink-muted">{{.Category}} · {{.State}}</p>
      </div>
      {{if .Chart}}<div class="min-w-0 overflow-x-auto py-1" tabindex="0" role="region" aria-label="{{.Label}} period chart, horizontally scrollable if needed">{{.Chart}}</div>{{else}}<p class="text-sm text-ink-muted">No current measurement. Missing is not zero.</p>{{end}}
    </li>
    {{end}}
  </ol>
</section>

<section aria-labelledby="analytics-values" class="mt-8">
  <h2 id="analytics-values" class="text-lg font-semibold text-ink">Exact values</h2>
  <p class="mt-2 text-sm text-ink-muted">The table and CSV retain missing values, source coverage and the same period selection.{{if eq .Journey.Selection.State "partial"}} A partial month cannot be compared as a complete month.{{end}}</p>
  <div class="mt-3 overflow-x-auto rounded-md border border-line" tabindex="0" role="region" aria-label="Service values table, horizontally scrollable if needed">
    <table id="analytics-value-table" class="w-full min-w-[38rem] text-left text-sm">
      <caption class="sr-only">Failed operations by service, {{.CurrentLabel}}{{if .PreviousLabel}} and {{.PreviousLabel}}{{end}}</caption>
      <thead class="bg-surface-raised text-ink-muted"><tr>
        <th scope="col" class="px-4 py-3 font-medium">Service</th>
        <th scope="col" class="px-4 py-3 text-right font-medium">{{.CurrentLabel}}</th>
        {{if .PreviousLabel}}<th scope="col" class="px-4 py-3 text-right font-medium">{{.PreviousLabel}}</th><th scope="col" class="px-4 py-3 font-medium">Count difference</th>{{end}}
        <th scope="col" class="px-4 py-3 font-medium">Current state</th>
      </tr></thead>
      <tbody class="divide-y divide-line">{{range .Rows}}<tr data-value-id="{{.ID}}">
        <th scope="row" class="max-w-xs px-4 py-0 font-medium [overflow-wrap:anywhere]">{{if $.Detail}}{{.Label}}{{else}}<a href="{{.Href}}" class="inline-flex min-h-11 items-center text-accent underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-focus">{{.Label}}</a>{{end}}</th>
        <td class="px-4 py-3 text-right tabular-nums">{{.Current}}</td>
        {{if $.PreviousLabel}}<td class="px-4 py-3 text-right tabular-nums">{{.Previous}}</td><td class="px-4 py-3">{{.Change}}</td>{{end}}
        <td class="px-4 py-3">{{.State}}</td>
      </tr>{{end}}</tbody>
    </table>
  </div>
</section>

{{end}}
<details class="mt-8 border-t border-line pt-5"{{if .Detail}} open{{end}}>
  <summary class="min-h-11 cursor-pointer py-3 text-sm font-semibold text-ink focus-visible:outline-2 focus-visible:outline-focus">Source, time window and interpretation</summary>
  <dl class="mt-4 grid gap-x-6 gap-y-3 text-sm sm:grid-cols-[10rem_minmax(0,1fr)]">
    <dt class="font-medium text-ink">Source</dt><dd id="analytics-source" class="max-w-[72ch] text-ink-muted [overflow-wrap:anywhere]">{{.Journey.Current.Contract.Source.Text}}</dd>
    <dt class="font-medium text-ink">Reporting window</dt><dd class="text-ink-muted">{{.Journey.Current.Contract.Window.Start.Format "2006-01-02"}} inclusive to {{.Journey.Current.Contract.Window.End.Format "2006-01-02"}} exclusive, UTC.{{if .PreviousLabel}} Previous: {{.Journey.Previous.Contract.Window.Start.Format "2006-01-02"}} inclusive to {{.Journey.Previous.Contract.Window.End.Format "2006-01-02"}} exclusive, UTC.{{end}}</dd>
    <dt class="font-medium text-ink">Source freshness</dt><dd class="text-ink-muted">{{.Observation}}</dd>
    <dt class="font-medium text-ink">Fixture clock</dt><dd class="text-ink-muted">{{.Evaluated}}, fixed for reproducible examples, not the current time.</dd>
    <dt class="font-medium text-ink">Grain and unit</dt><dd class="max-w-[72ch] text-ink-muted">{{.Journey.Current.Contract.Grain}}; {{.Journey.Current.Contract.Unit}}.</dd>
    <dt class="font-medium text-ink">Aggregation</dt><dd class="max-w-[72ch] text-ink-muted">{{.Journey.Current.Contract.Aggregation}}.</dd>
    {{if .Journey.Current.Contract.CoverageNote}}<dt class="font-medium text-ink">Coverage</dt><dd class="max-w-[72ch] text-ink-muted">{{.Journey.Current.Contract.CoverageNote}}</dd>{{end}}
    <dt class="font-medium text-ink">Uncertainty</dt><dd class="max-w-[72ch] text-ink-muted">{{.Journey.Current.Contract.Uncertainty}}</dd>
  </dl>
</details>

<aside class="mt-8 border-t border-line pt-5 text-sm text-ink-muted" aria-label="Integration and proof status">
  <p class="max-w-[72ch]">This reference composes separate typed period datasets, shared validation, SVG primitives and CSV encoding. Apps still own queries, authorization, aggregation and failure policy.</p>
  <p class="mt-2 max-w-[72ch]"><a href="{{.Recipe}}" class="inline-flex min-h-11 items-center text-accent underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-focus">Inspect the reusable single-dataset renderer</a>. It renders the selected current period through <code>analytics.RenderBars</code>, without the app-owned paired-period workflow.</p>
  <p class="mt-2 max-w-[72ch]">Local browser fixture checks are documented, not certification. Full keyboard, zoom, contrast, performance and consumer acceptance remain open. <a href="{{.Catalog}}" class="text-accent underline underline-offset-4">Inspect the capability and its limits</a>.</p>
</aside>`

const analyticsErrorHTML = `
<div class="max-w-2xl">
  <p class="text-xs font-medium uppercase tracking-wide text-ink-muted">Analytics reference · {{.Status}}</p>
  <h1 class="mt-2 text-2xl font-semibold text-ink">The report could not be shown</h1>
  <p class="mt-3 leading-relaxed text-ink-muted">{{.Message}}</p>
  <a href="{{.Back}}" class="mt-5 inline-flex min-h-11 items-center rounded-md bg-accent px-4 py-2 text-sm font-medium text-accent-ink hover:bg-accent-hover focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">Return to a valid report</a>
</div>`
