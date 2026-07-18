// Package improvementflywheel accumulates workflow-outcome signals and emits
// prioritised improvement proposals deterministically.
//
// The flywheel cycle:
//
//  1. Observe(signal) — receives a Signal and groups it into a PatternObservation
//     when the signal type+key combination has been seen before, or seeds a new
//     observation when it hasn't. No proposal is emitted immediately.
//
//  2. Proposals() — returns a deduplicated, deterministically sorted slice of
//     Proposals derived from all observations. The sort order is:
//     a) descending Frequency (most-seen patterns first)
//     b) ascending PatternType string (tie-break for stability)
//     c) ascending Key string (final tie-break)
//
// The flywheel never auto-applies changes — proposals require human approval.
// This package produces the proposal list; adoption is out of scope.
//
// A Flywheel is not safe for concurrent use; guard it with your own
// synchronization when observing signals from multiple goroutines.
package improvementflywheel

// PatternType classifies the kind of repeated signal that produced an observation.
type PatternType string

const (
	// PatternFalsePositive — concern flags repeatedly dismissed with similar rationale.
	PatternFalsePositive PatternType = "false_positive"
	// PatternMissedClaim — claims that repeatedly fail the eval-gate after generation.
	PatternMissedClaim PatternType = "missed_claim"
	// PatternRepeatedCorrection — the same fact corrected by multiple reviewers independently.
	PatternRepeatedCorrection PatternType = "repeated_correction"
	// PatternOperatorFriction — elicitation requests repeatedly answered the same way.
	PatternOperatorFriction PatternType = "operator_friction"
)

// Signal is a single workflow-outcome event that feeds the flywheel.
type Signal struct {
	// Type classifies what kind of outcome this signal represents.
	Type PatternType
	// Key is a stable identifier for the pattern (e.g. concern category, claim
	// type, field path, or elicitation prompt key). Signals with the same Type
	// and Key are grouped into the same PatternObservation.
	Key string
	// WorkflowID is the source workflow run that produced this signal.
	WorkflowID string
	// Detail is an optional human-readable description of this specific instance.
	Detail string
}

// PatternObservation groups repeated signals into a named observation.
type PatternObservation struct {
	// Type is the pattern type shared by all grouped signals.
	Type PatternType
	// Key is the stable grouping key.
	Key string
	// Frequency is the number of signals observed for this (Type, Key) pair.
	Frequency int
	// WorkflowIDs contains the source workflow IDs of all grouped signals
	// (deduplicated, sorted for determinism).
	WorkflowIDs []string
	// Examples holds the Detail strings of the first few signals (up to maxExamples).
	Examples []string
}

// ChangeTarget enumerates the kinds of platform artifact a proposal targets.
type ChangeTarget string

const (
	// TargetCatalogEntry — the proposal targets a catalog rule entry.
	TargetCatalogEntry ChangeTarget = "catalog_entry"
	// TargetProsePolicy — the proposal targets a prose policy rule.
	TargetProsePolicy ChangeTarget = "prose_policy"
	// TargetPromptSection — the proposal targets a prompt section.
	TargetPromptSection ChangeTarget = "prompt_section"
	// TargetCUEConstraint — the proposal targets a CUE schema constraint.
	TargetCUEConstraint ChangeTarget = "cue_constraint"
	// TargetFoundationBehavior — the proposal targets a foundation behavior.
	TargetFoundationBehavior ChangeTarget = "foundation_behavior"
)

// Proposal is a structured improvement suggestion derived from one or more
// PatternObservations. It is produced by Proposals() and requires human review
// before any adoption.
type Proposal struct {
	// ID is a stable, deterministic identifier derived from the observation key.
	ID string
	// ObservationKey is the (Type+Key) this proposal derives from.
	ObservationKey string
	// PatternType is the type of the underlying pattern.
	PatternType PatternType
	// ChangeTarget identifies the kind of artifact the proposal would change.
	ChangeTarget ChangeTarget
	// Summary is a short human-readable description of the proposed change.
	Summary string
	// Frequency is the observation frequency — used to prioritise the list.
	Frequency int
	// SupportingWorkflowIDs are the workflow runs that support this proposal.
	SupportingWorkflowIDs []string
}
