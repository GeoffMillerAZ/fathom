// Package governedinvestigation is an end-to-end scenario blueprint that wires
// evidence-span, evidence-graph, investigation-journal, hypothesis-ranker, and
// dad-investigator into a seeded, deterministic governed investigation, then
// proposes the one irreversible remediation the finding implies through
// agent-harness under the strict effect gate — a human approval (seeded) sits
// between the model's tool call and its dispatch.
//
// The scenario demonstrates the full evidence→investigation→finding→gated
// remediation pipeline without any external data sources or network calls.
// All inputs are seeded fixtures defined in this package; the model is an
// llmclient.MockClient script and the approver is a fixture that approves.
//
// Concurrency: Run is safe to call concurrently — every call builds its own
// journal, registry, harness and sink; nothing is shared across calls.
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

// Remediation is the outcome of the scenario's one irreversible action:
// the scripted model's rollback_deployment call, dispatched through
// agent-harness with EffectPolicy strict, so the tool's declared destructive
// Effect routes it to the seeded approver before it runs.
type Remediation struct {
	// ToolName is the destructive tool the model called.
	ToolName string

	// Effect is the gate's label for why approval was required.
	Effect string

	// Approved reports whether the seeded approver granted the dispatch.
	Approved bool

	// ApprovedBy is the approver's identity as recorded on the gate event.
	ApprovedBy string

	// GateApproved and GateDenied count the effect-gate events the harness
	// emitted for this run; exactly one approval and no denials is the
	// expected shape.
	GateApproved int
	GateDenied   int

	// Stop is the harness stop cause for the remediation run.
	Stop string

	// Summary is the model's final text after the rollback ran.
	Summary string
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

	// Remediation is the gated dispatch of the one destructive tool the
	// finding implies.
	Remediation Remediation
}
