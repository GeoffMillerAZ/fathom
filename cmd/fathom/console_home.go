package main

// console_home.go — the Now page at / (vision 35 §3). F0 moves the existing
// front door onto the console shell unchanged in content; wave B rebuilds it
// as the attention queue (OC-17..OC-20).

import (
	"github.com/geoffmilleraz/fathom/internal/console"
)

func init() {
	declarePage(consolePage{
		ID: "home", Route: "/{$}", Href: "/", Group: "Now", Order: 0, Nav: true,
		Title: "Now", NavLabel: "Home",
		Question: "What needs my attention, and what changed since I last looked?",
		Shows:    []console.MetricID{console.MetricServices, console.MetricCalls, console.MetricObservations, console.MetricAgentTools},
		Live:     []string{"index", "sessions", "plans"},
		Build:    buildHomePage,
	})
}

// homeModel is the front door's JSON projection.
type homeModel struct {
	Impact            obsImpact `json:"impact"`
	AgentSurfaceTools int       `json:"agent_surface_tools"`
	AgentSchemaBytes  int       `json:"agent_schema_bytes"`
	DegradedNote      string    `json:"degraded_note,omitempty"`
	CallLogNote       string    `json:"call_log_note,omitempty"`
}

func buildHomePage(cr *consoleRequest) (consoleOut, error) {
	m := cr.model()
	return consoleOut{
		Meta:    modelMeta(m),
		Model:   homeModel{Impact: m.Impact, AgentSurfaceTools: m.AgentSurfaceTools, AgentSchemaBytes: m.AgentSchemaBytes, DegradedNote: m.DegradedNote, CallLogNote: m.CallLogNote},
		Metrics: pageMetrics(cr.page, consoleModelMetrics(m)),
		Body:    func() string { return obsContent("home", renderObsLandingBody(m), m) },
	}, nil
}

// pageMetrics keeps only the shared metrics the page declares it shows.
func pageMetrics(p *consolePage, all console.Metrics) console.Metrics {
	if len(p.Shows) == 0 {
		return nil
	}
	out := console.Metrics{}
	for _, id := range p.Shows {
		if v, ok := all[id]; ok {
			out[id] = v
		}
	}
	return out
}
