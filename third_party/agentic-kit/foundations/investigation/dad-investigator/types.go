// Package dadinvestigator implements the DAD investigator pattern as a
// deterministic state machine over evidence.
//
// DAD stands for:
//
//	Decompose — break the investigation question into sub-questions
//	           by cataloguing the incoming evidence items.
//	Analyse   — score and rank each evidence item to surface the most
//	           significant signals.
//	Decide    — synthesise the analysed evidence into a Decision with a
//	           confidence band, marking the investigation terminal when
//	           sufficient confidence is reached or evidence is exhausted.
//
// The state machine converges deterministically to a finding given the
// same (State, []EvidenceItem) inputs.
//
// Concurrency contract: Investigator is a zero-size type whose methods are
// pure functions with no shared mutable state; all exported functions are
// safe to call concurrently from multiple goroutines.
package dadinvestigator

// Phase represents the current stage of a DAD investigation loop.
// The machine advances: Decompose → Analyse → Decide, then either loops
// back to Decompose (if more evidence exists and confidence is not yet
// sufficient) or terminates with Found or Exhausted.
type Phase string

const (
	// PhaseDecompose is the first phase: catalogue incoming evidence.
	PhaseDecompose Phase = "decompose"

	// PhaseAnalyse is the second phase: score and rank evidence.
	PhaseAnalyse Phase = "analyse"

	// PhaseDecide is the third phase: synthesise a Decision.
	PhaseDecide Phase = "decide"

	// PhaseFound is the terminal phase: a high-confidence finding was reached.
	PhaseFound Phase = "found"

	// PhaseExhausted is the terminal phase: evidence was exhausted without
	// reaching sufficient confidence.
	PhaseExhausted Phase = "exhausted"
)

// Confidence represents the confidence band of a Decision.
type Confidence string

const (
	// ConfidenceNone means no confidence has been established yet.
	ConfidenceNone Confidence = "none"
	// ConfidenceLow means the evidence supports only a low-confidence
	// determination.
	ConfidenceLow Confidence = "low"
	// ConfidenceMedium means the evidence supports a medium-confidence
	// determination.
	ConfidenceMedium Confidence = "medium"
	// ConfidenceHigh means the evidence supports a high-confidence
	// determination.
	ConfidenceHigh Confidence = "high"
)

// confidenceRank maps Confidence values to numeric ranks for comparison.
var confidenceRank = map[Confidence]int{
	ConfidenceNone:   0,
	ConfidenceLow:    1,
	ConfidenceMedium: 2,
	ConfidenceHigh:   3,
}

// ConfidenceGTE returns true if a is greater than or equal to b.
func ConfidenceGTE(a, b Confidence) bool {
	return confidenceRank[a] >= confidenceRank[b]
}

// EvidenceItem is a single piece of evidence fed into the investigation.
// It mirrors #EvidenceItem in schema.cue.
type EvidenceItem struct {
	// ID is a unique identifier for this evidence item.
	ID string

	// Content is the text body of the evidence.
	Content string

	// Weight is the relative importance of this evidence item.
	// Must be > 0; treated as 1 if zero or negative.
	Weight int
}

// Decision is the output produced by the Decide phase for a single step.
// It mirrors #Decision in schema.cue.
type Decision struct {
	// Finding is a short description of the conclusion reached in this step.
	Finding string

	// Confidence is the confidence band attached to this decision.
	Confidence Confidence

	// Terminal indicates whether this decision terminates the investigation.
	// When Terminal is true the machine is in PhaseFound or PhaseExhausted.
	Terminal bool
}

// State holds the current state of the DAD investigation state machine.
type State struct {
	// Phase is the current phase of the investigation.
	Phase Phase

	// Pending is the evidence items not yet processed in this loop iteration.
	Pending []EvidenceItem

	// Analysed is the evidence items that have been scored in the current
	// Analyse phase. Reset on each new Decompose phase.
	Analysed []ScoredEvidence

	// AccumulatedWeight is the sum of weights of all evidence processed so
	// far. It drives the confidence calculation.
	AccumulatedWeight int

	// Steps is the number of full DAD loops completed.
	Steps int
}

// ScoredEvidence is an EvidenceItem annotated with a score produced during
// the Analyse phase.
type ScoredEvidence struct {
	// Item is the original evidence item.
	Item EvidenceItem

	// Score is the normalised relevance score in [0, 100].
	Score int
}
