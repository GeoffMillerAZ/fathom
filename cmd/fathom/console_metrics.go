package main

// console_metrics.go — OC-9's adapter from the observatory model to the
// shared metric projections (internal/console/metrics.go). The fleet tiles,
// the home page and every page's JSON "metrics" section read these, so the
// same metric can never render two ways.

import (
	"github.com/geoffmilleraz/fathom/internal/console"
)

// fleetFactsFromModel is the registry's answer as the projections see it.
func fleetFactsFromModel(m observatoryModel) console.FleetFacts {
	f := console.FleetFacts{RegistryNote: m.DegradedNote}
	for _, s := range m.Services {
		fact := console.ServiceFact{Name: s.Service, SymbolsNote: s.SymbolsNote, OpenObservations: s.OpenObservations}
		if s.SymbolsNote == "" {
			n := s.Symbols
			fact.Symbols = &n
		}
		f.Services = append(f.Services, fact)
	}
	return f
}

// modelCallFacts is the model's one read of the resolved call log. The
// model's own CallLogNote is its read state and wins (the model's contract:
// empty means the log was read). A model assembled from a snapshot rather
// than buildObservatoryModel (a fixture, a preview) carries only per-service
// counts, so its total is their sum, labeled as such.
func modelCallFacts(m observatoryModel) console.CallLogFacts {
	f := m.callFacts
	switch {
	case m.CallLogNote != "":
		f.AbsentReason = m.CallLogNote
	case f.Log.Rule == "":
		rows := 0
		for _, s := range m.Services {
			rows += s.CallsTotal
		}
		f = console.CallLogFacts{Rows: rows, Log: console.ResolvedCallLog{Path: "snapshot", Rule: "snapshot", Note: "a model snapshot's per-service call counts"}}
	}
	return f
}

// modelSessions is the model's shared sessions metric.
func modelSessions(m observatoryModel) console.Value[int] {
	if m.sessions.State == "" {
		return console.Absent[int]("the session ledger was not read for this snapshot")
	}
	return m.sessions
}

// consoleModelMetrics is every shared metric for a page backed by the model.
func consoleModelMetrics(m observatoryModel) console.Metrics {
	f := fleetFactsFromModel(m)
	return console.Metrics{
		console.MetricServices:     console.ServiceCount(f),
		console.MetricSymbols:      console.SymbolTotal(f),
		console.MetricCalls:        console.CallTotal(modelCallFacts(m)),
		console.MetricSessions:     modelSessions(m),
		console.MetricObservations: console.OpenObservations(f),
	}
}
