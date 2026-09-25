package gallery

import (
	"net/http"
	"time"
)

// A failed source is an app-owned response, never an empty or missing Dataset.
// Call before building either reporting window, resolving detail or encoding CSV.
func writeAnalyticsSourceError(w http.ResponseWriter, r *http.Request, selection analyticsSelection, theme, reportPath string) {
	recovery := selection
	recovery.State = analyticsStateComplete
	comparison := "No comparison"
	if selection.Compare == analyticsComparePrevious {
		comparison = "Previous calendar month"
	}
	period := string(selection.Period)
	if start, err := time.Parse("2006-01", period); err == nil {
		period = start.Format("January 2006")
	}
	content, err := htmlf("analytics-source-error", analyticsSourceErrorHTML, struct {
		Period, Group, Comparison, Recovery string
	}{period, analyticsCategoryLabel(selection.Category), comparison, analyticsURL(reportPath, recovery, theme)})
	writeAnalyticsPage(w, r, http.StatusServiceUnavailable, "Service operations — source unavailable", content, err, selection, theme, reportPath)
}

const analyticsSourceErrorHTML = `
<section id="analytics-source-error" aria-labelledby="analytics-source-error-heading" class="max-w-2xl">
  <p class="text-xs font-medium uppercase tracking-wide text-ink-muted">Analytics reference · synthetic source failure</p>
  <h1 id="analytics-source-error-heading" class="mt-2 text-2xl font-semibold text-ink">The data source could not be read</h1>
  <p class="mt-3 leading-relaxed text-ink">The synthetic source failed. No current or previous measurements can be trusted, so totals, comparisons, service details and exports are withheld.</p>
  <p class="mt-3 leading-relaxed text-ink-muted">This is not zero activity and not an empty result. Missing observations are a different condition. Even an empty group cannot be reported as a successful query while the source has failed.</p>
  <dl class="mt-6 space-y-2 text-sm">
    <dt class="font-medium text-ink">Reporting month</dt><dd class="text-ink-muted">{{.Period}} · UTC</dd>
    <dt class="font-medium text-ink">Service group</dt><dd class="text-ink-muted">{{.Group}}</dd>
    <dt class="font-medium text-ink">Comparison</dt><dd class="text-ink-muted">{{.Comparison}}</dd>
  </dl>
  <a href="{{.Recovery}}" class="mt-5 inline-flex min-h-11 items-center rounded-md bg-accent px-4 py-2 text-sm font-medium text-accent-ink hover:bg-accent-hover focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">Load the complete fixture</a>
  <p class="mt-3 text-sm leading-relaxed text-ink-muted">Recovery changes only the source condition; your reporting month, comparison, group and appearance remain selected. This is a local demonstration. No live service was queried or refreshed.</p>
</section>`
