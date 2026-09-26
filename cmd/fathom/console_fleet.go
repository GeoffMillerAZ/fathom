package main

// console_fleet.go — the Fleet page at /fleet (vision 35 §3). F0 moves the
// fleet tiles onto the shell; the page's JSON carries `fathom fleet report
// --json` computed by the verb's own function (buildFleetReport), so the
// page and the verb cannot disagree (P10, OC-8). Wave B rebuilds the page
// as the sortable fleet table (OC-19, OC-20).

import (
	"net/http"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
)

func init() {
	declarePage(consolePage{
		ID: "fleet", Route: "/fleet", Href: "/fleet", Group: "Fleet", Order: 0, Nav: true,
		Title:    "Fleet",
		Question: "How is every project doing: wired, fresh, proven, used?",
		Verbs:    []string{"fathom fleet report --json"},
		Shows:    []console.MetricID{console.MetricServices, console.MetricSymbols, console.MetricCalls, console.MetricObservations},
		Live:     []string{"index", "layers", "sessions"},
		Build:    buildFleetPage,
	})
}

func buildFleetPage(cr *consoleRequest) (consoleOut, error) {
	m := cr.model()
	out := consoleOut{
		Meta:    modelMeta(m),
		Model:   m,
		Metrics: pageMetrics(cr.page, consoleModelMetrics(m)),
		Body:    func() string { return obsContent("fleet", renderObsFleet(m), m) },
	}
	// The verb section is computed for JSON only: the fleet report walks
	// every repository and is not on the HTML render path.
	if wantsJSON(cr.r) {
		hc := &http.Client{Timeout: 5 * time.Second}
		rep, err := buildFleetReport(hc, cr.deps.selfURL, fleetReportNow())
		if err != nil {
			out.Sections = map[string]any{"fathom fleet report --json": map[string]string{"error": err.Error()}}
		} else {
			out.Sections = map[string]any{"fathom fleet report --json": rep}
		}
	}
	return out, nil
}
