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
		fact := console.ServiceFact{Name: s.Service, SymbolsNote: s.SymbolsNote, OpenObservations: s.OpenObservations, ObservationsErr: s.ObservationsError}
		if s.SymbolsNote == "" {
			n := s.Symbols
			fact.Symbols = &n
		}
		f.Services = append(f.Services, fact)
	}
	return f
}

// modelCallFacts is the model's one read of the resolved call log. A model
// built by buildObservatoryModel carries its facts (a resolved rule), and
// CallTotal classifies them itself: a missing or unconfigured log is
// absent, a log that exists and could not be read is an ERROR naming the
// failure (design §2) — the model's CallLogNote says the same words, but it
// must not turn that error into an absence. A model assembled from a
// snapshot rather than buildObservatoryModel (a fixture, a preview) carries
// no facts: its own CallLogNote is its read state, and otherwise its total
// is the per-service counts' sum, labeled as such.
func modelCallFacts(m observatoryModel) console.CallLogFacts {
	f := m.callFacts
	switch {
	case f.Log.Rule != "":
		return f
	case m.CallLogNote != "":
		f.AbsentReason = m.CallLogNote
	default:
		rows := 0
		for _, s := range m.Services {
			rows += s.CallsTotal
		}
		f = console.CallLogFacts{Rows: rows, Log: console.ResolvedCallLog{Path: "snapshot", Rule: "snapshot", Note: "a model snapshot's per-service call counts"}}
	}
	return f
}

// callLogRuleCaption names the log a call figure was read from and the rule
// that chose it (OC-2), for a tile's caption.
func callLogRuleCaption(f console.CallLogFacts) string {
	if f.Log.Path == "" || f.Log.Rule == "snapshot" {
		return ""
	}
	return "read from " + f.Log.Path + " (rule " + string(f.Log.Rule) + ")"
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
