// Package governedinvestigation is an end-to-end scenario blueprint that wires
// evidence-span, evidence-graph, investigation-journal, hypothesis-ranker, and
// dad-investigator into a seeded, deterministic governed investigation.
//
// The scenario demonstrates the full evidence→investigation→finding pipeline
// without any external data sources or network calls. All inputs are seeded
// fixtures defined in this package.
//
// Entry point: [Run]. It returns a [Report] that captures every intermediate
// state and the terminal finding. Calling Run with the same seed always
// produces an identical Report.
package governedinvestigation

import (
	dad "github.com/geoffmilleraz/agentic-kit/foundations/investigation/dad-investigator"
	hypothesisranker "github.com/geoffmilleraz/agentic-kit/foundations/investigation/hypothesis-ranker"
)

// Finding is the terminal finding produced by the investigation.
type Finding struct {
	// Summary is the narrative conclusion derived from the DAD loop.
	Summary string

	// Confidence is the confidence band of the terminal decision.
	Confidence dad.Confidence

	// TopHypothesis is the highest-ranked hypothesis after evidence analysis.
	TopHypothesis hypothesisranker.Hypothesis
}

// JournalEntry is a single recorded step in the scenario's investigation log.
type JournalEntry struct {
	// Step is the name of the investigation step (mirrors journal Entry.Step).
	Step string

	// Note is the human-readable description of the step.
	Note string
}

// Report is the full outcome of a single [Run] invocation. It carries the
// terminal finding, the investigation journal, and the hypothesis ranking
// so callers can inspect every layer of the pipeline.
type Report struct {
	// Journal is the ordered list of steps taken during the investigation.
	Journal []JournalEntry

	// RankedHypotheses is the list of hypotheses sorted by support descending,
	// as produced by hypothesis-ranker after evidence collection.
	RankedHypotheses []hypothesisranker.Hypothesis

	// Finding is the terminal finding from the DAD loop.
	Finding Finding

	// EvidenceCount is the number of seeded evidence items that were processed.
	EvidenceCount int
}
