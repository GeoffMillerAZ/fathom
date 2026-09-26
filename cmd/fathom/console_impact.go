package main

// console_impact.go — the Impact page at /impact (vision 35 §3, Evidence).
// F0 moves the four-questions page onto the shell unchanged in content; wave
// F rebuilds it with ordered questions, arm ranges and the catches timeline
// (OC-41, OC-42).

import (
	"github.com/geoffmilleraz/fathom/internal/console"
)

func init() {
	declarePage(consolePage{
		ID: "impact", Route: "/impact", Href: "/impact", Group: "Evidence", Order: 0, Nav: true,
		Title:    "Impact",
		Question: "Has fathom helped, measured honestly, with ranges and arm counts?",
		Shows:    []console.MetricID{console.MetricSessions, console.MetricObservations},
		Live:     []string{"sessions", "plans"},
		Build:    buildImpactPage,
	})
}

func buildImpactPage(cr *consoleRequest) (consoleOut, error) {
	m := cr.model()
	return consoleOut{
		Meta:    modelMeta(m),
		Model:   m.Impact,
		Metrics: pageMetrics(cr.page, consoleModelMetrics(m)),
		Body:    func() string { return obsContent("impact", renderObsImpact(m), m) },
	}, nil
}
