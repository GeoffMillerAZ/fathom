// Package investigation composes evidence, investigation, and review foundations
// into a governed investigation workflow.
//
// The kit wires together:
//   - foundations/evidence/evidence-graph   (evidence store + contradiction detection)
//   - foundations/evidence/evidence-span    (canonical span type for evidence ingestion)
//   - foundations/evidence/evidence-resolver (span registry and resolution)
//   - foundations/investigation/investigation-journal (append-only audit trail)
//   - foundations/investigation/hypothesis-ranker     (evidence-weighted ranking)
//   - foundations/investigation/convergence-oracle    (convergence decision)
//   - foundations/investigation/dad-investigator      (DAD reasoning loop)
//   - foundations/review/concern-flags                (typed concern surface)
//
// # Finding evidence provenance contract
//
// Every Finding carries SupportingEvidenceIDs: the IDs of the evidence items
// (EvidenceInput.ID values) that were fed into the DAD step that produced the
// finding. A finding with an empty SupportingEvidenceIDs slice is a degraded
// state — it means the DAD loop produced a finding with no linked evidence.
// This is the anti-pattern the kit exists to prevent.
//
// The kit does NOT hard-fail on zero-evidence findings, because the DAD machine
// may produce a NeedsMoreSources decision (not a real finding) before any
// evidence is added. Instead, a finding with zero evidence IDs:
//   - has EvidenceDegraded set to true, and
//   - causes a "finding_without_evidence" concern flag to be raised.
//
// Callers MUST inspect EvidenceDegraded and surface it to users. A finding
// with EvidenceDegraded=true cannot be cited in a legal or governance report.
//
// Concurrency: Investigation is NOT safe for concurrent use; it accumulates
// mutable state (evidence graph, journal, hypotheses) and callers must not
// share an Investigation across goroutines without external synchronisation.
package investigation

import (
	dadinvestigator "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/investigation/dad-investigator"
	concernflags "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/review/concern-flags"
)

// EvidenceInput is a single piece of evidence submitted to an Investigation.
type EvidenceInput struct {
	// ID is a stable unique identifier for this evidence item.
	ID string

	// Content is the textual body of the evidence.
	Content string

	// Weight is the relative importance of this evidence item; must be > 0.
	Weight int

	// EntityID groups evidence items under a shared real-world entity.
	EntityID string

	// Field names the attribute being claimed (e.g. "owner", "status").
	Field string

	// Value is the claimed value for EntityID.Field.
	Value string

	// Confidence is the certainty score in [0, 1].
	Confidence float64

	// Source identifies the originating source (e.g. "code-review", "alert").
	Source string
}

// Status is the current convergence state of an Investigation.
type Status struct {
	// Verdict is the convergence oracle's assessment ("converged", "exploring",
	// or "needs_sources").
	Verdict string

	// Steps is the number of full DAD loops completed so far.
	Steps int

	// Phase is the current DAD phase of the investigation state machine.
	Phase string

	// FindingCount is the number of findings accumulated so far.
	FindingCount int

	// ContradictionCount is the number of detected evidence contradictions.
	ContradictionCount int
}

// Finding is a single governance-annotated finding produced by the investigation.
type Finding struct {
	// Text is the human-readable finding description produced by the DAD loop.
	Text string

	// Confidence is the DAD investigator's confidence band for this finding.
	Confidence dadinvestigator.Confidence

	// Terminal indicates whether the DAD machine reached a terminal phase.
	Terminal bool

	// ConcernFlags are the typed concerns raised against this finding.
	ConcernFlags []concernflags.ConcernFlag

	// SupportingEvidenceIDs contains the IDs of the EvidenceInput values that
	// were active in the DAD step that produced this finding. These are the IDs
	// that give the finding its epistemic grounding.
	//
	// Empty SupportingEvidenceIDs means the finding was produced without any
	// linked evidence — a degraded state. Inspect EvidenceDegraded to detect this.
	//
	// Callers must surface this slice when citing the finding in reports; a
	// finding without evidence IDs cannot be used to support a governance claim.
	SupportingEvidenceIDs []string

	// EvidenceDegraded is true when SupportingEvidenceIDs is empty, meaning
	// this finding was produced without any linked evidence items. A concern
	// flag of type "finding_without_evidence" is also raised automatically.
	//
	// A finding with EvidenceDegraded=true must NOT be cited in legal or
	// governance reports. It signals that the investigation step lacked proper
	// evidentiary grounding — the DAD machine advanced without evidence input.
	EvidenceDegraded bool
}
